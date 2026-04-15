package vm_test

import (
	"bytes"
	"strings"
	"testing"

	"masminterpreter/internal/masm"
	"masminterpreter/vm"
)

func runSource(t *testing.T, source, stdin string) string {
	t.Helper()
	program, err := masm.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	var stdout bytes.Buffer
	machine := vm.NewMachine(strings.NewReader(stdin), &stdout, &stdout)
	code, err := machine.Run(program)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}
	return stdout.String()
}

func TestProcedureCompatibilityCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "proto invoke uses local c",
			source: `
INCLUDE Irvine32.inc

SumTwo PROTO C, left:DWORD, right:DWORD

.code
main PROC
    mov ebx, 99
    INVOKE SumTwo, 7, 5
    call WriteDec
    call Crlf
    mov eax, ebx
    call WriteDec
    exit
main ENDP

SumTwo PROC C USES ebx, left:DWORD, right:DWORD
    LOCAL temp:DWORD
    mov ebx, left
    add ebx, right
    mov temp, ebx
    mov eax, temp
    ret
SumTwo ENDP

END main
`,
			want: "12\r\n99",
		},
		{
			name: "stdcall invoke repeated calls",
			source: `
INCLUDE Irvine32.inc

EchoStd PROTO STDCALL, value:DWORD

.code
main PROC
    INVOKE EchoStd, 11
    call WriteDec
    call Crlf
    INVOKE EchoStd, 22
    call WriteDec
    exit
main ENDP

EchoStd PROC STDCALL, value:DWORD
    mov eax, value
    ret
EchoStd ENDP

END main
`,
			want: "11\r\n22",
		},
		{
			name: "pascal invoke order",
			source: `
INCLUDE Irvine32.inc

ShowPair PROTO PASCAL, first:DWORD, second:DWORD

.code
main PROC
    INVOKE ShowPair, 4, 9
    call WriteDec
    exit
main ENDP

ShowPair PROC PASCAL, first:DWORD, second:DWORD
    mov eax, first
    call WriteDec
    call Crlf
    mov eax, second
    ret
ShowPair ENDP

END main
`,
			want: "4\r\n9",
		},
		{
			name: "explicit ret cleanup",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
    push 33
    call EchoRet
    call WriteDec
    call Crlf
    push 44
    call EchoRet
    call WriteDec
    exit
main ENDP

EchoRet PROC value:DWORD
    mov eax, value
    ret 4
EchoRet ENDP

END main
`,
			want: "33\r\n44",
		},
		{
			name: "ptr word type and dereference",
			source: `
INCLUDE Irvine32.inc

ZeroWord PROTO dataPtr:PTR WORD

.data
myData WORD 1234h

.code
main PROC
    INVOKE ZeroWord, ADDR myData
    mov eax, TYPE WORD
    call WriteDec
    call Crlf
    mov eax, 0
    mov ax, myData
    call WriteHex
    exit
main ENDP

ZeroWord PROC dataPtr:PTR WORD
    mov esi, dataPtr
    mov WORD PTR[esi], 0
    ret
ZeroWord ENDP

END main
`,
			want: "2\r\n00000000",
		},
		{
			name: "lea stack parameter",
			source: `
INCLUDE Irvine32.inc

ReadBack PROTO value:DWORD

.code
main PROC
    INVOKE ReadBack, 123
    call WriteDec
    exit
main ENDP

ReadBack PROC value:DWORD
    lea esi, value
    mov eax, [esi]
    ret
ReadBack ENDP

END main
`,
			want: "123",
		},
		{
			name: "pushad popad restore registers",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
    mov eax, 1
    mov ebx, 2
    mov ecx, 3
    pushad
    mov eax, 10
    mov ebx, 20
    mov ecx, 30
    popad
    call WriteDec
    call Crlf
    mov eax, ebx
    call WriteDec
    call Crlf
    mov eax, ecx
    call WriteDec
    exit
main ENDP

END main
`,
			want: "1\r\n2\r\n3",
		},
		{
			name: "pushfd popfd restore flags",
			source: `
INCLUDE Irvine32.inc
INCLUDE Macros.inc

.code
main PROC
    mov eax, 1
    cmp eax, 2
    pushfd
    xor eax, eax
    cmp eax, 0
    popfd
    je wrong
    mWrite "ok"
    exit
wrong:
    mWrite "bad"
    exit
main ENDP

END main
`,
			want: "ok",
		},
		{
			name: "leave unwinds manual frame",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
    push ebp
    mov ebp, esp
    sub esp, 4
    mov DWORD PTR [ebp-4], 77
    mov eax, DWORD PTR [ebp-4]
    leave
    call WriteDec
    exit
main ENDP

END main
`,
			want: "77",
		},
		{
			name: "uses with multiple locals and ptr dword",
			source: `
INCLUDE Irvine32.inc

ArraySum PROTO, ptrArray:PTR DWORD, szArray:DWORD

.data
items DWORD 1,2,3,4

.code
main PROC
    mov ebx, 7
    mov esi, 8
    INVOKE ArraySum, ADDR items, LENGTHOF items
    call WriteDec
    call Crlf
    mov eax, ebx
    call WriteDec
    call Crlf
    mov eax, esi
    call WriteDec
    exit
main ENDP

ArraySum PROC USES ebx esi ecx, ptrArray:PTR DWORD, szArray:DWORD
    LOCAL running:DWORD, savedCount:DWORD
    mov esi, ptrArray
    mov ecx, szArray
    mov savedCount, ecx
    mov running, 0
sum_loop:
    mov ebx, [esi]
    add running, ebx
    add esi, 4
    loop sum_loop
    mov eax, running
    ret
ArraySum ENDP

END main
`,
			want: "10\r\n7\r\n8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runSource(t, tt.source, ""); got != tt.want {
				t.Fatalf("unexpected output %q", got)
			}
		})
	}
}
