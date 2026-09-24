package main

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

type invocation struct {
	name  string
	args  []string
	stdin []byte
}

func TestSplitArgsPreservesSeparatorArgumentsVerbatim(t *testing.T) {
	overlays, kubeconformArgs := splitArgs([]string{"overlays/dev", "overlays/prod", "--", "-schema-location", "https://example.invalid/two words", "--", "-strict"})
	if !reflect.DeepEqual(overlays, []string{"overlays/dev", "overlays/prod"}) {
		t.Fatalf("overlays = %#v", overlays)
	}
	want := []string{"-schema-location", "https://example.invalid/two words", "--", "-strict"}
	if !reflect.DeepEqual(kubeconformArgs, want) {
		t.Fatalf("kubeconform args = %#v, want %#v", kubeconformArgs, want)
	}
}

func TestRunWithoutOverlaysUsesUsageBeforeLookup(t *testing.T) {
	var stderr bytes.Buffer
	lookups := 0
	status := run([]string{"--", "-strict"}, func(string) (string, error) {
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
	var calls []invocation
	status := run([]string{"overlays/dev", "--", "-schema-location", "https://example.invalid/two words"}, lookupOK, func(name string, args []string, stdin io.Reader, stdout io.Writer, _ io.Writer) error {
		var input []byte
		if stdin != nil {
			var err error
			input, err = io.ReadAll(stdin)
			if err != nil {
				t.Fatal(err)
			}
		}
		calls = append(calls, invocation{name: name, args: append([]string(nil), args...), stdin: input})
		if name == "/bin/kustomize" {
			_, _ = stdout.Write([]byte{'y', 0xff, '\n'})
		}
		return nil
	}, io.Discard, io.Discard)
	if status != 0 {
		t.Fatalf("status = %d", status)
	}
	want := []invocation{
		{name: "/bin/kustomize", args: []string{"build", "overlays/dev"}, stdin: nil},
		{name: "/bin/kubeconform", args: []string{"-schema-location", "https://example.invalid/two words"}, stdin: []byte{'y', 0xff, '\n'}},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRunBuildFailureSkipsValidation(t *testing.T) {
	var calls []invocation
	status := run([]string{"broken"}, lookupOK, func(name string, args []string, stdin io.Reader, _ io.Writer, _ io.Writer) error {
		calls = append(calls, invocation{name: name, args: append([]string(nil), args...)})
		return errors.New("build failed")
	}, io.Discard, io.Discard)
	if status != 1 || !reflect.DeepEqual(calls, []invocation{{name: "/bin/kustomize", args: []string{"build", "broken"}}}) {
		t.Fatalf("status=%d calls=%#v", status, calls)
	}
}

func TestRunValidationFailureReturnsOne(t *testing.T) {
	status := run([]string{"valid"}, lookupOK, func(name string, args []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
		if name == "/bin/kustomize" {
			_, _ = stdout.Write([]byte("apiVersion: v1\n"))
			return nil
		}
		return errors.New("validation failed")
	}, io.Discard, io.Discard)
	if status != 1 {
		t.Fatalf("status = %d", status)
	}
}

func TestRunMultipleOverlaysContinuesAndAggregatesFailures(t *testing.T) {
	var calls []invocation
	status := run([]string{"bad-build", "bad-validation", "good"}, lookupOK, func(name string, args []string, stdin io.Reader, stdout io.Writer, _ io.Writer) error {
		var input []byte
		if stdin != nil {
			input, _ = io.ReadAll(stdin)
		}
		calls = append(calls, invocation{name: name, args: append([]string(nil), args...), stdin: input})
		if name == "/bin/kustomize" {
			if args[1] == "bad-build" {
				return errors.New("build failed")
			}
			_, _ = stdout.Write([]byte(args[1]))
			return nil
		}
		if string(input) == "bad-validation" {
			return errors.New("validation failed")
		}
		return nil
	}, io.Discard, io.Discard)
	if status != 1 || len(calls) != 5 {
		t.Fatalf("status=%d calls=%#v", status, calls)
	}
	if calls[4].name != "/bin/kubeconform" || string(calls[4].stdin) != "good" {
		t.Fatalf("did not continue to final overlay: %#v", calls[4])
	}
}

func TestRunMultipleSuccessfulOverlaysReturnsZero(t *testing.T) {
	status := run([]string{"one", "two"}, lookupOK, func(name string, args []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
		if name == "/bin/kustomize" {
			_, _ = stdout.Write([]byte(args[1]))
		}
		return nil
	}, io.Discard, io.Discard)
	if status != 0 {
		t.Fatalf("status = %d", status)
	}
}

func TestRunReportsMissingExecutables(t *testing.T) {
	for _, missing := range []string{"kustomize", "kubeconform"} {
		t.Run(missing, func(t *testing.T) {
			var stderr bytes.Buffer
			var lookedUp []string
			status := run([]string{"overlay"}, func(name string) (string, error) {
				lookedUp = append(lookedUp, name)
				if name == missing {
					return "", errors.New("missing")
				}
				return "/bin/" + name, nil
			}, func(string, []string, io.Reader, io.Writer, io.Writer) error {
				t.Fatal("command must not run")
				return nil
			}, io.Discard, &stderr)
			if status != 1 || !reflect.DeepEqual(lookedUp, []string{"kustomize", "kubeconform"}) || !strings.Contains(stderr.String(), missing) || !strings.Contains(stderr.String(), "hook environment is incomplete") {
				t.Fatalf("status=%d lookups=%#v stderr=%q", status, lookedUp, stderr.String())
			}
		})
	}
}

func lookupOK(name string) (string, error) {
	return "/bin/" + name, nil
}
