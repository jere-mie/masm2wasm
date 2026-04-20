package vm_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"masminterpreter/internal/masm"
	"masminterpreter/vm"
)

// runBookFile reads a .asm file, parses, and runs it through the VM.
// Unlike runSource it tolerates any exit code – we only care about crashes.
func runBookFile(t *testing.T, path, stdin string) (exitCode int, output string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	program, err := masm.Parse(string(data))
	if err != nil {
		t.Fatalf("PARSE FAILED: %v", err)
	}
	var stdout bytes.Buffer
	machine := vm.NewMachine(strings.NewReader(stdin), &stdout, &stdout)
	code, err := machine.Run(program)
	if err != nil {
		t.Fatalf("RUN FAILED (exit=%d): %v", code, err)
	}
	return code, stdout.String()
}

// TestBookExamples runs all 32-bit console book examples from ch05-ch12
// through the parser and VM, verifying they don't crash.
//
// Skipped programs: GUI (GraphWin.inc), 16-bit (Irvine16.inc), multi-file,
// interactive key loops, extreme timing loops, heavy resource use.
func TestBookExamples(t *testing.T) {
	base := filepath.Join("..", "reference", "asmbook", "Irvine", "Examples")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		t.Skip("reference examples directory not found")
	}

	type ex struct {
		path  string // relative to base
		stdin string // stdin to feed
		skip  string // non-empty = skip reason
	}

	examples := []ex{
		// ===== Chapter 3: Assembly Language Fundamentals =====
		{path: filepath.Join("ch03", "AddTwo.asm")},
		{path: filepath.Join("ch03", "AddTwoSum.asm")},
		{path: filepath.Join("ch03", "AddTwoSum_64.asm"), skip: "64-bit program (uses rax)"},
		{path: filepath.Join("ch03", "AddVariables.asm")},
		{path: filepath.Join("ch03", "template.asm")},

		// ===== Chapter 4: Data Transfers, Addressing, and Arithmetic =====
		{path: filepath.Join("ch04", "32 bit", "AdditionTest.asm")},
		{path: filepath.Join("ch04", "32 bit", "CopyStr.asm")},
		{path: filepath.Join("ch04", "32 bit", "Moves.asm")},
		{path: filepath.Join("ch04", "32 bit", "Operator.asm")},
		{path: filepath.Join("ch04", "32 bit", "Pointers.asm")},
		{path: filepath.Join("ch04", "32 bit", "SumArray.asm")},
		{path: filepath.Join("ch04", "32 bit", "template.asm")},

		// ===== Chapter 5: Procedures and Library Calls =====
		{path: filepath.Join("ch05", "32 bit", "CodePageDemo.asm")},
		{path: filepath.Join("ch05", "32 bit", "colors.asm")},
		{path: filepath.Join("ch05", "32 bit", "InputLoop.asm"), stdin: "42\n-100\n0\n255\n\n"},
		{path: filepath.Join("ch05", "32 bit", "msgbox.asm")},
		{path: filepath.Join("ch05", "32 bit", "msgboxAsk.asm"), stdin: "no\n"},
		{path: filepath.Join("ch05", "32 bit", "TestLib1.asm"), stdin: "42\n\n"},
		{path: filepath.Join("ch05", "32 bit", "TestLib2.asm")},
		{path: filepath.Join("ch05", "32 bit", "TestLib3.asm"), skip: "inner loop 0FFFFFFFh iterations, too slow"},

		// ===== Chapter 6: Conditional Processing =====
		{path: filepath.Join("ch06", "32 bit", "ArryScan.asm")},
		{path: filepath.Join("ch06", "32 bit", "Encrypt.asm"), stdin: "Hello\n"},
		{path: filepath.Join("ch06", "32 bit", "Finite.asm"), stdin: "+123\n"},
		{path: filepath.Join("ch06", "32 bit", "Flowchart.asm")},
		{path: filepath.Join("ch06", "32 bit", "ifstatements.asm")},
		{path: filepath.Join("ch06", "32 bit", "Loopnz.asm")},
		{path: filepath.Join("ch06", "32 bit", "ProcTble.asm"), stdin: "B"},
		{path: filepath.Join("ch06", "32 bit", "Regist.asm")},
		{path: filepath.Join("ch06", "32 bit", "RegistAlt.asm")},
		{path: filepath.Join("ch06", "32 bit", "Section_6_7_1.asm")},
		{path: filepath.Join("ch06", "32 bit", "SetCur.asm")},
		{path: filepath.Join("ch06", "32 bit", "TestGetCommandTail.asm")},

		// ===== Chapter 7: Integer Arithmetic =====
		{path: filepath.Join("ch07", "32 bit", "AddPacked.asm")},
		{path: filepath.Join("ch07", "32 bit", "ASCII_add.asm")},
		{path: filepath.Join("ch07", "32 bit", "BinToAsc.asm")},
		{path: filepath.Join("ch07", "32 bit", "Bmult.asm")},
		{path: filepath.Join("ch07", "32 bit", "CompareMult.asm"), skip: "tight timing loops, too slow for VM"},
		{path: filepath.Join("ch07", "32 bit", "Divide32.asm")},
		{path: filepath.Join("ch07", "32 bit", "encrypt_1.asm"), stdin: "Hello\n"},
		{path: filepath.Join("ch07", "32 bit", "Express.asm"), skip: "intentional div-by-zero demo"},
		{path: filepath.Join("ch07", "32 bit", "ExtAdd.asm")},
		{path: filepath.Join("ch07", "32 bit", "Idiv.asm")},
		{path: filepath.Join("ch07", "32 bit", "imul.asm")},
		{path: filepath.Join("ch07", "32 bit", "Multiply.asm")},
		{path: filepath.Join("ch07", "32 bit", "MultiShift.asm")},
		{path: filepath.Join("ch07", "32 bit", "Shrd.asm")},
		{path: filepath.Join("ch07", "32 bit", "WriteBin.asm")},

		// ===== Chapter 8: Advanced Procedures =====
		{path: filepath.Join("ch08", "32 bit", "AddTwo.asm")},
		{path: filepath.Join("ch08", "32 bit", "ArrayFill.asm")},
		{path: filepath.Join("ch08", "32 bit", "ArrySum.asm")},
		{path: filepath.Join("ch08", "32 bit", "Fact.asm")},
		{path: filepath.Join("ch08", "32 bit", "LocalExample.asm"), skip: "empty main PROC with no exit, falls through"},
		{path: filepath.Join("ch08", "32 bit", "LocalVars.asm")},
		{path: filepath.Join("ch08", "32 bit", "MakeArray.asm")},
		{path: filepath.Join("ch08", "32 bit", "Multiword.asm")},
		{path: filepath.Join("ch08", "32 bit", "Params.asm")},
		{path: filepath.Join("ch08", "32 bit", "proc.asm")},
		{path: filepath.Join("ch08", "32 bit", "Prototypes.asm")},
		{path: filepath.Join("ch08", "32 bit", "Read_File.asm"), skip: "manual stack manipulation in disassembly demo"},
		{path: filepath.Join("ch08", "32 bit", "Recurse.asm"), stdin: "3\n"},
		{path: filepath.Join("ch08", "32 bit", "RecursiveSum.asm")},
		{path: filepath.Join("ch08", "32 bit", "Reglist.asm")},
		{path: filepath.Join("ch08", "32 bit", "Smallint.asm"), skip: "parse error: ENDP without PROC in reference file"},
		{path: filepath.Join("ch08", "32 bit", "swap.asm")},
		{path: filepath.Join("ch08", "32 bit", "Test_WriteStackFrame.asm")},
		{path: filepath.Join("ch08", "32 bit", "Uppercase.asm")},
		{path: filepath.Join("ch08", "32 bit", "UsesTest.asm")},

		// ===== Chapter 9: Strings and Arrays =====
		{path: filepath.Join("ch09", "32 bit", "Base-Index.asm")},
		{path: filepath.Join("ch09", "32 bit", "Bsort.asm"), skip: "no main PROC, procedure library only"},
		{path: filepath.Join("ch09", "32 bit", "Cmpsb.asm")},
		{path: filepath.Join("ch09", "32 bit", "Compare.asm")},
		{path: filepath.Join("ch09", "32 bit", "CopyStr.asm")},
		{path: filepath.Join("ch09", "32 bit", "Length.asm")},
		{path: filepath.Join("ch09", "32 bit", "Mult.asm")},
		{path: filepath.Join("ch09", "32 bit", "RowSum.asm"), stdin: "1\n"},
		{path: filepath.Join("ch09", "32 bit", "StringDemo.asm")},
		{path: filepath.Join("ch09", "32 bit", "Table.asm")},
		{path: filepath.Join("ch09", "32 bit", "Table2.asm")},
		{path: filepath.Join("ch09", "32 bit", "Trim.asm")},
		{path: filepath.Join("ch09", "32 bit", "Ucase.asm")},

		// ===== Chapter 10: Structures and Macros =====
		{path: filepath.Join("ch10", "AllPoints.asm")},
		{path: filepath.Join("ch10", "Fibon.asm")},
		{path: filepath.Join("ch10", "HelloNew.asm")},
		{path: filepath.Join("ch10", "List.asm")},
		{path: filepath.Join("ch10", "Macro1.asm")},
		{path: filepath.Join("ch10", "Macro2.asm"), stdin: "test\n"},
		{path: filepath.Join("ch10", "Macro3.asm")},
		{path: filepath.Join("ch10", "MacroTest.asm"), stdin: "John\n"},
		{path: filepath.Join("ch10", "Repeat.asm"), skip: "FOR-macro struct instantiation with OFFSET member access not yet supported"},
		{path: filepath.Join("ch10", "RowSum.asm")},
		{path: filepath.Join("ch10", "ShowTime.asm")},
		{path: filepath.Join("ch10", "Struct1.asm"), skip: "test_alignment loop 0FFFFFFFFh iterations, too slow"},
		{path: filepath.Join("ch10", "Struct2.asm")},
		{path: filepath.Join("ch10", "TestDump.asm")},
		{path: filepath.Join("ch10", "Union.asm")},
		{path: filepath.Join("ch10", "Walk.asm")},
		{path: filepath.Join("ch10", "Wraps.asm"), stdin: "John\nDoe\n"},

		// ===== Chapter 11: MS-Windows Programming =====
		{path: filepath.Join("ch11", "AppendFile.asm"), skip: "depends on existing output.txt file"},
		{path: filepath.Join("ch11", "CheckError.asm")},
		{path: filepath.Join("ch11", "Console1.asm")},
		{path: filepath.Join("ch11", "CreateFile.asm"), skip: "creates file, needs stdin filename"},
		{path: filepath.Join("ch11", "HeapTest1.asm")},
		{path: filepath.Join("ch11", "HeapTest2.asm"), skip: "allocates up to 400MB heap"},
		{path: filepath.Join("ch11", "Keybd.asm")},
		{path: filepath.Join("ch11", "MessageBox.asm")},
		{path: filepath.Join("ch11", "PeekInput.asm"), skip: "interactive Delay/ReadKey polling loop"},
		{path: filepath.Join("ch11", "ReadCharTest.asm"), stdin: "x"},
		{path: filepath.Join("ch11", "ReadConsole.asm"), stdin: "test input\n"},
		{path: filepath.Join("ch11", "ReadFile.asm"), skip: "needs user-supplied filename and external file"},
		{path: filepath.Join("ch11", "Scroll.asm"), stdin: "xx"},
		{path: filepath.Join("ch11", "template.asm")},
		{path: filepath.Join("ch11", "TestReadkey.asm"), skip: "interactive Delay/ReadKey polling loop"},
		{path: filepath.Join("ch11", "Timer.asm"), skip: "inner loop 10000100h iterations, too slow"},
		{path: filepath.Join("ch11", "TimingLoop.asm"), skip: "5-second Sleep-based timing loop"},
		{path: filepath.Join("ch11", "WriteColors.asm"), skip: "typo in reference file: 19.20 should be 19,20"},
		{path: filepath.Join("ch11", "WriteFile.asm"), skip: "creates output.txt as side effect"},

		// ===== Chapter 12: Floating-Point Processing =====
		{path: filepath.Join("ch12", "Exceptions.asm")},
		{path: filepath.Join("ch12", "Expr.asm")},
		{path: filepath.Join("ch12", "expressions.asm")},
		{path: filepath.Join("ch12", "FCompare.asm")},
		{path: filepath.Join("ch12", "floatTest32.asm"), stdin: "1.5\n2.5\n"},
		{path: filepath.Join("ch12", "LoadAndStore.asm")},
		{path: filepath.Join("ch12", "LossOfPrecision.asm")},
		{path: filepath.Join("ch12", "MixedMode.asm")},
	}

	for _, e := range examples {
		parts := strings.Split(e.path, string(filepath.Separator))
		ch := parts[0]
		name := strings.TrimSuffix(parts[len(parts)-1], ".asm")
		testName := ch + "/" + name

		e := e // capture
		t.Run(testName, func(t *testing.T) {
			if e.skip != "" {
				t.Skip(e.skip)
				return
			}
			fullPath := filepath.Join(base, e.path)
			code, out := runBookFile(t, fullPath, e.stdin)
			t.Logf("exit=%d output=%d bytes", code, len(out))
		})
	}
}
