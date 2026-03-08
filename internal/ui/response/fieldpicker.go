// SPDX-License-Identifier: MIT

package response

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"rest-helper/internal/clipboard"
	"rest-helper/internal/ui/styles"
	"rest-helper/internal/ui/widget"
)

// FieldCopiedMsg is sent when a field value has been copied to clipboard.
type FieldCopiedMsg struct {
	Path  string
	Error error
}

// FieldPickerClosedMsg is sent when the field picker is dismissed.
type FieldPickerClosedMsg struct{}

// PathValue holds a flattened JSON field's path and value for the field picker.
type PathValue struct {
	path    string
	value   string // raw JSON value for copying
	display string // truncated display string
}

var (
	fpSelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1F2937")).
			Background(styles.PrimaryColor).
			Bold(true)
	fpPathStyle = lipgloss.NewStyle().Foreground(styles.SecondaryColor)
	fpValStyle  = lipgloss.NewStyle().Foreground(styles.TextColor)
)

// fieldPickerContent implements widget.PickerContent for JSON field copying.
type fieldPickerContent struct{}

func (c *fieldPickerContent) MatchFilter(item PathValue, query string) bool {
	return strings.Contains(strings.ToLower(item.path), query) ||
		strings.Contains(strings.ToLower(item.display), query)
}

func (c *fieldPickerContent) RenderItem(item PathValue, isCursor bool, width int) string {
	sep := " = "
	plain := item.path + sep + item.display
	truncated := ansi.StringWidth(plain) > width
	if truncated {
		plain = ansi.Truncate(plain, width-1, "…")
	}
	if isCursor {
		return fpSelectedStyle.Render(fmt.Sprintf("%-*s", width, plain))
	}
	if truncated {
		// After truncation, path+sep may exceed plain length; render as single styled string
		prefixLen := len(item.path) + len(sep)
		if prefixLen > len(plain) {
			return fpPathStyle.Render(plain)
		}
		return fpPathStyle.Render(item.path) + fpValStyle.Render(plain[len(item.path):])
	}
	return fpPathStyle.Render(item.path) + fpValStyle.Render(sep+item.display)
}

func (c *fieldPickerContent) Title(_, _ int) string {
	return styles.BoldStyle.Foreground(styles.PrimaryColor).
		Render("Copy Field")
}

func (c *fieldPickerContent) Footer(filtered, total int) string {
	countInfo := ""
	if filtered != total {
		countInfo = fmt.Sprintf(" (%d/%d)", filtered, total)
	}
	return styles.MutedStyle.Render("↑↓ select │ enter copy │ ? help │ esc close" + countInfo)
}

func (c *fieldPickerContent) EmptyText() string {
	return styles.MutedStyle.Render("  No matching fields")
}

func (c *fieldPickerContent) HandleKey(key string, item *PathValue) (tea.Cmd, widget.PickerAction) {
	switch key {
	case "esc":
		return func() tea.Msg { return FieldPickerClosedMsg{} }, widget.PickerHandled
	case "enter":
		if item == nil {
			return nil, widget.PickerHandled
		}
		err := clipboard.Write(item.value)
		path := item.path
		return func() tea.Msg {
			return FieldCopiedMsg{Path: path, Error: err}
		}, widget.PickerHandled
	}
	return nil, widget.PickerNotHandled
}

func (c *fieldPickerContent) HandleMsg(_ tea.Msg) (tea.Cmd, []PathValue, bool) {
	return nil, nil, false
}

func (c *fieldPickerContent) HelpContent() [][2]string {
	return [][2]string{
		{"/ (type)", "Filter fields"},
		{"↑ / ↓", "Navigate"},
		{"Enter", "Copy value to clipboard"},
		{"Esc", "Close picker"},
	}
}

// NewFieldPicker creates a field picker overlay for the given JSON body.
func NewFieldPicker(body string, width, height int) *widget.OverlayPicker[PathValue] {
	var items []PathValue

	// First item: entire body
	items = append(items, PathValue{
		path:    ".",
		value:   body,
		display: "(entire body)",
	})

	// Flatten JSON
	var data any
	if err := json.Unmarshal([]byte(body), &data); err == nil {
		flattenJSON("", data, &items)
	}

	// Compute content width from all items (fixed, not affected by filter)
	cw := len("↑↓ select │ enter copy │ ? help │ esc close")
	if tw := lipgloss.Width(styles.BoldStyle.Foreground(styles.PrimaryColor).Render("Copy Field")); tw > cw {
		cw = tw
	}
	for _, item := range items {
		pw := len(item.path) + 3 + len(item.display) // " = "
		if pw > cw {
			cw = pw
		}
	}

	return widget.NewOverlayPicker(&fieldPickerContent{}, items, "Filter fields...", width, height, 7, cw, "fp-vscrollbar")
}

func flattenJSON(prefix string, v any, out *[]PathValue) {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			path := prefix + "." + k
			flattenJSON(path, val[k], out)
		}
	case []any:
		for i, child := range val {
			path := fmt.Sprintf("%s[%d]", prefix, i)
			flattenJSON(path, child, out)
		}
	default:
		raw, _ := json.Marshal(val)
		rawStr := string(raw)
		// For strings, store unquoted value for copying
		copyVal := rawStr
		if s, ok := val.(string); ok {
			copyVal = s
		}
		*out = append(*out, PathValue{
			path:    prefix,
			value:   copyVal,
			display: rawStr,
		})
	}
}
