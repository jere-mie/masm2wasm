package masm

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type exprToken struct {
	kind string
	text string
}

func (p *Parser) evalExpr(lineNo int, expr string) (int64, error) {
	tokens, err := tokenizeExpr(expr)
	if err != nil {
		return 0, fmt.Errorf("line %d: %w", lineNo, err)
	}
	parser := exprParser{parser: p, lineNo: lineNo, tokens: tokens}
	value, err := parser.parseExpression()
	if err != nil {
		return 0, err
	}
	if parser.pos != len(tokens) {
		return 0, fmt.Errorf("line %d: unexpected token %q", lineNo, tokens[parser.pos].text)
	}
	return value, nil
}

type exprParser struct {
	parser *Parser
	lineNo int
	tokens []exprToken
	pos    int
}

func (p *exprParser) parseExpression() (int64, error) {
	value, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for p.has("+") || p.has("-") {
		op := p.consume().text
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == "+" {
			value += right
		} else {
			value -= right
		}
	}
	return value, nil
}

func (p *exprParser) parseTerm() (int64, error) {
	value, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	for p.has("*") || p.has("/") {
		op := p.consume().text
		right, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		if op == "*" {
			value *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("line %d: division by zero", p.lineNo)
			}
			value /= right
		}
	}
	return value, nil
}

func (p *exprParser) parseUnary() (int64, error) {
	if p.has("+") {
		p.consume()
		return p.parseUnary()
	}
	if p.has("-") {
		p.consume()
		value, err := p.parseUnary()
		return -value, err
	}
	if p.hasIdent("offset") || p.hasIdent("addr") || p.hasIdent("lengthof") || p.hasIdent("length") || p.hasIdent("sizeof") || p.hasIdent("size") || p.hasIdent("type") {
		op := strings.ToLower(p.consume().text)
		next := p.consume()
		if next.kind != "ident" {
			return 0, fmt.Errorf("line %d: %s requires an identifier", p.lineNo, op)
		}
		switch op {
		case "offset", "addr":
			symbol, ok := p.parser.exprSymbols[strings.ToLower(next.text)]
			if !ok {
				return 0, fmt.Errorf("line %d: unknown symbol %q", p.lineNo, next.text)
			}
			return int64(symbol.Address), nil
		case "lengthof", "length":
			symbol, ok := p.parser.exprSymbols[strings.ToLower(next.text)]
			if !ok {
				return 0, fmt.Errorf("line %d: unknown symbol %q", p.lineNo, next.text)
			}
			return int64(symbol.Length), nil
		case "sizeof", "size":
			symbol, ok := p.parser.exprSymbols[strings.ToLower(next.text)]
			if !ok {
				return 0, fmt.Errorf("line %d: unknown symbol %q", p.lineNo, next.text)
			}
			return int64(symbol.Size), nil
		case "type":
			if symbol, ok := p.parser.exprSymbols[strings.ToLower(next.text)]; ok {
				return int64(symbol.ElemSize), nil
			}
			if size := p.parser.typeSize(next.text); size != 0 {
				return int64(size), nil
			}
			return int64(typeSpecValueSize(next.text)), nil
		}
	}
	return p.parsePrimary()
}

func (p *exprParser) parsePrimary() (int64, error) {
	if p.has("(") {
		p.consume()
		value, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		if !p.has(")") {
			return 0, fmt.Errorf("line %d: missing closing parenthesis", p.lineNo)
		}
		p.consume()
		return value, nil
	}
	token := p.consume()
	switch token.kind {
	case "number":
		return parseNumberToken(token.text)
	case "char":
		value, err := parseStringLiteral(token.text)
		if err != nil {
			return 0, fmt.Errorf("line %d: %w", p.lineNo, err)
		}
		if len(value) != 1 {
			return 0, fmt.Errorf("line %d: character literal must contain exactly one character", p.lineNo)
		}
		return int64(value[0]), nil
	case "current":
		return p.parser.currentOffset, nil
	case "ident":
		lower := strings.ToLower(token.text)
		if value, ok := p.parser.constants[lower]; ok {
			return value, nil
		}
		if symbol, ok := p.parser.exprSymbols[lower]; ok {
			return int64(symbol.Address), nil
		}
		return 0, fmt.Errorf("line %d: unknown identifier %q", p.lineNo, token.text)
	default:
		return 0, fmt.Errorf("line %d: unexpected token %q", p.lineNo, token.text)
	}
}

func (p *exprParser) has(text string) bool {
	return p.pos < len(p.tokens) && p.tokens[p.pos].text == text
}

func (p *exprParser) hasIdent(text string) bool {
	return p.pos < len(p.tokens) && p.tokens[p.pos].kind == "ident" && strings.EqualFold(p.tokens[p.pos].text, text)
}

func (p *exprParser) consume() exprToken {
	token := p.tokens[p.pos]
	p.pos++
	return token
}

func tokenizeExpr(expr string) ([]exprToken, error) {
	var tokens []exprToken
	for i := 0; i < len(expr); {
		switch ch := expr[i]; {
		case unicode.IsSpace(rune(ch)):
			i++
		case ch == '(' || ch == ')' || ch == '+' || ch == '-' || ch == '*' || ch == '/':
			tokens = append(tokens, exprToken{kind: "op", text: string(ch)})
			i++
		case ch == '$':
			tokens = append(tokens, exprToken{kind: "current", text: "$"})
			i++
		case ch == '\'' && i+2 < len(expr):
			j := i + 1
			for j < len(expr) && expr[j] != '\'' {
				if expr[j] == '\\' && j+1 < len(expr) {
					j += 2
					continue
				}
				j++
			}
			if j >= len(expr) {
				return nil, fmt.Errorf("unterminated character literal")
			}
			tokens = append(tokens, exprToken{kind: "char", text: expr[i : j+1]})
			i = j + 1
		case isIdentStart(ch):
			j := i + 1
			for j < len(expr) && isIdentPart(expr[j]) {
				j++
			}
			tokens = append(tokens, exprToken{kind: "ident", text: expr[i:j]})
			i = j
		case isNumericStart(ch):
			j := i + 1
			for j < len(expr) && isNumericPart(expr[j]) {
				j++
			}
			tokens = append(tokens, exprToken{kind: "number", text: expr[i:j]})
			i = j
		default:
			return nil, fmt.Errorf("unexpected character %q in expression", ch)
		}
	}
	return tokens, nil
}

func parseNumberToken(token string) (int64, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, fmt.Errorf("empty numeric token")
	}
	lower := strings.ToLower(token)
	switch {
	case strings.HasSuffix(lower, "h"):
		value, err := strconv.ParseInt(lower[:len(lower)-1], 16, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid hex literal %q", token)
		}
		return value, nil
	case strings.HasSuffix(lower, "b"):
		value, err := strconv.ParseInt(lower[:len(lower)-1], 2, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid binary literal %q", token)
		}
		return value, nil
	default:
		value, err := strconv.ParseInt(token, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid numeric literal %q", token)
		}
		return value, nil
	}
}

func isIdentStart(ch byte) bool {
	return ch == '_' || ch == '@' || ch == '$' || ch == '?' || ch == '.' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z'
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || ch >= '0' && ch <= '9'
}

func isNumericStart(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isNumericPart(ch byte) bool {
	return ch >= '0' && ch <= '9' || ch >= 'A' && ch <= 'F' || ch >= 'a' && ch <= 'f' || ch == 'h' || ch == 'H' || ch == 'b' || ch == 'B'
}
