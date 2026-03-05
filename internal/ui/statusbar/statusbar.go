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

func (m Model) ViewLayer() *lipgloss.Layer {
	style := lipgloss.NewStyle().
		Background(lipgloss.Color("#1F2937")).
		Foreground(styles.MutedColor).
		Width(m.width).
		Padding(0, 1)

	left := m.text
	helpLabel := "?/F1: help"
	right := fmt.Sprintf("History: %d items | %s", m.historyCount, helpLabel)

	spaces := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if spaces < 1 {
		spaces = 1
	}

	content := left + fmt.Sprintf("%*s", spaces, "") + right
	full := style.Render(content)

	// Help label position: padding(1) + left + spaces + "History: N items | "
	helpPrefix := fmt.Sprintf("History: %d items | ", m.historyCount)
	helpX := 1 + lipgloss.Width(left) + spaces + lipgloss.Width(helpPrefix)
	helpRendered := lipgloss.NewStyle().
		Background(lipgloss.Color("#1F2937")).
		Foreground(styles.MutedColor).
		Render(helpLabel)

	return lipgloss.NewLayer(full,
		lipgloss.NewLayer(helpRendered).ID("help-btn").X(helpX).Z(1),
	).ID("statusbar")
}
