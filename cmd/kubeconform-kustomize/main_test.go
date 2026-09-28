package main

import (
	"bytes"
	"errors"
	"fmt"
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
	t.Parallel()

	overlays, kubeconformArgs := splitArgs(
		[]string{
			"overlays/dev",
			"overlays/prod",
			"--",
			"-schema-location",
			"https://example.invalid/two words",
			"--",
			"-strict",
		},
	)
	if !reflect.DeepEqual(overlays, []string{"overlays/dev", "overlays/prod"}) {
		t.Fatalf("overlays = %#v", overlays)
	}

	want := []string{"-schema-location", "https://example.invalid/two words", "--", "-strict"}
	if !reflect.DeepEqual(kubeconformArgs, want) {
		t.Fatalf("kubeconform args = %#v, want %#v", kubeconformArgs, want)
	}
}

func TestRunWithoutOverlaysUsesUsageBeforeLookup(t *testing.T) {
	t.Parallel()

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

func TestRunAcceptsKubeconformFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "strict", args: []string{"-strict"}},
		{name: "summary", args: []string{"-summary"}},
		{name: "json output", args: []string{"-output", "json"}},
		{
			name: "schema location",
			args: []string{"-schema-location", "https://example.invalid/{{.ResourceKind}}.json"},
		},
		{name: "kubernetes version", args: []string{"-kubernetes-version", "1.36.2"}},
		{name: "one worker", args: []string{"-n", "1"}},
		{
			name: "repeated schema locations",
			args: []string{"-schema-location", "a", "-schema-location", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls []invocation

			status := run(
				append([]string{"overlay", "--"}, tt.args...),
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

					calls = append(calls, invocation{name: name, args: append([]string(nil), args...), stdin: input})
					if name == "/bin/kustomize" {
						_, _ = stdout.Write([]byte("rendered"))
					}

					return nil
				},
				io.Discard,
				io.Discard,
			)
			if status != exitOK {
				t.Fatalf("status = %d", status)
			}

			if len(calls) != 2 || !reflect.DeepEqual(calls[1].args, tt.args) || string(calls[1].stdin) != "rendered" {
				t.Fatalf("calls = %#v", calls)
			}
		})
	}
}

func TestRunRejectsPositionalKubeconformInputsBeforeExecution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "file", args: []string{"manifest.yaml"}, want: "manifest.yaml"},
		{name: "directory", args: []string{"manifests/"}, want: "manifests/"},
		{name: "with flag", args: []string{"-strict", "extra.yaml"}, want: "extra.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer

			status := run(
				append([]string{"overlay", "--"}, tt.args...),
				func(string) (string, error) {
					t.Fatal("lookup must not run")

					return "", nil
				},
				func(string, []string, io.Reader, io.Writer, io.Writer) error {
					t.Fatal("command must not run")

					return nil
				},
				io.Discard,
				&stderr,
			)
			if status != exitUsageError || !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("status=%d stderr=%q", status, stderr.String())
			}
		})
	}
}

func TestRunRejectsNonPositiveWorkerCountsBeforeExecution(t *testing.T) {
	t.Parallel()

	for _, workers := range []string{"0", "-1"} {
		t.Run(workers, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer

			status := run(
				[]string{"overlay", "--", "-n", workers},
				func(string) (string, error) {
					t.Fatal("lookup must not run")

					return "", nil
				},
				func(string, []string, io.Reader, io.Writer, io.Writer) error {
					t.Fatal("command must not run")

					return nil
				},
				io.Discard,
				&stderr,
			)
			if status != exitUsageError || !strings.Contains(stderr.String(), "worker count (-n) must be greater than zero") {
				t.Fatalf("status=%d stderr=%q", status, stderr.String())
			}
		})
	}
}

func TestRunRejectsKubeconformHelpAndVersionBeforeExecution(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"-h"}, {"-v"}} {
		t.Run(args[0], func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer

			status := run(
				append([]string{"overlay", "--"}, args...),
				func(string) (string, error) {
					t.Fatal("lookup must not run")

					return "", nil
				},
				func(string, []string, io.Reader, io.Writer, io.Writer) error {
					t.Fatal("command must not run")

					return nil
				},
				io.Discard,
				&stderr,
			)
			if status != exitUsageError || !strings.Contains(stderr.String(), "do not validate") {
				t.Fatalf("status=%d stderr=%q", status, stderr.String())
			}
		})
	}
}

func TestRunRejectsMachineReadableOutputAcrossMultipleOverlays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		args   []string
		format string
	}{
		{name: "json", args: []string{"-output", "json"}, format: "json"},
		{name: "junit", args: []string{"-output", "junit"}, format: "junit"},
		{name: "tap", args: []string{"-output", "tap"}, format: "tap"},
		{name: "equals json", args: []string{"-output=json"}, format: "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer

			status := run(
				append([]string{"one", "two", "--"}, tt.args...),
				func(string) (string, error) {
					t.Fatal("lookup must not run")

					return "", nil
				},
				func(string, []string, io.Reader, io.Writer, io.Writer) error {
					t.Fatal("command must not run")

					return nil
				},
				io.Discard,
				&stderr,
			)
			if status != exitUsageError || !strings.Contains(stderr.String(), tt.format) {
				t.Fatalf("status=%d stderr=%q", status, stderr.String())
			}
		})
	}
}

func TestRunAllowsPrettyOutputAcrossMultipleOverlays(t *testing.T) {
	t.Parallel()

	var calls []invocation

	status := run(
		[]string{"one", "two", "--", "-output", "pretty"},
		lookupOK,
		func(name string, args []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
			calls = append(calls, invocation{name: name, args: append([]string(nil), args...)})
			if name == "/bin/kustomize" {
				_, _ = stdout.Write([]byte(args[1]))
			}

			return nil
		},
		io.Discard,
		io.Discard,
	)
	if status != exitOK {
		t.Fatalf("status = %d", status)
	}

	if len(calls) != 4 || !reflect.DeepEqual(calls[1].args, []string{"-output", "pretty"}) ||
		!reflect.DeepEqual(calls[3].args, []string{"-output", "pretty"}) {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestRunForwardsConsumerSchemaLocationsAcrossMultipleOverlays(t *testing.T) {
	t.Parallel()

	kubeconformArgs := []string{
		"-schema-location",
		"default",
		"-schema-location",
		"https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json",
		"-schema-location",
		"https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/{{.NormalizedKubernetesVersion}}/{{.ResourceKind}}.json",
	}
	var calls []invocation

	status := run(
		append([]string{"apps/a", "apps/b", "--"}, kubeconformArgs...),
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

			calls = append(calls, invocation{name: name, args: append([]string(nil), args...), stdin: input})
			if name == "/bin/kustomize" {
				_, _ = stdout.Write([]byte(args[1]))
			}

			return nil
		},
		io.Discard,
		io.Discard,
	)
	if status != exitOK {
		t.Fatalf("status = %d", status)
	}

	want := []invocation{
		{name: "/bin/kustomize", args: []string{"build", "apps/a"}},
		{name: "/bin/kubeconform", args: kubeconformArgs, stdin: []byte("apps/a")},
		{name: "/bin/kustomize", args: []string{"build", "apps/b"}},
		{name: "/bin/kubeconform", args: kubeconformArgs, stdin: []byte("apps/b")},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRunOneOverlayForwardsDiscreteArgumentsAndBytes(t *testing.T) {
	t.Parallel()

	var calls []invocation

	status := run(
		[]string{"overlays/dev", "--", "-schema-location", "https://example.invalid/two words"},
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

			calls = append(calls, invocation{name: name, args: append([]string(nil), args...), stdin: input})

			if name == "/bin/kustomize" {
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
		{name: "/bin/kustomize", args: []string{"build", "overlays/dev"}},
		{
			name:  "/bin/kubeconform",
			args:  []string{"-schema-location", "https://example.invalid/two words"},
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
			calls = append(calls, invocation{name: name, args: append([]string(nil), args...)})

			return errors.New("build failed")
		},
		io.Discard,
		io.Discard,
	)
	if status != 1 ||
		!reflect.DeepEqual(calls, []invocation{{name: "/bin/kustomize", args: []string{"build", "broken"}}}) {
		t.Fatalf("status=%d calls=%#v", status, calls)
	}
}

func TestRunValidationFailureReturnsOne(t *testing.T) {
	t.Parallel()

	status := run(
		[]string{"valid"},
		lookupOK,
		func(name string, _ []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
			if name == "/bin/kustomize" {
				_, _ = stdout.Write([]byte("apiVersion: v1\n"))

				return nil
			}

			return errors.New("validation failed")
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
		},
		io.Discard,
		io.Discard,
	)
	if status != 1 || len(calls) != 5 {
		t.Fatalf("status=%d calls=%#v", status, calls)
	}

	if calls[4].name != "/bin/kubeconform" || string(calls[4].stdin) != "good" {
		t.Fatalf("did not continue to final overlay: %#v", calls[4])
	}
}

func TestRunMultipleSuccessfulOverlaysReturnsZero(t *testing.T) {
	t.Parallel()

	status := run(
		[]string{"one", "two"},
		lookupOK,
		func(name string, args []string, _ io.Reader, stdout io.Writer, _ io.Writer) error {
			if name == "/bin/kustomize" {
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
					return "", errors.New("missing")
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

func TestRunReportsExecutionErrorsOnStderr(t *testing.T) {
	t.Parallel()

	permErr := fmt.Errorf("running kustomize: %w", errors.New("permission denied"))

	var stderr bytes.Buffer

	status := run(
		[]string{"overlay"},
		lookupOK,
		func(name string, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			if name == "/bin/kustomize" {
				return permErr
			}

			t.Fatal("kubeconform must not run when build failed")

			return nil
		},
		io.Discard,
		&stderr,
	)
	if status != 1 {
		t.Fatalf("status = %d", status)
	}

	if !strings.Contains(stderr.String(), "overlay") ||
		!strings.Contains(stderr.String(), "permission denied") {
		t.Fatalf("stderr = %q, want it to mention the overlay and the underlying error", stderr.String())
	}
}

func TestRunReportsKubeconformExecutionErrorOnStderr(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer

	status := run(
		[]string{"overlay"},
		lookupOK,
		func(name string, _ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
			if name == "/bin/kubeconform" {
				return fmt.Errorf("running kubeconform: %w", errors.New("exec format error"))
			}

			return nil
		},
		io.Discard,
		&stderr,
	)
	if status != 1 {
		t.Fatalf("status = %d", status)
	}

	if !strings.Contains(stderr.String(), "overlay") ||
		!strings.Contains(stderr.String(), "exec format error") {
		t.Fatalf("stderr = %q, want it to mention the overlay and the underlying error", stderr.String())
	}
}

func lookupOK(name string) (string, error) {
	return "/bin/" + name, nil
}
