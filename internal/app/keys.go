// SPDX-License-Identifier: MIT

package app

import "charm.land/bubbles/v2/key"

type KeyMap struct {
	Send              key.Binding
	Method            key.Binding
	Tab               key.Binding
	ShiftTab          key.Binding
	Quit              key.Binding
	QuitAlt           key.Binding
	URLBar            key.Binding
	HeaderTab         key.Binding
	BodyTab           key.Binding
	AuthTab           key.Binding
	HistoryTab        key.Binding
	ResponseBodyTab   key.Binding
	ResponseHeaderTab key.Binding
	Help              key.Binding
}

var Keys = KeyMap{
	Send: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "send request"),
	),
	Method: key.NewBinding(
		key.WithKeys("ctrl+p"),
		key.WithHelp("ctrl+p", "change method"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next panel"),
	),
	ShiftTab: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev panel"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	),
	QuitAlt: key.NewBinding(
		key.WithKeys("ctrl+q"),
		key.WithHelp("ctrl+q", "quit"),
	),
	URLBar: key.NewBinding(
		key.WithKeys("alt+u"),
		key.WithHelp("alt+u", "URL bar"),
	),
	HeaderTab: key.NewBinding(
		key.WithKeys("alt+e"),
		key.WithHelp("alt+e", "headers tab"),
	),
	BodyTab: key.NewBinding(
		key.WithKeys("alt+b"),
		key.WithHelp("alt+b", "body tab"),
	),
	AuthTab: key.NewBinding(
		key.WithKeys("alt+a"),
		key.WithHelp("alt+a", "auth tab"),
	),
	HistoryTab: key.NewBinding(
		key.WithKeys("alt+h"),
		key.WithHelp("alt+h", "history"),
	),
	ResponseBodyTab: key.NewBinding(
		key.WithKeys("alt+r"),
		key.WithHelp("alt+r", "response body"),
	),
	ResponseHeaderTab: key.NewBinding(
		key.WithKeys("alt+d"),
		key.WithHelp("alt+d", "response headers"),
	),
	Help: key.NewBinding(
		key.WithKeys("ctrl+?", "f1", "?"),
		key.WithHelp("?/F1", "help"),
	),
}
