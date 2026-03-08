// SPDX-License-Identifier: MIT

package request

import (
	"encoding/json"
	"fmt"

	"rest-helper/internal/ui/widget/textarea"

	tea "charm.land/bubbletea/v2"
	"gopkg.in/yaml.v3"
)

type bodyFormat int

const (
	formatJSON bodyFormat = iota
	formatYAML
)

var formatNames = map[bodyFormat]string{
	formatJSON: "JSON",
	formatYAML: "YAML",
}

type BodyModel struct {
	textarea    textarea.Model
	format      bodyFormat
	focused     bool
	allocHeight int // height allocated by parent (before scrollbar adjustment)
}

func NewBody() BodyModel {
	ta := textarea.New()
	ta.Placeholder = "key: value"
	ta.CharLimit = 0
	ta.SetWidth(60)
	ta.SetHeight(10)
	ta.Prompt = ""

	return BodyModel{
		textarea: ta,
		format:   formatYAML,
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
	case formatYAML:
		return yamlToJSON(raw)
	default:
		return raw, nil
	}
}

func (m BodyModel) Format() bodyFormat {
	return m.format
}

func (m *BodyModel) SetFormat(f bodyFormat) {
	m.format = f
	m.updatePlaceholder()
}

func (m *BodyModel) ToggleFormat() {
	if m.format == formatJSON {
		m.format = formatYAML
	} else {
		m.format = formatJSON
	}
	m.updatePlaceholder()
}

func (m *BodyModel) updatePlaceholder() {
	if m.format == formatYAML {
		m.textarea.Placeholder = "key: value"
	} else {
		m.textarea.Placeholder = `{"key": "value"}`
	}
}

func (m *BodyModel) SetValue(v string) {
	m.textarea.SetValue(v)
	// SetValue leaves cursor at end; move to top so the view starts there.
	// textarea.Update ignores input when not focused, so temporarily focus.
	wasFocused := m.textarea.Focused()
	if !wasFocused {
		m.textarea.Focus()
	}
	m.textarea, _ = m.textarea.Update(tea.KeyPressMsg{Code: tea.KeyHome, Mod: tea.ModCtrl})
	if !wasFocused {
		m.textarea.Blur()
	}
}

func (m *BodyModel) Focus() {
	m.focused = true
	m.textarea.Focus()
}

func (m *BodyModel) Blur() {
	m.focused = false
	m.textarea.Blur()
}

func (m *BodyModel) ToggleWrap() {
	m.textarea.SetWrapMode(!m.textarea.WrapMode())
	h := m.allocHeight
	if !m.textarea.WrapMode() {
		h-- // reserve 1 row for horizontal scrollbar
	}
	m.textarea.SetHeight(h)
}

func (m *BodyModel) WrapMode() bool {
	return m.textarea.WrapMode()
}

func (m *BodyModel) SetSize(w, h int) {
	m.allocHeight = h
	m.textarea.SetWidth(w - 4) // border(2) + padding(1) + right margin(1)
	if !m.textarea.WrapMode() {
		h-- // reserve 1 row for horizontal scrollbar
	}
	m.textarea.SetHeight(h)
}

func (m BodyModel) Update(msg tea.Msg) (BodyModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+t":
			m.ToggleFormat()
			return m, nil
		case "ctrl+w":
			m.ToggleWrap()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m BodyModel) View() string {
	return m.textarea.View()
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
