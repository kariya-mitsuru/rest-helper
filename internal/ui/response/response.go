// SPDX-License-Identifier: MIT

package response

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"gopkg.in/yaml.v3"

	"rest-helper/internal/http"
	"rest-helper/internal/ui/styles"
)

type displayFormat int

const (
	formatJSON displayFormat = iota
	formatYAML
)

type responseTab int

const (
	TabBody responseTab = iota
	TabHeaders
)

var respTabsConfig = []styles.TabDef{
	{"Body", "R", int(TabBody)},
	{"Headers", "D", int(TabHeaders)},
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
	preferredFormat displayFormat // user's preferred format for JSON responses
	wrapMode        bool          // true=wrap, false=horizontal scroll
	xOffset         int           // horizontal scroll position
	rawLines        []string      // pre-wrap content lines for horizontal scroll
	maxLineWidth    int           // max visible width among rawLines
	dragging        bool          // true while dragging the horizontal scrollbar
	fieldPicker     *FieldPickerModel
	activeTab       responseTab
	screenW         int // full terminal width (for overlay sizing)
	screenH         int // full terminal height
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
	// Use preferred format if the body is JSON, otherwise fall back to JSON (raw)
	m.display = formatJSON
	if json.Valid([]byte(resp.Body)) {
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
	if m.response == nil || m.response.Body == "" {
		return false
	}
	return json.Valid([]byte(m.response.Body))
}

// refreshContent re-renders the response body with the current viewport width.
func (m *Model) refreshContent() {
	if m.response == nil {
		return
	}

	// Get syntax-highlighted content WITHOUT wrapping
	var raw string
	if m.activeTab == TabHeaders {
		raw = m.formatHeaders()
	} else if m.display == formatYAML && m.isBodyJSON() {
		raw = formatAsYAML(m.response.Body, 0)
	} else {
		raw = formatBody(m.response.Body, 0)
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

// ToggleFormat switches between JSON and YAML display for JSON responses.
func (m *Model) ToggleFormat() {
	if !m.isBodyJSON() {
		return
	}
	if m.display == formatJSON {
		m.display = formatYAML
	} else {
		m.display = formatJSON
	}
	m.preferredFormat = m.display
	m.refreshContent()
}

// formatHeaders returns a colorized Key: Value display of response headers.
func (m *Model) formatHeaders() string {
	if m.response == nil || len(m.response.Headers) == 0 {
		return styles.MutedStyle.Render("(no headers)")
	}

	var b strings.Builder

	keys := make([]string, 0, len(m.response.Headers))
	for k := range m.response.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

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
func (m *Model) SetTab(tab responseTab) {
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

func (m *Model) openFieldPicker() {
	fp := NewFieldPicker(m.response.Body, m.screenW, m.screenH)
	m.fieldPicker = &fp
}

// UpdateFieldPicker forwards a message to the field picker.
// The caller must check FieldPickerVisible() before calling this.
func (m *Model) UpdateFieldPicker(msg tea.Msg) tea.Cmd {
	fp, cmd := m.fieldPicker.Update(msg)
	m.fieldPicker = &fp
	return cmd
}

// ViewFieldPicker renders the field picker overlay.
// The caller must check FieldPickerVisible() before calling this.
func (m Model) ViewFieldPicker() string {
	return m.fieldPicker.View()
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

// GetPreferredFormat returns the current preferred format as "JSON" or "YAML".
func (m Model) GetPreferredFormat() string {
	if m.preferredFormat == formatYAML {
		return "YAML"
	}
	return "JSON"
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
			if m.activeTab == TabBody {
				m.ToggleFormat()
				return m, nil
			}
		case "y":
			if m.activeTab == TabBody && m.response != nil && m.response.Body != "" {
				m.openFieldPicker()
				return m, nil
			}
		case "ctrl+w":
			m.ToggleWrap()
			return m, nil
		case "home":
			m.viewport.GotoTop()
			return m, nil
		case "end":
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
	borderStyle := styles.NormalBorder
	if m.focused {
		borderStyle = styles.FocusedBorder
	}

	line0 := m.renderTitle()

	parts := []string{line0}
	if meta := m.renderMeta(); meta != "" {
		parts = append(parts, meta)
	}
	parts = append(parts, m.viewport.View())

	if !m.wrapMode && m.maxLineWidth > m.contentWidth() {
		parts = append(parts, m.renderHScrollBar())
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	full := borderStyle.Width(m.width).Height(m.height).Render(content)

	// Tab buttons as initial child layers (Y=1 inside border)
	titleW := lipgloss.Width(line0)
	x := 1 + titleW + 2 // border(1) + title + gap(2)
	children, x := styles.RenderTabLayers(respTabsConfig, int(m.activeTab), "resp-tab-", x, 1)

	if m.response != nil {
		// Format toggle (or placeholder gap)
		if m.activeTab == TabBody && m.isBodyJSON() {
			label := "JSON"
			if m.display == formatYAML {
				label = "YAML"
			}
			rendered := styles.ActiveTab.Render(label)
			children = append(children, lipgloss.NewLayer(rendered).
				ID("resp-format-toggle").
				X(x).Y(1).Z(1))
		}
		x += 4 + 2 // placeholder + gap

		// Wrap/Scroll toggle
		wrapLabel := "Scroll"
		if m.wrapMode {
			wrapLabel = " Wrap "
		}
		wrapRendered := styles.ActiveTab.Render(wrapLabel)
		children = append(children, lipgloss.NewLayer(wrapRendered).
			ID("resp-wrap-toggle").
			X(x).Y(1).Z(1))

		// Scrollbar
		if !m.wrapMode && m.maxLineWidth > m.contentWidth() {
			sb := m.renderHScrollBar()
			sbY := m.ScrollBarRelY()
			children = append(children, lipgloss.NewLayer(sb).
				ID("resp-scrollbar").
				X(1).Y(sbY).Z(1))
		}
	}

	return lipgloss.NewLayer(full, children...).ID("response")
}

// renderHScrollBar renders a horizontal scrollbar indicating xOffset position.
func (m Model) renderHScrollBar() string {
	trackW := m.contentWidth()
	if trackW <= 0 {
		return ""
	}

	totalW := m.maxLineWidth
	viewW := m.contentWidth()

	// Thumb size: proportional to visible fraction
	thumbW := trackW * viewW / totalW
	if thumbW < 1 {
		thumbW = 1
	}
	if thumbW > trackW {
		thumbW = trackW
	}

	// Thumb position
	maxOff := m.maxXOffset()
	thumbPos := 0
	if maxOff > 0 {
		thumbPos = (trackW - thumbW) * m.xOffset / maxOff
	}
	if thumbPos+thumbW > trackW {
		thumbPos = trackW - thumbW
	}

	trackStyle := lipgloss.NewStyle().Foreground(styles.BorderColor)
	thumbStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)

	var b strings.Builder
	for i := 0; i < trackW; i++ {
		if i >= thumbPos && i < thumbPos+thumbW {
			b.WriteString(thumbStyle.Render("━"))
		} else {
			b.WriteString(trackStyle.Render("─"))
		}
	}
	return b.String()
}

// HasScrollBar returns true when the horizontal scrollbar is visible.
func (m Model) HasScrollBar() bool {
	return !m.wrapMode && m.maxLineWidth > m.contentWidth()
}

// ScrollBarRelY returns the scrollbar's row relative to the response panel top.
// Returns -1 if scrollbar is not visible.
func (m Model) ScrollBarRelY() int {
	if !m.HasScrollBar() {
		return -1
	}
	headerRows := 3 // border(1) + title/tabs(1) + meta(1)
	if lipgloss.Width(m.renderMeta()) == 0 {
		headerRows = 2 // border(1) + title/tabs(1)
	}
	return headerRows + m.viewport.Height()
}

// HandleScrollBarMouse maps a click/drag column position on the scrollbar track to xOffset.
func (m *Model) HandleScrollBarMouse(col int) {
	trackW := m.contentWidth()
	if trackW <= 0 {
		return
	}
	maxOff := m.maxXOffset()
	// Map col to xOffset: col 0 → offset 0, col trackW-1 → offset maxOff
	newOffset := maxOff * col / trackW
	if newOffset < 0 {
		newOffset = 0
	}
	if newOffset > maxOff {
		newOffset = maxOff
	}
	m.xOffset = newOffset
	m.applyXOffset()
}

// StartDrag marks the scrollbar as being dragged.
func (m *Model) StartDrag() {
	m.dragging = true
}

// StopDrag ends a scrollbar drag.
func (m *Model) StopDrag() {
	m.dragging = false
}

// IsDragging returns whether the scrollbar is currently being dragged.
func (m Model) IsDragging() bool {
	return m.dragging
}

// renderTitle returns the fixed "Response" prefix.
func (m Model) renderTitle() string {
	return lipgloss.NewStyle().Bold(true).Render("Response")
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

func (m *Model) SetScreenSize(w, h int) {
	m.screenW = w
	m.screenH = h
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.SetWidth(w - 4)
	// base viewport height: content area (h-2) minus header (1) minus tabs (1)
	vpH := h - 4
	if vpH < 1 {
		vpH = 1
	}
	m.viewport.SetHeight(vpH)
	// Re-wrap content for new width (also adjusts viewport height for scrollbar)
	m.refreshContent()

	if m.fieldPicker != nil {
		m.fieldPicker.width = w
		m.fieldPicker.height = h
		m.fieldPicker.filter.SetWidth(w - 6)
	}
}

// updateViewportHeight adjusts viewport height to reserve space for the header rows
// and horizontal scrollbar when needed.
func (m *Model) updateViewportHeight() {
	// border(2) + title/tabs(1) + meta(1) = 4 lines reserved
	// When no meta, only 3 lines reserved
	metaW := lipgloss.Width(m.renderMeta())
	base := m.height - 4
	if metaW == 0 {
		base = m.height - 3
	}
	if !m.wrapMode && m.maxLineWidth > m.contentWidth() {
		base-- // reserve 1 line for scrollbar
	}
	if base < 1 {
		base = 1
	}
	m.viewport.SetHeight(base)
}

// Shared color palette for syntax highlighting.
var (
	hlKeyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#06B6D4"))
	hlStrStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	hlNumStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	hlBoolStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5CF6"))
	hlNullStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	hlPunctStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
)

// --- Formatting ---

func formatBody(body string, width int) string {
	if body == "" {
		return styles.MutedStyle.Render("(empty response)")
	}

	// Try to pretty-print JSON
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(body), "", "  "); err == nil {
		return syntaxHighlight(buf.String(), width)
	}

	// Non-JSON: ANSI-aware wrap
	if width > 0 {
		return ansi.Hardwrap(body, width, false)
	}
	return body
}

func formatAsYAML(body string, width int) string {
	var data any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return formatBody(body, width)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(data); err != nil {
		return formatBody(body, width)
	}

	return syntaxHighlightYAML(buf.String(), width)
}

func syntaxHighlightYAML(yamlStr string, width int) string {
	var result strings.Builder

	lines := strings.Split(strings.TrimRight(yamlStr, "\n"), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := line[:len(line)-len(trimmed)]
		result.WriteString(indent)

		if strings.HasPrefix(trimmed, "- ") {
			// List item
			result.WriteString(hlPunctStyle.Render("- "))
			rest := trimmed[2:]
			if strings.Contains(rest, ": ") {
				result.WriteString(colorizeYAMLKeyValue(rest, hlKeyStyle, hlStrStyle, hlNumStyle, hlBoolStyle, hlNullStyle, hlPunctStyle))
			} else {
				result.WriteString(colorizeYAMLValue(rest, hlStrStyle, hlNumStyle, hlBoolStyle, hlNullStyle))
			}
		} else if strings.Contains(trimmed, ": ") {
			result.WriteString(colorizeYAMLKeyValue(trimmed, hlKeyStyle, hlStrStyle, hlNumStyle, hlBoolStyle, hlNullStyle, hlPunctStyle))
		} else if strings.HasSuffix(trimmed, ":") {
			// Key with no value (nested object/array follows)
			result.WriteString(hlKeyStyle.Render(strings.TrimSuffix(trimmed, ":")))
			result.WriteString(hlPunctStyle.Render(":"))
		} else {
			result.WriteString(colorizeYAMLValue(trimmed, hlStrStyle, hlNumStyle, hlBoolStyle, hlNullStyle))
		}

		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	if width > 0 {
		return ansi.Hardwrap(result.String(), width, false)
	}
	return result.String()
}

func colorizeYAMLKeyValue(s string, keyStyle, strStyle, numStyle, boolStyle, nullStyle, punctStyle lipgloss.Style) string {
	parts := strings.SplitN(s, ": ", 2)
	key := parts[0]
	val := ""
	if len(parts) > 1 {
		val = parts[1]
	}
	return keyStyle.Render(key) + punctStyle.Render(": ") + colorizeYAMLValue(val, strStyle, numStyle, boolStyle, nullStyle)
}

func colorizeYAMLValue(val string, strStyle, numStyle, boolStyle, nullStyle lipgloss.Style) string {
	switch {
	case val == "null" || val == "~":
		return nullStyle.Render(val)
	case val == "true" || val == "false":
		return boolStyle.Render(val)
	case len(val) > 0 && (val[0] >= '0' && val[0] <= '9' || val[0] == '-' || val[0] == '.'):
		return numStyle.Render(val)
	case strings.HasPrefix(val, "'") || strings.HasPrefix(val, "\""):
		return strStyle.Render(val)
	case val == "[]" || val == "{}":
		return nullStyle.Render(val)
	default:
		return strStyle.Render(val)
	}
}

func syntaxHighlight(jsonStr string, width int) string {
	var result strings.Builder

	// Colorize on original (unwrapped) lines
	lines := strings.Split(jsonStr, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := line[:len(line)-len(trimmed)]
		result.WriteString(indent)

		if strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			key := strings.TrimSpace(parts[0])
			val := ""
			if len(parts) > 1 {
				val = strings.TrimSpace(parts[1])
			}
			result.WriteString(hlKeyStyle.Render(key))
			result.WriteString(hlPunctStyle.Render(": "))
			result.WriteString(colorizeValue(val, hlStrStyle, hlNumStyle, hlBoolStyle, hlNullStyle, hlPunctStyle))
		} else {
			result.WriteString(colorizeValue(trimmed, hlStrStyle, hlNumStyle, hlBoolStyle, hlNullStyle, hlPunctStyle))
		}

		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	// ANSI-aware wrap preserves color across broken lines
	// Wrap (not Wordwrap) also force-breaks lines with no spaces
	if width > 0 {
		return ansi.Hardwrap(result.String(), width, false)
	}
	return result.String()
}

func colorizeValue(val string, strStyle, numStyle, boolStyle, nullStyle, punctStyle lipgloss.Style) string {
	cleaned := strings.TrimSuffix(val, ",")
	trailing := ""
	if strings.HasSuffix(val, ",") {
		trailing = punctStyle.Render(",")
	}

	switch {
	case cleaned == "{" || cleaned == "}" || cleaned == "[" || cleaned == "]" ||
		cleaned == "{}" || cleaned == "[]":
		return punctStyle.Render(cleaned) + trailing
	case cleaned == "null":
		return nullStyle.Render(cleaned) + trailing
	case cleaned == "true" || cleaned == "false":
		return boolStyle.Render(cleaned) + trailing
	case strings.HasPrefix(cleaned, "\""):
		return strStyle.Render(cleaned) + trailing
	case len(cleaned) > 0 && (cleaned[0] >= '0' && cleaned[0] <= '9' || cleaned[0] == '-'):
		return numStyle.Render(cleaned) + trailing
	default:
		return val
	}
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
