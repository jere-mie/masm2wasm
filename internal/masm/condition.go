package masm

import (
	"fmt"
	"strings"

	"masminterpreter/vm"
)

type condNode interface{}

type condFlag struct {
	Name string
}

type condCompare struct {
	Left  string
	Op    string
	Right string
}

type condAnd struct {
	Parts []condNode
}

type condOr struct {
	Parts []condNode
}

type condNot struct {
	Part condNode
}

type condToken struct {
	kind string
	text string
}

type condParser struct {
	lineNo int
	tokens []condToken
	pos    int
}

func (p *Parser) openIf(lineNo int, cond string) error {
	node, err := parseCondition(lineNo, cond)
	if err != nil {
		return err
	}
	frame := ifFrame{
		nextLabel: p.newSyntheticLabel("if_next"),
		endLabel:  p.newSyntheticLabel("if_end"),
	}
	p.ifStack = append(p.ifStack, frame)
	return p.compileCondFalse(lineNo, node, frame.nextLabel)
}

func (p *Parser) elseIf(lineNo int, cond string) error {
	if len(p.ifStack) == 0 {
		return fmt.Errorf("line %d: .ELSEIF without matching .IF", lineNo)
	}
	frame := &p.ifStack[len(p.ifStack)-1]
	p.addInst(lineNo, ".ELSEIF", "jmp", vm.Operand{Kind: "name", Text: frame.endLabel})
	p.addLabel(frame.nextLabel)
	next := p.newSyntheticLabel("if_next")
	frame.nextLabel = next
	node, err := parseCondition(lineNo, cond)
	if err != nil {
		return err
	}
	return p.compileCondFalse(lineNo, node, next)
}

func (p *Parser) elseBlock(lineNo int) error {
	if len(p.ifStack) == 0 {
		return fmt.Errorf("line %d: .ELSE without matching .IF", lineNo)
	}
	frame := &p.ifStack[len(p.ifStack)-1]
	p.addInst(lineNo, ".ELSE", "jmp", vm.Operand{Kind: "name", Text: frame.endLabel})
	if frame.nextLabel != "" {
		p.addLabel(frame.nextLabel)
		frame.nextLabel = ""
	}
	return nil
}

func (p *Parser) endIf(lineNo int) error {
	if len(p.ifStack) == 0 {
		return fmt.Errorf("line %d: .ENDIF without matching .IF", lineNo)
	}
	frame := p.ifStack[len(p.ifStack)-1]
	p.ifStack = p.ifStack[:len(p.ifStack)-1]
	if frame.nextLabel != "" {
		p.addLabel(frame.nextLabel)
	}
	p.addLabel(frame.endLabel)
	return nil
}

func (p *Parser) openWhile(lineNo int, cond string) error {
	if strings.TrimSpace(cond) == "" {
		return fmt.Errorf("line %d: .WHILE requires a condition", lineNo)
	}
	frame := whileFrame{
		startLabel: p.newSyntheticLabel("while_start"),
		endLabel:   p.newSyntheticLabel("while_end"),
	}
	p.whileStack = append(p.whileStack, frame)
	p.addLabel(frame.startLabel)
	node, err := parseCondition(lineNo, cond)
	if err != nil {
		return err
	}
	return p.compileCondFalse(lineNo, node, frame.endLabel)
}

func (p *Parser) endWhile(lineNo int) error {
	if len(p.whileStack) == 0 {
		return fmt.Errorf("line %d: .ENDW without matching .WHILE", lineNo)
	}
	frame := p.whileStack[len(p.whileStack)-1]
	p.whileStack = p.whileStack[:len(p.whileStack)-1]
	p.addInst(lineNo, ".ENDW", "jmp", vm.Operand{Kind: "name", Text: frame.startLabel})
	p.addLabel(frame.endLabel)
	return nil
}

func (p *Parser) openRepeat(_ int) error {
	frame := repeatFrame{startLabel: p.newSyntheticLabel("repeat_start")}
	p.repeatStack = append(p.repeatStack, frame)
	p.addLabel(frame.startLabel)
	return nil
}

func (p *Parser) untilRepeat(lineNo int, cond string) error {
	if len(p.repeatStack) == 0 {
		return fmt.Errorf("line %d: .UNTIL without matching .REPEAT", lineNo)
	}
	if strings.TrimSpace(cond) == "" {
		return fmt.Errorf("line %d: .UNTIL requires a condition", lineNo)
	}
	frame := p.repeatStack[len(p.repeatStack)-1]
	p.repeatStack = p.repeatStack[:len(p.repeatStack)-1]
	node, err := parseCondition(lineNo, cond)
	if err != nil {
		return err
	}
	return p.compileCondFalse(lineNo, node, frame.startLabel)
}

func (p *Parser) compileCondFalse(lineNo int, node condNode, falseLabel string) error {
	switch n := node.(type) {
	case condFlag:
		p.addInst(lineNo, ".IF", falseJumpForFlag(n.Name), vm.Operand{Kind: "name", Text: falseLabel})
		return nil
	case condCompare:
		left, err := p.parseOperand(lineNo, "cmp", n.Left)
		if err != nil {
			return err
		}
		right, err := p.parseOperand(lineNo, "cmp", n.Right)
		if err != nil {
			return err
		}
		p.addInst(lineNo, ".IF", "cmp", left, right)
		p.addInst(lineNo, ".IF", falseJumpForCompare(n.Op), vm.Operand{Kind: "name", Text: falseLabel})
		return nil
	case condAnd:
		for _, part := range n.Parts {
			if err := p.compileCondFalse(lineNo, part, falseLabel); err != nil {
				return err
			}
		}
		return nil
	case condOr:
		skip := p.newSyntheticLabel("if_or")
		for i, part := range n.Parts {
			if i == len(n.Parts)-1 {
				if err := p.compileCondFalse(lineNo, part, falseLabel); err != nil {
					return err
				}
				break
			}
			if err := p.compileCondTrue(lineNo, part, skip); err != nil {
				return err
			}
		}
		p.addLabel(skip)
		return nil
	case condNot:
		return p.compileCondTrue(lineNo, n.Part, falseLabel)
	default:
		return fmt.Errorf("line %d: unsupported .IF condition", lineNo)
	}
}

func (p *Parser) compileCondTrue(lineNo int, node condNode, trueLabel string) error {
	switch n := node.(type) {
	case condFlag:
		p.addInst(lineNo, ".IF", trueJumpForFlag(n.Name), vm.Operand{Kind: "name", Text: trueLabel})
		return nil
	case condCompare:
		left, err := p.parseOperand(lineNo, "cmp", n.Left)
		if err != nil {
			return err
		}
		right, err := p.parseOperand(lineNo, "cmp", n.Right)
		if err != nil {
			return err
		}
		p.addInst(lineNo, ".IF", "cmp", left, right)
		p.addInst(lineNo, ".IF", trueJumpForCompare(n.Op), vm.Operand{Kind: "name", Text: trueLabel})
		return nil
	case condAnd:
		skip := p.newSyntheticLabel("if_and")
		for i, part := range n.Parts {
			if i == len(n.Parts)-1 {
				if err := p.compileCondTrue(lineNo, part, trueLabel); err != nil {
					return err
				}
				break
			}
			if err := p.compileCondFalse(lineNo, part, skip); err != nil {
				return err
			}
		}
		p.addLabel(skip)
		return nil
	case condOr:
		for _, part := range n.Parts {
			if err := p.compileCondTrue(lineNo, part, trueLabel); err != nil {
				return err
			}
		}
		return nil
	case condNot:
		return p.compileCondFalse(lineNo, n.Part, trueLabel)
	default:
		return fmt.Errorf("line %d: unsupported .IF condition", lineNo)
	}
}

func parseCondition(lineNo int, text string) (condNode, error) {
	tokens, err := tokenizeCondition(text)
	if err != nil {
		return nil, fmt.Errorf("line %d: %w", lineNo, err)
	}
	parser := condParser{lineNo: lineNo, tokens: tokens}
	node, err := parser.parseOr()
	if err != nil {
		return nil, err
	}
	if parser.pos != len(tokens) {
		return nil, fmt.Errorf("line %d: unexpected token %q in .IF", lineNo, tokens[parser.pos].text)
	}
	return node, nil
}

func (p *condParser) parseOr() (condNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	parts := []condNode{left}
	for p.has("||") {
		p.consume()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		parts = append(parts, right)
	}
	if len(parts) == 1 {
		return left, nil
	}
	return condOr{Parts: parts}, nil
}

func (p *condParser) parseAnd() (condNode, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	parts := []condNode{left}
	for p.has("&&") {
		p.consume()
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		parts = append(parts, right)
	}
	if len(parts) == 1 {
		return left, nil
	}
	return condAnd{Parts: parts}, nil
}

func (p *condParser) parsePrimary() (condNode, error) {
	if p.has("!") {
		p.consume()
		part, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return condNot{Part: part}, nil
	}
	if p.has("(") {
		p.consume()
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if !p.has(")") {
			return nil, fmt.Errorf("line %d: missing closing parenthesis in .IF", p.lineNo)
		}
		p.consume()
		return node, nil
	}
	if p.hasKind("flag") {
		return condFlag{Name: strings.ToLower(p.consume().text)}, nil
	}
	return p.parseCompare()
}

func (p *condParser) parseCompare() (condNode, error) {
	left := p.collectUntilComparison()
	if left == "" {
		return nil, fmt.Errorf("line %d: missing left side of .IF comparison", p.lineNo)
	}
	if !p.hasKind("cmp") {
		return nil, fmt.Errorf("line %d: expected comparison operator in .IF", p.lineNo)
	}
	op := p.consume().text
	right := p.collectValue()
	if right == "" {
		return nil, fmt.Errorf("line %d: missing right side of .IF comparison", p.lineNo)
	}
	return condCompare{Left: left, Op: op, Right: right}, nil
}

func (p *condParser) collectUntilComparison() string {
	start := p.pos
	for p.pos < len(p.tokens) && p.tokens[p.pos].kind != "cmp" {
		if p.tokens[p.pos].text == "(" || p.tokens[p.pos].text == ")" || p.tokens[p.pos].text == "&&" || p.tokens[p.pos].text == "||" {
			break
		}
		p.pos++
	}
	return joinCondTokens(p.tokens[start:p.pos])
}

func (p *condParser) collectValue() string {
	start := p.pos
	for p.pos < len(p.tokens) {
		token := p.tokens[p.pos].text
		if token == ")" || token == "&&" || token == "||" {
			break
		}
		p.pos++
	}
	return joinCondTokens(p.tokens[start:p.pos])
}

func (p *condParser) has(text string) bool {
	return p.pos < len(p.tokens) && p.tokens[p.pos].text == text
}

func (p *condParser) hasKind(kind string) bool {
	return p.pos < len(p.tokens) && p.tokens[p.pos].kind == kind
}

func (p *condParser) consume() condToken {
	token := p.tokens[p.pos]
	p.pos++
	return token
}

func tokenizeCondition(text string) ([]condToken, error) {
	var tokens []condToken
	for i := 0; i < len(text); {
		switch {
		case text[i] == ' ' || text[i] == '\t':
			i++
		case strings.HasPrefix(text[i:], "&&") || strings.HasPrefix(text[i:], "||") || strings.HasPrefix(text[i:], "==") || strings.HasPrefix(text[i:], "!=") || strings.HasPrefix(text[i:], "<=") || strings.HasPrefix(text[i:], ">="):
			op := text[i : i+2]
			kind := "op"
			if op != "&&" && op != "||" {
				kind = "cmp"
			}
			tokens = append(tokens, condToken{kind: kind, text: op})
			i += 2
		case text[i] == '<' || text[i] == '>':
			tokens = append(tokens, condToken{kind: "cmp", text: text[i : i+1]})
			i++
		case text[i] == '(' || text[i] == ')':
			tokens = append(tokens, condToken{kind: "paren", text: text[i : i+1]})
			i++
		case text[i] == '!':
			tokens = append(tokens, condToken{kind: "op", text: "!"})
			i++
		case text[i] == '\'':
			j := i + 1
			for j < len(text) && text[j] != '\'' {
				if text[j] == '\\' && j+1 < len(text) {
					j += 2
					continue
				}
				j++
			}
			if j >= len(text) {
				return nil, fmt.Errorf("unterminated character literal in .IF")
			}
			tokens = append(tokens, condToken{kind: "value", text: text[i : j+1]})
			i = j + 1
		default:
			j := i + 1
			for j < len(text) && !strings.ContainsRune("()&|=!<> \t", rune(text[j])) {
				j++
			}
			token := strings.TrimSpace(text[i:j])
			kind := "value"
			switch {
			case strings.EqualFold(token, "ZERO?") || strings.EqualFold(token, "CARRY?"):
				kind = "flag"
			case strings.EqualFold(token, "AND"):
				kind = "op"
				token = "&&"
			case strings.EqualFold(token, "OR"):
				kind = "op"
				token = "||"
			case strings.EqualFold(token, "NOT"):
				kind = "op"
				token = "!"
			case strings.EqualFold(token, "EQ"):
				kind = "cmp"
				token = "=="
			case strings.EqualFold(token, "NE"):
				kind = "cmp"
				token = "!="
			case strings.EqualFold(token, "LT"):
				kind = "cmp"
				token = "<"
			case strings.EqualFold(token, "LE"):
				kind = "cmp"
				token = "<="
			case strings.EqualFold(token, "GT"):
				kind = "cmp"
				token = ">"
			case strings.EqualFold(token, "GE"):
				kind = "cmp"
				token = ">="
			}
			tokens = append(tokens, condToken{kind: kind, text: token})
			i = j
		}
	}
	return tokens, nil
}

func joinCondTokens(tokens []condToken) string {
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		parts = append(parts, token.text)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func falseJumpForFlag(flag string) string {
	switch strings.ToLower(flag) {
	case "zero?":
		return "jnz"
	case "carry?":
		return "jnc"
	default:
		return "jnz"
	}
}

func trueJumpForFlag(flag string) string {
	switch strings.ToLower(flag) {
	case "zero?":
		return "jz"
	case "carry?":
		return "jc"
	default:
		return "jz"
	}
}

func falseJumpForCompare(op string) string {
	switch op {
	case "==":
		return "jne"
	case "!=":
		return "je"
	case "<":
		return "jge"
	case "<=":
		return "jg"
	case ">":
		return "jle"
	case ">=":
		return "jl"
	default:
		return "jne"
	}
}

func trueJumpForCompare(op string) string {
	switch op {
	case "==":
		return "je"
	case "!=":
		return "jne"
	case "<":
		return "jl"
	case "<=":
		return "jle"
	case ">":
		return "jg"
	case ">=":
		return "jge"
	default:
		return "je"
	}
}
