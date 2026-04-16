package vm_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"masminterpreter/internal/masm"
	"masminterpreter/vm"
)

func TestBookCompatibilityExpansionCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "typedef pointers and addr memory expression",
			source: `
INCLUDE Irvine32.inc

PBYTE TYPEDEF PTR BYTE
Swap PROTO, pValX:PTR DWORD, pValY:PTR DWORD

.data
arrayB BYTE 10h,20h,30h
ptr1 PBYTE arrayB
Array DWORD 1,2

.code
main PROC
    mov esi,ptr1
    movzx eax,BYTE PTR [esi+1]
    call WriteHex
    call Crlf

    INVOKE Swap, ADDR Array, ADDR [Array+4]
    mov eax,Array
    call WriteDec
    call Crlf
    mov eax,Array[4]
    call WriteDec
    exit
main ENDP

Swap PROC USES eax esi edi, pValX:PTR DWORD, pValY:PTR DWORD
    mov esi,pValX
    mov edi,pValY
    mov eax,[esi]
    xchg eax,[edi]
    mov [esi],eax
    ret
Swap ENDP

END main
`,
			want: "00000020\r\n2\r\n1",
		},
		{
			name: "signed data and bitwise expressions",
			source: `
INCLUDE Irvine32.inc

.data
byteVal SBYTE -48
wordVal SWORD -5000
attr BYTE (blue SHL 4) OR yellow
mask DWORD NOT 0F0h AND 0FFh

.code
main PROC
    movzx eax,attr
    call WriteHex
    call Crlf
    mov eax,mask
    call WriteHex
    call Crlf

    mov al,byteVal
    cbw
    mov bl,5
    idiv bl
    movsx eax,al
    call WriteInt
    call Crlf

    mov ax,wordVal
    cwd
    mov bx,256
    idiv bx
    movsx eax,ax
    call WriteInt
    exit
main ENDP

END main
`,
			want: "0000001E\r\n0000000F\r\n-9\r\n-19",
		},
		{
			name: "carry flags pusha popa and loopnz",
			source: `
INCLUDE Irvine32.inc
INCLUDE Macros.inc

.data
array SWORD -3,-6,-1,-10,10,30,40,4

.code
main PROC
    mov eax,0FFFFFFFFh
    stc
    adc eax,0
    call WriteHex
    call Crlf

    mov eax,0
    stc
    sbb eax,1
    call WriteHex
    call Crlf

    clc
    jc bad
    cmc
    jnc bad

    mov eax,1
    mov ebx,2
    pusha
    mov eax,10
    mov ebx,20
    popa
    call WriteDec
    call Crlf
    mov eax,ebx
    call WriteDec
    call Crlf

    mov esi,OFFSET array
    mov ecx,LENGTHOF array
next:
    test WORD PTR [esi],8000h
    pushfd
    add esi,TYPE array
    popfd
    loopnz next
    sub esi,TYPE array
    movsx eax,WORD PTR [esi]
    call WriteInt
    exit

bad:
    mWrite "bad"
    exit
main ENDP

END main
`,
			want: "00000000\r\nFFFFFFFE\r\n1\r\n2\r\n10",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := runSource(t, tc.source, ""); got != tc.want {
				t.Fatalf("unexpected output %q", got)
			}
		})
	}
}

func TestHeapGetCharAndExitProcessCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

ExitProcess PROTO, dwExitCode:DWORD

.data
heapHandle DWORD ?

.code
main PROC
    INVOKE GetProcessHeap
    mov heapHandle,eax
    INVOKE HeapAlloc, heapHandle, HEAP_ZERO_MEMORY, 6
    mov esi,eax
    mov BYTE PTR [esi],'O'
    mov BYTE PTR [esi+1],'K'
    mov BYTE PTR [esi+2],0
    mov edx,esi
    call WriteString
    call Crlf

    call GetChar
    call WriteChar

    INVOKE HeapFree, heapHandle, 0, esi
    INVOKE ExitProcess, 7
main ENDP

END main
`

	program, err := masm.Parse(source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	var stdout bytes.Buffer
	machine := vm.NewMachine(strings.NewReader("z"), &stdout, &stdout)
	code, err := machine.Run(program)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if code != 7 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if got := stdout.String(); got != "OK\r\nz" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestProcedureTableIndirectCallCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.data
CaseTable  BYTE 'A'
           DWORD Process_A
           BYTE 'B'
           DWORD Process_B
           BYTE 'C'
           DWORD Process_C
NumberOfEntries = 3

prompt BYTE "Press capital A,B,or C: ",0
msgA BYTE "Process_A",0
msgB BYTE "Process_B",0
msgC BYTE "Process_C",0

.code
main PROC
    mov edx,OFFSET prompt
    call WriteString
    call ReadChar
    mov ebx,OFFSET CaseTable
    mov ecx,NumberOfEntries
L1:
    cmp al,[ebx]
    jne L2
    call NEAR PTR [ebx + 1]
    call WriteString
    call Crlf
    exit
L2:
    add ebx,5
    loop L1
    exit
main ENDP

Process_A PROC
    mov edx,OFFSET msgA
    ret
Process_A ENDP

Process_B PROC
    mov edx,OFFSET msgB
    ret
Process_B ENDP

Process_C PROC
    mov edx,OFFSET msgC
    ret
Process_C ENDP

END main
`

	if got := runSource(t, source, "B"); got != "Press capital A,B,or C: Process_B\r\n" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestConsoleWindowInfoAndLastErrorCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.data
outHandle HANDLE 0
windowRect SMALL_RECT <0,0,60,11>
info CONSOLE_SCREEN_BUFFER_INFO <>

.code
main PROC
    INVOKE GetStdHandle, STD_OUTPUT_HANDLE
    mov outHandle,eax
    INVOKE SetConsoleTextAttribute, outHandle, yellow
    INVOKE SetConsoleWindowInfo, outHandle, TRUE, ADDR windowRect
    INVOKE GetConsoleScreenBufferInfo, outHandle, ADDR info
    movzx eax, info.wAttributes
    call WriteDec
    call Crlf
    movzx eax, info.srWindow.Right
    call WriteDec
    call Crlf
    movzx eax, info.srWindow.Bottom
    call WriteDec
    call Crlf
    INVOKE HeapFree, 1234h, 0, 5678h
    INVOKE GetLastError
    call WriteDec
    call Crlf
    INVOKE GetKeyState, VK_NUMLOCK
    call WriteDec
    exit
main ENDP

END main
`

	if got := runSource(t, source, ""); got != "14\r\n60\r\n11\r\n6\r\n1" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestConsoleOutputCharacterAndAttributeCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.data
outHandle HANDLE ?
cellsWritten DWORD ?
xyPos COORD <10,2>
info CONSOLE_SCREEN_BUFFER_INFO <>
buffer BYTE "ABCDE"
BufSize DWORD ($ - buffer)
attributes WORD 1Eh,2Eh,3Eh,4Eh,5Eh

.code
main PROC
    INVOKE GetStdHandle, STD_OUTPUT_HANDLE
    mov outHandle,eax
    INVOKE SetConsoleTextAttribute, outHandle, yellow

    INVOKE WriteConsoleOutputAttribute,
      outHandle, ADDR attributes,
      BufSize, xyPos,
      ADDR cellsWritten
    mov eax,cellsWritten
    call WriteDec
    call Crlf

    INVOKE WriteConsoleOutputCharacter,
      outHandle, ADDR buffer, BufSize,
      xyPos, ADDR cellsWritten
    mov eax,cellsWritten
    call WriteDec
    call Crlf

    INVOKE GetConsoleScreenBufferInfo, outHandle, ADDR info
    movzx eax, info.wAttributes
    call WriteDec
    call Crlf
    movzx eax, info.dwCursorPosition.X
    call WriteDec
    call Crlf
    movzx eax, info.dwCursorPosition.Y
    call WriteDec
    exit
main ENDP

END main
`

	if got := runSource(t, source, ""); got != "5\r\n5\r\n14\r\n0\r\n0" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestCRuntimePrintfAndScanfCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

printf PROTO C, format:PTR BYTE, args:VARARG
scanf PROTO C, format:PTR BYTE, args:VARARG

.data
prompt BYTE "Enter a float, followed by a double: ",0
formatTwo BYTE "%.2f",9,"%.3f",0dh,0ah,0
strSingle BYTE "%f",0
strDouble BYTE "%lf",0
valStr BYTE "float1 = %.3f",0dh,0ah,0
val1 REAL8 456.789
val2 REAL8 864.231
float1 REAL4 ?
double1 REAL8 ?

.code
main PROC
    INVOKE printf, ADDR formatTwo, val1, val2
    INVOKE printf, ADDR prompt
    INVOKE scanf, ADDR strSingle, ADDR float1
    INVOKE scanf, ADDR strDouble, ADDR double1

    fld float1
    sub esp,8
    fstp qword ptr [esp]
    push OFFSET valStr
    call printf
    add esp,12

    mov eax,DWORD PTR [double1+4]
    call WriteHex
    call Crlf
    exit
main ENDP

END main
`

	if got := runSource(t, source, "1.25\n2.5\n"); got != "456.79\t864.231\r\nEnter a float, followed by a double: float1 = 1.250\r\n40040000\r\n" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestCRuntimeSystemAndFileCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

printf PROTO C, pString:PTR BYTE, args:VARARG
scanf  PROTO C, pFormat:PTR BYTE, pBuffer:PTR BYTE, args:VARARG
system PROTO C, pCommand:PTR BYTE
fopen  PROTO C, filename:PTR BYTE, mode:PTR BYTE
fclose PROTO C, pFile:DWORD

.data
clsCmd BYTE "cls",0
dirCmd BYTE "dir/w",0
prompt BYTE "Enter the name of a file: ",0
fmt BYTE "%s",0
modeStr BYTE "r",0
okMsg BYTE "The file has been opened and closed",0dh,0ah,0
failMsg BYTE "cannot open file",0dh,0ah,0
fileName BYTE 60 DUP(0)
pFile DWORD ?

.code
main PROC
    INVOKE system, ADDR clsCmd
    INVOKE system, ADDR dirCmd
    INVOKE printf, ADDR prompt
    INVOKE scanf, ADDR fmt, ADDR fileName
    INVOKE fopen, ADDR fileName, ADDR modeStr
    mov pFile,eax
    .IF eax == 0
        INVOKE printf, ADDR failMsg
        exit
    .ENDIF
    INVOKE printf, ADDR okMsg
    INVOKE fclose, pFile
    exit
main ENDP

END main
`

	got := runSource(t, source, "machine.go\n")
	for _, want := range []string{"\x1b[2J\x1b[H", "machine.go", "Enter the name of a file: ", "The file has been opened and closed\r\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output %q missing %q", got, want)
		}
	}
}

func TestMessageBoxCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.data
caption1 BYTE "Survey",0
question1 BYTE "Proceed with the assignment?",0
caption2 BYTE "Information",0
question2 BYTE "Save your work first?",0
caption3 BYTE "Stack Call",0
question3 BYTE "Continue anyway?",0

.code
main PROC
    mov ebx,OFFSET caption1
    mov edx,OFFSET question1
    call MsgBoxAsk
    call WriteDec
    call Crlf

    INVOKE MessageBox, NULL, ADDR question2, ADDR caption2, MB_YESNOCANCEL + MB_DEFBUTTON2
    call WriteDec
    call Crlf

    push MB_YESNO
    push OFFSET caption3
    push OFFSET question3
    push 0
    call MessageBox
    call WriteDec
    exit
main ENDP

END main
`

	want := "[MessageBox] Survey\r\nProceed with the assignment?\r\n7\r\n[MessageBox] Information\r\nSave your work first?\r\n7\r\n[MessageBox] Stack Call\r\nContinue anyway?\r\n6"
	if got := runSource(t, source, "no\n"); got != want {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestFormatMessageAndLocalFreeCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.data
missingFile BYTE "__definitely_missing__.txt",0
pErrorMsg DWORD ?

.code
main PROC
    INVOKE CreateFile, ADDR missingFile, GENERIC_READ, 0, 0, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, 0
    INVOKE GetLastError
    INVOKE FormatMessage, FORMAT_MESSAGE_ALLOCATE_BUFFER + FORMAT_MESSAGE_FROM_SYSTEM, \
        NULL, eax, NULL, ADDR pErrorMsg, NULL, NULL
    mov edx,pErrorMsg
    call WriteString
    INVOKE LocalFree, pErrorMsg
    call WriteDec
    exit
main ENDP

END main
`

	if got := runSource(t, source, ""); got != "The system cannot find the file specified.\r\n0" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestSetFilePointerAppendCompatibility(t *testing.T) {
	filePath := t.TempDir() + `\output.txt`
	if err := os.WriteFile(filePath, []byte("Existing line\r\n"), 0o666); err != nil {
		t.Fatalf("seed file failed: %v", err)
	}

	source := fmt.Sprintf(`
INCLUDE Irvine32.inc

.data
buffer BYTE "Appended line",0dh,0ah
bufSize DWORD ($ - buffer)
filename BYTE %q,0
fileHandle HANDLE ?
bytesWritten DWORD ?

.code
main PROC
    INVOKE CreateFile,
      ADDR filename, GENERIC_WRITE, DO_NOT_SHARE, NULL,
      OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, 0
    mov fileHandle,eax
    INVOKE SetFilePointer, fileHandle, 0, 0, FILE_END
    INVOKE WriteFile,
      fileHandle, ADDR buffer, bufSize,
      ADDR bytesWritten, 0
    mov eax,bytesWritten
    call WriteDec
    call Crlf
    INVOKE CloseHandle, fileHandle
    exit
main ENDP

END main
`, filePath)

	if got := runSource(t, source, ""); got != "15\r\n" {
		t.Fatalf("unexpected output %q", got)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	if got := string(data); got != "Existing line\r\nAppended line\r\n" {
		t.Fatalf("unexpected file contents %q", got)
	}
}

func TestConsole2AndSetFilePointerErrorCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

.data
outHandle DWORD ?
scrSize COORD <120,50>
xyPos COORD <20,5>
consoleInfo CONSOLE_SCREEN_BUFFER_INFO <>
cursorInfo CONSOLE_CURSOR_INFO <>
titleStr BYTE "Console2 Demo Program",0
pErrorMsg DWORD ?

.code
main PROC
    INVOKE GetStdHandle, STD_OUTPUT_HANDLE
    mov outHandle,eax

    INVOKE GetConsoleCursorInfo, outHandle, ADDR cursorInfo
    mov cursorInfo.dwSize,75
    mov cursorInfo.bVisible,FALSE
    INVOKE SetConsoleCursorInfo, outHandle, ADDR cursorInfo
    INVOKE SetConsoleScreenBufferSize, outHandle, scrSize
    INVOKE SetConsoleCursorPosition, outHandle, xyPos
    INVOKE SetConsoleTitle, ADDR titleStr
    INVOKE GetConsoleScreenBufferInfo, outHandle, ADDR consoleInfo
    INVOKE GetConsoleCursorInfo, outHandle, ADDR cursorInfo

    movzx eax,consoleInfo.dwSize.X
    call WriteDec
    call Crlf
    movzx eax,consoleInfo.dwSize.Y
    call WriteDec
    call Crlf
    movzx eax,consoleInfo.dwCursorPosition.X
    call WriteDec
    call Crlf
    movzx eax,consoleInfo.dwCursorPosition.Y
    call WriteDec
    call Crlf
    mov eax,cursorInfo.dwSize
    call WriteDec
    call Crlf
    mov eax,cursorInfo.bVisible
    call WriteDec
    call Crlf

    INVOKE SetFilePointer, 1234h, 0, 0, FILE_END
    INVOKE GetLastError
    call WriteDec
    call Crlf
    INVOKE FormatMessage, FORMAT_MESSAGE_ALLOCATE_BUFFER + FORMAT_MESSAGE_FROM_SYSTEM, \
        NULL, eax, NULL, ADDR pErrorMsg, NULL, NULL
    mov edx,pErrorMsg
    call WriteString
    INVOKE LocalFree, pErrorMsg
    exit
main ENDP

END main
`

	want := "\x1b[6;21H120\r\n50\r\n20\r\n5\r\n75\r\n0\r\n6\r\nThe handle is invalid.\r\n"
	if got := runSource(t, source, ""); got != want {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestConsoleCodePageCompatibility(t *testing.T) {
	source := `
INCLUDE Irvine32.inc

SetConsoleOutputCP PROTO, pageNum:DWORD
GetConsoleOutputCP PROTO
SetConsoleCP PROTO, pageNum:DWORD
GetConsoleCP PROTO

.data
char1252 BYTE 80h,0
badMsg BYTE "stack mismatch",0

.code
main PROC
    INVOKE GetConsoleOutputCP
    call WriteDec
    call Crlf

    INVOKE SetConsoleOutputCP, 1252
    INVOKE GetConsoleOutputCP
    call WriteDec
    call Crlf

    INVOKE SetConsoleCP, 850
    INVOKE GetConsoleCP
    call WriteDec
    call Crlf

    mov edx,OFFSET char1252
    call WriteString
    mov al,80h
    call WriteChar
    call Crlf

    mov ebx,esp
    push 437
    call SetConsoleOutputCP
    cmp esp,ebx
    jne bad
    call GetConsoleOutputCP
    call WriteDec
    exit

bad:
    mov edx,OFFSET badMsg
    call WriteString
    exit
main ENDP

END main
`

	if got := runSource(t, source, ""); got != "437\r\n1252\r\n850\r\n€€\r\n437" {
		t.Fatalf("unexpected output %q", got)
	}
}
