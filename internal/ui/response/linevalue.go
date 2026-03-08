// SPDX-License-Identifier: MIT

package response

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// lineInfo holds the key and value extracted from a single display line.
type lineInfo struct {
	key   string // key name (unquoted); empty for array elements / raw lines
	value string // plain value for "Copy value"
	raw   string // JSON-encoded value token (for formatting key+value pairs)
}

// formatJSON returns the key+value as a JSON pair: "key": value
func (l lineInfo) formatJSON() string {
	jsonKey, _ := json.Marshal(l.key)
	return fmt.Sprintf("%s: %s", jsonKey, l.raw)
}

// formatYAML returns the key+value as YAML.
// For structured values (objects/arrays) it produces multi-line YAML.
func (l lineInfo) formatYAML() string {
	// Check if the value is structured JSON (object or array)
	var data any
	if err := json.Unmarshal([]byte(l.raw), &data); err == nil {
		switch data.(type) {
		case map[string]any, []any:
			wrapper := map[string]any{l.key: data}
			out, err := yaml.Marshal(wrapper)
			if err == nil {
				return strings.TrimRight(string(out), "\n")
			}
		}
	}
	// Scalar value
	if yamlNeedsQuoting(l.value) {
		return fmt.Sprintf("%s: %q", l.key, l.value)
	}
	return fmt.Sprintf("%s: %s", l.key, l.value)
}

// yamlNeedsQuoting returns true when a plain YAML scalar would be ambiguous.
func yamlNeedsQuoting(v string) bool {
	if v == "" {
		return true
	}
	switch v {
	case "null", "~", "true", "false":
		return true
	}
	if v[0] == '{' || v[0] == '[' || v[0] == '"' || v[0] == '\'' ||
		v[0] == '|' || v[0] == '>' || v[0] == '%' || v[0] == '@' || v[0] == '`' {
		return true
	}
	if strings.ContainsAny(v, ":#") {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Top-level dispatcher
// ---------------------------------------------------------------------------

// buildLineInfos rebuilds the lineInfos slice to match rawLines.
func (m *Model) buildLineInfos() {
	if m.response == nil {
		m.lineInfos = nil
		return
	}

	if m.activeTab == TabHeaders {
		m.lineInfos = m.buildHeaderLineInfos()
		return
	}

	body := m.response.Body
	if body == "" {
		m.lineInfos = nil
		return
	}

	switch m.display {
	case formatJSON:
		m.lineInfos = buildJSONLineInfos(body)
	case formatYAML:
		if m.isBodyJSON() {
			m.lineInfos = buildYAMLLineInfos(body)
		} else {
			m.lineInfos = buildRawLineInfos(body)
		}
	default:
		m.lineInfos = buildRawLineInfos(body)
	}
}

// ---------------------------------------------------------------------------
// JSON
// ---------------------------------------------------------------------------

// buildJSONLineInfos parses JSON, pretty-prints it, and extracts key+value per line.
func buildJSONLineInfos(body string) []lineInfo {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(body), "", "  "); err != nil {
		return buildRawLineInfos(body)
	}

	lines := strings.Split(buf.String(), "\n")
	infos := make([]lineInfo, len(lines))
	for i, line := range lines {
		infos[i] = extractJSONLineInfo(line)
	}

	// Fill subtree values for keys pointing to nested structures
	// and for bare array elements that are objects/arrays.
	fillJSONSubtrees(lines, infos)

	return infos
}

// extractJSONLineInfo extracts key, value, and raw JSON value from a pretty-printed line.
func extractJSONLineInfo(line string) lineInfo {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimSuffix(trimmed, ",")

	// Structural tokens
	if trimmed == "" || trimmed == "{" || trimmed == "}" ||
		trimmed == "[" || trimmed == "]" {
		return lineInfo{}
	}
	// Empty containers are leaf values
	if trimmed == "{}" || trimmed == "[]" {
		return lineInfo{value: trimmed, raw: trimmed}
	}

	// "key": value pattern — find end of key string
	if strings.HasPrefix(trimmed, "\"") {
		for i := 1; i < len(trimmed); i++ {
			if trimmed[i] == '\\' {
				i++
				continue
			}
			if trimmed[i] == '"' {
				keyRaw := trimmed[:i+1]
				rest := trimmed[i+1:]
				if strings.HasPrefix(rest, ": ") {
					val := strings.TrimSpace(rest[2:])
					val = strings.TrimSuffix(val, ",")
					var key string
					_ = json.Unmarshal([]byte(keyRaw), &key)
					if val == "{" || val == "[" {
						// Nested structure — value will be filled by fillJSONSubtrees
						return lineInfo{key: key}
					}
					return lineInfo{key: key, value: unquoteJSON(val), raw: val}
				}
				break
			}
		}
	}

	// Standalone value (array element)
	return lineInfo{value: unquoteJSON(trimmed), raw: trimmed}
}

// fillJSONSubtrees fills value/raw for lines that start nested structures.
func fillJSONSubtrees(lines []string, infos []lineInfo) {
	for i := range infos {
		if infos[i].value != "" {
			continue // already has a value
		}
		if infos[i].key != "" {
			// Key with nested value (e.g. "items": [)
			if raw := extractJSONSubtreeRaw(lines, i); raw != "" {
				infos[i].value = raw
				infos[i].raw = raw
			}
			continue
		}
		// Bare { or [ that is an array element (skip root brackets)
		if i == 0 || i == len(lines)-1 {
			continue
		}
		trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(lines[i]), ","))
		if trimmed == "{" || trimmed == "[" {
			if raw := extractJSONSubtreeRaw(lines, i); raw != "" {
				infos[i].value = raw
				infos[i].raw = raw
			}
		}
	}
}

// extractJSONSubtreeRaw finds the opening bracket on startLine, collects
// lines through the matching close bracket, and returns compact JSON.
func extractJSONSubtreeRaw(lines []string, startLine int) string {
	line := lines[startLine]
	bracketIdx := strings.LastIndexAny(line, "{[")
	if bracketIdx < 0 {
		return ""
	}

	depth := 0
	inString := false
	var parts []string

	for i := startLine; i < len(lines); i++ {
		var scanStart int
		if i == startLine {
			scanStart = bracketIdx
			parts = append(parts, line[bracketIdx:])
		} else {
			scanStart = 0
			parts = append(parts, lines[i])
		}

		scanLine := lines[i]
		for j := scanStart; j < len(scanLine); j++ {
			ch := scanLine[j]
			if inString {
				if ch == '\\' {
					j++
					continue
				}
				if ch == '"' {
					inString = false
				}
				continue
			}
			if ch == '"' {
				inString = true
				continue
			}
			if ch == '{' || ch == '[' {
				depth++
			}
			if ch == '}' || ch == ']' {
				depth--
			}
		}
		if depth == 0 {
			break
		}
	}

	subtree := strings.Join(parts, "\n")
	subtree = strings.TrimRight(subtree, " \t\n")
	subtree = strings.TrimSuffix(subtree, ",")

	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(subtree)); err != nil {
		return ""
	}
	return compact.String()
}

// unquoteJSON returns an unquoted string for JSON string values,
// or the original value for other types.
func unquoteJSON(val string) string {
	if strings.HasPrefix(val, "\"") {
		var s string
		if err := json.Unmarshal([]byte(val), &s); err == nil {
			return s
		}
	}
	return val
}

// ---------------------------------------------------------------------------
// YAML
// ---------------------------------------------------------------------------

// buildYAMLLineInfos generates YAML from the JSON body and extracts key+value per line.
func buildYAMLLineInfos(body string) []lineInfo {
	var data any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return nil
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(data); err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	infos := make([]lineInfo, len(lines))
	for i, line := range lines {
		infos[i] = extractYAMLLineInfo(line)
	}

	// Fill subtree values for keys with nested structures
	fillYAMLSubtrees(lines, infos)

	return infos
}

// extractYAMLLineInfo extracts key, value, and raw JSON value from a YAML line.
func extractYAMLLineInfo(line string) lineInfo {
	trimmed := strings.TrimSpace(line)

	// List item: "- key: value", "- key:", or "- value"
	if strings.HasPrefix(trimmed, "- ") {
		rest := trimmed[2:]
		if strings.Contains(rest, ": ") {
			parts := strings.SplitN(rest, ": ", 2)
			return lineInfo{key: parts[0], value: parts[1], raw: yamlValueToJSON(parts[1])}
		}
		if strings.HasSuffix(rest, ":") {
			key := strings.TrimSuffix(rest, ":")
			return lineInfo{key: key} // nested value, filled later
		}
		return lineInfo{value: rest, raw: yamlValueToJSON(rest)}
	}

	// Key: value
	if strings.Contains(trimmed, ": ") {
		parts := strings.SplitN(trimmed, ": ", 2)
		return lineInfo{key: parts[0], value: parts[1], raw: yamlValueToJSON(parts[1])}
	}

	// Key with no value (nested structure follows)
	if strings.HasSuffix(trimmed, ":") {
		key := strings.TrimSuffix(trimmed, ":")
		return lineInfo{key: key} // filled later
	}

	return lineInfo{value: trimmed, raw: yamlValueToJSON(trimmed)}
}

// fillYAMLSubtrees fills value/raw for YAML lines with nested structures.
func fillYAMLSubtrees(lines []string, infos []lineInfo) {
	for i := range infos {
		if infos[i].key != "" && infos[i].value == "" {
			subtreeYAML := extractYAMLSubtree(lines, i)
			if subtreeYAML == "" {
				continue
			}
			var subtreeData any
			if err := yaml.Unmarshal([]byte(subtreeYAML), &subtreeData); err == nil {
				raw, _ := json.Marshal(subtreeData)
				infos[i].value = string(raw)
				infos[i].raw = string(raw)
			}
		}
	}
}

// extractYAMLSubtree extracts the indented block following startLine.
func extractYAMLSubtree(lines []string, startLine int) string {
	if startLine >= len(lines)-1 {
		return ""
	}

	nextLine := lines[startLine+1]
	baseIndent := len(nextLine) - len(strings.TrimLeft(nextLine, " "))

	var subtreeLines []string
	for i := startLine + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent < baseIndent {
			break
		}
		if len(line) >= baseIndent {
			subtreeLines = append(subtreeLines, line[baseIndent:])
		} else {
			subtreeLines = append(subtreeLines, line)
		}
	}

	return strings.Join(subtreeLines, "\n")
}

// yamlValueToJSON converts a plain YAML scalar to its JSON representation.
func yamlValueToJSON(val string) string {
	switch {
	case val == "null" || val == "~":
		return "null"
	case val == "true" || val == "false":
		return val
	default:
		if len(val) > 0 && (val[0] >= '0' && val[0] <= '9' || val[0] == '-' || val[0] == '.') {
			if json.Valid([]byte(val)) {
				return val
			}
		}
		b, _ := json.Marshal(val)
		return string(b)
	}
}

// ---------------------------------------------------------------------------
// Headers / RAW
// ---------------------------------------------------------------------------

// buildHeaderLineInfos returns one lineInfo per header line (sorted by key).
func (m *Model) buildHeaderLineInfos() []lineInfo {
	keys := m.sortedHeaderKeys()
	if len(keys) == 0 {
		return nil
	}

	var infos []lineInfo
	for _, k := range keys {
		for _, v := range m.response.Headers[k] {
			raw, _ := json.Marshal(v)
			infos = append(infos, lineInfo{
				key:   k,
				value: v,
				raw:   string(raw),
			})
		}
	}
	return infos
}

// buildRawLineInfos returns each line of the body as its own value (no key).
func buildRawLineInfos(body string) []lineInfo {
	lines := strings.Split(body, "\n")
	infos := make([]lineInfo, len(lines))
	for i, l := range lines {
		infos[i] = lineInfo{value: l}
	}
	return infos
}
