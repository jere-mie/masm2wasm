package masm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"masminterpreter/vm"
)

type compileTimeState struct {
	ints  map[string]int64
	texts map[string]string
}

var reIsDefinedCall = regexp.MustCompile(`(?i)\bIsDefined\s*\(\s*([A-Za-z_@?$\.][A-Za-z0-9_@?$\.]*)\s*\)`)

func expandCompileTime(lines []sourceLine) ([]sourceLine, error) {
	state := &compileTimeState{
		ints:  map[string]int64{},
		texts: map[string]string{},
	}
	for k, v := range builtinConstants {
		state.ints[k] = v
	}
	return expandCompileTimeBlock(lines, state, true, nil)
}

func expandCompileTimeBlock(lines []sourceLine, state *compileTimeState, preserveAssignments bool, replacements map[string]string) ([]sourceLine, error) {
	var out []sourceLine
	for i := 0; i < len(lines); i++ {
		src := lines[i]
		controlLine := applyCompileTimeText(src.Number, src.Text, state, replacements, preserveAssignments, false)
		if strings.HasPrefix(strings.TrimSpace(controlLine), "%") {
			controlLine = strings.TrimSpace(strings.TrimSpace(controlLine)[1:])
		}
		trimmed := strings.TrimSpace(controlLine)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)

		switch {
		case strings.HasPrefix(lower, "echo "):
			continue
		case strings.EqualFold(trimmed, "EXITM"):
			continue
		case strings.HasPrefix(lower, "local "):
			if !preserveAssignments {
				continue
			}
		case strings.EqualFold(opcodeWord(trimmed), "ifb"):
			trueBody, falseBody, next, err := collectConditionalBlock(lines, i+1)
			if err != nil {
				return nil, err
			}
			_, rest := splitOpcode(trimmed)
			chosen := falseBody
			if evalIFB(rest) {
				chosen = trueBody
			}
			expanded, err := expandCompileTimeBlock(chosen, state, preserveAssignments, replacements)
			if err != nil {
				return nil, err
			}
			out = append(out, expanded...)
			i = next
			continue
		case strings.EqualFold(opcodeWord(trimmed), "ifidni"):
			trueBody, falseBody, next, err := collectConditionalBlock(lines, i+1)
			if err != nil {
				return nil, err
			}
			_, rest := splitOpcode(trimmed)
			chosen := falseBody
			if evalIFIDNI(rest) {
				chosen = trueBody
			}
			expanded, err := expandCompileTimeBlock(chosen, state, preserveAssignments, replacements)
			if err != nil {
				return nil, err
			}
			out = append(out, expanded...)
			i = next
			continue
		case strings.EqualFold(opcodeWord(trimmed), "ifdef"):
			trueBody, falseBody, next, err := collectConditionalBlock(lines, i+1)
			if err != nil {
				return nil, err
			}
			_, rest := splitOpcode(trimmed)
			chosen := falseBody
			if isCompileTimeDefined(rest, state) {
				chosen = trueBody
			}
			expanded, err := expandCompileTimeBlock(chosen, state, preserveAssignments, replacements)
			if err != nil {
				return nil, err
			}
			out = append(out, expanded...)
			i = next
			continue
		case strings.EqualFold(opcodeWord(trimmed), "ifndef"):
			trueBody, falseBody, next, err := collectConditionalBlock(lines, i+1)
			if err != nil {
				return nil, err
			}
			_, rest := splitOpcode(trimmed)
			chosen := falseBody
			if !isCompileTimeDefined(rest, state) {
				chosen = trueBody
			}
			expanded, err := expandCompileTimeBlock(chosen, state, preserveAssignments, replacements)
			if err != nil {
				return nil, err
			}
			out = append(out, expanded...)
			i = next
			continue
		case strings.EqualFold(opcodeWord(trimmed), "if"):
			trueBody, falseBody, next, err := collectConditionalBlock(lines, i+1)
			if err != nil {
				return nil, err
			}
			_, rest := splitOpcode(trimmed)
			cond, err := evalCompileTimeCondition(src.Number, rest, state)
			if err != nil {
				if preserveAssignments {
					out = append(out, sourceLine{Number: src.Number, Text: controlLine})
					i = next
					continue
				}
				return nil, err
			}
			chosen := falseBody
			if cond {
				chosen = trueBody
			}
			expanded, err := expandCompileTimeBlock(chosen, state, preserveAssignments, replacements)
			if err != nil {
				return nil, err
			}
			out = append(out, expanded...)
			i = next
			continue
		case strings.HasPrefix(lower, "rept ") || strings.HasPrefix(lower, "repeat "):
			body, next, err := collectCompileTimeBlock(lines, i+1)
			if err != nil {
				return nil, err
			}
			_, countExpr := splitOpcode(trimmed)
			count, err := evalCompileTimeExpr(src.Number, countExpr, state)
			if err != nil {
				return nil, err
			}
			if count < 0 {
				return nil, fmt.Errorf("line %d: repeat count cannot be negative", src.Number)
			}
			for iter := int64(0); iter < count; iter++ {
				expanded, err := expandCompileTimeBlock(body, state, false, replacements)
				if err != nil {
					return nil, err
				}
				out = append(out, expanded...)
			}
			i = next
			continue
		case strings.HasPrefix(lower, "while "):
			body, next, err := collectCompileTimeBlock(lines, i+1)
			if err != nil {
				return nil, err
			}
			cond := strings.TrimSpace(trimmed[len("while "):])
			guard := 0
			for {
				ok, err := evalCompileTimeCondition(src.Number, cond, state)
				if err != nil {
					return nil, err
				}
				if !ok {
					break
				}
				expanded, err := expandCompileTimeBlock(body, state, false, replacements)
				if err != nil {
					return nil, err
				}
				out = append(out, expanded...)
				guard++
				if guard > 100000 {
					return nil, fmt.Errorf("line %d: WHILE expansion limit exceeded", src.Number)
				}
			}
			i = next
			continue
		case strings.HasPrefix(lower, "forc "):
			body, next, err := collectCompileTimeBlock(lines, i+1)
			if err != nil {
				return nil, err
			}
			name, payload, err := parseForHeader(trimmed[len("forc "):])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", src.Number, err)
			}
			for _, ch := range parseForCText(payload) {
				child := mergeCompileTimeReplacements(replacements, map[string]string{strings.ToLower(name): string(ch)})
				expanded, err := expandCompileTimeBlock(body, state, false, child)
				if err != nil {
					return nil, err
				}
				out = append(out, expanded...)
			}
			i = next
			continue
		case strings.HasPrefix(lower, "for "):
			body, next, err := collectCompileTimeBlock(lines, i+1)
			if err != nil {
				return nil, err
			}
			name, payload, err := parseForHeader(trimmed[len("for "):])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", src.Number, err)
			}
			for _, item := range splitTopLevel(payload, ',') {
				item = strings.TrimSpace(item)
				if item == "" {
					continue
				}
				child := mergeCompileTimeReplacements(replacements, map[string]string{strings.ToLower(name): item})
				expanded, err := expandCompileTimeBlock(body, state, false, child)
				if err != nil {
					return nil, err
				}
				out = append(out, expanded...)
			}
			i = next
			continue
		}

		if match := reTextEqu.FindStringSubmatch(trimmed); match != nil {
			value := strings.TrimSpace(match[2])
			state.texts[strings.ToLower(match[1])] = value
			if preserveAssignments {
				out = append(out, sourceLine{Number: src.Number, Text: controlLine})
			}
			continue
		}
		if match := reConstEq.FindStringSubmatch(trimmed); match != nil {
			value, err := evalCompileTimeExpr(src.Number, match[2], state)
			if err != nil {
				if preserveAssignments {
					out = append(out, sourceLine{Number: src.Number, Text: controlLine})
					continue
				}
				return nil, err
			}
			state.ints[strings.ToLower(match[1])] = value
			if preserveAssignments {
				out = append(out, sourceLine{Number: src.Number, Text: fmt.Sprintf("%s = %s", match[1], strings.TrimSpace(match[2]))})
			}
			continue
		}
		if match := reConstEqu.FindStringSubmatch(trimmed); match != nil {
			value, err := evalCompileTimeExpr(src.Number, match[2], state)
			if err == nil {
				state.ints[strings.ToLower(match[1])] = value
			} else {
				state.texts[strings.ToLower(match[1])] = strings.TrimSpace(match[2])
			}
			if preserveAssignments {
				out = append(out, sourceLine{Number: src.Number, Text: controlLine})
			}
			continue
		}
		if strings.EqualFold(trimmed, "ENDM") {
			return nil, fmt.Errorf("line %d: unexpected ENDM", src.Number)
		}
		if strings.EqualFold(trimmed, "ELSE") || strings.EqualFold(trimmed, "ENDIF") {
			return nil, fmt.Errorf("line %d: unexpected %s", src.Number, trimmed)
		}
		line := controlLine
		if !preserveAssignments {
			line = applyCompileTimeText(src.Number, line, state, nil, false, true)
		}
		out = append(out, sourceLine{Number: src.Number, Text: line})
	}
	return out, nil
}

func collectCompileTimeBlock(lines []sourceLine, start int) ([]sourceLine, int, error) {
	depth := 0
	var body []sourceLine
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i].Text)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "rept "), strings.HasPrefix(lower, "repeat "), strings.HasPrefix(lower, "while "), strings.HasPrefix(lower, "for "), strings.HasPrefix(lower, "forc "):
			depth++
		case strings.EqualFold(trimmed, "ENDM"):
			if depth == 0 {
				return body, i, nil
			}
			depth--
		}
		body = append(body, lines[i])
	}
	return nil, 0, fmt.Errorf("unterminated compile-time block")
}

func collectConditionalBlock(lines []sourceLine, start int) (trueBody []sourceLine, falseBody []sourceLine, end int, err error) {
	depth := 0
	current := &trueBody
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i].Text)
		word := strings.ToLower(opcodeWord(trimmed))
		switch word {
		case "if", "ifb", "ifidni", "ifdef", "ifndef":
			depth++
		case "else":
			if depth == 0 {
				current = &falseBody
				continue
			}
		case "endif":
			if depth == 0 {
				return trueBody, falseBody, i, nil
			}
			depth--
		}
		*current = append(*current, lines[i])
	}
	return nil, nil, 0, fmt.Errorf("unterminated IF/ENDIF block")
}

func parseForHeader(spec string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(spec), ",", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed FOR/FORC header")
	}
	name := strings.TrimSpace(parts[0])
	payload := strings.TrimSpace(parts[1])
	if !isIdentifier(name) {
		return "", "", fmt.Errorf("invalid FOR/FORC iterator %q", name)
	}
	if len(payload) < 2 || payload[0] != '<' || payload[len(payload)-1] != '>' {
		return "", "", fmt.Errorf("FOR/FORC payload must use angle brackets")
	}
	return name, payload[1 : len(payload)-1], nil
}

func parseForCText(payload string) []rune {
	var out []rune
	escape := false
	for _, ch := range payload {
		if escape {
			out = append(out, ch)
			escape = false
			continue
		}
		if ch == '!' {
			escape = true
			continue
		}
		out = append(out, ch)
	}
	return out
}

func evalIFB(rest string) bool {
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "<") && strings.HasSuffix(rest, ">") {
		rest = rest[1 : len(rest)-1]
	}
	return strings.TrimSpace(rest) == ""
}

func evalIFIDNI(rest string) bool {
	parts := splitTopLevel(rest, ',')
	if len(parts) != 2 {
		return false
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if strings.HasPrefix(left, "<") && strings.HasSuffix(left, ">") {
		left = left[1 : len(left)-1]
	}
	if strings.HasPrefix(right, "<") && strings.HasSuffix(right, ">") {
		right = right[1 : len(right)-1]
	}
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func mergeCompileTimeReplacements(base map[string]string, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func applyCompileTimeText(lineNo int, text string, state *compileTimeState, replacements map[string]string, preserveAssignments bool, includeNumeric bool) string {
	merged := replacements
	if !preserveAssignments {
		merged = mergeCompileTimeReplacements(state.texts, replacements)
		if includeNumeric {
			merged = mergeCompileTimeReplacements(merged, compileTimeNumericReplacements(state))
		}
	}
	if len(merged) == 0 {
		return expandPercentText(lineNo, text, state)
	}
	return expandPercentText(lineNo, substituteMacroText(text, merged, nil), state)
}

func compileTimeNumericReplacements(state *compileTimeState) map[string]string {
	if len(state.ints) == 0 {
		return nil
	}
	out := make(map[string]string, len(state.ints))
	for k, v := range state.ints {
		out[k] = strconv.FormatInt(v, 10)
	}
	return out
}

func expandPercentText(lineNo int, text string, state *compileTimeState) string {
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
		case ch == '%':
			expanded, consumed := expandPercentToken(lineNo, text[i+1:], state)
			if consumed == 0 {
				i++
				continue
			}
			out.WriteString(expanded)
			i += consumed + 1
		default:
			out.WriteByte(text[i])
			i++
		}
	}
	return out.String()
}

func expandPercentToken(lineNo int, text string, state *compileTimeState) (string, int) {
	if text == "" {
		return "", 0
	}
	if text[0] == '(' {
		depth := 1
		for i := 1; i < len(text); i++ {
			switch text[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					expr := strings.TrimSpace(text[1:i])
					if strings.EqualFold(expr, "@LINE") {
						return strconv.Itoa(lineNo), i + 1
					}
					value, err := evalCompileTimeExpr(lineNo, expr, state)
					if err != nil {
						return expr, i + 1
					}
					return strconv.FormatInt(value, 10), i + 1
				}
			}
		}
		return "", 0
	}
	j := 0
	for j < len(text) && !strings.ContainsRune(" \t,+-*/()<>=", rune(text[j])) {
		j++
	}
	token := text[:j]
	if token == "" {
		return "", 0
	}
	if strings.EqualFold(token, "@LINE") {
		return strconv.Itoa(lineNo), j
	}
	if value, err := evalCompileTimeExpr(lineNo, token, state); err == nil {
		return strconv.FormatInt(value, 10), j
	}
	return token, j
}

func evalCompileTimeExpr(lineNo int, expr string, state *compileTimeState) (int64, error) {
	parser := &Parser{
		constants:   map[string]int64{},
		exprSymbols: map[string]vm.Symbol{},
	}
	for k, v := range state.ints {
		parser.constants[k] = v
	}
	return parser.evalExpr(lineNo, strings.TrimSpace(expr))
}

func evalCompileTimeCondition(lineNo int, text string, state *compileTimeState) (bool, error) {
	normalized := normalizeConditionText(rewriteIsDefinedCalls(text, state))
	node, err := parseCondition(lineNo, normalized)
	if err != nil {
		return false, err
	}
	return evalCompileTimeCondNode(lineNo, node, state)
}

func rewriteIsDefinedCalls(text string, state *compileTimeState) string {
	return reIsDefinedCall.ReplaceAllStringFunc(text, func(match string) string {
		sub := reIsDefinedCall.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		if isCompileTimeDefined(sub[1], state) {
			return "1"
		}
		return "0"
	})
}

func isCompileTimeDefined(name string, state *compileTimeState) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return false
	}
	if _, ok := state.ints[lower]; ok {
		return true
	}
	if _, ok := state.texts[lower]; ok {
		return true
	}
	return false
}

func normalizeConditionText(text string) string {
	tokens, err := tokenizeCondition(text)
	if err != nil {
		return text
	}
	var parts []string
	for _, token := range tokens {
		parts = append(parts, token.text)
	}
	return strings.Join(parts, " ")
}

func evalCompileTimeCondNode(lineNo int, node condNode, state *compileTimeState) (bool, error) {
	switch n := node.(type) {
	case condNot:
		value, err := evalCompileTimeCondNode(lineNo, n.Part, state)
		return !value, err
	case condAnd:
		for _, part := range n.Parts {
			value, err := evalCompileTimeCondNode(lineNo, part, state)
			if err != nil {
				return false, err
			}
			if !value {
				return false, nil
			}
		}
		return true, nil
	case condOr:
		for _, part := range n.Parts {
			value, err := evalCompileTimeCondNode(lineNo, part, state)
			if err != nil {
				return false, err
			}
			if value {
				return true, nil
			}
		}
		return false, nil
	case condCompare:
		left, err := evalCompileTimeExpr(lineNo, n.Left, state)
		if err != nil {
			return false, err
		}
		right, err := evalCompileTimeExpr(lineNo, n.Right, state)
		if err != nil {
			return false, err
		}
		switch n.Op {
		case "==":
			return left == right, nil
		case "!=":
			return left != right, nil
		case "<":
			return left < right, nil
		case "<=":
			return left <= right, nil
		case ">":
			return left > right, nil
		case ">=":
			return left >= right, nil
		default:
			return false, fmt.Errorf("line %d: unsupported compile-time comparison %q", lineNo, n.Op)
		}
	case condExpr:
		value, err := evalCompileTimeExpr(lineNo, n.Expr, state)
		if err != nil {
			return false, err
		}
		return value != 0, nil
	default:
		return false, fmt.Errorf("line %d: unsupported compile-time condition", lineNo)
	}
}
