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
	"rest-helper/internal/ui/urlbar"
)

type Model struct {
	urlbar    urlbar.Model
	request   request.Model
	response  response.Model
	statusbar statusbar.Model
	help      help.Model

	historyPicker *historypicker.Model

	lastReq        *http.Request
	lastRawBody    string
	lastBodyFormat string

	focus  FocusPanel
	width  int
	height int
	ready  bool

	// Layout values
	reqH   int
	respH  int
	availH int
}

func New(version string) Model {
	m := Model{
		urlbar:    urlbar.New(),
		request:   request.New(),
		response:  response.New(),
		statusbar: statusbar.New(),
		help:      help.New(version),
		focus:     FocusURLBar,
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
		return m.handleMouseClick(msg)

	case tea.MouseReleaseMsg:
		if m.response.IsDragging() {
			m.response.StopDrag()
			return m, nil
		}
		if m.response.IsVDragging() {
			m.response.StopVDrag()
			return m, nil
		}
		return m, nil

	case tea.MouseMotionMsg:
		if m.response.IsDragging() {
			m.response.HandleScrollBarMouse(msg.X)
			return m, nil
		}
		if m.response.IsVDragging() {
			m.response.HandleVScrollBarMouse(msg.Y - m.response.VDragBaseY())
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
			hp, cmd := m.historyPicker.Update(msg)
			m.historyPicker = &hp
			return m, cmd
		}
		if m.response.FieldPickerVisible() {
			cmd := m.response.UpdateFieldPicker(msg)
			return m, cmd
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

	case tea.KeyPressMsg:
		// History picker overlay captures all input when visible
		if m.historyPicker != nil {
			hp, cmd := m.historyPicker.Update(msg)
			m.historyPicker = &hp
			return m, cmd
		}

		// Field picker overlay captures all input when visible
		if m.response.FieldPickerVisible() {
			cmd := m.response.UpdateFieldPicker(msg)
			return m, cmd
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
			if m.focus != FocusRequest {
				m.setFocus(FocusRequest)
			}
			return m, nil

		case key.Matches(msg, Keys.BodyTab):
			m.request.SetTab(request.TabBody)
			if m.focus != FocusRequest {
				m.setFocus(FocusRequest)
			}
			return m, nil

		case key.Matches(msg, Keys.AuthTab):
			m.request.SetTab(request.TabAuth)
			if m.focus != FocusRequest {
				m.setFocus(FocusRequest)
			}
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
		hp, cmd := m.historyPicker.Update(msg)
		m.historyPicker = &hp
		if cmd != nil {
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
		storage.SaveHistory(entry)
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
	hp := historypicker.New(entries, m.width, m.height)
	m.historyPicker = &hp
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
	layers := []*lipgloss.Layer{
		m.urlbar.ViewLayer(),
		m.request.ViewLayer().Y(1),
		m.response.ViewLayer().Y(1 + m.reqH),
		m.statusbar.ViewLayer().Y(1 + m.availH),
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
		hpv := m.historyPicker.View()
		layers = append(layers, lipgloss.NewLayer(hpv).
			ID("historypicker").Y(1).Z(50))
	}
	if m.response.FieldPickerVisible() {
		fp := m.response.ViewFieldPicker()
		x, y := centerOverlay(m.width, m.height, fp)
		layers = append(layers, lipgloss.NewLayer(fp).
			ID("fieldpicker").X(x).Y(y).Z(50))
	}
	if m.help.Visible {
		hv := m.help.View()
		x, y := centerOverlay(m.width, m.height, hv)
		layers = append(layers, lipgloss.NewLayer(hv).
			ID("help").X(x).Y(y).Z(50))
	}

	return lipgloss.NewCompositor(layers...)
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

	if hit.Empty() {
		return *m, nil
	}

	switch hit.ID() {
	// --- URL bar ---
	case "method-btn":
		m.setFocus(FocusURLBar)
		m.urlbar.ToggleSelect()
		return *m, nil
	case "send-btn":
		m.setFocus(FocusURLBar)
		return m.sendRequest()
	case "url-input", "url-hint", "urlbar":
		m.setFocus(FocusURLBar)

	// --- Status bar ---
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
	case "req-body-vscrollbar":
		m.setFocus(FocusRequest)
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
	case "resp-scrollbar":
		m.setFocus(FocusResponse)
		b := hit.Bounds()
		m.response.HandleScrollBarMouse(msg.X - b.Min.X)
		m.response.StartDrag()
	case "resp-vscrollbar":
		m.setFocus(FocusResponse)
		b := hit.Bounds()
		m.response.HandleVScrollBarMouse(msg.Y - b.Min.Y)
		m.response.StartVDrag(b.Min.Y)
	case "response":
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
}

func (m *Model) layout() {
	m.urlbar.SetWidth(m.width)
	m.statusbar.SetWidth(m.width)
	m.help.SetSize(m.width, m.height)

	availH := m.height - 2

	reqH := availH * 2 / 5
	respH := availH - reqH

	if reqH < 6 {
		reqH = 6
	}
	if respH < 5 {
		respH = 5
	}

	m.reqH = reqH
	m.respH = respH
	m.availH = availH

	m.request.SetSize(m.width, reqH)
	m.response.SetSize(m.width, respH)
	m.response.SetScreenSize(m.width, m.height)
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
