package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type lookupFunc func(string) (string, error)

type runFunc func(string, []string, io.Reader, io.Writer, io.Writer) error

func main() {
	os.Exit(run(os.Args[1:], exec.LookPath, runCommand, os.Stdout, os.Stderr))
}

func runCommand(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	command := exec.Command(name, args...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func run(args []string, lookup lookupFunc, command runFunc, stdout, stderr io.Writer) int {
	overlays, kubeconformArgs := splitArgs(args)
	if len(overlays) == 0 {
		fmt.Fprintln(stderr, "usage: kubeconform-kustomize <overlay> [<overlay> ...] -- [<kubeconform args> ...]")
		return 2
	}

	kustomize, kustomizeErr := lookup("kustomize")
	kubeconform, kubeconformErr := lookup("kubeconform")
	if kustomizeErr != nil {
		fmt.Fprintln(stderr, "hook environment is incomplete: executable kustomize was not found")
		return 1
	}
	if kubeconformErr != nil {
		fmt.Fprintln(stderr, "hook environment is incomplete: executable kubeconform was not found")
		return 1
	}

	failed := false
	for _, overlay := range overlays {
		var rendered bytes.Buffer
		if err := command(kustomize, []string{"build", overlay}, nil, &rendered, stderr); err != nil {
			failed = true
			continue
		}
		if err := command(kubeconform, kubeconformArgs, bytes.NewReader(rendered.Bytes()), stdout, stderr); err != nil {
			failed = true
		}
	}
	if failed {
		return 1
	}
	return 0
}

func splitArgs(args []string) ([]string, []string) {
	for index, arg := range args {
		if arg == "--" {
			return args[:index], args[index+1:]
		}
	}
	return args, nil
}
