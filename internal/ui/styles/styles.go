// SPDX-License-Identifier: MIT

package styles

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	// Colors
	PrimaryColor   = lipgloss.Color("#7C3AED")
	SecondaryColor = lipgloss.Color("#06B6D4")
	SuccessColor   = lipgloss.Color("#10B981")
	ErrorColor     = lipgloss.Color("#EF4444")
	WarningColor   = lipgloss.Color("#F59E0B")
	MutedColor     = lipgloss.Color("#6B7280")
	TextColor      = lipgloss.Color("#E5E7EB")
	BgColor        = lipgloss.Color("#1F2937")
	BorderColor    = lipgloss.Color("#374151")
	FocusBorderC   = lipgloss.Color("#7C3AED")

	// Method colors
	MethodColors = map[string]color.Color{
		"GET":     lipgloss.Color("#10B981"),
		"POST":    lipgloss.Color("#3B82F6"),
		"PUT":     lipgloss.Color("#F59E0B"),
		"PATCH":   lipgloss.Color("#8B5CF6"),
		"DELETE":  lipgloss.Color("#EF4444"),
		"HEAD":    lipgloss.Color("#06B6D4"),
		"OPTIONS": lipgloss.Color("#6B7280"),
	}

	// Panel styles
	FocusedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(FocusBorderC).
			PaddingLeft(1)

	NormalBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			PaddingLeft(1)

	// Status code styles
	StatusOK = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)

	StatusClientErr = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	StatusServerErr = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	StatusRedirect = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	StatusInfo = lipgloss.NewStyle().
			Foreground(MutedColor).
			Bold(true)

	// Tab styles
	ActiveTab = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Underline(true)

	InactiveTab = lipgloss.NewStyle().
			Foreground(MutedColor).
			Underline(true)

	// Misc
	MutedStyle = lipgloss.NewStyle().Foreground(MutedColor)
	BoldStyle  = lipgloss.NewStyle().Bold(true)
)

// TabDef describes a tab used for both configuration and rendering.
type TabDef struct {
	Name  string
	Key   string
	Index int
}

// RenderTabLayers creates child layers for a row of tabs.
// activeIndex indicates which tab is currently active.
// Returns the layers and the next X position after the last tab.
func RenderTabLayers(tabs []TabDef, activeIndex int, idPrefix string, startX, y int) ([]*lipgloss.Layer, int) {
	x := startX
	var layers []*lipgloss.Layer
	for _, t := range tabs {
		label := t.Name + " [Alt+" + t.Key + "]"
		var rendered string
		if t.Index == activeIndex {
			rendered = ActiveTab.Render(label)
		} else {
			rendered = InactiveTab.Render(label)
		}
		id := idPrefix + strings.ToLower(t.Name)
		layers = append(layers, lipgloss.NewLayer(rendered).
			ID(id).X(x).Y(y).Z(1))
		x += lipgloss.Width(rendered) + 2
	}
	return layers, x
}

// VScrollbar returns a vertical scrollbar column as a slice of strings (one per row).
// total is the total number of items, visible is the number of visible rows,
// and offset is the index of the first visible item.
// Returns nil if no scrollbar is needed (all items visible).
func VScrollbar(total, visible, offset int) []string {
	if total <= visible {
		return nil
	}

	thumbPos, thumbSize := ScrollbarThumb(total, visible, offset)

	trackStyle := lipgloss.NewStyle().Foreground(BorderColor)
	thumbStyle := lipgloss.NewStyle().Foreground(MutedColor)

	track := make([]string, visible)
	for i := range visible {
		if i >= thumbPos && i < thumbPos+thumbSize {
			track[i] = thumbStyle.Render("┃")
		} else {
			track[i] = trackStyle.Render("│")
		}
	}
	return track
}

// ScrollbarThumb returns the thumb position and size within a scrollbar track.
// Works for both vertical and horizontal scrollbars.
func ScrollbarThumb(total, visible, offset int) (thumbPos, thumbSize int) {
	if total <= visible {
		return 0, visible
	}
	thumbSize = visible * visible / total
	if thumbSize < 1 {
		thumbSize = 1
	}
	maxOffset := total - visible
	maxThumbPos := visible - thumbSize
	if maxOffset > 0 {
		thumbPos = offset * maxThumbPos / maxOffset
	}
	return
}

// RenderHScrollbar renders a horizontal scrollbar string.
func RenderHScrollbar(totalW, viewW, offset int) string {
	trackW := viewW
	if trackW <= 0 {
		return ""
	}
	thumbPos, thumbSize := ScrollbarThumb(totalW, viewW, offset)
	trackStyle := lipgloss.NewStyle().Foreground(BorderColor)
	thumbStyle := lipgloss.NewStyle().Foreground(MutedColor)
	var b strings.Builder
	for i := range trackW {
		if i >= thumbPos && i < thumbPos+thumbSize {
			b.WriteString(thumbStyle.Render("━"))
		} else {
			b.WriteString(trackStyle.Render("─"))
		}
	}
	return b.String()
}

// DragToOffset converts a thumb position (from drag) to a scroll offset.
// trackSize is the scrollbar track length, totalSize is the total content size,
// and thumbTop is the thumb's current top position in the track.
func DragToOffset(thumbTop, trackSize, totalSize int) int {
	if totalSize <= trackSize {
		return 0
	}
	thumbSize := trackSize * trackSize / totalSize
	if thumbSize < 1 {
		thumbSize = 1
	}
	maxThumbPos := trackSize - thumbSize
	maxOff := totalSize - trackSize
	if maxThumbPos <= 0 {
		return 0
	}
	offset := thumbTop * maxOff / maxThumbPos
	if offset < 0 {
		return 0
	}
	if offset > maxOff {
		return maxOff
	}
	return offset
}

func MethodStyle(method string) lipgloss.Style {
	color, ok := MethodColors[method]
	if !ok {
		color = MutedColor
	}
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true)
}

// BorderStyleForFocus returns FocusedBorder if focused, NormalBorder otherwise.
func BorderStyleForFocus(focused bool) lipgloss.Style {
	if focused {
		return FocusedBorder
	}
	return NormalBorder
}

// WrapToggleLabel returns the tab label for wrap/scroll mode toggle.
func WrapToggleLabel(wrapMode bool) string {
	if wrapMode {
		return " Wrap  [Ctrl+W]"
	}
	return "Scroll [Ctrl+W]"
}

func StatusCodeStyle(code int) lipgloss.Style {
	switch {
	case code >= 100 && code < 200:
		return StatusInfo
	case code >= 200 && code < 300:
		return StatusOK
	case code >= 300 && code < 400:
		return StatusRedirect
	case code >= 400 && code < 500:
		return StatusClientErr
	default:
		return StatusServerErr
	}
}
