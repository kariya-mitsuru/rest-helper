// SPDX-License-Identifier: MIT

package statusbar

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
)

type Model struct {
	text  string
	width int
}

func New() Model {
	return Model{
		text: "Ready",
	}
}

func (m *Model) SetText(text string) {
	m.text = text
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

	spaces := m.width - lipgloss.Width(left) - lipgloss.Width(helpLabel) - 4
	if spaces < 1 {
		spaces = 1
	}

	content := left + fmt.Sprintf("%*s", spaces, "") + helpLabel
	full := style.Render(content)

	helpX := 1 + lipgloss.Width(left) + spaces
	helpRendered := lipgloss.NewStyle().
		Background(lipgloss.Color("#1F2937")).
		Foreground(styles.MutedColor).
		Render(helpLabel)

	return lipgloss.NewLayer(full,
		lipgloss.NewLayer(helpRendered).ID("help-btn").X(helpX).Z(1),
	).ID("statusbar")
}
