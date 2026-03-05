// SPDX-License-Identifier: MIT

package urlbar

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
)

var methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

type Model struct {
	urlInput   textinput.Model
	methodIdx  int
	focused    bool
	width      int
	selectOpen bool
	selectIdx  int
}

func New() Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "https://api.example.com/endpoint"
	ti.CharLimit = 2048
	ti.SetWidth(60)
	ti.Focus()

	return Model{
		urlInput:  ti,
		methodIdx: 0,
		focused:   true,
	}
}

func (m Model) Method() string {
	return methods[m.methodIdx]
}

func (m Model) URL() string {
	return m.urlInput.Value()
}

func (m *Model) SetURL(url string) {
	m.urlInput.SetValue(url)
	m.urlInput.SetWidth(m.inputWidth())
	// CursorEnd forces offsetRight recalculation for the new value length,
	// then CursorStart resets view to show the beginning of the URL.
	m.urlInput.CursorEnd()
	m.urlInput.CursorStart()
}

func (m *Model) SetMethod(method string) {
	for i, meth := range methods {
		if meth == method {
			m.methodIdx = i
			m.urlInput.SetWidth(m.inputWidth())
			return
		}
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) Focus() {
	m.focused = true
	m.urlInput.Focus()
}

func (m *Model) Blur() {
	m.focused = false
	m.urlInput.Blur()
	m.selectOpen = false
}

// SelectOpen returns true when the method dropdown is visible.
func (m Model) SelectOpen() bool {
	return m.selectOpen
}

// ToggleSelect opens or closes the method dropdown.
func (m *Model) ToggleSelect() {
	if m.selectOpen {
		m.selectOpen = false
	} else {
		m.selectOpen = true
		m.selectIdx = m.methodIdx
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.selectOpen {
			return m.updateSelect(msg)
		}
	}

	if !m.focused {
		return m, nil
	}

	var cmd tea.Cmd
	m.urlInput, cmd = m.urlInput.Update(msg)
	return m, cmd
}

func (m Model) updateSelect(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectIdx > 0 {
			m.selectIdx--
		}
	case "down", "j":
		if m.selectIdx < len(methods)-1 {
			m.selectIdx++
		}
	case "enter":
		m.methodIdx = m.selectIdx
		m.urlInput.SetWidth(m.inputWidth())
		m.selectOpen = false
	case "esc":
		m.selectOpen = false
	}
	return m, nil
}

func (m Model) methodBtnView() string {
	method := methods[m.methodIdx]
	return styles.MethodStyle(method).
		Padding(0, 1).
		Bold(true).
		Render(fmt.Sprintf(" %s ▼", method))
}

func (m Model) sendBtnView() string {
	return lipgloss.NewStyle().
		Background(styles.PrimaryColor).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Render("Send [Ctrl+S]")
}

// inputWidth computes the URL text input width based on the current terminal
// width and method button size.
func (m Model) inputWidth() int {
	width := m.width
	if width < 40 {
		width = 80
	}

	hint := styles.MutedStyle.Render(" [Alt+U]")

	methodW := lipgloss.Width(m.methodBtnView())
	sendW := lipgloss.Width(m.sendBtnView())
	hintW := lipgloss.Width(hint)
	// 3 for spaces between elements (method url hint send), 1 margin for cursor
	w := width - methodW - sendW - hintW - 4
	if w < 20 {
		w = 20
	}
	return w
}

func (m Model) View() string {
	hint := styles.MutedStyle.Render(" [Alt+U]")

	m.urlInput.SetWidth(m.inputWidth())
	urlStr := m.urlInput.View()

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		m.methodBtnView(),
		" ",
		urlStr,
		" ",
		hint,
		" ",
		m.sendBtnView(),
	)
}

// DropdownView returns the rendered dropdown to be overlaid by the parent.
func (m Model) DropdownView() string {
	var b strings.Builder
	for i, method := range methods {
		style := styles.MethodStyle(method).Padding(0, 1).Width(12)
		if i == m.selectIdx {
			style = style.Reverse(true)
		}
		b.WriteString(style.Render(method))
		if i < len(methods)-1 {
			b.WriteString("\n")
		}
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.PrimaryColor).
		Padding(0, 1).
		Render(b.String())
}

// IsMethodClick returns true if the given column falls within the method button area.
func (m Model) IsMethodClick(col int) bool {
	return col < lipgloss.Width(m.methodBtnView())
}

// IsSendClick returns true if the given column falls within the Send button area.
func (m Model) IsSendClick(col int) bool {
	methodW := lipgloss.Width(m.methodBtnView())
	sendW := lipgloss.Width(m.sendBtnView())
	hintW := lipgloss.Width(styles.MutedStyle.Render(" [Alt+U]"))
	// Send button starts after: methodBtn + " " + urlInput + " " + hint + " "
	sendStart := methodW + 1 + m.inputWidth() + 1 + hintW + 1
	sendEnd := sendStart + sendW
	return col >= sendStart && col < sendEnd
}

// ClickDropdown handles a mouse click on the dropdown overlay.
// row is relative to the dropdown top (overlayAt row=1, so absolute row - 1).
// col is the absolute X coordinate.
// Returns true if a method was selected.
func (m *Model) ClickDropdown(row, col int) bool {
	// Dropdown width: border(1) + padding(1) + content(12) + padding(1) + border(1) = 16
	const dropdownW = 16
	if col < 0 || col >= dropdownW {
		return false
	}
	// row 0 = top border, row 1..len(methods) = items, after = bottom border
	idx := row - 1
	if idx < 0 || idx >= len(methods) {
		return false
	}
	m.methodIdx = idx
	m.urlInput.SetWidth(m.inputWidth())
	m.selectOpen = false
	return true
}

func (m *Model) SetWidth(w int) {
	m.width = w
	m.urlInput.SetWidth(m.inputWidth())
}
