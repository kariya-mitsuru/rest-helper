// SPDX-License-Identifier: MIT

package response

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"rest-helper/internal/clipboard"
	"rest-helper/internal/ui/widget"
)

// ContextMenuCopiedMsg is sent when a value is copied via the context menu.
type ContextMenuCopiedMsg struct {
	Value string
	Error error
}

// ContextMenuVisible returns whether the context menu is open.
func (m Model) ContextMenuVisible() bool {
	return m.ctxMenu != nil
}

// CloseContextMenu dismisses the context menu.
func (m *Model) CloseContextMenu() {
	m.ctxMenu = nil
	m.ctxLine = lineInfo{}
}

// OpenContextMenu opens a context menu for the value at the given screen position.
// panelMinY is the top Y coordinate of the response panel on screen.
func (m *Model) OpenContextMenu(screenX, screenY, panelMinY int) {
	viewportLine := screenY - panelMinY - m.headerRowCount()
	if viewportLine < 0 || viewportLine >= m.viewport.Height() {
		return
	}

	rawIdx := m.rawLineAt(m.viewport.YOffset() + viewportLine)
	if rawIdx < 0 || rawIdx >= len(m.lineInfos) {
		return
	}

	info := m.lineInfos[rawIdx]
	if info.value == "" {
		return
	}

	m.ctxLine = info
	items := []widget.MenuItem{
		{Label: "Copy value", ID: "copy-value"},
	}
	if info.key != "" {
		items = append(items,
			widget.MenuItem{Label: "Copy as JSON", ID: "copy-json"},
			widget.MenuItem{Label: "Copy as YAML", ID: "copy-yaml"},
		)
	}
	m.ctxMenu = widget.NewContextMenu(items, screenX, screenY, m.screenW, m.screenH)
}

// HandleContextMenuKey processes a keyboard event for the context menu.
func (m *Model) HandleContextMenuKey(msg tea.KeyPressMsg) tea.Cmd {
	item, action := m.ctxMenu.HandleKey(msg.String())
	return m.processContextMenuAction(item, action)
}

// HandleContextMenuClick processes a mouse click for the context menu.
func (m *Model) HandleContextMenuClick(screenX, screenY int) tea.Cmd {
	item, action := m.ctxMenu.HandleClick(screenX, screenY)
	return m.processContextMenuAction(item, action)
}

func (m *Model) processContextMenuAction(item widget.MenuItem, action widget.MenuAction) tea.Cmd {
	switch action {
	case widget.MenuClose:
		m.CloseContextMenu()
		return nil
	case widget.MenuSelected:
		var copyText string
		switch item.ID {
		case "copy-value":
			copyText = m.ctxLine.value
		case "copy-json":
			copyText = m.ctxLine.formatJSON()
		case "copy-yaml":
			copyText = m.ctxLine.formatYAML()
		default:
			m.CloseContextMenu()
			return nil
		}
		err := clipboard.Write(copyText)
		m.CloseContextMenu()
		return func() tea.Msg {
			return ContextMenuCopiedMsg{Value: copyText, Error: err}
		}
	}
	return nil
}

// BuildContextMenuLayer returns the context menu as a lipgloss Layer.
func (m Model) BuildContextMenuLayer() *lipgloss.Layer {
	return m.ctxMenu.BuildLayer()
}

// rawLineAt maps a viewport line index to a rawLines index.
// In non-wrap mode the mapping is 1:1; in wrap mode it accounts for
// lines that span multiple viewport rows.
func (m *Model) rawLineAt(viewportLine int) int {
	if !m.wrapMode {
		return viewportLine
	}
	w := m.contentWidth()
	if w <= 0 {
		return viewportLine
	}
	cur := 0
	for i, line := range m.rawLines {
		wrapped := ansi.Hardwrap(line, w, false)
		n := strings.Count(wrapped, "\n") + 1
		if cur+n > viewportLine {
			return i
		}
		cur += n
	}
	return len(m.rawLines) - 1
}
