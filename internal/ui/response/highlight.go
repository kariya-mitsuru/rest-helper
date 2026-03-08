// SPDX-License-Identifier: MIT

package response

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"gopkg.in/yaml.v3"

	"rest-helper/internal/ui/styles"
)

// Shared color palette for syntax highlighting, using the shared styles palette.
var emptyResponseText = styles.MutedStyle.Render("(empty response)")

var (
	hlKeyStyle   = lipgloss.NewStyle().Foreground(styles.SecondaryColor)
	hlStrStyle   = lipgloss.NewStyle().Foreground(styles.SuccessColor)
	hlNumStyle   = lipgloss.NewStyle().Foreground(styles.WarningColor)
	hlBoolStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5CF6"))
	hlNullStyle  = lipgloss.NewStyle().Foreground(styles.MutedColor)
	hlPunctStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
)

func formatBody(body string) string {
	if body == "" {
		return emptyResponseText
	}

	// Try to pretty-print JSON
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(body), "", "  "); err == nil {
		return syntaxHighlight(buf.String())
	}

	return body
}

func formatAsYAML(body string) string {
	var data any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return formatBody(body)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(data); err != nil {
		return formatBody(body)
	}

	return syntaxHighlightYAML(buf.String())
}

func syntaxHighlightYAML(yamlStr string) string {
	var result strings.Builder

	lines := strings.Split(strings.TrimRight(yamlStr, "\n"), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := line[:len(line)-len(trimmed)]
		result.WriteString(indent)

		if strings.HasPrefix(trimmed, "- ") {
			// List item
			result.WriteString(hlPunctStyle.Render("- "))
			rest := trimmed[2:]
			if strings.Contains(rest, ": ") {
				result.WriteString(colorizeYAMLKeyValue(rest))
			} else {
				result.WriteString(colorizeYAMLValue(rest))
			}
		} else if strings.Contains(trimmed, ": ") {
			result.WriteString(colorizeYAMLKeyValue(trimmed))
		} else if strings.HasSuffix(trimmed, ":") {
			// Key with no value (nested object/array follows)
			result.WriteString(hlKeyStyle.Render(strings.TrimSuffix(trimmed, ":")))
			result.WriteString(hlPunctStyle.Render(":"))
		} else {
			result.WriteString(colorizeYAMLValue(trimmed))
		}

		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func colorizeYAMLKeyValue(s string) string {
	parts := strings.SplitN(s, ": ", 2)
	key := parts[0]
	val := ""
	if len(parts) > 1 {
		val = parts[1]
	}
	return hlKeyStyle.Render(key) + hlPunctStyle.Render(": ") + colorizeYAMLValue(val)
}

func colorizeYAMLValue(val string) string {
	switch {
	case val == "null" || val == "~":
		return hlNullStyle.Render(val)
	case val == "true" || val == "false":
		return hlBoolStyle.Render(val)
	case len(val) > 0 && (val[0] >= '0' && val[0] <= '9' || val[0] == '-' || val[0] == '.'):
		return hlNumStyle.Render(val)
	case strings.HasPrefix(val, "'") || strings.HasPrefix(val, "\""):
		return hlStrStyle.Render(val)
	case val == "[]" || val == "{}":
		return hlNullStyle.Render(val)
	default:
		return hlStrStyle.Render(val)
	}
}

func syntaxHighlight(jsonStr string) string {
	var result strings.Builder

	// Colorize on original (unwrapped) lines
	lines := strings.Split(jsonStr, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := line[:len(line)-len(trimmed)]
		result.WriteString(indent)

		if strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			key := strings.TrimSpace(parts[0])
			val := ""
			if len(parts) > 1 {
				val = strings.TrimSpace(parts[1])
			}
			result.WriteString(hlKeyStyle.Render(key))
			result.WriteString(hlPunctStyle.Render(": "))
			result.WriteString(colorizeValue(val))
		} else {
			result.WriteString(colorizeValue(trimmed))
		}

		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func colorizeValue(val string) string {
	cleaned := strings.TrimSuffix(val, ",")
	trailing := ""
	if strings.HasSuffix(val, ",") {
		trailing = hlPunctStyle.Render(",")
	}

	switch {
	case cleaned == "{" || cleaned == "}" || cleaned == "[" || cleaned == "]" ||
		cleaned == "{}" || cleaned == "[]":
		return hlPunctStyle.Render(cleaned) + trailing
	case cleaned == "null":
		return hlNullStyle.Render(cleaned) + trailing
	case cleaned == "true" || cleaned == "false":
		return hlBoolStyle.Render(cleaned) + trailing
	case strings.HasPrefix(cleaned, "\""):
		return hlStrStyle.Render(cleaned) + trailing
	case len(cleaned) > 0 && (cleaned[0] >= '0' && cleaned[0] <= '9' || cleaned[0] == '-'):
		return hlNumStyle.Render(cleaned) + trailing
	default:
		return val
	}
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
