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
			BorderForeground(FocusBorderC)

	NormalBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor)

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

func MethodStyle(method string) lipgloss.Style {
	color, ok := MethodColors[method]
	if !ok {
		color = MutedColor
	}
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true)
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
