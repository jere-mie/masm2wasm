package vm_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"masminterpreter/internal/masm"
	"masminterpreter/vm"
)

func TestHelloWorldProgram(t *testing.T) {
	source := `
TITLE Hello World
INCLUDE Irvine32.inc
INCLUDELIB Irvine32.lib

.data
helloMsg BYTE "Hello, World!", 0

.code
main PROC
    mov edx, OFFSET helloMsg
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
	var stdout bytes.Buffer
	machine := vm.NewMachine(strings.NewReader(""), &stdout, &stdout)
	code, err := machine.Run(program)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if got := stdout.String(); got != "Hello, World!\r\n" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestFactorialProgram(t *testing.T) {
	source := `
TITLE Factorial Calculator

INCLUDE Irvine32.inc
INCLUDELIB Irvine32.lib

.data
prompt BYTE "Enter a number: ", 0
resultMsg BYTE "Factorial = ", 0
errorMsg BYTE "Invalid input. Please enter a positive integer.", 0
num DWORD ?
result DWORD ?

.code
main PROC
    mov edx, OFFSET prompt
    call WriteString
    call ReadInt
    mov num, eax

    mov eax, num
    mov ecx, eax
    dec ecx
L1:
    mul ecx
    loop L1
    mov result, eax

    cmp num, 0
    jle error
    mov edx, OFFSET resultMsg
    call WriteString
    call WriteDec
    jmp done
error:
    mov edx, OFFSET errorMsg
    call WriteString
    call Crlf
done:
    exit
main ENDP
END main
`
	program, err := masm.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	var stdout bytes.Buffer
	machine := vm.NewMachine(strings.NewReader("5\n"), &stdout, &stdout)
	code, err := machine.Run(program)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if got := stdout.String(); got != "Enter a number: Factorial = 120" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestStringMacrosInvokeAndIf(t *testing.T) {
	source := `
TITLE String Features
INCLUDE Irvine32.inc
INCLUDE Macros.inc

.data
text  BYTE "abc///",0
other BYTE "ABC",0

.code
main PROC
    INVOKE Str_trim, ADDR text,'/'
    INVOKE Str_ucase, ADDR text
    INVOKE Str_compare, ADDR text, ADDR other
    .IF ZERO?
        mWrite "match"
    .ELSE
        mWrite "mismatch"
    .ENDIF
    call Crlf
    exit
main ENDP
END main
`
	program, err := masm.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	var stdout bytes.Buffer
	machine := vm.NewMachine(strings.NewReader(""), &stdout, &stdout)
	code, err := machine.Run(program)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if got := stdout.String(); got != "match\r\n" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestFileIOProgram(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "io-test.txt")
	source := fmt.Sprintf(`
TITLE File I/O
INCLUDE Irvine32.inc

.data
filename BYTE "%s",0
payload  BYTE "hello wasm",0
buffer   BYTE 32 DUP(0)
fileHandle DWORD ?

.code
main PROC
    mov edx, OFFSET filename
    call CreateOutputFile
    mov fileHandle, eax
    mov eax, fileHandle
    mov edx, OFFSET payload
    mov ecx, 10
    call WriteToFile
    mov eax, fileHandle
    call CloseFile

    mov edx, OFFSET filename
    call OpenInputFile
    mov fileHandle, eax
    mov eax, fileHandle
    mov edx, OFFSET buffer
    mov ecx, SIZEOF buffer
    call ReadFromFile
    mov buffer[eax], 0
    mov eax, fileHandle
    call CloseFile

    mov edx, OFFSET buffer
    call WriteString
    exit
main ENDP
END main
`, path)
	program, err := masm.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	var stdout bytes.Buffer
	machine := vm.NewMachine(strings.NewReader(""), &stdout, &stdout)
	code, err := machine.Run(program)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if got := stdout.String(); got != "hello wasm" {
		t.Fatalf("unexpected output %q", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading output file failed: %v", err)
	}
	if string(data) != "hello wasm" {
		t.Fatalf("unexpected file contents %q", string(data))
	}
}
