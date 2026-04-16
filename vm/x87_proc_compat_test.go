package vm_test

import "testing"

func TestX87IntegerArithmeticCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.data
intOne WORD 20
intTwo WORD 10

.code
main PROC
    finit
    fild intOne
    fiadd intTwo
    call WriteFloat
    call Crlf

    finit
    fild intOne
    fisub intTwo
    call WriteFloat
    call Crlf

    finit
    fild intOne
    fisubr intTwo
    call WriteFloat
    call Crlf

    finit
    fild intOne
    fimul intTwo
    call WriteFloat
    call Crlf

    finit
    fild intOne
    fidiv intTwo
    call WriteFloat
    call Crlf

    finit
    fild intOne
    fidivr intTwo
    call WriteFloat
    exit
main ENDP

END main
`

	want := "3.000000000000E+01\r\n1.000000000000E+01\r\n-1.000000000000E+01\r\n2.000000000000E+02\r\n2.000000000000E+00\r\n5.000000000000E-01"
	if got := runSource(t, source, ""); got != want {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestX87TranscendentalCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.data
xVal REAL8 2.0
yVal REAL8 5.0

.code
main PROC
    finit
    fld yVal
    fld xVal
    fyl2x
    call WriteFloat
    call Crlf
    f2xm1
    call WriteFloat
    exit
main ENDP

END main
`

	want := "5.000000000000E+00\r\n3.100000000000E+01"
	if got := runSource(t, source, ""); got != want {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestProcPrivateParsingCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.686P
.XMM
.model flat,stdcall

.code
main PROC
    call helper
    call WriteDec
    exit
main ENDP

helper PROC PRIVATE
    mov eax,42
    ret
helper ENDP

END main
`

	if got := runSource(t, source, ""); got != "42" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestBareEndParsingCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.386
.model flat,stdcall

.code
main PROC
    mov eax,7
    call WriteDec
    exit
main ENDP

END
`

	if got := runSource(t, source, ""); got != "7" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestEndTerminatesParsingCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.code
main PROC
    mov eax,9
    call WriteDec
    exit
main ENDP

END main

main PROC
    mov eax,123
    ret
main ENDP
`

	if got := runSource(t, source, ""); got != "9" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestInputRecordAggregateCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.data
evBuffer INPUT_RECORD <>

.code
main PROC
    mov evBuffer.EventType,KEY_EVENT
    mov evBuffer.Event.bKeyDown,1
    mov evBuffer.Event.uChar.AsciiChar,'A'
    movzx eax,evBuffer.Event.uChar.AsciiChar
    call WriteChar
    exit
main ENDP

END main
`

	if got := runSource(t, source, ""); got != "A" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestMultilineIfContinuationCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.code
main PROC
    mov dx,VK_NUMLOCK
    .IF dx == VK_SHIFT || dx == VK_CONTROL || dx == VK_MENU || \
        dx == VK_CAPITAL || dx == VK_NUMLOCK || dx == VK_SCROLL
        mWrite "match"
    .ENDIF
    exit
main ENDP

END main
`

	if got := runSource(t, source, ""); got != "match" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestIfBitmaskCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

CTRL_MASK = 4
KEY_MASKS = 7

.code
main PROC
    mov ebx,CTRL_MASK
    .IF ebx & CTRL_MASK
        mWrite "mask"
    .ENDIF
    .IF !(ebx & KEY_MASKS)
        mWrite "bad"
    .ENDIF
    exit
main ENDP

END main
`

	if got := runSource(t, source, ""); got != "mask" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestProcedureScopedDataSymbolsCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.code
main PROC
    call firstProc
    call Crlf
    call secondProc
    exit
main ENDP

firstProc PROC
.data
message BYTE "first",0
.code
    mov edx,OFFSET message
    call WriteString
    ret
firstProc ENDP

secondProc PROC
.data
message BYTE "second",0
.code
    mov edx,OFFSET message
    call WriteString
    ret
secondProc ENDP

END main
`

	if got := runSource(t, source, ""); got != "first\r\nsecond" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestCompileTimeIsDefinedCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

StudentMode = 1

IFDEF StudentMode
definedValue = 11
ELSE
definedValue = 99
ENDIF

IFNDEF RealMode
undefinedValue = 22
ELSE
undefinedValue = 99
ENDIF

IF IsDefined( StudentMode )
macroValue = 33
ELSE
macroValue = 99
ENDIF

.code
main PROC
    mov eax,definedValue
    call WriteDec
    mov eax,undefinedValue
    call WriteDec
    mov eax,macroValue
    call WriteDec
    exit
main ENDP

END main
`

	if got := runSource(t, source, ""); got != "112233" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestStartupPseudoOpCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.code
main PROC
    Startup
    mWrite <"flat",0Dh,0Ah>
    mWrite "mode"
    exit
main ENDP

END main
`

	if got := runSource(t, source, ""); got != "flat\r\nmode" {
		t.Fatalf("unexpected output %q", got)
	}
}
