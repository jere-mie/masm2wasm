package runner

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Options struct {
	Args       []string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	WorkingDir string
}

func RunModuleFile(path string, options Options) error {
	moduleBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return RunModuleBytes(moduleBytes, options)
}

func RunModuleBytes(moduleBytes []byte, options Options) error {
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return err
	}

	stdin := options.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := options.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := options.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	workingDir := options.WorkingDir
	if workingDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("determine working directory: %w", err)
		}
		workingDir = cwd
	}

	config := wazero.NewModuleConfig().
		WithStdin(stdin).
		WithStdout(stdout).
		WithStderr(stderr).
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep().
		WithFSConfig(wazero.NewFSConfig().WithDirMount(workingDir, "/"))

	if len(options.Args) > 0 {
		config = config.WithArgs(options.Args...)
	}

	_, err := runtime.InstantiateWithConfig(ctx, moduleBytes, config)
	return err
}
