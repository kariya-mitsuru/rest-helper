// SPDX-License-Identifier: MIT

package sidebar

import (
	"fmt"
	"net/url"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/storage"
	"rest-helper/internal/ui/styles"
)

// HistorySelectedMsg is sent when a history item is selected.
type HistorySelectedMsg struct {
	Entry storage.HistoryEntry
}

// HistoryUpdatedMsg triggers a refresh of the history list.
type HistoryUpdatedMsg struct{}

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
	selected map[int64]bool // entry ID -> selected
	cursor   int
	offset   int
	focused  bool
	width    int
	height   int

	confirm      confirmAction
	confirmLabel string
}

func New() Model {
	return Model{
		selected: make(map[int64]bool),
	}
}

func (m *Model) LoadHistory() tea.Cmd {
	return func() tea.Msg {
		entries, err := storage.ListHistory(200)
		if err != nil {
			return nil
		}
		return historyLoadedMsg{entries: entries}
	}
}

type historyLoadedMsg struct {
	entries []storage.HistoryEntry
}

func (m *Model) Focus()             { m.focused = true }
func (m *Model) Blur()              { m.focused = false }
func (m *Model) SetSize(w, h int)   { m.width = w; m.height = h }
func (m Model) EntryCount() int     { return len(m.entries) }
func (m Model) InConfirmMode() bool { return m.confirm != confirmNone }
func (m Model) HasSelection() bool  { return m.selectedCount() > 0 }
func (m Model) HasEntries() bool    { return len(m.entries) > 0 }

func (m Model) selectedCount() int {
	count := 0
	for _, e := range m.entries {
		if m.selected[e.ID] {
			count++
		}
	}
	return count
}

// --- Navigation ---

func (m *Model) CursorUp() {
	if m.cursor > 0 {
		m.cursor--
		m.ensureVisible()
	}
}

func (m *Model) CursorDown() {
	if m.cursor < len(m.entries)-1 {
		m.cursor++
		m.ensureVisible()
	}
}

func (m *Model) SelectCurrent() tea.Cmd {
	if m.cursor >= len(m.entries) {
		return nil
	}
	entry := m.entries[m.cursor]
	return func() tea.Msg {
		return HistorySelectedMsg{Entry: entry}
	}
}

// --- Selection ---

func (m *Model) ToggleSelection() {
	if m.cursor >= len(m.entries) {
		return
	}
	id := m.entries[m.cursor].ID
	if m.selected[id] {
		delete(m.selected, id)
	} else {
		m.selected[id] = true
	}
	// Move cursor down for quick multi-select
	if m.cursor < len(m.entries)-1 {
		m.cursor++
		m.ensureVisible()
	}
}

func (m *Model) ClearSelection() {
	m.selected = make(map[int64]bool)
}

// --- Delete actions ---

func (m *Model) DeleteSingleOrSelected() tea.Cmd {
	if len(m.entries) == 0 {
		return nil
	}
	sel := m.selectedCount()
	if sel > 0 {
		m.confirm = confirmDeleteSelected
		m.confirmLabel = fmt.Sprintf("Delete %d selected? (y/n)", sel)
		return nil
	}
	// Single delete (no confirm)
	entry := m.entries[m.cursor]
	_ = storage.DeleteHistory(entry.ID) // best-effort, list refreshes next
	return m.LoadHistory()
}

func (m *Model) RequestDeleteOlder() {
	if m.cursor < len(m.entries) {
		m.confirm = confirmDeleteOlder
		m.confirmLabel = "Delete older entries? (y/n)"
	}
}

func (m *Model) RequestClearAll() {
	if len(m.entries) > 0 {
		m.confirm = confirmClearAll
		m.confirmLabel = fmt.Sprintf("Clear all %d entries? (y/n)", len(m.entries))
	}
}

func (m *Model) RequestDeleteDuplicates() {
	if len(m.entries) > 0 {
		m.confirm = confirmDeleteDuplicates
		m.confirmLabel = "Remove duplicate entries? (y/n)"
	}
}

func (m *Model) ConfirmYes() tea.Cmd {
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
		m.ClearSelection()

	case confirmDeleteOlder:
		if m.cursor < len(m.entries) {
			storage.DeleteHistoryOlderThan(m.entries[m.cursor].ID)
		}

	case confirmClearAll:
		storage.ClearHistory()
		m.ClearSelection()

	case confirmDeleteDuplicates:
		storage.DeleteHistoryDuplicates()
		m.ClearSelection()
	}

	return m.LoadHistory()
}

func (m *Model) ConfirmCancel() {
	m.confirm = confirmNone
	m.confirmLabel = ""
}

// headerLineCount returns the number of lines the header occupies (1 or 2).
func (m Model) headerLineCount() int {
	contentW := m.width - 2
	if contentW < 10 {
		contentW = 10
	}
	title := lipgloss.NewStyle().Bold(true).Render("History")
	shortcut := styles.MutedStyle.Render(" [Alt+H]")
	count := styles.MutedStyle.Render(fmt.Sprintf(" (%d)", len(m.entries)))
	selStr := ""
	if sel := m.selectedCount(); sel > 0 {
		selStr = lipgloss.NewStyle().Foreground(styles.PrimaryColor).Render(
			fmt.Sprintf(" [%d sel]", sel),
		)
	}
	if lipgloss.Width(title+shortcut+count+selStr) <= contentW {
		return 1
	}
	return 2
}

// ClickAt handles a mouse click at the given row (relative to the sidebar top).
// It accounts for the border and header to determine which entry was clicked.
func (m *Model) ClickAt(row int) tea.Cmd {
	// row 0 = top border, then header lines, then entries
	entryStart := 1 + m.headerLineCount()
	idx := row - entryStart + m.offset
	if idx < 0 || idx >= len(m.entries) {
		return nil
	}
	m.cursor = idx
	m.ensureVisible()
	return m.SelectCurrent()
}

// --- Update (non-key messages only) ---

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case historyLoadedMsg:
		m.entries = msg.entries
		if m.cursor >= len(m.entries) {
			m.cursor = max(0, len(m.entries)-1)
		}
		// Prune selection to valid IDs
		valid := make(map[int64]bool)
		for _, e := range m.entries {
			if m.selected[e.ID] {
				valid[e.ID] = true
			}
		}
		m.selected = valid
		return m, nil

	case HistoryUpdatedMsg:
		return m, m.LoadHistory()
	}

	return m, nil
}

// --- View ---

func (m *Model) ensureVisible() {
	visibleH := m.height - 4
	if visibleH < 1 {
		visibleH = 1
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleH {
		m.offset = m.cursor - visibleH + 1
	}
}

func (m Model) View() string {
	borderStyle := styles.NormalBorder
	if m.focused {
		borderStyle = styles.FocusedBorder
	}

	title := lipgloss.NewStyle().Bold(true).Render("History")
	shortcut := styles.MutedStyle.Render(" [Alt+H]")
	count := styles.MutedStyle.Render(fmt.Sprintf(" (%d)", len(m.entries)))

	selStr := ""
	sel := m.selectedCount()
	if sel > 0 {
		selStr = lipgloss.NewStyle().Foreground(styles.PrimaryColor).Render(
			fmt.Sprintf(" [%d sel]", sel),
		)
	}

	contentW := m.width - 2
	if contentW < 10 {
		contentW = 10
	}

	// If everything fits on one line, keep it together; otherwise wrap count + sel to next line.
	firstLine := title + shortcut + count + selStr
	var header string
	if m.headerLineCount() == 1 {
		header = firstLine + "\n"
	} else {
		header = title + shortcut + "\n" + count + selStr + "\n"
	}

	// Reserve space for confirm bar
	confirmH := 0
	if m.confirm != confirmNone {
		confirmH = 3
	}

	visibleH := m.height - 4 - m.headerLineCount() - confirmH
	if visibleH < 1 {
		visibleH = 1
	}

	var b strings.Builder
	b.WriteString(header)

	if len(m.entries) == 0 {
		b.WriteString(styles.MutedStyle.Render("  No history yet"))
	} else {
		end := m.offset + visibleH
		if end > len(m.entries) {
			end = len(m.entries)
		}

		for i := m.offset; i < end; i++ {
			entry := m.entries[i]

			isSelected := m.selected[entry.ID]
			isCursor := m.focused && i == m.cursor

			statusStr := ""
			statusW := 0
			if entry.StatusCode > 0 {
				raw := fmt.Sprintf(" %d", entry.StatusCode)
				statusW = len(raw)
				statusStr = raw
			}

			// method(7) + space(1) + status
			pathW := contentW - 8 - statusW
			path := shortenURL(entry.URL, pathW)

			if isSelected || isCursor {
				// Build plain text, apply a single style for the whole line
				plain := fmt.Sprintf("%-7s %s%s", entry.Method, path, statusStr)
				lineStyle := lipgloss.NewStyle()
				if isSelected {
					lineStyle = lineStyle.Reverse(true)
				}
				if isCursor {
					lineStyle = lineStyle.Underline(true)
				}
				b.WriteString(lineStyle.Render(plain))
			} else {
				// Normal: per-component coloring
				method := styles.MethodStyle(entry.Method).Render(
					fmt.Sprintf("%-7s", entry.Method),
				)
				if statusStr != "" {
					statusStr = styles.StatusCodeStyle(entry.StatusCode).Render(statusStr)
				}
				b.WriteString(fmt.Sprintf("%s %s%s", method, path, statusStr))
			}

			if i < end-1 {
				b.WriteString("\n")
			}
		}
	}

	// Confirmation bar at bottom
	if m.confirm != confirmNone {
		b.WriteString("\n\n")
		confirmStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBF24")).
			Bold(true)
		b.WriteString(confirmStyle.Render(m.confirmLabel))
	}

	return borderStyle.
		Width(m.width).
		Height(m.height).
		Render(b.String())
}

func shortenURL(rawURL string, maxLen int) string {
	if maxLen < 2 {
		return "…"
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" {
		if len(rawURL) > maxLen {
			return rawURL[:maxLen-1] + "…"
		}
		return rawURL
	}

	path := u.Path
	if len(path) > maxLen {
		path = path[:maxLen-1] + "…"
	}
	return path
}
