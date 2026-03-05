// SPDX-License-Identifier: MIT

package request

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
)

type Tab int

const (
	TabHeaders Tab = iota
	TabBody
	TabAuth
)

type tabInfo struct {
	name string
	tab  Tab
	key  string
}

var tabsConfig = []tabInfo{
	{"Body", TabBody, "B"},
	{"Headers", TabHeaders, "E"},
	{"Auth", TabAuth, "A"},
}

type Model struct {
	headers HeadersModel
	body    BodyModel
	auth    AuthModel

	activeTab Tab
	focused   bool
	width     int
	height    int
}

func New() Model {
	return Model{
		headers:   NewHeaders(),
		body:      NewBody(),
		auth:      NewAuth(),
		activeTab: TabBody,
	}
}

func (m Model) GetHeaders() map[string]string {
	h := m.headers.Headers()
	// Merge auth header
	if k, v := m.auth.AuthHeader(); k != "" {
		h[k] = v
	}
	return h
}

// GetBody returns the body as JSON (converting from YAML if needed).
// Returns the raw value and any conversion error.
func (m Model) GetBody() (string, error) {
	return m.body.JSONValue()
}

func (m *Model) SetHeaders(h map[string]string) {
	m.headers.SetHeaders(h)
}

func (m *Model) SetBody(b string) {
	m.body.SetValue(b)
}

// GetRawBody returns the body text as entered (before conversion).
func (m Model) GetRawBody() string {
	return m.body.Value()
}

// GetBodyFormat returns "JSON" or "YAML".
func (m Model) GetBodyFormat() string {
	if m.body.Format() == FormatYAML {
		return "YAML"
	}
	return "JSON"
}

// SetBodyFormat sets the body format mode.
func (m *Model) SetBodyFormat(format string) {
	if format == "YAML" {
		m.body.SetFormat(FormatYAML)
	} else {
		m.body.SetFormat(FormatJSON)
	}
}

func (m *Model) SetTab(tab Tab) {
	m.activeTab = tab
	m.updateTabFocus()
}

func (m *Model) Focus() {
	m.focused = true
	m.updateTabFocus()
}

func (m *Model) Blur() {
	m.focused = false
	m.headers.Blur()
	m.body.Blur()
	m.auth.Blur()
}

func (m *Model) updateTabFocus() {
	m.headers.Blur()
	m.body.Blur()
	m.auth.Blur()

	if !m.focused {
		return
	}

	switch m.activeTab {
	case TabHeaders:
		m.headers.Focus()
	case TabBody:
		m.body.Focus()
	case TabAuth:
		m.auth.Focus()
	}
}

// ClickContent handles a click within the request panel content area.
// relRow and relCol are relative to the request panel top-left (including border).
func (m *Model) ClickContent(relRow, relCol int) {
	switch m.activeTab {
	case TabBody:
		// Format label row: border(1) + tabs(1) + textarea_height
		// textarea height = (m.height - 3) - 1 = m.height - 4
		formatRow := 1 + 1 + (m.height - 4)
		if relRow != formatRow {
			return
		}
		// Format label is rendered as "  " + "JSON/YAML" starting at col 2
		label := formatNames[m.body.format]
		labelW := lipgloss.Width(styles.ActiveTab.Render(label))
		if relCol >= 2 && relCol < 2+labelW {
			m.body.ToggleFormat()
		}

	case TabAuth:
		// Button row is at relRow == 3 (border=1, tabs=1, blank=1, button at row 3)
		if relRow == 3 {
			m.auth.ToggleSelect()
		}
		// Hint row: "  Token  toggle visibility [Ctrl+E]" at relRow == 5
		if relRow == 5 && m.auth.HasTokenField() {
			hintStart := lipgloss.Width(lipgloss.NewStyle().Bold(true).Render("  Token")) + 2
			hintEnd := hintStart + lipgloss.Width(styles.MutedStyle.Underline(true).Render("toggle visibility [Ctrl+E]"))
			if relCol >= hintStart && relCol < hintEnd {
				m.auth.ToggleTokenVisibility()
			}
		}
	}
}

// ClickTabAt determines which tab was clicked based on the column position
// (relative to the request panel's content area) and switches to it.
func (m *Model) ClickTabAt(col int) {
	// Skip past "Request  " title prefix
	titleW := lipgloss.Width(lipgloss.NewStyle().Bold(true).Render("Request"))
	pos := titleW + 2
	for _, t := range tabsConfig {
		label := fmt.Sprintf("%s [Alt+%s]", t.name, t.key)
		w := lipgloss.Width(styles.InactiveTab.Render(label))
		if col >= pos && col < pos+w {
			m.activeTab = t.tab
			m.updateTabFocus()
			return
		}
		pos += w + 2 // 2 spaces between tabs
	}
}

// AuthSelectOpen returns true when the auth type dropdown is visible.
func (m Model) AuthSelectOpen() bool {
	return m.auth.SelectOpen()
}

// AuthToggleSelect opens or closes the auth type dropdown.
func (m *Model) AuthToggleSelect() {
	m.auth.ToggleSelect()
}

// AuthDropdownView returns the rendered dropdown overlay.
func (m Model) AuthDropdownView() string {
	return m.auth.DropdownView()
}

// AuthClickDropdown handles a mouse click on the dropdown overlay.
func (m *Model) AuthClickDropdown(row, col int) bool {
	return m.auth.ClickDropdown(row, col)
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.body.SetSize(w, h-3)
	m.headers.SetSize(w, h-3)
	m.auth.SetWidth(w)
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	var cmd tea.Cmd
	switch m.activeTab {
	case TabHeaders:
		m.headers, cmd = m.headers.Update(msg)
	case TabBody:
		m.body, cmd = m.body.Update(msg)
	case TabAuth:
		m.auth, cmd = m.auth.Update(msg)
	}

	return m, cmd
}

func (m Model) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("Request")
	tabs := title + "  " + m.renderTabs()

	borderStyle := styles.NormalBorder
	if m.focused {
		borderStyle = styles.FocusedBorder
	}

	var content string
	switch m.activeTab {
	case TabHeaders:
		content = m.headers.View()
	case TabBody:
		content = m.body.View()
	case TabAuth:
		content = m.auth.View()
	}

	inner := lipgloss.JoinVertical(lipgloss.Left, tabs, content)

	return borderStyle.
		Width(m.width).
		Height(m.height).
		Render(inner)
}

func (m Model) renderTabs() string {
	var parts []string
	for _, t := range tabsConfig {
		label := fmt.Sprintf("%s [Alt+%s]", t.name, t.key)
		if t.tab == m.activeTab {
			parts = append(parts, styles.ActiveTab.Render(label))
		} else {
			parts = append(parts, styles.InactiveTab.Render(label))
		}
	}

	return strings.Join(parts, "  ")
}
