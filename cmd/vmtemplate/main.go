package main

import (
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"

	"masminterpreter/vm"
)

const (
	payloadMarker = "MASM2WASM-PAYLOAD-V1\nLEN="
	headerSize    = len(payloadMarker) + 8 + 1
)

//go:embed program.payload
var embeddedProgram []byte

func main() {
	payload, err := loadProgram()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	program, err := vm.ProgramFromJSON(payload)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	machine := vm.NewMachine(os.Stdin, os.Stdout, os.Stderr)
	if len(os.Args) > 1 {
		machine.SetArgs(os.Args[1:])
	}
	code, err := machine.Run(program)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if code != 0 {
		os.Exit(code)
	}
}

func loadProgram() ([]byte, error) {
	if len(embeddedProgram) < headerSize {
		return nil, fmt.Errorf("embedded payload is truncated")
	}
	if string(embeddedProgram[:len(payloadMarker)]) != payloadMarker {
		return nil, fmt.Errorf("embedded payload marker not found")
	}
	lengthBytes := embeddedProgram[len(payloadMarker) : len(payloadMarker)+8]
	length, err := hex.DecodeString(string(lengthBytes))
	if err != nil || len(length) != 4 {
		return nil, fmt.Errorf("embedded payload length is invalid")
	}
	size := int(uint32(length[0])<<24 | uint32(length[1])<<16 | uint32(length[2])<<8 | uint32(length[3]))
	if size == 0 {
		return nil, fmt.Errorf("translated module does not contain a program payload yet")
	}
	if headerSize+size > len(embeddedProgram) {
		return nil, fmt.Errorf("embedded payload size %d exceeds template bounds", size)
	}
	return append([]byte(nil), embeddedProgram[headerSize:headerSize+size]...), nil
}
