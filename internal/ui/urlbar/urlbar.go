// SPDX-License-Identifier: MIT

package urlbar

import (
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
	"rest-helper/internal/ui/widget"
)

var methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

type Model struct {
	urlInput  textinput.Model
	methodIdx int
	focused   bool
	width     int
	dropdown  widget.Dropdown
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
	m.dropdown.Close()
}

// SelectOpen returns true when the method dropdown is visible.
func (m Model) SelectOpen() bool {
	return m.dropdown.IsOpen()
}

// ToggleSelect opens or closes the method dropdown.
func (m *Model) ToggleSelect() {
	m.dropdown.Toggle(m.methodIdx)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.dropdown.IsOpen() {
			if idx, handled := m.dropdown.HandleKey(msg.String(), len(methods)); handled {
				if idx >= 0 {
					m.methodIdx = idx
					m.urlInput.SetWidth(m.inputWidth())
				}
			}
			return m, nil
		}
	}

	if !m.focused {
		return m, nil
	}

	var cmd tea.Cmd
	m.urlInput, cmd = m.urlInput.Update(msg)
	return m, cmd
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

func (m Model) histBtnView() string {
	return lipgloss.NewStyle().Foreground(styles.TextColor).Render(" ▾")
}

func (m Model) hintView() string {
	return styles.MutedStyle.Render(" [Alt+U]")
}

// inputWidth computes the URL text input width based on the current terminal
// width and method button size.
func (m Model) inputWidth() int {
	width := m.width
	if width < 40 {
		width = 80
	}

	methodW := lipgloss.Width(m.methodBtnView())
	sendW := lipgloss.Width(m.sendBtnView())
	hintW := lipgloss.Width(m.hintView())
	histW := lipgloss.Width(m.histBtnView())
	// spaces between elements (method url hist hint send), 1 margin for cursor
	w := width - methodW - sendW - hintW - histW - 4
	if w < 20 {
		w = 20
	}
	return w
}

func (m Model) ViewLayer() *lipgloss.Layer {
	methodBtn := m.methodBtnView()
	methodW := lipgloss.Width(methodBtn)

	histBtn := m.histBtnView()
	histW := lipgloss.Width(histBtn)

	hint := m.hintView()
	hintW := lipgloss.Width(hint)

	m.urlInput.SetWidth(m.inputWidth())
	urlStr := m.urlInput.View()
	urlW := lipgloss.Width(urlStr)

	sendBtn := m.sendBtnView()

	urlX := methodW + 1
	histX := urlX + urlW + 1
	hintX := histX + histW
	sendX := hintX + hintW + 1

	return lipgloss.NewLayer("",
		lipgloss.NewLayer(methodBtn).ID("method-btn").Z(1),
		lipgloss.NewLayer(urlStr).ID("url-input").X(urlX),
		lipgloss.NewLayer(histBtn).ID("history-btn").X(histX).Z(1),
		lipgloss.NewLayer(hint).ID("url-hint").X(hintX),
		lipgloss.NewLayer(sendBtn).ID("send-btn").X(sendX).Z(1),
	).ID("urlbar")
}

// DropdownView returns the rendered dropdown to be overlaid by the parent.
func (m Model) DropdownView() string {
	return m.dropdown.Render(methods, func(item string, _ int, highlighted bool) lipgloss.Style {
		style := styles.MethodStyle(item).Padding(0, 1).Width(12)
		if highlighted {
			style = style.Reverse(true)
		}
		return style
	})
}

// ClickDropdown handles a mouse click on the dropdown overlay.
// row is relative to the dropdown top, col is the absolute X coordinate.
// Returns true if a method was selected.
func (m *Model) ClickDropdown(row, col int) bool {
	const dropdownW = 16
	idx, ok := m.dropdown.HandleClick(row, col, len(methods), dropdownW)
	if ok {
		m.methodIdx = idx
		m.urlInput.SetWidth(m.inputWidth())
	}
	return ok
}

func (m *Model) SetWidth(w int) {
	m.width = w
	m.urlInput.SetWidth(m.inputWidth())
}
