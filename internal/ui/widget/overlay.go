// SPDX-License-Identifier: MIT

package widget

import (
	"strings"

	"charm.land/lipgloss/v2"

	"rest-helper/internal/ui/styles"
)

// OverlayStyle returns the standard style for popup overlay windows.
func OverlayStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.PrimaryColor).
		Padding(0, 1)
}

// RenderKeyHelp renders a list of key-description pairs in a consistent style.
func RenderKeyHelp(keys [][2]string) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(styles.SecondaryColor).
		Bold(true).
		Width(16)
	descStyle := lipgloss.NewStyle().
		Foreground(styles.TextColor)

	var b strings.Builder
	for _, kv := range keys {
		b.WriteString("  ")
		b.WriteString(keyStyle.Render(kv[0]))
		b.WriteString(descStyle.Render(kv[1]))
		b.WriteByte('\n')
	}
	return b.String()
}

// ScrollbarLayer creates a vertical scrollbar child layer positioned inside
// the right border of an overlay whose rendered width is fullWidth.
// Returns nil if no scrollbar is needed.
func ScrollbarLayer(id string, total, visible, offset, fullWidth, sbY, z int) *lipgloss.Layer {
	if total <= visible {
		return nil
	}
	sb := styles.VScrollbar(total, visible, offset)
	sbStr := strings.Join(sb, "\n")
	sbX := fullWidth - 2 // inside right border
	return lipgloss.NewLayer(sbStr).ID(id).X(sbX).Y(sbY).Z(z)
}
