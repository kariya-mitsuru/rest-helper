// SPDX-License-Identifier: MIT

package request

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"rest-helper/internal/ui/styles"
)

type headerPair struct {
	key   textinput.Model
	value textinput.Model
}

type HeadersModel struct {
	pairs        []headerPair
	cursor       int
	colFocus     int // 0=key, 1=value
	focused      bool
	width        int
	height       int // available rows for pair display
	scrollOffset int // first visible pair index
}

func NewHeaders() HeadersModel {
	m := HeadersModel{}
	m.addEmptyPair()
	return m
}

// columnWidths calculates key and value column widths from the panel width.
func columnWidths(w int) (keyW, valW int) {
	// content width: panel width - border(2) - prefix(2) - margin(1)
	contentW := w - 5
	if contentW < 30 {
		contentW = 30
	}
	keyW = contentW/3 - 1
	if keyW > 30 {
		keyW = 30
	}
	valW = contentW - keyW - 2
	return
}

func (m *HeadersModel) addEmptyPair() {
	ki := textinput.New()
	ki.Placeholder = "Header name"
	ki.CharLimit = 256
	ki.Prompt = ""

	vi := textinput.New()
	vi.Placeholder = "Value"
	vi.CharLimit = 1024
	vi.Prompt = ""

	keyW, valW := columnWidths(m.width)
	// -1 so textinput scrolls before cursor overflows the display column
	ki.SetWidth(keyW - 1)
	vi.SetWidth(valW - 1)

	m.pairs = append(m.pairs, headerPair{key: ki, value: vi})
}

func (m HeadersModel) Headers() map[string]string {
	headers := make(map[string]string)
	for _, p := range m.pairs {
		k := strings.TrimSpace(p.key.Value())
		v := strings.TrimSpace(p.value.Value())
		if k != "" {
			headers[k] = v
		}
	}
	return headers
}

func (m *HeadersModel) SetHeaders(headers map[string]string) {
	m.pairs = nil
	for k, v := range headers {
		ki := textinput.New()
		ki.Placeholder = "Header name"
		ki.CharLimit = 256
		ki.Prompt = ""
		ki.SetValue(k)

		vi := textinput.New()
		vi.Placeholder = "Value"
		vi.CharLimit = 1024
		vi.Prompt = ""
		vi.SetValue(v)

		m.pairs = append(m.pairs, headerPair{key: ki, value: vi})
	}
	m.addEmptyPair()
	// Apply widths to all pairs
	if m.width > 0 {
		m.SetSize(m.width, m.height)
	}
	m.cursor = 0
	m.colFocus = 0
	m.scrollOffset = 0
	// Reset cursor positions to start so text is visible from the beginning
	for i := range m.pairs {
		m.pairs[i].key.CursorStart()
		m.pairs[i].value.CursorStart()
	}
}

func (m *HeadersModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	keyW, valW := columnWidths(w)
	// -1 so textinput scrolls before cursor overflows the display column
	for i := range m.pairs {
		m.pairs[i].key.SetWidth(keyW - 1)
		m.pairs[i].value.SetWidth(valW - 1)
	}
}

func (m *HeadersModel) Focus() {
	m.focused = true
	m.updateInputFocus()
}

func (m *HeadersModel) Blur() {
	m.focused = false
	m.blurAllInputs()
}

func (m *HeadersModel) blurAllInputs() {
	for i := range m.pairs {
		m.pairs[i].key.Blur()
		m.pairs[i].value.Blur()
	}
}

func (m *HeadersModel) updateInputFocus() {
	m.blurAllInputs()
	if m.cursor < len(m.pairs) {
		if m.colFocus == 0 {
			m.pairs[m.cursor].key.Focus()
		} else {
			m.pairs[m.cursor].value.Focus()
		}
	}
	m.ensureCursorVisible()
}

// ensureCursorVisible adjusts scrollOffset so the cursor row is visible.
func (m *HeadersModel) ensureCursorVisible() {
	visible := m.visibleRows()
	if visible <= 0 {
		return
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}
}

// visibleRows returns how many pair rows fit in the display area.
func (m HeadersModel) visibleRows() int {
	// height minus: column header(1), help line when focused(1)
	rows := m.height - 1
	if m.focused {
		rows--
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m HeadersModel) Update(msg tea.Msg) (HeadersModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up":
			if m.cursor > 0 {
				m.cursor--
				m.updateInputFocus()
			}
			return m, nil
		case "down":
			if m.cursor < len(m.pairs)-1 {
				m.cursor++
				m.updateInputFocus()
			}
			return m, nil
		case "right":
			// Switch to Value only when cursor is at end of Key text
			if m.colFocus == 0 {
				p := m.pairs[m.cursor]
				if p.key.Position() >= len(p.key.Value()) {
					m.colFocus = 1
					m.updateInputFocus()
					m.pairs[m.cursor].value.CursorStart()
					return m, nil
				}
			}
		case "left":
			// Switch to Key only when cursor is at start of Value text
			if m.colFocus == 1 {
				p := m.pairs[m.cursor]
				if p.value.Position() == 0 {
					m.colFocus = 0
					m.updateInputFocus()
					m.pairs[m.cursor].key.CursorEnd()
					return m, nil
				}
			}
		case "ctrl+d":
			if len(m.pairs) > 1 {
				m.pairs = append(m.pairs[:m.cursor], m.pairs[m.cursor+1:]...)
				if m.cursor >= len(m.pairs) {
					m.cursor = len(m.pairs) - 1
				}
				m.updateInputFocus()
			}
			return m, nil
		case "enter":
			// Skip if current row is empty
			cur := m.pairs[m.cursor]
			if cur.key.Value() == "" && cur.value.Value() == "" {
				return m, nil
			}
			// Auto-add new row when pressing enter on last row
			if m.cursor == len(m.pairs)-1 {
				m.addEmptyPair()
			}
			m.cursor++
			m.colFocus = 0
			m.updateInputFocus()
			return m, nil
		}
	}

	// Forward to active input
	var cmd tea.Cmd
	if m.cursor < len(m.pairs) {
		if m.colFocus == 0 {
			m.pairs[m.cursor].key, cmd = m.pairs[m.cursor].key.Update(msg)
		} else {
			m.pairs[m.cursor].value, cmd = m.pairs[m.cursor].value.Update(msg)
		}
	}

	// Auto-add empty row if last row has content
	last := m.pairs[len(m.pairs)-1]
	if last.key.Value() != "" || last.value.Value() != "" {
		m.addEmptyPair()
	}

	return m, cmd
}

func (m HeadersModel) View() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().
		Foreground(styles.MutedColor).
		Bold(true)

	keyColW, valColW := columnWidths(m.width)
	b.WriteString(headerStyle.Render(fmt.Sprintf("  %-*s  %s", keyColW, "Key", "Value")))
	b.WriteString("\n")

	visible := m.visibleRows()
	end := m.scrollOffset + visible
	if end > len(m.pairs) {
		end = len(m.pairs)
	}

	for i := m.scrollOffset; i < end; i++ {
		p := m.pairs[i]
		prefix := "  "
		if m.focused && i == m.cursor {
			prefix = lipgloss.NewStyle().Foreground(styles.PrimaryColor).Render("> ")
		}
		keyView := fixedWidth(p.key.View(), keyColW)
		valView := fixedWidth(p.value.View(), valColW)
		b.WriteString(fmt.Sprintf("%s%s  %s", prefix, keyView, valView))
		b.WriteString("\n")
	}

	if m.focused {
		// Pad blank lines so help stays at the bottom
		for i := end - m.scrollOffset; i < visible; i++ {
			b.WriteString("\n")
		}
		help := "  enter: new row | ctrl+d: delete"
		if m.scrollOffset > 0 || end < len(m.pairs) {
			help += fmt.Sprintf(" | %d/%d", m.cursor+1, len(m.pairs))
		}
		b.WriteString(styles.MutedStyle.Render(help))
	}

	return b.String()
}

// fixedWidth truncates or pads s to exactly w visible characters.
// ANSI escape sequences are preserved correctly.
func fixedWidth(s string, w int) string {
	sw := ansi.StringWidth(s)
	if sw > w {
		return ansi.Truncate(s, w, "")
	}
	if sw < w {
		return s + strings.Repeat(" ", w-sw)
	}
	return s
}
