package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"masminterpreter/internal/generator"
	"masminterpreter/internal/masm"
	"masminterpreter/internal/runner"
	"masminterpreter/internal/version"
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
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		writeMainUsage(os.Stdout)
		return 0
	}

	switch args[0] {
	case "build":
		return runBuild(args[1:])
	case "run":
		return runRun(args[1:])
	case "version":
		fmt.Println(version.Summary())
		return 0
	case "help":
		if len(args) > 1 {
			switch args[1] {
			case "build":
				writeBuildUsage(os.Stdout)
				return 0
			case "run":
				writeRunUsage(os.Stdout)
				return 0
			case "version":
				fmt.Println(version.Summary())
				return 0
			}
		}
		writeMainUsage(os.Stdout)
		return 0
	case "-h", "--help":
		writeMainUsage(os.Stdout)
		return 0
	case "-v", "--version":
		fmt.Println(version.Summary())
		return 0
	default:
		return runBuild(args)
	}
}

func runBuild(args []string) int {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	input := flags.String("input", "", "Path to the MASM source file, or - for stdin")
	flags.StringVar(input, "i", "", "Path to the MASM source file, or - for stdin")
	output := flags.String("output", "", "Path to the output WASM file")
	flags.StringVar(output, "o", "", "Path to the output WASM file")
	flags.Usage = func() { writeBuildUsage(os.Stderr) }

	if err := flags.Parse(reorderFlagArgs(args, map[string]bool{
		"-input":  true,
		"-i":      true,
		"-output": true,
		"-o":      true,
	})); err != nil {
		return 2
	}
	if *input == "" && flags.NArg() > 0 {
		*input = flags.Arg(0)
	}
	if *input == "" {
		fmt.Fprintln(os.Stderr, "missing input path")
		writeBuildUsage(os.Stderr)
		return 2
	}

	moduleBytes, err := buildModuleFromInput(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if *output == "" {
		*output = defaultOutputPath(*input)
	}
	if err := writeModule(*output, moduleBytes); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *output != "-" {
		fmt.Println(*output)
	}
	return 0
}

func runRun(args []string) int {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	input := flags.String("input", "", "Path to a .wasm module or .asm source file")
	flags.StringVar(input, "i", "", "Path to a .wasm module or .asm source file")
	output := flags.String("output", "", "Optional path to save the generated WASM when running an .asm file")
	flags.StringVar(output, "o", "", "Optional path to save the generated WASM when running an .asm file")
	stdinText := flags.String("stdin", "", "Optional stdin text to feed into the module")
	stdinFile := flags.String("stdin-file", "", "Optional file to feed to stdin")
	stdoutFile := flags.String("stdout-file", "", "Optional file to capture stdout")
	var wasiArgs stringList
	flags.Var(&wasiArgs, "arg", "Argument to pass to the WASI module (repeatable)")
	flags.Usage = func() { writeRunUsage(os.Stderr) }

	if err := flags.Parse(reorderFlagArgs(args, map[string]bool{
		"-input":       true,
		"-i":           true,
		"-output":      true,
		"-o":           true,
		"-stdin":       true,
		"-stdin-file":  true,
		"-stdout-file": true,
		"-arg":         true,
	})); err != nil {
		return 2
	}
	if *input == "" && flags.NArg() > 0 {
		*input = flags.Arg(0)
	}
	if *input == "" {
		fmt.Fprintln(os.Stderr, "missing input path")
		writeRunUsage(os.Stderr)
		return 2
	}

	remainingArgs := flags.Args()
	if len(remainingArgs) > 0 && remainingArgs[0] == *input {
		remainingArgs = remainingArgs[1:]
	}
	moduleArgs := append([]string(nil), wasiArgs...)
	moduleArgs = append(moduleArgs, remainingArgs...)

	runOptions, cleanup, err := makeRunOptions(*stdinText, *stdinFile, *stdoutFile, moduleArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer cleanup()

	if strings.EqualFold(filepath.Ext(*input), ".asm") {
		moduleBytes, err := buildModuleFromInput(*input)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if *output != "" {
			if err := writeModule(*output, moduleBytes); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}
		if err := runner.RunModuleBytes(moduleBytes, runOptions); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if *output != "" {
		fmt.Fprintln(os.Stderr, "-output is only supported when running an .asm file")
		return 2
	}
	if err := runner.RunModuleFile(*input, runOptions); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func buildModuleFromInput(input string) ([]byte, error) {
	source, err := readInput(input)
	if err != nil {
		return nil, err
	}
	program, err := masm.Parse(string(source))
	if err != nil {
		return nil, err
	}
	return generator.BuildProgramBytes(program)
}

func readInput(input string) ([]byte, error) {
	if input == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(input)
}

func writeModule(output string, moduleBytes []byte) error {
	if output == "" || output == "-" {
		_, err := os.Stdout.Write(moduleBytes)
		return err
	}
	return os.WriteFile(output, moduleBytes, 0o644)
}

func defaultOutputPath(input string) string {
	if input == "-" {
		return "-"
	}
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	return filepath.Join(filepath.Dir(input), base+".wasm")
}

func makeRunOptions(stdinText, stdinFile, stdoutFile string, args []string) (runner.Options, func(), error) {
	options := runner.Options{
		Args:   args,
		Stderr: os.Stderr,
	}
	cleanup := func() {}

	if stdinFile != "" {
		file, err := os.Open(stdinFile)
		if err != nil {
			return runner.Options{}, cleanup, err
		}
		options.Stdin = file
		cleanup = combineCleanup(cleanup, func() { _ = file.Close() })
	} else if stdinText != "" {
		options.Stdin = bytes.NewBufferString(stdinText)
	}

	if stdoutFile != "" {
		file, err := os.Create(stdoutFile)
		if err != nil {
			cleanup()
			return runner.Options{}, func() {}, err
		}
		options.Stdout = file
		cleanup = combineCleanup(cleanup, func() { _ = file.Close() })
	}

	return options, cleanup, nil
}

func combineCleanup(a, b func()) func() {
	return func() {
		if b != nil {
			b()
		}
		if a != nil {
			a()
		}
	}
}

func reorderFlagArgs(args []string, valueFlags map[string]bool) []string {
	separator := -1
	for i, arg := range args {
		if arg == "--" {
			separator = i
			break
		}
	}
	head := args
	tail := []string{}
	if separator >= 0 {
		head = args[:separator]
		tail = args[separator+1:]
	}

	flags := make([]string, 0, len(head))
	positionals := make([]string, 0, len(head))
	for i := 0; i < len(head); i++ {
		token := head[i]
		if strings.HasPrefix(token, "-") && token != "-" {
			flags = append(flags, token)
			flagName := token
			if idx := strings.Index(flagName, "="); idx >= 0 {
				flagName = flagName[:idx]
			}
			if strings.Contains(token, "=") || !valueFlags[flagName] {
				continue
			}
			if i+1 < len(head) {
				flags = append(flags, head[i+1])
				i++
			}
			continue
		}
		positionals = append(positionals, token)
	}

	reordered := append(flags, positionals...)
	if separator >= 0 {
		reordered = append(reordered, "--")
		reordered = append(reordered, tail...)
	}
	return reordered
}

func writeMainUsage(w io.Writer) {
	fmt.Fprintf(w, `%s

Usage:
  masm2wasm build [options] <source.asm>
  masm2wasm run [options] <program.wasm|source.asm> [-- module args...]
  masm2wasm version

Commands:
  build     Translate MASM source into a WASI WebAssembly program.
  run       Run a generated WASI module, or build-and-run a MASM source file.
  version   Show the CLI version.

Compatibility note:
  Running "masm2wasm <source.asm>" without a subcommand still behaves like "masm2wasm build <source.asm>".

Use "masm2wasm help build" or "masm2wasm help run" for command-specific options.
`, version.Summary())
}

func writeBuildUsage(w io.Writer) {
	fmt.Fprint(w, `
Usage:
  masm2wasm build [options] <source.asm>
  masm2wasm <source.asm>

Options:
  -input, -i    Path to the MASM source file, or - for stdin
  -output, -o   Path to the output WASM file (defaults to <input>.wasm)
`)
}

func writeRunUsage(w io.Writer) {
	fmt.Fprint(w, `
Usage:
  masm2wasm run [options] <program.wasm|source.asm> [module args...]

Options:
  -input, -i       Path to a .wasm module or .asm source file
  -output, -o      Optional path to save generated WASM when running an .asm file
  -stdin           Optional stdin text to feed into the module
  -stdin-file      Optional file to feed to stdin
  -stdout-file     Optional file to capture stdout
  -arg             Argument to pass to the WASI module (repeatable)

If the input is a .asm file, masm2wasm builds it in memory and runs it immediately.
`)
}
