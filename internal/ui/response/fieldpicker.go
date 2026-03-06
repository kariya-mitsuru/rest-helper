// SPDX-License-Identifier: MIT

package response

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"rest-helper/internal/clipboard"
	"rest-helper/internal/ui/styles"
)

// FieldCopiedMsg is sent when a field value has been copied to clipboard.
type FieldCopiedMsg struct {
	Path  string
	Error error
}

// FieldPickerClosedMsg is sent when the field picker is dismissed.
type FieldPickerClosedMsg struct{}

type pathValue struct {
	path    string
	value   string // raw JSON value for copying
	display string // truncated display string
}

// FieldPickerModel provides a filterable list of JSON fields to copy.
type FieldPickerModel struct {
	items    []pathValue
	filtered []int // indices into items
	filter   textinput.Model
	cursor   int
	scroll   int
	width    int
	height   int
	contentW int    // pre-computed content width from all items
	fixedVis int    // visible rows locked at open time
	body     string // raw response body for "entire body" copy
}

// NewFieldPicker creates a field picker for the given JSON body.
func NewFieldPicker(body string, width, height int) FieldPickerModel {
	ti := textinput.New()
	ti.Placeholder = "Filter fields..."
	ti.Prompt = "/ "
	ti.SetWidth(width - 7) // border(2) + padding(2) + prompt(2) + cursor(1)
	ti.Focus()

	m := FieldPickerModel{
		filter: ti,
		width:  width,
		height: height,
		body:   body,
	}

	// First item: entire body
	m.items = append(m.items, pathValue{
		path:    ".",
		value:   body,
		display: "(entire body)",
	})

	// Flatten JSON
	var data any
	if err := json.Unmarshal([]byte(body), &data); err == nil {
		flattenJSON("", data, &m.items)
	}

	// Compute content width from all items (fixed, not affected by filter)
	cw := len("↑↓ select  enter copy  esc close")
	if tw := lipgloss.Width(lipgloss.NewStyle().Bold(true).Foreground(styles.PrimaryColor).Render("Copy Field")); tw > cw {
		cw = tw
	}
	for _, item := range m.items {
		pw := len(item.path) + 3 + len(item.display) // " = "
		if pw > cw {
			cw = pw
		}
	}
	m.contentW = cw

	m.applyFilter()
	m.fixedVis = m.visibleRows()
	return m
}

func flattenJSON(prefix string, v any, out *[]pathValue) {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			path := prefix + "." + k
			flattenJSON(path, val[k], out)
		}
	case []any:
		for i, child := range val {
			path := fmt.Sprintf("%s[%d]", prefix, i)
			flattenJSON(path, child, out)
		}
	default:
		raw, _ := json.Marshal(val)
		rawStr := string(raw)
		// For strings, store unquoted value for copying
		copyVal := rawStr
		if s, ok := val.(string); ok {
			copyVal = s
		}
		*out = append(*out, pathValue{
			path:    prefix,
			value:   copyVal,
			display: rawStr,
		})
	}
}

func (m *FieldPickerModel) applyFilter() {
	query := strings.ToLower(m.filter.Value())
	m.filtered = nil
	for i, item := range m.items {
		if query == "" ||
			strings.Contains(strings.ToLower(item.path), query) ||
			strings.Contains(strings.ToLower(item.display), query) {
			m.filtered = append(m.filtered, i)
		}
	}
	m.cursor = 0
	m.scroll = 0
}

func (m FieldPickerModel) visibleRows() int {
	if m.fixedVis > 0 {
		return m.fixedVis
	}
	// height minus: border(2) + title(1) + filter(1) + blank(1) + help(1) + blank(1)
	maxRows := m.height - 7
	if maxRows < 3 {
		maxRows = 3
	}
	n := len(m.items)
	if n < 1 {
		n = 1
	}
	if n < maxRows {
		return n
	}
	return maxRows
}

func (m FieldPickerModel) Update(msg tea.Msg) (FieldPickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		if msg.Button == tea.MouseWheelUp {
			return m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		}
		return m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return FieldPickerClosedMsg{} }
		case "up":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.scroll {
					m.scroll = m.cursor
				}
			}
			return m, nil
		case "down":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				vis := m.visibleRows()
				if m.cursor >= m.scroll+vis {
					m.scroll = m.cursor - vis + 1
				}
			}
			return m, nil
		case "pgup":
			vis := m.visibleRows()
			m.cursor -= vis
			if m.cursor < 0 {
				m.cursor = 0
			}
			if m.cursor < m.scroll {
				m.scroll = m.cursor
			}
			return m, nil
		case "pgdown":
			vis := m.visibleRows()
			m.cursor += vis
			maxIdx := len(m.filtered) - 1
			if maxIdx < 0 {
				maxIdx = 0
			}
			if m.cursor > maxIdx {
				m.cursor = maxIdx
			}
			if m.cursor >= m.scroll+vis {
				m.scroll = m.cursor - vis + 1
			}
			return m, nil
		case "home":
			m.cursor = 0
			m.scroll = 0
			return m, nil
		case "end":
			m.cursor = len(m.filtered) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
			vis := m.visibleRows()
			if m.cursor >= m.scroll+vis {
				m.scroll = m.cursor - vis + 1
			}
			return m, nil
		case "enter":
			if len(m.filtered) == 0 {
				return m, nil
			}
			item := m.items[m.filtered[m.cursor]]
			err := clipboard.Write(item.value)
			return m, func() tea.Msg {
				return FieldCopiedMsg{Path: item.path, Error: err}
			}
		}
	}

	// Forward to filter input
	prev := m.filter.Value()
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	if m.filter.Value() != prev {
		m.applyFilter()
	}
	return m, cmd
}

func (m FieldPickerModel) View() string {
	// Max width from screen
	maxW := m.width - 4 // border(2) + padding(2)
	if maxW < 30 {
		maxW = 30
	}

	title := lipgloss.NewStyle().Bold(true).
		Foreground(styles.PrimaryColor).
		Render("Copy Field")

	// Use pre-computed content width (based on all items, not filtered)
	innerW := m.contentW
	if innerW > maxW {
		innerW = maxW
	}

	// Update filter width to match content width (prompt "/ " (2) + cursor (1))
	m.filter.SetWidth(innerW - 3)
	filterLine := m.filter.View()

	vis := m.visibleRows()
	end := m.scroll + vis
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1F2937")).
		Background(styles.PrimaryColor).
		Bold(true)
	pathStyle := lipgloss.NewStyle().Foreground(styles.SecondaryColor)
	valStyle := lipgloss.NewStyle().Foreground(styles.TextColor)

	var lines []string
	for i := m.scroll; i < end; i++ {
		idx := m.filtered[i]
		item := m.items[idx]

		plain := item.path + " = " + item.display
		if ansi.StringWidth(plain) > innerW {
			plain = ansi.Truncate(plain, innerW-1, "…")
		}

		if i == m.cursor {
			line := selectedStyle.Render(fmt.Sprintf("%-*s", innerW, plain))
			lines = append(lines, line)
		} else {
			// Split back into path and value parts for coloring
			sep := " = "
			path := item.path
			val := plain[len(path)+len(sep):]
			line := pathStyle.Render(path) + valStyle.Render(sep+val)
			lines = append(lines, line)
		}
	}

	if len(m.filtered) == 0 {
		lines = append(lines, styles.MutedStyle.Render("  No matching fields"))
	}

	// Pad to fixed height so overlay doesn't shrink when filtering
	for len(lines) < vis {
		lines = append(lines, "")
	}

	listContent := strings.Join(lines, "\n")

	countInfo := ""
	if len(m.filtered) != len(m.items) {
		countInfo = fmt.Sprintf(" (%d/%d)", len(m.filtered), len(m.items))
	}

	help := styles.MutedStyle.Render("↑↓ select  enter copy  esc close" + countInfo)

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(filterLine)
	b.WriteString("\n\n")
	b.WriteString(listContent)
	b.WriteString("\n\n")
	b.WriteString(help)

	overlayStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.PrimaryColor).
		Padding(0, 1)

	return overlayStyle.Render(b.String())
}
