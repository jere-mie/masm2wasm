package vm_test

import "testing"

func TestParserWideningCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "compile time rept while anonymous data",
			source: `
INCLUDE Irvine32.inc

.data
nums LABEL DWORD
val = 1
REPT 3
    DWORD val
    val = val + 1
ENDM
f1 = 1
f2 = 1
f3 = f1 + f2
DWORD f1, f2
WHILE f3 LT 10
    DWORD f3
    f1 = f2
    f2 = f3
    f3 = f1 + f2
ENDM

.code
main PROC
    mov esi, OFFSET nums
    mov ecx, 9
L1:
    mov eax, [esi]
    call WriteDec
    call Crlf
    add esi, 4
    loop L1
    exit
main ENDP

END main
`,
			want: "1\r\n2\r\n3\r\n1\r\n1\r\n2\r\n3\r\n5\r\n8\r\n",
		},
		{
			name: "compile time for and forc data",
			source: `
INCLUDE Irvine32.inc

.data
numbers LABEL DWORD
FOR value,<11,22,33>
    DWORD value
ENDM
letters LABEL BYTE
FORC ch,<XYZ>
    BYTE "&ch"
ENDM
BYTE 0

.code
main PROC
    mov esi, OFFSET numbers
    mov ecx, 3
L1:
    mov eax, [esi]
    call WriteDec
    call Crlf
    add esi, 4
    loop L1
    mov edx, OFFSET letters
    call WriteString
    exit
main ENDP

END main
`,
			want: "11\r\n22\r\n33\r\nXYZ",
		},
		{
			name: "textual condition operators and short jump",
			source: `
INCLUDE Irvine32.inc

checkValue = 5

.code
main PROC
    .IF checkValue EQ 5 OR checkValue LT 0
        mWrite "ok"
    .ENDIF
    jmp short done
    mWrite "bad"
done:
    exit
main ENDP

END main
`,
			want: "ok",
		},
		{
			name: "label ptr and anonymous aggregate data",
			source: `
INCLUDE Irvine32.inc

ListNode STRUCT
    NodeData DWORD ?
    NextPtr DWORD ?
ListNode ENDS

.data
LinkedList LABEL PTR ListNode
ListNode <11, ($ + SIZEOF ListNode)>
ListNode <22, 0>

.code
main PROC
    mov esi, OFFSET LinkedList
nextNode:
    mov eax, (ListNode PTR [esi]).NodeData
    call WriteDec
    call Crlf
    mov esi, (ListNode PTR [esi]).NextPtr
    .IF esi EQ 0
        jmp short done
    .ENDIF
    jmp short nextNode
done:
    exit
main ENDP

END main
`,
			want: "11\r\n22\r\n",
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
