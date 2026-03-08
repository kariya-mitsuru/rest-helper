// SPDX-License-Identifier: MIT

package widget

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Dropdown manages the open/close and highlight state for a select dropdown.
type Dropdown struct {
	open      bool
	highlight int
}

// Open opens the dropdown with the highlight at currentIdx.
func (d *Dropdown) Open(currentIdx int) {
	d.open = true
	d.highlight = currentIdx
}

// Close closes the dropdown.
func (d *Dropdown) Close() {
	d.open = false
}

// Toggle opens or closes the dropdown.
func (d *Dropdown) Toggle(currentIdx int) {
	if d.open {
		d.Close()
	} else {
		d.Open(currentIdx)
	}
}

// IsOpen returns true when the dropdown is visible.
func (d Dropdown) IsOpen() bool {
	return d.open
}

// Highlight returns the currently highlighted index.
func (d Dropdown) Highlight() int {
	return d.highlight
}

// HandleKey processes navigation keys (up/k, down/j, enter, esc).
// Returns (selectedIdx, handled). selectedIdx is -1 unless Enter was pressed.
func (d *Dropdown) HandleKey(key string, itemCount int) (selectedIdx int, handled bool) {
	switch key {
	case "up", "k":
		if d.highlight > 0 {
			d.highlight--
		}
		return -1, true
	case "down", "j":
		if d.highlight < itemCount-1 {
			d.highlight++
		}
		return -1, true
	case "enter":
		idx := d.highlight
		d.open = false
		return idx, true
	case "esc":
		d.open = false
		return -1, true
	}
	return -1, false
}

// HandleClick processes a mouse click on the dropdown.
// row/col are relative to the dropdown's top-left corner.
// dropdownW is the total rendered width (typically border+padding+itemWidth+padding+border).
// Returns (selectedIdx, ok).
func (d *Dropdown) HandleClick(row, col, itemCount, dropdownW int) (int, bool) {
	if col < 0 || col >= dropdownW {
		return -1, false
	}
	idx := row - 1 // row 0 = top border
	if idx < 0 || idx >= itemCount {
		return -1, false
	}
	d.open = false
	return idx, true
}

// Render creates a dropdown overlay. itemStyle returns the style for each item.
func (d Dropdown) Render(items []string, itemStyle func(item string, idx int, highlighted bool) lipgloss.Style) string {
	var b strings.Builder
	for i, item := range items {
		style := itemStyle(item, i, i == d.highlight)
		b.WriteString(style.Render(item))
		if i < len(items)-1 {
			b.WriteString("\n")
		}
	}
	return OverlayStyle().Render(b.String())
}
