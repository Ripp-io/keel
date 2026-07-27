package merge

import (
	"fmt"
	"strings"
)

// EvalCondition evaluates condition expressions against a variables map.
//
// Supported syntax:
//   - Equality:  keel.agent == 'claude'
//   - Inequality: keel.agent != 'cursor'
//   - In list:   keel.agent in ['claude', 'cursor']
//   - Not in:    keel.agent not in ['gemini']
//   - OR:        keel.agent == 'claude' || keel.agent == 'cursor'
//   - AND:       keel.scope == 'project' && keel.agent == 'claude'
//   - Combined:  a == 'x' || b == 'y' && c == 'z'  (AND binds tighter than OR)
//
// The left side is a dotted variable path. For == and !=, the right side is a
// quoted string (single or double quotes). For in/not in, the right side is a
// bracket-delimited list of quoted strings. Parentheses are not supported.
func evalCondition(expr string, vars map[string]string) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false, fmt.Errorf("empty condition expression")
	}

	// OR has lowest precedence: any clause true → whole expression true.
	orClauses := splitLogical(expr, "||")
	for _, orClause := range orClauses {
		orClause = strings.TrimSpace(orClause)
		if orClause == "" {
			continue
		}

		// AND has higher precedence: all parts must be true.
		andParts := splitLogical(orClause, "&&")
		allTrue := true
		for _, part := range andParts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			result, err := evalSimple(part, vars)
			if err != nil {
				return false, err
			}
			if !result {
				allTrue = false
				break
			}
		}
		if allTrue {
			return true, nil
		}
	}

	return false, nil
}

// splitLogical splits an expression on a logical operator (|| or &&) while
// respecting quoted strings. Operators inside quotes are not split points.
func splitLogical(expr, op string) []string {
	// Fast path: no operator present at all.
	if !strings.Contains(expr, op) {
		return []string{expr}
	}

	var parts []string
	var current strings.Builder
	i := 0
	for i < len(expr) {
		// Track quoted strings — skip over their contents.
		if expr[i] == '\'' || expr[i] == '"' {
			quote := expr[i]
			current.WriteByte(expr[i])
			i++
			for i < len(expr) && expr[i] != quote {
				if expr[i] == '\\' && i+1 < len(expr) {
					current.WriteByte(expr[i])
					i++
					if i < len(expr) {
						current.WriteByte(expr[i])
						i++
					}
					continue
				}
				current.WriteByte(expr[i])
				i++
			}
			if i < len(expr) {
				current.WriteByte(expr[i])
				i++
			}
			continue
		}

		// Check for the operator at this position.
		if i+len(op) <= len(expr) && expr[i:i+len(op)] == op {
			parts = append(parts, current.String())
			current.Reset()
			i += len(op)
			continue
		}

		current.WriteByte(expr[i])
		i++
	}
	parts = append(parts, current.String())
	return parts
}

// evalSimple evaluates a single comparison expression:
//   - key == 'value'
//   - key != 'value'
//   - key in ['a', 'b']
//   - key not in ['a', 'b']
func evalSimple(expr string, vars map[string]string) (bool, error) {
	expr = strings.TrimSpace(expr)

	// Try "not in" first (before "in" to avoid partial match).
	if left, list, ok := parseInExpr(expr, " not in "); ok {
		leftVal := vars[left]
		for _, item := range list {
			if leftVal == item {
				return false, nil
			}
		}
		return true, nil
	}

	// Try "in".
	if left, list, ok := parseInExpr(expr, " in "); ok {
		leftVal := vars[left]
		for _, item := range list {
			if leftVal == item {
				return true, nil
			}
		}
		return false, nil
	}

	// Fall back to == / != comparison.
	var op string
	var opIdx int

	if idx := findOperator(expr, "!="); idx >= 0 {
		op = "!="
		opIdx = idx
	} else if idx := findOperator(expr, "=="); idx >= 0 {
		op = "=="
		opIdx = idx
	} else {
		return false, fmt.Errorf("no operator found in condition %q (expected ==, !=, in, or not in)", expr)
	}

	left := strings.TrimSpace(expr[:opIdx])
	right := strings.TrimSpace(expr[opIdx+2:])

	if left == "" {
		return false, fmt.Errorf("empty left side in condition %q", expr)
	}

	// Right side must be a single-quoted or double-quoted string literal.
	if len(right) < 2 {
		return false, fmt.Errorf("invalid right side in condition %q", expr)
	}
	quote := right[0]
	if (quote != '\'' && quote != '"') || right[len(right)-1] != quote {
		return false, fmt.Errorf("right side must be a quoted string in condition %q", expr)
	}
	rightVal := right[1 : len(right)-1]

	leftVal := vars[left]

	switch op {
	case "==":
		return leftVal == rightVal, nil
	case "!=":
		return leftVal != rightVal, nil
	default:
		return false, fmt.Errorf("unsupported operator %q", op)
	}
}

// parseInExpr tries to parse "key in ['a', 'b']" or "key not in ['a', 'b']".
// The separator must be " in " or " not in " (with spaces).
// Returns the variable name, the list of string values, and whether parsing succeeded.
func parseInExpr(expr, sep string) (string, []string, bool) {
	idx := strings.Index(expr, sep)
	if idx < 0 {
		return "", nil, false
	}

	left := strings.TrimSpace(expr[:idx])
	right := strings.TrimSpace(expr[idx+len(sep):])

	if left == "" {
		return "", nil, false
	}

	// Right side must be a bracket-delimited list: ["a", "b"] or ['a', 'b']
	if len(right) < 2 || right[0] != '[' || right[len(right)-1] != ']' {
		return "", nil, false
	}

	inner := strings.TrimSpace(right[1 : len(right)-1])
	if inner == "" {
		return left, nil, true // empty list
	}

	// Parse comma-separated quoted strings.
	var items []string
	for _, part := range splitListItems(inner) {
		part = strings.TrimSpace(part)
		if len(part) < 2 {
			return "", nil, false
		}
		quote := part[0]
		if (quote != '\'' && quote != '"') || part[len(part)-1] != quote {
			return "", nil, false
		}
		items = append(items, part[1:len(part)-1])
	}

	return left, items, true
}

// splitListItems splits a string on commas while respecting quoted strings.
func splitListItems(s string) []string {
	var items []string
	var current strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\'' || s[i] == '"' {
			quote := s[i]
			current.WriteByte(s[i])
			i++
			for i < len(s) && s[i] != quote {
				if s[i] == '\\' && i+1 < len(s) {
					current.WriteByte(s[i])
					i++
					if i < len(s) {
						current.WriteByte(s[i])
						i++
					}
					continue
				}
				current.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				current.WriteByte(s[i])
				i++
			}
			continue
		}
		if s[i] == ',' {
			items = append(items, current.String())
			current.Reset()
			i++
			continue
		}
		current.WriteByte(s[i])
		i++
	}
	items = append(items, current.String())
	return items
}

// findOperator locates a == or != operator outside of quoted strings.
func findOperator(expr, op string) int {
	i := 0
	for i < len(expr) {
		if expr[i] == '\'' || expr[i] == '"' {
			quote := expr[i]
			i++
			for i < len(expr) && expr[i] != quote {
				i++
			}
			if i < len(expr) {
				i++ // skip closing quote
			}
			continue
		}
		if i+len(op) <= len(expr) && expr[i:i+len(op)] == op {
			return i
		}
		i++
	}
	return -1
}
