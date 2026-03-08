// SPDX-License-Identifier: MIT

package request

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
	"rest-helper/internal/ui/widget"
)

type Tab int

const (
	TabHeaders Tab = iota
	TabBody
	TabAuth
)

var tabsConfig = []styles.TabDef{
	{Name: "Body", Key: "B", Index: int(TabBody)},
	{Name: "Headers", Key: "E", Index: int(TabHeaders)},
	{Name: "Auth", Key: "A", Index: int(TabAuth)},
}

type Model struct {
	headers HeadersModel
	body    BodyModel
	auth    AuthModel

	activeTab Tab
	focused   bool
	width     int
	height    int

	bodyVDrag widget.ScrollDrag
	bodyHDrag widget.ScrollDrag

	hideTitle bool
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
	if m.body.Format() == formatYAML {
		return "YAML"
	}
	return "JSON"
}

// SetBodyFormat sets the body format mode.
func (m *Model) SetBodyFormat(format string) {
	if format == "YAML" {
		m.body.SetFormat(formatYAML)
	} else {
		m.body.SetFormat(formatJSON)
	}
}

func (m *Model) SetHideTitle(hide bool) {
	m.hideTitle = hide
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
	m.updateTabFocus()
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
		// textarea.Update ignores input when not focused, so temporarily focus.
		wasFocused := m.body.textarea.Focused()
		if !wasFocused {
			m.body.textarea.Focus()
		}
		m.body.textarea, _ = m.body.textarea.Update(tea.KeyPressMsg{Code: key})
		if !wasFocused {
			m.body.textarea.Blur()
		}
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

// HandleBodyVScrollClick handles a click on the body vertical scrollbar.
func (m *Model) HandleBodyVScrollClick(localY, baseY int) {
	total := m.body.textarea.TotalLineCount()
	vis := m.body.textarea.ViewportHeight()
	if vis <= 1 || total <= vis {
		return
	}
	if newOff, changed := m.bodyVDrag.HandleClickAndClamp(localY, baseY, total, vis, m.body.textarea.YOffset()); changed {
		m.body.textarea.SetYOffset(newOff)
	}
}

// handleBodyVScrollDrag processes mouse motion during body scrollbar drag.
func (m *Model) handleBodyVScrollDrag(mouseY int) {
	total := m.body.textarea.TotalLineCount()
	vis := m.body.textarea.ViewportHeight()
	if vis <= 1 || total <= vis {
		return
	}
	m.body.textarea.SetYOffset(m.bodyVDrag.DragOffset(mouseY, vis, total))
}

// ToggleBodyWrap toggles between wrap and scroll mode for the body textarea.
func (m *Model) ToggleBodyWrap() { m.body.ToggleWrap() }

// HandleBodyHScrollClick handles a click on the body horizontal scrollbar.
func (m *Model) HandleBodyHScrollClick(localCol, baseX int) {
	maxW := m.body.textarea.MaxLineWidth()
	contentW := m.body.textarea.Width()
	if contentW <= 0 || maxW <= contentW {
		return
	}
	if delta := m.bodyHDrag.HandleClick(localCol, baseX, maxW, contentW, m.body.textarea.XOffset()); delta != 0 {
		m.body.textarea.SetXOffset(m.body.textarea.XOffset() + delta)
	}
}

// handleBodyHScrollDrag processes mouse motion during body horizontal scrollbar drag.
func (m *Model) handleBodyHScrollDrag(mouseX int) {
	maxW := m.body.textarea.MaxLineWidth()
	contentW := m.body.textarea.Width()
	if contentW <= 0 || maxW <= contentW {
		return
	}
	m.body.textarea.SetXOffset(m.bodyHDrag.DragOffset(mouseX, contentW, maxW))
}

// IsDragging reports whether any scrollbar drag is active.
func (m Model) IsDragging() bool {
	return m.bodyVDrag.Active() || m.bodyHDrag.Active()
}

// StopDrag ends any active scrollbar drag. Returns true if a drag was stopped.
func (m *Model) StopDrag() bool {
	if m.bodyVDrag.Active() {
		m.bodyVDrag.Stop()
		return true
	}
	if m.bodyHDrag.Active() {
		m.bodyHDrag.Stop()
		return true
	}
	return false
}

// HandleDragMotion processes mouse motion for any active scrollbar drag.
// Returns true if a drag was handled.
func (m *Model) HandleDragMotion(x, y int) bool {
	if m.bodyVDrag.Active() {
		m.handleBodyVScrollDrag(y)
		return true
	}
	if m.bodyHDrag.Active() {
		m.handleBodyHScrollDrag(x)
		return true
	}
	return false
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
	title := styles.BoldStyle.Render("Request")

	borderStyle := styles.BorderStyleForFocus(m.focused)

	var content string
	switch m.activeTab {
	case TabHeaders:
		content = m.headers.View()
	case TabBody:
		content = m.body.View()
	case TabAuth:
		content = m.auth.View()
	}

	// Reserve first row for tab buttons (rendered as child layers)
	inner := "\n" + content
	full := borderStyle.Width(m.width).Height(m.height).Render(inner)

	// Title on top border (hidden in fullscreen mode; app renders tabs instead)
	var children []*lipgloss.Layer
	if !m.hideTitle {
		children = append(children, lipgloss.NewLayer(title).X(2).Y(0).Z(1))
	}

	// Tab buttons on first content row (Y=1, inside border)
	tabLayers, tabEndX := styles.RenderTabLayers(tabsConfig, int(m.activeTab), "req-tab-", 2, 1)
	children = append(children, tabLayers...)

	switch m.activeTab {
	case TabHeaders:
		// Vertical scrollbar: border(1) + tabs(1) + column header(1) = Y offset 3
		total := len(m.headers.pairs)
		vis := m.headers.visibleRows()
		if sbLayer := widget.ScrollbarLayer("req-headers-vscrollbar", total, vis, m.headers.scrollOffset, m.width, 3, 1); sbLayer != nil {
			children = append(children, sbLayer)
		}

	case TabBody:
		// Format toggle on tab row
		x := tabEndX
		formatLabel := styles.ActiveTab.Render(formatNames[m.body.format] + " [Ctrl+T]")
		children = append(children, lipgloss.NewLayer(formatLabel).
			ID("req-format-toggle").
			X(x).Y(1).Z(1))
		x += lipgloss.Width(formatLabel) + 2

		// Wrap/Scroll toggle on tab row
		wrapRendered := styles.ActiveTab.Render(styles.WrapToggleLabel(m.body.WrapMode()))
		children = append(children, lipgloss.NewLayer(wrapRendered).
			ID("req-wrap-toggle").
			X(x).Y(1).Z(1))

		// Vertical scrollbar
		total := m.body.textarea.TotalLineCount()
		vis := m.body.textarea.ViewportHeight()
		if sbLayer := widget.ScrollbarLayer("req-body-vscrollbar", total, vis, m.body.textarea.YOffset(), m.width, 2, 1); sbLayer != nil {
			children = append(children, sbLayer)
		}

		// Horizontal scrollbar (scroll mode only)
		if !m.body.WrapMode() {
			maxW := m.body.textarea.MaxLineWidth()
			contentW := m.body.textarea.Width()
			if maxW > contentW {
				sb := styles.RenderHScrollbar(maxW, contentW, m.body.textarea.XOffset())
				sbY := m.height - 2 // last content row inside border
				children = append(children, lipgloss.NewLayer(sb).
					ID("req-body-hscrollbar").
					X(2).Y(sbY).Z(1))
			}
		}

	case TabAuth:
		// Auth type button
		label := styles.BoldStyle.Render("Auth Type")
		typeName := authTypes[m.auth.typeIdx]
		typeBtn := styles.BoldStyle.Foreground(styles.PrimaryColor).
			Render(typeName + " ▼")
		btnX := 1 + 1 + 2 + lipgloss.Width(label) + 2 // border(1) + padding(1) + indent(2) + label + gap(2)
		children = append(children, lipgloss.NewLayer(typeBtn).
			ID("req-auth-type-btn").
			X(btnX).Y(3).Z(1))

		// Visibility toggle hint
		if m.auth.HasTokenField() {
			hint := styles.MutedStyle.Underline(true).
				Render("toggle visibility [Ctrl+E]")
			tokenLabel := styles.BoldStyle.Render("  Token")
			hintX := 1 + 1 + lipgloss.Width(tokenLabel) + 2 // border(1) + padding(1) + label + gap(2)
			children = append(children, lipgloss.NewLayer(hint).
				ID("req-visibility-hint").
				X(hintX).Y(5).Z(1))
		}
	}

	return lipgloss.NewLayer(full, children...).ID("request")
}
