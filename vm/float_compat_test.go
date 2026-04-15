package vm_test

import "testing"

func TestFloatCompatibilityCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
		stdin  string
		want   string
	}{
		{
			name: "real8 add and writefloat",
			source: `
INCLUDE Irvine32.inc

.data
a REAL8 2.5
b REAL8 3.25

.code
main PROC
    finit
    fld a
    fadd b
    call WriteFloat
    exit
main ENDP

END main
`,
			want: "5.750000000000E+00",
		},
		{
			name: "readfloat multiply st operands",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
    finit
    call ReadFloat
    call ReadFloat
    fmul ST(0),ST(1)
    call WriteFloat
    exit
main ENDP

END main
`,
			stdin: "2.5\n4\n",
			want:  "1.000000000000E+01",
		},
		{
			name: "showfpustack two values",
			source: `
INCLUDE Irvine32.inc

.data
first REAL8 123.456
second REAL8 10.0

.code
main PROC
    finit
    fld first
    fld second
    call ShowFPUStack
    exit
main ENDP

END main
`,
			want: "\r\n------ FPU Stack ------\r\nST(0): 1.000000000000E+01\r\nST(1): 1.234560000000E+02\r\n",
		},
		{
			name: "fadd no operand pops stack",
			source: `
INCLUDE Irvine32.inc

.data
a REAL8 2.0
b REAL8 3.5

.code
main PROC
    finit
    fld a
    fld b
    fadd
    call ShowFPUStack
    exit
main ENDP

END main
`,
			want: "\r\n------ FPU Stack ------\r\nST(0): 5.500000000000E+00\r\n",
		},
		{
			name: "real4 sum and sqrt",
			source: `
INCLUDE Irvine32.inc

.data
vals REAL4 4.0,5.0
result REAL4 ?

.code
main PROC
    finit
    fld vals
    fadd [vals+4]
    fsqrt
    fstp result
    fld result
    call WriteFloat
    exit
main ENDP

END main
`,
			want: "3.000000000000E+00",
		},
		{
			name: "fild and fist nearest",
			source: `
INCLUDE Irvine32.inc

.data
N SDWORD 20
X REAL8 3.5
Z SDWORD ?

.code
main PROC
    finit
    fild N
    fadd X
    fist Z
    mov eax, Z
    call WriteDec
    exit
main ENDP

END main
`,
			want: "24",
		},
		{
			name: "fild and fist truncate",
			source: `
INCLUDE Irvine32.inc

.data
N SDWORD 20
X REAL8 3.5
Z SDWORD ?
ctrlWord WORD ?

.code
main PROC
    finit
    fstcw ctrlWord
    or ctrlWord, 110000000000b
    fldcw ctrlWord
    fild N
    fadd X
    fist Z
    mov eax, Z
    call WriteDec
    exit
main ENDP

END main
`,
			want: "23",
		},
		{
			name: "fcomi compare with epsilon",
			source: `
INCLUDE Irvine32.inc
INCLUDE Macros.inc

.data
epsilon REAL8 1.0E-12
val2 REAL8 0.0
val3 REAL8 1.001E-13

.code
main PROC
    finit
    fld epsilon
    fld val2
    fsub val3
    fabs
    fcomi ST(0),ST(1)
    ja skip
    mWrite "equal"
skip:
    exit
main ENDP

END main
`,
			want: "equal",
		},
		{
			name: "fcomp fnstsw sahf jnb path",
			source: `
INCLUDE Irvine32.inc

.data
X REAL8 1.1
Y REAL8 1.2
N DWORD 0

.code
main PROC
    finit
    mov N,0
    fld X
    fcomp Y
    fnstsw ax
    sahf
    jnb L1
    mov N,1
L1:
    mov eax,N
    call WriteDec
    exit
main ENDP

END main
`,
			want: "1",
		},
		{
			name: "float expression with fchs and fmul",
			source: `
INCLUDE Irvine32.inc

.data
valA REAL8 1.5
valB REAL8 2.5
valC REAL8 3.0
valD REAL8 ?

.code
main PROC
    finit
    fld valA
    fchs
    fld valB
    fmul valC
    fadd
    fstp valD
    fld valD
    call WriteFloat
    exit
main ENDP

END main
`,
			want: "6.000000000000E+00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runSource(t, tt.source, tt.stdin); got != tt.want {
				t.Fatalf("unexpected output %q", got)
			}
		})
	}
}
