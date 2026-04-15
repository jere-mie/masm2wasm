package generator_test

import (
	"bytes"
	"context"
	"testing"

	"masminterpreter/internal/generator"
	"masminterpreter/internal/masm"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func TestBuildProgramBytesProducesRunnableModule(t *testing.T) {
	source := `
TITLE Hello
INCLUDE Irvine32.inc

.data
msg BYTE "Hello from generated wasm",0

.code
main PROC
    mov edx, OFFSET msg
    call WriteString
    call Crlf
    exit
main ENDP
END main
`
	program, err := masm.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	moduleBytes, err := generator.BuildProgramBytes(program)
	if err != nil {
		t.Fatalf("build bytes failed: %v", err)
	}
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		t.Fatalf("instantiate wasi failed: %v", err)
	}
	var stdout bytes.Buffer
	config := wazero.NewModuleConfig().
		WithStdout(&stdout).
		WithStderr(&stdout).
		WithFSConfig(wazero.NewFSConfig().WithDirMount(".", "/"))
	if _, err := runtime.InstantiateWithConfig(ctx, moduleBytes, config); err != nil {
		t.Fatalf("instantiate module failed: %v", err)
	}
	if got := stdout.String(); got != "Hello from generated wasm\r\n" {
		t.Fatalf("unexpected stdout %q", got)
	}
}
