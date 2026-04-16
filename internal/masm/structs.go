package masm

import (
	"fmt"
	"strings"

	"masminterpreter/vm"
)

type aggregateField struct {
	Name     string
	Decl     string
	TypeName string
	Offset   int64
	Size     int
	Length   int
	ElemSize int
	Default  []byte
}

type aggregateDef struct {
	Name    string
	Kind    string
	Fields  []aggregateField
	Size    int
	Default []byte
}

type aggregateBuilder struct {
	FieldName string
	TypeName  string
	Kind      string
	Fields    []aggregateField
	Size      int
}

func (p *Parser) addCompileSymbol(symbol vm.Symbol) {
	p.exprSymbols[strings.ToLower(symbol.Name)] = symbol
}

func (p *Parser) addRuntimeSymbol(symbol vm.Symbol) {
	p.program.Symbols = append(p.program.Symbols, symbol)
	p.symbols[strings.ToLower(symbol.Name)] = symbol
	p.addCompileSymbol(symbol)
}

func (p *Parser) isAggregateType(name string) bool {
	_, ok := p.aggregates[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func (p *Parser) typeSize(name string) int {
	resolved := p.resolveTypeName(name)
	if size := declSize(p.canonicalTypeName(resolved)); size != 0 {
		return size
	}
	if def, ok := p.aggregates[strings.ToLower(strings.TrimSpace(resolved))]; ok {
		return def.Size
	}
	return 0
}

func (p *Parser) resolveTypeName(name string) string {
	current := strings.TrimSpace(name)
	seen := map[string]struct{}{}
	for {
		key := strings.ToLower(current)
		if _, ok := seen[key]; ok {
			return current
		}
		seen[key] = struct{}{}
		next, ok := p.typeAliases[key]
		if !ok {
			return current
		}
		current = strings.TrimSpace(next)
	}
}

func (p *Parser) canonicalTypeName(name string) string {
	resolved := p.resolveTypeName(name)
	lower := strings.ToLower(strings.TrimSpace(resolved))
	switch {
	case strings.Contains(lower, "ptr"), strings.Contains(lower, "near32"), strings.Contains(lower, "handle"):
		return "DWORD"
	default:
		return normalizeDecl(resolved)
	}
}

func (p *Parser) registerAggregateDef(def aggregateDef) {
	p.aggregates[strings.ToLower(def.Name)] = def
	p.addCompileSymbol(vm.Symbol{
		Name:     def.Name,
		Address:  0,
		Size:     uint32(def.Size),
		Length:   1,
		ElemSize: uint32(def.Size),
		Decl:     def.Name,
	})
	p.addAggregateTypeSymbols(def.Name, 0, def)
}

func (p *Parser) addAggregateTypeSymbols(prefix string, base int64, def aggregateDef) {
	for _, field := range def.Fields {
		name := prefix + "." + field.Name
		symbol := vm.Symbol{
			Name:     name,
			Address:  uint32(base + field.Offset),
			Size:     uint32(field.Size),
			Length:   uint32(maxInt(field.Length, 1)),
			ElemSize: uint32(maxInt(field.ElemSize, maxInt(field.Size, 1))),
			Decl:     fieldDeclName(field),
		}
		p.addCompileSymbol(symbol)
		if field.TypeName != "" {
			child := p.aggregates[strings.ToLower(field.TypeName)]
			p.addAggregateTypeSymbols(name, base+field.Offset, child)
		}
	}
}

func (p *Parser) addAggregateRuntimeSymbols(prefix string, base uint32, def aggregateDef) {
	for _, field := range def.Fields {
		name := prefix + "." + field.Name
		symbol := vm.Symbol{
			Name:     name,
			Address:  base + uint32(field.Offset),
			Size:     uint32(field.Size),
			Length:   uint32(maxInt(field.Length, 1)),
			ElemSize: uint32(maxInt(field.ElemSize, maxInt(field.Size, 1))),
			Decl:     fieldDeclName(field),
		}
		p.addRuntimeSymbol(symbol)
		if field.TypeName != "" {
			child := p.aggregates[strings.ToLower(field.TypeName)]
			p.addAggregateRuntimeSymbols(name, base+uint32(field.Offset), child)
		}
	}
}

func fieldDeclName(field aggregateField) string {
	if field.TypeName != "" {
		return field.TypeName
	}
	return field.Decl
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func alignValue(value int, alignment int) int {
	if alignment <= 1 {
		return value
	}
	rem := value % alignment
	if rem == 0 {
		return value
	}
	return value + alignment - rem
}

func parseAggregateStart(line string) (fieldName string, typeName string, kind string, ok bool) {
	parts := strings.Fields(strings.TrimSpace(line))
	if len(parts) < 2 {
		return "", "", "", false
	}
	switch {
	case len(parts) >= 2 && strings.EqualFold(parts[1], "STRUCT"):
		return "", parts[0], "struct", isIdentifier(parts[0])
	case len(parts) >= 2 && strings.EqualFold(parts[1], "UNION"):
		return "", parts[0], "union", isIdentifier(parts[0])
	case strings.EqualFold(parts[0], "STRUCT") && len(parts) >= 2:
		return parts[1], "__inline_struct_" + strings.ToLower(parts[1]), "struct", isIdentifier(parts[1])
	case strings.EqualFold(parts[0], "UNION") && len(parts) >= 2:
		return parts[1], "__inline_union_" + strings.ToLower(parts[1]), "union", isIdentifier(parts[1])
	default:
		return "", "", "", false
	}
}

func parseEndsLine(line string) (name string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if strings.EqualFold(trimmed, "ENDS") {
		return "", true
	}
	parts := strings.Fields(trimmed)
	if len(parts) == 2 && strings.EqualFold(parts[1], "ENDS") && isIdentifier(parts[0]) {
		return parts[0], true
	}
	return "", false
}

func (p *Parser) beginAggregate(lineNo int, line string) error {
	fieldName, typeName, kind, ok := parseAggregateStart(line)
	if !ok {
		return fmt.Errorf("line %d: malformed aggregate declaration %q", lineNo, line)
	}
	if fieldName == "" {
		fieldName = typeName
	}
	if len(p.aggregateStack) == 0 {
		typeName = fieldName
	} else {
		typeName = fmt.Sprintf("%s$%d", typeName, len(p.aggregateStack)+1)
	}
	p.aggregateStack = append(p.aggregateStack, aggregateBuilder{
		FieldName: fieldName,
		TypeName:  typeName,
		Kind:      kind,
	})
	return nil
}

func (p *Parser) closeAggregate(lineNo int, line string) error {
	if len(p.aggregateStack) == 0 {
		return fmt.Errorf("line %d: ENDS without STRUCT/UNION", lineNo)
	}
	name, ok := parseEndsLine(line)
	if !ok {
		return fmt.Errorf("line %d: malformed ENDS line", lineNo)
	}
	builder := p.aggregateStack[len(p.aggregateStack)-1]
	p.aggregateStack = p.aggregateStack[:len(p.aggregateStack)-1]
	if name != "" && !strings.EqualFold(name, builder.FieldName) && !strings.EqualFold(name, builder.TypeName) {
		return fmt.Errorf("line %d: ENDS name %q does not match %q", lineNo, name, builder.FieldName)
	}
	def := aggregateDef{
		Name:   builder.TypeName,
		Kind:   builder.Kind,
		Fields: append([]aggregateField(nil), builder.Fields...),
		Size:   builder.Size,
	}
	def.Default = make([]byte, def.Size)
	for _, field := range def.Fields {
		copy(def.Default[int(field.Offset):int(field.Offset)+len(field.Default)], field.Default)
	}
	p.registerAggregateDef(def)
	if len(p.aggregateStack) == 0 {
		return nil
	}
	parent := &p.aggregateStack[len(p.aggregateStack)-1]
	field := aggregateField{
		Name:     builder.FieldName,
		TypeName: def.Name,
		Decl:     def.Name,
		Offset:   currentAggregateOffset(*parent),
		Size:     def.Size,
		Length:   1,
		ElemSize: def.Size,
		Default:  append([]byte(nil), def.Default...),
	}
	if parent.Kind == "union" {
		field.Offset = 0
		if def.Size > parent.Size {
			parent.Size = def.Size
		}
	} else {
		parent.Size += def.Size
	}
	parent.Fields = append(parent.Fields, field)
	return nil
}

func currentAggregateOffset(builder aggregateBuilder) int64 {
	if builder.Kind == "union" {
		return 0
	}
	return int64(builder.Size)
}

func (p *Parser) parseAggregateLine(lineNo int, line string) error {
	if _, _, _, ok := parseAggregateStart(line); ok {
		return p.beginAggregate(lineNo, line)
	}
	if _, ok := parseEndsLine(line); ok {
		return p.closeAggregate(lineNo, line)
	}
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(strings.ToLower(trimmed), "align ") {
		alignArg := strings.TrimSpace(trimmed[len("align "):])
		alignSize := p.typeSize(alignArg)
		if alignSize == 0 {
			value, err := p.evalExpr(lineNo, alignArg)
			if err != nil {
				return fmt.Errorf("line %d: invalid ALIGN operand %q", lineNo, alignArg)
			}
			alignSize = int(value)
		}
		current := &p.aggregateStack[len(p.aggregateStack)-1]
		if current.Kind != "union" {
			current.Size = alignValue(current.Size, alignSize)
		}
		return nil
	}
	return p.parseAggregateField(lineNo, line)
}

func parseDeclTokens(line string) (name string, decl string, initExpr string, ok bool) {
	first, rest := splitOpcode(strings.TrimSpace(line))
	if first == "" {
		return "", "", "", false
	}
	second, tail := splitOpcode(rest)
	if second == "" {
		return "", "", "", false
	}
	return first, second, strings.TrimSpace(tail), true
}

func parseLabelDecl(line string) (name string, decl string, ok bool) {
	first, rest := splitOpcode(strings.TrimSpace(line))
	second, tail := splitOpcode(rest)
	if !isIdentifier(first) || !strings.EqualFold(second, "LABEL") {
		return "", "", false
	}
	decl = strings.TrimSpace(tail)
	if decl == "" {
		return "", "", false
	}
	return first, normalizeDecl(decl), true
}

func (p *Parser) parseAggregateField(lineNo int, line string) error {
	name, decl, initExpr, ok := parseDeclTokens(line)
	if !ok || !isIdentifier(name) {
		return fmt.Errorf("line %d: unsupported aggregate field %q", lineNo, line)
	}
	if initExpr == "" {
		initExpr = "?"
	}
	offset := int64(0)
	current := &p.aggregateStack[len(p.aggregateStack)-1]
	if current.Kind != "union" {
		offset = int64(current.Size)
	}
	resolvedDecl := decl
	if !p.isAggregateType(decl) {
		resolvedDecl = p.canonicalTypeName(decl)
	}
	size := p.typeSize(resolvedDecl)
	if size == 0 {
		return fmt.Errorf("line %d: unknown field type %q", lineNo, decl)
	}
	data, items, err := p.parseTypeInitializers(lineNo, resolvedDecl, initExpr)
	if err != nil {
		return err
	}
	field := aggregateField{
		Name:     name,
		Decl:     resolvedDecl,
		Offset:   offset,
		Size:     size,
		Length:   items,
		ElemSize: size,
	}
	if p.isAggregateType(resolvedDecl) {
		field.TypeName = resolvedDecl
		field.ElemSize = size
	} else if elem := declSize(resolvedDecl); elem != 0 {
		field.ElemSize = elem
		field.Size = len(data)
	}
	field.Default = make([]byte, field.Size)
	copy(field.Default, data)
	current.Fields = append(current.Fields, field)
	if current.Kind == "union" {
		if field.Size > current.Size {
			current.Size = field.Size
		}
	} else {
		current.Size += field.Size
	}
	return nil
}

func (p *Parser) parseTypeInitializers(lineNo int, decl, initExpr string) ([]byte, int, error) {
	if p.isAggregateType(decl) {
		return p.parseAggregateInitializers(lineNo, decl, initExpr)
	}
	return p.parseInitializers(lineNo, p.canonicalTypeName(decl), initExpr)
}

func (p *Parser) parseAggregateInitializers(lineNo int, typeName, initExpr string) ([]byte, int, error) {
	def, ok := p.aggregates[strings.ToLower(typeName)]
	if !ok {
		return nil, 0, fmt.Errorf("line %d: unknown aggregate type %q", lineNo, typeName)
	}
	var data []byte
	count := 0
	for _, part := range splitTopLevel(initExpr, ',') {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if replacement, ok := p.textConstants[strings.ToLower(part)]; ok {
			payload, items, err := p.parseAggregateInitializers(lineNo, typeName, replacement)
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
			payload, items, err := p.parseAggregateInitializers(lineNo, typeName, dup[2])
			if err != nil {
				return nil, 0, err
			}
			for i := int64(0); i < repeat; i++ {
				data = append(data, payload...)
				count += items
			}
			continue
		}
		instance, err := p.parseAggregateInstance(lineNo, def, part)
		if err != nil {
			return nil, 0, err
		}
		data = append(data, instance...)
		count++
	}
	return data, count, nil
}

func (p *Parser) parseAggregateInstance(lineNo int, def aggregateDef, token string) ([]byte, error) {
	token = strings.TrimSpace(token)
	if token == "?" {
		return make([]byte, def.Size), nil
	}
	if replacement, ok := p.textConstants[strings.ToLower(token)]; ok {
		token = replacement
	}
	data := append([]byte(nil), def.Default...)
	payload, grouped := unwrapAggregateLiteral(token)
	if !grouped {
		if token == "" || token == "<>" || token == "{}" {
			return data, nil
		}
		if len(def.Fields) == 0 {
			return data, nil
		}
		if err := p.applyAggregateField(lineNo, data, def.Fields[0], token); err != nil {
			return nil, err
		}
		return data, nil
	}
	if def.Kind == "union" {
		for _, part := range splitTopLevel(payload, ',') {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if err := p.applyAggregateField(lineNo, data, def.Fields[0], part); err != nil {
				return nil, err
			}
			return data, nil
		}
		return data, nil
	}
	parts := splitTopLevel(payload, ',')
	for i, field := range def.Fields {
		if i >= len(parts) {
			break
		}
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		if err := p.applyAggregateField(lineNo, data, field, part); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func unwrapAggregateLiteral(token string) (string, bool) {
	token = strings.TrimSpace(token)
	if len(token) >= 2 {
		if (token[0] == '<' && token[len(token)-1] == '>') || (token[0] == '{' && token[len(token)-1] == '}') {
			return strings.TrimSpace(token[1 : len(token)-1]), true
		}
	}
	return token, false
}

func (p *Parser) applyAggregateField(lineNo int, data []byte, field aggregateField, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	offset := int(field.Offset)
	if field.TypeName != "" {
		def := p.aggregates[strings.ToLower(field.TypeName)]
		payload, err := p.parseAggregateInstance(lineNo, def, token)
		if err != nil {
			return err
		}
		copy(data[offset:offset+len(payload)], payload)
		return nil
	}
	payload, _, err := p.parseTypeInitializers(lineNo, field.Decl, token)
	if err != nil {
		return err
	}
	if len(payload) > field.Size {
		return fmt.Errorf("line %d: initializer %q exceeds field %q", lineNo, token, field.Name)
	}
	copy(data[offset:offset+len(payload)], payload)
	return nil
}

func (p *Parser) parseAggregateMemberOperand(lineNo int, text string) (vm.Operand, bool, error) {
	baseText, memberPath, ok := splitMemberAccess(text)
	if !ok {
		return vm.Operand{}, false, nil
	}
	baseOp, def, ok, err := p.parseAggregateBaseOperand(lineNo, baseText)
	if !ok || err != nil {
		return vm.Operand{}, ok, err
	}
	field, fieldOffset, err := p.resolveAggregateField(def, memberPath)
	if err != nil {
		return vm.Operand{}, true, fmt.Errorf("line %d: %w", lineNo, err)
	}
	baseOp.Kind = "mem"
	baseOp.Offset += fieldOffset
	baseOp.Size = maxInt(field.ElemSize, 1)
	return baseOp, true, nil
}

func splitMemberAccess(text string) (string, string, bool) {
	var quote rune
	bracketDepth := 0
	parenDepth := 0
	for i, ch := range text {
		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			}
		case ch == '\'' || ch == '"':
			quote = ch
		case ch == '[':
			bracketDepth++
		case ch == ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case ch == '(':
			parenDepth++
		case ch == ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case ch == '.' && bracketDepth == 0 && parenDepth == 0:
			return strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+1:]), true
		}
	}
	return "", "", false
}

func (p *Parser) parseAggregateBaseOperand(lineNo int, text string) (vm.Operand, aggregateDef, bool, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		inner := strings.TrimSpace(text[1 : len(text)-1])
		if typeName, ptrExpr, ok := parseTypedPointer(inner); ok {
			def, ok := p.aggregates[strings.ToLower(typeName)]
			if !ok {
				return vm.Operand{}, aggregateDef{}, true, fmt.Errorf("unknown aggregate type %q", typeName)
			}
			mem, matched, err := p.parseMemoryOperand(lineNo, ptrExpr, 0)
			if !matched || err != nil {
				if err == nil {
					err = fmt.Errorf("unsupported typed pointer operand %q", text)
				}
				return vm.Operand{}, aggregateDef{}, true, err
			}
			return mem, def, true, nil
		}
	}
	if symbol, ok := p.lookupScopedSymbol(text); ok {
		def, ok := p.aggregates[strings.ToLower(symbol.Decl)]
		if ok {
			return vm.Operand{Kind: "mem", Text: symbol.Name, Size: int(symbol.ElemSize)}, def, true, nil
		}
	}
	mem, matched, err := p.parseMemoryOperand(lineNo, text, 0)
	if matched || err != nil {
		if err != nil {
			return vm.Operand{}, aggregateDef{}, true, err
		}
		if mem.Text != "" {
			if symbol, ok := p.lookupScopedSymbol(mem.Text); ok {
				if def, ok := p.aggregates[strings.ToLower(symbol.Decl)]; ok {
					return mem, def, true, nil
				}
			}
		}
	}
	return vm.Operand{}, aggregateDef{}, false, nil
}

func parseTypedPointer(text string) (typeName string, ptrExpr string, ok bool) {
	lower := strings.ToLower(text)
	idx := strings.Index(lower, " ptr ")
	if idx < 0 {
		return "", "", false
	}
	typeName = strings.TrimSpace(text[:idx])
	ptrExpr = strings.TrimSpace(text[idx+len(" ptr "):])
	return typeName, ptrExpr, typeName != "" && ptrExpr != ""
}

func (p *Parser) resolveAggregateField(def aggregateDef, path string) (aggregateField, int64, error) {
	current := def
	total := int64(0)
	parts := strings.Split(path, ".")
	var field aggregateField
	for i, part := range parts {
		found := false
		for _, candidate := range current.Fields {
			if strings.EqualFold(candidate.Name, strings.TrimSpace(part)) {
				field = candidate
				total += candidate.Offset
				found = true
				break
			}
		}
		if !found {
			return aggregateField{}, 0, fmt.Errorf("unknown member %q in %q", part, path)
		}
		if i < len(parts)-1 {
			if field.TypeName == "" {
				return aggregateField{}, 0, fmt.Errorf("member %q is not an aggregate", field.Name)
			}
			current = p.aggregates[strings.ToLower(field.TypeName)]
		}
	}
	return field, total, nil
}

func seedBuiltinAggregates(p *Parser) {
	register := func(def aggregateDef) { p.registerAggregateDef(def) }
	register(aggregateDef{
		Name: "COORD",
		Kind: "struct",
		Fields: []aggregateField{
			{Name: "X", Decl: "WORD", Offset: 0, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "Y", Decl: "WORD", Offset: 2, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
		},
		Size:    4,
		Default: make([]byte, 4),
	})
	register(aggregateDef{
		Name: "SYSTEMTIME",
		Kind: "struct",
		Fields: []aggregateField{
			{Name: "wYear", Decl: "WORD", Offset: 0, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "wMonth", Decl: "WORD", Offset: 2, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "wDayOfWeek", Decl: "WORD", Offset: 4, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "wDay", Decl: "WORD", Offset: 6, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "wHour", Decl: "WORD", Offset: 8, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "wMinute", Decl: "WORD", Offset: 10, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "wSecond", Decl: "WORD", Offset: 12, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "wMilliseconds", Decl: "WORD", Offset: 14, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
		},
		Size:    16,
		Default: make([]byte, 16),
	})
	register(aggregateDef{
		Name: "FILETIME",
		Kind: "struct",
		Fields: []aggregateField{
			{Name: "loDateTime", Decl: "DWORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "hiDateTime", Decl: "DWORD", Offset: 4, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
		},
		Size:    8,
		Default: make([]byte, 8),
	})
	register(aggregateDef{
		Name: "SMALL_RECT",
		Kind: "struct",
		Fields: []aggregateField{
			{Name: "Left", Decl: "WORD", Offset: 0, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "Top", Decl: "WORD", Offset: 2, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "Right", Decl: "WORD", Offset: 4, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "Bottom", Decl: "WORD", Offset: 6, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
		},
		Size:    8,
		Default: make([]byte, 8),
	})
	register(aggregateDef{
		Name: "CONSOLE_CURSOR_INFO",
		Kind: "struct",
		Fields: []aggregateField{
			{Name: "dwSize", Decl: "DWORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "bVisible", Decl: "DWORD", Offset: 4, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
		},
		Size:    8,
		Default: make([]byte, 8),
	})
	register(aggregateDef{
		Name: "CONSOLE_SCREEN_BUFFER_INFO",
		Kind: "struct",
		Fields: []aggregateField{
			{Name: "dwSize", Decl: "COORD", TypeName: "COORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "dwCursorPosition", Decl: "COORD", TypeName: "COORD", Offset: 4, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "wAttributes", Decl: "WORD", Offset: 8, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "srWindow", Decl: "SMALL_RECT", TypeName: "SMALL_RECT", Offset: 10, Size: 8, Length: 1, ElemSize: 8, Default: make([]byte, 8)},
			{Name: "dwMaximumWindowSize", Decl: "COORD", TypeName: "COORD", Offset: 18, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
		},
		Size:    22,
		Default: make([]byte, 22),
	})
	register(aggregateDef{
		Name: "KEY_EVENT_UCHAR",
		Kind: "union",
		Fields: []aggregateField{
			{Name: "UnicodeChar", Decl: "WORD", Offset: 0, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "AsciiChar", Decl: "BYTE", Offset: 0, Size: 1, Length: 1, ElemSize: 1, Default: make([]byte, 1)},
		},
		Size:    2,
		Default: make([]byte, 2),
	})
	register(aggregateDef{
		Name: "KEY_EVENT_RECORD",
		Kind: "struct",
		Fields: []aggregateField{
			{Name: "bKeyDown", Decl: "DWORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "wRepeatCount", Decl: "WORD", Offset: 4, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "wVirtualKeyCode", Decl: "WORD", Offset: 6, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "wVirtualScanCode", Decl: "WORD", Offset: 8, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "uChar", Decl: "KEY_EVENT_UCHAR", TypeName: "KEY_EVENT_UCHAR", Offset: 10, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "dwControlKeyState", Decl: "DWORD", Offset: 12, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
		},
		Size:    16,
		Default: make([]byte, 16),
	})
	register(aggregateDef{
		Name: "MOUSE_EVENT_RECORD",
		Kind: "struct",
		Fields: []aggregateField{
			{Name: "dwMousePosition", Decl: "COORD", TypeName: "COORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "dwButtonState", Decl: "DWORD", Offset: 4, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "dwMouseControlKeyState", Decl: "DWORD", Offset: 8, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "dwEventFlags", Decl: "DWORD", Offset: 12, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
		},
		Size:    16,
		Default: make([]byte, 16),
	})
	register(aggregateDef{
		Name: "WINDOW_BUFFER_SIZE_RECORD",
		Kind: "struct",
		Fields: []aggregateField{
			{Name: "dwSize", Decl: "COORD", TypeName: "COORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
		},
		Size:    4,
		Default: make([]byte, 4),
	})
	register(aggregateDef{
		Name: "MENU_EVENT_RECORD",
		Kind: "struct",
		Fields: []aggregateField{
			{Name: "dwCommandId", Decl: "DWORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
		},
		Size:    4,
		Default: make([]byte, 4),
	})
	register(aggregateDef{
		Name: "FOCUS_EVENT_RECORD",
		Kind: "struct",
		Fields: []aggregateField{
			{Name: "bSetFocus", Decl: "DWORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
		},
		Size:    4,
		Default: make([]byte, 4),
	})
	register(aggregateDef{
		Name: "INPUT_RECORD_EVENT",
		Kind: "union",
		Fields: []aggregateField{
			{Name: "bKeyDown", Decl: "DWORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "wRepeatCount", Decl: "WORD", Offset: 4, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "wVirtualKeyCode", Decl: "WORD", Offset: 6, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "wVirtualScanCode", Decl: "WORD", Offset: 8, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "uChar", Decl: "KEY_EVENT_UCHAR", TypeName: "KEY_EVENT_UCHAR", Offset: 10, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "dwControlKeyState", Decl: "DWORD", Offset: 12, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "dwMousePosition", Decl: "COORD", TypeName: "COORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "dwButtonState", Decl: "DWORD", Offset: 4, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "dwMouseControlKeyState", Decl: "DWORD", Offset: 8, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "dwEventFlags", Decl: "DWORD", Offset: 12, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "dwSize", Decl: "COORD", TypeName: "COORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "dwCommandId", Decl: "DWORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
			{Name: "bSetFocus", Decl: "DWORD", Offset: 0, Size: 4, Length: 1, ElemSize: 4, Default: make([]byte, 4)},
		},
		Size:    16,
		Default: make([]byte, 16),
	})
	register(aggregateDef{
		Name: "INPUT_RECORD",
		Kind: "struct",
		Fields: []aggregateField{
			{Name: "EventType", Decl: "WORD", Offset: 0, Size: 2, Length: 1, ElemSize: 2, Default: make([]byte, 2)},
			{Name: "Event", Decl: "INPUT_RECORD_EVENT", TypeName: "INPUT_RECORD_EVENT", Offset: 4, Size: 16, Length: 1, ElemSize: 16, Default: make([]byte, 16)},
		},
		Size:    20,
		Default: make([]byte, 20),
	})
}
