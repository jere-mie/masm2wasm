package vm_test

import "testing"

func TestLoopAndIndexedAddressingCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "symbol scaled dword index",
			source: `
INCLUDE Irvine32.inc

.data
items DWORD 10,20,30,40

.code
main PROC
    mov esi, 2
    mov eax, items[esi*4]
    call WriteDec
    exit
main ENDP

END main
`,
			want: "30",
		},
		{
			name: "base plus scaled type symbol",
			source: `
INCLUDE Irvine32.inc

.data
table WORD 10h,20h,30h,40h

.code
main PROC
    mov ebx, OFFSET table
    mov esi, 3
    movzx eax, WORD PTR [ebx + esi*TYPE table]
    call WriteHex
    exit
main ENDP

END main
`,
			want: "00000040",
		},
		{
			name: "scaled index with displacement",
			source: `
INCLUDE Irvine32.inc

.data
items DWORD 100,200,300,400

.code
main PROC
    mov esi, 1
    mov eax, [items + esi*4 + 4]
    call WriteDec
    exit
main ENDP

END main
`,
			want: "300",
		},
		{
			name: "base index displacement byte",
			source: `
INCLUDE Irvine32.inc

.data
bytes BYTE 1,2,3,4,5,6

.code
main PROC
    mov ebx, OFFSET bytes
    mov esi, 2
    mov eax, 0
    mov al, [ebx + esi + 2]
    call WriteDec
    exit
main ENDP

END main
`,
			want: "5",
		},
		{
			name: "while simple increment",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
    mov eax, 0
.WHILE eax < 4
    inc eax
.ENDW
    call WriteDec
    exit
main ENDP

END main
`,
			want: "4",
		},
		{
			name: "while with boolean and",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
    mov eax, 0
    mov ebx, 3
.WHILE (eax < 5 && ebx > 0)
    inc eax
    dec ebx
.ENDW
    call WriteDec
    exit
main ENDP

END main
`,
			want: "3",
		},
		{
			name: "nested while loops",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
    mov eax, 0
    mov ebx, 0
.WHILE ebx < 2
    mov ecx, 0
    .WHILE ecx < 3
        inc eax
        inc ecx
    .ENDW
    inc ebx
.ENDW
    call WriteDec
    exit
main ENDP

END main
`,
			want: "6",
		},
		{
			name: "repeat until compare",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
    mov eax, 0
.REPEAT
    add eax, 2
.UNTIL eax >= 8
    call WriteDec
    exit
main ENDP

END main
`,
			want: "8",
		},
		{
			name: "repeat with conditional body",
			source: `
INCLUDE Irvine32.inc

.code
main PROC
    mov eax, 0
    mov ecx, 0
.REPEAT
    inc ecx
    .IF ecx > 2
        add eax, ecx
    .ENDIF
.UNTIL ecx >= 5
    call WriteDec
    exit
main ENDP

END main
`,
			want: "12",
		},
		{
			name: "flowchart style scaled indexing",
			source: `
INCLUDE Irvine32.inc

.data
sample DWORD 50
array DWORD 10,60,20,33,72,89,45,65,72,18

.code
main PROC
    mov eax, 0
    mov edx, sample
    mov esi, 0
.WHILE esi < LENGTHOF array
    .IF array[esi*4] > edx
        add eax, array[esi*4]
    .ENDIF
    inc esi
.ENDW
    call WriteDec
    exit
main ENDP

END main
`,
			want: "358",
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
