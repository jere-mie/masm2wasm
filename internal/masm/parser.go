package masm

import (
	"encoding/binary"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"masminterpreter/vm"
)

var (
	reProc      = regexp.MustCompile(`(?i)^([A-Za-z_@$?.][\w@$?.]*)\s+PROC\b`)
	reEndp      = regexp.MustCompile(`(?i)^([A-Za-z_@$?.][\w@$?.]*)\s+ENDP\b`)
	reData      = regexp.MustCompile(`(?i)^(?:(\w+)\s+)?(BYTE|SBYTE|WORD|SWORD|DWORD|SDWORD|HANDLE|QWORD|REAL4|REAL8|REAL10|DB|DW|DD|DQ|DT|TBYTE)\s*(.*)$`)
	reConstEq   = regexp.MustCompile(`(?i)^([A-Za-z_@$?.][\w@$?.]*)\s*=\s*(.+)$`)
	reConstEqu  = regexp.MustCompile(`(?i)^([A-Za-z_@$?.][\w@$?.]*)\s+EQU\s+(.+)$`)
	reDup       = regexp.MustCompile(`(?i)^(.*?)\s+DUP\s*\((.*)\)$`)
	reProtoLike = regexp.MustCompile(`(?i)^[A-Za-z_@$?.][\w@$?.]*\s+PROTO\b`)
	reMacro     = regexp.MustCompile(`(?i)^([A-Za-z_@$?.][\w@$?.]*)\s+MACRO\b(.*)$`)
	reTextEqu   = regexp.MustCompile(`(?i)^([A-Za-z_@$?.][\w@$?.]*)\s+TEXTEQU\s+(.+)$`)
	reTypeDef   = regexp.MustCompile(`(?i)^([A-Za-z_@$?.][\w@$?.]*)\s+TYPEDEF\s+(.+)$`)
)

var builtinConstants = map[string]int64{
	"do_not_share":                   0,
	"null":                           0,
	"false":                          0,
	"true":                           1,
	"file_attribute_normal":          0x80,
	"generic_read":                   0x80000000,
	"generic_write":                  0x40000000,
	"create_always":                  2,
	"open_existing":                  3,
	"file_begin":                     0,
	"file_current":                   1,
	"file_end":                       2,
	"std_input_handle":               -10,
	"std_output_handle":              -11,
	"std_error_handle":               -12,
	"invalid_handle_value":           -1,
	"heap_zero_memory":               0x00000008,
	"vk_numlock":                     0x90,
	"vk_scroll":                      0x91,
	"vk_shift":                       0x10,
	"vk_tab":                         0x09,
	"vk_control":                     0x11,
	"vk_menu":                        0x12,
	"vk_11":                          0x0C,
	"vk_12":                          0x0D,
	"vk_capital":                     0x14,
	"vk_prior":                       0x21,
	"vk_next":                        0x22,
	"vk_end":                         0x23,
	"vk_home":                        0x24,
	"vk_left":                        0x25,
	"vk_up":                          0x26,
	"vk_right":                       0x27,
	"vk_down":                        0x28,
	"vk_insert":                      0x2D,
	"vk_delete":                      0x2E,
	"vk_f1":                          0x70,
	"vk_f10":                         0x79,
	"vk_f11":                         0x7A,
	"vk_f12":                         0x7B,
	"vk_add":                         0x6B,
	"vk_subtract":                    0x6D,
	"vk_lshift":                      0xA0,
	"vk_rshift":                      0xA1,
	"vk_lcontrol":                    0xA2,
	"vk_rcontrol":                    0xA3,
	"vk_lmenu":                       0xA4,
	"vk_rmenu":                       0xA5,
	"key_event":                      1,
	"right_alt_pressed":              0x0001,
	"left_alt_pressed":               0x0002,
	"right_ctrl_pressed":             0x0004,
	"left_ctrl_pressed":              0x0008,
	"shift_pressed":                  0x0010,
	"numlock_on":                     0x0020,
	"scrolllock_on":                  0x0040,
	"capslock_on":                    0x80,
	"black":                          0x0,
	"blue":                           0x1,
	"green":                          0x2,
	"cyan":                           0x3,
	"red":                            0x4,
	"magenta":                        0x5,
	"brown":                          0x6,
	"lightgray":                      0x7,
	"gray":                           0x8,
	"lightblue":                      0x9,
	"lightgreen":                     0xA,
	"lightcyan":                      0xB,
	"lightred":                       0xC,
	"lightmagenta":                   0xD,
	"yellow":                         0xE,
	"white":                          0xF,
	"mb_ok":                          0,
	"mb_okcancel":                    1,
	"mb_abortretryignore":            2,
	"mb_yesnocancel":                 3,
	"mb_yesno":                       4,
	"mb_retrycancel":                 5,
	"mb_canceltrycontinue":           6,
	"mb_defbutton1":                  0x0000,
	"mb_defbutton2":                  0x0100,
	"mb_defbutton3":                  0x0200,
	"mb_defbutton4":                  0x0300,
	"mb_iconhand":                    0x0010,
	"mb_iconstop":                    0x0010,
	"mb_iconquestion":                0x0020,
	"mb_iconexclamation":             0x0030,
	"mb_iconwarning":                 0x0030,
	"mb_iconasterisk":                0x0040,
	"mb_iconinformation":             0x0040,
	"idok":                           1,
	"idcancel":                       2,
	"idabort":                        3,
	"idretry":                        4,
	"idignore":                       5,
	"idyes":                          6,
	"idno":                           7,
	"idclose":                        8,
	"idhelp":                         9,
	"idtryagain":                     10,
	"idcontinue":                     11,
	"format_message_allocate_buffer": 0x0100,
	"format_message_from_system":     0x1000,
	"@version":                       600,
}

type sourceLine struct {
	Number int
	Text   string
}

type procFixup struct {
	Name   string
	Offset uint32
	Size   int
}

type ifFrame struct {
	nextLabel string
	endLabel  string
}

type whileFrame struct {
	startLabel string
	endLabel   string
}

type repeatFrame struct {
	startLabel string
}

type macroParam struct {
	Name        string
	Required    bool
	DefaultText string
}

type macroDef struct {
	Name   string
	Params []macroParam
	Body   []string
}

type Parser struct {
	lines          []sourceLine
	program        vm.Program
	symbols        map[string]vm.Symbol
	exprSymbols    map[string]vm.Symbol
	constants      map[string]int64
	textConstants  map[string]string
	typeAliases    map[string]string
	procSigs       map[string]procSignature
	aggregates     map[string]aggregateDef
	aggregateStack []aggregateBuilder
	currentProc    *vm.Procedure
	currentCtx     *procContext
	section        string
	lastDataIndex  int
	currentOffset  int64
	labelCounter   int
	ifStack        []ifFrame
	whileStack     []whileFrame
	repeatStack    []repeatFrame
	procFixups     []procFixup
}

func Parse(source string) (*vm.Program, error) {
	lines, err := preprocess(source)
	if err != nil {
		return nil, err
	}
	p := &Parser{
		lines:         lines,
		symbols:       map[string]vm.Symbol{},
		exprSymbols:   map[string]vm.Symbol{},
		constants:     map[string]int64{},
		textConstants: map[string]string{},
		typeAliases:   map[string]string{},
		procSigs:      map[string]procSignature{},
		aggregates:    map[string]aggregateDef{},
		lastDataIndex: -1,
		currentOffset: 4,
		program:       vm.Program{Data: make([]byte, 4)},
	}
	for k, v := range builtinConstants {
		p.constants[k] = v
	}
	seedBuiltinAggregates(p)
	if err := p.parse(); err != nil {
		return nil, err
	}
	if err := p.finalizeProcedureRefs(); err != nil {
		return nil, err
	}
	if err := p.program.Validate(); err != nil {
		return nil, err
	}
	return &p.program, nil
}

func (p *Parser) makeScopedSymbolName(name string) string {
	if p.currentProc == nil {
		return name
	}
	return fmt.Sprintf("%s$%s", p.currentProc.Name, name)
}

func (p *Parser) lookupScopedSymbol(name string) (vm.Symbol, bool) {
	lower := strings.ToLower(strings.TrimSpace(name))
	if p.currentCtx != nil {
		if symbol, ok := p.currentCtx.localSymbols[lower]; ok {
			return symbol, true
		}
	}
	symbol, ok := p.symbols[lower]
	return symbol, ok
}

func (p *Parser) lookupScopedExprSymbol(name string) (vm.Symbol, bool) {
	if symbol, ok := p.lookupScopedSymbol(name); ok {
		return symbol, true
	}
	symbol, ok := p.exprSymbols[strings.ToLower(strings.TrimSpace(name))]
	return symbol, ok
}

func preprocess(source string) ([]sourceLine, error) {
	raw := strings.Split(source, "\n")
	lines := make([]sourceLine, 0, len(raw))
	var pending strings.Builder
	pendingLine := 0
	commentDelim := ""
	for i, rawLine := range raw {
		line := strings.TrimSpace(stripComment(rawLine))
		if commentDelim != "" {
			trimmed := strings.TrimSpace(line)
			if trimmed == commentDelim || strings.HasSuffix(trimmed, commentDelim) {
				commentDelim = ""
			}
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "comment ") {
			rest := strings.TrimSpace(line[len("comment "):])
			if rest != "" {
				commentDelim = string(rest[0])
				continue
			}
		}
		if line == "" {
			continue
		}
		if pending.Len() == 0 {
			pendingLine = i + 1
		} else {
			pending.WriteByte(' ')
		}
		continued := continuesLine(line)
		line = strings.TrimSpace(strings.TrimSuffix(line, `\`))
		pending.WriteString(line)
		if continued {
			continue
		}
		lines = append(lines, sourceLine{Number: pendingLine, Text: pending.String()})
		pending.Reset()
	}
	if pending.Len() > 0 {
		lines = append(lines, sourceLine{Number: pendingLine, Text: pending.String()})
	}
	expanded, err := expandMacros(lines)
	if err != nil {
		return nil, err
	}
	return expandCompileTime(expanded)
}

func continuesLine(line string) bool {
	return strings.HasSuffix(line, ",") || strings.HasSuffix(line, `\`)
}

func expandMacros(lines []sourceLine) ([]sourceLine, error) {
	macros := map[string]macroDef{}
	out := make([]sourceLine, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i].Text)
		if line == "" {
			continue
		}
		if match := reMacro.FindStringSubmatch(line); match != nil && !isNonMacroDirective(match[1]) {
			name := match[1]
			params, err := parseMacroParams(strings.TrimSpace(match[2]))
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lines[i].Number, err)
			}
			body := []string{}
			for i++; i < len(lines); i++ {
				current := strings.TrimSpace(lines[i].Text)
				if strings.EqualFold(current, "ENDM") {
					break
				}
				body = append(body, current)
			}
			if i >= len(lines) || !strings.EqualFold(strings.TrimSpace(lines[i].Text), "ENDM") {
				return nil, fmt.Errorf("line %d: unterminated MACRO %q", lines[i-1].Number, name)
			}
			macros[strings.ToLower(name)] = macroDef{Name: name, Params: params, Body: body}
			continue
		}
		expanded, err := expandMacroLine(lines[i], macros, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, expanded...)
	}
	return out, nil
}

func isNonMacroDirective(name string) bool {
	switch strings.ToLower(name) {
	case "title", "name", "include", "includelib", "option", "extern", "externdef":
		return true
	default:
		return false
	}
}

func expandMacroLine(line sourceLine, macros map[string]macroDef, depth int) ([]sourceLine, error) {
	if depth > 32 {
		return nil, fmt.Errorf("line %d: macro expansion limit exceeded", line.Number)
	}
	op, rest := splitOpcode(strings.TrimSpace(line.Text))
	if op == "" {
		return []sourceLine{line}, nil
	}
	def, ok := macros[strings.ToLower(op)]
	if !ok {
		return []sourceLine{line}, nil
	}
	args := []string{}
	if strings.TrimSpace(rest) != "" {
		args = splitTopLevel(rest, ',')
	}
	values, err := bindMacroArgs(line.Number, def, args)
	if err != nil {
		return nil, err
	}
	localNames := collectMacroLocals(def.Body)
	localValues := map[string]string{}
	for _, name := range localNames {
		localValues[strings.ToLower(name)] = fmt.Sprintf("__macro_%s_%d_%d_%s", strings.ToLower(def.Name), line.Number, depth, name)
	}
	var out []sourceLine
	for _, bodyLine := range def.Body {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(bodyLine)), "local ") {
			continue
		}
		expandedText := substituteMacroText(bodyLine, values, localValues)
		expandedLines, err := expandMacroLine(sourceLine{Number: line.Number, Text: expandedText}, macros, depth+1)
		if err != nil {
			return nil, err
		}
		out = append(out, expandedLines...)
	}
	return out, nil
}

func parseMacroParams(spec string) ([]macroParam, error) {
	if spec == "" {
		return nil, nil
	}
	parts := splitTopLevel(spec, ',')
	params := make([]macroParam, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		params = append(params, parseMacroParam(part))
	}
	return params, nil
}

func parseMacroParam(spec string) macroParam {
	param := macroParam{}
	defaultSplit := strings.SplitN(spec, ":=", 2)
	head := strings.TrimSpace(defaultSplit[0])
	if len(defaultSplit) == 2 {
		param.DefaultText = strings.TrimSpace(defaultSplit[1])
	}
	if idx := strings.Index(head, ":"); idx >= 0 {
		param.Name = strings.TrimSpace(head[:idx])
		tail := strings.TrimSpace(head[idx+1:])
		param.Required = strings.EqualFold(tail, "REQ")
	} else {
		param.Name = strings.TrimSpace(head)
	}
	return param
}

func bindMacroArgs(lineNo int, def macroDef, args []string) (map[string]string, error) {
	if len(args) > len(def.Params) {
		return nil, fmt.Errorf("line %d: macro %q received too many arguments", lineNo, def.Name)
	}
	values := map[string]string{}
	for i, param := range def.Params {
		if i < len(args) && strings.TrimSpace(args[i]) != "" {
			value := strings.TrimSpace(args[i])
			if payload, grouped := unwrapAggregateLiteral(value); grouped {
				value = payload
			}
			values[strings.ToLower(param.Name)] = value
			continue
		}
		if param.DefaultText != "" {
			value := strings.TrimSpace(param.DefaultText)
			if payload, grouped := unwrapAggregateLiteral(value); grouped {
				value = payload
			}
			values[strings.ToLower(param.Name)] = value
			continue
		}
		if param.Required {
			return nil, fmt.Errorf("line %d: macro %q requires argument %q", lineNo, def.Name, param.Name)
		}
		values[strings.ToLower(param.Name)] = ""
	}
	return values, nil
}

func collectMacroLocals(body []string) []string {
	var locals []string
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(trimmed), "local ") {
			continue
		}
		for _, part := range splitTopLevel(strings.TrimSpace(trimmed[len("local "):]), ',') {
			part = strings.TrimSpace(part)
			if idx := strings.Index(part, ":"); idx >= 0 {
				part = strings.TrimSpace(part[:idx])
			}
			if part != "" {
				locals = append(locals, part)
			}
		}
	}
	return locals
}

func substituteMacroText(text string, values, locals map[string]string) string {
	text = replaceMacroAmpersandRefs(text, values)
	text = replaceMacroAmpersandRefs(text, locals)
	text = replaceMacroIdentifiers(text, locals)
	text = replaceMacroIdentifiers(text, values)
	return text
}

func replaceMacroAmpersandRefs(text string, replacements map[string]string) string {
	if len(replacements) == 0 || !strings.Contains(text, "&") {
		return text
	}
	var out strings.Builder
	for i := 0; i < len(text); {
		if text[i] != '&' {
			out.WriteByte(text[i])
			i++
			continue
		}
		j := i + 1
		for j < len(text) && isIdentPart(text[j]) {
			j++
		}
		if j == i+1 {
			out.WriteByte(text[i])
			i++
			continue
		}
		name := text[i+1 : j]
		if value, ok := replacements[strings.ToLower(name)]; ok {
			out.WriteString(value)
			if j < len(text) && text[j] == '&' {
				j++
			}
			i = j
			continue
		}
		out.WriteString(text[i:j])
		i = j
	}
	return out.String()
}

func replaceMacroIdentifiers(text string, replacements map[string]string) string {
	if len(replacements) == 0 {
		return text
	}
	var out strings.Builder
	var quote rune
	for i := 0; i < len(text); {
		ch := rune(text[i])
		switch {
		case quote != 0:
			out.WriteByte(text[i])
			if ch == quote {
				quote = 0
			}
			i++
		case ch == '\'' || ch == '"':
			quote = ch
			out.WriteByte(text[i])
			i++
		case isIdentStart(text[i]):
			j := i + 1
			for j < len(text) && isIdentPart(text[j]) {
				j++
			}
			ident := text[i:j]
			if value, ok := replacements[strings.ToLower(ident)]; ok {
				out.WriteString(value)
			} else {
				out.WriteString(ident)
			}
			i = j
		default:
			out.WriteByte(text[i])
			i++
		}
	}
	return out.String()
}

func (p *Parser) parse() error {
parseLoop:
	for _, src := range p.lines {
		line := strings.TrimSpace(src.Text)
		if line == "" {
			continue
		}
		if len(p.aggregateStack) > 0 {
			if err := p.parseAggregateLine(src.Number, line); err != nil {
				return err
			}
			continue
		}
		if _, _, _, ok := parseAggregateStart(line); ok {
			if err := p.beginAggregate(src.Number, line); err != nil {
				return err
			}
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.EqualFold(opcodeWord(line), "title"):
			continue
		case strings.EqualFold(opcodeWord(line), "name"):
			continue
		case strings.EqualFold(opcodeWord(line), "include") || strings.EqualFold(opcodeWord(line), "includelib") || strings.EqualFold(opcodeWord(line), "option"):
			continue
		case strings.EqualFold(opcodeWord(line), "extern") || strings.EqualFold(opcodeWord(line), "externdef") || strings.EqualFold(opcodeWord(line), "extrn"):
			continue
		case strings.EqualFold(opcodeWord(line), "public") || strings.EqualFold(opcodeWord(line), "private") || strings.EqualFold(opcodeWord(line), "assume"):
			continue
		case lower == ".nolist" || lower == ".list":
			continue
		case lower == ".386" || lower == ".386p" || lower == ".486" || lower == ".486p" || lower == ".586" || lower == ".586p" || lower == ".686" || lower == ".686p":
			continue
		case lower == ".mmx" || lower == ".xmm":
			continue
		case strings.HasPrefix(lower, ".model") || strings.HasPrefix(lower, ".stack"):
			continue
		case lower == ".startup" || strings.HasPrefix(lower, ".exit"):
			continue
		case lower == ".const":
			p.section = "const"
			continue
		case strings.HasPrefix(lower, ".data"):
			p.section = "data"
			continue
		case lower == ".code":
			p.section = "code"
			continue
		case lower == "end":
			break parseLoop
		case strings.HasPrefix(lower, "end "):
			fields := strings.Fields(line)
			if len(fields) > 1 {
				p.program.Entry = fields[1]
			}
			break parseLoop
		case reProtoLike.MatchString(line):
			if err := p.parseProtoLine(src.Number, line); err != nil {
				return err
			}
			continue
		case reTypeDef.MatchString(line):
			if err := p.parseTypeDefLine(src.Number, line); err != nil {
				return err
			}
			continue
		}

		if match := reConstEq.FindStringSubmatch(line); match != nil {
			value, err := p.evalExpr(src.Number, match[2])
			if err != nil {
				return err
			}
			p.constants[strings.ToLower(match[1])] = value
			continue
		}
		if match := reTextEqu.FindStringSubmatch(line); match != nil {
			p.textConstants[strings.ToLower(match[1])] = strings.TrimSpace(match[2])
			continue
		}
		if match := reConstEqu.FindStringSubmatch(line); match != nil {
			value, err := p.evalExpr(src.Number, match[2])
			if err == nil {
				p.constants[strings.ToLower(match[1])] = value
				continue
			}
			textValue := strings.TrimSpace(match[2])
			if textValue != "" {
				p.textConstants[strings.ToLower(match[1])] = textValue
				continue
			}
			return err
		}

		switch p.section {
		case "const":
			return fmt.Errorf("line %d: only constant definitions are supported in .const", src.Number)
		case "data":
			if err := p.parseDataLine(src.Number, line); err != nil {
				return err
			}
		case "code":
			if err := p.parseCodeLine(src.Number, line); err != nil {
				return err
			}
		default:
			return fmt.Errorf("line %d: content outside .data/.code: %s", src.Number, line)
		}
	}
	if p.currentProc != nil {
		return fmt.Errorf("unterminated procedure %q", p.currentProc.Name)
	}
	if len(p.aggregateStack) != 0 {
		return fmt.Errorf("unterminated STRUCT/UNION definition")
	}
	if len(p.ifStack) != 0 {
		return fmt.Errorf("unterminated .IF block")
	}
	if len(p.whileStack) != 0 {
		return fmt.Errorf("unterminated .WHILE block")
	}
	if len(p.repeatStack) != 0 {
		return fmt.Errorf("unterminated .REPEAT block")
	}
	return nil
}

func (p *Parser) parseDataLine(lineNo int, line string) error {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "align ") {
		alignArg := strings.TrimSpace(line[len("align "):])
		alignSize := p.typeSize(alignArg)
		if alignSize == 0 {
			value, err := p.evalExpr(lineNo, alignArg)
			if err != nil {
				return fmt.Errorf("line %d: invalid ALIGN operand %q", lineNo, alignArg)
			}
			alignSize = int(value)
		}
		padTo := alignValue(len(p.program.Data), alignSize)
		if padTo > len(p.program.Data) {
			p.program.Data = append(p.program.Data, make([]byte, padTo-len(p.program.Data))...)
		}
		p.currentOffset = int64(len(p.program.Data))
		p.lastDataIndex = -1
		return nil
	}
	if name, decl, ok := parseLabelDecl(line); ok {
		elemSize := p.typeSize(decl)
		if elemSize == 0 {
			elemSize = typeSpecValueSize(decl)
		}
		if elemSize == 0 {
			return fmt.Errorf("line %d: unknown LABEL type %q", lineNo, decl)
		}
		symbol := vm.Symbol{
			Name:     name,
			Address:  uint32(len(p.program.Data)),
			Size:     0,
			Length:   0,
			ElemSize: uint32(elemSize),
			Decl:     decl,
		}
		if p.currentCtx != nil {
			symbol.Name = p.makeScopedSymbolName(name)
			p.currentCtx.localSymbols[strings.ToLower(name)] = symbol
		}
		p.addRuntimeSymbol(symbol)
		p.lastDataIndex = len(p.program.Symbols) - 1
		p.currentOffset = int64(len(p.program.Data))
		return nil
	}
	match := reData.FindStringSubmatch(line)
	name := ""
	decl := ""
	initExpr := ""
	if match != nil {
		name = strings.TrimSpace(match[1])
		decl = normalizeDecl(match[2])
		initExpr = strings.TrimSpace(match[3])
	} else {
		var ok bool
		name, decl, initExpr, ok = parseDeclTokens(line)
		if !ok || !isIdentifier(name) || (p.typeSize(decl) == 0 && !p.isAggregateType(decl)) {
			first, rest := splitOpcode(line)
			if (p.isAggregateType(first) || p.typeSize(first) != 0) && strings.TrimSpace(rest) != "" {
				name = ""
				decl = first
				initExpr = rest
			} else {
				return fmt.Errorf("line %d: unsupported data declaration: %s", lineNo, line)
			}
		}
	}
	if initExpr == "" {
		initExpr = "?"
	}

	data, items, err := p.parseTypeInitializers(lineNo, decl, initExpr)
	if err != nil {
		return err
	}
	elemSize := p.typeSize(decl)
	if elemSize == 0 {
		return fmt.Errorf("line %d: unsupported declaration type %q", lineNo, decl)
	}

	if name == "" {
		if p.lastDataIndex < 0 {
			p.program.Data = append(p.program.Data, data...)
			p.currentOffset = int64(len(p.program.Data))
			return nil
		}
		symbol := &p.program.Symbols[p.lastDataIndex]
		if symbol.Decl != decl {
			p.program.Data = append(p.program.Data, data...)
			p.currentOffset = int64(len(p.program.Data))
			return nil
		}
		symbol.Size += uint32(len(data))
		symbol.Length += uint32(items)
		p.program.Data = append(p.program.Data, data...)
		p.currentOffset = int64(len(p.program.Data))
		return nil
	}

	symbol := vm.Symbol{
		Name:     name,
		Address:  uint32(len(p.program.Data)),
		Size:     uint32(len(data)),
		Length:   uint32(items),
		ElemSize: uint32(elemSize),
		Decl:     decl,
	}
	if p.currentCtx != nil {
		scopedName := p.makeScopedSymbolName(name)
		symbol.Name = scopedName
		p.currentCtx.localSymbols[strings.ToLower(name)] = symbol
	}
	p.program.Data = append(p.program.Data, data...)
	baseIndex := len(p.program.Symbols)
	p.addRuntimeSymbol(symbol)
	if def, ok := p.aggregates[strings.ToLower(decl)]; ok {
		p.addAggregateRuntimeSymbols(symbol.Name, symbol.Address, def)
	}
	p.lastDataIndex = baseIndex
	p.currentOffset = int64(len(p.program.Data))
	return nil
}

func (p *Parser) parseInitializers(lineNo int, decl, initExpr string) ([]byte, int, error) {
	var data []byte
	var count int
	for _, part := range splitTopLevel(initExpr, ',') {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if replacement, ok := p.textConstants[strings.ToLower(part)]; ok {
			if payload, grouped := unwrapAggregateLiteral(replacement); grouped {
				replacement = payload
			}
			payload, items, err := p.parseInitializers(lineNo, decl, replacement)
			if err != nil {
				return nil, 0, err
			}
			data = append(data, payload...)
			count += items
			continue
		}
		if dup := reDup.FindStringSubmatch(part); dup != nil {
			repeat, err := p.evalExpr(lineNo, dup[1])
			if err != nil {
				return nil, 0, err
			}
			if repeat < 0 {
				return nil, 0, fmt.Errorf("line %d: DUP count cannot be negative", lineNo)
			}
			payload, items, err := p.parseInitializers(lineNo, decl, dup[2])
			if err != nil {
				return nil, 0, err
			}
			for i := int64(0); i < repeat; i++ {
				data = append(data, payload...)
				count += items
			}
			continue
		}
		p.currentOffset = int64(len(p.program.Data) + len(data))
		bytes, items, err := p.initBytes(lineNo, decl, part)
		if err != nil {
			return nil, 0, err
		}
		data = append(data, bytes...)
		count += items
	}
	return data, count, nil
}

func (p *Parser) initBytes(lineNo int, decl, token string) ([]byte, int, error) {
	elemSize := declSize(decl)
	if token == "?" {
		return make([]byte, elemSize), 1, nil
	}
	if quoted(token) && decl == "BYTE" {
		value, err := parseStringLiteral(token)
		if err != nil {
			return nil, 0, fmt.Errorf("line %d: %w", lineNo, err)
		}
		return []byte(value), len(value), nil
	}
	if decl == "REAL4" || decl == "REAL8" || decl == "REAL10" {
		value, err := parseFloatToken(token)
		if err != nil {
			return nil, 0, fmt.Errorf("line %d: %w", lineNo, err)
		}
		buf := make([]byte, elemSize)
		switch decl {
		case "REAL4":
			binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(value)))
		case "REAL8":
			binary.LittleEndian.PutUint64(buf, math.Float64bits(value))
		case "REAL10":
			binary.LittleEndian.PutUint64(buf[:8], math.Float64bits(value))
		}
		return buf, 1, nil
	}

	value, err := p.evalExpr(lineNo, token)
	if err != nil {
		if elemSize == 4 && isIdentifier(strings.TrimSpace(token)) {
			p.procFixups = append(p.procFixups, procFixup{
				Name:   strings.TrimSpace(token),
				Offset: uint32(p.currentOffset),
				Size:   elemSize,
			})
			return make([]byte, elemSize), 1, nil
		}
		return nil, 0, err
	}
	buf := make([]byte, elemSize)
	switch elemSize {
	case 1:
		buf[0] = byte(value)
	case 2:
		binary.LittleEndian.PutUint16(buf, uint16(value))
	case 4:
		binary.LittleEndian.PutUint32(buf, uint32(value))
	case 8:
		binary.LittleEndian.PutUint64(buf, uint64(value))
	default:
		return nil, 0, fmt.Errorf("line %d: unsupported element width %d", lineNo, elemSize)
	}
	return buf, 1, nil
}

func (p *Parser) finalizeProcedureRefs() error {
	procAddrs := map[string]uint32{}
	for i := range p.program.Procedures {
		addr := uint32(0x70000000 + i*4)
		p.program.Procedures[i].Address = addr
		procAddrs[strings.ToLower(p.program.Procedures[i].Name)] = addr
	}
	for _, fixup := range p.procFixups {
		addr, ok := procAddrs[strings.ToLower(fixup.Name)]
		if !ok {
			return fmt.Errorf("unknown procedure %q referenced in data", fixup.Name)
		}
		if int(fixup.Offset)+fixup.Size > len(p.program.Data) {
			return fmt.Errorf("procedure fixup for %q is out of range", fixup.Name)
		}
		switch fixup.Size {
		case 4:
			binary.LittleEndian.PutUint32(p.program.Data[fixup.Offset:], addr)
		default:
			return fmt.Errorf("unsupported procedure fixup size %d for %q", fixup.Size, fixup.Name)
		}
	}
	return nil
}

func (p *Parser) parseCodeLine(lineNo int, line string) error {
	if match := reProc.FindStringSubmatch(line); match != nil {
		return p.beginProc(lineNo, line)
	}
	if match := reEndp.FindStringSubmatch(line); match != nil {
		return p.endProc(lineNo)
	}
	if p.currentProc == nil {
		return nil
	}

	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case strings.HasPrefix(lower, ".if "):
		return p.openIf(lineNo, strings.TrimSpace(line[3:]))
	case strings.HasPrefix(lower, ".while"):
		return p.openWhile(lineNo, strings.TrimSpace(line[len(".while"):]))
	case strings.HasPrefix(lower, ".elseif "):
		return p.elseIf(lineNo, strings.TrimSpace(line[7:]))
	case lower == ".else":
		return p.elseBlock(lineNo)
	case lower == ".endif":
		return p.endIf(lineNo)
	case lower == ".repeat":
		return p.openRepeat(lineNo)
	case strings.HasPrefix(lower, ".until"):
		return p.untilRepeat(lineNo, strings.TrimSpace(line[len(".until"):]))
	case lower == ".endw":
		return p.endWhile(lineNo)
	case strings.HasPrefix(lower, "local "):
		return p.parseLocalLine(lineNo, line)
	}

	if label, rest, ok := splitLabel(line); ok {
		p.addLabel(strings.ToLower(label))
		line = strings.TrimSpace(rest)
		if line == "" {
			return nil
		}
		lower = strings.ToLower(line)
	}

	op, rest := splitOpcode(line)
	switch strings.ToLower(op) {
	case "startup":
		return nil
	case "mwrite":
		return p.parseMWrite(lineNo, line, rest, false)
	case "mwriteln":
		return p.parseMWrite(lineNo, line, rest, true)
	case "mwritestring":
		name := strings.TrimSpace(rest)
		addr, err := p.evalExpr(lineNo, "OFFSET "+name)
		if err != nil {
			return err
		}
		p.addInst(lineNo, line, "mov", vm.Operand{Kind: "reg", Text: "edx"}, vm.Operand{Kind: "imm", Value: addr})
		p.addInst(lineNo, line, "call", vm.Operand{Kind: "name", Text: "WriteString"})
		return nil
	case "mwritespace":
		count := int64(1)
		if strings.TrimSpace(rest) != "" {
			value, err := p.evalExpr(lineNo, rest)
			if err != nil {
				return err
			}
			count = value
		}
		p.addInst(lineNo, line, "mwritespace", vm.Operand{Kind: "imm", Value: count})
		return nil
	case "mreadstring":
		name := strings.TrimSpace(rest)
		size, err := p.evalExpr(lineNo, "SIZEOF "+name)
		if err != nil {
			return err
		}
		addr, err := p.evalExpr(lineNo, "OFFSET "+name)
		if err != nil {
			return err
		}
		p.addInst(lineNo, line, "mov", vm.Operand{Kind: "reg", Text: "edx"}, vm.Operand{Kind: "imm", Value: addr})
		p.addInst(lineNo, line, "mov", vm.Operand{Kind: "reg", Text: "ecx"}, vm.Operand{Kind: "imm", Value: size})
		p.addInst(lineNo, line, "call", vm.Operand{Kind: "name", Text: "ReadString"})
		return nil
	case "mgotoxy":
		args := splitTopLevel(rest, ',')
		if len(args) != 2 {
			return fmt.Errorf("line %d: mGotoxy expects X,Y", lineNo)
		}
		x, err := p.evalExpr(lineNo, args[0])
		if err != nil {
			return err
		}
		y, err := p.evalExpr(lineNo, args[1])
		if err != nil {
			return err
		}
		p.addInst(lineNo, line, "mov", vm.Operand{Kind: "reg", Text: "dl"}, vm.Operand{Kind: "imm", Value: x})
		p.addInst(lineNo, line, "mov", vm.Operand{Kind: "reg", Text: "dh"}, vm.Operand{Kind: "imm", Value: y})
		p.addInst(lineNo, line, "call", vm.Operand{Kind: "name", Text: "Gotoxy"})
		return nil
	case "mdumpmem":
		args, err := p.parseOperands(lineNo, "mdumpmem", rest)
		if err != nil {
			return err
		}
		if len(args) != 3 {
			return fmt.Errorf("line %d: mDumpMem expects address, count, component size", lineNo)
		}
		p.addInst(lineNo, line, "mdumpmem", args...)
		return nil
	case "mdump":
		parts := splitTopLevel(rest, ',')
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			return fmt.Errorf("line %d: mDump expects a variable name", lineNo)
		}
		args, err := p.parseOperands(lineNo, "mdump", parts[0])
		if err != nil {
			return err
		}
		showLabel := int64(0)
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			showLabel = 1
		}
		args = append(args, vm.Operand{Kind: "imm", Value: showLabel})
		p.addInst(lineNo, line, "mdump", args...)
		return nil
	case "mshow":
		parts := splitTopLevel(rest, ',')
		if len(parts) == 0 {
			return fmt.Errorf("line %d: mShow expects an argument", lineNo)
		}
		value, err := p.parseOperand(lineNo, "mshow", strings.TrimSpace(parts[0]))
		if err != nil {
			return err
		}
		format := "HIN"
		if len(parts) > 1 {
			format = strings.TrimSpace(parts[1])
			if idx := strings.Index(format, ":="); idx >= 0 {
				format = strings.TrimSpace(format[idx+2:])
			}
			format = strings.Trim(format, "<>")
		}
		p.addInst(lineNo, line, "mshow", value, vm.Operand{Kind: "string", Text: format})
		return nil
	case "mshowregister":
		parts := splitTopLevel(rest, ',')
		if len(parts) != 2 {
			return fmt.Errorf("line %d: mShowRegister expects regName, regValue", lineNo)
		}
		regName := strings.TrimSpace(parts[0])
		regVal, err := p.parseOperand(lineNo, "mshowregister", strings.TrimSpace(parts[1]))
		if err != nil {
			return err
		}
		p.addInst(lineNo, line, "mshowregister", vm.Operand{Kind: "string", Text: regName}, regVal)
		return nil
	case "invoke":
		return p.parseInvoke(lineNo, line, rest)
	case "rep", "repe", "repz", "repne", "repnz":
		prefixedOp, prefixedRest := splitOpcode(rest)
		if prefixedOp == "" {
			return fmt.Errorf("line %d: %s requires an instruction", lineNo, op)
		}
		args, err := p.parseOperands(lineNo, strings.ToLower(prefixedOp), prefixedRest)
		if err != nil {
			return err
		}
		prefix := strings.ToLower(op)
		if prefix == "repz" {
			prefix = "repe"
		} else if prefix == "repnz" {
			prefix = "repne"
		}
		p.addInst(lineNo, line, prefix+"_"+strings.ToLower(prefixedOp), args...)
		return nil
	case "ret":
		args, err := p.parseOperands(lineNo, "ret", rest)
		if err != nil {
			return err
		}
		return p.rewriteRet(lineNo, line, args)
	}

	args, err := p.parseOperands(lineNo, strings.ToLower(op), rest)
	if err != nil {
		return err
	}
	p.addInst(lineNo, line, strings.ToLower(op), args...)
	return nil
}

func (p *Parser) parseInvoke(lineNo int, source, rest string) error {
	parts := splitTopLevel(rest, ',')
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return fmt.Errorf("line %d: INVOKE requires a target", lineNo)
	}
	target := strings.TrimSpace(parts[0])
	if strings.EqualFold(target, "ExitProcess") {
		if len(parts) > 2 {
			return fmt.Errorf("line %d: ExitProcess expects at most one argument", lineNo)
		}
		args := []vm.Operand{}
		if len(parts) == 2 {
			op, err := p.parseOperand(lineNo, "invoke", strings.TrimSpace(parts[1]))
			if err != nil {
				return err
			}
			args = append(args, op)
		}
		p.addInst(lineNo, source, "exit", args...)
		return nil
	}
	if isDirectInvokeBuiltinTarget(target) {
		args := []vm.Operand{{Kind: "name", Text: target}}
		for _, part := range parts[1:] {
			op, err := p.parseOperand(lineNo, "invoke", strings.TrimSpace(part))
			if err != nil {
				return err
			}
			args = append(args, op)
		}
		p.addInst(lineNo, source, "invoke", args...)
		return nil
	}
	if sig, ok := p.procSigs[strings.ToLower(target)]; ok && len(parts) > 1 {
		args := make([]vm.Operand, 0, len(parts)-1)
		for _, part := range parts[1:] {
			op, err := p.parseOperand(lineNo, "invoke", strings.TrimSpace(part))
			if err != nil {
				return err
			}
			args = append(args, op)
		}
		for i := len(args) - 1; i >= 0; i-- {
			p.addInst(lineNo, source, "push", args[i])
		}
		p.addInst(lineNo, source, "call", vm.Operand{Kind: "name", Text: target})
		if strings.EqualFold(sig.Convention, "c") {
			p.addInst(lineNo, source, "add", vm.Operand{Kind: "reg", Text: "esp"}, vm.Operand{Kind: "imm", Value: int64(autoCleanupBytes(sig))})
		}
		return nil
	}
	args := []vm.Operand{{Kind: "name", Text: target}}
	for _, part := range parts[1:] {
		op, err := p.parseOperand(lineNo, "invoke", strings.TrimSpace(part))
		if err != nil {
			return err
		}
		args = append(args, op)
	}
	p.addInst(lineNo, source, "invoke", args...)
	return nil
}

func isDirectInvokeBuiltinTarget(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "printf", "scanf", "system", "fopen", "fclose", "setconsoleoutputcp", "getconsoleoutputcp", "setconsolecp", "getconsolecp":
		return true
	default:
		return false
	}
}

func (p *Parser) parseMWrite(lineNo int, source, rest string, addNewline bool) error {
	text, err := p.parseTextLiteral(lineNo, strings.TrimSpace(rest))
	if err != nil {
		return err
	}
	op := "mwrite"
	if addNewline {
		op = "mwriteln"
	}
	p.addInst(lineNo, source, op, vm.Operand{Kind: "string", Text: text})
	return nil
}

func (p *Parser) parseTextLiteral(lineNo int, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("line %d: expected text literal", lineNo)
	}
	if strings.HasPrefix(text, "<") && strings.HasSuffix(text, ">") {
		payload := text[1 : len(text)-1]
		var out []byte
		for _, part := range splitTopLevel(payload, ',') {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if quoted(part) {
				value, err := parseStringLiteral(part)
				if err != nil {
					return "", fmt.Errorf("line %d: %w", lineNo, err)
				}
				out = append(out, []byte(value)...)
				continue
			}
			value, err := p.evalExpr(lineNo, part)
			if err != nil {
				return "", err
			}
			out = append(out, byte(value))
		}
		return string(out), nil
	}
	if quoted(text) {
		return parseStringLiteral(text)
	}
	return "", fmt.Errorf("line %d: unsupported text literal %q", lineNo, text)
}

func (p *Parser) parseOperands(lineNo int, opcode, rest string) ([]vm.Operand, error) {
	if strings.TrimSpace(rest) == "" {
		return nil, nil
	}
	var args []vm.Operand
	for _, arg := range splitTopLevel(rest, ',') {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		operand, err := p.parseOperand(lineNo, opcode, arg)
		if err != nil {
			if trimmed, ok := trimLooseOperandComment(arg); ok {
				operand, err = p.parseOperand(lineNo, opcode, trimmed)
			}
		}
		if err != nil {
			return nil, err
		}
		args = append(args, operand)
	}
	return args, nil
}

func (p *Parser) parseOperand(lineNo int, opcode, text string) (vm.Operand, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return vm.Operand{}, fmt.Errorf("line %d: empty operand", lineNo)
	}
	lower := strings.ToLower(text)

	if isJumpOp(opcode) || opcode == "call" {
		for _, prefix := range []string{"short ", "near ptr ", "near ", "far ptr ", "far "} {
			if strings.HasPrefix(lower, prefix) {
				text = strings.TrimSpace(text[len(prefix):])
				lower = strings.ToLower(text)
				break
			}
		}
	}

	explicitSize := 0
	for _, spec := range []struct {
		prefix string
		size   int
	}{
		{prefix: "byte ptr", size: 1},
		{prefix: "word ptr", size: 2},
		{prefix: "dword ptr", size: 4},
		{prefix: "sdword ptr", size: 4},
		{prefix: "qword ptr", size: 8},
		{prefix: "tbyte ptr", size: 10},
	} {
		if strings.HasPrefix(lower, spec.prefix) {
			rest := text[len(spec.prefix):]
			if rest != "" {
				first := rest[0]
				if first != ' ' && first != '\t' && first != '[' {
					continue
				}
			}
			explicitSize = spec.size
			text = strings.TrimSpace(rest)
			lower = strings.ToLower(text)
			break
		}
	}

	if replacement, ok := p.textConstants[strings.ToLower(text)]; ok && !strings.EqualFold(strings.TrimSpace(replacement), text) {
		if payload, grouped := unwrapAggregateLiteral(replacement); grouped {
			replacement = payload
		}
		return p.parseOperand(lineNo, opcode, replacement)
	}

	if quoted(text) {
		value, err := parseStringLiteral(text)
		if err != nil {
			return vm.Operand{}, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if len(value) == 1 {
			return vm.Operand{Kind: "imm", Value: int64(value[0]), Text: text}, nil
		}
		return vm.Operand{Kind: "string", Text: value}, nil
	}

	if isJumpOp(opcode) || opcode == "call" {
		if isIdentifier(text) {
			return vm.Operand{Kind: "name", Text: text}, nil
		}
	}

	if strings.HasPrefix(lower, "offset ") || strings.HasPrefix(lower, "addr ") {
		ref := strings.TrimSpace(text[strings.Index(text, " ")+1:])
		if op, ok := p.lookupProcAlias(ref); ok {
			if strings.HasPrefix(lower, "addr ") {
				return op, nil
			}
		}
		if mem, ok, err := p.parseMemoryOperand(lineNo, ref, 0); ok || err != nil {
			if err != nil {
				return vm.Operand{}, err
			}
			if mem.Base != "" || mem.Index != "" {
				return vm.Operand{}, fmt.Errorf("line %d: %s requires a direct memory address", lineNo, strings.Fields(lower)[0])
			}
			addr := mem.Offset
			if mem.Text != "" {
				symbol, ok := p.lookupScopedSymbol(mem.Text)
				if !ok {
					return vm.Operand{}, fmt.Errorf("line %d: unknown symbol %q", lineNo, mem.Text)
				}
				addr += int64(symbol.Address)
			}
			return vm.Operand{Kind: "imm", Text: text, Value: addr}, nil
		}
		value, err := p.evalExpr(lineNo, text)
		if err != nil {
			return vm.Operand{}, err
		}
		return vm.Operand{Kind: "imm", Text: text, Value: value}, nil
	}
	if op, ok := p.lookupProcAlias(text); ok {
		return op, nil
	}
	if member, ok, err := p.parseAggregateMemberOperand(lineNo, text); ok || err != nil {
		return member, err
	}
	if mem, ok, err := p.parseMemoryOperand(lineNo, text, explicitSize); ok || err != nil {
		return mem, err
	}
	if stIndex, ok := parseSTOperand(text); ok {
		return vm.Operand{Kind: "st", Value: int64(stIndex), Text: fmt.Sprintf("st(%d)", stIndex)}, nil
	}
	if isRegister(lower) {
		return vm.Operand{Kind: "reg", Text: lower}, nil
	}
	if symbol, ok := p.lookupScopedSymbol(text); ok {
		return vm.Operand{Kind: "symbol", Text: symbol.Name, Size: int(symbol.ElemSize)}, nil
	}
	if value, err := p.evalExpr(lineNo, text); err == nil {
		return vm.Operand{Kind: "imm", Text: text, Value: value}, nil
	}
	if isIdentifier(text) {
		return vm.Operand{Kind: "name", Text: text}, nil
	}
	return vm.Operand{}, fmt.Errorf("line %d: unsupported operand %q", lineNo, text)
}

func (p *Parser) parseMemoryOperand(lineNo int, text string, explicitSize int) (vm.Operand, bool, error) {
	original := text
	prefix := ""
	inner := ""
	switch {
	case strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]"):
		inner = strings.TrimSpace(text[1 : len(text)-1])
	case strings.Contains(text, "[") && strings.HasSuffix(text, "]"):
		idx := strings.Index(text, "[")
		prefix = strings.TrimSpace(text[:idx])
		inner = strings.TrimSpace(text[idx+1 : len(text)-1])
	default:
		return vm.Operand{}, false, nil
	}

	op := vm.Operand{Kind: "mem", Size: explicitSize}
	if prefix != "" {
		if symbol, ok := p.lookupScopedSymbol(prefix); ok {
			op.Text = symbol.Name
			if op.Size == 0 {
				op.Size = int(symbol.ElemSize)
			}
		} else {
			value, err := p.evalExpr(lineNo, prefix)
			if err != nil {
				return vm.Operand{}, true, fmt.Errorf("line %d: unsupported memory base %q", lineNo, original)
			}
			op.Offset += value
		}
	}

	for _, term := range splitAddressTerms(inner) {
		value := strings.TrimSpace(term.Text)
		if value == "" {
			continue
		}
		sign := int64(1)
		if term.Negative {
			sign = -1
		}
		lower := strings.ToLower(value)
		if reg, scale, ok, err := p.parseScaledRegisterTerm(lineNo, value); ok || err != nil {
			if err != nil {
				return vm.Operand{}, true, fmt.Errorf("line %d: unsupported scaled index %q", lineNo, original)
			}
			if sign < 0 {
				return vm.Operand{}, true, fmt.Errorf("line %d: negative scaled indexes are not supported in %q", lineNo, original)
			}
			if op.Index != "" {
				return vm.Operand{}, true, fmt.Errorf("line %d: too many scaled indexes in memory operand %q", lineNo, original)
			}
			op.Index = reg
			op.Scale = scale
			continue
		}
		if isRegister(lower) {
			if op.Base == "" {
				op.Base = lower
			} else if op.Index == "" {
				op.Index = lower
			} else {
				return vm.Operand{}, true, fmt.Errorf("line %d: too many registers in memory operand %q", lineNo, original)
			}
			if sign < 0 {
				return vm.Operand{}, true, fmt.Errorf("line %d: negative registers are not supported in %q", lineNo, original)
			}
			continue
		}
		if symbol, ok := p.lookupScopedSymbol(value); ok {
			if op.Text == "" {
				op.Text = symbol.Name
				if op.Size == 0 {
					op.Size = int(symbol.ElemSize)
				}
			} else {
				op.Offset += sign * int64(symbol.Address)
			}
			continue
		}
		exprValue, err := p.evalExpr(lineNo, value)
		if err != nil {
			return vm.Operand{}, true, fmt.Errorf("line %d: unsupported memory expression %q", lineNo, original)
		}
		op.Offset += sign * exprValue
	}
	if op.Base == "" && op.Index == "" && op.Text == "" {
		return vm.Operand{}, true, fmt.Errorf("line %d: unsupported memory operand %q", lineNo, original)
	}
	return op, true, nil
}

func (p *Parser) parseTypeDefLine(lineNo int, line string) error {
	match := reTypeDef.FindStringSubmatch(strings.TrimSpace(line))
	if match == nil {
		return fmt.Errorf("line %d: invalid TYPEDEF %q", lineNo, line)
	}
	alias := strings.TrimSpace(match[1])
	typeSpec := strings.TrimSpace(match[2])
	if !isIdentifier(alias) || typeSpec == "" {
		return fmt.Errorf("line %d: invalid TYPEDEF %q", lineNo, line)
	}
	p.typeAliases[strings.ToLower(alias)] = typeSpec
	return nil
}

func (p *Parser) addInst(lineNo int, source, op string, args ...vm.Operand) {
	p.currentProc.Instructions = append(p.currentProc.Instructions, vm.Instruction{
		Op:     op,
		Args:   args,
		Line:   lineNo,
		Source: source,
	})
}

func (p *Parser) addLabel(name string) {
	p.currentProc.Labels[name] = len(p.currentProc.Instructions)
}

func (p *Parser) newSyntheticLabel(prefix string) string {
	p.labelCounter++
	return fmt.Sprintf("__masm_%s_%d", prefix, p.labelCounter)
}

func splitOpcode(line string) (string, string) {
	idx := strings.IndexAny(line, " \t")
	if idx == -1 {
		return line, ""
	}
	return line[:idx], strings.TrimSpace(line[idx+1:])
}

func splitLabel(line string) (string, string, bool) {
	if idx := strings.Index(line, ":"); idx > 0 {
		label := strings.TrimSpace(line[:idx])
		if isIdentifier(label) {
			return label, line[idx+1:], true
		}
	}
	return "", line, false
}

func splitTopLevel(s string, sep rune) []string {
	var parts []string
	var current strings.Builder
	var quote rune
	depth := 0
	angleDepth := 0
	braceDepth := 0
	for _, ch := range s {
		switch {
		case quote != 0:
			current.WriteRune(ch)
			if ch == quote {
				quote = 0
			}
		case ch == '\'' || ch == '"':
			quote = ch
			current.WriteRune(ch)
		case ch == '<':
			angleDepth++
			current.WriteRune(ch)
		case ch == '>':
			if angleDepth > 0 {
				angleDepth--
			}
			current.WriteRune(ch)
		case ch == '{':
			braceDepth++
			current.WriteRune(ch)
		case ch == '}':
			if braceDepth > 0 {
				braceDepth--
			}
			current.WriteRune(ch)
		case ch == '(':
			depth++
			current.WriteRune(ch)
		case ch == ')':
			if depth > 0 {
				depth--
			}
			current.WriteRune(ch)
		case ch == sep && depth == 0 && angleDepth == 0 && braceDepth == 0:
			parts = append(parts, current.String())
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}
	parts = append(parts, current.String())
	return parts
}

type addressTerm struct {
	Text     string
	Negative bool
}

func splitAddressTerms(expr string) []addressTerm {
	var terms []addressTerm
	var current strings.Builder
	var quote rune
	negative := false
	for i, ch := range expr {
		switch {
		case quote != 0:
			current.WriteRune(ch)
			if ch == quote {
				quote = 0
			}
		case ch == '\'' || ch == '"':
			quote = ch
			current.WriteRune(ch)
		case (ch == '+' || ch == '-') && i > 0:
			terms = append(terms, addressTerm{Text: strings.TrimSpace(current.String()), Negative: negative})
			current.Reset()
			negative = ch == '-'
		default:
			if current.Len() == 0 && ch == '-' {
				negative = true
				continue
			}
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		terms = append(terms, addressTerm{Text: strings.TrimSpace(current.String()), Negative: negative})
	}
	return terms
}

func (p *Parser) parseScaledRegisterTerm(lineNo int, text string) (string, int64, bool, error) {
	parts := strings.Split(text, "*")
	if len(parts) != 2 {
		return "", 0, false, nil
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	switch {
	case isRegister(strings.ToLower(left)):
		scale, err := p.evalExpr(lineNo, right)
		if err != nil {
			return "", 0, true, err
		}
		if scale <= 0 {
			return "", 0, true, fmt.Errorf("scaled index must be positive")
		}
		return strings.ToLower(left), scale, true, nil
	case isRegister(strings.ToLower(right)):
		scale, err := p.evalExpr(lineNo, left)
		if err != nil {
			return "", 0, true, err
		}
		if scale <= 0 {
			return "", 0, true, fmt.Errorf("scaled index must be positive")
		}
		return strings.ToLower(right), scale, true, nil
	default:
		return "", 0, false, nil
	}
}

func stripComment(line string) string {
	var builder strings.Builder
	var quote rune
	for _, ch := range line {
		switch {
		case quote != 0:
			builder.WriteRune(ch)
			if ch == quote {
				quote = 0
			}
		case ch == '\'' || ch == '"':
			quote = ch
			builder.WriteRune(ch)
		case ch == ';':
			return builder.String()
		default:
			builder.WriteRune(ch)
		}
	}
	return builder.String()
}

func quoted(s string) bool {
	return len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\''))
}

func unquoteRaw(token string) string {
	if quoted(token) {
		return token[1 : len(token)-1]
	}
	return token
}

func parseStringLiteral(token string) (string, error) {
	if !quoted(token) {
		return "", fmt.Errorf("expected quoted string, got %q", token)
	}
	quote := token[0]
	body := token[1 : len(token)-1]
	body = strings.ReplaceAll(body, `\r`, "\r")
	body = strings.ReplaceAll(body, `\n`, "\n")
	body = strings.ReplaceAll(body, `\\`, `\`)
	if quote == '"' {
		body = strings.ReplaceAll(body, `\"`, `"`)
	} else {
		body = strings.ReplaceAll(body, `\'`, `'`)
	}
	return body, nil
}

func normalizeDecl(name string) string {
	switch strings.ToUpper(name) {
	case "SBYTE":
		return "BYTE"
	case "SWORD":
		return "WORD"
	case "HANDLE":
		return "DWORD"
	case "DB":
		return "BYTE"
	case "DW":
		return "WORD"
	case "DD":
		return "DWORD"
	case "DQ":
		return "QWORD"
	case "DT", "TBYTE":
		return "REAL10"
	default:
		return strings.ToUpper(name)
	}
}

func declSize(name string) int {
	switch normalizeDecl(name) {
	case "BYTE":
		return 1
	case "WORD":
		return 2
	case "DWORD", "SDWORD":
		return 4
	case "REAL4":
		return 4
	case "QWORD":
		return 8
	case "REAL8":
		return 8
	case "REAL10":
		return 10
	default:
		return 0
	}
}

func opcodeWord(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if idx := strings.IndexAny(line, " \t"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func trimLooseOperandComment(text string) (string, bool) {
	for i := 0; i+2 < len(text); i++ {
		if (text[i] == ' ' || text[i] == '\t') && (text[i+1] == ' ' || text[i+1] == '\t') {
			candidate := strings.TrimSpace(text[:i])
			if candidate != "" {
				return candidate, true
			}
		}
	}
	return "", false
}

func parseFloatToken(token string) (float64, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, fmt.Errorf("expected floating-point literal")
	}
	value, err := strconv.ParseFloat(token, 64)
	if err == nil {
		return value, nil
	}
	if numErr, ok := err.(*strconv.NumError); ok && numErr.Err == strconv.ErrRange {
		if strings.HasPrefix(strings.TrimSpace(token), "-") {
			return math.Inf(-1), nil
		}
		return math.Inf(1), nil
	}
	if intValue, intErr := parseNumberToken(token); intErr == nil {
		return float64(intValue), nil
	}
	return 0, fmt.Errorf("invalid floating-point literal %q", token)
}

func parseSTOperand(text string) (int, bool) {
	lower := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(text), " ", ""))
	if !strings.HasPrefix(lower, "st(") || !strings.HasSuffix(lower, ")") {
		return 0, false
	}
	value, err := strconv.Atoi(lower[3 : len(lower)-1])
	if err != nil || value < 0 || value > 7 {
		return 0, false
	}
	return value, true
}

func isRegister(name string) bool {
	switch strings.ToLower(name) {
	case "eax", "ebx", "ecx", "edx", "esi", "edi", "ebp", "esp",
		"ax", "bx", "cx", "dx", "si", "di", "bp", "sp",
		"al", "ah", "bl", "bh", "cl", "ch", "dl", "dh":
		return true
	default:
		return false
	}
}

func isIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, ch := range name {
		if i == 0 {
			if !(ch == '_' || ch == '@' || ch == '$' || ch == '?' || ch == '.' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z') {
				return false
			}
			continue
		}
		if !(ch == '_' || ch == '@' || ch == '$' || ch == '?' || ch == '.' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9') {
			return false
		}
	}
	return true
}

func isJumpOp(op string) bool {
	switch strings.ToLower(op) {
	case "jmp", "je", "jz", "jne", "jnz", "jl", "jle", "jg", "jge", "jng", "jnl", "jnge", "jnle", "ja", "jae", "jb", "jbe", "jc", "jnc", "jnb", "jna", "jnbe", "jnae", "js", "jns", "jo", "jno", "loop", "jcxz", "jecxz":
		return true
	default:
		return false
	}
}
