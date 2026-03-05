// SPDX-License-Identifier: MIT

package app

// FocusPanel identifies which panel has focus.
type FocusPanel int

const (
	FocusURLBar FocusPanel = iota
	FocusRequest
	FocusResponse
	FocusSidebar
)
