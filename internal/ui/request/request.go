// SPDX-License-Identifier: MIT

package request

import (
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

var tabsConfig = []styles.TabDef{
	{"Body", "B", int(TabBody)},
	{"Headers", "E", int(TabHeaders)},
	{"Auth", "A", int(TabAuth)},
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

// ToggleBodyFormat toggles the body between JSON and YAML format.
func (m *Model) ToggleBodyFormat() {
	m.body.ToggleFormat()
}

// ToggleTokenVisibility toggles the auth token between password and plain text.
func (m *Model) ToggleTokenVisibility() {
	m.auth.ToggleTokenVisibility()
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

// HandleWheel processes a mouse wheel event regardless of focus state.
func (m *Model) HandleWheel(msg tea.MouseWheelMsg) {
	switch m.activeTab {
	case TabBody:
		key := tea.KeyDown
		if msg.Button == tea.MouseWheelUp {
			key = tea.KeyUp
		}
		m.body.textarea, _ = m.body.textarea.Update(tea.KeyPressMsg{Code: key})
	case TabHeaders:
		if msg.Button == tea.MouseWheelUp {
			if m.headers.cursor > 0 {
				m.headers.cursor--
				m.headers.ensureCursorVisible()
			}
		} else {
			if m.headers.cursor < len(m.headers.pairs)-1 {
				m.headers.cursor++
				m.headers.ensureCursorVisible()
			}
		}
	}
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

func (m Model) ViewLayer() *lipgloss.Layer {
	title := lipgloss.NewStyle().Bold(true).Render("Request")

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

	// Parent content: title + content (tabs rendered as child layers only)
	inner := lipgloss.JoinVertical(lipgloss.Left, title, content)
	full := borderStyle.Width(m.width).Height(m.height).Render(inner)

	// Tab buttons as initial child layers (Y=1 inside border, after title)
	tabX := 1 + lipgloss.Width(title) + 2 // border(1) + title + gap(2)
	children, _ := styles.RenderTabLayers(tabsConfig, int(m.activeTab), "req-tab-", tabX, 1)

	switch m.activeTab {
	case TabBody:
		// Format toggle at bottom row
		formatLabel := styles.ActiveTab.Render(formatNames[m.body.format])
		children = append(children, lipgloss.NewLayer(formatLabel).
			ID("req-format-toggle").
			X(3).Y(m.height-2).Z(1)) // border(1) + indent(2)

	case TabAuth:
		// Auth type button
		label := lipgloss.NewStyle().Bold(true).Render("Auth Type")
		typeName := authTypes[m.auth.typeIdx]
		typeBtn := lipgloss.NewStyle().Bold(true).
			Foreground(styles.PrimaryColor).
			Render(typeName + " ▼")
		btnX := 1 + 2 + lipgloss.Width(label) + 2
		children = append(children, lipgloss.NewLayer(typeBtn).
			ID("req-auth-type-btn").
			X(btnX).Y(3).Z(1))

		// Visibility toggle hint
		if m.auth.HasTokenField() {
			hint := styles.MutedStyle.Underline(true).
				Render("toggle visibility [Ctrl+E]")
			tokenLabel := lipgloss.NewStyle().Bold(true).Render("  Token")
			hintX := 1 + lipgloss.Width(tokenLabel) + 2
			children = append(children, lipgloss.NewLayer(hint).
				ID("req-visibility-hint").
				X(hintX).Y(5).Z(1))
		}
	}

	return lipgloss.NewLayer(full, children...).ID("request")
}
