package merge

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
)

const maxConditionalDepth = 100

// conditionalFrame tracks the state of a single conditional block on the
// nesting stack. Each <!-- keel-if: ... --> pushes a frame; the matching
// <!-- keel-endif --> pops it.
type conditionalFrame struct {
	matched  bool // a branch in this block has already matched
	emitting bool // the current branch is actively emitting lines
}

// ProcessConditionals evaluates <!-- keel-if: EXPR --> / <!-- keel-elif: EXPR -->
// / <!-- keel-else --> / <!-- keel-endif --> conditional blocks in markdown content.
// Lines inside non-matching branches are dropped. All marker lines are stripped
// from the output regardless of match. Nesting is supported via a stack.
// Markers inside fenced code blocks (``` ... ```) are treated as literal text.
func ProcessConditionals(content []byte, vars map[string]string) []byte {
	if len(content) == 0 {
		return content
	}
	if len(vars) == 0 {
		if bytes.Contains(content, []byte("<!-- keel-if:")) ||
			bytes.Contains(content, []byte("<!-- keel-elif:")) {
			slog.Warn("conditional markers found but no variables provided; markers will be preserved verbatim")
		}
		return content
	}

	// Normalize \r\n to \n for consistent processing
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	lines := bytes.Split(normalized, []byte("\n"))

	var out bytes.Buffer
	var stack []conditionalFrame
	inCodeBlock := false
	firstLine := true

	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)

		// Track fenced code blocks — toggle on ``` at start of line
		if bytes.HasPrefix(trimmed, []byte("```")) {
			inCodeBlock = !inCodeBlock
		}

		// Skip marker detection inside code blocks
		if inCodeBlock {
			if isEmitting(stack) {
				if !firstLine {
					out.WriteByte('\n')
				}
				out.Write(line)
				firstLine = false
			}
			continue
		}

		// Check for conditional markers
		if marker, expr := parseConditionalMarker(trimmed); marker != "" {
			switch marker {
			case "if":
				if len(stack) > maxConditionalDepth {
					// Bail out — likely malicious or malformed input
					slog.Warn("conditional nesting exceeds maximum depth, truncating", "max", maxConditionalDepth)
					break
				}
				match, err := evalCondition(expr, vars)
				if err != nil {
					match = false
				}
				stack = append(stack, conditionalFrame{
					matched:  match,
					emitting: match,
				})

			case "elif":
				if len(stack) > 0 {
					frame := &stack[len(stack)-1]
					if frame.matched {
						// A previous branch already matched
						frame.emitting = false
					} else {
						match, err := evalCondition(expr, vars)
						if err != nil {
							match = false
						}
						frame.matched = match
						frame.emitting = match
					}
				}

			case "else":
				if len(stack) > 0 {
					frame := &stack[len(stack)-1]
					if frame.matched {
						frame.emitting = false
					} else {
						frame.matched = true
						frame.emitting = true
					}
				}

			case "endif":
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
			// Marker lines are always stripped from output
			continue
		}

		// Non-marker line: emit if we're in a matching branch (or not in any conditional)
		if isEmitting(stack) {
			if !firstLine {
				out.WriteByte('\n')
			}
			out.Write(line)
			firstLine = false
		}
	}

	result := out.Bytes()

	// Preserve trailing newline if original had one
	if len(result) > 0 && len(normalized) > 0 && normalized[len(normalized)-1] == '\n' && result[len(result)-1] != '\n' {
		result = append(result, '\n')
	}

	return result
}

// isEmitting returns true if all frames on the stack are in an emitting state,
// meaning we are inside the matched branch of every nested conditional.
// An empty stack means we're at top level (always emitting).
func isEmitting(stack []conditionalFrame) bool {
	for _, frame := range stack {
		if !frame.emitting {
			return false
		}
	}
	return true
}

// parseConditionalMarker checks if a trimmed line is a keel conditional marker.
// Returns the marker type ("if", "elif", "else", "endif") and the expression
// (for "if" and "elif"), or empty strings if not a marker.
func parseConditionalMarker(trimmed []byte) (marker, expr string) {
	s := string(trimmed)

	// Check for the various marker patterns
	for _, prefix := range []struct {
		tag    string
		marker string
		hasArg bool
	}{
		{"<!-- keel-if:", "if", true},
		{"<!-- keel-elif:", "elif", true},
		{"<!-- keel-else", "else", false},
		{"<!-- keel-endif", "endif", false},
	} {
		if !strings.HasPrefix(s, prefix.tag) {
			continue
		}
		if !strings.HasSuffix(s, "-->") {
			continue
		}
		if prefix.hasArg {
			inner := s[len(prefix.tag) : len(s)-3] // strip "<!-- keel-XX:" and "-->"
			return prefix.marker, strings.TrimSpace(inner)
		}
		return prefix.marker, ""
	}

	return "", ""
}

// FilterConditionJSON filters a JSON map by evaluating _keel.condition
// expressions against the provided variables. Objects whose condition evaluates
// to false are excluded. The _keel key is stripped from all output objects.
// The input map is never modified; a new map is returned.
func FilterConditionJSON(data map[string]any, vars map[string]string) (map[string]any, error) {
	if data == nil {
		return nil, nil
	}
	if len(vars) == 0 {
		return copyMap(data), nil
	}
	return filterConditionJSONRecursive(data, vars, "", 0)
}

func filterConditionJSONRecursive(data map[string]any, vars map[string]string, path string, depth int) (map[string]any, error) {
	if depth > maxStripKeelDepth {
		return nil, fmt.Errorf("condition filtering exceeded maximum nesting depth %d", maxStripKeelDepth)
	}

	result := make(map[string]any)

	for key, value := range data {
		nested, isMap := value.(map[string]any)
		if !isMap {
			result[key] = value
			continue
		}

		currentPath := key
		if path != "" {
			currentPath = path + "." + key
		}

		// Check for _keel within this nested map
		keelRaw, hasKeel := nested["_keel"]
		if !hasKeel {
			// No _keel — recurse into the map for deeper nesting
			filtered, err := filterConditionJSONRecursive(nested, vars, currentPath, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = filtered
			continue
		}

		keelMap, keelIsMap := keelRaw.(map[string]any)
		if !keelIsMap {
			// _keel is not a map — keep entry as-is, recurse rest
			filtered, err := filterConditionJSONRecursive(nested, vars, currentPath, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = filtered
			continue
		}

		// Check for condition expression
		condRaw, hasCondition := keelMap["condition"]
		if hasCondition {
			condStr, ok := condRaw.(string)
			if !ok {
				return nil, fmt.Errorf("key %q has non-string _keel.condition", currentPath)
			}
			match, err := evalCondition(condStr, vars)
			if err != nil {
				return nil, fmt.Errorf("evaluating condition for key %q: %w", currentPath, err)
			}
			if !match {
				// Condition not met — exclude this entry
				continue
			}
		}

		// Object passes — copy without _keel, strip condition from _keel
		cleaned := copyMapExcludingKey(nested, "_keel")

		// Rebuild _keel without condition
		newKeel := make(map[string]any)
		for k, v := range keelMap {
			if k == "condition" {
				continue
			}
			newKeel[k] = v
		}
		if len(newKeel) > 0 {
			cleaned["_keel"] = newKeel
		}

		// Recurse into the cleaned map for deeper nesting
		filtered, err := filterConditionJSONRecursive(cleaned, vars, currentPath, depth+1)
		if err != nil {
			return nil, err
		}
		result[key] = filtered
	}

	return result, nil
}

// copyMap returns a shallow copy of a map[string]any.
func copyMap(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// copyMapExcludingKey returns a shallow copy of a map excluding a specific key.
func copyMapExcludingKey(m map[string]any, excludeKey string) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		if k == excludeKey {
			continue
		}
		result[k] = v
	}
	return result
}

// SplitAgentConditionals partitions markdown content into universal and agent-specific
// buckets based on whether conditional blocks reference keel.agent.
//
// - Universal output contains all content that is NOT inside an agent-conditional block.
// - Agent-specific output maps agent names to their conditional content.
// - A conditional block is "agent-specific" if its expression references keel.agent
//   (e.g., "keel.agent == 'claude'" or "keel.agent == 'claude' && keel.scope == 'project'").
// - Non-agent conditionals (e.g., "keel.scope == 'project'") remain in universal output unchanged.
// - keel-elif chains that reference different agents split into multiple agent buckets.
// - keel-else in an agent-conditional block goes to universal (fallback for unknown agents).
//
// The function preserves all markers in the output — it splits, but doesn't evaluate.
func SplitAgentConditionals(content []byte) (universal []byte, agentAddenda map[string][]byte) {
	if len(content) == 0 {
		return content, nil
	}

	// Normalize \r\n to \n for consistent processing
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	lines := bytes.Split(normalized, []byte("\n"))

	agentAddenda = make(map[string][]byte)
	var universalOut bytes.Buffer
	var currentAgentOut *bytes.Buffer
	var currentAgent string
	var nestingDepth int          // Track nesting depth (nested conditionals inside agent blocks)
	var topLevelAgentBlock bool   // True when we're in a top-level agent conditional

	inCodeBlock := false
	firstUniversalLine := true
	agentFirstLine := make(map[string]bool)

	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)

		// Track fenced code blocks — toggle on ``` at start of line
		if bytes.HasPrefix(trimmed, []byte("```")) {
			inCodeBlock = !inCodeBlock
		}

		// Skip marker detection inside code blocks
		if inCodeBlock {
			if currentAgentOut != nil {
				if !agentFirstLine[currentAgent] {
					currentAgentOut.WriteByte('\n')
				}
				currentAgentOut.Write(line)
				agentFirstLine[currentAgent] = false
			} else {
				if !firstUniversalLine {
					universalOut.WriteByte('\n')
				}
				universalOut.Write(line)
				firstUniversalLine = false
			}
			continue
		}

		// Check for conditional markers
		if marker, expr := parseConditionalMarker(trimmed); marker != "" {
			switch marker {
			case "if":
				// Determine if this condition references keel.agent
				agent := extractAgentFromCondition(expr)

				if currentAgentOut != nil {
					// Inside an agent block — nested conditional goes to agent output
					nestingDepth++
					if !agentFirstLine[currentAgent] {
						currentAgentOut.WriteByte('\n')
					}
					currentAgentOut.Write(line)
					agentFirstLine[currentAgent] = false
				} else if agent != "" {
					// Starting a new top-level agent-specific block
					currentAgent = agent
					buf := new(bytes.Buffer)
					currentAgentOut = buf
					agentFirstLine[currentAgent] = true
					nestingDepth = 0
					topLevelAgentBlock = true
					currentAgentOut.Write(line)
					agentFirstLine[currentAgent] = false
				} else {
					// Non-agent conditional at top level — goes to universal
					if !firstUniversalLine {
						universalOut.WriteByte('\n')
					}
					universalOut.Write(line)
					firstUniversalLine = false
				}

			case "elif":
				agent := extractAgentFromCondition(expr)

				if nestingDepth > 0 {
					// Nested elif — goes to current agent output
					if !agentFirstLine[currentAgent] {
						currentAgentOut.WriteByte('\n')
					}
					currentAgentOut.Write(line)
					agentFirstLine[currentAgent] = false
				} else if topLevelAgentBlock && agent != "" {
					// Top-level elif with different agent — switch agents
					if currentAgentOut != nil {
						appendAddenda(agentAddenda, currentAgent, currentAgentOut.Bytes())
					}
					currentAgent = agent
					buf := new(bytes.Buffer)
					currentAgentOut = buf
					agentFirstLine[currentAgent] = true
					currentAgentOut.Write(line)
					agentFirstLine[currentAgent] = false
				} else if topLevelAgentBlock {
					// Top-level elif without agent reference — goes to universal
					if currentAgentOut != nil {
						appendAddenda(agentAddenda, currentAgent, currentAgentOut.Bytes())
						currentAgentOut = nil
						currentAgent = ""
						topLevelAgentBlock = false
					}
					if !firstUniversalLine {
						universalOut.WriteByte('\n')
					}
					universalOut.Write(line)
					firstUniversalLine = false
				} else {
					// elif at universal level
					if !firstUniversalLine {
						universalOut.WriteByte('\n')
					}
					universalOut.Write(line)
					firstUniversalLine = false
				}

			case "else":
				if nestingDepth > 0 {
					// Nested else — goes to current agent output
					if !agentFirstLine[currentAgent] {
						currentAgentOut.WriteByte('\n')
					}
					currentAgentOut.Write(line)
					agentFirstLine[currentAgent] = false
				} else if topLevelAgentBlock {
					// Top-level else in an agent block — close agent output and switch to universal
					if currentAgentOut != nil {
						agentAddenda[currentAgent] = currentAgentOut.Bytes()
						currentAgentOut = nil
						currentAgent = ""
						topLevelAgentBlock = false
					}
					if !firstUniversalLine {
						universalOut.WriteByte('\n')
					}
					universalOut.Write(line)
					firstUniversalLine = false
				} else {
					// else at universal level
					if !firstUniversalLine {
						universalOut.WriteByte('\n')
					}
					universalOut.Write(line)
					firstUniversalLine = false
				}

			case "endif":
				if nestingDepth > 0 {
					// Closing a nested conditional
					nestingDepth--
					if !agentFirstLine[currentAgent] {
						currentAgentOut.WriteByte('\n')
					}
					currentAgentOut.Write(line)
					agentFirstLine[currentAgent] = false
				} else if topLevelAgentBlock {
					// Closing the top-level agent block
					if currentAgentOut != nil {
						if !agentFirstLine[currentAgent] {
							currentAgentOut.WriteByte('\n')
						}
						currentAgentOut.Write(line)
						agentFirstLine[currentAgent] = false
						appendAddenda(agentAddenda, currentAgent, currentAgentOut.Bytes())
						currentAgentOut = nil
						currentAgent = ""
						topLevelAgentBlock = false
					}
				} else {
					// Closing a universal-level conditional
					if !firstUniversalLine {
						universalOut.WriteByte('\n')
					}
					universalOut.Write(line)
					firstUniversalLine = false
				}
			}
			continue
		}

		// Non-marker line: emit to current output (agent or universal)
		if currentAgentOut != nil {
			if !agentFirstLine[currentAgent] {
				currentAgentOut.WriteByte('\n')
			}
			currentAgentOut.Write(line)
			agentFirstLine[currentAgent] = false
		} else {
			if !firstUniversalLine {
				universalOut.WriteByte('\n')
			}
			universalOut.Write(line)
			firstUniversalLine = false
		}
	}

	// Flush any unclosed agent block (malformed input without matching endif).
	if currentAgentOut != nil {
		appendAddenda(agentAddenda, currentAgent, currentAgentOut.Bytes())
	}

	universal = universalOut.Bytes()

	// Preserve trailing newline if original had one
	if len(universal) > 0 && len(normalized) > 0 && normalized[len(normalized)-1] == '\n' && universal[len(universal)-1] != '\n' {
		universal = append(universal, '\n')
	}

	return universal, agentAddenda
}

// appendAddenda appends content to an agent's addenda entry, joining with
// a newline if the agent already has existing content.
func appendAddenda(addenda map[string][]byte, agent string, content []byte) {
	if existing, ok := addenda[agent]; ok {
		combined := make([]byte, 0, len(existing)+1+len(content))
		combined = append(combined, existing...)
		combined = append(combined, '\n')
		combined = append(combined, content...)
		addenda[agent] = combined
	} else {
		addenda[agent] = content
	}
}

// extractAgentFromCondition analyzes a conditional expression and returns the agent
// name if the expression references keel.agent with a simple equality check
// (e.g., "keel.agent == 'claude'"). Returns empty string if the expression does not
// reference keel.agent or uses a complex condition.
//
// Supported patterns:
//   - "keel.agent == 'claude'" → "claude"
//   - "keel.agent == 'claude' && keel.scope == 'project'" → "claude"
//   - "keel.scope == 'project'" → ""
//   - "keel.agent in ['claude', 'cursor']" → "" (complex, not a single agent)
func extractAgentFromCondition(expr string) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}

	// Split on OR — if there's an OR, it's complex (multiple agents or conditions)
	if strings.Contains(expr, "||") {
		return ""
	}

	// Split on AND — check each clause
	andParts := splitLogical(expr, "&&")
	var agentName string
	for _, part := range andParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if this part is "keel.agent == 'value'"
		opIdx := findOperator(part, "==")
		if opIdx < 0 {
			// Could be != or in/not in — we only handle == for extraction
			if strings.Contains(part, "keel.agent") {
				// Complex agent condition
				return ""
			}
			continue
		}

		left := strings.TrimSpace(part[:opIdx])
		right := strings.TrimSpace(part[opIdx+2:])

		if left != "keel.agent" {
			continue
		}

		// Right side must be a quoted string
		if len(right) < 2 {
			return ""
		}
		quote := right[0]
		if (quote != '\'' && quote != '"') || right[len(right)-1] != quote {
			return ""
		}

		agentName = right[1 : len(right)-1]
		break
	}

	return agentName
}

// SplitAgentConditionJSON partitions a JSON map into universal and agent-specific
// buckets based on whether _keel.condition expressions reference keel.agent.
//
// - Universal output contains all entries that do NOT have a _keel.condition or have
//   conditions that do NOT reference keel.agent.
// - Agent-specific output maps agent names to their conditional entries.
// - An entry is "agent-specific" if its _keel.condition references keel.agent
//   (e.g., "keel.agent == 'claude'").
//
// The function preserves _keel metadata in the output — it splits, but doesn't evaluate.
func SplitAgentConditionJSON(data map[string]any) (universal map[string]any, agentSpecific map[string]map[string]any) {
	if data == nil {
		return nil, nil
	}

	universal = make(map[string]any)
	agentSpecific = make(map[string]map[string]any)

	for key, value := range data {
		nested, isMap := value.(map[string]any)
		if !isMap {
			universal[key] = value
			continue
		}

		// Check for _keel within this nested map
		keelRaw, hasKeel := nested["_keel"]
		if !hasKeel {
			// No _keel — goes to universal
			universal[key] = value
			continue
		}

		keelMap, keelIsMap := keelRaw.(map[string]any)
		if !keelIsMap {
			// _keel is not a map — goes to universal
			universal[key] = value
			continue
		}

		// Check for condition expression
		condRaw, hasCondition := keelMap["condition"]
		if !hasCondition {
			// No condition — goes to universal
			universal[key] = value
			continue
		}

		condStr, ok := condRaw.(string)
		if !ok {
			// Non-string condition — goes to universal
			universal[key] = value
			continue
		}

		// Determine if this condition references keel.agent
		agent := extractAgentFromCondition(condStr)
		if agent != "" {
			// Agent-specific entry
			if agentSpecific[agent] == nil {
				agentSpecific[agent] = make(map[string]any)
			}
			agentSpecific[agent][key] = value
		} else {
			// Non-agent condition or complex condition — goes to universal
			universal[key] = value
		}
	}

	return universal, agentSpecific
}
