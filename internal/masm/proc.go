package masm

import (
	"fmt"
	"strconv"
	"strings"

	"masminterpreter/vm"
)

type procParam struct {
	Name      string
	TypeSpec  string
	ValueSize int
	SlotSize  int
}

type procSignature struct {
	Name       string
	Convention string
	Uses       []string
	Params     []procParam
}

type procContext struct {
	sig             procSignature
	aliases         map[string]vm.Operand
	localSymbols    map[string]vm.Symbol
	localBytes      int
	localPatchIndex int
	frameStarted    bool
	explicitCleanup int
}

func (p *Parser) parseProtoLine(lineNo int, line string) error {
	name, sig, err := parseProcSignature(lineNo, line, "proto")
	if err != nil {
		return err
	}
	p.procSigs[strings.ToLower(name)] = sig
	return nil
}

func (p *Parser) beginProc(lineNo int, line string) error {
	name, sig, err := parseProcSignature(lineNo, line, "proc")
	if err != nil {
		return err
	}
	if p.currentProc != nil {
		return fmt.Errorf("line %d: nested PROC is not supported", lineNo)
	}
	p.program.Procedures = append(p.program.Procedures, vm.Procedure{
		Name:   name,
		Labels: map[string]int{},
	})
	p.currentProc = &p.program.Procedures[len(p.program.Procedures)-1]
	p.currentCtx = &procContext{
		sig:             sig,
		aliases:         map[string]vm.Operand{},
		localSymbols:    map[string]vm.Symbol{},
		localPatchIndex: -1,
	}
	p.procSigs[strings.ToLower(name)] = sig
	if len(sig.Params) > 0 || len(sig.Uses) > 0 {
		p.ensureFrameStarted(lineNo)
	}
	p.installParamAliases()
	return nil
}

func (p *Parser) endProc(lineNo int) error {
	if p.currentProc == nil {
		return fmt.Errorf("line %d: ENDP without PROC", lineNo)
	}
	if p.currentCtx != nil && p.currentCtx.localPatchIndex >= 0 {
		p.currentProc.Instructions[p.currentCtx.localPatchIndex].Args[1].Value = int64(p.currentCtx.localBytes)
	}
	p.currentProc = nil
	p.currentCtx = nil
	p.ifStack = nil
	p.whileStack = nil
	p.repeatStack = nil
	return nil
}

func (p *Parser) ensureFrameStarted(lineNo int) {
	if p.currentCtx == nil || p.currentCtx.frameStarted {
		return
	}
	p.currentCtx.frameStarted = true
	p.addInst(lineNo, "__auto_frame__", "push", vm.Operand{Kind: "reg", Text: "ebp"})
	p.addInst(lineNo, "__auto_frame__", "mov", vm.Operand{Kind: "reg", Text: "ebp"}, vm.Operand{Kind: "reg", Text: "esp"})
	for _, reg := range p.currentCtx.sig.Uses {
		p.addInst(lineNo, "__auto_frame__", "push", vm.Operand{Kind: "reg", Text: reg})
	}
}

func (p *Parser) installParamAliases() {
	if p.currentCtx == nil {
		return
	}
	offset := int64(8)
	for _, param := range p.currentCtx.sig.Params {
		p.currentCtx.aliases[strings.ToLower(param.Name)] = vm.Operand{
			Kind:   "mem",
			Base:   "ebp",
			Offset: offset,
			Size:   param.ValueSize,
		}
		offset += int64(param.SlotSize)
	}
}

func (p *Parser) parseLocalLine(lineNo int, line string) error {
	if p.currentProc == nil || p.currentCtx == nil {
		return fmt.Errorf("line %d: LOCAL outside of procedure", lineNo)
	}
	p.ensureFrameStarted(lineNo)
	specs := splitTopLevel(strings.TrimSpace(line[len("local"):]), ',')
	for _, spec := range specs {
		name, valueSize, bytes, err := parseLocalSpec(strings.TrimSpace(spec))
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNo, err)
		}
		p.currentCtx.localBytes += bytes
		offset := -int64(len(p.currentCtx.sig.Uses)*4 + p.currentCtx.localBytes)
		p.currentCtx.aliases[strings.ToLower(name)] = vm.Operand{
			Kind:   "mem",
			Base:   "ebp",
			Offset: offset,
			Size:   valueSize,
		}
	}
	if p.currentCtx.localPatchIndex == -1 {
		p.addInst(lineNo, "__auto_locals__", "sub", vm.Operand{Kind: "reg", Text: "esp"}, vm.Operand{Kind: "imm", Value: 0})
		p.currentCtx.localPatchIndex = len(p.currentProc.Instructions) - 1
	}
	p.currentProc.Instructions[p.currentCtx.localPatchIndex].Args[1].Value = int64(p.currentCtx.localBytes)
	return nil
}

func (p *Parser) rewriteRet(lineNo int, line string, args []vm.Operand) error {
	if p.currentCtx == nil || !p.currentCtx.frameStarted {
		p.addInst(lineNo, line, "ret", args...)
		return nil
	}
	if p.currentCtx.localBytes > 0 {
		p.addInst(lineNo, "__auto_epilogue__", "add", vm.Operand{Kind: "reg", Text: "esp"}, vm.Operand{Kind: "imm", Value: int64(p.currentCtx.localBytes)})
	}
	for i := len(p.currentCtx.sig.Uses) - 1; i >= 0; i-- {
		p.addInst(lineNo, "__auto_epilogue__", "pop", vm.Operand{Kind: "reg", Text: p.currentCtx.sig.Uses[i]})
	}
	p.addInst(lineNo, "__auto_epilogue__", "pop", vm.Operand{Kind: "reg", Text: "ebp"})

	retArgs := args
	if len(retArgs) == 0 && autoCleanupBytes(p.currentCtx.sig) > 0 {
		retArgs = []vm.Operand{{Kind: "imm", Value: int64(autoCleanupBytes(p.currentCtx.sig))}}
	}
	p.addInst(lineNo, line, "ret", retArgs...)
	return nil
}

func autoCleanupBytes(sig procSignature) int {
	if strings.EqualFold(sig.Convention, "c") {
		return 0
	}
	total := 0
	for _, param := range sig.Params {
		total += param.SlotSize
	}
	return total
}

func parseProcSignature(lineNo int, line, keyword string) (string, procSignature, error) {
	upper := strings.ToUpper(line)
	idx := strings.Index(upper, strings.ToUpper(" "+keyword))
	if idx < 0 {
		return "", procSignature{}, fmt.Errorf("line %d: malformed %s line", lineNo, keyword)
	}
	name := strings.TrimSpace(line[:idx])
	if !isIdentifier(name) {
		return "", procSignature{}, fmt.Errorf("line %d: invalid %s name %q", lineNo, keyword, name)
	}
	suffix := strings.TrimSpace(line[idx+len(keyword)+1:])
	sig := procSignature{Name: name, Convention: "stdcall"}
	if suffix == "" {
		return name, sig, nil
	}

	parts := splitTopLevel(suffix, ',')
	head := []string{}
	if len(parts) > 0 {
		head = strings.Fields(strings.TrimSpace(parts[0]))
	}
	paramSpecs := []string{}
	inUses := false
	paramMode := false
	currentParam := ""
	flushParam := func() {
		if strings.TrimSpace(currentParam) != "" {
			paramSpecs = append(paramSpecs, strings.TrimSpace(currentParam))
			currentParam = ""
		}
	}
	for _, token := range head {
		lower := strings.ToLower(strings.TrimSpace(token))
		switch {
		case lower == "":
		case !paramMode && lower == "uses":
			inUses = true
		case !paramMode && keyword == "proc" && (lower == "near" || lower == "near32" || lower == "far" || lower == "private" || lower == "public"):
			continue
		case !paramMode && isCallingConvention(lower):
			sig.Convention = lower
		case !paramMode && inUses && !strings.Contains(token, ":"):
			sig.Uses = append(sig.Uses, strings.ToLower(token))
		default:
			paramMode = true
			inUses = false
			if strings.Contains(token, ":") && currentParam != "" {
				flushParam()
			}
			if currentParam == "" {
				currentParam = token
			} else {
				currentParam += " " + token
			}
		}
	}
	flushParam()
	if len(parts) > 1 {
		for _, part := range parts[1:] {
			part = strings.TrimSpace(part)
			if part != "" {
				paramSpecs = append(paramSpecs, part)
			}
		}
	}
	for _, spec := range paramSpecs {
		param, err := parseParamSpec(spec)
		if err != nil {
			return "", procSignature{}, fmt.Errorf("line %d: %w", lineNo, err)
		}
		sig.Params = append(sig.Params, param)
	}
	return name, sig, nil
}

func parseParamSpec(spec string) (procParam, error) {
	parts := strings.SplitN(strings.TrimSpace(spec), ":", 2)
	if len(parts) != 2 {
		return procParam{}, fmt.Errorf("invalid parameter spec %q", spec)
	}
	name := strings.TrimSpace(parts[0])
	typeSpec := strings.TrimSpace(parts[1])
	if !isIdentifier(name) {
		return procParam{}, fmt.Errorf("invalid parameter name %q", name)
	}
	valueSize := typeSpecValueSize(typeSpec)
	slotSize := valueSize
	if slotSize < 4 {
		slotSize = 4
	}
	return procParam{
		Name:      name,
		TypeSpec:  typeSpec,
		ValueSize: valueSize,
		SlotSize:  slotSize,
	}, nil
}

func parseLocalSpec(spec string) (name string, valueSize int, totalBytes int, err error) {
	parts := strings.SplitN(strings.TrimSpace(spec), ":", 2)
	if len(parts) != 2 {
		return "", 0, 0, fmt.Errorf("invalid LOCAL spec %q", spec)
	}
	left := strings.TrimSpace(parts[0])
	typeSpec := strings.TrimSpace(parts[1])
	count := 1
	if idx := strings.Index(left, "["); idx >= 0 && strings.HasSuffix(left, "]") {
		name = strings.TrimSpace(left[:idx])
		n, parseErr := strconv.Atoi(strings.TrimSpace(left[idx+1 : len(left)-1]))
		if parseErr != nil || n <= 0 {
			return "", 0, 0, fmt.Errorf("invalid LOCAL array count in %q", spec)
		}
		count = n
	} else {
		name = left
	}
	if !isIdentifier(name) {
		return "", 0, 0, fmt.Errorf("invalid LOCAL name %q", name)
	}
	valueSize = typeSpecValueSize(typeSpec)
	totalBytes = valueSize * count
	return name, valueSize, totalBytes, nil
}

func typeSpecValueSize(typeSpec string) int {
	lower := strings.ToLower(strings.TrimSpace(typeSpec))
	switch {
	case strings.Contains(lower, "ptr"), strings.Contains(lower, "near32"), strings.Contains(lower, "handle"):
		return 4
	case strings.Contains(lower, "real10"), strings.Contains(lower, "tbyte"):
		return 10
	case strings.Contains(lower, "real8"):
		return 8
	case strings.Contains(lower, "real4"):
		return 4
	case strings.Contains(lower, "qword"):
		return 8
	case strings.Contains(lower, "word"):
		if strings.Contains(lower, "dword") {
			return 4
		}
		return 2
	case strings.Contains(lower, "byte"):
		return 1
	default:
		return 4
	}
}

func isCallingConvention(name string) bool {
	switch strings.ToLower(name) {
	case "c", "stdcall", "pascal":
		return true
	default:
		return false
	}
}

func (p *Parser) lookupProcAlias(name string) (vm.Operand, bool) {
	if p.currentCtx == nil {
		return vm.Operand{}, false
	}
	op, ok := p.currentCtx.aliases[strings.ToLower(strings.TrimSpace(name))]
	return op, ok
}
