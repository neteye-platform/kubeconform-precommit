// Package main implements kubeconform-kustomize, a small wrapper that
// renders one or more kustomize overlays and validates the rendered
// manifests with kubeconform, aggregating failures across overlays.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

const (
	exitOK                   = 0
	exitFailure              = 1
	exitUsageError           = 2
	kustomizeName            = "kustomize"
	kubeconformName          = "kubeconform"
	kustomizeBuildSubcommand = "build"
)

type lookupFunc func(string) (string, error)

type runFunc func(string, []string, io.Reader, io.Writer, io.Writer) error

func main() {
	os.Exit(run(os.Args[1:], exec.LookPath, runCommand, os.Stdout, os.Stderr))
}

// runCommand executes name with args, wiring stdin/stdout/stderr, and wraps
// any execution error with the command name for easier debugging.
func runCommand(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	//nolint:gosec // name/args originate from a fixed internal set (kustomize/kubeconform
	// looked up via exec.LookPath) and CLI args passed through verbatim by design; this is
	// the intended purpose of this wrapper, not attacker-controlled input.
	command := exec.CommandContext(context.Background(), name, args...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr

	err := command.Run()
	if err != nil {
		return fmt.Errorf("running %s: %w", name, err)
	}

	return nil
}

// printLine writes msg to w and reports exitFailure if the write itself
// fails, so callers can propagate a sane exit code even when stderr is
// broken.
func printLine(w io.Writer, msg string) (int, bool) {
	_, err := fmt.Fprintln(w, msg)
	if err != nil {
		return exitFailure, false
	}

	return exitOK, true
}

// lookupExecutables resolves the kustomize and kubeconform executables,
// reporting a descriptive error to stderr on failure.
func lookupExecutables(lookup lookupFunc, stderr io.Writer) (string, string, int, bool) {
	kustomize, kustomizeErr := lookup(kustomizeName)
	kubeconform, kubeconformErr := lookup(kubeconformName)

	if kustomizeErr != nil {
		status, ok := printLine(
			stderr,
			"hook environment is incomplete: executable kustomize was not found",
		)
		if !ok {
			return "", "", status, false
		}

		return "", "", exitFailure, false
	}

	if kubeconformErr != nil {
		status, ok := printLine(
			stderr,
			"hook environment is incomplete: executable kubeconform was not found",
		)
		if !ok {
			return "", "", status, false
		}

		return "", "", exitFailure, false
	}

	return kustomize, kubeconform, exitOK, true
}

func run(args []string, lookup lookupFunc, command runFunc, stdout, stderr io.Writer) int {
	overlays, kubeconformArgs := splitArgs(args)
	if len(overlays) == 0 {
		status, ok := printLine(
			stderr,
			"usage: kubeconform-kustomize <overlay> [<overlay> ...] -- [<kubeconform args> ...]",
		)
		if !ok {
			return status
		}

		return exitUsageError
	}

	kustomize, kubeconform, status, ok := lookupExecutables(lookup, stderr)
	if !ok {
		return status
	}

	failed := false

	for _, overlay := range overlays {
		if !validateOverlay(
			overlay,
			kustomize,
			kubeconform,
			kubeconformArgs,
			command,
			stdout,
			stderr,
		) {
			failed = true
		}
	}

	if failed {
		return exitFailure
	}

	return exitOK
}

// validateOverlay renders a single overlay via kustomize and pipes the
// result into kubeconform, returning false if either step fails.
func validateOverlay(
	overlay, kustomize, kubeconform string,
	kubeconformArgs []string,
	command runFunc,
	stdout, stderr io.Writer,
) bool {
	var rendered bytes.Buffer

	err := command(
		kustomize,
		[]string{kustomizeBuildSubcommand, overlay},
		nil,
		&rendered,
		stderr,
	)
	if err != nil {
		return false
	}

	err = command(
		kubeconform,
		kubeconformArgs,
		bytes.NewReader(rendered.Bytes()),
		stdout,
		stderr,
	)

	return err == nil
}

func splitArgs(args []string) ([]string, []string) {
	for index, arg := range args {
		if arg == "--" {
			return args[:index], args[index+1:]
		}
	}

	return args, nil
}
