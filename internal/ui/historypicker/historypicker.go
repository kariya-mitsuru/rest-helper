// SPDX-License-Identifier: MIT

package historypicker

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/storage"
	"rest-helper/internal/ui/styles"
)

// HistorySelectedMsg is sent when a history entry is chosen.
type HistorySelectedMsg struct {
	Entry storage.HistoryEntry
}

// HistoryClosedMsg is sent when the picker is dismissed.
type HistoryClosedMsg struct{}

// HistoryChangedMsg is sent when entries have been modified (deleted etc).
type HistoryChangedMsg struct{}

type confirmAction int

const (
	confirmNone confirmAction = iota
	confirmDeleteSelected
	confirmDeleteOlder
	confirmClearAll
	confirmDeleteDuplicates
)

type Model struct {
	entries  []storage.HistoryEntry
	filtered []int // indices into entries
	selected map[int64]bool
	filter   textinput.Model
	cursor   int
	scroll   int
	width    int
	height   int
	innerW   int // content width locked at open time
	fixedVis int // visible rows locked at open time

	confirm      confirmAction
	confirmLabel string
}

// New creates a HistoryPicker pre-loaded with the given entries.
func New(entries []storage.HistoryEntry, width, height int) Model {
	ti := textinput.New()
	ti.Placeholder = "Filter history..."
	ti.Prompt = "/ "
	ti.SetWidth(width - 7)
	ti.Focus()

	m := Model{
		entries:  entries,
		selected: make(map[int64]bool),
		filter:   ti,
		width:    width,
		height:   height,
	}

	// Compute content width from entries
	helpW := len("↑↓ nav  enter select  space mark  d del  esc close")
	cw := helpW
	for _, e := range entries {
		// method(7) + space(1) + url + status(max 4)
		w := 8 + len(e.URL) + 4
		if w > cw {
			cw = w
		}
	}
	// Clamp to screen: innerW + border(2) + padding(2) <= width
	maxW := width - 4
	if maxW < 30 {
		maxW = 30
	}
	if cw > maxW {
		cw = maxW
	}
	m.innerW = cw

	ti.SetWidth(cw - 3) // minus prompt "/ " (2) and cursor (1)

	m.applyFilter()
	m.fixedVis = m.visibleRows()
	return m
}

func (m *Model) applyFilter() {
	query := strings.ToLower(m.filter.Value())
	m.filtered = nil
	for i, entry := range m.entries {
		if query == "" ||
			strings.Contains(strings.ToLower(entry.Method), query) ||
			strings.Contains(strings.ToLower(entry.URL), query) {
			m.filtered = append(m.filtered, i)
		}
	}
	m.cursor = 0
	m.scroll = 0
}

func (m Model) visibleRows() int {
	if m.fixedVis > 0 {
		return m.fixedVis
	}
	// Initial calculation: shrink to fit actual content
	maxRows := m.height - 8 // Y offset(1) + border(2) + title(1) + filter(1) + blank(1) + help(1) + blank(1)
	if maxRows < 3 {
		maxRows = 3
	}
	n := len(m.filtered)
	if n < 1 {
		n = 1
	}
	if n < maxRows {
		return n
	}
	return maxRows
}

func (m Model) selectedCount() int {
	count := 0
	for _, e := range m.entries {
		if m.selected[e.ID] {
			count++
		}
	}
	return count
}

func (m Model) hasSelection() bool { return m.selectedCount() > 0 }

// currentEntry returns the entry under the cursor, if any.
func (m Model) currentEntry() (storage.HistoryEntry, bool) {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return storage.HistoryEntry{}, false
	}
	return m.entries[m.filtered[m.cursor]], true
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case historyReloadedMsg:
		m.entries = msg.entries
		// Prune selection
		valid := make(map[int64]bool)
		for _, e := range m.entries {
			if m.selected[e.ID] {
				valid[e.ID] = true
			}
		}
		m.selected = valid
		m.applyFilter()
		return m, func() tea.Msg { return HistoryChangedMsg{} }

	case tea.MouseWheelMsg:
		if msg.Button == tea.MouseWheelUp {
			return m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		}
		return m.Update(tea.KeyPressMsg{Code: tea.KeyDown})

	case tea.KeyPressMsg:
		// Confirmation mode
		if m.confirm != confirmNone {
			switch msg.String() {
			case "y", "Y":
				return m.confirmYes()
			default:
				m.confirm = confirmNone
				m.confirmLabel = ""
			}
			return m, nil
		}

		switch msg.String() {
		case "esc":
			if m.hasSelection() {
				m.selected = make(map[int64]bool)
				return m, nil
			}
			return m, func() tea.Msg { return HistoryClosedMsg{} }

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
			if entry, ok := m.currentEntry(); ok {
				return m, func() tea.Msg {
					return HistorySelectedMsg{Entry: entry}
				}
			}
			return m, nil

		case "space":
			if entry, ok := m.currentEntry(); ok {
				if m.selected[entry.ID] {
					delete(m.selected, entry.ID)
				} else {
					m.selected[entry.ID] = true
				}
				// Move cursor down for quick multi-select
				if m.cursor < len(m.filtered)-1 {
					m.cursor++
					vis := m.visibleRows()
					if m.cursor >= m.scroll+vis {
						m.scroll = m.cursor - vis + 1
					}
				}
			}
			return m, nil

		case "d":
			return m.deleteSingleOrSelected()

		case "D":
			if _, ok := m.currentEntry(); ok {
				m.confirm = confirmDeleteOlder
				m.confirmLabel = "Delete older entries? (y/n)"
			}
			return m, nil

		case "ctrl+d":
			if len(m.entries) > 0 {
				m.confirm = confirmClearAll
				m.confirmLabel = fmt.Sprintf("Clear all %d entries? (y/n)", len(m.entries))
			}
			return m, nil

		case "ctrl+x":
			if len(m.entries) > 0 {
				m.confirm = confirmDeleteDuplicates
				m.confirmLabel = "Remove duplicate entries? (y/n)"
			}
			return m, nil
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

func (m Model) deleteSingleOrSelected() (Model, tea.Cmd) {
	if len(m.entries) == 0 {
		return m, nil
	}
	sel := m.selectedCount()
	if sel > 0 {
		m.confirm = confirmDeleteSelected
		m.confirmLabel = fmt.Sprintf("Delete %d selected? (y/n)", sel)
		return m, nil
	}
	// Single delete (no confirm)
	if entry, ok := m.currentEntry(); ok {
		_ = storage.DeleteHistory(entry.ID)
		return m, m.reloadEntries()
	}
	return m, nil
}

func (m Model) confirmYes() (Model, tea.Cmd) {
	action := m.confirm
	m.confirm = confirmNone
	m.confirmLabel = ""

	switch action {
	case confirmDeleteSelected:
		var ids []int64
		for _, e := range m.entries {
			if m.selected[e.ID] {
				ids = append(ids, e.ID)
			}
		}
		storage.DeleteHistoryBatch(ids)
		m.selected = make(map[int64]bool)

	case confirmDeleteOlder:
		if entry, ok := m.currentEntry(); ok {
			storage.DeleteHistoryOlderThan(entry.ID)
		}

	case confirmClearAll:
		storage.ClearHistory()
		m.selected = make(map[int64]bool)

	case confirmDeleteDuplicates:
		storage.DeleteHistoryDuplicates()
		m.selected = make(map[int64]bool)
	}

	return m, m.reloadEntries()
}

func (m Model) reloadEntries() tea.Cmd {
	return func() tea.Msg {
		entries, err := storage.ListHistory(200)
		if err != nil {
			return HistoryChangedMsg{}
		}
		return historyReloadedMsg{entries: entries}
	}
}

type historyReloadedMsg struct {
	entries []storage.HistoryEntry
}


func (m Model) View() string {
	innerW := m.innerW

	title := lipgloss.NewStyle().Bold(true).
		Foreground(styles.PrimaryColor).
		Render("History")

	countInfo := fmt.Sprintf(" (%d)", len(m.entries))
	if len(m.filtered) != len(m.entries) {
		countInfo = fmt.Sprintf(" (%d/%d)", len(m.filtered), len(m.entries))
	}
	if sel := m.selectedCount(); sel > 0 {
		countInfo += lipgloss.NewStyle().Foreground(styles.PrimaryColor).
			Render(fmt.Sprintf(" [%d sel]", sel))
	}
	title += styles.MutedStyle.Render(countInfo)

	m.filter.SetWidth(innerW - 3)
	filterLine := m.filter.View()

	vis := m.visibleRows()
	end := m.scroll + vis
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	var lines []string
	for i := m.scroll; i < end; i++ {
		idx := m.filtered[i]
		entry := m.entries[idx]
		line := m.renderEntry(entry, i == m.cursor, innerW)
		lines = append(lines, line)
	}

	if len(m.filtered) == 0 {
		lines = append(lines, styles.MutedStyle.Render("  No matching entries"))
	}

	for len(lines) < vis {
		lines = append(lines, "")
	}

	listContent := strings.Join(lines, "\n")

	help := styles.MutedStyle.Render("↑↓ nav  enter select  space mark  d del  esc close")

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(filterLine)
	b.WriteString("\n\n")
	b.WriteString(listContent)

	if m.confirm != confirmNone {
		confirmStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBF24")).
			Bold(true)
		b.WriteString("\n\n")
		b.WriteString(confirmStyle.Render(m.confirmLabel))
	}

	b.WriteString("\n\n")
	b.WriteString(help)

	overlayStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.PrimaryColor).
		Padding(0, 1)

	return overlayStyle.Render(b.String())
}

func (m Model) renderEntry(entry storage.HistoryEntry, isCursor bool, width int) string {
	isSelected := m.selected[entry.ID]

	statusStr := ""
	statusW := 0
	if entry.StatusCode > 0 {
		raw := fmt.Sprintf(" %d", entry.StatusCode)
		statusW = len(raw)
		statusStr = raw
	}

	// method(7) + space(1) + path + status
	pathW := width - 8 - statusW
	path := truncateURL(entry.URL, pathW)

	if isSelected || isCursor {
		plain := fmt.Sprintf("%-7s %s%s", entry.Method, path, statusStr)
		lineStyle := lipgloss.NewStyle()
		if isSelected {
			lineStyle = lineStyle.Reverse(true)
		}
		if isCursor {
			lineStyle = lineStyle.Underline(true)
		}
		return lineStyle.Render(plain)
	}

	method := styles.MethodStyle(entry.Method).Render(
		fmt.Sprintf("%-7s", entry.Method),
	)
	if statusStr != "" {
		statusStr = styles.StatusCodeStyle(entry.StatusCode).Render(statusStr)
	}
	return fmt.Sprintf("%s %s%s", method, path, statusStr)
}

func truncateURL(rawURL string, maxLen int) string {
	if maxLen < 2 {
		return "…"
	}
	if len(rawURL) > maxLen {
		return rawURL[:maxLen-1] + "…"
	}
	return rawURL
}
