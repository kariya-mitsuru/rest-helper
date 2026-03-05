// SPDX-License-Identifier: MIT

package statusbar

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
)

type Model struct {
	text         string
	historyCount int
	width        int
}

func New() Model {
	return Model{
		text: "Ready",
	}
}

func (m *Model) SetText(text string) {
	m.text = text
}

func (m *Model) SetHistoryCount(count int) {
	m.historyCount = count
}

func (m *Model) SetWidth(w int) {
	m.width = w
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

func (m Model) View() string {
	style := lipgloss.NewStyle().
		Background(lipgloss.Color("#1F2937")).
		Foreground(styles.MutedColor).
		Width(m.width).
		Padding(0, 1)

	left := m.text
	right := fmt.Sprintf("History: %d items | ?/F1: help", m.historyCount)

	spaces := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if spaces < 1 {
		spaces = 1
	}

	content := left + fmt.Sprintf("%*s", spaces, "") + right
	return style.Render(content)
}

// IsHelpClick returns true if the given absolute column falls on the "?/F1: help" label.
func (m Model) IsHelpClick(col int) bool {
	helpLabel := "?/F1: help"
	helpW := lipgloss.Width(helpLabel)
	// View() content starts at col 1 (left padding) and is m.width-4 chars long,
	// so the content string ends at column m.width-4.
	// The help label is the last helpW chars of the content.
	helpEnd := m.width - 4
	helpStart := helpEnd - helpW + 1
	return col >= helpStart && col <= helpEnd
}
