// SPDX-License-Identifier: MIT

package app

import (
	"encoding/json"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/http"
	"rest-helper/internal/storage"
	"rest-helper/internal/ui/help"
	"rest-helper/internal/ui/historypicker"
	"rest-helper/internal/ui/request"
	"rest-helper/internal/ui/response"
	"rest-helper/internal/ui/statusbar"
	"rest-helper/internal/ui/styles"
	"rest-helper/internal/ui/urlbar"
	"rest-helper/internal/ui/widget"
)

// LayoutMode controls how request and response panels are displayed.
type LayoutMode int

const (
	LayoutSplit LayoutMode = iota // both panels visible (default)
	LayoutFull                    // one panel at a time, tab to switch
)

type Model struct {
	urlbar    urlbar.Model
	request   request.Model
	response  response.Model
	statusbar statusbar.Model
	help      help.Model

	historyPicker *widget.OverlayPicker[storage.HistoryEntry]

	lastReq        *http.Request
	lastRawBody    string
	lastBodyFormat string

	focus  FocusPanel
	width  int
	height int
	ready  bool

	// Layout values
	reqH       int
	respH      int
	availH     int
	reqHDelta  int  // user adjustment to default request height
	borderDrag bool // mouse drag on request/response boundary

	layoutMode LayoutMode // split or fullscreen
	fullPanel  FocusPanel // which panel is shown in fullscreen (Request or Response)
}

func New(version string) Model {
	m := Model{
		urlbar:    urlbar.New(),
		request:   request.New(),
		response:  response.New(),
		statusbar: statusbar.New(),
		help:      help.New(version),
		focus:     FocusURLBar,
		fullPanel: FocusRequest,
	}

	// Restore persisted UI preferences.
	if v, _ := storage.GetSetting(storage.KeyBodyFormat); v != "" {
		m.request.SetBodyFormat(v)
	}
	if v, _ := storage.GetSetting(storage.KeyResponseFormat); v != "" {
		m.response.SetPreferredFormat(v)
	}
	if v, _ := storage.GetSetting(storage.KeyResponseWrap); v != "" {
		m.response.SetWrapMode(v == "true")
	}
	if v, _ := storage.GetSetting(storage.KeyLayoutMode); v == "full" {
		m.layoutMode = LayoutFull
	}
	m.applyLayoutMode()

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.urlbar.Init(),
		m.request.Init(),
		m.response.Init(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.layout()
		return m, nil

	case tea.MouseClickMsg:
		if m.help.Visible {
			m.help.Visible = false
			return m, nil
		}
		// Context menu captures all clicks when open
		if m.response.ContextMenuVisible() {
			cmd := m.response.HandleContextMenuClick(msg.X, msg.Y)
			return m, cmd
		}
		// Picker overlays capture all clicks when open
		if m.historyPicker != nil {
			hit := m.hitTest(msg.X, msg.Y)
			switch hit.ID() {
			case "hp-vscrollbar":
				b := hit.Bounds()
				m.historyPicker.HandleScrollClick(msg.Y, b.Min.Y)
			case "historypicker":
				b := hit.Bounds()
				if cmd := m.historyPicker.HandleClick(msg.Y - b.Min.Y); cmd != nil {
					return m, cmd
				}
			default:
				// Click outside picker closes it
				m.historyPicker = nil
			}
			return m, nil
		}
		if m.response.FieldPickerVisible() {
			hit := m.hitTest(msg.X, msg.Y)
			fp := m.response.GetFieldPicker()
			switch hit.ID() {
			case "fp-vscrollbar":
				b := hit.Bounds()
				fp.HandleScrollClick(msg.Y, b.Min.Y)
			case "fieldpicker":
				b := hit.Bounds()
				if cmd := fp.HandleClick(msg.Y - b.Min.Y); cmd != nil {
					return m, cmd
				}
			default:
				// Click outside picker closes it
				m.response.CloseFieldPicker()
			}
			return m, nil
		}
		return m.handleMouseClick(msg)

	case tea.MouseReleaseMsg:
		if m.borderDrag {
			m.borderDrag = false
			return m, nil
		}
		if m.request.StopDrag() {
			return m, nil
		}
		if m.response.StopDrag() {
			return m, nil
		}
		if m.historyPicker != nil && m.historyPicker.HandleMouseUp() {
			return m, nil
		}
		if fp := m.response.GetFieldPicker(); fp != nil && fp.HandleMouseUp() {
			return m, nil
		}
		return m, nil

	case tea.MouseMotionMsg:
		if m.borderDrag {
			// mouseY is the desired boundary row; boundary is at row reqH
			// (urlbar occupies row 0, request starts at row 1)
			newReqH := msg.Y
			if newReqH < minReqH {
				newReqH = minReqH
			}
			maxReqH := m.availH - minRespH
			if newReqH > maxReqH {
				newReqH = maxReqH
			}
			m.reqHDelta = newReqH - m.availH*2/5
			m.layout()
			return m, nil
		}
		if m.request.HandleDragMotion(msg.X, msg.Y) {
			return m, nil
		}
		if m.response.HandleDragMotion(msg.X, msg.Y) {
			return m, nil
		}
		if m.historyPicker != nil && m.historyPicker.HandleMouseMove(msg.Y) {
			return m, nil
		}
		if fp := m.response.GetFieldPicker(); fp != nil && fp.HandleMouseMove(msg.Y) {
			return m, nil
		}
		return m, nil

	case tea.MouseWheelMsg:
		if m.help.Visible {
			var cmd tea.Cmd
			m.help, cmd = m.help.Update(msg)
			return m, cmd
		}
		if m.historyPicker != nil {
			return m, m.historyPicker.Update(msg)
		}
		if m.response.FieldPickerVisible() {
			return m, m.response.UpdateFieldPicker(msg)
		}
		return m.handleMouseWheel(msg)

	case historypicker.HistorySelectedMsg:
		m.historyPicker = nil
		m.loadFromHistory(msg.Entry)
		return m, nil

	case historypicker.HistoryClosedMsg:
		m.historyPicker = nil
		return m, nil

	case historypicker.HistoryChangedMsg:
		return m, nil

	case response.FieldCopiedMsg:
		m.response.CloseFieldPicker()
		if msg.Error != nil {
			m.statusbar.SetText("Copy failed: " + msg.Error.Error())
		} else {
			m.statusbar.SetText("Copied: " + msg.Path)
		}
		return m, nil

	case response.FieldPickerClosedMsg:
		m.response.CloseFieldPicker()
		return m, nil

	case response.ContextMenuCopiedMsg:
		if msg.Error != nil {
			m.statusbar.SetText("Copy failed: " + msg.Error.Error())
		} else {
			display := msg.Value
			if len(display) > 40 {
				display = display[:40] + "…"
			}
			m.statusbar.SetText("Copied: " + display)
		}
		return m, nil

	case tea.KeyPressMsg:
		// Context menu captures all input when visible
		if m.response.ContextMenuVisible() {
			cmd := m.response.HandleContextMenuKey(msg)
			return m, cmd
		}

		// History picker overlay captures all input when visible
		if m.historyPicker != nil {
			return m, m.historyPicker.Update(msg)
		}

		// Field picker overlay captures all input when visible
		if m.response.FieldPickerVisible() {
			return m, m.response.UpdateFieldPicker(msg)
		}

		// Help overlay captures all input when visible
		if m.help.Visible {
			var cmd tea.Cmd
			m.help, cmd = m.help.Update(msg)
			return m, cmd
		}

		// Method dropdown captures all input when open
		if m.urlbar.SelectOpen() {
			if key.Matches(msg, Keys.Method) {
				m.urlbar.ToggleSelect()
				return m, nil
			}
			var cmd tea.Cmd
			m.urlbar, cmd = m.urlbar.Update(msg)
			return m, cmd
		}

		// Auth dropdown captures all input when open
		if m.request.AuthSelectOpen() {
			var cmd tea.Cmd
			m.request, cmd = m.request.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, Keys.Quit), key.Matches(msg, Keys.QuitAlt):
			return m, tea.Quit

		case key.Matches(msg, Keys.Help):
			// Don't intercept '?' when a text input is focused
			if msg.String() == "?" && (m.focus == FocusURLBar || m.focus == FocusRequest) {
				break
			}
			m.help.Toggle()
			return m, nil

		case key.Matches(msg, Keys.Send):
			return m.sendRequest()

		case key.Matches(msg, Keys.Method):
			m.urlbar.ToggleSelect()
			return m, nil

		case key.Matches(msg, Keys.LayoutToggle):
			m.toggleLayoutMode()
			return m, nil

		case key.Matches(msg, Keys.Tab):
			m.cycleFocus(1)
			return m, nil

		case key.Matches(msg, Keys.ShiftTab):
			m.cycleFocus(-1)
			return m, nil

		case key.Matches(msg, Keys.URLBar):
			m.setFocus(FocusURLBar)
			return m, nil

		case key.Matches(msg, Keys.HeaderTab):
			m.request.SetTab(request.TabHeaders)
			m.setFocus(FocusRequest)
			return m, nil

		case key.Matches(msg, Keys.BodyTab):
			m.request.SetTab(request.TabBody)
			m.setFocus(FocusRequest)
			return m, nil

		case key.Matches(msg, Keys.AuthTab):
			m.request.SetTab(request.TabAuth)
			m.setFocus(FocusRequest)
			return m, nil

		case key.Matches(msg, Keys.HistoryTab):
			m.openHistoryPicker()
			return m, nil

		case key.Matches(msg, Keys.ResponseBodyTab):
			m.setFocus(FocusResponse)
			m.response.SetTab(response.TabBody)
			return m, nil

		case key.Matches(msg, Keys.ResponseHeaderTab):
			m.setFocus(FocusResponse)
			m.response.SetTab(response.TabHeaders)
			return m, nil

		case key.Matches(msg, Keys.ResizeUp):
			if m.layoutMode == LayoutSplit {
				m.reqHDelta--
				m.layout()
			}
			return m, nil

		case key.Matches(msg, Keys.ResizeDown):
			if m.layoutMode == LayoutSplit {
				m.reqHDelta++
				m.layout()
			}
			return m, nil
		}

		// URL bar: up/down opens history picker
		if m.focus == FocusURLBar {
			switch msg.String() {
			case "up", "down":
				m.openHistoryPicker()
				return m, nil
			}
		}

	case http.ResponseMsg:
		if msg.Err != nil {
			m.response.SetError(msg.Err)
			m.statusbar.SetText("Error")
		} else {
			m.response.SetResponse(msg.Response)
			m.statusbar.SetText(fmt.Sprintf("%s  %dms",
				msg.Response.Status,
				msg.Response.Duration.Milliseconds(),
			))
			cmds = append(cmds, m.saveHistory(msg.Response))
		}
		return m, tea.Batch(cmds...)
	}

	// Forward to history picker if open
	if m.historyPicker != nil {
		if cmd := m.historyPicker.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	m.statusbar, _ = m.statusbar.Update(msg)

	switch m.focus {
	case FocusURLBar:
		var cmd tea.Cmd
		m.urlbar, cmd = m.urlbar.Update(msg)
		cmds = append(cmds, cmd)
	case FocusRequest:
		prevBodyFmt := m.request.GetBodyFormat()
		var cmd tea.Cmd
		m.request, cmd = m.request.Update(msg)
		cmds = append(cmds, cmd)
		if f := m.request.GetBodyFormat(); f != prevBodyFmt {
			_ = storage.SetSetting(storage.KeyBodyFormat, f)
		}
	case FocusResponse:
		prevFmt := m.response.GetPreferredFormat()
		prevWrap := m.response.GetWrapMode()
		var cmd tea.Cmd
		m.response, cmd = m.response.Update(msg)
		cmds = append(cmds, cmd)
		if f := m.response.GetPreferredFormat(); f != prevFmt {
			_ = storage.SetSetting(storage.KeyResponseFormat, f)
		}
		if w := m.response.GetWrapMode(); w != prevWrap {
			_ = storage.SetSetting(storage.KeyResponseWrap, fmt.Sprintf("%t", w))
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) sendRequest() (Model, tea.Cmd) {
	url := m.urlbar.URL()
	if url == "" {
		return *m, nil
	}

	headers := m.request.GetHeaders()
	body, err := m.request.GetBody()
	if err != nil {
		m.response.SetError(err)
		m.statusbar.SetText("Body error")
		return *m, nil
	}

	m.lastRawBody = m.request.GetRawBody()
	m.lastBodyFormat = m.request.GetBodyFormat()

	req := http.Request{
		Method:  m.urlbar.Method(),
		URL:     url,
		Headers: headers,
		Body:    body,
	}
	m.lastReq = &req

	m.response.SetLoading()
	m.statusbar.SetText("Sending...")
	return *m, http.Send(req)
}

func (m *Model) saveHistory(resp *http.Response) tea.Cmd {
	if m.lastReq == nil {
		return nil
	}

	reqHeaders, _ := json.Marshal(m.lastReq.Headers)
	respHeaders, _ := json.Marshal(resp.Headers)

	entry := &storage.HistoryEntry{
		Method:          m.lastReq.Method,
		URL:             m.lastReq.URL,
		RequestHeaders:  string(reqHeaders),
		RequestBody:     m.lastRawBody,
		BodyFormat:      m.lastBodyFormat,
		StatusCode:      resp.StatusCode,
		ResponseProto:   resp.Proto,
		ResponseStatus:  resp.Status,
		ResponseHeaders: string(respHeaders),
		ResponseBody:    resp.Body,
		ResponseTimeMs:  resp.Duration.Milliseconds(),
		ResponseSize:    resp.Size,
	}

	return func() tea.Msg {
		_ = storage.SaveHistory(entry)
		return nil
	}
}

func (m *Model) loadFromHistory(entry storage.HistoryEntry) {
	m.urlbar.SetMethod(entry.Method)
	m.urlbar.SetURL(entry.URL)

	headers := storage.HeadersFromJSON(entry.RequestHeaders)
	m.request.SetHeaders(headers)
	m.request.SetBodyFormat(entry.BodyFormat)
	m.request.SetBody(entry.RequestBody)

	if entry.StatusCode > 0 {
		var respHeaders map[string][]string
		_ = json.Unmarshal([]byte(entry.ResponseHeaders), &respHeaders) // best-effort
		status := entry.ResponseStatus
		if status == "" {
			status = fmt.Sprintf("%d", entry.StatusCode)
		}
		proto := entry.ResponseProto
		if proto == "" {
			proto = "HTTP/1.1"
		}
		m.response.SetResponse(&http.Response{
			StatusCode: entry.StatusCode,
			Status:     status,
			Proto:      proto,
			Headers:    respHeaders,
			Body:       entry.ResponseBody,
			Duration:   time.Duration(entry.ResponseTimeMs) * time.Millisecond,
			Size:       entry.ResponseSize,
		})
	}
}

func (m *Model) openHistoryPicker() {
	entries, _ := storage.ListHistory(200)
	m.historyPicker = historypicker.New(entries, m.width, m.height)
}

// centerOverlay returns X, Y to center the rendered overlay within the screen.
func centerOverlay(screenW, screenH int, rendered string) (int, int) {
	x := (screenW - lipgloss.Width(rendered)) / 2
	y := (screenH - lipgloss.Height(rendered)) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

// buildCompositor creates a Compositor from the current UI state.
func (m Model) buildCompositor() *lipgloss.Compositor {
	// Base UI layers
	var layers []*lipgloss.Layer

	if m.layoutMode == LayoutFull {
		layers = append(layers, m.urlbar.ViewLayer())
		// Only show the active panel
		if m.fullPanel == FocusRequest {
			layers = append(layers, m.request.ViewLayer().Y(1))
		} else {
			layers = append(layers, m.response.ViewLayer().Y(1))
		}
		layers = append(layers, m.statusbar.ViewLayer().Y(1+m.availH))

		// Panel switch tabs on the border
		layers = append(layers, m.fullscreenTabLayers()...)
	} else {
		layers = append(layers,
			m.urlbar.ViewLayer(),
			m.request.ViewLayer().Y(1),
			m.response.ViewLayer().Y(1+m.reqH),
			m.statusbar.ViewLayer().Y(1+m.availH),
		)
	}

	// Dropdown overlays (Z=5, above clickable children at Z=1)
	if m.urlbar.SelectOpen() {
		layers = append(layers, lipgloss.NewLayer(m.urlbar.DropdownView()).
			ID("method-dropdown").Y(1).Z(5))
	}
	if m.request.AuthSelectOpen() {
		layers = append(layers, lipgloss.NewLayer(m.request.AuthDropdownView()).
			ID("auth-dropdown").X(3).Y(5).Z(5))
	}

	// Full-screen overlays (Z=50)
	if m.historyPicker != nil {
		hpLayer := m.historyPicker.BuildLayer()
		hpLayer.ID("historypicker").Y(1).Z(50)
		layers = append(layers, hpLayer)
	}
	if m.response.FieldPickerVisible() {
		fpLayer := m.response.BuildFieldPickerLayer()
		x, y := centerOverlay(m.width, m.height, fpLayer.GetContent())
		fpLayer.ID("fieldpicker").X(x).Y(y).Z(50)
		layers = append(layers, fpLayer)
	}
	if m.response.ContextMenuVisible() {
		layers = append(layers, m.response.BuildContextMenuLayer())
	}
	if m.help.Visible {
		hv := m.help.View()
		x, y := centerOverlay(m.width, m.height, hv)
		layers = append(layers, lipgloss.NewLayer(hv).
			ID("help").X(x).Y(y).Z(50))
	}

	return lipgloss.NewCompositor(layers...)
}

// fullscreenTabLayers renders Request/Response tabs on the panel border in fullscreen mode.
func (m Model) fullscreenTabLayers() []*lipgloss.Layer {
	reqLabel := " Request "
	respLabel := " Response "

	var reqRendered, respRendered string
	if m.fullPanel == FocusRequest {
		reqRendered = styles.BoldStyle.Render(reqLabel)
		respRendered = styles.MutedStyle.Render(respLabel)
	} else {
		reqRendered = styles.MutedStyle.Render(reqLabel)
		respRendered = styles.BoldStyle.Render(respLabel)
	}

	x := 2
	reqLayer := lipgloss.NewLayer(reqRendered).ID("full-tab-request").X(x).Y(1).Z(2)
	x += lipgloss.Width(reqRendered) + 1
	respLayer := lipgloss.NewLayer(respRendered).ID("full-tab-response").X(x).Y(1).Z(2)

	return []*lipgloss.Layer{reqLayer, respLayer}
}

// hitTest performs a hit test by building a compositor from the current state.
func (m Model) hitTest(x, y int) lipgloss.LayerHit {
	return m.buildCompositor().Hit(x, y)
}

func (m *Model) handleMouseClick(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	hit := m.hitTest(msg.X, msg.Y)

	// Dropdowns capture all clicks when open: hit selects, miss closes.
	if m.urlbar.SelectOpen() {
		if hit.ID() == "method-dropdown" {
			b := hit.Bounds()
			m.urlbar.ClickDropdown(msg.Y-b.Min.Y, msg.X-b.Min.X)
		}
		if m.urlbar.SelectOpen() {
			m.urlbar.ToggleSelect()
		}
		return *m, nil
	}
	if m.request.AuthSelectOpen() {
		if hit.ID() == "auth-dropdown" {
			b := hit.Bounds()
			m.request.AuthClickDropdown(msg.Y-b.Min.Y, msg.X-b.Min.X)
		}
		if m.request.AuthSelectOpen() {
			m.request.AuthToggleSelect()
		}
		return *m, nil
	}

	// Boundary drag: click on the border between request and response (split mode only)
	if m.layoutMode == LayoutSplit && (msg.Y == m.reqH || msg.Y == 1+m.reqH) {
		m.borderDrag = true
	}

	if hit.Empty() {
		return *m, nil
	}

	switch hit.ID() {
	// --- Fullscreen panel tabs ---
	case "full-tab-request":
		m.fullPanel = FocusRequest
		m.setFocus(FocusRequest)
		return *m, nil
	case "full-tab-response":
		m.fullPanel = FocusResponse
		m.setFocus(FocusResponse)
		return *m, nil

	// --- URL bar ---
	case "method-btn":
		m.setFocus(FocusURLBar)
		m.urlbar.ToggleSelect()
		return *m, nil
	case "send-btn":
		m.setFocus(FocusURLBar)
		return m.sendRequest()
	case "history-btn":
		m.openHistoryPicker()
		return *m, nil
	case "url-input", "url-hint", "urlbar":
		m.setFocus(FocusURLBar)

	// --- Status bar ---
	case "layout-btn":
		m.toggleLayoutMode()
		return *m, nil
	case "help-btn":
		m.help.Toggle()
	case "statusbar":
		// no action

	// --- Request panel ---
	case "req-tab-body":
		m.setFocus(FocusRequest)
		m.request.SetTab(request.TabBody)
	case "req-tab-headers":
		m.setFocus(FocusRequest)
		m.request.SetTab(request.TabHeaders)
	case "req-tab-auth":
		m.setFocus(FocusRequest)
		m.request.SetTab(request.TabAuth)
	case "req-format-toggle":
		m.setFocus(FocusRequest)
		m.request.ToggleBodyFormat()
		_ = storage.SetSetting(storage.KeyBodyFormat, m.request.GetBodyFormat())
	case "req-auth-type-btn":
		m.setFocus(FocusRequest)
		m.request.AuthToggleSelect()
	case "req-visibility-hint":
		m.setFocus(FocusRequest)
		m.request.ToggleTokenVisibility()
	case "req-wrap-toggle":
		m.setFocus(FocusRequest)
		m.request.ToggleBodyWrap()
	case "req-body-vscrollbar":
		m.setFocus(FocusRequest)
		b := hit.Bounds()
		m.request.HandleBodyVScrollClick(msg.Y-b.Min.Y, b.Min.Y)
	case "req-body-hscrollbar":
		m.setFocus(FocusRequest)
		b := hit.Bounds()
		m.request.HandleBodyHScrollClick(msg.X-b.Min.X, b.Min.X)
	case "request":
		m.setFocus(FocusRequest)

	// --- Response panel ---
	case "resp-tab-body":
		m.setFocus(FocusResponse)
		m.response.SetTab(response.TabBody)
	case "resp-tab-headers":
		m.setFocus(FocusResponse)
		m.response.SetTab(response.TabHeaders)
	case "resp-format-toggle":
		m.setFocus(FocusResponse)
		m.response.ToggleFormat()
		_ = storage.SetSetting(storage.KeyResponseFormat, m.response.GetPreferredFormat())
	case "resp-wrap-toggle":
		m.setFocus(FocusResponse)
		m.response.ToggleWrap()
		_ = storage.SetSetting(storage.KeyResponseWrap, fmt.Sprintf("%t", m.response.GetWrapMode()))
	case "resp-copy-btn":
		m.setFocus(FocusResponse)
		m.response.OpenFieldPicker()
	case "resp-scrollbar":
		m.setFocus(FocusResponse)
		b := hit.Bounds()
		m.response.HandleHScrollClick(msg.X-b.Min.X, b.Min.X)
	case "resp-vscrollbar":
		m.setFocus(FocusResponse)
		b := hit.Bounds()
		m.response.HandleVScrollClick(msg.Y-b.Min.Y, b.Min.Y)
	case "response":
		if msg.Button == tea.MouseRight {
			m.setFocus(FocusResponse)
			b := hit.Bounds()
			m.response.OpenContextMenu(msg.X, msg.Y, b.Min.Y)
			return *m, nil
		}
		m.setFocus(FocusResponse)
	}

	return *m, nil
}

func (m *Model) handleMouseWheel(msg tea.MouseWheelMsg) (Model, tea.Cmd) {
	hit := m.hitTest(msg.X, msg.Y)

	switch hit.ID() {
	case "request":
		m.request.HandleWheel(msg)
	case "response":
		m.response.HandleWheel(msg)
	}

	return *m, nil
}

func (m *Model) cycleFocus(dir int) {
	panels := []FocusPanel{FocusURLBar, FocusRequest, FocusResponse}
	current := 0
	for i, p := range panels {
		if p == m.focus {
			current = i
			break
		}
	}

	next := (current + dir + len(panels)) % len(panels)
	m.setFocus(panels[next])
}

func (m *Model) setFocus(panel FocusPanel) {
	m.urlbar.Blur()
	m.request.Blur()
	m.response.Blur()

	m.focus = panel
	switch panel {
	case FocusURLBar:
		m.urlbar.Focus()
	case FocusRequest:
		m.request.Focus()
	case FocusResponse:
		m.response.Focus()
	}

	// In fullscreen mode, keep the visible panel in sync with focus.
	if m.layoutMode == LayoutFull && (panel == FocusRequest || panel == FocusResponse) {
		m.fullPanel = panel
	}
}

const (
	minReqH  = 8
	minRespH = 6
)

func (m *Model) clampDelta() {
	availH := m.height - 2
	base := availH * 2 / 5
	minDelta := minReqH - base
	maxDelta := availH - minRespH - base
	if m.reqHDelta < minDelta {
		m.reqHDelta = minDelta
	}
	if m.reqHDelta > maxDelta {
		m.reqHDelta = maxDelta
	}
}

func (m *Model) layout() {
	m.urlbar.SetWidth(m.width)
	m.statusbar.SetWidth(m.width)
	m.help.SetSize(m.width, m.height)

	availH := m.height - 2
	m.availH = availH
	m.response.SetScreenSize(m.width, m.height)

	if m.layoutMode == LayoutFull {
		// Fullscreen: one panel takes all available height
		m.reqH = availH
		m.respH = availH
		m.request.SetSize(m.width, availH)
		m.response.SetSize(m.width, availH)
	} else {
		m.clampDelta()
		reqH := availH*2/5 + m.reqHDelta
		respH := availH - reqH
		m.reqH = reqH
		m.respH = respH
		m.request.SetSize(m.width, reqH)
		m.response.SetSize(m.width, respH)
	}

	if m.historyPicker != nil {
		m.historyPicker.SetSize(m.width, m.height)
	}
}

// applyLayoutMode updates panel state to match the current layout mode.
func (m *Model) applyLayoutMode() {
	isFull := m.layoutMode == LayoutFull
	m.request.SetHideTitle(isFull)
	m.response.SetHideTitle(isFull)
	if isFull {
		m.statusbar.SetLayoutLabel("Full")
	} else {
		m.statusbar.SetLayoutLabel("Split")
	}
}

// toggleLayoutMode switches between split and fullscreen modes.
func (m *Model) toggleLayoutMode() {
	if m.layoutMode == LayoutSplit {
		m.layoutMode = LayoutFull
		// Show the currently focused panel, or default to request
		if m.focus == FocusRequest || m.focus == FocusResponse {
			m.fullPanel = m.focus
		}
	} else {
		m.layoutMode = LayoutSplit
	}
	m.applyLayoutMode()
	m.layout()

	modeStr := "split"
	if m.layoutMode == LayoutFull {
		modeStr = "full"
	}
	_ = storage.SetSetting(storage.KeyLayoutMode, modeStr)
}

func (m Model) View() tea.View {
	if !m.ready {
		return tea.NewView("Loading...")
	}

	comp := m.buildCompositor()
	canvas := lipgloss.NewCanvas(m.width, m.height)
	canvas.Compose(comp)

	v := tea.NewView(canvas.Render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}
