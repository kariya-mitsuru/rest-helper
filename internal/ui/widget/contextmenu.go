// SPDX-License-Identifier: MIT

package widget

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
)

// MenuAction represents the result of a context menu interaction.
type MenuAction int

const (
	MenuNone     MenuAction = iota // no action taken
	MenuClose                      // menu should close
	MenuSelected                   // item was selected
)

// MenuItem represents a single entry in a context menu.
type MenuItem struct {
	Label string
	ID    string
}

// ContextMenu is a popup menu that appears at a specific screen position.
type ContextMenu struct {
	items     []MenuItem
	highlight int
	X, Y      int // screen position
}

// NewContextMenu creates a context menu at the given screen position.
// The position is clamped to stay within the screen bounds.
func NewContextMenu(items []MenuItem, x, y, screenW, screenH int) *ContextMenu {
	w := contextMenuWidth(items)
	h := len(items) + 2 // items + border top/bottom
	if x+w > screenW {
		x = screenW - w
	}
	if y+h > screenH {
		y = screenH - h
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return &ContextMenu{items: items, X: x, Y: y}
}

func contextMenuWidth(items []MenuItem) int {
	maxW := 0
	for _, item := range items {
		if w := len(item.Label); w > maxW {
			maxW = w
		}
	}
	return maxW + 4 // border(2) + padding(2)
}

// HandleKey processes a keyboard event and returns the selected item and action.
func (m *ContextMenu) HandleKey(key string) (MenuItem, MenuAction) {
	switch key {
	case "up", "k":
		if m.highlight > 0 {
			m.highlight--
		}
		return MenuItem{}, MenuNone
	case "down", "j":
		if m.highlight < len(m.items)-1 {
			m.highlight++
		}
		return MenuItem{}, MenuNone
	case "enter":
		if len(m.items) > 0 {
			return m.items[m.highlight], MenuSelected
		}
		return MenuItem{}, MenuClose
	case "esc":
		return MenuItem{}, MenuClose
	}
	return MenuItem{}, MenuClose
}

// HandleClick processes a mouse click at screen coordinates.
// Returns the selected item and action.
func (m *ContextMenu) HandleClick(screenX, screenY int) (MenuItem, MenuAction) {
	w := contextMenuWidth(m.items)
	h := len(m.items) + 2

	// Click outside the menu closes it
	if screenX < m.X || screenX >= m.X+w || screenY < m.Y || screenY >= m.Y+h {
		return MenuItem{}, MenuClose
	}

	// Convert to item index (subtract top border)
	idx := screenY - m.Y - 1
	if idx >= 0 && idx < len(m.items) {
		return m.items[idx], MenuSelected
	}
	return MenuItem{}, MenuNone
}

// BuildLayer renders the context menu as a lipgloss Layer.
func (m ContextMenu) BuildLayer() *lipgloss.Layer {
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1F2937")).
		Background(styles.PrimaryColor).
		Bold(true)
	normalStyle := lipgloss.NewStyle().
		Foreground(styles.TextColor)

	itemW := 0
	for _, item := range m.items {
		if w := len(item.Label); w > itemW {
			itemW = w
		}
	}

	var b strings.Builder
	for i, item := range m.items {
		if i > 0 {
			b.WriteByte('\n')
		}
		label := fmt.Sprintf("%-*s", itemW, item.Label)
		if i == m.highlight {
			b.WriteString(selectedStyle.Render(label))
		} else {
			b.WriteString(normalStyle.Render(label))
		}
	}

	content := OverlayStyle().Render(b.String())
	return lipgloss.NewLayer(content).
		ID("contextmenu").
		X(m.X).Y(m.Y).Z(55)
}
