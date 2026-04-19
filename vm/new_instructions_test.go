package vm_test

import "testing"

func TestCMOVccInstructions(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "cmove moves when equal",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 10
    mov ebx, 99
    cmp eax, 10
    cmove eax, ebx
    call WriteDec
    exit
main ENDP
END main
`,
			want: "99",
		},
		{
			name: "cmove does not move when not equal",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 10
    mov ebx, 99
    cmp eax, 5
    cmove eax, ebx
    call WriteDec
    exit
main ENDP
END main
`,
			want: "10",
		},
		{
			name: "cmovne moves when not equal",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 10
    mov ebx, 42
    cmp eax, 5
    cmovne eax, ebx
    call WriteDec
    exit
main ENDP
END main
`,
			want: "42",
		},
		{
			name: "cmovg moves when greater (signed)",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 1
    mov ebx, 77
    cmp eax, 0
    cmovg eax, ebx
    call WriteDec
    exit
main ENDP
END main
`,
			want: "77",
		},
		{
			name: "cmova moves when above (unsigned)",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 5
    mov ebx, 55
    cmp eax, 3
    cmova eax, ebx
    call WriteDec
    exit
main ENDP
END main
`,
			want: "55",
		},
		{
			name: "cmovb moves when below (unsigned)",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 2
    mov ebx, 33
    cmp eax, 5
    cmovb eax, ebx
    call WriteDec
    exit
main ENDP
END main
`,
			want: "33",
		},
		{
			name: "cmovs moves when sign set",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 0
    mov ebx, 88
    sub eax, 1
    cmovs eax, ebx
    call WriteDec
    exit
main ENDP
END main
`,
			want: "88",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runSource(t, tt.source, ""); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSETccInstructions(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "sete sets 1 when equal",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    xor eax, eax
    cmp eax, 0
    sete al
    call WriteDec
    exit
main ENDP
END main
`,
			want: "1",
		},
		{
			name: "sete sets 0 when not equal",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 5
    cmp eax, 3
    sete al
    call WriteDec
    exit
main ENDP
END main
`,
			want: "0",
		},
		{
			name: "setne sets 1 when not equal",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 5
    cmp eax, 3
    setne al
    call WriteDec
    exit
main ENDP
END main
`,
			want: "1",
		},
		{
			name: "setg sets 1 when greater signed",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 10
    cmp eax, 5
    setg al
    movzx eax, al
    call WriteDec
    exit
main ENDP
END main
`,
			want: "1",
		},
		{
			name: "setl sets 1 when less signed",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    xor eax, eax
    mov ecx, 3
    cmp ecx, 10
    setl al
    call WriteDec
    exit
main ENDP
END main
`,
			want: "1",
		},
		{
			name: "seta sets 1 when above unsigned",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    xor eax, eax
    mov ecx, 10
    cmp ecx, 5
    seta al
    call WriteDec
    exit
main ENDP
END main
`,
			want: "1",
		},
		{
			name: "setb sets 1 when below unsigned",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    xor eax, eax
    mov ecx, 2
    cmp ecx, 5
    setb al
    call WriteDec
    exit
main ENDP
END main
`,
			want: "1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runSource(t, tt.source, ""); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBitTestInstructions(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "bt sets CF from bit",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 5
    bt eax, 2
    jc bitSet
    mov eax, 0
    jmp done
bitSet:
    mov eax, 1
done:
    call WriteDec
    exit
main ENDP
END main
`,
			want: "1",
		},
		{
			name: "bt clears CF when bit not set",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 5
    bt eax, 1
    jc bitSet
    mov eax, 0
    jmp done
bitSet:
    mov eax, 1
done:
    call WriteDec
    exit
main ENDP
END main
`,
			want: "0",
		},
		{
			name: "bts sets bit and returns old in CF",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 0
    bts eax, 3
    call WriteDec
    exit
main ENDP
END main
`,
			want: "8",
		},
		{
			name: "btr clears bit",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 15
    btr eax, 1
    call WriteDec
    exit
main ENDP
END main
`,
			want: "13",
		},
		{
			name: "btc toggles bit",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 5
    btc eax, 1
    call WriteDec
    exit
main ENDP
END main
`,
			want: "7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runSource(t, tt.source, ""); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBSFBSRInstructions(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "bsf finds lowest set bit",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov ecx, 12
    bsf eax, ecx
    call WriteDec
    exit
main ENDP
END main
`,
			want: "2",
		},
		{
			name: "bsr finds highest set bit",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov ecx, 12
    bsr eax, ecx
    call WriteDec
    exit
main ENDP
END main
`,
			want: "3",
		},
		{
			name: "bsf sets ZF when source is zero",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov ecx, 0
    bsf eax, ecx
    jz isZero
    mov eax, 99
    jmp done
isZero:
    mov eax, 1
done:
    call WriteDec
    exit
main ENDP
END main
`,
			want: "1",
		},
		{
			name: "bsr sets ZF when source is zero",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov ecx, 0
    bsr eax, ecx
    jz isZero
    mov eax, 99
    jmp done
isZero:
    mov eax, 1
done:
    call WriteDec
    exit
main ENDP
END main
`,
			want: "1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runSource(t, tt.source, ""); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBSWAPInstruction(t *testing.T) {
	// 0x12345678 -> 0x78563412 = 2018915346
	source := `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 12345678h
    bswap eax
    call WriteHex
    exit
main ENDP
END main
`
	if got := runSource(t, source, ""); got != "78563412" {
		t.Fatalf("got %q, want %q", got, "78563412")
	}
}

func TestXADDInstruction(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "xadd exchanges and adds",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 10
    mov ebx, 20
    xadd eax, ebx
    call WriteDec
    mov eax, ebx
    call Crlf
    call WriteDec
    exit
main ENDP
END main
`,
			want: "30\r\n10",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runSource(t, tt.source, ""); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCMPXCHGInstruction(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "cmpxchg stores src when equal",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 5
    mov ebx, 5
    mov ecx, 99
    cmpxchg ebx, ecx
    je wasEqual
    mov eax, 0
    jmp done
wasEqual:
    mov eax, ebx
done:
    call WriteDec
    exit
main ENDP
END main
`,
			want: "99",
		},
		{
			name: "cmpxchg loads dest to accumulator when not equal",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov eax, 5
    mov ebx, 7
    mov ecx, 99
    cmpxchg ebx, ecx
    call WriteDec
    exit
main ENDP
END main
`,
			want: "7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runSource(t, tt.source, ""); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAADInstruction(t *testing.T) {
	source := `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov ah, 3
    mov al, 5
    aad
    movzx eax, al
    call WriteDec
    exit
main ENDP
END main
`
	// AH*10 + AL = 3*10 + 5 = 35, AH = 0
	if got := runSource(t, source, ""); got != "35" {
		t.Fatalf("got %q, want %q", got, "35")
	}
}

func TestAAMInstruction(t *testing.T) {
	source := `
INCLUDE Irvine32.inc
.data
.code
main PROC
    mov al, 35
    aam
    movzx ecx, al
    movzx eax, ah
    call WriteDec
    call Crlf
    mov eax, ecx
    call WriteDec
    exit
main ENDP
END main
`
	// AL=35: AH = 35/10 = 3, AL = 35 mod 10 = 5
	if got := runSource(t, source, ""); got != "3\r\n5" {
		t.Fatalf("got %q, want %q", got, "3\r\n5")
	}
}

func TestLAHFInstruction(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "lahf captures CF",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    stc
    lahf
    movzx eax, ah
    and eax, 1
    call WriteDec
    exit
main ENDP
END main
`,
			want: "1",
		},
		{
			name: "lahf captures ZF",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    xor eax, eax
    lahf
    movzx eax, ah
    shr eax, 6
    and eax, 1
    call WriteDec
    exit
main ENDP
END main
`,
			want: "1",
		},
		{
			name: "lahf roundtrips with sahf",
			source: `
INCLUDE Irvine32.inc
.data
.code
main PROC
    stc
    lahf
    clc
    sahf
    jc wasSet
    mov eax, 0
    jmp done
wasSet:
    mov eax, 1
done:
    call WriteDec
    exit
main ENDP
END main
`,
			want: "1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runSource(t, tt.source, ""); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
