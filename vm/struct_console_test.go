package vm_test

import (
	"strings"
	"testing"
)

func TestStructAndConsoleCompatibilityCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
		stdin  string
		want   string
		check  func(t *testing.T, got string)
	}{
		{
			name: "struct field access and size operators",
			source: `
INCLUDE Irvine32.inc

COORD STRUCT
  X WORD ?
  Y WORD ?
COORD ENDS

Rectangle STRUCT
  UpperLeft COORD <>
  LowerRight COORD <>
Rectangle ENDS

.data
rect Rectangle <>

.code
main PROC
    mov eax, TYPE Rectangle
    call WriteDec
    call Crlf
    mov eax, SIZE Rectangle
    call WriteDec
    call Crlf
    mov eax, TYPE rect.UpperLeft.X
    call WriteDec
    call Crlf
    mov rect.UpperLeft.X, 30
    mov esi, OFFSET rect
    mov (Rectangle PTR [esi]).UpperLeft.Y, 40
    movzx eax, rect.UpperLeft.X
    call WriteDec
    call Crlf
    movzx eax, rect.UpperLeft.Y
    call WriteDec
    exit
main ENDP

END main
`,
			want: "8\r\n8\r\n2\r\n30\r\n40",
		},
		{
			name: "union field access",
			source: `
INCLUDE Irvine32.inc

Integer UNION
  D DWORD ?
  W WORD  ?
  B BYTE  ?
Integer ENDS

.data
val1 Integer <12345678h>
val2 Integer <>
val3 Integer <>

.code
main PROC
    mov eax, 12345678h
    mov val1.B, al
    mov val2.W, ax
    mov val3.D, eax
    movzx eax, val1.B
    call WriteHex
    call Crlf
    movzx eax, val2.W
    call WriteHex
    call Crlf
    mov eax, val3.D
    call WriteHex
    exit
main ENDP

END main
`,
			want: "00000078\r\n00005678\r\n12345678",
		},
		{
			name: "label text equ and writeconsole",
			source: `
INCLUDE Irvine32.inc

.data
endl EQU <0dh,0ah>
message LABEL BYTE
  BYTE "Hello", endl
messageSize DWORD ($-message)
consoleHandle DWORD ?
bytesWritten DWORD ?

.code
main PROC
    INVOKE GetStdHandle, STD_OUTPUT_HANDLE
    mov consoleHandle, eax
    INVOKE WriteConsole, consoleHandle, ADDR message, messageSize, ADDR bytesWritten, 0
    call Crlf
    mov eax, bytesWritten
    call WriteDec
    exit
main ENDP

END main
`,
			want: "Hello\r\n\r\n7",
		},
		{
			name: "console info shims",
			source: `
INCLUDE SmallWin.inc

.data
outHandle DWORD ?
scrSize COORD <20,5>
consoleInfo CONSOLE_SCREEN_BUFFER_INFO <>
cursorInfo CONSOLE_CURSOR_INFO <>

.code
main PROC
    INVOKE GetStdHandle, STD_OUTPUT_HANDLE
    mov outHandle, eax
    INVOKE SetConsoleScreenBufferSize, outHandle, scrSize
    INVOKE GetConsoleCursorInfo, outHandle, ADDR cursorInfo
    INVOKE GetConsoleScreenBufferInfo, outHandle, ADDR consoleInfo
    movzx eax, consoleInfo.dwSize.X
    call WriteDec
    call Crlf
    movzx eax, consoleInfo.dwSize.Y
    call WriteDec
    call Crlf
    mov eax, cursorInfo.dwSize
    call WriteDec
    exit
main ENDP

END main
`,
			want: "20\r\n5\r\n25",
		},
		{
			name: "getlocaltime fills systemtime",
			source: `
INCLUDE Irvine32.inc

.data
sysTime SYSTEMTIME <>

.code
main PROC
    INVOKE GetLocalTime, ADDR sysTime
    movzx eax, sysTime.wMonth
    cmp eax, 1
    jb bad
    cmp eax, 12
    ja bad
    mWrite "ok"
    exit
bad:
    mWrite "bad"
    exit
main ENDP

END main
`,
			want: "ok",
		},
		{
			name: "readconsole shim",
			source: `
INCLUDE Irvine32.inc

BufSize = 16

.data
buffer BYTE BufSize DUP(0)
stdInHandle HANDLE ?
bytesRead DWORD ?

.code
main PROC
    INVOKE GetStdHandle, STD_INPUT_HANDLE
    mov stdInHandle, eax
    INVOKE ReadConsole, stdInHandle, ADDR buffer, BufSize, ADDR bytesRead, 0
    mov eax, bytesRead
    mov buffer[eax], 0
    mov edx, OFFSET buffer
    call WriteString
    exit
main ENDP

END main
`,
			stdin: "abc\n",
			want:  "abc\r\n",
		},
		{
			name: "write stack frame helpers",
			source: `
INCLUDE Irvine32.inc

.data
procName BYTE "demo",0

.code
main PROC
    call demo
    exit
main ENDP

demo PROC
    push ebp
    mov ebp, esp
    INVOKE WriteStackFrame, 0, 0, 0
    INVOKE WriteStackFrameName, 0, 0, 0, ADDR procName
    mov esp, ebp
    pop ebp
    ret
demo ENDP

END main
`,
			check: func(t *testing.T, got string) {
				if !strings.Contains(got, "Stack frame") || !strings.Contains(got, "demo") {
					t.Fatalf("unexpected output %q", got)
				}
			},
		},
		{
			name: "textequ and peek console input",
			source: `
INCLUDE Irvine32.inc

.data
keybuf BYTE 50 DUP(0)
recordsRead DWORD ?
stdInHandle DWORD ?

repeatCount TEXTEQU <BYTE PTR [keybuf+4]>
virtualKeyCode TEXTEQU <WORD PTR [keybuf+10]>
asciiCode TEXTEQU <[keybuf+14]>

.code
main PROC
    INVOKE GetStdHandle, STD_INPUT_HANDLE
    mov stdInHandle, eax
    INVOKE PeekConsoleInput, stdInHandle, ADDR keybuf, 1, ADDR recordsRead
    cmp recordsRead, 0
    je fail
    cmp WORD PTR keybuf, KEY_EVENT
    jne fail
    cmp repeatCount, 1
    jne fail
    mov dx, virtualKeyCode
    mov al, asciiCode
    call WriteChar
    call Crlf
    movzx eax, dx
    call WriteDec
    exit
fail:
    mWrite "bad"
    exit
main ENDP

END main
`,
			stdin: "A",
			want:  "A\r\n65",
		},
		{
			name: "flush console input buffer",
			source: `
INCLUDE Irvine32.inc

.data
stdInHandle DWORD ?
events DWORD ?

.code
main PROC
    INVOKE GetStdHandle, STD_INPUT_HANDLE
    mov stdInHandle, eax
    INVOKE GetNumberOfConsoleInputEvents, stdInHandle, ADDR events
    mov eax, events
    call WriteDec
    call Crlf
    INVOKE FlushConsoleInputBuffer, stdInHandle
    INVOKE GetNumberOfConsoleInputEvents, stdInHandle, ADDR events
    mov eax, events
    call WriteDec
    exit
main ENDP

END main
`,
			stdin: "xyz",
			want:  "1\r\n0",
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
