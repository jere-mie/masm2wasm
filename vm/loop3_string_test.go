package vm_test

import "testing"

func TestStringInstructionCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "lodsb forward",
			source: `
INCLUDE Irvine32.inc

.data
values BYTE 10,20,30

.code
main PROC
    cld
    mov esi, OFFSET values
    lodsb
    movzx eax, al
    call WriteDec
    call Crlf
    lodsb
    movzx eax, al
    call WriteDec
    exit
main ENDP

END main
`,
			want: "10\r\n20",
		},
		{
			name: "lodsd stosd transform",
			source: `
INCLUDE Irvine32.inc

.data
src DWORD 1,2,3
dst DWORD 3 DUP(0)

.code
main PROC
    cld
    mov esi, OFFSET src
    mov edi, OFFSET dst
    mov ecx, LENGTHOF src
L1:
    lodsd
    add eax, 5
    stosd
    loop L1
    mov eax, dst[8]
    call WriteDec
    exit
main ENDP

END main
`,
			want: "8",
		},
		{
			name: "rep movsb string copy",
			source: `
INCLUDE Irvine32.inc

.data
src  BYTE "copy me",0
dest BYTE 16 DUP(0)

.code
main PROC
    cld
    mov esi, OFFSET src
    mov edi, OFFSET dest
    mov ecx, LENGTHOF src
    rep movsb
    mov edx, OFFSET dest
    call WriteString
    exit
main ENDP

END main
`,
			want: "copy me",
		},
		{
			name: "rep movsd dword copy",
			source: `
INCLUDE Irvine32.inc

.data
src DWORD 100,200,300,400
dst DWORD 4 DUP(0)

.code
main PROC
    cld
    mov esi, OFFSET src
    mov edi, OFFSET dst
    mov ecx, LENGTHOF src
    rep movsd
    mov eax, dst[12]
    call WriteDec
    exit
main ENDP

END main
`,
			want: "400",
		},
		{
			name: "repe cmpsb equal strings",
			source: `
INCLUDE Irvine32.inc

.data
left  BYTE "ABC",0
right BYTE "ABC",0
equalMsg BYTE "equal",0
diffMsg BYTE "diff",0

.code
main PROC
    cld
    mov esi, OFFSET left
    mov edi, OFFSET right
    mov ecx, LENGTHOF left
    repe cmpsb
    jne different
    mov edx, OFFSET equalMsg
    jmp done
different:
    mov edx, OFFSET diffMsg
done:
    call WriteString
    exit
main ENDP

END main
`,
			want: "equal",
		},
		{
			name: "repe cmpsb less than",
			source: `
INCLUDE Irvine32.inc

.data
left  BYTE "ABC",0
right BYTE "ABD",0
lowMsg BYTE "lower",0
highMsg BYTE "higher",0

.code
main PROC
    cld
    mov esi, OFFSET left
    mov edi, OFFSET right
    mov ecx, LENGTHOF left
    repe cmpsb
    jb lower
    mov edx, OFFSET highMsg
    jmp done
lower:
    mov edx, OFFSET lowMsg
done:
    call WriteString
    exit
main ENDP

END main
`,
			want: "lower",
		},
		{
			name: "repne scasb find terminator",
			source: `
INCLUDE Irvine32.inc

.data
buffer BYTE "abc",0,"zzz"

.code
main PROC
    cld
    mov edi, OFFSET buffer
    mov ecx, SIZEOF buffer
    mov al, 0
    repne scasb
    mov eax, edi
    sub eax, OFFSET buffer
    call WriteDec
    exit
main ENDP

END main
`,
			want: "4",
		},
		{
			name: "std lodsb reverse",
			source: `
INCLUDE Irvine32.inc

.data
values BYTE 10,20,30

.code
main PROC
    std
    mov esi, OFFSET values + 2
    lodsb
    movzx eax, al
    call WriteDec
    call Crlf
    lodsb
    movzx eax, al
    call WriteDec
    exit
main ENDP

END main
`,
			want: "30\r\n20",
		},
		{
			name: "std stosb reverse fill",
			source: `
INCLUDE Irvine32.inc

.data
buffer BYTE 4 DUP(0)

.code
main PROC
    std
    mov edi, OFFSET buffer + 2
    mov al, 'A'
    stosb
    mov al, 'B'
    stosb
    mov al, 'C'
    stosb
    cld
    mov edx, OFFSET buffer
    call WriteString
    exit
main ENDP

END main
`,
			want: "CBA",
		},
		{
			name: "rep stosw fill words",
			source: `
INCLUDE Irvine32.inc

.data
buffer WORD 3 DUP(0)

.code
main PROC
    cld
    mov edi, OFFSET buffer
    mov ax, 1234h
    mov ecx, LENGTHOF buffer
    rep stosw
    movzx eax, WORD PTR buffer[4]
    call WriteHex
    exit
main ENDP

END main
`,
			want: "00001234",
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
