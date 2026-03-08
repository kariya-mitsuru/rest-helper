// SPDX-License-Identifier: MIT

package widget

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
)

// PickerAction indicates what the OverlayPicker should do after a key handler.
type PickerAction int

const (
	// PickerNotHandled means the key was not consumed by the content.
	PickerNotHandled PickerAction = iota
	// PickerHandled means the key was consumed with no further action.
	PickerHandled
	// PickerMoveDown means the key was consumed and the cursor should move down.
	PickerMoveDown
)

// PickerContent defines the content-specific behavior for an OverlayPicker.
type PickerContent[T any] interface {
	// MatchFilter reports whether the item matches the query.
	// The query is already lowercased and guaranteed to be non-empty.
	MatchFilter(item T, query string) bool

	// RenderItem renders an item. isCursor indicates the cursor position.
	RenderItem(item T, isCursor bool, width int) string

	// Title returns the rendered title string.
	Title(filtered, total int) string

	// Footer returns the rendered footer string.
	Footer(filtered, total int) string

	// EmptyText returns the text shown when no items match the filter.
	EmptyText() string

	// HandleKey handles a key press not consumed by navigation.
	// item is the item under the cursor (nil if none).
	HandleKey(key string, item *T) (tea.Cmd, PickerAction)

	// HandleMsg handles non-key/non-wheel messages.
	// If newItems is non-nil, the picker replaces its items and refilters.
	HandleMsg(msg tea.Msg) (cmd tea.Cmd, newItems []T, handled bool)

	// HelpContent returns help key bindings to display in the picker.
	// Return nil if this picker has no help.
	HelpContent() [][2]string
}

// OverlayPicker provides a complete popup picker overlay driven by a
// PickerContent implementation. It owns the items, filter input, navigation,
// scrollbar, resize, and rendering.
type OverlayPicker[T any] struct {
	content      PickerContent[T]
	items        []T
	filtered     []int
	contentW     int
	innerW       int
	fixedVis     int
	heightOffset int
	scrollbarID  string
	filter       textinput.Model
	width        int
	height       int
	vDrag        ScrollDrag
	showHelp     bool

	ListNav
}

// NewOverlayPicker creates a fully initialized overlay picker.
// heightOffset is the number of rows subtracted from terminal height to compute
// visible list rows (border, title, filter, blanks, footer, and any Y offset).
func NewOverlayPicker[T any](content PickerContent[T], items []T, placeholder string, width, height, heightOffset, contentWidth int, scrollbarID string) *OverlayPicker[T] {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = "/ "
	ti.SetWidth(width - 7) // border(2) + padding(2) + prompt(2) + cursor(1)
	ti.Focus()

	p := &OverlayPicker[T]{
		content:      content,
		items:        items,
		filter:       ti,
		width:        width,
		height:       height,
		heightOffset: heightOffset,
		scrollbarID:  scrollbarID,
		contentW:     contentWidth,
	}
	p.reclampInnerW()
	p.applyFilter()
	p.fixedVis = p.calcVisibleRows()
	return p
}

// SetSize updates dimensions after a terminal resize.
func (p *OverlayPicker[T]) SetSize(width, height int) {
	p.width = width
	p.height = height
	p.reclampInnerW()
	p.fixedVis = p.calcVisibleRows()
	p.ListNav.Clamp(len(p.filtered), p.fixedVis)
}

// HandleClick handles a mouse click on the picker overlay.
// localY is the Y position relative to the overlay's top-left corner.
// Returns a tea.Cmd if an item was selected (e.g. enter action), nil otherwise.
func (p *OverlayPicker[T]) HandleClick(localY int) tea.Cmd {
	// Items start at: border(1) + title(1) + filter(1) + blank(1) = 4
	itemIdx := localY - 4
	vis := p.visibleRows()
	if itemIdx < 0 || itemIdx >= vis {
		return nil
	}
	absIdx := p.Scroll + itemIdx
	if absIdx >= len(p.filtered) {
		return nil
	}
	// Move cursor to clicked item and trigger enter
	p.Cursor = absIdx
	cmd, _ := p.content.HandleKey("enter", p.cursorItem())
	return cmd
}

// HandleMouseUp stops any active scroll drag.
// Returns true if a drag was active and was stopped.
func (p *OverlayPicker[T]) HandleMouseUp() bool {
	if p.vDrag.Active() {
		p.vDrag.Stop()
		return true
	}
	return false
}

// HandleMouseMove processes mouse motion during a scroll drag.
// Returns true if a drag is active and the event was handled.
func (p *OverlayPicker[T]) HandleMouseMove(mouseY int) bool {
	if !p.vDrag.Active() {
		return false
	}
	count := len(p.filtered)
	vis := p.visibleRows()
	p.ListNav.SetScroll(p.vDrag.DragOffset(mouseY, vis, count), count, vis)
	return true
}

// HandleScrollClick handles a scrollbar click with proper thumb hit testing.
func (p *OverlayPicker[T]) HandleScrollClick(mouseY, baseY int) {
	localY := mouseY - baseY
	count := len(p.filtered)
	vis := p.visibleRows()
	if delta := p.vDrag.HandleClick(localY, baseY, count, vis, p.Scroll); delta != 0 {
		if delta < 0 {
			p.ListNav.PageUp(vis)
		} else {
			p.ListNav.PageDown(count, vis)
		}
	}
}

// Update handles all messages for the picker. Since the picker is modal,
// it processes everything: content messages, content keys, navigation,
// and filter input.
func (p *OverlayPicker[T]) Update(msg tea.Msg) tea.Cmd {
	// Help mode — dismiss on any key
	if p.showHelp {
		if _, ok := msg.(tea.KeyPressMsg); ok {
			p.showHelp = false
			return nil
		}
		return nil
	}

	// Content-specific messages
	if cmd, newItems, handled := p.content.HandleMsg(msg); handled {
		if newItems != nil {
			p.items = newItems
			p.applyFilter()
		}
		return cmd
	}

	// Content-specific keys (called before navigation for confirm mode etc.)
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		// Help toggle (only if content provides help)
		key := msg.String()
		if (key == "?" || key == "f1") && p.content.HelpContent() != nil {
			p.showHelp = true
			return nil
		}

		if cmd, action := p.content.HandleKey(key, p.cursorItem()); action != PickerNotHandled {
			if action == PickerMoveDown {
				p.ListNav.MoveDown(len(p.filtered), p.visibleRows())
			}
			return cmd
		}
	}

	// Navigation (wheel + keys)
	count := len(p.filtered)
	vis := p.visibleRows()
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		p.ListNav.HandleWheel(msg.Button == tea.MouseWheelUp, count, vis)
		return nil
	case tea.KeyPressMsg:
		if p.ListNav.HandleKey(msg.String(), count, vis) {
			return nil
		}
	}

	// Filter input
	prev := p.filter.Value()
	var cmd tea.Cmd
	p.filter, cmd = p.filter.Update(msg)
	if p.filter.Value() != prev {
		p.applyFilter()
	}
	return cmd
}

// BuildLayer renders the complete overlay popup.
func (p *OverlayPicker[T]) BuildLayer() *lipgloss.Layer {
	innerW := p.innerW

	p.filter.SetWidth(innerW - 3)
	filterLine := p.filter.View()

	vis := p.visibleRows()
	start, end := p.Scroll, p.Scroll+vis
	if end > len(p.filtered) {
		end = len(p.filtered)
	}

	var lines []string
	for i := start; i < end; i++ {
		item := p.items[p.filtered[i]]
		lines = append(lines, p.content.RenderItem(item, i == p.Cursor, innerW))
	}

	filtered := len(p.filtered)
	total := len(p.items)

	if filtered == 0 {
		lines = append(lines, p.content.EmptyText())
	}

	// Pad to fixed height so overlay doesn't shrink when filtering
	for len(lines) < vis {
		lines = append(lines, "")
	}

	listContent := strings.Join(lines, "\n")

	title := p.content.Title(filtered, total)
	footer := p.content.Footer(filtered, total)

	var b strings.Builder
	b.WriteString(title)
	b.WriteByte('\n')
	b.WriteString(filterLine)
	b.WriteString("\n\n")
	b.WriteString(listContent)
	b.WriteString("\n\n")
	b.WriteString(footer)

	full := OverlayStyle().Render(b.String())

	var children []*lipgloss.Layer
	// scrollbar Y: border(1) + title(1) + filter(1) + blank(1) = 4
	if sbLayer := ScrollbarLayer(p.scrollbarID, filtered, vis, p.Scroll, lipgloss.Width(full), 4, 51); sbLayer != nil {
		children = append(children, sbLayer)
	}

	// Help overlay: render on top of the picker content
	if p.showHelp {
		if keys := p.content.HelpContent(); keys != nil {
			children = append(children, p.buildHelpLayer(keys, lipgloss.Width(full), lipgloss.Height(full)))
		}
	}

	return lipgloss.NewLayer(full, children...)
}

// buildHelpLayer creates a child layer with help content centered over the picker.
func (p *OverlayPicker[T]) buildHelpLayer(keys [][2]string, parentW, parentH int) *lipgloss.Layer {
	title := styles.BoldStyle.Foreground(styles.PrimaryColor).
		Render("Keyboard Shortcuts")
	helpBody := RenderKeyHelp(keys)
	bodyW := lipgloss.Width(helpBody)

	footer := styles.MutedStyle.Render("Press any key to close")
	footerW := lipgloss.Width(footer)
	if footerW < bodyW {
		pad := (bodyW - footerW) / 2
		footer = strings.Repeat(" ", pad) + footer
	}

	var b strings.Builder
	b.WriteString(title)
	b.WriteByte('\n')
	b.WriteString(helpBody)
	b.WriteByte('\n')
	b.WriteString(footer)

	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(styles.PrimaryColor).
		Padding(1, 3)
	rendered := helpStyle.Render(b.String())

	hx := (parentW - lipgloss.Width(rendered)) / 2
	hy := (parentH - lipgloss.Height(rendered)) / 2
	if hx < 0 {
		hx = 0
	}
	if hy < 0 {
		hy = 0
	}

	return lipgloss.NewLayer(rendered).X(hx).Y(hy).Z(52)
}

func (p *OverlayPicker[T]) cursorItem() *T {
	if len(p.filtered) == 0 || p.Cursor >= len(p.filtered) {
		return nil
	}
	return &p.items[p.filtered[p.Cursor]]
}

func (p *OverlayPicker[T]) visibleRows() int {
	if p.fixedVis > 0 {
		return p.fixedVis
	}
	return 3
}

func (p *OverlayPicker[T]) applyFilter() {
	query := strings.ToLower(p.filter.Value())
	count := len(p.items)
	p.filtered = nil
	for i := 0; i < count; i++ {
		if query == "" || p.content.MatchFilter(p.items[i], query) {
			p.filtered = append(p.filtered, i)
		}
	}
	p.Cursor = 0
	p.Scroll = 0
}

func (p *OverlayPicker[T]) reclampInnerW() {
	maxW := p.width - 4 // border(2) + padding(2)
	if maxW < 30 {
		maxW = 30
	}
	p.innerW = p.contentW
	if p.innerW > maxW {
		p.innerW = maxW
	}
	p.filter.SetWidth(p.innerW - 3) // prompt "/ " (2) + cursor (1)
}

func (p *OverlayPicker[T]) calcVisibleRows() int {
	maxRows := p.height - p.heightOffset
	if maxRows < 3 {
		maxRows = 3
	}
	n := len(p.items)
	if n < 1 {
		n = 1
	}
	if n < maxRows {
		return n
	}
	return maxRows
}
