package vm_test

import "testing"

func TestPtrIndexedByteMoveCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.data
littleEndian DWORD 41424344h
bigEndian BYTE 5 DUP(0)

.code
main PROC
    mov al, BYTE PTR littleEndian[3]
    mov bigEndian, al
    mov al, BYTE PTR littleEndian[2]
    mov bigEndian[1], al
    mov al, BYTE PTR littleEndian[1]
    mov bigEndian[2], al
    mov al, BYTE PTR littleEndian[0]
    mov bigEndian[3], al
    mov edx, OFFSET bigEndian
    call WriteString
    exit
main ENDP

END main
`

	if got := runSource(t, source, ""); got != "ABCD" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestBitwiseCompatibilityCases(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.code
main PROC
    mov al, 1
    shl al, 7
    movzx eax, al
    call WriteHex
    call Crlf

    mov al, 96h
    shr al, 2
    movzx eax, al
    call WriteHex
    call Crlf

    mov al, 96h
    sar al, 2
    movzx eax, al
    call WriteHex
    call Crlf

    mov al, 81h
    rol al, 1
    movzx eax, al
    call WriteHex
    call Crlf

    mov cl, 2
    ror al, cl
    movzx eax, al
    call WriteHex
    call Crlf

    mov eax, 0
    cmp eax, 1
    mov al, 81h
    rcl al, 1
    movzx eax, al
    call WriteHex
    call Crlf

    rcr al, 1
    movzx eax, al
    call WriteHex
    exit
main ENDP

END main
`

	want := "00000080\r\n00000025\r\n000000E5\r\n00000003\r\n000000C0\r\n00000003\r\n00000081"
	if got := runSource(t, source, ""); got != want {
		t.Fatalf("unexpected output %q", got)
	}
}
