package generator

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"

	"masminterpreter/vm"
)

const (
	payloadMarker   = "MASM2WASM-PAYLOAD-V1\nLEN="
	payloadHeader   = payloadMarker + "00000000\n"
	payloadCapacity = 1 << 20
	payloadNeedle   = payloadHeader + "~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
)

//go:embed vmtemplate.wasm
var templateWASM []byte

type BuildOptions struct {
	OutputPath string
}

func BuildProgram(program *vm.Program, options BuildOptions) error {
	module, err := BuildProgramBytes(program)
	if err != nil {
		return err
	}
	if options.OutputPath == "" || options.OutputPath == "-" {
		_, err = os.Stdout.Write(module)
		return err
	}
	return os.WriteFile(options.OutputPath, module, 0o644)
}

func BuildProgramBytes(program *vm.Program) ([]byte, error) {
	if len(templateWASM) == 0 {
		return nil, fmt.Errorf("embedded template wasm is missing")
	}
	jsonData, err := program.ToJSON()
	if err != nil {
		return nil, err
	}
	if len(jsonData) > payloadCapacity {
		return nil, fmt.Errorf("program IR is %d bytes, which exceeds the %d-byte template payload capacity", len(jsonData), payloadCapacity)
	}

	start := bytes.Index(templateWASM, []byte(payloadNeedle))
	if start < 0 {
		return nil, fmt.Errorf("template payload marker not found")
	}
	out := append([]byte(nil), templateWASM...)
	lengthStart := start + len(payloadMarker)
	copy(out[lengthStart:lengthStart+8], []byte(fmt.Sprintf("%08X", len(jsonData))))
	payloadStart := start + len(payloadHeader)
	copy(out[payloadStart:payloadStart+len(jsonData)], jsonData)
	for i := payloadStart + len(jsonData); i < payloadStart+payloadCapacity; i++ {
		out[i] = '~'
	}
	return out, nil
}
