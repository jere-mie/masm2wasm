package vm_test

import "testing"

func TestAdditionalFloatAndParserCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "comment block ignored",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
COMMENT @
This is a block comment.
It should be skipped entirely.
@
    mWrite "ok"
    exit
main ENDP

END main
`,
			want: "ok",
		},
		{
			name: "comment block terminator can end a text line",
			source: `
INCLUDE Irvine32.inc

COMMENT @
copied definitions go here @

.code
main PROC
    mWrite "ok"
    exit
main ENDP

END main
`,
			want: "ok",
		},
		{
			name: "jng alias works",
			source: `
INCLUDE Irvine32.inc

.data
a DWORD 5
b DWORD 5

.code
main PROC
    mov eax, a
    cmp eax, b
    jng less_or_equal
    mWrite "bad"
    exit
less_or_equal:
    mWrite "ok"
    exit
main ENDP

END main
`,
			want: "ok",
		},
		{
			name: "negated zero flag condition works",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
    mov eax, 1
    test eax, eax
    .IF !Zero?
        mWrite "ok"
    .ELSE
        mWrite "bad"
    .ENDIF
    exit
main ENDP

END main
`,
			want: "ok",
		},
		{
			name: "fld1 and fdivr",
			source: `
INCLUDE Irvine32.inc

.data
four REAL8 4.0

.code
main PROC
    finit
    fld four
    fld1
    fdivr
    call WriteFloat
    exit
main ENDP

END main
`,
			want: "2.500000000000E-01",
		},
		{
			name: "frndint with truncate mode",
			source: `
INCLUDE Irvine32.inc

.data
value REAL8 3.75
ctrlWord WORD ?

.code
main PROC
    finit
    fstcw ctrlWord
    or ctrlWord, 110000000000b
    fldcw ctrlWord
    fld value
    frndint
    call WriteFloat
    exit
main ENDP

END main
`,
			want: "3.000000000000E+00",
		},
		{
			name: "fstsw alias with sahf",
			source: `
INCLUDE Irvine32.inc

.data
x REAL8 1.1
y REAL8 1.2
n DWORD 0

.code
main PROC
    finit
    fld x
    fcomp y
    fstsw ax
    sahf
    jb below
    mov n, 0
    jmp done
below:
    mov n, 1
done:
    mov eax, n
    call WriteDec
    exit
main ENDP

END main
`,
			want: "1",
		},
		{
			name: "fincstp rotates stack view",
			source: `
INCLUDE Irvine32.inc

.data
a REAL8 1.0
b REAL8 2.0
c REAL8 3.0

.code
main PROC
    finit
    fld a
    fld b
    fld c
    fincstp
    call ShowFPUStack
    exit
main ENDP

END main
`,
			want: "\r\n------ FPU Stack ------\r\nST(0): 2.000000000000E+00\r\nST(1): 1.000000000000E+00\r\nST(2): 3.000000000000E+00\r\n",
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
