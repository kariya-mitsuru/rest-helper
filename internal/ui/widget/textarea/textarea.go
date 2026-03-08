// Package textarea provides a multi-line text input component for Bubble Tea
// applications. Forked from charm.land/bubbles/v2/textarea with unused features
// removed (line numbers, prompt function, case transforms, real cursor) and
// viewport access added for mouse scroll support.
package textarea

import (
	"fmt"
	"hash/fnv"
	"image/color"
	"slices"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	rw "github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

const (
	minHeight        = 1
	defaultHeight    = 6
	defaultWidth     = 40
	defaultCharLimit = 0
	defaultMaxHeight = 99

	maxLines = 10000
)

// Internal messages for clipboard operations.
type (
	pasteMsg    string
	pasteErrMsg struct{ error }
)

// KeyMap is the key bindings for different actions within the textarea.
type KeyMap struct {
	CharacterBackward       key.Binding
	CharacterForward        key.Binding
	DeleteAfterCursor       key.Binding
	DeleteBeforeCursor      key.Binding
	DeleteCharacterBackward key.Binding
	DeleteCharacterForward  key.Binding
	DeleteWordBackward      key.Binding
	DeleteWordForward       key.Binding
	InsertNewline           key.Binding
	LineEnd                 key.Binding
	LineNext                key.Binding
	LinePrevious            key.Binding
	LineStart               key.Binding
	PageUp                  key.Binding
	PageDown                key.Binding
	Paste                   key.Binding
	WordBackward            key.Binding
	WordForward             key.Binding
	InputBegin              key.Binding
	InputEnd                key.Binding
}

// DefaultKeyMap returns the default set of key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		CharacterForward:        key.NewBinding(key.WithKeys("right", "ctrl+f")),
		CharacterBackward:       key.NewBinding(key.WithKeys("left", "ctrl+b")),
		WordForward:             key.NewBinding(key.WithKeys("alt+right", "alt+f")),
		WordBackward:            key.NewBinding(key.WithKeys("alt+left", "alt+b")),
		LineNext:                key.NewBinding(key.WithKeys("down", "ctrl+n")),
		LinePrevious:            key.NewBinding(key.WithKeys("up", "ctrl+p")),
		DeleteWordBackward:      key.NewBinding(key.WithKeys("alt+backspace", "ctrl+w")),
		DeleteWordForward:       key.NewBinding(key.WithKeys("alt+delete", "alt+d")),
		DeleteAfterCursor:       key.NewBinding(key.WithKeys("ctrl+k")),
		DeleteBeforeCursor:      key.NewBinding(key.WithKeys("ctrl+u")),
		InsertNewline:           key.NewBinding(key.WithKeys("enter", "ctrl+m")),
		DeleteCharacterBackward: key.NewBinding(key.WithKeys("backspace", "ctrl+h")),
		DeleteCharacterForward:  key.NewBinding(key.WithKeys("delete", "ctrl+d")),
		LineStart:               key.NewBinding(key.WithKeys("home", "ctrl+a")),
		LineEnd:                 key.NewBinding(key.WithKeys("end", "ctrl+e")),
		PageUp:                  key.NewBinding(key.WithKeys("pgup")),
		PageDown:                key.NewBinding(key.WithKeys("pgdown")),
		Paste:                   key.NewBinding(key.WithKeys("ctrl+v")),
		InputBegin:              key.NewBinding(key.WithKeys("alt+<", "ctrl+home")),
		InputEnd:                key.NewBinding(key.WithKeys("alt+>", "ctrl+end")),
	}
}

// LineInfo is a helper for keeping track of line information regarding
// soft-wrapped lines.
type LineInfo struct {
	Width        int
	CharWidth    int
	Height       int
	StartColumn  int
	ColumnOffset int
	RowOffset    int
	CharOffset   int
}

// Styles are the styles for the textarea.
type Styles struct {
	Focused StyleState
	Blurred StyleState
	Cursor  CursorStyle
}

// CursorStyle is the style for the virtual cursor.
type CursorStyle struct {
	Color color.Color
	Shape tea.CursorShape
	Blink bool
}

// StyleState that will be applied to the text area.
type StyleState struct {
	Base        lipgloss.Style
	Text        lipgloss.Style
	CursorLine  lipgloss.Style
	EndOfBuffer lipgloss.Style
	Placeholder lipgloss.Style
	Prompt      lipgloss.Style
}

func (s StyleState) computedCursorLine() lipgloss.Style {
	return s.CursorLine.Inherit(s.Base).Inline(true)
}

func (s StyleState) computedEndOfBuffer() lipgloss.Style {
	return s.EndOfBuffer.Inherit(s.Base).Inline(true)
}

func (s StyleState) computedPlaceholder() lipgloss.Style {
	return s.Placeholder.Inherit(s.Base).Inline(true)
}

func (s StyleState) computedPrompt() lipgloss.Style {
	return s.Prompt.Inherit(s.Base).Inline(true)
}

func (s StyleState) computedText() lipgloss.Style {
	return s.Text.Inherit(s.Base).Inline(true)
}

// line is the input to the text wrapping function (memoization key).
type line struct {
	runes []rune
	width int
}

// Hash returns a hash of the line.
func (w line) Hash() string {
	h := fnv.New64a()
	h.Write([]byte(string(w.runes)))
	h.Write([]byte{byte(w.width >> 8), byte(w.width)})
	return fmt.Sprintf("%x", h.Sum64())
}

// Model is the Bubble Tea model for the textarea.
type Model struct {
	Err error

	cache *MemoCache[line, [][]rune]

	// Prompt is printed at the beginning of each line.
	Prompt string

	// Placeholder is the text displayed when empty.
	Placeholder string

	// KeyMap encodes the keybindings recognized by the widget.
	KeyMap KeyMap

	// virtualCursor manages the virtual cursor.
	virtualCursor cursor.Model

	// CharLimit is the maximum number of characters. 0 = no limit.
	CharLimit int

	// MaxHeight is the maximum height in rows. 0 = no limit.
	MaxHeight int

	styles Styles

	promptWidth int
	width       int
	height      int
	value       [][]rune
	focus       bool
	col         int
	row         int

	lastCharOffset int

	viewport *viewport.Model
	rsan     Sanitizer

	wrapMode bool // true = wrap (default), false = horizontal scroll
	xOffset  int  // horizontal scroll offset (used when wrapMode is false)
}

// New creates a new model with default settings.
func New() Model {
	vp := viewport.New()
	vp.KeyMap = viewport.KeyMap{}
	cur := cursor.New()

	styles := DefaultDarkStyles()

	m := Model{
		CharLimit:     defaultCharLimit,
		MaxHeight:     defaultMaxHeight,
		Prompt:        lipgloss.ThickBorder().Left + " ",
		styles:        styles,
		cache:         NewMemoCache[line, [][]rune](maxLines),
		virtualCursor: cur,
		KeyMap:        DefaultKeyMap(),
		value:         make([][]rune, minHeight, maxLines),
		focus:         false,
		col:           0,
		row:           0,
		wrapMode:      true,
		viewport:      &vp,
	}

	m.SetHeight(defaultHeight)
	m.SetWidth(defaultWidth)
	m.updateVirtualCursorStyle()

	return m
}

// DefaultStyles returns the default styles for the textarea.
func DefaultStyles(isDark bool) Styles {
	lightDark := lipgloss.LightDark(isDark)

	var s Styles
	s.Focused = StyleState{
		Base:        lipgloss.NewStyle(),
		CursorLine:  lipgloss.NewStyle().Background(lightDark(lipgloss.Color("255"), lipgloss.Color("0"))),
		EndOfBuffer: lipgloss.NewStyle().Foreground(lightDark(lipgloss.Color("254"), lipgloss.Color("0"))),
		Placeholder: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Prompt:      lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
		Text:        lipgloss.NewStyle(),
	}
	s.Blurred = StyleState{
		Base:        lipgloss.NewStyle(),
		CursorLine:  lipgloss.NewStyle().Foreground(lightDark(lipgloss.Color("245"), lipgloss.Color("7"))),
		EndOfBuffer: lipgloss.NewStyle().Foreground(lightDark(lipgloss.Color("254"), lipgloss.Color("0"))),
		Placeholder: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Prompt:      lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
		Text:        lipgloss.NewStyle().Foreground(lightDark(lipgloss.Color("245"), lipgloss.Color("7"))),
	}
	s.Cursor = CursorStyle{
		Color: lipgloss.Color("7"),
		Shape: tea.CursorBlock,
		Blink: true,
	}
	return s
}

// DefaultDarkStyles returns the default styles for a dark background.
func DefaultDarkStyles() Styles { return DefaultStyles(true) }

// Styles returns the current styles.
func (m Model) GetStyles() Styles { return m.styles }

// SetStyles updates styling.
func (m *Model) SetStyles(s Styles) {
	m.styles = s
	m.updateVirtualCursorStyle()
}

func (m *Model) updateVirtualCursorStyle() {
	m.virtualCursor.Style = lipgloss.NewStyle().Foreground(m.styles.Cursor.Color)
	if m.styles.Cursor.Blink {
		m.virtualCursor.SetMode(cursor.CursorBlink)
		return
	}
	m.virtualCursor.SetMode(cursor.CursorStatic)
}

// SetValue sets the value of the text input.
func (m *Model) SetValue(s string) {
	m.Reset()
	m.InsertString(s)
}

// InsertString inserts a string at the cursor position.
func (m *Model) InsertString(s string) {
	m.insertRunesFromUserInput([]rune(s))
}

// insertRunesFromUserInput inserts runes at the current cursor position.
func (m *Model) insertRunesFromUserInput(runes []rune) {
	runes = m.san().Sanitize(runes)

	if m.CharLimit > 0 {
		availSpace := m.CharLimit - m.Length()
		if availSpace <= 0 {
			return
		}
		if availSpace < len(runes) {
			runes = runes[:availSpace]
		}
	}

	var lines [][]rune
	lstart := 0
	for i := range runes {
		if runes[i] == '\n' {
			lines = append(lines, runes[lstart:i:i])
			lstart = i + 1
		}
	}
	if lstart <= len(runes) {
		lines = append(lines, runes[lstart:])
	}

	if maxLines > 0 && len(m.value)+len(lines)-1 > maxLines {
		allowedHeight := max(0, maxLines-len(m.value)+1)
		lines = lines[:allowedHeight]
	}

	if len(lines) == 0 {
		return
	}

	tail := make([]rune, len(m.value[m.row][m.col:]))
	copy(tail, m.value[m.row][m.col:])

	m.value[m.row] = append(m.value[m.row][:m.col], lines[0]...)
	m.col += len(lines[0])

	if numExtraLines := len(lines) - 1; numExtraLines > 0 {
		var newGrid [][]rune
		if cap(m.value) >= len(m.value)+numExtraLines {
			newGrid = m.value[:len(m.value)+numExtraLines]
		} else {
			newGrid = make([][]rune, len(m.value)+numExtraLines)
			copy(newGrid, m.value[:m.row+1])
		}
		copy(newGrid[m.row+1+numExtraLines:], m.value[m.row+1:])
		m.value = newGrid
		for _, l := range lines[1:] {
			m.row++
			m.value[m.row] = l
			m.col = len(l)
		}
	}

	m.value[m.row] = append(m.value[m.row], tail...)
	m.SetCursorColumn(m.col)
}

// Value returns the value of the text input.
func (m Model) Value() string {
	if m.value == nil {
		return ""
	}
	var v strings.Builder
	for _, l := range m.value {
		v.WriteString(string(l))
		v.WriteByte('\n')
	}
	return strings.TrimSuffix(v.String(), "\n")
}

// Length returns the number of characters currently in the text input.
func (m *Model) Length() int {
	var l int
	for _, row := range m.value {
		l += uniseg.StringWidth(string(row))
	}
	return l + len(m.value) - 1
}

// LineCount returns the number of lines.
func (m *Model) LineCount() int { return len(m.value) }

// Line returns the 0-indexed cursor row.
func (m Model) Line() int { return m.row }

// Column returns the 0-indexed cursor column.
func (m Model) Column() int { return m.col }

// YOffset returns the viewport's current scroll offset.
func (m Model) YOffset() int { return m.viewport.YOffset() }

// SetYOffset sets the viewport's scroll offset for mouse drag support.
func (m *Model) SetYOffset(offset int) { m.viewport.SetYOffset(offset) }

// TotalLineCount returns the total number of rendered (soft-wrapped) lines.
func (m Model) TotalLineCount() int {
	total := 0
	for _, line := range m.value {
		total += len(m.memoizedWrap(line, m.width))
	}
	return total
}

// ViewportHeight returns the visible viewport height.
func (m Model) ViewportHeight() int { return m.height }

// WrapMode returns whether wrap mode is enabled.
func (m Model) WrapMode() bool { return m.wrapMode }

// SetWrapMode enables or disables line wrapping.
func (m *Model) SetWrapMode(wrap bool) {
	// Preserve cursor's screen row across the mode change.
	screenRow := m.cursorLineNumber() - m.viewport.YOffset()

	m.wrapMode = wrap
	if wrap {
		m.xOffset = 0
	}

	// Restore: set YOffset so the cursor stays at the same screen row.
	newYOff := m.cursorLineNumber() - screenRow
	total := m.TotalLineCount()
	maxYOff := total - m.viewport.Height()
	if maxYOff < 0 {
		maxYOff = 0
	}
	if newYOff > maxYOff {
		newYOff = maxYOff
	}
	if newYOff < 0 {
		newYOff = 0
	}
	m.viewport.SetYOffset(newYOff)
}

// XOffset returns the horizontal scroll offset.
func (m Model) XOffset() int { return m.xOffset }

// SetXOffset sets the horizontal scroll offset.
func (m *Model) SetXOffset(offset int) {
	if offset < 0 {
		offset = 0
	}
	max := m.MaxLineWidth() - m.width
	if max < 0 {
		max = 0
	}
	if offset > max {
		offset = max
	}
	m.xOffset = offset
}

// MaxLineWidth returns the width of the longest line (in runes).
func (m Model) MaxLineWidth() int {
	maxW := 0
	for _, line := range m.value {
		w := uniseg.StringWidth(string(line))
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

// ScrollPercent returns the scroll percentage (0-1).
func (m Model) ScrollPercent() float64 { return m.viewport.ScrollPercent() }

func (m *Model) setCursorLineRelative(delta int) {
	if delta == 0 {
		return
	}

	li := m.LineInfo()
	charOffset := max(m.lastCharOffset, li.CharOffset)
	m.lastCharOffset = charOffset

	const trailingSpace = 2

	if delta > 0 {
		for range delta {
			if li.RowOffset+1 >= li.Height && m.row < len(m.value)-1 {
				m.row++
				m.col = 0
			} else {
				m.col = min(li.StartColumn+li.Width+trailingSpace, len(m.value[m.row])-1)
			}
			li = m.LineInfo()
		}
	} else {
		for range -delta {
			if li.RowOffset <= 0 && m.row > 0 {
				m.row--
				m.col = len(m.value[m.row])
			} else {
				m.col = li.StartColumn - trailingSpace
			}
			li = m.LineInfo()
		}
	}

	nli := m.LineInfo()
	m.col = nli.StartColumn

	if nli.Width <= 0 {
		m.repositionView()
		return
	}

	offset := 0
	for offset < charOffset {
		if m.row >= len(m.value) || m.col >= len(m.value[m.row]) || offset >= nli.CharWidth-1 {
			break
		}
		offset += rw.RuneWidth(m.value[m.row][m.col])
		m.col++
	}
	m.repositionView()
}

// CursorDown moves the cursor down by one line.
func (m *Model) CursorDown() { m.setCursorLineRelative(1) }

// CursorUp moves the cursor up by one line.
func (m *Model) CursorUp() { m.setCursorLineRelative(-1) }

// SetCursorColumn moves the cursor to the given column.
func (m *Model) SetCursorColumn(col int) {
	m.col = clamp(col, 0, len(m.value[m.row]))
	m.lastCharOffset = 0
}

// CursorStart moves the cursor to the start of the line.
func (m *Model) CursorStart() { m.SetCursorColumn(0) }

// CursorEnd moves the cursor to the end of the line.
func (m *Model) CursorEnd() { m.SetCursorColumn(len(m.value[m.row])) }

// Focused returns the focus state.
func (m Model) Focused() bool { return m.focus }

func (m Model) activeStyle() *StyleState {
	if m.focus {
		return &m.styles.Focused
	}
	return &m.styles.Blurred
}

// Focus sets the focus state.
func (m *Model) Focus() tea.Cmd {
	m.focus = true
	return m.virtualCursor.Focus()
}

// Blur removes the focus state.
func (m *Model) Blur() {
	m.focus = false
	m.virtualCursor.Blur()
}

// Reset sets the input to its default state.
func (m *Model) Reset() {
	m.value = make([][]rune, minHeight, maxLines)
	m.col = 0
	m.row = 0
	m.viewport.GotoTop()
	m.SetCursorColumn(0)
}

func (m *Model) san() Sanitizer {
	if m.rsan == nil {
		m.rsan = NewSanitizer()
	}
	return m.rsan
}

func (m *Model) deleteBeforeCursor() {
	m.value[m.row] = m.value[m.row][m.col:]
	m.SetCursorColumn(0)
}

func (m *Model) deleteAfterCursor() {
	m.value[m.row] = m.value[m.row][:m.col]
	m.SetCursorColumn(len(m.value[m.row]))
}

func (m *Model) deleteWordLeft() {
	if m.col == 0 || len(m.value[m.row]) == 0 {
		return
	}

	oldCol := m.col
	m.SetCursorColumn(m.col - 1)
	for unicode.IsSpace(m.value[m.row][m.col]) {
		if m.col <= 0 {
			break
		}
		m.SetCursorColumn(m.col - 1)
	}

	for m.col > 0 {
		if !unicode.IsSpace(m.value[m.row][m.col]) {
			m.SetCursorColumn(m.col - 1)
		} else {
			if m.col > 0 {
				m.SetCursorColumn(m.col + 1)
			}
			break
		}
	}

	if oldCol > len(m.value[m.row]) {
		m.value[m.row] = m.value[m.row][:m.col]
	} else {
		m.value[m.row] = append(m.value[m.row][:m.col], m.value[m.row][oldCol:]...)
	}
}

func (m *Model) deleteWordRight() {
	if m.col >= len(m.value[m.row]) || len(m.value[m.row]) == 0 {
		return
	}

	oldCol := m.col
	for m.col < len(m.value[m.row]) && unicode.IsSpace(m.value[m.row][m.col]) {
		m.SetCursorColumn(m.col + 1)
	}
	for m.col < len(m.value[m.row]) {
		if !unicode.IsSpace(m.value[m.row][m.col]) {
			m.SetCursorColumn(m.col + 1)
		} else {
			break
		}
	}

	if m.col > len(m.value[m.row]) {
		m.value[m.row] = m.value[m.row][:oldCol]
	} else {
		m.value[m.row] = append(m.value[m.row][:oldCol], m.value[m.row][m.col:]...)
	}
	m.SetCursorColumn(oldCol)
}

func (m *Model) characterRight() {
	if m.col < len(m.value[m.row]) {
		m.SetCursorColumn(m.col + 1)
	} else if m.row < len(m.value)-1 {
		m.row++
		m.CursorStart()
	}
}

func (m *Model) characterLeft(insideLine bool) {
	if m.col == 0 && m.row != 0 {
		m.row--
		m.CursorEnd()
		if !insideLine {
			return
		}
	}
	if m.col > 0 {
		m.SetCursorColumn(m.col - 1)
	}
}

func (m *Model) wordLeft() {
	for {
		m.characterLeft(true)
		if m.col < len(m.value[m.row]) && !unicode.IsSpace(m.value[m.row][m.col]) {
			break
		}
	}
	for m.col > 0 {
		if unicode.IsSpace(m.value[m.row][m.col-1]) {
			break
		}
		m.SetCursorColumn(m.col - 1)
	}
}

func (m *Model) wordRight() {
	for m.col >= len(m.value[m.row]) || unicode.IsSpace(m.value[m.row][m.col]) {
		if m.row == len(m.value)-1 && m.col == len(m.value[m.row]) {
			break
		}
		m.characterRight()
	}
	for m.col < len(m.value[m.row]) {
		if unicode.IsSpace(m.value[m.row][m.col]) {
			break
		}
		m.SetCursorColumn(m.col + 1)
	}
}

// LineInfo returns the number of characters from the start of the
// (soft-wrapped) line and the (soft-wrapped) line width.
func (m Model) LineInfo() LineInfo {
	grid := m.memoizedWrap(m.value[m.row], m.width)

	var counter int
	for i, line := range grid {
		if counter+len(line) == m.col && i+1 < len(grid) {
			return LineInfo{
				CharOffset:   0,
				ColumnOffset: 0,
				Height:       len(grid),
				RowOffset:    i + 1,
				StartColumn:  m.col,
				Width:        len(grid[i+1]),
				CharWidth:    uniseg.StringWidth(string(line)),
			}
		}

		if counter+len(line) >= m.col {
			return LineInfo{
				CharOffset:   uniseg.StringWidth(string(line[:max(0, m.col-counter)])),
				ColumnOffset: m.col - counter,
				Height:       len(grid),
				RowOffset:    i,
				StartColumn:  counter,
				Width:        len(line),
				CharWidth:    uniseg.StringWidth(string(line)),
			}
		}

		counter += len(line)
	}
	return LineInfo{}
}

func (m *Model) repositionView() {
	// Clamp YOffset so we don't scroll past the content.
	total := m.TotalLineCount()
	maxYOff := total - m.viewport.Height()
	if maxYOff < 0 {
		maxYOff = 0
	}
	if m.viewport.YOffset() > maxYOff {
		m.viewport.SetYOffset(maxYOff)
	}

	minimum := m.viewport.YOffset()
	maximum := minimum + m.viewport.Height() - 1
	if row := m.cursorLineNumber(); row < minimum {
		m.viewport.ScrollUp(minimum - row)
	} else if row > maximum {
		m.viewport.ScrollDown(row - maximum)
	}

	// Horizontal auto-scroll in scroll mode.
	if !m.wrapMode {
		cursorX := m.LineInfo().CharOffset
		if cursorX < m.xOffset {
			m.xOffset = cursorX
		} else if cursorX+1 > m.xOffset+m.width {
			m.xOffset = cursorX - m.width + 1
		}
	}
}

// Width returns the width of the textarea.
func (m Model) Width() int { return m.width }

// MoveToBegin moves the cursor to the beginning of the input.
func (m *Model) MoveToBegin() {
	m.row = 0
	m.SetCursorColumn(0)
	m.repositionView()
}

// MoveToEnd moves the cursor to the end of the input.
func (m *Model) MoveToEnd() {
	m.row = len(m.value) - 1
	m.SetCursorColumn(len(m.value[m.row]))
	m.repositionView()
}

// PageUp moves the cursor up by one page.
func (m *Model) PageUp() {
	if offset := m.viewport.YOffset() - m.cursorLineNumber(); offset < 0 {
		m.setCursorLineRelative(offset)
		return
	}
	m.setCursorLineRelative(-m.height)
}

// PageDown moves the cursor down by one page.
func (m *Model) PageDown() {
	if offset := m.cursorLineNumber() - m.viewport.YOffset(); offset < m.height-1 {
		m.setCursorLineRelative(m.height - 1 - offset)
		return
	}
	m.setCursorLineRelative(m.height)
}

// SetWidth sets the width of the textarea.
func (m *Model) SetWidth(w int) {
	m.promptWidth = uniseg.StringWidth(m.Prompt)

	reservedOuter := m.activeStyle().Base.GetHorizontalFrameSize()
	reservedInner := m.promptWidth

	minWidth := reservedInner + reservedOuter + 1
	inputWidth := max(w, minWidth)

	m.viewport.SetWidth(inputWidth - reservedOuter)
	m.width = inputWidth - reservedOuter - reservedInner
}

// Height returns the current height.
func (m Model) Height() int { return m.height }

// SetHeight sets the height of the textarea.
func (m *Model) SetHeight(h int) {
	if m.MaxHeight > 0 {
		m.height = clamp(h, minHeight, m.MaxHeight)
		m.viewport.SetHeight(clamp(h, minHeight, m.MaxHeight))
	} else {
		m.height = max(h, minHeight)
		m.viewport.SetHeight(max(h, minHeight))
	}
	m.repositionView()
}

// Update is the Bubble Tea update loop.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focus {
		m.virtualCursor.Blur()
		return m, nil
	}

	oldRow, oldCol := m.cursorLineNumber(), m.col
	var cmds []tea.Cmd

	if m.value[m.row] == nil {
		m.value[m.row] = make([]rune, 0)
	}

	if m.MaxHeight > 0 && m.MaxHeight != m.cache.Capacity() {
		m.cache = NewMemoCache[line, [][]rune](m.MaxHeight)
	}

	switch msg := msg.(type) {
	case tea.PasteMsg:
		m.insertRunesFromUserInput([]rune(msg.Content))
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.DeleteAfterCursor):
			m.col = clamp(m.col, 0, len(m.value[m.row]))
			if m.col >= len(m.value[m.row]) {
				m.mergeLineBelow(m.row)
				break
			}
			m.deleteAfterCursor()
		case key.Matches(msg, m.KeyMap.DeleteBeforeCursor):
			m.col = clamp(m.col, 0, len(m.value[m.row]))
			if m.col <= 0 {
				m.mergeLineAbove(m.row)
				break
			}
			m.deleteBeforeCursor()
		case key.Matches(msg, m.KeyMap.DeleteCharacterBackward):
			m.col = clamp(m.col, 0, len(m.value[m.row]))
			if m.col <= 0 {
				m.mergeLineAbove(m.row)
				break
			}
			if len(m.value[m.row]) > 0 {
				m.value[m.row] = append(m.value[m.row][:max(0, m.col-1)], m.value[m.row][m.col:]...)
				if m.col > 0 {
					m.SetCursorColumn(m.col - 1)
				}
			}
		case key.Matches(msg, m.KeyMap.DeleteCharacterForward):
			if len(m.value[m.row]) > 0 && m.col < len(m.value[m.row]) {
				m.value[m.row] = slices.Delete(m.value[m.row], m.col, m.col+1)
			} else if m.col >= len(m.value[m.row]) {
				m.mergeLineBelow(m.row)
				break
			}
		case key.Matches(msg, m.KeyMap.DeleteWordBackward):
			if m.col <= 0 {
				m.mergeLineAbove(m.row)
				break
			}
			m.deleteWordLeft()
		case key.Matches(msg, m.KeyMap.DeleteWordForward):
			m.col = clamp(m.col, 0, len(m.value[m.row]))
			if m.col >= len(m.value[m.row]) {
				m.mergeLineBelow(m.row)
				break
			}
			m.deleteWordRight()
		case key.Matches(msg, m.KeyMap.InsertNewline):
			if m.MaxHeight > 0 && len(m.value) >= m.MaxHeight {
				return m, nil
			}
			m.col = clamp(m.col, 0, len(m.value[m.row]))
			m.splitLine(m.row, m.col)
		case key.Matches(msg, m.KeyMap.LineEnd):
			m.CursorEnd()
		case key.Matches(msg, m.KeyMap.LineStart):
			m.CursorStart()
		case key.Matches(msg, m.KeyMap.CharacterForward):
			m.characterRight()
		case key.Matches(msg, m.KeyMap.LineNext):
			m.CursorDown()
		case key.Matches(msg, m.KeyMap.WordForward):
			m.wordRight()
		case key.Matches(msg, m.KeyMap.Paste):
			return m, Paste
		case key.Matches(msg, m.KeyMap.CharacterBackward):
			m.characterLeft(false)
		case key.Matches(msg, m.KeyMap.LinePrevious):
			m.CursorUp()
		case key.Matches(msg, m.KeyMap.WordBackward):
			m.wordLeft()
		case key.Matches(msg, m.KeyMap.InputBegin):
			m.MoveToBegin()
		case key.Matches(msg, m.KeyMap.InputEnd):
			m.MoveToEnd()
		case key.Matches(msg, m.KeyMap.PageUp):
			m.PageUp()
		case key.Matches(msg, m.KeyMap.PageDown):
			m.PageDown()
		default:
			m.insertRunesFromUserInput([]rune(msg.Text))
		}

	case pasteMsg:
		m.insertRunesFromUserInput([]rune(msg))

	case pasteErrMsg:
		m.Err = msg
	}

	view := m.view()
	m.viewport.SetContent(view)
	vp, cmd := m.viewport.Update(msg)
	m.viewport = &vp
	cmds = append(cmds, cmd)

	m.virtualCursor, cmd = m.virtualCursor.Update(msg)
	newRow, newCol := m.cursorLineNumber(), m.col
	if (newRow != oldRow || newCol != oldCol) && m.virtualCursor.Mode() == cursor.CursorBlink {
		m.virtualCursor.IsBlinked = false
		cmd = m.virtualCursor.Blink()
	}
	cmds = append(cmds, cmd)

	m.repositionView()

	return m, tea.Batch(cmds...)
}

func (m *Model) view() string {
	if len(m.Value()) == 0 && m.row == 0 && m.col == 0 && m.Placeholder != "" {
		return m.placeholderView()
	}
	m.virtualCursor.TextStyle = m.activeStyle().computedCursorLine()

	var (
		s        strings.Builder
		style    lipgloss.Style
		newLines int
		lineInfo = m.LineInfo()
		styles   = m.activeStyle()
	)

	for l, line := range m.value {
		wrappedLines := m.memoizedWrap(line, m.width)

		if m.row == l {
			style = styles.computedCursorLine()
		} else {
			style = styles.computedText()
		}

		for wl, wrappedLine := range wrappedLines {
			prompt := styles.computedPrompt().Render(m.Prompt)
			s.WriteString(style.Render(prompt))

			// In scroll mode, apply horizontal offset to the line
			displayLine := wrappedLine
			colOffset := lineInfo.ColumnOffset
			if !m.wrapMode && m.xOffset > 0 {
				if m.xOffset < len(displayLine) {
					displayLine = displayLine[m.xOffset:]
				} else {
					displayLine = nil
				}
				if m.row == l && lineInfo.RowOffset == wl {
					colOffset -= m.xOffset
				}
			}

			strwidth := uniseg.StringWidth(string(displayLine))
			padding := m.width - strwidth
			if strwidth > m.width {
				// Truncate to visible width
				truncated := make([]rune, 0, m.width)
				tw := 0
				for _, r := range displayLine {
					rw := uniseg.StringWidth(string(r))
					if tw+rw > m.width {
						break
					}
					truncated = append(truncated, r)
					tw += rw
				}
				displayLine = truncated
				strwidth = tw
				padding = m.width - strwidth
			}

			if m.row == l && lineInfo.RowOffset == wl && colOffset >= 0 && colOffset < len(displayLine) {
				s.WriteString(style.Render(string(displayLine[:colOffset])))
				if m.col >= len(line) && lineInfo.CharOffset >= m.width {
					m.virtualCursor.SetChar(" ")
					s.WriteString(m.virtualCursor.View())
				} else {
					m.virtualCursor.SetChar(string(displayLine[colOffset]))
					s.WriteString(style.Render(m.virtualCursor.View()))
					s.WriteString(style.Render(string(displayLine[colOffset+1:])))
				}
			} else if m.row == l && lineInfo.RowOffset == wl && colOffset == len(displayLine) {
				// Cursor is at end of line — show space cursor
				s.WriteString(style.Render(string(displayLine)))
				m.virtualCursor.SetChar(" ")
				s.WriteString(m.virtualCursor.View())
				padding--
			} else {
				s.WriteString(style.Render(string(displayLine)))
			}
			s.WriteString(style.Render(strings.Repeat(" ", max(0, padding))))
			s.WriteRune('\n')
			newLines++
		}
	}

	// Pad to fill height.
	for range m.height {
		s.WriteString(m.Prompt)
		eob := styles.computedEndOfBuffer().Render(" ")
		rightGap := strings.Repeat(" ", max(0, m.Width()-1))
		s.WriteString(eob + rightGap)
		s.WriteRune('\n')
	}

	return s.String()
}

// View renders the text area in its current state.
func (m Model) View() string {
	m.viewport.SetContent(m.view())
	view := m.viewport.View()
	styles := m.activeStyle()
	return styles.Base.Render(view)
}

func (m Model) placeholderView() string {
	var (
		s      strings.Builder
		p      = m.Placeholder
		styles = m.activeStyle()
	)

	for i := range m.height {
		lineStyle := styles.computedPlaceholder()
		if i == 0 {
			lineStyle = styles.computedCursorLine()
		}

		prompt := styles.computedPrompt().Render(m.Prompt)
		s.WriteString(lineStyle.Render(prompt))

		switch {
		case i == 0:
			m.virtualCursor.TextStyle = styles.computedPlaceholder()
			ch, rest, _, _ := uniseg.FirstGraphemeClusterInString(p, 0)
			m.virtualCursor.SetChar(ch)
			s.WriteString(lineStyle.Render(m.virtualCursor.View()))
			s.WriteString(lineStyle.Render(styles.computedPlaceholder().Render(rest)))
			gap := strings.Repeat(" ", max(0, m.width-lipgloss.Width(p)))
			s.WriteString(lineStyle.Render(gap))
		default:
			eob := styles.computedEndOfBuffer().Render(" ")
			s.WriteString(eob)
		}

		s.WriteRune('\n')
	}

	m.viewport.SetContent(s.String())
	return styles.Base.Render(m.viewport.View())
}

// Blink returns the blink command for the virtual cursor.
func Blink() tea.Msg { return cursor.Blink() }

func (m Model) memoizedWrap(runes []rune, width int) [][]rune {
	if !m.wrapMode {
		// No wrapping: return the line as a single element.
		return [][]rune{runes}
	}
	input := line{runes: runes, width: width}
	if v, ok := m.cache.Get(input); ok {
		return v
	}
	v := wrap(runes, width)
	m.cache.Set(input, v)
	return v
}

func (m Model) cursorLineNumber() int {
	ln := 0
	for i := range m.row {
		ln += len(m.memoizedWrap(m.value[i], m.width))
	}
	ln += m.LineInfo().RowOffset
	return ln
}

func (m *Model) mergeLineBelow(row int) {
	if row >= len(m.value)-1 {
		return
	}
	m.value[row] = append(m.value[row], m.value[row+1]...)
	for i := row + 1; i < len(m.value)-1; i++ {
		m.value[i] = m.value[i+1]
	}
	if len(m.value) > 0 {
		m.value = m.value[:len(m.value)-1]
	}
}

func (m *Model) mergeLineAbove(row int) {
	if row <= 0 {
		return
	}
	m.col = len(m.value[row-1])
	m.row = m.row - 1
	m.value[row-1] = append(m.value[row-1], m.value[row]...)
	for i := row; i < len(m.value)-1; i++ {
		m.value[i] = m.value[i+1]
	}
	if len(m.value) > 0 {
		m.value = m.value[:len(m.value)-1]
	}
}

func (m *Model) splitLine(row, col int) {
	head, tailSrc := m.value[row][:col], m.value[row][col:]
	tail := make([]rune, len(tailSrc))
	copy(tail, tailSrc)

	m.value = append(m.value[:row+1], m.value[row:]...)
	m.value[row] = head
	m.value[row+1] = tail

	m.col = 0
	m.row++
}

// Paste is a command for pasting from the clipboard.
func Paste() tea.Msg {
	str, err := clipboard.ReadAll()
	if err != nil {
		return pasteErrMsg{err}
	}
	return pasteMsg(str)
}

func wrap(runes []rune, width int) [][]rune {
	var (
		lines  = [][]rune{{}}
		word   = []rune{}
		row    int
		spaces int
	)

	for _, r := range runes {
		if unicode.IsSpace(r) {
			spaces++
		} else {
			word = append(word, r)
		}

		if spaces > 0 {
			if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces > width {
				row++
				lines = append(lines, []rune{})
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], repeatSpaces(spaces)...)
				spaces = 0
				word = nil
			} else {
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], repeatSpaces(spaces)...)
				spaces = 0
				word = nil
			}
		} else {
			lastCharLen := rw.RuneWidth(word[len(word)-1])
			if uniseg.StringWidth(string(word))+lastCharLen > width {
				if len(lines[row]) > 0 {
					row++
					lines = append(lines, []rune{})
				}
				lines[row] = append(lines[row], word...)
				word = nil
			}
		}
	}

	if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces >= width {
		lines = append(lines, []rune{})
		lines[row+1] = append(lines[row+1], word...)
		spaces++
		lines[row+1] = append(lines[row+1], repeatSpaces(spaces)...)
	} else {
		lines[row] = append(lines[row], word...)
		spaces++
		lines[row] = append(lines[row], repeatSpaces(spaces)...)
	}

	return lines
}

func repeatSpaces(n int) []rune {
	return []rune(strings.Repeat(string(' '), n))
}

func clamp(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}
