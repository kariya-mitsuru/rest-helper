// SPDX-License-Identifier: MIT

package widget

// ListNav provides cursor and scroll state for navigable lists.
// Embed this struct and call its methods for consistent navigation behavior.
type ListNav struct {
	Cursor int
	Scroll int
}

// MoveUp moves the cursor up by one, scrolling if needed.
func (n *ListNav) MoveUp() {
	if n.Cursor > 0 {
		n.Cursor--
		if n.Cursor < n.Scroll {
			n.Scroll = n.Cursor
		}
	}
}

// MoveDown moves the cursor down by one, scrolling if needed.
func (n *ListNav) MoveDown(count, visible int) {
	if n.Cursor < count-1 {
		n.Cursor++
		if n.Cursor >= n.Scroll+visible {
			n.Scroll = n.Cursor - visible + 1
		}
	}
}

// PageUp moves the cursor up by one page.
func (n *ListNav) PageUp(visible int) {
	n.Cursor -= visible
	if n.Cursor < 0 {
		n.Cursor = 0
	}
	if n.Cursor < n.Scroll {
		n.Scroll = n.Cursor
	}
}

// PageDown moves the cursor down by one page.
func (n *ListNav) PageDown(count, visible int) {
	n.Cursor += visible
	maxIdx := count - 1
	if maxIdx < 0 {
		maxIdx = 0
	}
	if n.Cursor > maxIdx {
		n.Cursor = maxIdx
	}
	if n.Cursor >= n.Scroll+visible {
		n.Scroll = n.Cursor - visible + 1
	}
}

// Home moves the cursor to the first item.
func (n *ListNav) Home() {
	n.Cursor = 0
	n.Scroll = 0
}

// End moves the cursor to the last item.
func (n *ListNav) End(count, visible int) {
	n.Cursor = count - 1
	if n.Cursor < 0 {
		n.Cursor = 0
	}
	if n.Cursor >= n.Scroll+visible {
		n.Scroll = n.Cursor - visible + 1
	}
}

// HandleKey processes common list navigation keys (up, down, pgup, pgdown, home, end).
// Returns true if the key was handled.
func (n *ListNav) HandleKey(key string, count, visible int) bool {
	switch key {
	case "up":
		n.MoveUp()
	case "down":
		n.MoveDown(count, visible)
	case "pgup":
		n.PageUp(visible)
	case "pgdown":
		n.PageDown(count, visible)
	case "home", "ctrl+home":
		n.Home()
	case "end", "ctrl+end":
		n.End(count, visible)
	default:
		return false
	}
	return true
}

// HandleWheel processes a scroll wheel event.
func (n *ListNav) HandleWheel(up bool, count, visible int) {
	if up {
		n.MoveUp()
	} else {
		n.MoveDown(count, visible)
	}
}

// Clamp adjusts Cursor and Scroll so they remain within valid bounds for the
// given item count and visible window size.
func (n *ListNav) Clamp(count, visible int) {
	if n.Cursor >= count {
		n.Cursor = count - 1
	}
	if n.Cursor < 0 {
		n.Cursor = 0
	}
	if n.Cursor >= n.Scroll+visible {
		n.Scroll = n.Cursor - visible + 1
	}
	if n.Scroll+visible > count {
		n.Scroll = count - visible
	}
	if n.Scroll < 0 {
		n.Scroll = 0
	}
}

// SetScroll sets the scroll offset and adjusts the cursor to remain visible.
func (n *ListNav) SetScroll(offset, count, visible int) {
	if visible <= 1 || count <= visible {
		return
	}
	n.Scroll = offset
	if n.Cursor < n.Scroll {
		n.Cursor = n.Scroll
	}
	if n.Cursor >= n.Scroll+visible {
		n.Cursor = n.Scroll + visible - 1
	}
}
