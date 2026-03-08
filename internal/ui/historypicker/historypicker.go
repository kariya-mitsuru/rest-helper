// SPDX-License-Identifier: MIT

package historypicker

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/storage"
	"rest-helper/internal/ui/styles"
	"rest-helper/internal/ui/widget"
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

// historyContent implements widget.PickerContent for history entry selection.
type historyContent struct {
	selected     map[int64]bool
	confirm      confirmAction
	confirmLabel string
}

func (c *historyContent) clearSelection() {
	c.selected = make(map[int64]bool)
}

func (c *historyContent) MatchFilter(item storage.HistoryEntry, query string) bool {
	return strings.Contains(strings.ToLower(item.Method), query) ||
		strings.Contains(strings.ToLower(item.URL), query)
}

func (c *historyContent) Title(filtered, total int) string {
	title := styles.BoldStyle.Foreground(styles.PrimaryColor).
		Render("History")

	countInfo := fmt.Sprintf(" (%d)", total)
	if filtered != total {
		countInfo = fmt.Sprintf(" (%d/%d)", filtered, total)
	}
	if sel := len(c.selected); sel > 0 {
		countInfo += lipgloss.NewStyle().Foreground(styles.PrimaryColor).
			Render(fmt.Sprintf(" [%d sel]", sel))
	}
	return title + styles.MutedStyle.Render(countInfo)
}

func (c *historyContent) Footer(_, _ int) string {
	if c.confirm != confirmNone {
		return lipgloss.NewStyle().
			Foreground(styles.WarningColor).
			Bold(true).
			Render(c.confirmLabel)
	}
	return styles.MutedStyle.Render("↑↓ nav │ enter select │ space mark │ d del │ ? help │ esc close")
}

func (c *historyContent) HelpContent() [][2]string {
	return [][2]string{
		{"/ (type)", "Filter entries"},
		{"↑ / ↓", "Navigate"},
		{"Enter", "Load entry"},
		{"Space", "Toggle select"},
		{"d", "Delete selected / single"},
		{"D", "Delete older entries"},
		{"Ctrl+D", "Clear all entries"},
		{"Ctrl+X", "Remove duplicates"},
		{"Esc", "Clear selection / close"},
	}
}

func (c *historyContent) EmptyText() string {
	return styles.MutedStyle.Render("  No matching entries")
}

func (c *historyContent) HandleKey(key string, item *storage.HistoryEntry) (tea.Cmd, widget.PickerAction) {
	// Confirmation mode — intercept all keys
	if c.confirm != confirmNone {
		switch key {
		case "y", "Y":
			return c.confirmYes(item), widget.PickerHandled
		default:
			c.confirm = confirmNone
			c.confirmLabel = ""
		}
		return nil, widget.PickerHandled
	}

	switch key {
	case "esc":
		if len(c.selected) > 0 {
			c.clearSelection()
			return nil, widget.PickerHandled
		}
		return func() tea.Msg { return HistoryClosedMsg{} }, widget.PickerHandled

	case "enter":
		if item != nil {
			entry := *item
			return func() tea.Msg {
				return HistorySelectedMsg{Entry: entry}
			}, widget.PickerHandled
		}
		return nil, widget.PickerHandled

	case "space":
		if item != nil {
			if c.selected[item.ID] {
				delete(c.selected, item.ID)
			} else {
				c.selected[item.ID] = true
			}
			return nil, widget.PickerMoveDown
		}
		return nil, widget.PickerHandled

	case "d":
		return c.deleteSingleOrSelected(item), widget.PickerHandled

	case "D":
		if item != nil {
			c.confirm = confirmDeleteOlder
			c.confirmLabel = "Delete older entries? (y/n)"
		}
		return nil, widget.PickerHandled

	case "ctrl+d":
		if item != nil {
			c.confirm = confirmClearAll
			c.confirmLabel = "Clear all? (y/n)"
		}
		return nil, widget.PickerHandled

	case "ctrl+x":
		if item != nil {
			c.confirm = confirmDeleteDuplicates
			c.confirmLabel = "Remove duplicate entries? (y/n)"
		}
		return nil, widget.PickerHandled
	}

	return nil, widget.PickerNotHandled
}

func (c *historyContent) HandleMsg(msg tea.Msg) (tea.Cmd, []storage.HistoryEntry, bool) {
	switch msg := msg.(type) {
	case historyReloadedMsg:
		// Prune selection
		valid := make(map[int64]bool)
		for _, e := range msg.entries {
			if c.selected[e.ID] {
				valid[e.ID] = true
			}
		}
		c.selected = valid
		return func() tea.Msg { return HistoryChangedMsg{} }, msg.entries, true
	}
	return nil, nil, false
}

func (c *historyContent) deleteSingleOrSelected(item *storage.HistoryEntry) tea.Cmd {
	if len(c.selected) > 0 {
		c.confirm = confirmDeleteSelected
		c.confirmLabel = fmt.Sprintf("Delete %d selected? (y/n)", len(c.selected))
		return nil
	}
	// Single delete (no confirm)
	if item != nil {
		_ = storage.DeleteHistory(item.ID)
		return reloadEntries()
	}
	return nil
}

func (c *historyContent) confirmYes(item *storage.HistoryEntry) tea.Cmd {
	action := c.confirm
	c.confirm = confirmNone
	c.confirmLabel = ""

	switch action {
	case confirmDeleteSelected:
		var ids []int64
		for id := range c.selected {
			ids = append(ids, id)
		}
		_ = storage.DeleteHistoryBatch(ids)
		c.clearSelection()

	case confirmDeleteOlder:
		if item != nil {
			_, _ = storage.DeleteHistoryOlderThan(item.ID)
		}

	case confirmClearAll:
		_ = storage.ClearHistory()
		c.clearSelection()

	case confirmDeleteDuplicates:
		_, _ = storage.DeleteHistoryDuplicates()
		c.clearSelection()
	}

	return reloadEntries()
}

func reloadEntries() tea.Cmd {
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

func (c *historyContent) RenderItem(entry storage.HistoryEntry, isCursor bool, width int) string {
	isSelected := c.selected[entry.ID]

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

// New creates a history picker overlay pre-loaded with the given entries.
func New(entries []storage.HistoryEntry, width, height int) *widget.OverlayPicker[storage.HistoryEntry] {
	content := &historyContent{
		selected: make(map[int64]bool),
	}

	// Compute content width from entries
	helpW := len("↑↓ nav │ enter select │ space mark │ d del │ ? help │ esc close")
	cw := helpW
	for _, e := range entries {
		// method(7) + space(1) + url + status(max 4)
		w := 8 + len(e.URL) + 4
		if w > cw {
			cw = w
		}
	}

	return widget.NewOverlayPicker(content, entries, "Filter history...", width, height, 8, cw, "hp-vscrollbar")
}
