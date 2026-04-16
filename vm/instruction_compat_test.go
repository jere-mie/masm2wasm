package vm_test

import "testing"

func TestFloatingPointInstructionParity(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "fcom sets status word without popping",
			source: `
INCLUDE Irvine32.inc

.data
X REAL8 1.1
Y REAL8 1.2
saved REAL8 ?
flag DWORD 0

.code
main PROC
    finit
    fld X
    fcom Y
    fnstsw ax
    sahf
    jnb doneCompare
    mov flag,1
doneCompare:
    fstp saved
    mov eax,flag
    call WriteDec
    call Crlf
    fld saved
    call WriteFloat
    exit
main ENDP

END main
`,
			want: "1\r\n1.100000000000E+00",
		},
		{
			name: "fistp stores integer and pops stack",
			source: `
INCLUDE Irvine32.inc

.data
X REAL8 3.5
Z SDWORD ?

.code
main PROC
    finit
    fld X
    fistp Z
    mov eax,Z
    call WriteDec
    call ShowFPUStack
    exit
main ENDP

END main
`,
			want: "4\r\n------ FPU Stack ------\r\n",
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

func TestShiftAndDecimalInstructionParity(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "shrd and shld sample style shifts",
			source: `
INCLUDE Irvine32.inc

.data
value DWORD 648B2165h

.code
main PROC
    mov eax,8C943A29h
    mov cl,4
    shrd value,eax,cl
    mov eax,value
    call WriteHex
    call Crlf

    mov ebx,648B2165h
    mov eax,8C943A29h
    shld ebx,eax,4
    mov eax,ebx
    call WriteHex
    exit
main ENDP

END main
`,
			want: "9648B216\r\n48B21658",
		},
		{
			name: "ascii and packed decimal adjust instructions",
			source: `
INCLUDE Irvine32.inc

DECIMAL_OFFSET = 5

.data
decimal_one BYTE "100123456789765"
decimal_two BYTE "900402076502015"
sum BYTE (SIZEOF decimal_one + 1) DUP(0),0
packed_1 WORD 4536h
packed_2 WORD 7207h
packed_sum DWORD ?

.code
main PROC
    mov esi,SIZEOF decimal_one - 1
    mov edi,SIZEOF decimal_one
    mov ecx,SIZEOF decimal_one
    mov bh,0
L1:
    mov ah,0
    mov al,decimal_one[esi]
    add al,bh
    aaa
    mov bh,ah
    or bh,30h
    add al,decimal_two[esi]
    aaa
    or bh,ah
    or bh,30h
    or al,30h
    mov sum[edi],al
    dec esi
    dec edi
    loop L1
    mov sum[edi],bh

    mov edx,OFFSET sum
    call WriteString
    call Crlf

    mov packed_sum,0
    mov esi,0
    mov al,BYTE PTR packed_1[esi]
    add al,BYTE PTR packed_2[esi]
    daa
    mov BYTE PTR packed_sum[esi],al

    inc esi
    mov al,BYTE PTR packed_1[esi]
    adc al,BYTE PTR packed_2[esi]
    daa
    mov BYTE PTR packed_sum[esi],al

    inc esi
    mov al,0
    adc al,0
    mov BYTE PTR packed_sum[esi],al

    mov eax,packed_sum
    call WriteHex
    call Crlf

    mov ah,0
    mov al,'5'
    sub al,'3'
    aas
    or al,30h
    call WriteChar
    call Crlf

    mov al,35h
    sub al,09h
    das
    movzx eax,al
    call WriteHex
    exit
main ENDP

END main
`,
			want: "1000525533291780\r\n00011743\r\n2\r\n00000026",
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

func TestFrameAndLookupInstructionParity(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "enter allocates stack frame for locals",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
    call Demo
    exit
main ENDP

Demo PROC
    enter 8,0
    mov DWORD PTR [ebp-4],1234
    mov DWORD PTR [ebp-8],5678
    mov eax,[ebp-4]
    add eax,[ebp-8]
    call WriteDec
    leave
    ret
Demo ENDP

END main
`,
			want: "6912",
		},
		{
			name: "xlat and pushf popf preserve translated flow",
			source: `
INCLUDE Irvine32.inc
INCLUDE Macros.inc

.data
digits BYTE "0123456789ABCDEF"

.code
main PROC
    mov ebx,OFFSET digits
    mov al,0Ah
    xlat
    call WriteChar
    call Crlf

    stc
    pushf
    clc
    popf
    jnc failed
    mWrite "carry"
    exit

failed:
    mWrite "bad"
    exit
main ENDP

END main
`,
			want: "A\r\ncarry",
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
