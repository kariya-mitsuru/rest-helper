// SPDX-License-Identifier: MIT

package help

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
)

type Model struct {
	Visible  bool
	version  string
	width    int
	height   int
	viewport viewport.Model
	ready    bool
}

func New(version string) Model {
	return Model{version: version}
}

func (m *Model) Toggle() {
	m.Visible = !m.Visible
	if m.Visible {
		m.viewport.GotoTop()
	}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.updateViewport()
}

func (m *Model) updateViewport() {
	content := m.renderContent()
	contentH := lipgloss.Height(content)

	// Max viewport height: screen minus border(2) + padding(2) + title(1) + footer(2)
	maxH := m.height - 7
	if maxH < 5 {
		maxH = 5
	}
	vpH := contentH
	if vpH > maxH {
		vpH = maxH
	}

	m.viewport.SetWidth(64)
	m.viewport.SetHeight(vpH)
	m.viewport.SetContent(content)
	m.ready = true
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.Visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "f1", "q", "?":
			m.Visible = false
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) renderContent() string {
	sections := []struct {
		header string
		keys   [][2]string
	}{
		{
			"General",
			[][2]string{
				{"Ctrl+S", "Send request"},
				{"Tab / Shift+Tab", "Switch panel focus"},
				{"Alt+U", "URL bar"},
				{"Alt+E / B / A", "Headers / Body / Auth"},
				{"Alt+H", "Open history"},
				{"Alt+R", "Response Body"},
				{"Alt+D", "Response Headers"},
				{"Ctrl+C / Ctrl+Q", "Quit"},
				{"? / F1", "Toggle this help"},
			},
		},
		{
			"URL Bar",
			[][2]string{
				{"Ctrl+P", "Open method selector"},
				{"Up/Down, Enter", "Select method"},
				{"Esc", "Close selector"},
				{"Type URL", "Enter request URL"},
			},
		},
		{
			"Body Tab",
			[][2]string{
				{"Ctrl+T", "Toggle JSON/YAML"},
			},
		},
		{
			"Headers Tab",
			[][2]string{
				{"Up/Down", "Navigate rows"},
				{"Left/Right", "Move cursor / switch column"},
				{"Enter", "New row"},
				{"Ctrl+D", "Delete row"},
			},
		},
		{
			"Auth Tab",
			[][2]string{
				{"Up/Down", "Select auth type"},
				{"Enter", "Open/confirm dropdown"},
				{"Esc", "Close dropdown"},
				{"Ctrl+E", "Toggle token visibility"},
			},
		},
		{
			"History (Alt+H / ↑↓ in URL bar)",
			[][2]string{
				{"/ (type)", "Filter entries"},
				{"Up/Down", "Navigate"},
				{"Enter", "Load entry"},
				{"Space", "Toggle select"},
				{"d", "Delete selected/single"},
				{"D", "Delete older"},
				{"Ctrl+D", "Clear all"},
				{"Ctrl+X", "Remove duplicates"},
				{"Esc", "Clear selection / close"},
			},
		},
		{
			"Response",
			[][2]string{
				{"Tab", "Switch Body/Headers"},
				{"Up/Down", "Scroll response"},
				{"PgUp/PgDn", "Page scroll"},
				{"Ctrl+T", "Toggle JSON/YAML view"},
				{"Ctrl+W", "Toggle wrap/scroll"},
				{"Left/Right", "Horizontal scroll"},
				{"y", "Copy field picker"},
			},
		},
	}

	keyStyle := lipgloss.NewStyle().
		Foreground(styles.SecondaryColor).
		Bold(true).
		Width(22)
	descStyle := lipgloss.NewStyle().
		Foreground(styles.TextColor)
	headerStyle := lipgloss.NewStyle().
		Foreground(styles.WarningColor).
		Bold(true).
		MarginTop(1)

	var b strings.Builder
	for _, section := range sections {
		b.WriteString(headerStyle.Render(section.header))
		b.WriteString("\n")
		for _, kv := range section.keys {
			b.WriteString("  ")
			b.WriteString(keyStyle.Render(kv[0]))
			b.WriteString(descStyle.Render(kv[1]))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m Model) View() string {
	if !m.Visible {
		return ""
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.PrimaryColor).
		Render("REST Helper " + m.version + " - Keyboard Shortcuts")

	line1 := styles.MutedStyle.Render("↑↓ scroll  ESC/? close")
	if m.viewport.TotalLineCount() > m.viewport.Height() {
		pct := int(m.viewport.ScrollPercent() * 100)
		line1 += styles.MutedStyle.Render("  " + fmt.Sprintf("%d%%", pct))
	}
	footer := line1 + "\n" + styles.MutedStyle.Render("© 2026 Mitsuru Kariya  MIT License")

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	b.WriteString(footer)

	overlayStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(styles.PrimaryColor).
		Padding(1, 3)

	return overlayStyle.Render(b.String())
}
