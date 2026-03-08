// SPDX-License-Identifier: MIT

package statusbar

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
)

type Model struct {
	text        string
	layoutLabel string
	width       int
}

func New() Model {
	return Model{
		text:        "Ready",
		layoutLabel: "Split",
	}
}

func (m *Model) SetText(text string) {
	m.text = text
}

func (m *Model) SetLayoutLabel(label string) {
	m.layoutLabel = label
}

func (m *Model) SetWidth(w int) {
	m.width = w
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

func (m Model) ViewLayer() *lipgloss.Layer {
	style := lipgloss.NewStyle().
		Background(styles.BgColor).
		Foreground(styles.MutedColor).
		Width(m.width).
		Padding(0, 1)

	left := m.text
	layoutLabel := m.layoutLabel + " [Alt+L]"
	helpLabel := "?/F1: help"

	rightParts := layoutLabel + "  " + helpLabel
	spaces := m.width - lipgloss.Width(left) - lipgloss.Width(rightParts) - 4
	if spaces < 1 {
		spaces = 1
	}

	content := left + fmt.Sprintf("%*s", spaces, "") + rightParts
	full := style.Render(content)

	btnStyle := style.UnsetWidth().UnsetPadding()

	layoutX := 1 + lipgloss.Width(left) + spaces
	layoutRendered := btnStyle.Render(layoutLabel)

	helpX := layoutX + lipgloss.Width(layoutLabel) + 2
	helpRendered := btnStyle.Render(helpLabel)

	return lipgloss.NewLayer(full,
		lipgloss.NewLayer(layoutRendered).ID("layout-btn").X(layoutX).Z(1),
		lipgloss.NewLayer(helpRendered).ID("help-btn").X(helpX).Z(1),
	).ID("statusbar")
}
