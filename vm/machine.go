package vm

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/bits"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	regEAX = iota
	regECX
	regEDX
	regEBX
	regESP
	regEBP
	regESI
	regEDI
)

const (
	stdInputHandleValue  = uint32(0xFFFFFFF6)
	stdOutputHandleValue = uint32(0xFFFFFFF5)
	stdErrorHandleValue  = uint32(0xFFFFFFF4)
)

type Machine struct {
	rawIn         io.Reader
	stdin         *bufio.Reader
	stdout        io.Writer
	stderr        io.Writer
	peekableInput bool

	program *Program
	memory  []byte
	symbols map[string]Symbol
	procs   map[string]*Procedure
	regs    [8]uint32

	zf bool
	sf bool
	cf bool
	of bool
	af bool
	pf bool
	df bool

	exitCode int
	args     []string

	rng       *rand.Rand
	startTime time.Time
	lastError string

	colorAttr uint32
	cursorX   int
	cursorY   int

	files      map[uint32]*os.File
	nextHandle uint32

	fpu            []float64
	fpuControlWord uint16
	fpuStatusWord  uint16

	consoleTitle      string
	cursorVisible     bool
	cursorSize        uint32
	screenWidth       int
	screenHeight      int
	inputConsoleMode  uint32
	outputConsoleMode uint32
}

type frame struct {
	proc *Procedure
	pc   int
}

func NewMachine(stdin io.Reader, stdout io.Writer, stderr io.Writer) *Machine {
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	return &Machine{
		rawIn:         stdin,
		stdin:         bufio.NewReader(stdin),
		stdout:        stdout,
		stderr:        stderr,
		peekableInput: isPeekableInput(stdin),
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (m *Machine) SetArgs(args []string) {
	m.args = append([]string(nil), args...)
}

func (m *Machine) Run(program *Program) (int, error) {
	if err := program.Validate(); err != nil {
		return 1, err
	}
	m.program = program
	m.memory = make([]byte, len(program.Data)+(1<<20))
	copy(m.memory, program.Data)
	m.symbols = make(map[string]Symbol, len(program.Symbols))
	for _, symbol := range program.Symbols {
		m.symbols[strings.ToLower(symbol.Name)] = symbol
	}
	m.procs = make(map[string]*Procedure, len(program.Procedures))
	for i := range program.Procedures {
		proc := &program.Procedures[i]
		m.procs[strings.ToLower(proc.Name)] = proc
	}
	m.regs = [8]uint32{}
	m.regs[regESP] = uint32(len(m.memory))
	m.zf, m.sf, m.cf, m.of, m.af, m.pf, m.df = false, false, false, false, false, false, false
	m.exitCode = 0
	m.lastError = ""
	m.startTime = time.Now()
	m.colorAttr = 0x07
	m.cursorX, m.cursorY = 0, 0
	m.files = map[uint32]*os.File{}
	m.nextHandle = 4
	m.fpu = nil
	m.fpuControlWord = 0x037F
	m.fpuStatusWord = 0
	m.consoleTitle = ""
	m.cursorVisible = true
	m.cursorSize = 25
	m.screenWidth = 80
	m.screenHeight = 25
	m.inputConsoleMode = 0
	m.outputConsoleMode = 0

	entry := strings.ToLower(program.Entry)
	if entry == "" {
		entry = strings.ToLower(program.Procedures[0].Name)
	}
	mainProc := m.procs[entry]
	if mainProc == nil {
		return 1, fmt.Errorf("entry procedure %q not found", program.Entry)
	}

	frames := []frame{{proc: mainProc, pc: 0}}
	for len(frames) > 0 {
		current := &frames[len(frames)-1]
		if current.pc < 0 || current.pc >= len(current.proc.Instructions) {
			if current.proc != mainProc {
				_, _ = m.pop32()
			}
			frames = frames[:len(frames)-1]
			continue
		}
		inst := current.proc.Instructions[current.pc]
		current.pc++
		jumped, err := m.execute(&frames, current, inst)
		if err != nil {
			return 1, fmt.Errorf("%s:%d: %w", current.proc.Name, inst.Line, err)
		}
		if jumped {
			continue
		}
	}
	for _, file := range m.files {
		_ = file.Close()
	}
	return m.exitCode, nil
}

func (m *Machine) execute(frames *[]frame, current *frame, inst Instruction) (bool, error) {
	switch strings.ToLower(inst.Op) {
	case "mov":
		return false, m.execMov(inst)
	case "lea":
		return false, m.execLea(inst)
	case "add":
		return false, m.execAddSub(inst, false)
	case "sub":
		return false, m.execAddSub(inst, true)
	case "xor":
		return false, m.execLogic(inst, "xor")
	case "and":
		return false, m.execLogic(inst, "and")
	case "or":
		return false, m.execLogic(inst, "or")
	case "test":
		return false, m.execTest(inst)
	case "inc":
		return false, m.execIncDec(inst, false)
	case "dec":
		return false, m.execIncDec(inst, true)
	case "cmp":
		return false, m.execCmp(inst)
	case "mul":
		return false, m.execMul(inst, false)
	case "imul":
		return false, m.execIMul(inst)
	case "div":
		return false, m.execDiv(inst, false)
	case "idiv":
		return false, m.execDiv(inst, true)
	case "neg":
		return false, m.execNeg(inst)
	case "not":
		return false, m.execNot(inst)
	case "shl", "sal":
		return false, m.execShift(inst, "shl")
	case "shr":
		return false, m.execShift(inst, "shr")
	case "sar":
		return false, m.execShift(inst, "sar")
	case "movzx":
		return false, m.execMovX(inst, false)
	case "movsx":
		return false, m.execMovX(inst, true)
	case "push":
		return false, m.execPush(inst)
	case "pop":
		return false, m.execPop(inst)
	case "pushad":
		return false, m.execPushad()
	case "popad":
		return false, m.execPopad()
	case "pushfd":
		return false, m.execPushfd()
	case "popfd":
		return false, m.execPopfd()
	case "leave":
		return false, m.execLeave()
	case "finit":
		return false, m.execFInit()
	case "fld":
		return false, m.execFLD(inst)
	case "fld1":
		return false, m.execFLD1()
	case "fldz":
		return false, m.execFLDZ()
	case "fild":
		return false, m.execFILD(inst)
	case "fst":
		return false, m.execFST(inst, false)
	case "fstp":
		return false, m.execFST(inst, true)
	case "fstsw":
		return false, m.execFNSTSW(inst)
	case "fist":
		return false, m.execFIST(inst)
	case "fadd":
		return false, m.execFArithmetic(inst, "add")
	case "fsub":
		return false, m.execFArithmetic(inst, "sub")
	case "fsubr":
		return false, m.execFReverseArithmetic(inst, "sub")
	case "fmul":
		return false, m.execFArithmetic(inst, "mul")
	case "fdiv":
		return false, m.execFArithmetic(inst, "div")
	case "fdivr":
		return false, m.execFReverseArithmetic(inst, "div")
	case "fchs":
		return false, m.execFUnary("chs")
	case "fabs":
		return false, m.execFUnary("abs")
	case "fsqrt":
		return false, m.execFUnary("sqrt")
	case "frndint":
		return false, m.execFRNDINT()
	case "ftst":
		return false, m.execFTST()
	case "fcomi":
		return false, m.execFCOMI(inst)
	case "fcomp":
		return false, m.execFCOMP(inst)
	case "fclex":
		m.fpuStatusWord = 0
		return false, nil
	case "fwait":
		return false, nil
	case "fincstp":
		return false, m.execFINCSTP()
	case "fstcw":
		return false, m.execFSTCW(inst)
	case "fldcw":
		return false, m.execFLDCW(inst)
	case "fnstsw":
		return false, m.execFNSTSW(inst)
	case "sahf":
		return false, m.execSAHF()
	case "cld":
		m.df = false
		return false, nil
	case "std":
		m.df = true
		return false, nil
	case "lodsb":
		return false, m.execLods(1)
	case "lodsw":
		return false, m.execLods(2)
	case "lodsd":
		return false, m.execLods(4)
	case "stosb":
		return false, m.execStos(1)
	case "stosw":
		return false, m.execStos(2)
	case "stosd":
		return false, m.execStos(4)
	case "movsb":
		return false, m.execMovs(1)
	case "movsw":
		return false, m.execMovs(2)
	case "movsd":
		return false, m.execMovs(4)
	case "cmpsb":
		return false, m.execCmps(1)
	case "cmpsw":
		return false, m.execCmps(2)
	case "cmpsd":
		return false, m.execCmps(4)
	case "scasb":
		return false, m.execScas(1)
	case "scasw":
		return false, m.execScas(2)
	case "scasd":
		return false, m.execScas(4)
	case "rep_lodsb":
		return false, m.execRepeatString("rep", "lodsb")
	case "rep_lodsw":
		return false, m.execRepeatString("rep", "lodsw")
	case "rep_lodsd":
		return false, m.execRepeatString("rep", "lodsd")
	case "rep_stosb":
		return false, m.execRepeatString("rep", "stosb")
	case "rep_stosw":
		return false, m.execRepeatString("rep", "stosw")
	case "rep_stosd":
		return false, m.execRepeatString("rep", "stosd")
	case "rep_movsb":
		return false, m.execRepeatString("rep", "movsb")
	case "rep_movsw":
		return false, m.execRepeatString("rep", "movsw")
	case "rep_movsd":
		return false, m.execRepeatString("rep", "movsd")
	case "repe_cmpsb":
		return false, m.execRepeatString("repe", "cmpsb")
	case "repe_cmpsw":
		return false, m.execRepeatString("repe", "cmpsw")
	case "repe_cmpsd":
		return false, m.execRepeatString("repe", "cmpsd")
	case "repne_cmpsb":
		return false, m.execRepeatString("repne", "cmpsb")
	case "repne_cmpsw":
		return false, m.execRepeatString("repne", "cmpsw")
	case "repne_cmpsd":
		return false, m.execRepeatString("repne", "cmpsd")
	case "repe_scasb":
		return false, m.execRepeatString("repe", "scasb")
	case "repe_scasw":
		return false, m.execRepeatString("repe", "scasw")
	case "repe_scasd":
		return false, m.execRepeatString("repe", "scasd")
	case "repne_scasb":
		return false, m.execRepeatString("repne", "scasb")
	case "repne_scasw":
		return false, m.execRepeatString("repne", "scasw")
	case "repne_scasd":
		return false, m.execRepeatString("repne", "scasd")
	case "xchg":
		return false, m.execXchg(inst)
	case "cdq":
		return false, m.execCDQ()
	case "nop":
		return false, nil
	case "jmp":
		return true, m.jump(current, inst)
	case "je", "jz":
		return m.jumpIf(current, inst, m.zf)
	case "jne", "jnz":
		return m.jumpIf(current, inst, !m.zf)
	case "jl", "jnge":
		return m.jumpIf(current, inst, m.sf != m.of)
	case "jle", "jng":
		return m.jumpIf(current, inst, m.zf || (m.sf != m.of))
	case "jg", "jnle":
		return m.jumpIf(current, inst, !m.zf && (m.sf == m.of))
	case "jge", "jnl":
		return m.jumpIf(current, inst, m.sf == m.of)
	case "jb", "jc", "jnae":
		return m.jumpIf(current, inst, m.cf)
	case "jbe", "jna":
		return m.jumpIf(current, inst, m.cf || m.zf)
	case "ja", "jnbe":
		return m.jumpIf(current, inst, !m.cf && !m.zf)
	case "jae", "jnc", "jnb":
		return m.jumpIf(current, inst, !m.cf)
	case "js":
		return m.jumpIf(current, inst, m.sf)
	case "jns":
		return m.jumpIf(current, inst, !m.sf)
	case "jo":
		return m.jumpIf(current, inst, m.of)
	case "jno":
		return m.jumpIf(current, inst, !m.of)
	case "jcxz", "jecxz":
		return m.jumpIf(current, inst, m.regs[regECX] == 0)
	case "loop":
		m.regs[regECX]--
		return m.jumpIf(current, inst, m.regs[regECX] != 0)
	case "call":
		return false, m.execCall(frames, inst)
	case "invoke":
		return false, m.execInvoke(inst)
	case "ret":
		return true, m.execRet(frames, inst)
	case "exit":
		return true, m.execExit(frames, inst)
	case "mwrite":
		return false, m.writeLiteral(inst, false)
	case "mwriteln":
		return false, m.writeLiteral(inst, true)
	case "mwritespace":
		return false, m.execWriteSpace(inst)
	case "mdumpmem":
		return false, m.execMacroDumpMem(inst)
	case "mdump":
		return false, m.execMacroDump(inst)
	case "mshow":
		return false, m.execMacroShow(inst)
	default:
		return false, fmt.Errorf("unsupported instruction %q", inst.Op)
	}
}

func (m *Machine) execMov(inst Instruction) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("mov expects two operands")
	}
	width, err := m.operandWidth(inst.Args[0], inst.Args[1], 4)
	if err != nil {
		return err
	}
	value, _, err := m.resolveValue(inst.Args[1], width)
	if err != nil {
		return err
	}
	return m.assign(inst.Args[0], value, width)
}

func (m *Machine) execLea(inst Instruction) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("lea expects two operands")
	}
	if inst.Args[0].Kind != "reg" || inst.Args[1].Kind != "mem" {
		return fmt.Errorf("lea requires register destination and memory source")
	}
	addr, err := m.resolveAddress(inst.Args[1])
	if err != nil {
		return err
	}
	return m.assign(inst.Args[0], addr, 4)
}

func (m *Machine) execAddSub(inst Instruction, subtract bool) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("%s expects two operands", inst.Op)
	}
	width, err := m.operandWidth(inst.Args[0], inst.Args[1], 4)
	if err != nil {
		return err
	}
	left, _, err := m.resolveValue(inst.Args[0], width)
	if err != nil {
		return err
	}
	right, _, err := m.resolveValue(inst.Args[1], width)
	if err != nil {
		return err
	}
	var result uint32
	if subtract {
		result = left - right
		m.updateSubFlags(left, right, result, width)
	} else {
		result = left + right
		m.updateAddFlags(left, right, result, width)
	}
	return m.assign(inst.Args[0], result, width)
}

func (m *Machine) execLogic(inst Instruction, op string) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("%s expects two operands", op)
	}
	width, err := m.operandWidth(inst.Args[0], inst.Args[1], 4)
	if err != nil {
		return err
	}
	left, _, err := m.resolveValue(inst.Args[0], width)
	if err != nil {
		return err
	}
	right, _, err := m.resolveValue(inst.Args[1], width)
	if err != nil {
		return err
	}
	var result uint32
	switch op {
	case "xor":
		result = left ^ right
	case "and":
		result = left & right
	case "or":
		result = left | right
	}
	m.assignLogicFlags(result, width)
	m.cf = false
	m.of = false
	return m.assign(inst.Args[0], result, width)
}

func (m *Machine) execTest(inst Instruction) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("test expects two operands")
	}
	width, err := m.operandWidth(inst.Args[0], inst.Args[1], 4)
	if err != nil {
		return err
	}
	left, _, err := m.resolveValue(inst.Args[0], width)
	if err != nil {
		return err
	}
	right, _, err := m.resolveValue(inst.Args[1], width)
	if err != nil {
		return err
	}
	result := left & right
	m.assignLogicFlags(result, width)
	m.cf = false
	m.of = false
	return nil
}

func (m *Machine) execIncDec(inst Instruction, decrement bool) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("%s expects one operand", inst.Op)
	}
	width, err := m.operandWidth(inst.Args[0], Operand{}, 4)
	if err != nil {
		return err
	}
	value, _, err := m.resolveValue(inst.Args[0], width)
	if err != nil {
		return err
	}
	carry := m.cf
	var result uint32
	if decrement {
		result = value - 1
		m.updateSubFlags(value, 1, result, width)
	} else {
		result = value + 1
		m.updateAddFlags(value, 1, result, width)
	}
	m.cf = carry
	return m.assign(inst.Args[0], result, width)
}

func (m *Machine) execCmp(inst Instruction) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("cmp expects two operands")
	}
	width, err := m.operandWidth(inst.Args[0], inst.Args[1], 4)
	if err != nil {
		return err
	}
	left, _, err := m.resolveValue(inst.Args[0], width)
	if err != nil {
		return err
	}
	right, _, err := m.resolveValue(inst.Args[1], width)
	if err != nil {
		return err
	}
	m.updateSubFlags(left, right, left-right, width)
	return nil
}

func (m *Machine) execMul(inst Instruction, signed bool) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("%s expects one operand", inst.Op)
	}
	width, err := m.operandWidth(inst.Args[0], Operand{}, 4)
	if err != nil {
		return err
	}
	left, _, err := m.resolveValue(Operand{Kind: "reg", Text: "eax"}, width)
	if err != nil {
		return err
	}
	right, _, err := m.resolveValue(inst.Args[0], width)
	if err != nil {
		return err
	}
	if signed {
		product := int64(signExtend(left, width)) * int64(signExtend(right, width))
		m.regs[regEAX] = uint32(product)
		m.regs[regEDX] = uint32(uint64(product) >> 32)
	} else {
		product := uint64(left) * uint64(right)
		m.regs[regEAX] = uint32(product)
		m.regs[regEDX] = uint32(product >> 32)
	}
	m.cf = m.regs[regEDX] != 0
	m.of = m.cf
	m.zf = m.regs[regEAX] == 0 && m.regs[regEDX] == 0
	m.sf = signBit(m.regs[regEAX], 4)
	m.pf = parity8(byte(m.regs[regEAX]))
	m.af = false
	return nil
}

func (m *Machine) execIMul(inst Instruction) error {
	switch len(inst.Args) {
	case 1:
		return m.execMul(inst, true)
	case 2, 3:
		dest := inst.Args[0]
		src := inst.Args[1]
		other := src
		if len(inst.Args) == 3 {
			other = inst.Args[2]
		}
		width, err := m.operandWidth(dest, src, 4)
		if err != nil {
			return err
		}
		left, _, err := m.resolveValue(src, width)
		if err != nil {
			return err
		}
		right, _, err := m.resolveValue(other, width)
		if err != nil {
			return err
		}
		product := int64(signExtend(left, width)) * int64(signExtend(right, width))
		result := uint32(product)
		mask := widthMask(width)
		truncated := result & mask
		signExtended := int64(signExtend(truncated, width))
		m.cf = product != signExtended
		m.of = m.cf
		m.assignLogicFlags(truncated, width)
		return m.assign(dest, truncated, width)
	default:
		return fmt.Errorf("imul supports one, two, or three operands")
	}
}

func (m *Machine) execDiv(inst Instruction, signed bool) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("%s expects one operand", inst.Op)
	}
	width, err := m.operandWidth(inst.Args[0], Operand{}, 4)
	if err != nil {
		return err
	}
	divisor, _, err := m.resolveValue(inst.Args[0], width)
	if err != nil {
		return err
	}
	if divisor == 0 {
		return fmt.Errorf("division by zero")
	}
	if signed {
		dividend := int64(int32(m.regs[regEAX]))
		if width == 2 {
			dividend = int64(int16(m.readRegister("ax")))
		}
		if width == 4 {
			dividend = int64(int32(m.regs[regEDX]))<<32 | int64(uint32(m.regs[regEAX]))
		}
		quotient := dividend / int64(int32(divisor))
		remainder := dividend % int64(int32(divisor))
		m.regs[regEAX] = uint32(quotient)
		m.regs[regEDX] = uint32(remainder)
	} else {
		dividend := uint64(m.regs[regEAX])
		if width == 4 {
			dividend = uint64(m.regs[regEDX])<<32 | uint64(m.regs[regEAX])
		}
		m.regs[regEAX] = uint32(dividend / uint64(divisor))
		m.regs[regEDX] = uint32(dividend % uint64(divisor))
	}
	return nil
}

func (m *Machine) execNeg(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("neg expects one operand")
	}
	width, err := m.operandWidth(inst.Args[0], Operand{}, 4)
	if err != nil {
		return err
	}
	value, _, err := m.resolveValue(inst.Args[0], width)
	if err != nil {
		return err
	}
	result := uint32(0) - value
	m.updateSubFlags(0, value, result, width)
	return m.assign(inst.Args[0], result, width)
}

func (m *Machine) execNot(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("not expects one operand")
	}
	width, err := m.operandWidth(inst.Args[0], Operand{}, 4)
	if err != nil {
		return err
	}
	value, _, err := m.resolveValue(inst.Args[0], width)
	if err != nil {
		return err
	}
	return m.assign(inst.Args[0], ^value, width)
}

func (m *Machine) execShift(inst Instruction, op string) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("%s expects two operands", op)
	}
	width, err := m.operandWidth(inst.Args[0], Operand{}, 4)
	if err != nil {
		return err
	}
	value, _, err := m.resolveValue(inst.Args[0], width)
	if err != nil {
		return err
	}
	count, _, err := m.resolveValue(inst.Args[1], 1)
	if err != nil {
		return err
	}
	count &= 0x1F
	if count == 0 {
		return nil
	}
	var result uint32
	switch op {
	case "shl":
		result = value << count
		m.cf = ((value << (count - 1)) & signMask(width)) != 0
	case "shr":
		result = value >> count
		m.cf = ((value >> (count - 1)) & 1) != 0
	case "sar":
		result = uint32(int32(signExtend(value, width)) >> count)
		m.cf = ((value >> (count - 1)) & 1) != 0
	}
	result = truncate(result, width)
	m.assignLogicFlags(result, width)
	if count == 1 {
		switch op {
		case "shl":
			m.of = signBit(result, width) != m.cf
		case "shr":
			m.of = signBit(value, width)
		case "sar":
			m.of = false
		}
	}
	return m.assign(inst.Args[0], result, width)
}

func (m *Machine) execMovX(inst Instruction, sign bool) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("%s expects two operands", inst.Op)
	}
	if inst.Args[0].Kind != "reg" {
		return fmt.Errorf("%s destination must be a register", inst.Op)
	}
	srcWidth, err := m.operandWidth(inst.Args[1], Operand{}, 1)
	if err != nil {
		return err
	}
	value, _, err := m.resolveValue(inst.Args[1], srcWidth)
	if err != nil {
		return err
	}
	if sign {
		value = uint32(signExtend(value, srcWidth))
	}
	return m.assign(inst.Args[0], value, registerWidth(inst.Args[0].Text))
}

func (m *Machine) execPush(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("push expects one operand")
	}
	width, err := m.operandWidth(inst.Args[0], Operand{}, 4)
	if err != nil {
		return err
	}
	value, _, err := m.resolveValue(inst.Args[0], width)
	if err != nil {
		return err
	}
	return m.push32(value)
}

func (m *Machine) execPop(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("pop expects one operand")
	}
	value, err := m.pop32()
	if err != nil {
		return err
	}
	return m.assign(inst.Args[0], value, 4)
}

func (m *Machine) execPushad() error {
	originalESP := m.regs[regESP]
	values := []uint32{
		m.regs[regEAX],
		m.regs[regECX],
		m.regs[regEDX],
		m.regs[regEBX],
		originalESP,
		m.regs[regEBP],
		m.regs[regESI],
		m.regs[regEDI],
	}
	for _, value := range values {
		if err := m.push32(value); err != nil {
			return err
		}
	}
	return nil
}

func (m *Machine) execPopad() error {
	order := []string{"edi", "esi", "ebp", "", "ebx", "edx", "ecx", "eax"}
	for _, reg := range order {
		value, err := m.pop32()
		if err != nil {
			return err
		}
		if reg != "" {
			m.writeRegister(reg, value)
		}
	}
	return nil
}

func (m *Machine) execPushfd() error {
	return m.push32(m.currentEFlags())
}

func (m *Machine) execPopfd() error {
	value, err := m.pop32()
	if err != nil {
		return err
	}
	m.applyEFlags(value)
	return nil
}

func (m *Machine) execLeave() error {
	m.regs[regESP] = m.regs[regEBP]
	value, err := m.pop32()
	if err != nil {
		return err
	}
	m.regs[regEBP] = value
	return nil
}

func (m *Machine) execFInit() error {
	m.fpu = nil
	m.fpuControlWord = 0x037F
	m.fpuStatusWord = 0
	return nil
}

func (m *Machine) execFLDZ() error {
	return m.fpuPush(0)
}

func (m *Machine) execFLD1() error {
	return m.fpuPush(1)
}

func (m *Machine) execFLD(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("fld expects one operand")
	}
	value, err := m.readFloatOperand(inst.Args[0])
	if err != nil {
		return err
	}
	return m.fpuPush(value)
}

func (m *Machine) execFILD(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("fild expects one operand")
	}
	value, err := m.readSignedIntegerOperand(inst.Args[0])
	if err != nil {
		return err
	}
	return m.fpuPush(float64(value))
}

func (m *Machine) execFST(inst Instruction, pop bool) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("%s expects one operand", inst.Op)
	}
	value, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	if err := m.writeFloatOperand(inst.Args[0], value); err != nil {
		return err
	}
	if pop {
		_, err = m.fpuPop()
		return err
	}
	return nil
}

func (m *Machine) execFIST(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("fist expects one operand")
	}
	value, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	rounded := m.roundFloatToInt(value)
	width, err := m.integerOperandWidth(inst.Args[0])
	if err != nil {
		return err
	}
	switch width {
	case 2:
		return m.assign(inst.Args[0], uint32(int16(rounded)), 2)
	case 4:
		return m.assign(inst.Args[0], uint32(int32(rounded)), 4)
	default:
		return fmt.Errorf("unsupported integer width %d for fist", width)
	}
}

func (m *Machine) execFArithmetic(inst Instruction, op string) error {
	apply := func(left, right float64) float64 {
		switch op {
		case "add":
			return left + right
		case "sub":
			return left - right
		case "mul":
			return left * right
		case "div":
			return left / right
		default:
			return left
		}
	}
	switch len(inst.Args) {
	case 0:
		left, err := m.fpuPeek(1)
		if err != nil {
			return err
		}
		right, err := m.fpuPeek(0)
		if err != nil {
			return err
		}
		if err := m.fpuSet(1, apply(left, right)); err != nil {
			return err
		}
		_, err = m.fpuPop()
		return err
	case 1:
		left, err := m.fpuPeek(0)
		if err != nil {
			return err
		}
		right, err := m.readFloatOperand(inst.Args[0])
		if err != nil {
			return err
		}
		return m.fpuSet(0, apply(left, right))
	case 2:
		if inst.Args[0].Kind != "st" || inst.Args[1].Kind != "st" {
			return fmt.Errorf("%s expects ST operands when using two arguments", inst.Op)
		}
		left, err := m.fpuPeek(int(inst.Args[0].Value))
		if err != nil {
			return err
		}
		right, err := m.fpuPeek(int(inst.Args[1].Value))
		if err != nil {
			return err
		}
		return m.fpuSet(int(inst.Args[0].Value), apply(left, right))
	default:
		return fmt.Errorf("%s expects zero, one, or two operands", inst.Op)
	}
}

func (m *Machine) execFReverseArithmetic(inst Instruction, op string) error {
	apply := func(left, right float64) float64 {
		switch op {
		case "sub":
			return right - left
		case "div":
			return right / left
		default:
			return right
		}
	}
	switch len(inst.Args) {
	case 0:
		left, err := m.fpuPeek(1)
		if err != nil {
			return err
		}
		right, err := m.fpuPeek(0)
		if err != nil {
			return err
		}
		if err := m.fpuSet(1, apply(left, right)); err != nil {
			return err
		}
		_, err = m.fpuPop()
		return err
	case 1:
		left, err := m.fpuPeek(0)
		if err != nil {
			return err
		}
		right, err := m.readFloatOperand(inst.Args[0])
		if err != nil {
			return err
		}
		return m.fpuSet(0, apply(left, right))
	case 2:
		if inst.Args[0].Kind != "st" || inst.Args[1].Kind != "st" {
			return fmt.Errorf("%s expects ST operands when using two arguments", inst.Op)
		}
		left, err := m.fpuPeek(int(inst.Args[0].Value))
		if err != nil {
			return err
		}
		right, err := m.fpuPeek(int(inst.Args[1].Value))
		if err != nil {
			return err
		}
		return m.fpuSet(int(inst.Args[0].Value), apply(left, right))
	default:
		return fmt.Errorf("%s expects zero, one, or two operands", inst.Op)
	}
}

func (m *Machine) execFUnary(op string) error {
	value, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	switch op {
	case "chs":
		value = -value
	case "abs":
		value = math.Abs(value)
	case "sqrt":
		value = math.Sqrt(value)
	default:
		return fmt.Errorf("unsupported floating-point unary op %q", op)
	}
	return m.fpuSet(0, value)
}

func (m *Machine) execFRNDINT() error {
	value, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	return m.fpuSet(0, float64(m.roundFloatToInt(value)))
}

func (m *Machine) execFTST() error {
	value, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	m.setFPUCompareStatus(value, 0)
	return nil
}

func (m *Machine) execFINCSTP() error {
	if len(m.fpu) <= 1 {
		return nil
	}
	top := m.fpu[len(m.fpu)-1]
	copy(m.fpu[1:], m.fpu[:len(m.fpu)-1])
	m.fpu[0] = top
	return nil
}

func (m *Machine) execFCOMI(inst Instruction) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("fcomi expects two operands")
	}
	left, err := m.readFloatOperand(inst.Args[0])
	if err != nil {
		return err
	}
	right, err := m.readFloatOperand(inst.Args[1])
	if err != nil {
		return err
	}
	m.setCPUFloatCompareFlags(left, right)
	return nil
}

func (m *Machine) execFCOMP(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("fcomp expects one operand")
	}
	left, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	right, err := m.readFloatOperand(inst.Args[0])
	if err != nil {
		return err
	}
	m.setFPUCompareStatus(left, right)
	_, err = m.fpuPop()
	return err
}

func (m *Machine) execFSTCW(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("fstcw expects one operand")
	}
	return m.assign(inst.Args[0], uint32(m.fpuControlWord), 2)
}

func (m *Machine) execFLDCW(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("fldcw expects one operand")
	}
	value, _, err := m.resolveValue(inst.Args[0], 2)
	if err != nil {
		return err
	}
	m.fpuControlWord = uint16(value)
	return nil
}

func (m *Machine) execFNSTSW(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("fnstsw expects one operand")
	}
	return m.assign(inst.Args[0], uint32(m.fpuStatusWord), 2)
}

func (m *Machine) execSAHF() error {
	ah := byte(m.readRegister("ah"))
	m.sf = ah&(1<<7) != 0
	m.zf = ah&(1<<6) != 0
	m.af = ah&(1<<4) != 0
	m.pf = ah&(1<<2) != 0
	m.cf = ah&(1<<0) != 0
	return nil
}

func (m *Machine) execLods(width int) error {
	value, err := m.readMemory(m.regs[regESI], width)
	if err != nil {
		return err
	}
	m.writeAccumulator(width, value)
	m.advanceStringRegs(width, true, false)
	return nil
}

func (m *Machine) execStos(width int) error {
	if err := m.writeMemory(m.regs[regEDI], truncate(m.readAccumulator(width), width), width); err != nil {
		return err
	}
	m.advanceStringRegs(width, false, true)
	return nil
}

func (m *Machine) execMovs(width int) error {
	value, err := m.readMemory(m.regs[regESI], width)
	if err != nil {
		return err
	}
	if err := m.writeMemory(m.regs[regEDI], value, width); err != nil {
		return err
	}
	m.advanceStringRegs(width, true, true)
	return nil
}

func (m *Machine) execCmps(width int) error {
	left, err := m.readMemory(m.regs[regESI], width)
	if err != nil {
		return err
	}
	right, err := m.readMemory(m.regs[regEDI], width)
	if err != nil {
		return err
	}
	m.updateSubFlags(left, right, truncate(left-right, width), width)
	m.advanceStringRegs(width, true, true)
	return nil
}

func (m *Machine) execScas(width int) error {
	left := m.readAccumulator(width)
	right, err := m.readMemory(m.regs[regEDI], width)
	if err != nil {
		return err
	}
	m.updateSubFlags(left, right, truncate(left-right, width), width)
	m.advanceStringRegs(width, false, true)
	return nil
}

func (m *Machine) execRepeatString(prefix, op string) error {
	for m.regs[regECX] != 0 {
		if err := m.execStringOp(op); err != nil {
			return err
		}
		m.regs[regECX]--
		switch prefix {
		case "repe":
			if !strings.HasPrefix(op, "cmps") && !strings.HasPrefix(op, "scas") {
				continue
			}
			if !m.zf {
				return nil
			}
		case "repne":
			if !strings.HasPrefix(op, "cmps") && !strings.HasPrefix(op, "scas") {
				continue
			}
			if m.zf {
				return nil
			}
		}
	}
	return nil
}

func (m *Machine) execStringOp(op string) error {
	switch op {
	case "lodsb":
		return m.execLods(1)
	case "lodsw":
		return m.execLods(2)
	case "lodsd":
		return m.execLods(4)
	case "stosb":
		return m.execStos(1)
	case "stosw":
		return m.execStos(2)
	case "stosd":
		return m.execStos(4)
	case "movsb":
		return m.execMovs(1)
	case "movsw":
		return m.execMovs(2)
	case "movsd":
		return m.execMovs(4)
	case "cmpsb":
		return m.execCmps(1)
	case "cmpsw":
		return m.execCmps(2)
	case "cmpsd":
		return m.execCmps(4)
	case "scasb":
		return m.execScas(1)
	case "scasw":
		return m.execScas(2)
	case "scasd":
		return m.execScas(4)
	default:
		return fmt.Errorf("unsupported repeated string instruction %q", op)
	}
}

func (m *Machine) advanceStringRegs(width int, advanceESI, advanceEDI bool) {
	step := uint32(width)
	if m.df {
		step = ^step + 1
	}
	if advanceESI {
		m.regs[regESI] += step
	}
	if advanceEDI {
		m.regs[regEDI] += step
	}
}

func (m *Machine) readAccumulator(width int) uint32 {
	switch width {
	case 1:
		return m.readRegister("al")
	case 2:
		return m.readRegister("ax")
	default:
		return m.regs[regEAX]
	}
}

func (m *Machine) writeAccumulator(width int, value uint32) {
	switch width {
	case 1:
		m.writeRegister("al", value)
	case 2:
		m.writeRegister("ax", value)
	default:
		m.regs[regEAX] = truncate(value, 4)
	}
}

func (m *Machine) execXchg(inst Instruction) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("xchg expects two operands")
	}
	width, err := m.operandWidth(inst.Args[0], inst.Args[1], 4)
	if err != nil {
		return err
	}
	left, _, err := m.resolveValue(inst.Args[0], width)
	if err != nil {
		return err
	}
	right, _, err := m.resolveValue(inst.Args[1], width)
	if err != nil {
		return err
	}
	if err := m.assign(inst.Args[0], right, width); err != nil {
		return err
	}
	return m.assign(inst.Args[1], left, width)
}

func (m *Machine) execCDQ() error {
	if int32(m.regs[regEAX]) < 0 {
		m.regs[regEDX] = 0xFFFFFFFF
	} else {
		m.regs[regEDX] = 0
	}
	return nil
}

func (m *Machine) execCall(frames *[]frame, inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("call expects one operand")
	}
	target := strings.ToLower(inst.Args[0].Text)
	if target == "" {
		return fmt.Errorf("call target missing")
	}
	if builtin := m.dispatchCallBuiltin(target); builtin != nil {
		return builtin()
	}
	proc := m.procs[target]
	if proc == nil {
		return fmt.Errorf("unknown procedure %q", inst.Args[0].Text)
	}
	if err := m.push32(0xC0DEF00D); err != nil {
		return err
	}
	*frames = append(*frames, frame{proc: proc, pc: 0})
	return nil
}

func (m *Machine) execInvoke(inst Instruction) error {
	if len(inst.Args) == 0 || inst.Args[0].Kind != "name" {
		return fmt.Errorf("invoke requires a target name")
	}
	name := strings.ToLower(inst.Args[0].Text)
	if builtin := m.dispatchInvokeBuiltin(name); builtin != nil {
		return builtin(inst.Args[1:])
	}
	if len(inst.Args) == 1 {
		return m.execCall(&[]frame{}, Instruction{Op: "call", Args: []Operand{{Kind: "name", Text: inst.Args[0].Text}}})
	}
	return fmt.Errorf("INVOKE with arguments is only supported for built-in procedures")
}

func (m *Machine) execRet(frames *[]frame, inst Instruction) error {
	if len(*frames) == 0 {
		return fmt.Errorf("empty call stack")
	}
	_, err := m.pop32()
	if err != nil {
		return err
	}
	if len(inst.Args) == 1 {
		adjust, _, err := m.resolveValue(inst.Args[0], 4)
		if err != nil {
			return err
		}
		m.regs[regESP] += adjust
	}
	*frames = (*frames)[:len(*frames)-1]
	return nil
}

func (m *Machine) execExit(frames *[]frame, inst Instruction) error {
	code := 0
	if len(inst.Args) == 1 {
		value, _, err := m.resolveValue(inst.Args[0], 4)
		if err != nil {
			return err
		}
		code = int(int32(value))
	}
	m.exitCode = code
	*frames = (*frames)[:0]
	return nil
}

func (m *Machine) writeLiteral(inst Instruction, addNewline bool) error {
	if len(inst.Args) != 1 || inst.Args[0].Kind != "string" {
		return fmt.Errorf("%s expects one string literal", inst.Op)
	}
	if _, err := io.WriteString(m.stdout, inst.Args[0].Text); err != nil {
		return err
	}
	if addNewline {
		_, err := io.WriteString(m.stdout, "\r\n")
		return err
	}
	return nil
}

func (m *Machine) execWriteSpace(inst Instruction) error {
	count := int64(1)
	if len(inst.Args) == 1 {
		count = inst.Args[0].Value
	}
	if count < 0 {
		count = 0
	}
	_, err := io.WriteString(m.stdout, strings.Repeat(" ", int(count)))
	return err
}

func (m *Machine) execMacroDumpMem(inst Instruction) error {
	if len(inst.Args) != 3 {
		return fmt.Errorf("mdumpmem expects three operands")
	}
	addr, _, err := m.resolveValue(inst.Args[0], 4)
	if err != nil {
		return err
	}
	count, _, err := m.resolveValue(inst.Args[1], 4)
	if err != nil {
		return err
	}
	size, _, err := m.resolveValue(inst.Args[2], 4)
	if err != nil {
		return err
	}
	return m.dumpMemory(addr, count, size)
}

func (m *Machine) execMacroDump(inst Instruction) error {
	if len(inst.Args) < 1 {
		return fmt.Errorf("mdump expects at least one operand")
	}
	showLabel := len(inst.Args) > 1 && inst.Args[1].Value != 0
	if showLabel {
		if _, err := io.WriteString(m.stdout, "\r\nVariable name: "+inst.Args[0].Text+"\r\n"); err != nil {
			return err
		}
	}
	symbol, ok := m.symbols[strings.ToLower(inst.Args[0].Text)]
	if !ok {
		return fmt.Errorf("unknown symbol %q", inst.Args[0].Text)
	}
	return m.dumpMemory(symbol.Address, uint32(symbol.Length), uint32(symbol.ElemSize))
}

func (m *Machine) execMacroShow(inst Instruction) error {
	if len(inst.Args) != 2 || inst.Args[1].Kind != "string" {
		return fmt.Errorf("mshow expects a value and format string")
	}
	value, width, err := m.resolveValue(inst.Args[0], 4)
	if err != nil {
		return err
	}
	name := inst.Args[0].Text
	if name == "" {
		name = inst.Args[0].Kind
	}
	if _, err := io.WriteString(m.stdout, "  "+name+" = "); err != nil {
		return err
	}
	for _, ch := range inst.Args[1].Text {
		switch ch {
		case 'H', 'h':
			if _, err := io.WriteString(m.stdout, formatHex(value, width)+"h  "); err != nil {
				return err
			}
		case 'D', 'd':
			if _, err := fmt.Fprint(m.stdout, truncate(value, width), "d  "); err != nil {
				return err
			}
		case 'I', 'i':
			if _, err := fmt.Fprint(m.stdout, signExtend(value, width), "d  "); err != nil {
				return err
			}
		case 'B', 'b':
			if _, err := io.WriteString(m.stdout, formatBin(value, width)+"b  "); err != nil {
				return err
			}
		case 'N', 'n':
			if _, err := io.WriteString(m.stdout, "\r\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Machine) jump(current *frame, inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("%s expects one target", inst.Op)
	}
	target := strings.ToLower(inst.Args[0].Text)
	index, ok := current.proc.Labels[target]
	if !ok {
		return fmt.Errorf("label %q not found", inst.Args[0].Text)
	}
	current.pc = index
	return nil
}

func (m *Machine) jumpIf(current *frame, inst Instruction, condition bool) (bool, error) {
	if !condition {
		return false, nil
	}
	return true, m.jump(current, inst)
}

func (m *Machine) dispatchCallBuiltin(name string) func() error {
	switch name {
	case "writestring":
		return m.builtinWriteString
	case "crlf":
		return m.builtinCrlf
	case "dumpregs":
		return m.builtinDumpRegs
	case "dumpmem":
		return m.builtinDumpMem
	case "writeint":
		return m.builtinWriteInt
	case "writedec":
		return m.builtinWriteDec
	case "writechar":
		return m.builtinWriteChar
	case "writehex":
		return m.builtinWriteHex
	case "writehexb":
		return m.builtinWriteHexB
	case "writebin":
		return m.builtinWriteBin
	case "writebinb":
		return m.builtinWriteBinB
	case "readint":
		return m.builtinReadInt
	case "readdec":
		return m.builtinReadDec
	case "readhex":
		return m.builtinReadHex
	case "readstring":
		return m.builtinReadString
	case "readchar":
		return m.builtinReadChar
	case "readkey":
		return m.builtinReadKey
	case "readkeyflush":
		return m.builtinReadKeyFlush
	case "clrscr":
		return m.builtinClrscr
	case "waitmsg":
		return m.builtinWaitMsg
	case "delay":
		return m.builtinDelay
	case "gotoxy":
		return m.builtinGotoxy
	case "getmaxxy":
		return m.builtinGetMaxXY
	case "gettextcolor":
		return m.builtinGetTextColor
	case "settextcolor":
		return m.builtinSetTextColor
	case "randomize":
		return m.builtinRandomize
	case "random32":
		return m.builtinRandom32
	case "randomrange":
		return m.builtinRandomRange
	case "getmseconds":
		return m.builtinGetMseconds
	case "getcommandtail":
		return m.builtinGetCommandTail
	case "readfloat":
		return m.builtinReadFloat
	case "writefloat":
		return m.builtinWriteFloat
	case "showfpustack":
		return m.builtinShowFPUStack
	case "isdigit":
		return m.builtinIsDigit
	case "parseinteger32":
		return m.builtinParseInteger32
	case "parsedecimal32":
		return m.builtinParseDecimal32
	case "strlen", "strlength", "str_length":
		return m.builtinStrLengthCall
	case "str_copy":
		return m.builtinStrCopyCall
	case "str_compare":
		return m.builtinStrCompareCall
	case "str_trim":
		return m.builtinStrTrimCall
	case "str_ucase":
		return m.builtinStrUCaseCall
	case "createoutputfile":
		return m.builtinCreateOutputFile
	case "openinputfile":
		return m.builtinOpenInputFile
	case "closefile":
		return m.builtinCloseFile
	case "readfromfile":
		return m.builtinReadFromFile
	case "writetofile":
		return m.builtinWriteToFile
	case "writewindowsmsg":
		return m.builtinWriteWindowsMsg
	default:
		return nil
	}
}

func (m *Machine) dispatchInvokeBuiltin(name string) func([]Operand) error {
	switch name {
	case "gettickcount":
		return func(args []Operand) error { return m.builtinGetTickCount() }
	case "sleep":
		return m.invokeSleep
	case "str_length", "strlength":
		return m.invokeStrLength
	case "str_copy":
		return m.invokeStrCopy
	case "str_compare":
		return m.invokeStrCompare
	case "str_trim":
		return m.invokeStrTrim
	case "str_ucase":
		return m.invokeStrUCase
	case "createfile":
		return m.invokeCreateFile
	case "readfile":
		return m.invokeReadFile
	case "writefile":
		return m.invokeWriteFile
	case "closehandle":
		return m.invokeCloseHandle
	case "getstdhandle":
		return m.invokeGetStdHandle
	case "getconsolemode":
		return m.invokeGetConsoleMode
	case "setconsolemode":
		return m.invokeSetConsoleMode
	case "flushconsoleinputbuffer":
		return m.invokeFlushConsoleInputBuffer
	case "peekconsoleinput":
		return m.invokePeekConsoleInput
	case "readconsoleinput":
		return m.invokeReadConsoleInput
	case "getnumberofconsoleinputevents":
		return m.invokeGetNumberOfConsoleInputEvents
	case "getkeystate":
		return m.invokeGetKeyState
	case "writeconsole", "writeconsolea":
		return m.invokeWriteConsole
	case "readconsole", "readconsolea":
		return m.invokeReadConsole
	case "setconsolecursorposition":
		return m.invokeSetConsoleCursorPosition
	case "getconsolecursorinfo":
		return m.invokeGetConsoleCursorInfo
	case "setconsolecursorinfo":
		return m.invokeSetConsoleCursorInfo
	case "setconsolescreenbuffersize":
		return m.invokeSetConsoleScreenBufferSize
	case "setconsoletitle", "setconsoletitlea":
		return m.invokeSetConsoleTitle
	case "getconsolescreenbufferinfo":
		return m.invokeGetConsoleScreenBufferInfo
	case "getlocaltime":
		return m.invokeGetLocalTime
	case "getsystemtime":
		return m.invokeGetSystemTime
	case "writestackframe":
		return m.invokeWriteStackFrame
	case "writestackframename":
		return m.invokeWriteStackFrameName
	default:
		return nil
	}
}

func (m *Machine) builtinWriteString() error {
	_, err := m.stdout.Write(m.readCString(m.regs[regEDX]))
	return err
}

func (m *Machine) builtinCrlf() error {
	_, err := io.WriteString(m.stdout, "\r\n")
	return err
}

func (m *Machine) builtinDumpRegs() error {
	lines := []string{
		"",
		fmt.Sprintf("  EAX=%08X  EBX=%08X  ECX=%08X  EDX=%08X", m.regs[regEAX], m.regs[regEBX], m.regs[regECX], m.regs[regEDX]),
		fmt.Sprintf("  ESI=%08X  EDI=%08X  EBP=%08X  ESP=%08X", m.regs[regESI], m.regs[regEDI], m.regs[regEBP], m.regs[regESP]),
		fmt.Sprintf("  EIP=%08X  EFL=%08X  CF=%d  SF=%d  ZF=%d  OF=%d  AF=%d  PF=%d", 0, m.currentEFlags(), boolToInt(m.cf), boolToInt(m.sf), boolToInt(m.zf), boolToInt(m.of), boolToInt(m.af), boolToInt(m.pf)),
		"",
	}
	_, err := io.WriteString(m.stdout, strings.Join(lines, "\r\n"))
	return err
}

func (m *Machine) builtinDumpMem() error {
	return m.dumpMemory(m.regs[regESI], m.regs[regECX], m.regs[regEBX])
}

func (m *Machine) builtinWriteInt() error {
	_, err := fmt.Fprint(m.stdout, int32(m.regs[regEAX]))
	return err
}

func (m *Machine) builtinWriteDec() error {
	_, err := fmt.Fprint(m.stdout, m.regs[regEAX])
	return err
}

func (m *Machine) builtinWriteChar() error {
	_, err := m.stdout.Write([]byte{byte(m.regs[regEAX])})
	return err
}

func (m *Machine) builtinWriteHex() error {
	_, err := io.WriteString(m.stdout, formatHex(m.regs[regEAX], 4))
	return err
}

func (m *Machine) builtinWriteHexB() error {
	width := clampWidth(int(m.regs[regEBX]))
	if width == 0 {
		width = 4
	}
	_, err := io.WriteString(m.stdout, formatHex(m.regs[regEAX], width))
	return err
}

func (m *Machine) builtinWriteBin() error {
	_, err := io.WriteString(m.stdout, formatBin(m.regs[regEAX], 4))
	return err
}

func (m *Machine) builtinWriteBinB() error {
	width := clampWidth(int(m.regs[regEBX]))
	if width == 0 {
		width = 4
	}
	_, err := io.WriteString(m.stdout, formatBin(m.regs[regEAX], width))
	return err
}

func (m *Machine) builtinReadInt() error {
	line, err := m.readLine()
	if err != nil {
		return err
	}
	value, ok := parseSignedInput(line, 10, 32)
	if !ok {
		m.regs[regEAX] = 0
		m.of = true
		m.cf = true
		return nil
	}
	m.regs[regEAX] = uint32(int32(value))
	m.of = false
	m.cf = false
	return nil
}

func (m *Machine) builtinReadDec() error {
	line, err := m.readLine()
	if err != nil {
		return err
	}
	value, ok := parseUnsignedInput(line, 10, 32)
	if !ok {
		m.regs[regEAX] = 0
		m.of = true
		m.cf = true
		return nil
	}
	m.regs[regEAX] = uint32(value)
	m.of = false
	m.cf = false
	return nil
}

func (m *Machine) builtinReadHex() error {
	line, err := m.readLine()
	if err != nil {
		return err
	}
	value, ok := parseUnsignedInput(line, 16, 32)
	if !ok {
		m.regs[regEAX] = 0
		m.of = true
		m.cf = true
		return nil
	}
	m.regs[regEAX] = uint32(value)
	m.of = false
	m.cf = false
	return nil
}

func (m *Machine) builtinReadFloat() error {
	line, err := m.readLine()
	if err != nil {
		return err
	}
	value, parseErr := strconv.ParseFloat(strings.TrimSpace(line), 64)
	if parseErr != nil {
		value = 0
	}
	return m.fpuPush(value)
}

func (m *Machine) builtinWriteFloat() error {
	value, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	text := strings.ToUpper(strconv.FormatFloat(value, 'E', 12, 64))
	text = strings.Replace(text, "E+", "E+", 1)
	text = strings.Replace(text, "E-", "E-", 1)
	_, err = io.WriteString(m.stdout, text)
	return err
}

func (m *Machine) builtinShowFPUStack() error {
	if _, err := io.WriteString(m.stdout, "\r\n------ FPU Stack ------\r\n"); err != nil {
		return err
	}
	for i := 0; i < len(m.fpu); i++ {
		value, err := m.fpuPeek(i)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(m.stdout, "ST(%d): %s\r\n", i, strings.ToUpper(strconv.FormatFloat(value, 'E', 12, 64))); err != nil {
			return err
		}
	}
	return nil
}

func (m *Machine) builtinReadString() error {
	line, err := m.readLine()
	if err != nil {
		return err
	}
	addr := m.regs[regEDX]
	capacity := int(m.regs[regECX])
	if capacity > 0 {
		capacity--
	}
	if capacity < 0 {
		capacity = 0
	}
	if len(line) > capacity {
		line = line[:capacity]
	}
	if int(addr)+len(line)+1 > len(m.memory) {
		return fmt.Errorf("readstring destination exceeds memory")
	}
	copy(m.memory[addr:], []byte(line))
	m.memory[int(addr)+len(line)] = 0
	m.regs[regEAX] = uint32(len(line))
	return nil
}

func (m *Machine) builtinReadChar() error {
	b, err := m.stdin.ReadByte()
	if err != nil {
		return err
	}
	m.writeRegister("al", uint32(b))
	return nil
}

func (m *Machine) builtinReadKey() error {
	if !m.peekableInput {
		line, err := m.readLine()
		if err != nil || line == "" {
			m.zf = true
			m.writeRegister("al", 0)
			m.writeRegister("ah", 0)
			m.regs[regEBX] = 0
			m.regs[regEDX] = 0
			return nil
		}
		b := line[0]
		m.writeRegister("al", uint32(b))
		m.writeRegister("ah", 0)
		m.regs[regEDX] = uint32(b)
		m.regs[regEBX] = 0
		m.zf = false
		return nil
	}
	b, err := m.stdin.Peek(1)
	if err != nil || len(b) == 0 {
		m.zf = true
		m.writeRegister("al", 0)
		m.writeRegister("ah", 0)
		m.regs[regEBX] = 0
		m.regs[regEDX] = 0
		return nil
	}
	_, _ = m.stdin.ReadByte()
	m.writeRegister("al", uint32(b[0]))
	m.writeRegister("ah", 0)
	m.regs[regEDX] = uint32(b[0])
	m.regs[regEBX] = 0
	m.zf = false
	return nil
}

func (m *Machine) builtinReadKeyFlush() error {
	for m.peekableInput {
		b, err := m.stdin.Peek(1)
		if err != nil || len(b) == 0 {
			break
		}
		_, _ = m.stdin.ReadByte()
	}
	return nil
}

func (m *Machine) builtinClrscr() error {
	_, err := io.WriteString(m.stdout, "\x1b[2J\x1b[H")
	m.cursorX, m.cursorY = 0, 0
	return err
}

func (m *Machine) builtinWaitMsg() error {
	if _, err := io.WriteString(m.stdout, "Press [Enter] to continue..."); err != nil {
		return err
	}
	_, err := m.readLine()
	return err
}

func (m *Machine) builtinDelay() error {
	time.Sleep(time.Duration(m.regs[regEAX]) * time.Millisecond)
	return nil
}

func (m *Machine) builtinGotoxy() error {
	x := int(m.readRegister("dl"))
	y := int(m.readRegister("dh"))
	m.cursorX, m.cursorY = x, y
	_, err := fmt.Fprintf(m.stdout, "\x1b[%d;%dH", y+1, x+1)
	return err
}

func (m *Machine) builtinGetMaxXY() error {
	m.writeRegister("dl", 79)
	m.writeRegister("dh", 24)
	return nil
}

func (m *Machine) builtinSetTextColor() error {
	m.colorAttr = uint32(m.readRegister("ax"))
	fg := int(m.colorAttr & 0x0F)
	bg := int((m.colorAttr >> 4) & 0x0F)
	_, err := fmt.Fprintf(m.stdout, "\x1b[%d;%dm", ansiFg(fg), ansiBg(bg))
	return err
}

func (m *Machine) builtinGetTextColor() error {
	m.regs[regEAX] = m.colorAttr
	return nil
}

func (m *Machine) builtinRandomize() error {
	m.rng.Seed(time.Now().UnixNano())
	return nil
}

func (m *Machine) builtinRandom32() error {
	m.regs[regEAX] = m.rng.Uint32()
	return nil
}

func (m *Machine) builtinRandomRange() error {
	if m.regs[regEAX] == 0 {
		m.regs[regEAX] = 0
		return nil
	}
	m.regs[regEAX] = uint32(m.rng.Int63n(int64(m.regs[regEAX])))
	return nil
}

func (m *Machine) builtinGetMseconds() error {
	now := time.Now()
	ms := ((now.Hour()*60+now.Minute())*60+now.Second())*1000 + now.Nanosecond()/1e6
	m.regs[regEAX] = uint32(ms)
	return nil
}

func (m *Machine) builtinGetTickCount() error {
	m.regs[regEAX] = uint32(time.Since(m.startTime).Milliseconds())
	return nil
}

func (m *Machine) builtinGetCommandTail() error {
	addr := m.regs[regEDX]
	tail := strings.Join(m.args, " ")
	if int(addr)+len(tail)+1 > len(m.memory) {
		return fmt.Errorf("command tail buffer exceeds memory")
	}
	copy(m.memory[addr:], []byte(tail))
	m.memory[int(addr)+len(tail)] = 0
	m.cf = len(tail) == 0
	return nil
}

func (m *Machine) builtinIsDigit() error {
	b := byte(m.readRegister("al"))
	m.zf = b >= '0' && b <= '9'
	return nil
}

func (m *Machine) builtinParseInteger32() error {
	return m.parseNumberFromMemory(true, 10)
}

func (m *Machine) builtinParseDecimal32() error {
	return m.parseNumberFromMemory(false, 10)
}

func (m *Machine) builtinStrLengthCall() error {
	m.regs[regEAX] = uint32(len(m.readCString(m.regs[regEDX])))
	return nil
}

func (m *Machine) builtinStrCopyCall() error {
	src := m.regs[regEDX]
	dst := m.regs[regEAX]
	return m.copyCString(src, dst)
}

func (m *Machine) builtinStrCompareCall() error {
	return m.compareCString(m.regs[regEDX], m.regs[regEAX])
}

func (m *Machine) builtinStrTrimCall() error {
	return m.trimCString(m.regs[regEDX], byte(m.readRegister("al")))
}

func (m *Machine) builtinStrUCaseCall() error {
	return m.upperCString(m.regs[regEDX])
}

func (m *Machine) builtinCreateOutputFile() error {
	name := string(m.readCString(m.regs[regEDX]))
	file, err := os.Create(name)
	if err != nil {
		m.lastError = err.Error()
		m.regs[regEAX] = 0xFFFFFFFF
		return nil
	}
	m.regs[regEAX] = m.storeHandle(file)
	return nil
}

func (m *Machine) builtinOpenInputFile() error {
	name := string(m.readCString(m.regs[regEDX]))
	file, err := os.Open(name)
	if err != nil {
		m.lastError = err.Error()
		m.regs[regEAX] = 0xFFFFFFFF
		return nil
	}
	m.regs[regEAX] = m.storeHandle(file)
	return nil
}

func (m *Machine) builtinCloseFile() error {
	handle := m.regs[regEAX]
	file, ok := m.files[handle]
	if !ok {
		m.lastError = "invalid file handle"
		return nil
	}
	delete(m.files, handle)
	return file.Close()
}

func (m *Machine) builtinReadFromFile() error {
	handle := m.regs[regEAX]
	file, ok := m.files[handle]
	if !ok {
		m.lastError = "invalid file handle"
		m.cf = true
		m.regs[regEAX] = 0
		return nil
	}
	addr := m.regs[regEDX]
	count := int(m.regs[regECX])
	if int(addr)+count > len(m.memory) {
		return fmt.Errorf("read buffer exceeds memory")
	}
	n, err := file.Read(m.memory[addr : addr+uint32(count)])
	if err != nil && err != io.EOF {
		m.lastError = err.Error()
		m.cf = true
		m.regs[regEAX] = 0
		return nil
	}
	m.cf = false
	m.regs[regEAX] = uint32(n)
	return nil
}

func (m *Machine) builtinWriteToFile() error {
	handle := m.regs[regEAX]
	file, ok := m.files[handle]
	if !ok {
		m.lastError = "invalid file handle"
		m.cf = true
		m.regs[regEAX] = 0
		return nil
	}
	addr := m.regs[regEDX]
	count := int(m.regs[regECX])
	if int(addr)+count > len(m.memory) {
		return fmt.Errorf("write buffer exceeds memory")
	}
	n, err := file.Write(m.memory[addr : addr+uint32(count)])
	if err != nil {
		m.lastError = err.Error()
		m.cf = true
		m.regs[regEAX] = 0
		return nil
	}
	m.cf = false
	m.regs[regEAX] = uint32(n)
	return nil
}

func (m *Machine) builtinWriteWindowsMsg() error {
	if m.lastError == "" {
		return nil
	}
	_, err := io.WriteString(m.stdout, m.lastError+"\r\n")
	return err
}

func (m *Machine) invokeStrLength(args []Operand) error {
	if len(args) != 1 {
		return fmt.Errorf("Str_length expects one argument")
	}
	addr, err := m.requireAddress(args[0])
	if err != nil {
		return err
	}
	m.regs[regEAX] = uint32(len(m.readCString(addr)))
	return nil
}

func (m *Machine) invokeStrCopy(args []Operand) error {
	if len(args) != 2 {
		return fmt.Errorf("Str_copy expects source,target")
	}
	src, err := m.requireAddress(args[0])
	if err != nil {
		return err
	}
	dst, err := m.requireAddress(args[1])
	if err != nil {
		return err
	}
	return m.copyCString(src, dst)
}

func (m *Machine) invokeStrCompare(args []Operand) error {
	if len(args) != 2 {
		return fmt.Errorf("Str_compare expects two pointers")
	}
	left, err := m.requireAddress(args[0])
	if err != nil {
		return err
	}
	right, err := m.requireAddress(args[1])
	if err != nil {
		return err
	}
	return m.compareCString(left, right)
}

func (m *Machine) invokeStrTrim(args []Operand) error {
	if len(args) != 2 {
		return fmt.Errorf("Str_trim expects pointer,char")
	}
	addr, err := m.requireAddress(args[0])
	if err != nil {
		return err
	}
	value, _, err := m.resolveValue(args[1], 1)
	if err != nil {
		return err
	}
	return m.trimCString(addr, byte(value))
}

func (m *Machine) invokeStrUCase(args []Operand) error {
	if len(args) != 1 {
		return fmt.Errorf("Str_ucase expects one pointer")
	}
	addr, err := m.requireAddress(args[0])
	if err != nil {
		return err
	}
	return m.upperCString(addr)
}

func (m *Machine) invokeCreateFile(args []Operand) error {
	if len(args) < 5 {
		return fmt.Errorf("CreateFile expects at least five arguments")
	}
	nameAddr, err := m.requireAddress(args[0])
	if err != nil {
		return err
	}
	access, _, err := m.resolveValue(args[1], 4)
	if err != nil {
		return err
	}
	disposition, _, err := m.resolveValue(args[4], 4)
	if err != nil {
		return err
	}
	name := string(m.readCString(nameAddr))
	file, err := openCreateFile(name, access, disposition)
	if err != nil {
		m.lastError = err.Error()
		m.regs[regEAX] = 0xFFFFFFFF
		return nil
	}
	m.regs[regEAX] = m.storeHandle(file)
	return nil
}

func (m *Machine) invokeReadFile(args []Operand) error {
	if len(args) < 4 {
		return fmt.Errorf("ReadFile expects four arguments")
	}
	handle, _, err := m.resolveValue(args[0], 4)
	if err != nil {
		return err
	}
	file, ok := m.files[handle]
	if !ok {
		m.lastError = "invalid file handle"
		m.regs[regEAX] = 0
		return nil
	}
	bufferAddr, err := m.requireAddress(args[1])
	if err != nil {
		return err
	}
	count, _, err := m.resolveValue(args[2], 4)
	if err != nil {
		return err
	}
	outAddr, err := m.requireAddress(args[3])
	if err != nil {
		return err
	}
	n, readErr := file.Read(m.memory[bufferAddr : bufferAddr+count])
	if readErr != nil && readErr != io.EOF {
		m.lastError = readErr.Error()
		m.regs[regEAX] = 0
		_ = m.writeMemory(outAddr, 0, 4)
		return nil
	}
	_ = m.writeMemory(outAddr, uint32(n), 4)
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeWriteFile(args []Operand) error {
	if len(args) < 4 {
		return fmt.Errorf("WriteFile expects four arguments")
	}
	handle, _, err := m.resolveValue(args[0], 4)
	if err != nil {
		return err
	}
	file, ok := m.files[handle]
	if !ok {
		m.lastError = "invalid file handle"
		m.regs[regEAX] = 0
		return nil
	}
	bufferAddr, err := m.requireAddress(args[1])
	if err != nil {
		return err
	}
	count, _, err := m.resolveValue(args[2], 4)
	if err != nil {
		return err
	}
	outAddr, err := m.requireAddress(args[3])
	if err != nil {
		return err
	}
	n, writeErr := file.Write(m.memory[bufferAddr : bufferAddr+count])
	if writeErr != nil {
		m.lastError = writeErr.Error()
		m.regs[regEAX] = 0
		_ = m.writeMemory(outAddr, 0, 4)
		return nil
	}
	_ = m.writeMemory(outAddr, uint32(n), 4)
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeCloseHandle(args []Operand) error {
	if len(args) != 1 {
		return fmt.Errorf("CloseHandle expects one argument")
	}
	handle, _, err := m.resolveValue(args[0], 4)
	if err != nil {
		return err
	}
	file, ok := m.files[handle]
	if !ok {
		m.regs[regEAX] = 0
		return nil
	}
	delete(m.files, handle)
	if err := file.Close(); err != nil {
		m.lastError = err.Error()
		m.regs[regEAX] = 0
		return nil
	}
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeGetStdHandle(args []Operand) error {
	if len(args) != 1 {
		return fmt.Errorf("GetStdHandle expects one argument")
	}
	handle, _, err := m.resolveValue(args[0], 4)
	if err != nil {
		return err
	}
	switch int32(handle) {
	case -10:
		m.regs[regEAX] = stdInputHandleValue
	case -11:
		m.regs[regEAX] = stdOutputHandleValue
	case -12:
		m.regs[regEAX] = stdErrorHandleValue
	default:
		m.regs[regEAX] = 0xFFFFFFFF
	}
	return nil
}

func (m *Machine) invokeGetConsoleMode(args []Operand) error {
	if len(args) != 2 {
		return fmt.Errorf("GetConsoleMode expects two arguments")
	}
	handle, _, err := m.resolveValue(args[0], 4)
	if err != nil {
		return err
	}
	outAddr, err := m.requireAddress(args[1])
	if err != nil {
		return err
	}
	mode := m.consoleModeForHandle(handle)
	if err := m.writeMemory(outAddr, mode, 4); err != nil {
		return err
	}
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeSetConsoleMode(args []Operand) error {
	if len(args) != 2 {
		return fmt.Errorf("SetConsoleMode expects two arguments")
	}
	handle, _, err := m.resolveValue(args[0], 4)
	if err != nil {
		return err
	}
	mode, _, err := m.resolveValue(args[1], 4)
	if err != nil {
		return err
	}
	m.setConsoleModeForHandle(handle, mode)
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeFlushConsoleInputBuffer(args []Operand) error {
	if len(args) != 1 {
		return fmt.Errorf("FlushConsoleInputBuffer expects one argument")
	}
	for m.peekableInput {
		b, err := m.stdin.Peek(1)
		if err != nil || len(b) == 0 {
			break
		}
		_, _ = m.stdin.ReadByte()
	}
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokePeekConsoleInput(args []Operand) error {
	return m.invokeConsoleInputRecord(args, false)
}

func (m *Machine) invokeReadConsoleInput(args []Operand) error {
	return m.invokeConsoleInputRecord(args, true)
}

func (m *Machine) invokeGetNumberOfConsoleInputEvents(args []Operand) error {
	if len(args) != 2 {
		return fmt.Errorf("GetNumberOfConsoleInputEvents expects two arguments")
	}
	outAddr, err := m.requireAddress(args[1])
	if err != nil {
		return err
	}
	count := uint32(0)
	if b, err := m.stdin.Peek(1); err == nil && len(b) > 0 {
		count = 1
	}
	if err := m.writeMemory(outAddr, count, 4); err != nil {
		return err
	}
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeGetKeyState(args []Operand) error {
	if len(args) != 1 {
		return fmt.Errorf("GetKeyState expects one argument")
	}
	key, _, err := m.resolveValue(args[0], 4)
	if err != nil {
		return err
	}
	var value uint32
	switch key {
	case 0x90:
		value = 0
	default:
		value = 0
	}
	m.regs[regEAX] = value
	return nil
}

func (m *Machine) invokeWriteConsole(args []Operand) error {
	if len(args) < 4 {
		return fmt.Errorf("WriteConsole expects at least four arguments")
	}
	handle, _, err := m.resolveValue(args[0], 4)
	if err != nil {
		return err
	}
	bufferAddr, err := m.requireAddress(args[1])
	if err != nil {
		return err
	}
	count, _, err := m.resolveValue(args[2], 4)
	if err != nil {
		return err
	}
	writtenAddr, err := m.requireAddress(args[3])
	if err != nil {
		return err
	}
	if int(bufferAddr)+int(count) > len(m.memory) {
		return fmt.Errorf("WriteConsole buffer exceeds memory")
	}
	data := m.memory[bufferAddr : bufferAddr+count]
	switch handle {
	case stdErrorHandleValue:
		_, err = m.stderr.Write(data)
	default:
		_, err = m.stdout.Write(data)
	}
	if err != nil {
		m.lastError = err.Error()
		m.regs[regEAX] = 0
		_ = m.writeMemory(writtenAddr, 0, 4)
		return nil
	}
	_ = m.writeMemory(writtenAddr, count, 4)
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeReadConsole(args []Operand) error {
	if len(args) < 4 {
		return fmt.Errorf("ReadConsole expects at least four arguments")
	}
	bufferAddr, err := m.requireAddress(args[1])
	if err != nil {
		return err
	}
	capacity, _, err := m.resolveValue(args[2], 4)
	if err != nil {
		return err
	}
	countAddr, err := m.requireAddress(args[3])
	if err != nil {
		return err
	}
	line, err := m.readLine()
	if err != nil {
		return err
	}
	data := []byte(line + "\r\n")
	if len(data) > int(capacity) {
		data = data[:capacity]
	}
	if int(bufferAddr)+len(data) > len(m.memory) {
		return fmt.Errorf("ReadConsole buffer exceeds memory")
	}
	copy(m.memory[bufferAddr:], data)
	if err := m.writeMemory(countAddr, uint32(len(data)), 4); err != nil {
		return err
	}
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeConsoleInputRecord(args []Operand, consume bool) error {
	if len(args) < 4 {
		return fmt.Errorf("%s expects four arguments", map[bool]string{true: "ReadConsoleInput", false: "PeekConsoleInput"}[consume])
	}
	bufferAddr, err := m.requireAddress(args[1])
	if err != nil {
		return err
	}
	countAddr, err := m.requireAddress(args[3])
	if err != nil {
		return err
	}
	b, err := m.stdin.Peek(1)
	if err != nil || len(b) == 0 {
		if err := m.writeMemory(countAddr, 0, 4); err != nil {
			return err
		}
		m.regs[regEAX] = 1
		return nil
	}
	ch := b[0]
	if consume {
		_, _ = m.stdin.ReadByte()
	}
	for i := 0; i < 20 && int(bufferAddr)+i < len(m.memory); i++ {
		m.memory[int(bufferAddr)+i] = 0
	}
	if err := m.writeWord(bufferAddr, 1); err != nil {
		return err
	}
	if err := m.writeMemory(bufferAddr+4, 1, 4); err != nil {
		return err
	}
	if err := m.writeWord(bufferAddr+10, uint16(virtualKeyCode(ch))); err != nil {
		return err
	}
	if err := m.writeWord(bufferAddr+12, 0); err != nil {
		return err
	}
	if err := m.writeMemory(bufferAddr+14, uint32(ch), 1); err != nil {
		return err
	}
	if err := m.writeMemory(countAddr, 1, 4); err != nil {
		return err
	}
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeSetConsoleCursorPosition(args []Operand) error {
	if len(args) != 2 {
		return fmt.Errorf("SetConsoleCursorPosition expects two arguments")
	}
	x, y, err := m.readCoordOperand(args[1])
	if err != nil {
		return err
	}
	m.cursorX = int(int16(x))
	m.cursorY = int(int16(y))
	_, err = fmt.Fprintf(m.stdout, "\x1b[%d;%dH", m.cursorY+1, m.cursorX+1)
	if err == nil {
		m.regs[regEAX] = 1
	} else {
		m.regs[regEAX] = 0
	}
	return err
}

func (m *Machine) invokeGetConsoleCursorInfo(args []Operand) error {
	if len(args) != 2 {
		return fmt.Errorf("GetConsoleCursorInfo expects two arguments")
	}
	addr, err := m.requireAddress(args[1])
	if err != nil {
		return err
	}
	if err := m.writeMemory(addr, m.cursorSize, 4); err != nil {
		return err
	}
	visible := uint32(0)
	if m.cursorVisible {
		visible = 1
	}
	if err := m.writeMemory(addr+4, visible, 4); err != nil {
		return err
	}
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeSetConsoleCursorInfo(args []Operand) error {
	if len(args) != 2 {
		return fmt.Errorf("SetConsoleCursorInfo expects two arguments")
	}
	addr, err := m.requireAddress(args[1])
	if err != nil {
		return err
	}
	size, err := m.readMemory(addr, 4)
	if err != nil {
		return err
	}
	visible, err := m.readMemory(addr+4, 4)
	if err != nil {
		return err
	}
	m.cursorSize = size
	m.cursorVisible = visible != 0
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeSetConsoleScreenBufferSize(args []Operand) error {
	if len(args) != 2 {
		return fmt.Errorf("SetConsoleScreenBufferSize expects two arguments")
	}
	x, y, err := m.readCoordOperand(args[1])
	if err != nil {
		return err
	}
	m.screenWidth = int(int16(x))
	m.screenHeight = int(int16(y))
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeSetConsoleTitle(args []Operand) error {
	if len(args) != 1 {
		return fmt.Errorf("SetConsoleTitle expects one argument")
	}
	addr, err := m.requireAddress(args[0])
	if err != nil {
		return err
	}
	m.consoleTitle = string(m.readCString(addr))
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeGetConsoleScreenBufferInfo(args []Operand) error {
	if len(args) != 2 {
		return fmt.Errorf("GetConsoleScreenBufferInfo expects two arguments")
	}
	addr, err := m.requireAddress(args[1])
	if err != nil {
		return err
	}
	if err := m.writeCoord(addr, uint16(m.screenWidth), uint16(m.screenHeight)); err != nil {
		return err
	}
	if err := m.writeCoord(addr+4, uint16(m.cursorX), uint16(m.cursorY)); err != nil {
		return err
	}
	if err := m.writeMemory(addr+8, m.colorAttr, 2); err != nil {
		return err
	}
	if err := m.writeWord(addr+10, 0); err != nil {
		return err
	}
	if err := m.writeWord(addr+12, 0); err != nil {
		return err
	}
	right := m.screenWidth - 1
	if right < 0 {
		right = 0
	}
	if err := m.writeWord(addr+14, uint16(right)); err != nil {
		return err
	}
	bottom := m.screenHeight - 1
	if bottom < 0 {
		bottom = 0
	}
	if err := m.writeWord(addr+16, uint16(bottom)); err != nil {
		return err
	}
	if err := m.writeCoord(addr+18, uint16(m.screenWidth), uint16(m.screenHeight)); err != nil {
		return err
	}
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeSleep(args []Operand) error {
	if len(args) != 1 {
		return fmt.Errorf("Sleep expects one argument")
	}
	delay, _, err := m.resolveValue(args[0], 4)
	if err != nil {
		return err
	}
	time.Sleep(time.Duration(uint32(delay)) * time.Millisecond)
	m.regs[regEAX] = 0
	return nil
}

func (m *Machine) invokeGetLocalTime(args []Operand) error {
	if len(args) != 1 {
		return fmt.Errorf("GetLocalTime expects one argument")
	}
	addr, err := m.requireAddress(args[0])
	if err != nil {
		return err
	}
	if err := m.writeSystemTime(addr, time.Now()); err != nil {
		return err
	}
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeGetSystemTime(args []Operand) error {
	if len(args) != 1 {
		return fmt.Errorf("GetSystemTime expects one argument")
	}
	addr, err := m.requireAddress(args[0])
	if err != nil {
		return err
	}
	if err := m.writeSystemTime(addr, time.Now().UTC()); err != nil {
		return err
	}
	m.regs[regEAX] = 1
	return nil
}

func (m *Machine) invokeWriteStackFrame(args []Operand) error {
	if len(args) != 3 {
		return fmt.Errorf("WriteStackFrame expects three arguments")
	}
	return m.writeStackFrame("", args)
}

func (m *Machine) invokeWriteStackFrameName(args []Operand) error {
	if len(args) != 4 {
		return fmt.Errorf("WriteStackFrameName expects four arguments")
	}
	nameAddr, err := m.requireAddress(args[3])
	if err != nil {
		return err
	}
	name := string(m.readCString(nameAddr))
	return m.writeStackFrame(name, args[:3])
}

func (m *Machine) writeStackFrame(name string, args []Operand) error {
	params, _, err := m.resolveValue(args[0], 4)
	if err != nil {
		return err
	}
	locals, _, err := m.resolveValue(args[1], 4)
	if err != nil {
		return err
	}
	saved, _, err := m.resolveValue(args[2], 4)
	if err != nil {
		return err
	}
	label := "Stack frame"
	if name != "" {
		label = "Stack frame: " + name
	}
	_, err = fmt.Fprintf(m.stdout, "%s\r\nparams=%d locals=%d saved=%d ebp=%08X esp=%08X\r\n", label, params, locals, saved, m.regs[regEBP], m.regs[regESP])
	return err
}

func (m *Machine) consoleModeForHandle(handle uint32) uint32 {
	switch handle {
	case stdInputHandleValue:
		return m.inputConsoleMode
	default:
		return m.outputConsoleMode
	}
}

func (m *Machine) setConsoleModeForHandle(handle uint32, mode uint32) {
	switch handle {
	case stdInputHandleValue:
		m.inputConsoleMode = mode
	default:
		m.outputConsoleMode = mode
	}
}

func (m *Machine) readCoordOperand(op Operand) (uint16, uint16, error) {
	switch op.Kind {
	case "imm":
		value := uint32(op.Value)
		return uint16(value), uint16(value >> 16), nil
	case "symbol":
		symbol, ok := m.symbols[strings.ToLower(op.Text)]
		if !ok {
			return 0, 0, fmt.Errorf("unknown symbol %q", op.Text)
		}
		return m.readCoord(symbol.Address)
	case "mem":
		addr, err := m.resolveAddress(op)
		if err != nil {
			return 0, 0, err
		}
		return m.readCoord(addr)
	default:
		return 0, 0, fmt.Errorf("operand %q is not a COORD value", op.Text)
	}
}

func (m *Machine) readCoord(addr uint32) (uint16, uint16, error) {
	x, err := m.readMemory(addr, 2)
	if err != nil {
		return 0, 0, err
	}
	y, err := m.readMemory(addr+2, 2)
	if err != nil {
		return 0, 0, err
	}
	return uint16(x), uint16(y), nil
}

func (m *Machine) writeCoord(addr uint32, x uint16, y uint16) error {
	if err := m.writeWord(addr, x); err != nil {
		return err
	}
	return m.writeWord(addr+2, y)
}

func (m *Machine) writeWord(addr uint32, value uint16) error {
	return m.writeMemory(addr, uint32(value), 2)
}

func (m *Machine) writeSystemTime(addr uint32, value time.Time) error {
	fields := []uint16{
		uint16(value.Year()),
		uint16(value.Month()),
		uint16(value.Weekday()),
		uint16(value.Day()),
		uint16(value.Hour()),
		uint16(value.Minute()),
		uint16(value.Second()),
		uint16(value.Nanosecond() / 1e6),
	}
	for i, field := range fields {
		if err := m.writeWord(addr+uint32(i*2), field); err != nil {
			return err
		}
	}
	return nil
}

func (m *Machine) operandWidth(primary Operand, secondary Operand, fallback int) (int, error) {
	if width := explicitWidth(primary); width != 0 {
		return width, nil
	}
	if width := explicitWidth(secondary); width != 0 {
		return width, nil
	}
	if primary.Kind == "symbol" {
		symbol, ok := m.symbols[strings.ToLower(primary.Text)]
		if !ok {
			return 0, fmt.Errorf("unknown symbol %q", primary.Text)
		}
		return clampWidth(int(symbol.ElemSize)), nil
	}
	if primary.Kind == "mem" && primary.Text != "" {
		if symbol, ok := m.symbols[strings.ToLower(primary.Text)]; ok {
			return clampWidth(int(symbol.ElemSize)), nil
		}
	}
	if primary.Kind == "reg" {
		return registerWidth(primary.Text), nil
	}
	if secondary.Kind == "reg" {
		return registerWidth(secondary.Text), nil
	}
	return fallback, nil
}

func (m *Machine) resolveValue(op Operand, width int) (uint32, int, error) {
	switch op.Kind {
	case "imm":
		return uint32(op.Value), clampWidth(width), nil
	case "reg":
		return m.readRegister(op.Text), registerWidth(op.Text), nil
	case "symbol":
		symbol, ok := m.symbols[strings.ToLower(op.Text)]
		if !ok {
			return 0, 0, fmt.Errorf("unknown symbol %q", op.Text)
		}
		readWidth := clampWidth(width)
		if readWidth == 0 {
			readWidth = clampWidth(int(symbol.ElemSize))
		}
		value, err := m.readMemory(symbol.Address, readWidth)
		return value, readWidth, err
	case "mem":
		readWidth := clampWidth(width)
		if readWidth == 0 {
			readWidth = clampWidth(op.Size)
		}
		if readWidth == 0 {
			readWidth = 4
		}
		addr, err := m.resolveAddress(op)
		if err != nil {
			return 0, 0, err
		}
		value, err := m.readMemory(addr, readWidth)
		return value, readWidth, err
	default:
		return 0, 0, fmt.Errorf("cannot read operand kind %q", op.Kind)
	}
}

func (m *Machine) assign(op Operand, value uint32, width int) error {
	width = clampWidth(width)
	if width == 0 {
		width = 4
	}
	switch op.Kind {
	case "reg":
		m.writeRegister(op.Text, truncate(value, width))
		return nil
	case "symbol":
		symbol, ok := m.symbols[strings.ToLower(op.Text)]
		if !ok {
			return fmt.Errorf("unknown symbol %q", op.Text)
		}
		writeWidth := width
		if writeWidth == 0 {
			writeWidth = clampWidth(int(symbol.ElemSize))
		}
		return m.writeMemory(symbol.Address, truncate(value, writeWidth), writeWidth)
	case "mem":
		addr, err := m.resolveAddress(op)
		if err != nil {
			return err
		}
		if op.Size != 0 {
			width = clampWidth(op.Size)
		}
		if width == 0 {
			width = 4
		}
		return m.writeMemory(addr, truncate(value, width), width)
	default:
		return fmt.Errorf("cannot assign to operand kind %q", op.Kind)
	}
}

func (m *Machine) resolveAddress(op Operand) (uint32, error) {
	if op.Kind != "mem" {
		return 0, fmt.Errorf("operand is not memory")
	}
	total := op.Offset
	if op.Text != "" {
		symbol, ok := m.symbols[strings.ToLower(op.Text)]
		if !ok {
			return 0, fmt.Errorf("unknown symbol %q", op.Text)
		}
		total += int64(symbol.Address)
	}
	if op.Base != "" {
		total += int64(m.readRegister(op.Base))
	}
	if op.Index != "" {
		scale := op.Scale
		if scale == 0 {
			scale = 1
		}
		total += int64(m.readRegister(op.Index)) * scale
	}
	if total < 0 {
		return 0, fmt.Errorf("negative address %d", total)
	}
	return uint32(total), nil
}

func (m *Machine) readMemory(addr uint32, width int) (uint32, error) {
	if int(addr)+width > len(m.memory) {
		return 0, fmt.Errorf("memory read out of range at %08X", addr)
	}
	switch width {
	case 1:
		return uint32(m.memory[addr]), nil
	case 2:
		return uint32(binary.LittleEndian.Uint16(m.memory[addr:])), nil
	case 4:
		return binary.LittleEndian.Uint32(m.memory[addr:]), nil
	default:
		return 0, fmt.Errorf("unsupported read width %d", width)
	}
}

func (m *Machine) writeMemory(addr uint32, value uint32, width int) error {
	if int(addr)+width > len(m.memory) {
		return fmt.Errorf("memory write out of range at %08X", addr)
	}
	switch width {
	case 1:
		m.memory[addr] = byte(value)
	case 2:
		binary.LittleEndian.PutUint16(m.memory[addr:], uint16(value))
	case 4:
		binary.LittleEndian.PutUint32(m.memory[addr:], value)
	default:
		return fmt.Errorf("unsupported write width %d", width)
	}
	return nil
}

func (m *Machine) readMemory64(addr uint32) (uint64, error) {
	if int(addr)+8 > len(m.memory) {
		return 0, fmt.Errorf("memory read out of range at %08X", addr)
	}
	return binary.LittleEndian.Uint64(m.memory[addr:]), nil
}

func (m *Machine) writeMemory64(addr uint32, value uint64) error {
	if int(addr)+8 > len(m.memory) {
		return fmt.Errorf("memory write out of range at %08X", addr)
	}
	binary.LittleEndian.PutUint64(m.memory[addr:], value)
	return nil
}

func (m *Machine) fpuPush(value float64) error {
	if len(m.fpu) >= 8 {
		return fmt.Errorf("fpu stack overflow")
	}
	m.fpu = append(m.fpu, value)
	return nil
}

func (m *Machine) fpuPeek(index int) (float64, error) {
	if index < 0 || index >= len(m.fpu) {
		return 0, fmt.Errorf("fpu stack index ST(%d) out of range", index)
	}
	return m.fpu[len(m.fpu)-1-index], nil
}

func (m *Machine) fpuSet(index int, value float64) error {
	if index < 0 || index >= len(m.fpu) {
		return fmt.Errorf("fpu stack index ST(%d) out of range", index)
	}
	m.fpu[len(m.fpu)-1-index] = value
	return nil
}

func (m *Machine) fpuPop() (float64, error) {
	if len(m.fpu) == 0 {
		return 0, fmt.Errorf("fpu stack underflow")
	}
	last := m.fpu[len(m.fpu)-1]
	m.fpu = m.fpu[:len(m.fpu)-1]
	return last, nil
}

func (m *Machine) readFloatOperand(op Operand) (float64, error) {
	switch op.Kind {
	case "st":
		return m.fpuPeek(int(op.Value))
	case "symbol":
		symbol, ok := m.symbols[strings.ToLower(op.Text)]
		if !ok {
			return 0, fmt.Errorf("unknown symbol %q", op.Text)
		}
		switch symbol.ElemSize {
		case 4:
			value, err := m.readMemory(symbol.Address, 4)
			if err != nil {
				return 0, err
			}
			return float64(math.Float32frombits(value)), nil
		case 8:
			value, err := m.readMemory64(symbol.Address)
			if err != nil {
				return 0, err
			}
			return math.Float64frombits(value), nil
		case 10:
			value, err := m.readMemory64(symbol.Address)
			if err != nil {
				return 0, err
			}
			return math.Float64frombits(value), nil
		default:
			return 0, fmt.Errorf("unsupported floating-point symbol width %d", symbol.ElemSize)
		}
	case "mem":
		addr, err := m.resolveAddress(op)
		if err != nil {
			return 0, err
		}
		width := op.Size
		if width == 0 && op.Text != "" {
			if symbol, ok := m.symbols[strings.ToLower(op.Text)]; ok {
				width = int(symbol.ElemSize)
			}
		}
		if width == 0 {
			width = 8
		}
		switch width {
		case 4:
			value, err := m.readMemory(addr, 4)
			if err != nil {
				return 0, err
			}
			return float64(math.Float32frombits(value)), nil
		case 8:
			value, err := m.readMemory64(addr)
			if err != nil {
				return 0, err
			}
			return math.Float64frombits(value), nil
		case 10:
			value, err := m.readMemory64(addr)
			if err != nil {
				return 0, err
			}
			return math.Float64frombits(value), nil
		default:
			return 0, fmt.Errorf("unsupported floating-point memory width %d", width)
		}
	case "imm":
		return float64(op.Value), nil
	default:
		return 0, fmt.Errorf("cannot read floating-point operand kind %q", op.Kind)
	}
}

func (m *Machine) writeFloatOperand(op Operand, value float64) error {
	switch op.Kind {
	case "st":
		return m.fpuSet(int(op.Value), value)
	case "symbol":
		symbol, ok := m.symbols[strings.ToLower(op.Text)]
		if !ok {
			return fmt.Errorf("unknown symbol %q", op.Text)
		}
		switch symbol.ElemSize {
		case 4:
			return m.writeMemory(symbol.Address, math.Float32bits(float32(value)), 4)
		case 8:
			return m.writeMemory64(symbol.Address, math.Float64bits(value))
		case 10:
			if err := m.writeMemory64(symbol.Address, math.Float64bits(value)); err != nil {
				return err
			}
			return m.writeMemory(symbol.Address+8, 0, 2)
		default:
			return fmt.Errorf("unsupported floating-point symbol width %d", symbol.ElemSize)
		}
	case "mem":
		addr, err := m.resolveAddress(op)
		if err != nil {
			return err
		}
		width := op.Size
		if width == 0 && op.Text != "" {
			if symbol, ok := m.symbols[strings.ToLower(op.Text)]; ok {
				width = int(symbol.ElemSize)
			}
		}
		if width == 0 {
			width = 8
		}
		switch width {
		case 4:
			return m.writeMemory(addr, math.Float32bits(float32(value)), 4)
		case 8:
			return m.writeMemory64(addr, math.Float64bits(value))
		case 10:
			if err := m.writeMemory64(addr, math.Float64bits(value)); err != nil {
				return err
			}
			return m.writeMemory(addr+8, 0, 2)
		default:
			return fmt.Errorf("unsupported floating-point memory width %d", width)
		}
	default:
		return fmt.Errorf("cannot write floating-point operand kind %q", op.Kind)
	}
}

func (m *Machine) readSignedIntegerOperand(op Operand) (int64, error) {
	switch op.Kind {
	case "symbol":
		symbol, ok := m.symbols[strings.ToLower(op.Text)]
		if !ok {
			return 0, fmt.Errorf("unknown symbol %q", op.Text)
		}
		switch symbol.ElemSize {
		case 2:
			value, err := m.readMemory(symbol.Address, 2)
			return int64(int16(value)), err
		case 4:
			value, err := m.readMemory(symbol.Address, 4)
			return int64(int32(value)), err
		case 8:
			value, err := m.readMemory64(symbol.Address)
			return int64(value), err
		default:
			return 0, fmt.Errorf("unsupported integer width %d", symbol.ElemSize)
		}
	case "mem":
		addr, err := m.resolveAddress(op)
		if err != nil {
			return 0, err
		}
		width := op.Size
		if width == 0 && op.Text != "" {
			if symbol, ok := m.symbols[strings.ToLower(op.Text)]; ok {
				width = int(symbol.ElemSize)
			}
		}
		switch width {
		case 2:
			value, err := m.readMemory(addr, 2)
			return int64(int16(value)), err
		case 4:
			value, err := m.readMemory(addr, 4)
			return int64(int32(value)), err
		case 8:
			value, err := m.readMemory64(addr)
			return int64(value), err
		default:
			return 0, fmt.Errorf("unsupported integer width %d", width)
		}
	default:
		return 0, fmt.Errorf("fild requires memory or symbol operand")
	}
}

func (m *Machine) integerOperandWidth(op Operand) (int, error) {
	switch op.Kind {
	case "symbol":
		symbol, ok := m.symbols[strings.ToLower(op.Text)]
		if !ok {
			return 0, fmt.Errorf("unknown symbol %q", op.Text)
		}
		return int(symbol.ElemSize), nil
	case "mem":
		if op.Size != 0 {
			return op.Size, nil
		}
		if op.Text != "" {
			if symbol, ok := m.symbols[strings.ToLower(op.Text)]; ok {
				return int(symbol.ElemSize), nil
			}
		}
		return 4, nil
	default:
		if op.Kind == "reg" {
			return registerWidth(op.Text), nil
		}
		return 4, nil
	}
}

func (m *Machine) roundFloatToInt(value float64) int64 {
	mode := (m.fpuControlWord >> 10) & 0x3
	switch mode {
	case 1:
		return int64(math.Floor(value))
	case 2:
		return int64(math.Ceil(value))
	case 3:
		return int64(math.Trunc(value))
	default:
		return int64(math.RoundToEven(value))
	}
}

func (m *Machine) setCPUFloatCompareFlags(left, right float64) {
	m.cf, m.pf, m.zf = false, false, false
	m.of, m.sf, m.af = false, false, false
	switch {
	case math.IsNaN(left) || math.IsNaN(right):
		m.cf, m.pf, m.zf = true, true, true
	case left < right:
		m.cf = true
	case left == right:
		m.zf = true
	}
}

func (m *Machine) setFPUCompareStatus(left, right float64) {
	m.fpuStatusWord &^= (1 << 8) | (1 << 10) | (1 << 14)
	switch {
	case math.IsNaN(left) || math.IsNaN(right):
		m.fpuStatusWord |= (1 << 8) | (1 << 10) | (1 << 14)
	case left < right:
		m.fpuStatusWord |= 1 << 8
	case left == right:
		m.fpuStatusWord |= 1 << 14
	}
}

func (m *Machine) push32(value uint32) error {
	esp := m.regs[regESP] - 4
	if err := m.writeMemory(esp, value, 4); err != nil {
		return err
	}
	m.regs[regESP] = esp
	return nil
}

func (m *Machine) pop32() (uint32, error) {
	value, err := m.readMemory(m.regs[regESP], 4)
	if err != nil {
		return 0, err
	}
	m.regs[regESP] += 4
	return value, nil
}

func (m *Machine) readRegister(name string) uint32 {
	name = strings.ToLower(name)
	switch name {
	case "eax":
		return m.regs[regEAX]
	case "ecx":
		return m.regs[regECX]
	case "edx":
		return m.regs[regEDX]
	case "ebx":
		return m.regs[regEBX]
	case "esp":
		return m.regs[regESP]
	case "ebp":
		return m.regs[regEBP]
	case "esi":
		return m.regs[regESI]
	case "edi":
		return m.regs[regEDI]
	case "ax":
		return m.regs[regEAX] & 0xFFFF
	case "cx":
		return m.regs[regECX] & 0xFFFF
	case "dx":
		return m.regs[regEDX] & 0xFFFF
	case "bx":
		return m.regs[regEBX] & 0xFFFF
	case "sp":
		return m.regs[regESP] & 0xFFFF
	case "bp":
		return m.regs[regEBP] & 0xFFFF
	case "si":
		return m.regs[regESI] & 0xFFFF
	case "di":
		return m.regs[regEDI] & 0xFFFF
	case "al":
		return m.regs[regEAX] & 0xFF
	case "ah":
		return (m.regs[regEAX] >> 8) & 0xFF
	case "bl":
		return m.regs[regEBX] & 0xFF
	case "bh":
		return (m.regs[regEBX] >> 8) & 0xFF
	case "cl":
		return m.regs[regECX] & 0xFF
	case "ch":
		return (m.regs[regECX] >> 8) & 0xFF
	case "dl":
		return m.regs[regEDX] & 0xFF
	case "dh":
		return (m.regs[regEDX] >> 8) & 0xFF
	default:
		return 0
	}
}

func (m *Machine) writeRegister(name string, value uint32) {
	name = strings.ToLower(name)
	switch name {
	case "eax":
		m.regs[regEAX] = value
	case "ecx":
		m.regs[regECX] = value
	case "edx":
		m.regs[regEDX] = value
	case "ebx":
		m.regs[regEBX] = value
	case "esp":
		m.regs[regESP] = value
	case "ebp":
		m.regs[regEBP] = value
	case "esi":
		m.regs[regESI] = value
	case "edi":
		m.regs[regEDI] = value
	case "ax":
		m.regs[regEAX] = (m.regs[regEAX] & 0xFFFF0000) | (value & 0xFFFF)
	case "cx":
		m.regs[regECX] = (m.regs[regECX] & 0xFFFF0000) | (value & 0xFFFF)
	case "dx":
		m.regs[regEDX] = (m.regs[regEDX] & 0xFFFF0000) | (value & 0xFFFF)
	case "bx":
		m.regs[regEBX] = (m.regs[regEBX] & 0xFFFF0000) | (value & 0xFFFF)
	case "sp":
		m.regs[regESP] = (m.regs[regESP] & 0xFFFF0000) | (value & 0xFFFF)
	case "bp":
		m.regs[regEBP] = (m.regs[regEBP] & 0xFFFF0000) | (value & 0xFFFF)
	case "si":
		m.regs[regESI] = (m.regs[regESI] & 0xFFFF0000) | (value & 0xFFFF)
	case "di":
		m.regs[regEDI] = (m.regs[regEDI] & 0xFFFF0000) | (value & 0xFFFF)
	case "al":
		m.regs[regEAX] = (m.regs[regEAX] & 0xFFFFFF00) | (value & 0xFF)
	case "ah":
		m.regs[regEAX] = (m.regs[regEAX] & 0xFFFF00FF) | ((value & 0xFF) << 8)
	case "bl":
		m.regs[regEBX] = (m.regs[regEBX] & 0xFFFFFF00) | (value & 0xFF)
	case "bh":
		m.regs[regEBX] = (m.regs[regEBX] & 0xFFFF00FF) | ((value & 0xFF) << 8)
	case "cl":
		m.regs[regECX] = (m.regs[regECX] & 0xFFFFFF00) | (value & 0xFF)
	case "ch":
		m.regs[regECX] = (m.regs[regECX] & 0xFFFF00FF) | ((value & 0xFF) << 8)
	case "dl":
		m.regs[regEDX] = (m.regs[regEDX] & 0xFFFFFF00) | (value & 0xFF)
	case "dh":
		m.regs[regEDX] = (m.regs[regEDX] & 0xFFFF00FF) | ((value & 0xFF) << 8)
	}
}

func (m *Machine) readLine() (string, error) {
	line, err := m.stdin.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")
	return line, nil
}

func (m *Machine) readCString(addr uint32) []byte {
	var out []byte
	for int(addr) < len(m.memory) && m.memory[addr] != 0 {
		out = append(out, m.memory[addr])
		addr++
	}
	return out
}

func (m *Machine) copyCString(src, dst uint32) error {
	data := m.readCString(src)
	if int(dst)+len(data)+1 > len(m.memory) {
		return fmt.Errorf("string copy exceeds memory")
	}
	copy(m.memory[dst:], data)
	m.memory[int(dst)+len(data)] = 0
	return nil
}

func (m *Machine) compareCString(left, right uint32) error {
	a := m.readCString(left)
	b := m.readCString(right)
	cmp := bytesCompare(a, b)
	m.zf = cmp == 0
	m.cf = cmp < 0
	m.sf = cmp < 0
	m.of = false
	m.af = false
	m.pf = cmp == 0
	return nil
}

func (m *Machine) trimCString(addr uint32, ch byte) error {
	data := m.readCString(addr)
	for len(data) > 0 && data[len(data)-1] == ch {
		data = data[:len(data)-1]
	}
	if int(addr)+len(data)+1 > len(m.memory) {
		return fmt.Errorf("string trim exceeds memory")
	}
	copy(m.memory[addr:], data)
	m.memory[int(addr)+len(data)] = 0
	return nil
}

func (m *Machine) upperCString(addr uint32) error {
	data := m.readCString(addr)
	for i := range data {
		if data[i] >= 'a' && data[i] <= 'z' {
			data[i] -= 32
		}
	}
	copy(m.memory[addr:], data)
	return nil
}

func (m *Machine) dumpMemory(addr, count, elemSize uint32) error {
	if elemSize == 0 {
		elemSize = 1
	}
	if _, err := io.WriteString(m.stdout, "\r\n"); err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		itemAddr := addr + i*elemSize
		if _, err := fmt.Fprintf(m.stdout, "%08X: ", itemAddr); err != nil {
			return err
		}
		switch elemSize {
		case 1:
			value, err := m.readMemory(itemAddr, 1)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(m.stdout, "%02X", value); err != nil {
				return err
			}
		case 2:
			value, err := m.readMemory(itemAddr, 2)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(m.stdout, "%04X", value); err != nil {
				return err
			}
		default:
			value, err := m.readMemory(itemAddr, 4)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(m.stdout, "%08X", value); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(m.stdout, "\r\n"); err != nil {
			return err
		}
	}
	return nil
}

func (m *Machine) requireAddress(op Operand) (uint32, error) {
	switch op.Kind {
	case "imm":
		return uint32(op.Value), nil
	case "mem":
		return m.resolveAddress(op)
	case "symbol":
		symbol, ok := m.symbols[strings.ToLower(op.Text)]
		if !ok {
			return 0, fmt.Errorf("unknown symbol %q", op.Text)
		}
		return symbol.Address, nil
	default:
		return 0, fmt.Errorf("operand %q is not an address", op.Text)
	}
}

func (m *Machine) parseNumberFromMemory(signed bool, base int) error {
	addr := m.regs[regEDX]
	length := int(m.regs[regECX])
	data := m.readCString(addr)
	if length > 0 && length < len(data) {
		data = data[:length]
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		m.regs[regEAX] = 0
		m.cf = false
		m.of = false
		return nil
	}
	if signed {
		value, ok := parseSignedInput(text, base, 32)
		if !ok {
			m.regs[regEAX] = 0
			m.cf, m.of = true, true
			return nil
		}
		m.regs[regEAX] = uint32(int32(value))
	} else {
		value, ok := parseUnsignedInput(text, base, 32)
		if !ok {
			m.regs[regEAX] = 0
			m.cf, m.of = true, true
			return nil
		}
		m.regs[regEAX] = uint32(value)
	}
	m.cf, m.of = false, false
	return nil
}

func (m *Machine) storeHandle(file *os.File) uint32 {
	handle := m.nextHandle
	m.nextHandle++
	m.files[handle] = file
	return handle
}

func (m *Machine) assignLogicFlags(result uint32, width int) {
	value := truncate(result, width)
	m.zf = value == 0
	m.sf = signBit(value, width)
	m.pf = parity8(byte(value))
	m.af = false
}

func (m *Machine) updateAddFlags(left, right, result uint32, width int) {
	mask := widthMask(width)
	left &= mask
	right &= mask
	result &= mask
	m.zf = result == 0
	m.sf = signBit(result, width)
	m.cf = uint64(left)+uint64(right) > uint64(mask)
	m.of = ((^(left ^ right)) & (left ^ result) & signMask(width)) != 0
	m.af = ((left ^ right ^ result) & 0x10) != 0
	m.pf = parity8(byte(result))
}

func (m *Machine) updateSubFlags(left, right, result uint32, width int) {
	mask := widthMask(width)
	left &= mask
	right &= mask
	result &= mask
	m.zf = result == 0
	m.sf = signBit(result, width)
	m.cf = left < right
	m.of = (((left ^ right) & (left ^ result)) & signMask(width)) != 0
	m.af = ((left ^ right ^ result) & 0x10) != 0
	m.pf = parity8(byte(result))
}

func (m *Machine) currentEFlags() uint32 {
	var value uint32 = 0x2
	if m.cf {
		value |= 1 << 0
	}
	if m.pf {
		value |= 1 << 2
	}
	if m.af {
		value |= 1 << 4
	}
	if m.zf {
		value |= 1 << 6
	}
	if m.sf {
		value |= 1 << 7
	}
	if m.df {
		value |= 1 << 10
	}
	if m.of {
		value |= 1 << 11
	}
	return value
}

func (m *Machine) applyEFlags(value uint32) {
	m.cf = value&(1<<0) != 0
	m.pf = value&(1<<2) != 0
	m.af = value&(1<<4) != 0
	m.zf = value&(1<<6) != 0
	m.sf = value&(1<<7) != 0
	m.df = value&(1<<10) != 0
	m.of = value&(1<<11) != 0
}

func explicitWidth(op Operand) int {
	switch op.Kind {
	case "reg":
		return registerWidth(op.Text)
	case "symbol", "mem":
		return clampWidth(op.Size)
	default:
		return 0
	}
}

func registerWidth(name string) int {
	switch strings.ToLower(name) {
	case "al", "ah", "bl", "bh", "cl", "ch", "dl", "dh":
		return 1
	case "ax", "bx", "cx", "dx", "si", "di", "bp", "sp":
		return 2
	default:
		return 4
	}
}

func clampWidth(width int) int {
	switch width {
	case 1, 2, 4:
		return width
	default:
		return 0
	}
}

func truncate(value uint32, width int) uint32 {
	return value & widthMask(width)
}

func widthMask(width int) uint32 {
	switch width {
	case 1:
		return 0xFF
	case 2:
		return 0xFFFF
	default:
		return 0xFFFFFFFF
	}
}

func signMask(width int) uint32 {
	switch width {
	case 1:
		return 0x80
	case 2:
		return 0x8000
	default:
		return 0x80000000
	}
}

func signExtend(value uint32, width int) int32 {
	switch width {
	case 1:
		return int32(int8(value))
	case 2:
		return int32(int16(value))
	default:
		return int32(value)
	}
}

func signBit(value uint32, width int) bool {
	return (value & signMask(width)) != 0
}

func parity8(v byte) bool {
	return bits.OnesCount8(v)%2 == 0
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func formatHex(value uint32, width int) string {
	switch width {
	case 1:
		return fmt.Sprintf("%02X", value&0xFF)
	case 2:
		return fmt.Sprintf("%04X", value&0xFFFF)
	default:
		return fmt.Sprintf("%08X", value)
	}
}

func formatBin(value uint32, width int) string {
	switch width {
	case 1:
		return fmt.Sprintf("%08b", value&0xFF)
	case 2:
		return fmt.Sprintf("%016b", value&0xFFFF)
	default:
		return fmt.Sprintf("%032b", value)
	}
}

func parseSignedInput(text string, base, bits int) (int64, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, true
	}
	value, err := strconv.ParseInt(text, base, bits)
	return value, err == nil
}

func parseUnsignedInput(text string, base, bits int) (uint64, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, true
	}
	value, err := strconv.ParseUint(text, base, bits)
	return value, err == nil
}

func ansiFg(color int) int {
	base := 30
	if color >= 8 {
		base = 90
		color -= 8
	}
	return base + min(color, 7)
}

func ansiBg(color int) int {
	base := 40
	if color >= 8 {
		base = 100
		color -= 8
	}
	return base + min(color, 7)
}

func bytesCompare(a, b []byte) int {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for i := 0; i < limit; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}

func openCreateFile(name string, access, disposition uint32) (*os.File, error) {
	flags := 0
	switch disposition {
	case 2: // CREATE_ALWAYS
		flags = os.O_CREATE | os.O_TRUNC
	case 3: // OPEN_EXISTING
		flags = 0
	default:
		flags = os.O_CREATE
	}
	if access&0x40000000 != 0 && access&0x80000000 != 0 {
		flags |= os.O_RDWR
	} else if access&0x40000000 != 0 {
		flags |= os.O_WRONLY
	} else {
		flags |= os.O_RDONLY
	}
	return os.OpenFile(name, flags, 0o666)
}

func isPeekableInput(r io.Reader) bool {
	switch r.(type) {
	case *bytes.Buffer, *bytes.Reader, *strings.Reader:
		return true
	default:
		return false
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func virtualKeyCode(ch byte) byte {
	if ch >= 'a' && ch <= 'z' {
		return ch - 'a' + 'A'
	}
	return ch
}
