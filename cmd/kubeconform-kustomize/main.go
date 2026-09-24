// Package main implements kubeconform-kustomize, a small wrapper that
// renders one or more kustomize overlays and validates the rendered
// manifests with kubeconform, aggregating failures across overlays.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/yannh/kubeconform/pkg/config"
)

// Exit codes documenting the CLI contract.
const (
	exitOK         = 0
	exitFailure    = 1
	exitUsageError = 2
)

type lookupFunc func(string) (string, error)

type runFunc func(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error

func main() {
	os.Exit(run(os.Args[1:], exec.LookPath, runCommand, os.Stdout, os.Stderr))
}

// runCommand executes name with args, wiring stdin/stdout/stderr, and wraps
// any execution error with the command name for easier debugging.
func runCommand(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	//nolint:gosec // G204: name is kustomize or kubeconform from exec.LookPath; args are passed without a shell.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %s: %w", name, err)
	}

	return nil
}

func run(args []string, lookup lookupFunc, command runFunc, stdout, stderr io.Writer) int {
	overlays, kubeconformArgs := splitArgs(args)
	if len(overlays) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: kubeconform-kustomize <overlay> [<overlay> ...] -- [<kubeconform args> ...]")

		return exitUsageError
	}

	cfg, _, err := config.FromFlags("kubeconform", kubeconformArgs)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "kubeconform-kustomize: invalid kubeconform arguments: %s\n", err)

		return exitUsageError
	}

	if cfg.Help || cfg.Version {
		_, _ = fmt.Fprintln(stderr, "kubeconform-kustomize: kubeconform help and version modes do not validate rendered overlays")

		return exitUsageError
	}

	if len(cfg.Files) > 0 {
		_, _ = fmt.Fprintf(
			stderr,
			"kubeconform-kustomize: positional kubeconform inputs are not allowed after \"--\" (got: %s); rendered overlays are always validated via stdin\n",
			strings.Join(cfg.Files, ", "),
		)

		return exitUsageError
	}

	kustomize, kustomizeErr := lookup("kustomize")
	kubeconform, kubeconformErr := lookup("kubeconform")

	if kustomizeErr != nil {
		_, _ = fmt.Fprintln(stderr, "hook environment is incomplete: executable kustomize was not found")

		return exitFailure
	}

	if kubeconformErr != nil {
		_, _ = fmt.Fprintln(stderr, "hook environment is incomplete: executable kubeconform was not found")

		return exitFailure
	}

	failed := false

	for _, overlay := range overlays {
		var rendered bytes.Buffer

		if err := command(kustomize, []string{"build", overlay}, nil, &rendered, stderr); err != nil {
			_, _ = fmt.Fprintf(stderr, "kubeconform-kustomize: %s: kustomize build failed: %s\n", overlay, err)

			failed = true

			continue
		}

		if err := command(kubeconform, kubeconformArgs, &rendered, stdout, stderr); err != nil {
			_, _ = fmt.Fprintf(stderr, "kubeconform-kustomize: %s: kubeconform validation failed: %s\n", overlay, err)

			failed = true
		}
	}

	if failed {
		return exitFailure
	}

	return exitOK
}

func splitArgs(args []string) ([]string, []string) {
	for index, arg := range args {
		if arg == "--" {
			return args[:index], args[index+1:]
		}
	}

	return args, nil
}
