package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"masminterpreter/internal/runner"
)

type stringList []string

func (s *stringList) String() string {
	return fmt.Sprint([]string(*s))
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	input := flag.String("input", "", "WASM module to execute")
	stdinText := flag.String("stdin", "", "Optional stdin text to feed into the module")
	stdinFile := flag.String("stdin-file", "", "Optional file to feed to stdin")
	stdoutFile := flag.String("stdout-file", "", "Optional file to capture stdout")
	var args stringList
	flag.Var(&args, "arg", "Argument to pass to the WASI module (repeatable)")
	flag.Parse()

	if *input == "" && flag.NArg() > 0 {
		*input = flag.Arg(0)
	}
	if *input == "" {
		fmt.Fprintln(os.Stderr, "missing -input")
		os.Exit(2)
	}

	options := runner.Options{
		Args:   args,
		Stderr: os.Stderr,
	}

	cleanup := func() {}
	if *stdinFile != "" {
		file, err := os.Open(*stdinFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		options.Stdin = file
		cleanup = func() { _ = file.Close() }
	} else if *stdinText != "" {
		options.Stdin = bytes.NewBufferString(*stdinText)
	}

	if *stdoutFile != "" {
		file, err := os.Create(*stdoutFile)
		if err != nil {
			cleanup()
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		options.Stdout = file
		previousCleanup := cleanup
		cleanup = func() {
			_ = file.Close()
			previousCleanup()
		}
	}
	defer cleanup()

	if err := runner.RunModuleFile(*input, options); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
