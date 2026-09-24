package main

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

const (
	testOverlayDev        = "overlays/dev"
	testOverlayProd       = "overlays/prod"
	testSchemaLocation    = "-schema-location"
	testSchemaLocationURL = "https://example.invalid/two words"
	testStrict            = "-strict"
	binKustomize          = "/bin/kustomize"
	binKubeconform        = "/bin/kubeconform"
)

var (
	errBuildFailed      = errors.New("build failed")
	errValidationFailed = errors.New("validation failed")
	errMissing          = errors.New("missing")
)

type invocation struct {
	name  string
	args  []string
	stdin []byte
}

func TestSplitArgsPreservesSeparatorArgumentsVerbatim(t *testing.T) {
	t.Parallel()

	overlays, kubeconformArgs := splitArgs(
		[]string{
			testOverlayDev,
			testOverlayProd,
			"--",
			testSchemaLocation,
			testSchemaLocationURL,
			"--",
			testStrict,
		},
	)
	if !reflect.DeepEqual(overlays, []string{testOverlayDev, testOverlayProd}) {
		t.Fatalf("overlays = %#v", overlays)
	}

	want := []string{testSchemaLocation, testSchemaLocationURL, "--", testStrict}
	if !reflect.DeepEqual(kubeconformArgs, want) {
		t.Fatalf("kubeconform args = %#v, want %#v", kubeconformArgs, want)
	}
}

func TestRunWithoutOverlaysUsesUsageBeforeLookup(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	lookups := 0

	status := run([]string{"--", testStrict}, func(string) (string, error) {
		lookups++

		return "", nil
	}, func(string, []string, io.Reader, io.Writer, io.Writer) error {
		t.Fatal("command must not run")

		return nil
	}, io.Discard, &stderr)
	if status != 2 || lookups != 0 || !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("status=%d lookups=%d stderr=%q", status, lookups, stderr.String())
	}
}

func TestRunOneOverlayForwardsDiscreteArgumentsAndBytes(t *testing.T) {
	t.Parallel()

	var calls []invocation

	status := run(
		[]string{testOverlayDev, "--", testSchemaLocation, testSchemaLocationURL},
		lookupOK,
		func(name string, args []string, stdin io.Reader, stdout io.Writer, _ io.Writer) error {
			var input []byte

			if stdin != nil {
				var err error

				input, err = io.ReadAll(stdin)
				if err != nil {
					t.Fatal(err)
				}
			}

			calls = append(
				calls,
				invocation{name: name, args: append([]string(nil), args...), stdin: input},
			)

			if name == binKustomize {
				_, _ = stdout.Write([]byte{'y', 0xff, '\n'})
			}

			return nil
		},
		io.Discard,
		io.Discard,
	)
	if status != 0 {
		t.Fatalf("status = %d", status)
	}

	want := []invocation{
		{name: binKustomize, args: []string{"build", testOverlayDev}, stdin: nil},
		{
			name:  binKubeconform,
			args:  []string{testSchemaLocation, testSchemaLocationURL},
			stdin: []byte{'y', 0xff, '\n'},
		},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRunBuildFailureSkipsValidation(t *testing.T) {
	t.Parallel()

	var calls []invocation

	status := run(
		[]string{"broken"},
		lookupOK,
		func(name string, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			calls = append(
				calls,
				invocation{name: name, args: append([]string(nil), args...), stdin: nil},
			)

			return errBuildFailed
		},
		io.Discard,
		io.Discard,
	)
	if status != 1 ||
		!reflect.DeepEqual(
			calls,
			[]invocation{{name: binKustomize, args: []string{"build", "broken"}, stdin: nil}},
		) {
		t.Fatalf("status=%d calls=%#v", status, calls)
	}
}

func TestRunValidationFailureReturnsOne(t *testing.T) {
	t.Parallel()

	status := run(
		[]string{"valid"},
		lookupOK,
		func(name string, _ []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
			if name == binKustomize {
				_, _ = stdout.Write([]byte("apiVersion: v1\n"))

				return nil
			}

			return errValidationFailed
		},
		io.Discard,
		io.Discard,
	)
	if status != 1 {
		t.Fatalf("status = %d", status)
	}
}

func TestRunMultipleOverlaysContinuesAndAggregatesFailures(t *testing.T) {
	t.Parallel()

	var calls []invocation

	status := run(
		[]string{"bad-build", "bad-validation", "good"},
		lookupOK,
		func(name string, args []string, stdin io.Reader, stdout io.Writer, _ io.Writer) error {
			var input []byte
			if stdin != nil {
				input, _ = io.ReadAll(stdin)
			}

			calls = append(
				calls,
				invocation{name: name, args: append([]string(nil), args...), stdin: input},
			)

			if name == binKustomize {
				if args[1] == "bad-build" {
					return errBuildFailed
				}

				_, _ = stdout.Write([]byte(args[1]))

				return nil
			}

			if string(input) == "bad-validation" {
				return errValidationFailed
			}

			return nil
		},
		io.Discard,
		io.Discard,
	)
	if status != 1 || len(calls) != 5 {
		t.Fatalf("status=%d calls=%#v", status, calls)
	}

	if calls[4].name != binKubeconform || string(calls[4].stdin) != "good" {
		t.Fatalf("did not continue to final overlay: %#v", calls[4])
	}
}

func TestRunMultipleSuccessfulOverlaysReturnsZero(t *testing.T) {
	t.Parallel()

	status := run(
		[]string{"one", "two"},
		lookupOK,
		func(name string, args []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
			if name == binKustomize {
				_, _ = stdout.Write([]byte(args[1]))
			}

			return nil
		},
		io.Discard,
		io.Discard,
	)
	if status != 0 {
		t.Fatalf("status = %d", status)
	}
}

func TestRunReportsMissingExecutables(t *testing.T) {
	t.Parallel()

	for _, missing := range []string{"kustomize", "kubeconform"} {
		t.Run(missing, func(t *testing.T) {
			t.Parallel()

			var (
				stderr   bytes.Buffer
				lookedUp []string
			)

			status := run([]string{"overlay"}, func(name string) (string, error) {
				lookedUp = append(lookedUp, name)

				if name == missing {
					return "", errMissing
				}

				return "/bin/" + name, nil
			}, func(string, []string, io.Reader, io.Writer, io.Writer) error {
				t.Fatal("command must not run")

				return nil
			}, io.Discard, &stderr)
			if status != 1 || !reflect.DeepEqual(lookedUp, []string{"kustomize", "kubeconform"}) ||
				!strings.Contains(stderr.String(), missing) ||
				!strings.Contains(stderr.String(), "hook environment is incomplete") {
				t.Fatalf("status=%d lookups=%#v stderr=%q", status, lookedUp, stderr.String())
			}
		})
	}
}

func lookupOK(name string) (string, error) {
	return "/bin/" + name, nil
}
