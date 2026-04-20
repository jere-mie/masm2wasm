package vm

import (
	"fmt"
	"math"
)

func (m *Machine) execFXCH(inst Instruction) error {
	idx := 1
	if len(inst.Args) == 1 {
		if inst.Args[0].Kind != "st" {
			return fmt.Errorf("fxch expects an ST operand")
		}
		idx = int(inst.Args[0].Value)
	}
	a, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	b, err := m.fpuPeek(idx)
	if err != nil {
		return err
	}
	if err := m.fpuSet(0, b); err != nil {
		return err
	}
	return m.fpuSet(idx, a)
}

func (m *Machine) execFArithmeticPop(inst Instruction, op string) error {
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
	case 2:
		if inst.Args[0].Kind != "st" || inst.Args[1].Kind != "st" {
			return fmt.Errorf("%s expects ST operands", inst.Op)
		}
		left, err := m.fpuPeek(int(inst.Args[0].Value))
		if err != nil {
			return err
		}
		right, err := m.fpuPeek(int(inst.Args[1].Value))
		if err != nil {
			return err
		}
		if err := m.fpuSet(int(inst.Args[0].Value), apply(left, right)); err != nil {
			return err
		}
		_, err = m.fpuPop()
		return err
	default:
		return fmt.Errorf("%s expects zero or two operands", inst.Op)
	}
}

func (m *Machine) execFReverseArithmeticPop(inst Instruction, op string) error {
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
	case 2:
		if inst.Args[0].Kind != "st" || inst.Args[1].Kind != "st" {
			return fmt.Errorf("%s expects ST operands", inst.Op)
		}
		left, err := m.fpuPeek(int(inst.Args[0].Value))
		if err != nil {
			return err
		}
		right, err := m.fpuPeek(int(inst.Args[1].Value))
		if err != nil {
			return err
		}
		if err := m.fpuSet(int(inst.Args[0].Value), apply(left, right)); err != nil {
			return err
		}
		_, err = m.fpuPop()
		return err
	default:
		return fmt.Errorf("%s expects zero or two operands", inst.Op)
	}
}

func (m *Machine) execFCOMIP(inst Instruction) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("fcomip expects two operands")
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
	_, err = m.fpuPop()
	return err
}

func (m *Machine) execFUCOM(inst Instruction) error {
	idx := 1
	if len(inst.Args) == 1 {
		if inst.Args[0].Kind != "st" {
			return fmt.Errorf("fucom expects an ST operand")
		}
		idx = int(inst.Args[0].Value)
	}
	left, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	right, err := m.fpuPeek(idx)
	if err != nil {
		return err
	}
	m.setFPUCompareStatus(left, right)
	return nil
}

func (m *Machine) execFUCOMP(inst Instruction) error {
	idx := 1
	if len(inst.Args) == 1 {
		if inst.Args[0].Kind != "st" {
			return fmt.Errorf("fucomp expects an ST operand")
		}
		idx = int(inst.Args[0].Value)
	}
	left, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	right, err := m.fpuPeek(idx)
	if err != nil {
		return err
	}
	m.setFPUCompareStatus(left, right)
	_, err = m.fpuPop()
	return err
}

func (m *Machine) execFUCOMI(inst Instruction) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("fucomi expects two operands")
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

func (m *Machine) execFUCOMIP(inst Instruction) error {
	if len(inst.Args) != 2 {
		return fmt.Errorf("fucomip expects two operands")
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
	_, err = m.fpuPop()
	return err
}

func (m *Machine) execFCOMPP() error {
	left, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	right, err := m.fpuPeek(1)
	if err != nil {
		return err
	}
	m.setFPUCompareStatus(left, right)
	if _, err := m.fpuPop(); err != nil {
		return err
	}
	_, err = m.fpuPop()
	return err
}

func (m *Machine) execFUCOMPP() error {
	left, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	right, err := m.fpuPeek(1)
	if err != nil {
		return err
	}
	m.setFPUCompareStatus(left, right)
	if _, err := m.fpuPop(); err != nil {
		return err
	}
	_, err = m.fpuPop()
	return err
}

func (m *Machine) execFSINCOS() error {
	value, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	sinV, cosV := math.Sincos(value)
	if err := m.fpuSet(0, sinV); err != nil {
		return err
	}
	return m.fpuPush(cosV)
}

func (m *Machine) execFPTAN() error {
	value, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	if err := m.fpuSet(0, math.Tan(value)); err != nil {
		return err
	}
	return m.fpuPush(1.0)
}

func (m *Machine) execFPATAN() error {
	x, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	y, err := m.fpuPeek(1)
	if err != nil {
		return err
	}
	if err := m.fpuSet(1, math.Atan2(y, x)); err != nil {
		return err
	}
	_, err = m.fpuPop()
	return err
}

func (m *Machine) execFPREM(ieee bool) error {
	st0, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	st1, err := m.fpuPeek(1)
	if err != nil {
		return err
	}
	var result float64
	if ieee {
		result = math.Remainder(st0, st1)
	} else {
		result = math.Mod(st0, st1)
	}
	m.fpuStatusWord &^= 1 << 10 // C2=0: reduction complete
	return m.fpuSet(0, result)
}

func (m *Machine) execFSCALE() error {
	st0, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	st1, err := m.fpuPeek(1)
	if err != nil {
		return err
	}
	return m.fpuSet(0, math.Ldexp(st0, int(st1)))
}

func (m *Machine) execFXTRACT() error {
	st0, err := m.fpuPeek(0)
	if err != nil {
		return err
	}
	frac, exp := math.Frexp(st0)
	// x87: ST(0)=significand in [1,2), push exponent
	if err := m.fpuSet(0, frac*2); err != nil {
		return err
	}
	return m.fpuPush(float64(exp - 1))
}

func (m *Machine) execFFREE(inst Instruction) error {
	return nil
}

func (m *Machine) execFDECSTP() error {
	if len(m.fpu) <= 1 {
		return nil
	}
	bottom := m.fpu[0]
	copy(m.fpu[:len(m.fpu)-1], m.fpu[1:])
	m.fpu[len(m.fpu)-1] = bottom
	return nil
}

func (m *Machine) execFSTENV(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("fstenv expects one operand")
	}
	addr, err := m.resolveAddress(inst.Args[0])
	if err != nil {
		return err
	}
	if err := m.writeMemory(addr+0, uint32(m.fpuControlWord), 2); err != nil {
		return err
	}
	if err := m.writeMemory(addr+4, uint32(m.fpuStatusWord), 2); err != nil {
		return err
	}
	for off := uint32(8); off < 28; off += 4 {
		if err := m.writeMemory(addr+off, 0, 4); err != nil {
			return err
		}
	}
	return nil
}

func (m *Machine) execFLDENV(inst Instruction) error {
	if len(inst.Args) != 1 {
		return fmt.Errorf("fldenv expects one operand")
	}
	addr, err := m.resolveAddress(inst.Args[0])
	if err != nil {
		return err
	}
	cw, err := m.readMemory(addr+0, 2)
	if err != nil {
		return err
	}
	sw, err := m.readMemory(addr+4, 2)
	if err != nil {
		return err
	}
	m.fpuControlWord = uint16(cw)
	m.fpuStatusWord = uint16(sw)
	return nil
}
