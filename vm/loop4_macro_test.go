package vm_test

import (
	"strings"
	"testing"

	"masminterpreter/internal/masm"
)

func TestMacroExpansionCases(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		want    string
		wantErr string
	}{
		{
			name: "simple macro call",
			source: `
INCLUDE Irvine32.inc

mPutchar MACRO value
    mov al, value
    call WriteChar
ENDM

.code
main PROC
    mPutchar 'A'
    exit
main ENDP

END main
`,
			want: "A",
		},
		{
			name: "nested macro expansion",
			source: `
INCLUDE Irvine32.inc

mPutchar MACRO value
    mov al, value
    call WriteChar
ENDM

mPair MACRO first, second
    mPutchar first
    mPutchar second
ENDM

.code
main PROC
    mPair 'A', 'B'
    exit
main ENDP

END main
`,
			want: "AB",
		},
		{
			name: "default macro argument",
			source: `
INCLUDE Irvine32.inc
INCLUDE Macros.inc

mSay MACRO text := <"default">
    mWrite text
ENDM

.code
main PROC
    mSay
    exit
main ENDP

END main
`,
			want: "default",
		},
		{
			name: "required macro argument error",
			source: `
INCLUDE Irvine32.inc

mNeed MACRO value:REQ
    mov eax, value
ENDM

.code
main PROC
    mNeed
    exit
main ENDP

END main
`,
			wantErr: `requires argument "value"`,
		},
		{
			name: "local labels unique per invocation",
			source: `
INCLUDE Irvine32.inc

mEmitA MACRO value
LOCAL done
    mov eax, value
    cmp eax, 0
    je done
    mov al, 'A'
    call WriteChar
done:
ENDM

.code
main PROC
    mEmitA 1
    mEmitA 1
    exit
main ENDP

END main
`,
			want: "AA",
		},
		{
			name: "local labels skip branch",
			source: `
INCLUDE Irvine32.inc

mEmitZ MACRO value
LOCAL skip
    mov eax, value
    cmp eax, 0
    je skip
    mov al, 'Z'
    call WriteChar
skip:
ENDM

.code
main PROC
    mEmitZ 0
    mEmitZ 1
    exit
main ENDP

END main
`,
			want: "Z",
		},
		{
			name: "expression argument expansion",
			source: `
INCLUDE Irvine32.inc

mShow MACRO value
    mov eax, value
    call WriteDec
ENDM

.code
main PROC
    mShow 2 + 3 * 4
    exit
main ENDP

END main
`,
			want: "14",
		},
		{
			name: "identifier argument expansion",
			source: `
INCLUDE Irvine32.inc

mShow MACRO value
    mov eax, value
    call WriteDec
ENDM

.data
num DWORD 7

.code
main PROC
    mShow num
    exit
main ENDP

END main
`,
			want: "7",
		},
		{
			name: "register argument in loop",
			source: `
INCLUDE Irvine32.inc

mEmit MACRO regValue
    mov al, regValue
    call WriteChar
ENDM

.code
main PROC
    mov al, 'A'
    mov ecx, 3
L1:
    mEmit al
    inc al
    loop L1
    exit
main ENDP

END main
`,
			want: "ABC",
		},
		{
			name: "ampersand substitution and default suffix",
			source: `
INCLUDE Irvine32.inc
INCLUDE Macros.inc

mWriteHex MACRO value, suffix := <"h">
    mov eax, value
    call WriteHex
    mWrite suffix
ENDM

mPutAmp MACRO ch
    mov al, &ch
    call WriteChar
ENDM

.code
main PROC
    mWriteHex 0ABCDh
    mPutAmp '!'
    exit
main ENDP

END main
`,
			want: "0000ABCDh!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr != "" {
				_, err := masm.Parse(tt.source)
				if err == nil {
					t.Fatalf("expected parse error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("unexpected parse error %q", err)
				}
				return
			}
			if got := runSource(t, tt.source, ""); got != tt.want {
				t.Fatalf("unexpected output %q", got)
			}
		})
	}
}
