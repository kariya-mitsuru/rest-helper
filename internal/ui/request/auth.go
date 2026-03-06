// SPDX-License-Identifier: MIT

package request

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
)

var authTypes = []string{"None", "Bearer", "Basic", "Custom"}

type AuthModel struct {
	typeIdx    int
	tokenInput textinput.Model
	focused    bool
	width      int
	selectOpen bool
	selectIdx  int
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
	// content width: panel width - border(2) - indent(2)
	tokenW := w - 4 - 2
	if tokenW < 20 {
		tokenW = 20
	}
	m.tokenInput.SetWidth(tokenW)
}

// SelectOpen returns true when the auth type dropdown is visible.
func (m AuthModel) SelectOpen() bool {
	return m.selectOpen
}

// ToggleSelect opens or closes the auth type dropdown.
func (m *AuthModel) ToggleSelect() {
	if m.selectOpen {
		m.selectOpen = false
	} else {
		m.selectOpen = true
		m.selectIdx = m.typeIdx
	}
}

// DropdownView returns the rendered dropdown overlay.
func (m AuthModel) DropdownView() string {
	var b strings.Builder
	for i, t := range authTypes {
		style := lipgloss.NewStyle().Foreground(styles.TextColor).Width(12).Padding(0, 1)
		if i == m.selectIdx {
			style = style.Reverse(true)
		}
		b.WriteString(style.Render(t))
		if i < len(authTypes)-1 {
			b.WriteString("\n")
		}
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.PrimaryColor).
		Padding(0, 1).
		Render(b.String())
}

// ClickDropdown handles a mouse click on the dropdown overlay.
// row is relative to the dropdown top, col is relative to the dropdown left.
// Returns true if an item was selected.
func (m *AuthModel) ClickDropdown(row, col int) bool {
	// Dropdown width: border(1) + padding(1) + content(12) + padding(1) + border(1) = 16
	const dropdownW = 16
	if col < 0 || col >= dropdownW {
		return false
	}
	// row 0 = top border, row 1..len(authTypes) = items, after = bottom border
	idx := row - 1
	if idx < 0 || idx >= len(authTypes) {
		return false
	}
	m.typeIdx = idx
	m.selectOpen = false
	m.updateTokenInputFocus()
	return true
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
	m.selectOpen = false
}

func (m AuthModel) Update(msg tea.Msg) (AuthModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Dropdown open: capture navigation keys
		if m.selectOpen {
			switch msg.String() {
			case "up", "k":
				if m.selectIdx > 0 {
					m.selectIdx--
				}
			case "down", "j":
				if m.selectIdx < len(authTypes)-1 {
					m.selectIdx++
				}
			case "enter":
				m.typeIdx = m.selectIdx
				m.selectOpen = false
				m.updateTokenInputFocus()
			case "esc":
				m.selectOpen = false
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

	// Compact button: "  Auth Type  None ▼"
	typeName := authTypes[m.typeIdx]
	label := lipgloss.NewStyle().Bold(true).Render("Auth Type")
	typeBtn := lipgloss.NewStyle().Bold(true).Foreground(styles.PrimaryColor).Render(typeName + " ▼")
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  %s", label, typeBtn))

	if m.typeIdx > 0 {
		b.WriteString("\n\n")
		tokenLabel := lipgloss.NewStyle().Bold(true).Render("  Token")
		hint := "  " + styles.MutedStyle.Underline(true).Render("toggle visibility [Ctrl+E]")
		b.WriteString(tokenLabel + hint)
		b.WriteString("\n")
		b.WriteString("  " + m.tokenInput.View())
	}

	return b.String()
}
