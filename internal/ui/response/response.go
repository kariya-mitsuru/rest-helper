// SPDX-License-Identifier: MIT

package response

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"rest-helper/internal/http"
	"rest-helper/internal/ui/styles"
	"rest-helper/internal/ui/widget"
)

type displayFormat int

const (
	formatJSON displayFormat = iota
	formatYAML
	formatRAW
)

type Tab int

const (
	TabBody Tab = iota
	TabHeaders
)

var tabsConfig = []styles.TabDef{
	{Name: "Body", Key: "R", Index: int(TabBody)},
	{Name: "Headers", Key: "D", Index: int(TabHeaders)},
}

type Model struct {
	viewport        viewport.Model
	response        *http.Response
	err             error
	focused         bool
	width           int
	height          int
	loading         bool
	display         displayFormat
	preferredFormat displayFormat     // user's preferred format for JSON responses
	wrapMode        bool              // true=wrap, false=horizontal scroll
	xOffset         int               // horizontal scroll position
	rawLines        []string          // pre-wrap content lines for horizontal scroll
	maxLineWidth    int               // max visible width among rawLines
	hDrag           widget.ScrollDrag // horizontal scrollbar drag state
	vDrag           widget.ScrollDrag
	fieldPicker     *widget.OverlayPicker[PathValue]
	ctxMenu         *widget.ContextMenu
	ctxLine         lineInfo   // info for the right-clicked line
	lineInfos       []lineInfo // extracted key+value per raw line
	bodyIsJSON      bool       // cached result of json.Valid on response body
	activeTab       Tab
	screenW         int // full terminal width (for overlay sizing)
	screenH         int // full terminal height
	hideTitle       bool
}

func New() Model {
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	vp.SetContent("Press Ctrl+S to send a request")

	return Model{
		viewport: vp,
		wrapMode: true,
	}
}

func (m *Model) Focus() {
	m.focused = true
}

func (m *Model) Blur() {
	m.focused = false
}

func (m *Model) SetLoading() {
	m.loading = true
	m.response = nil
	m.err = nil
	m.activeTab = TabBody
	m.viewport.SetContent("  Sending request...")
}

func (m *Model) SetResponse(resp *http.Response) {
	m.loading = false
	m.response = resp
	m.err = nil
	m.xOffset = 0
	m.activeTab = TabBody
	m.bodyIsJSON = resp.Body != "" && json.Valid([]byte(resp.Body))
	// Use preferred format if the body is JSON, otherwise fall back to JSON (raw)
	m.display = formatJSON
	if m.bodyIsJSON {
		m.display = m.preferredFormat
	}
	m.refreshContent()
}

func (m *Model) SetError(err error) {
	m.loading = false
	m.response = nil
	m.err = err
	m.activeTab = TabBody
	m.viewport.SetContent(
		lipgloss.NewStyle().Foreground(styles.ErrorColor).Render(
			fmt.Sprintf("Error: %s", err.Error()),
		),
	)
	m.viewport.GotoTop()
}

func (m *Model) isBodyJSON() bool {
	return m.bodyIsJSON
}

// refreshContent re-renders the response body with the current viewport width.
func (m *Model) refreshContent() {
	if m.response == nil {
		return
	}

	// Get content WITHOUT wrapping
	var raw string
	if m.activeTab == TabHeaders {
		raw = m.formatHeaders()
	} else if m.display == formatRAW {
		raw = m.response.Body
		if raw == "" {
			raw = emptyResponseText
		}
	} else if m.display == formatYAML && m.isBodyJSON() {
		raw = formatAsYAML(m.response.Body)
	} else {
		raw = formatBody(m.response.Body)
	}

	m.rawLines = strings.Split(raw, "\n")

	// Compute max visible line width for scroll limit
	m.maxLineWidth = 0
	for _, line := range m.rawLines {
		if w := ansi.StringWidth(line); w > m.maxLineWidth {
			m.maxLineWidth = w
		}
	}

	// Adjust viewport height: scrollbar takes 1 line in scroll mode when needed
	m.updateViewportHeight()

	if m.wrapMode {
		// Wrap mode: apply hard wrap
		w := m.contentWidth()
		if w > 0 {
			m.viewport.SetContent(ansi.Hardwrap(raw, w, false))
		} else {
			m.viewport.SetContent(raw)
		}
	} else {
		// Scroll mode: apply horizontal offset
		m.applyXOffset()
	}
	m.viewport.GotoTop()
	m.buildLineInfos()
}

func (m Model) contentWidth() int {
	w := m.viewport.Width()
	if w < 20 {
		w = 80
	}
	return w
}

// maxXOffset returns the maximum meaningful xOffset value.
func (m Model) maxXOffset() int {
	max := m.maxLineWidth - m.contentWidth()
	if max < 0 {
		max = 0
	}
	return max
}

// clampXOffset ensures xOffset is within valid bounds.
func (m *Model) clampXOffset() {
	if max := m.maxXOffset(); m.xOffset > max {
		m.xOffset = max
	}
	if m.xOffset < 0 {
		m.xOffset = 0
	}
}

// applyXOffset renders rawLines with horizontal scrolling at the current xOffset.
func (m *Model) applyXOffset() {
	w := m.contentWidth()
	var b strings.Builder
	for i, line := range m.rawLines {
		truncated := ansi.TruncateLeft(line, m.xOffset, "")
		if w > 0 {
			truncated = ansi.Truncate(truncated, w, "")
		}
		b.WriteString(truncated)
		if i < len(m.rawLines)-1 {
			b.WriteByte('\n')
		}
	}
	m.viewport.SetContent(b.String())
}

// ToggleFormat cycles display format: JSON → YAML → RAW → JSON.
// For non-JSON responses, toggles between RAW and the current format.
func (m *Model) ToggleFormat() {
	if m.isBodyJSON() {
		switch m.display {
		case formatJSON:
			m.display = formatYAML
		case formatYAML:
			m.display = formatRAW
		default:
			m.display = formatJSON
		}
	} else {
		if m.display == formatRAW {
			m.display = formatJSON
		} else {
			m.display = formatRAW
		}
	}
	if m.display != formatRAW {
		m.preferredFormat = m.display
	}
	m.refreshContent()
}

// sortedHeaderKeys returns the response header keys in sorted order.
func (m *Model) sortedHeaderKeys() []string {
	if m.response == nil || len(m.response.Headers) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m.response.Headers))
	for k := range m.response.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// formatHeaders returns a colorized Key: Value display of response headers.
func (m *Model) formatHeaders() string {
	keys := m.sortedHeaderKeys()
	if len(keys) == 0 {
		return styles.MutedStyle.Render("(no headers)")
	}

	var b strings.Builder
	first := true
	for _, k := range keys {
		for _, v := range m.response.Headers[k] {
			if !first {
				b.WriteByte('\n')
			}
			first = false
			b.WriteString(hlKeyStyle.Render(k))
			b.WriteString(hlPunctStyle.Render(": "))
			b.WriteString(hlStrStyle.Render(v))
		}
	}

	return b.String()
}

// SetTab sets the active response tab.
func (m *Model) SetTab(tab Tab) {
	if tab != m.activeTab {
		m.activeTab = tab
		m.xOffset = 0
		m.refreshContent()
	}
}

// FieldPickerVisible returns whether the field picker overlay is open.
func (m Model) FieldPickerVisible() bool {
	return m.fieldPicker != nil
}

// OpenFieldPicker opens the field picker overlay for the current response body.
func (m *Model) OpenFieldPicker() {
	m.fieldPicker = NewFieldPicker(m.response.Body, m.screenW, m.screenH)
}

// UpdateFieldPicker forwards a message to the field picker.
// The caller must check FieldPickerVisible() before calling this.
func (m *Model) UpdateFieldPicker(msg tea.Msg) tea.Cmd {
	return m.fieldPicker.Update(msg)
}

// BuildFieldPickerLayer returns the field picker as a lipgloss Layer.
// The caller must check FieldPickerVisible() before calling this.
func (m Model) BuildFieldPickerLayer() *lipgloss.Layer {
	return m.fieldPicker.BuildLayer()
}

// GetFieldPicker returns the field picker, or nil if not visible.
func (m *Model) GetFieldPicker() *widget.OverlayPicker[PathValue] {
	return m.fieldPicker
}

// CloseFieldPicker closes the field picker.
func (m *Model) CloseFieldPicker() {
	m.fieldPicker = nil
}

// ToggleWrap switches between wrap and horizontal scroll modes.
func (m *Model) ToggleWrap() {
	m.wrapMode = !m.wrapMode
	m.xOffset = 0
	m.refreshContent()
}

// SetPreferredFormat sets the preferred display format ("JSON" or "YAML").
func (m *Model) SetPreferredFormat(f string) {
	if f == "YAML" {
		m.preferredFormat = formatYAML
	} else {
		m.preferredFormat = formatJSON
	}
}

// GetPreferredFormat returns the current preferred format as "JSON", "YAML", or "RAW".
func (m Model) GetPreferredFormat() string {
	switch m.preferredFormat {
	case formatYAML:
		return "YAML"
	default:
		return "JSON"
	}
}

// SetWrapMode sets the wrap mode.
func (m *Model) SetWrapMode(wrap bool) {
	m.wrapMode = wrap
}

// GetWrapMode returns the current wrap mode.
func (m Model) GetWrapMode() bool {
	return m.wrapMode
}

// HandleWheel processes a mouse wheel event, forwarding it to the viewport
// regardless of focus state.
func (m *Model) HandleWheel(msg tea.MouseWheelMsg) {
	if !m.wrapMode && msg.Mod&tea.ModShift != 0 {
		switch msg.Button {
		case tea.MouseWheelUp:
			m.xOffset -= 3
			m.clampXOffset()
			m.applyXOffset()
			return
		case tea.MouseWheelDown:
			m.xOffset += 3
			m.clampXOffset()
			m.applyXOffset()
			return
		}
	}
	m.viewport, _ = m.viewport.Update(msg)
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+t":
			if m.activeTab == TabBody && m.response != nil && m.response.Body != "" {
				m.ToggleFormat()
				return m, nil
			}
		case "y":
			if m.activeTab == TabBody && m.response != nil && m.response.Body != "" {
				m.OpenFieldPicker()
				return m, nil
			}
		case "ctrl+w":
			m.ToggleWrap()
			return m, nil
		case "home", "ctrl+home":
			m.viewport.GotoTop()
			return m, nil
		case "end", "ctrl+end":
			m.viewport.GotoBottom()
			return m, nil
		case "left":
			if !m.wrapMode && m.xOffset > 0 {
				m.xOffset--
				m.applyXOffset()
				return m, nil
			}
		case "right":
			if !m.wrapMode && m.xOffset < m.maxXOffset() {
				m.xOffset++
				m.applyXOffset()
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) ViewLayer() *lipgloss.Layer {
	borderStyle := styles.BorderStyleForFocus(m.focused)

	title := m.renderTitle()
	meta := m.renderMeta()
	hasMeta := meta != ""

	// Reserve first row for tab buttons (rendered as child layers)
	parts := []string{""}
	if hasMeta {
		parts = append(parts, meta)
	}
	parts = append(parts, m.viewport.View())

	if m.HasScrollBar() {
		// Reserve a line for the scrollbar (rendered as a child layer below)
		parts = append(parts, "")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	full := borderStyle.Width(m.width).Height(m.height).Render(content)

	// Title on top border (hidden in fullscreen mode; app renders tabs instead)
	var children []*lipgloss.Layer
	if !m.hideTitle {
		children = append(children, lipgloss.NewLayer(title).X(2).Y(0).Z(1))
	}

	// Tab buttons on first content row (Y=1, inside border)
	tabLayers, x := styles.RenderTabLayers(tabsConfig, int(m.activeTab), "resp-tab-", 2, 1)
	children = append(children, tabLayers...)

	if m.response != nil {
		// Format toggle (shown for JSON bodies, or when RAW is active)
		if m.activeTab == TabBody && (m.isBodyJSON() || m.display == formatRAW) {
			label := "JSON [Ctrl+T]"
			switch m.display {
			case formatYAML:
				label = "YAML [Ctrl+T]"
			case formatRAW:
				label = "RAW  [Ctrl+T]"
			}
			rendered := styles.ActiveTab.Render(label)
			children = append(children, lipgloss.NewLayer(rendered).
				ID("resp-format-toggle").
				X(x).Y(1).Z(1))
			x += lipgloss.Width(rendered) + 2
		}

		// Wrap/Scroll toggle
		wrapRendered := styles.ActiveTab.Render(styles.WrapToggleLabel(m.wrapMode))
		children = append(children, lipgloss.NewLayer(wrapRendered).
			ID("resp-wrap-toggle").
			X(x).Y(1).Z(1))
		x += lipgloss.Width(wrapRendered) + 2

		// Copy button (only for body tab with content)
		if m.activeTab == TabBody && m.response.Body != "" {
			copyRendered := styles.InactiveTab.Render("Copy [y]")
			children = append(children, lipgloss.NewLayer(copyRendered).
				ID("resp-copy-btn").
				X(x).Y(1).Z(1))
		}

		// Horizontal scrollbar
		if m.HasScrollBar() {
			sb := m.renderHScrollBar()
			sbY := m.ScrollBarRelY()
			children = append(children, lipgloss.NewLayer(sb).
				ID("resp-scrollbar").
				X(2).Y(sbY).Z(1))
		}

		// Vertical scrollbar
		total := m.viewport.TotalLineCount()
		vis := m.viewport.Height()
		sbY := 2 // border(1) + tabs(1)
		if hasMeta {
			sbY = 3
		}
		if sbLayer := widget.ScrollbarLayer("resp-vscrollbar", total, vis, m.viewport.YOffset(), m.width, sbY, 1); sbLayer != nil {
			children = append(children, sbLayer)
		}
	}

	return lipgloss.NewLayer(full, children...).ID("response")
}

// renderHScrollBar renders a horizontal scrollbar indicating xOffset position.
func (m Model) renderHScrollBar() string {
	return styles.RenderHScrollbar(m.maxLineWidth, m.contentWidth(), m.xOffset)
}

// HasScrollBar returns true when the horizontal scrollbar is visible.
func (m Model) HasScrollBar() bool {
	return !m.wrapMode && m.maxLineWidth > m.contentWidth()
}

// headerRowCount returns the number of rows above the viewport content.
// border(1) + tabs(1) + meta(1) = 3 when meta is present, 2 otherwise.
func (m Model) headerRowCount() int {
	if m.hasMeta() {
		return 3
	}
	return 2
}

// ScrollBarRelY returns the scrollbar's row relative to the response panel top.
// Returns -1 if scrollbar is not visible.
func (m Model) ScrollBarRelY() int {
	if !m.HasScrollBar() {
		return -1
	}
	return m.headerRowCount() + m.viewport.Height()
}

// HandleHScrollClick handles a click on the horizontal scrollbar.
func (m *Model) HandleHScrollClick(localCol, baseX int) {
	trackW := m.contentWidth()
	if trackW <= 0 {
		return
	}
	if delta := m.hDrag.HandleClick(localCol, baseX, m.maxLineWidth, trackW, m.xOffset); delta != 0 {
		m.xOffset += delta
		m.clampXOffset()
		m.applyXOffset()
	}
}

// handleHScrollDrag processes mouse motion during horizontal scrollbar drag.
func (m *Model) handleHScrollDrag(mouseX int) {
	trackW := m.contentWidth()
	if trackW <= 0 {
		return
	}
	m.xOffset = m.hDrag.DragOffset(mouseX, trackW, m.maxLineWidth)
	m.applyXOffset()
}

// handleVScrollDrag processes mouse motion during vertical scrollbar drag.
func (m *Model) handleVScrollDrag(mouseY int) {
	total := m.viewport.TotalLineCount()
	vis := m.viewport.Height()
	if vis <= 1 || total <= vis {
		return
	}
	m.viewport.SetYOffset(m.vDrag.DragOffset(mouseY, vis, total))
}

// IsDragging reports whether any scrollbar drag is active.
func (m Model) IsDragging() bool {
	return m.hDrag.Active() || m.vDrag.Active()
}

// StopDrag ends any active scrollbar drag. Returns true if a drag was stopped.
func (m *Model) StopDrag() bool {
	if m.hDrag.Active() {
		m.hDrag.Stop()
		return true
	}
	if m.vDrag.Active() {
		m.vDrag.Stop()
		return true
	}
	return false
}

// HandleDragMotion processes mouse motion for any active scrollbar drag.
// Returns true if a drag was handled.
func (m *Model) HandleDragMotion(x, y int) bool {
	if m.hDrag.Active() {
		m.handleHScrollDrag(x)
		return true
	}
	if m.vDrag.Active() {
		m.handleVScrollDrag(y)
		return true
	}
	return false
}

// HandleVScrollClick handles a click on the vertical scrollbar.
func (m *Model) HandleVScrollClick(localY, baseY int) {
	total := m.viewport.TotalLineCount()
	vis := m.viewport.Height()
	if vis <= 1 || total <= vis {
		return
	}
	if newOff, changed := m.vDrag.HandleClickAndClamp(localY, baseY, total, vis, m.viewport.YOffset()); changed {
		m.viewport.SetYOffset(newOff)
	}
}

// renderTitle returns the fixed "Response" prefix.
func (m Model) renderTitle() string {
	return styles.BoldStyle.Render("Response")
}

// renderMeta returns the status line with duration and size, or loading/error indicator.
func (m Model) renderMeta() string {
	if m.loading {
		return styles.MutedStyle.Render("sending...")
	}
	if m.err != nil {
		return lipgloss.NewStyle().Foreground(styles.ErrorColor).Render("Error")
	}
	if m.response == nil {
		return ""
	}
	resp := m.response
	proto := resp.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	statusStyle := styles.StatusCodeStyle(resp.StatusCode)
	return fmt.Sprintf("%s  %s  %s",
		statusStyle.Render(proto+" "+resp.Status),
		styles.MutedStyle.Render(fmt.Sprintf("%dms", resp.Duration.Milliseconds())),
		styles.MutedStyle.Render(formatSize(resp.Size)),
	)
}

func (m *Model) SetHideTitle(hide bool) {
	m.hideTitle = hide
}

func (m *Model) SetScreenSize(w, h int) {
	m.screenW = w
	m.screenH = h
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.SetWidth(w - 4) // border(2) + padding(1) + right margin(1)
	// base viewport height: content area (h-2) minus tabs(1) minus meta(1)
	vpH := h - 4
	if vpH < 1 {
		vpH = 1
	}
	m.viewport.SetHeight(vpH)
	// Re-wrap content for new width (also adjusts viewport height for scrollbar)
	m.refreshContent()

	if m.fieldPicker != nil {
		m.fieldPicker.SetSize(m.screenW, m.screenH)
	}
}

// updateViewportHeight adjusts viewport height to reserve space for the header rows
// and horizontal scrollbar when needed.
// hasMeta returns true when a meta line (status/loading/error) should be displayed.
func (m Model) hasMeta() bool {
	return m.loading || m.err != nil || m.response != nil
}

func (m *Model) updateViewportHeight() {
	// border(2) + tabs(1) + meta(1) = 4 lines reserved (title is on the border)
	// When no meta, only 3 lines reserved
	base := m.height - 4
	if !m.hasMeta() {
		base = m.height - 3
	}
	if m.HasScrollBar() {
		base-- // reserve 1 line for scrollbar
	}
	if base < 1 {
		base = 1
	}
	m.viewport.SetHeight(base)
}
