package vm_test

import (
	"strings"
	"testing"
)

func TestParityRegressionCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
		stdin  string
		want   string
		check  func(t *testing.T, got string)
	}{
		{
			name: "db alias and title-like symbol names",
			source: `
TITLE Demo
INCLUDE Irvine32.inc

.data
titleStr BYTE "title symbol works",0
caption db "db alias works",0

.code
main PROC
    mov edx, OFFSET titleStr
    call WriteString
    call Crlf
    mov edx, OFFSET caption
    call WriteString
    exit
main ENDP

END main
`,
			want: "title symbol works\r\ndb alias works",
		},
		{
			name: "top level version if",
			source: `
INCLUDE Irvine32.inc

IF @Version GT 510
.data
msg BYTE "new path",0
ELSE
.data
msg BYTE "old path",0
ENDIF

.code
main PROC
    mov edx, OFFSET msg
    call WriteString
    exit
main ENDP

END main
`,
			want: "new path",
		},
		{
			name: "real10 loads and stores",
			source: `
INCLUDE Irvine32.inc

.data
bigVal REAL10 1.0123456789012345E+864
copyVal REAL10 ?

.code
main PROC
    finit
    fld bigVal
    fstp copyVal
    fld copyVal
    call ShowFPUStack
    exit
main ENDP

END main
`,
			check: func(t *testing.T, got string) {
				if !strings.Contains(strings.ToUpper(got), "INF") {
					t.Fatalf("expected INF-like output, got %q", got)
				}
			},
		},
		{
			name: "macro default angle literal and loose operand comment",
			source: `
INCLUDE Irvine32.inc

SayLn MACRO text := <" ">
    mWrite text
    call Crlf
ENDM

.data
value DWORD 7

.code
main PROC
    mov eax, 10
    sub eax, value        subtract the sample value
    call WriteInt
    call Crlf
    SayLn
    SayLn "done"
    exit
main ENDP

END main
`,
			want: "3\r\n \r\ndone\r\n",
		},
		{
			name: "mixed case ampersand macro substitution",
			source: `
INCLUDE Irvine32.inc

ShowRegister MACRO regName
.data
tempStr BYTE "  &regName=",0
.code
    push eax
    push edx
    mov edx, OFFSET tempStr
    call WriteString
    pop edx
    mov eax, regName
    call WriteHex
    pop eax
ENDM

.code
main PROC
    mov ecx, 0
    ShowRegister ECX
    exit
main ENDP

END main
`,
			want: "  ECX=00000000",
		},
		{
			name: "invoke sleep shim",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
    INVOKE Sleep, 1
    mWrite "awake"
    exit
main ENDP

END main
`,
			want: "awake",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runSource(t, tt.source, tt.stdin)
			if tt.check != nil {
				tt.check(t, got)
				return
			}
			if got != tt.want {
				t.Fatalf("unexpected output %q", got)
			}
		})
	}
}
