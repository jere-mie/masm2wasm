package vm

import (
	"fmt"
	"io"
	"time"
)

func (m *Machine) execMacroShowRegister(inst Instruction) error {
	if len(inst.Args) < 2 {
		return fmt.Errorf("mshowregister expects regName and regValue")
	}
	name := inst.Args[0].Text
	width, err := m.operandWidth(inst.Args[1], inst.Args[1], 4)
	if err != nil {
		width = 4
	}
	val, _, err := m.resolveValue(inst.Args[1], width)
	if err != nil {
		return err
	}
	s := fmt.Sprintf("  %s=%08X", name, val)
	_, err = io.WriteString(m.stdout, s)
	return err
}

func (m *Machine) builtinGetDateTime() error {
	// GetDateTime: receives pointer to SYSTEMTIME struct in ESI
	addr := m.regs[regESI]
	now := time.Now()
	// SYSTEMTIME: wYear(2), wMonth(2), wDayOfWeek(2), wDay(2),
	//             wHour(2), wMinute(2), wSecond(2), wMilliseconds(2)
	if err := m.writeMemory(addr+0, uint32(now.Year()), 2); err != nil {
		return err
	}
	if err := m.writeMemory(addr+2, uint32(now.Month()), 2); err != nil {
		return err
	}
	if err := m.writeMemory(addr+4, uint32(now.Weekday()), 2); err != nil {
		return err
	}
	if err := m.writeMemory(addr+6, uint32(now.Day()), 2); err != nil {
		return err
	}
	if err := m.writeMemory(addr+8, uint32(now.Hour()), 2); err != nil {
		return err
	}
	if err := m.writeMemory(addr+10, uint32(now.Minute()), 2); err != nil {
		return err
	}
	if err := m.writeMemory(addr+12, uint32(now.Second()), 2); err != nil {
		return err
	}
	if err := m.writeMemory(addr+14, uint32(now.Nanosecond()/1e6), 2); err != nil {
		return err
	}
	return nil
}

func (m *Machine) builtinWriteStackFrame() error {
	// Simplified stub: print EBP-based stack frame
	ebp := m.regs[regEBP]
	_, err := fmt.Fprintf(m.stdout, "Stack Frame (EBP=%08Xh)\r\n", ebp)
	return err
}

func (m *Machine) builtinWriteStackFrameName() error {
	// Simplified stub: print EBP-based stack frame with proc name from EDX
	ebp := m.regs[regEBP]
	_, err := fmt.Fprintf(m.stdout, "Stack Frame (EBP=%08Xh)\r\n", ebp)
	return err
}

// Win32 API stubs for completeness
func (m *Machine) builtinGetCommandLineA() error {
	// Return pointer to empty command line in EAX
	m.regs[regEAX] = 0
	return nil
}

func (m *Machine) builtinWsprintf() error {
	// wsprintf stub - just set EAX to 0 (no chars written)
	m.regs[regEAX] = 0
	return nil
}

func (m *Machine) builtinHeapSize() error {
	// HeapSize stub - return 0
	m.regs[regEAX] = 0
	return nil
}
