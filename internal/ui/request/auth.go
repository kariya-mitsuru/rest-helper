// SPDX-License-Identifier: MIT

package request

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
	"rest-helper/internal/ui/widget"
)

var authTypes = []string{"None", "Bearer", "Basic", "Custom"}

type AuthModel struct {
	typeIdx    int
	tokenInput textinput.Model
	focused    bool
	width      int
	dropdown   widget.Dropdown
}

func NewAuth() AuthModel {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Enter token..."
	ti.CharLimit = 4096
	ti.SetWidth(50)
	ti.EchoMode = textinput.EchoPassword

	return AuthModel{
		typeIdx:    0,
		tokenInput: ti,
	}
}

func (m AuthModel) AuthHeader() (string, string) {
	token := strings.TrimSpace(m.tokenInput.Value())
	if token == "" {
		return "", ""
	}

	switch authTypes[m.typeIdx] {
	case "Bearer":
		return "Authorization", "Bearer " + token
	case "Basic":
		return "Authorization", "Basic " + token
	case "Custom":
		return "Authorization", token
	default:
		return "", ""
	}
}

func (m *AuthModel) SetToken(tokenType, token string) {
	for i, t := range authTypes {
		if t == tokenType {
			m.typeIdx = i
			break
		}
	}
	m.tokenInput.SetValue(token)
}

func (m *AuthModel) SetWidth(w int) {
	m.width = w
	// content width: panel width - border(2) - padding(1) - indent(2)
	tokenW := w - 5 - 2
	if tokenW < 20 {
		tokenW = 20
	}
	m.tokenInput.SetWidth(tokenW)
}

// SelectOpen returns true when the auth type dropdown is visible.
func (m AuthModel) SelectOpen() bool {
	return m.dropdown.IsOpen()
}

// ToggleSelect opens or closes the auth type dropdown.
func (m *AuthModel) ToggleSelect() {
	m.dropdown.Toggle(m.typeIdx)
}

// DropdownView returns the rendered dropdown overlay.
func (m AuthModel) DropdownView() string {
	return m.dropdown.Render(authTypes, func(item string, _ int, highlighted bool) lipgloss.Style {
		style := lipgloss.NewStyle().Foreground(styles.TextColor).Width(12).Padding(0, 1)
		if highlighted {
			style = style.Reverse(true)
		}
		return style
	})
}

// ClickDropdown handles a mouse click on the dropdown overlay.
// row is relative to the dropdown top, col is relative to the dropdown left.
// Returns true if an item was selected.
func (m *AuthModel) ClickDropdown(row, col int) bool {
	const dropdownW = 16
	idx, ok := m.dropdown.HandleClick(row, col, len(authTypes), dropdownW)
	if ok {
		m.typeIdx = idx
		m.updateTokenInputFocus()
	}
	return ok
}

// HasTokenField returns true when a token type (non-None) is selected.
func (m AuthModel) HasTokenField() bool {
	return m.typeIdx > 0
}

// updateTokenInputFocus focuses or blurs the token input based on typeIdx.
func (m *AuthModel) updateTokenInputFocus() {
	if m.typeIdx > 0 {
		m.tokenInput.Focus()
	} else {
		m.tokenInput.Blur()
	}
}

// ToggleTokenVisibility switches the token between password and plain text.
func (m *AuthModel) ToggleTokenVisibility() {
	if m.tokenInput.EchoMode == textinput.EchoPassword {
		m.tokenInput.EchoMode = textinput.EchoNormal
	} else {
		m.tokenInput.EchoMode = textinput.EchoPassword
	}
}

func (m *AuthModel) Focus() {
	m.focused = true
	if m.typeIdx > 0 {
		m.tokenInput.Focus()
	}
}

func (m *AuthModel) Blur() {
	m.focused = false
	m.tokenInput.Blur()
	m.dropdown.Close()
}

func (m AuthModel) Update(msg tea.Msg) (AuthModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Dropdown open: capture all keys
		if m.dropdown.IsOpen() {
			if idx, handled := m.dropdown.HandleKey(msg.String(), len(authTypes)); handled {
				if idx >= 0 {
					m.typeIdx = idx
					m.updateTokenInputFocus()
				}
			}
			return m, nil
		}

		// Dropdown closed
		switch msg.String() {
		case "up":
			if m.typeIdx > 0 {
				m.typeIdx--
				m.updateTokenInputFocus()
			}
			return m, nil
		case "down":
			if m.typeIdx < len(authTypes)-1 {
				m.typeIdx++
				m.updateTokenInputFocus()
			}
			return m, nil
		case "ctrl+e":
			m.ToggleTokenVisibility()
			return m, nil
		}
	}

	if m.typeIdx > 0 {
		var cmd tea.Cmd
		m.tokenInput, cmd = m.tokenInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m AuthModel) View() string {
	var b strings.Builder

	// Auth type row: label + button (button rendered as child layer in ViewLayer)
	label := styles.BoldStyle.Render("Auth Type")
	b.WriteString("\n")
	b.WriteString("  " + label)

	if m.typeIdx > 0 {
		b.WriteString("\n\n")
		tokenLabel := styles.BoldStyle.Render("  Token")
		b.WriteString(tokenLabel)
		b.WriteString("\n")
		b.WriteString("  " + m.tokenInput.View())
	}

	return b.String()
}
