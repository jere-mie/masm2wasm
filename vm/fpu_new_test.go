package vm_test

import "testing"

func TestFPUNewInstructions(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "fxch swaps st0 and st1",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 10.0
b REAL8 20.0
.code
main PROC
    finit
    fld a
    fld b
    fxch
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "1.000000000000E+01",
		},
		{
			name: "fxch with explicit st(1)",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 5.0
b REAL8 7.0
.code
main PROC
    finit
    fld a
    fld b
    fxch ST(1)
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "5.000000000000E+00",
		},
		{
			name: "faddp adds and pops",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 3.0
b REAL8 4.0
.code
main PROC
    finit
    fld a
    fld b
    faddp ST(1), ST(0)
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "7.000000000000E+00",
		},
		{
			name: "fsubp subtracts and pops",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 10.0
b REAL8 3.0
.code
main PROC
    finit
    fld a
    fld b
    fsubp ST(1), ST(0)
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "7.000000000000E+00",
		},
		{
			name: "fmulp multiplies and pops",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 5.0
b REAL8 6.0
.code
main PROC
    finit
    fld a
    fld b
    fmulp ST(1), ST(0)
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "3.000000000000E+01",
		},
		{
			name: "fdivp divides and pops",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 20.0
b REAL8 4.0
.code
main PROC
    finit
    fld a
    fld b
    fdivp ST(1), ST(0)
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "5.000000000000E+00",
		},
		{
			name: "fldpi loads pi",
			source: `
INCLUDE Irvine32.inc
.code
main PROC
    finit
    fldpi
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "3.141592653590E+00",
		},
		{
			name: "fldl2e loads log2(e)",
			source: `
INCLUDE Irvine32.inc
.code
main PROC
    finit
    fldl2e
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "1.442695040889E+00",
		},
		{
			name: "fldln2 loads ln(2)",
			source: `
INCLUDE Irvine32.inc
.code
main PROC
    finit
    fldln2
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "6.931471805599E-01",
		},
		{
			name: "fsin computes sine",
			source: `
INCLUDE Irvine32.inc
.data
val REAL8 0.0
.code
main PROC
    finit
    fld val
    fsin
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "0.000000000000E+00",
		},
		{
			name: "fcos computes cosine",
			source: `
INCLUDE Irvine32.inc
.data
val REAL8 0.0
.code
main PROC
    finit
    fld val
    fcos
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "1.000000000000E+00",
		},
		{
			name: "fsincos pushes sin and cos",
			source: `
INCLUDE Irvine32.inc
.data
val REAL8 0.0
.code
main PROC
    finit
    fld val
    fsincos
    ; ST(0)=cos, ST(1)=sin
    call WriteFloat
    call Crlf
    fxch
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "1.000000000000E+00\r\n0.000000000000E+00",
		},
		{
			name: "fptan computes tangent and pushes 1.0",
			source: `
INCLUDE Irvine32.inc
.data
val REAL8 0.0
.code
main PROC
    finit
    fld val
    fptan
    ; ST(0) = 1.0, ST(1) = tan(val)
    call WriteFloat
    call Crlf
    fxch
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "1.000000000000E+00\r\n0.000000000000E+00",
		},
		{
			name: "fpatan computes atan2",
			source: `
INCLUDE Irvine32.inc
.data
x REAL8 1.0
y REAL8 1.0
.code
main PROC
    finit
    fld x
    fld y
    fpatan
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "7.853981633974E-01",
		},
		{
			name: "fprem computes truncated remainder",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 7.0
b REAL8 3.0
.code
main PROC
    finit
    fld b
    fld a
    fprem
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "1.000000000000E+00",
		},
		{
			name: "fprem1 computes ieee remainder",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 7.0
b REAL8 3.0
.code
main PROC
    finit
    fld b
    fld a
    fprem1
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "1.000000000000E+00",
		},
		{
			name: "fscale doubles value (scale by 1)",
			source: `
INCLUDE Irvine32.inc
.data
val REAL8 3.0
exp REAL8 1.0
.code
main PROC
    finit
    fld exp
    fld val
    fscale
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "6.000000000000E+00",
		},
		{
			name: "fxtract extracts exponent and significand",
			source: `
INCLUDE Irvine32.inc
.data
val REAL8 8.0
.code
main PROC
    finit
    fld val
    fxtract
    ; ST(0)=exponent, ST(1)=significand
    call WriteFloat
    call Crlf
    fxch
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "3.000000000000E+00\r\n1.000000000000E+00",
		},
		{
			name: "fdecstp rotates stack down",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 10.0
b REAL8 20.0
.code
main PROC
    finit
    fld a
    fld b
    fdecstp
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "1.000000000000E+01",
		},
		{
			name: "fcompp compares and pops both",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 3.0
b REAL8 5.0
.code
main PROC
    finit
    fld a
    fld b
    fcompp
    fnstsw ax
    sahf
    ja above
    mov eax, 0
    call WriteDec
    jmp done
above:
    mov eax, 1
    call WriteDec
done:
    exit
main ENDP
END main
`,
			want: "1",
		},
		{
			name: "fucomip compares and sets cpu flags",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 10.0
b REAL8 5.0
.code
main PROC
    finit
    fld b
    fld a
    fucomip ST(0), ST(1)
    ja isAbove
    mov eax, 0
    call WriteDec
    jmp done
isAbove:
    mov eax, 1
    call WriteDec
done:
    exit
main ENDP
END main
`,
			want: "1",
		},
		{
			name: "fnop does nothing",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 42.0
.code
main PROC
    finit
    fld a
    fnop
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "4.200000000000E+01",
		},
		{
			name: "fsubrp reverse subtracts and pops",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 3.0
b REAL8 10.0
.code
main PROC
    finit
    fld a
    fld b
    fsubrp ST(1), ST(0)
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "7.000000000000E+00",
		},
		{
			name: "fdivrp reverse divides and pops",
			source: `
INCLUDE Irvine32.inc
.data
a REAL8 20.0
b REAL8 4.0
.code
main PROC
    finit
    fld a
    fld b
    fdivrp ST(1), ST(0)
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "2.000000000000E-01",
		},
		{
			name: "fldlg2 loads log10(2)",
			source: `
INCLUDE Irvine32.inc
.code
main PROC
    finit
    fldlg2
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "3.010299956640E-01",
		},
		{
			name: "fldl2t loads log2(10)",
			source: `
INCLUDE Irvine32.inc
.code
main PROC
    finit
    fldl2t
    call WriteFloat
    exit
main ENDP
END main
`,
			want: "3.321928094887E+00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runSource(t, tt.source, "")
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
