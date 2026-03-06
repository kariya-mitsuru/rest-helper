// SPDX-License-Identifier: MIT

package request

import (
	"encoding/json"
	"fmt"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"gopkg.in/yaml.v3"

	"rest-helper/internal/ui/styles"
)

type BodyFormat int

const (
	FormatJSON BodyFormat = iota
	FormatYAML
)

var formatNames = map[BodyFormat]string{
	FormatJSON: "JSON",
	FormatYAML: "YAML",
}

type BodyModel struct {
	textarea textarea.Model
	format   BodyFormat
	focused  bool
}

func NewBody() BodyModel {
	ta := textarea.New()
	ta.Placeholder = "key: value"
	ta.CharLimit = 0
	ta.SetWidth(60)
	ta.SetHeight(10)
	ta.ShowLineNumbers = false
	ta.Prompt = ""

	return BodyModel{
		textarea: ta,
		format:   FormatYAML,
	}
}

// Value returns the raw text as entered.
func (m BodyModel) Value() string {
	return m.textarea.Value()
}

// JSONValue returns the body as JSON. If format is YAML, converts to JSON.
func (m BodyModel) JSONValue() (string, error) {
	raw := m.textarea.Value()
	if raw == "" {
		return "", nil
	}

	switch m.format {
	case FormatYAML:
		return yamlToJSON(raw)
	default:
		return raw, nil
	}
}

func (m BodyModel) Format() BodyFormat {
	return m.format
}

func (m *BodyModel) SetFormat(f BodyFormat) {
	m.format = f
	m.updatePlaceholder()
}

func (m *BodyModel) ToggleFormat() {
	if m.format == FormatJSON {
		m.format = FormatYAML
	} else {
		m.format = FormatJSON
	}
	m.updatePlaceholder()
}

func (m *BodyModel) updatePlaceholder() {
	if m.format == FormatYAML {
		m.textarea.Placeholder = "key: value"
	} else {
		m.textarea.Placeholder = `{"key": "value"}`
	}
}

func (m *BodyModel) SetValue(v string) {
	m.textarea.SetValue(v)
}

func (m *BodyModel) Focus() {
	m.focused = true
	m.textarea.Focus()
}

func (m *BodyModel) Blur() {
	m.focused = false
	m.textarea.Blur()
}

func (m *BodyModel) SetSize(w, h int) {
	m.textarea.SetWidth(w - 4)
	m.textarea.SetHeight(h - 1)
}

func (m BodyModel) Update(msg tea.Msg) (BodyModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+t" {
			m.ToggleFormat()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m BodyModel) View() string {
	content := m.textarea.View()

	formatLabel := styles.ActiveTab.Render(formatNames[m.format])

	content += "\n  " + formatLabel
	if m.focused {
		content += styles.MutedStyle.Render("  ctrl+t: toggle format")
	}
	return content
}

func yamlToJSON(yamlStr string) (string, error) {
	var data any
	if err := yaml.Unmarshal([]byte(yamlStr), &data); err != nil {
		return "", fmt.Errorf("YAML parse error: %w", err)
	}

	data = convertYAMLToJSON(data)

	b, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("JSON marshal error: %w", err)
	}
	return string(b), nil
}

// convertYAMLToJSON recursively converts map[string]any (from yaml) to
// a structure that json.Marshal handles correctly. YAML produces
// map[string]any which is fine, but older versions may produce
// map[any]any which needs conversion.
func convertYAMLToJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(val))
		for k, v := range val {
			m[k] = convertYAMLToJSON(v)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(val))
		for k, v := range val {
			m[fmt.Sprintf("%v", k)] = convertYAMLToJSON(v)
		}
		return m
	case []any:
		for i, item := range val {
			val[i] = convertYAMLToJSON(item)
		}
		return val
	default:
		return v
	}
}
