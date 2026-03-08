// SPDX-License-Identifier: MIT

package widget

import "rest-helper/internal/ui/styles"

// ScrollDrag tracks scrollbar drag state (works for both vertical and horizontal).
// Embed this struct to add drag support to any scrollable component.
type ScrollDrag struct {
	dragging   bool
	base       int // screen coordinate of scrollbar origin when drag started
	grabOffset int // distance from thumb start to grab point
}

// Start begins a drag, recording the scrollbar's screen origin and grab offset.
func (d *ScrollDrag) Start(base, grabOffset int) {
	d.dragging = true
	d.base = base
	d.grabOffset = grabOffset
}

// Stop ends the drag.
func (d *ScrollDrag) Stop() {
	d.dragging = false
}

// Active returns whether a drag is in progress.
func (d ScrollDrag) Active() bool {
	return d.dragging
}

// Base returns the screen coordinate recorded at drag start.
func (d ScrollDrag) Base() int {
	return d.base
}

// GrabOffset returns the distance from thumb start to the grab point.
func (d ScrollDrag) GrabOffset() int {
	return d.grabOffset
}

// HandleClick processes a scrollbar click at localPos (relative to scrollbar origin).
// base is the screen coordinate of the scrollbar origin.
// total is the total content size, visible is the viewport size, and offset is the
// current scroll offset.
// If the click is on the thumb, drag is started and 0 is returned.
// If the click is before the thumb, -visible is returned (page backward).
// If the click is after the thumb, +visible is returned (page forward).
func (d *ScrollDrag) HandleClick(localPos, base, total, visible, offset int) int {
	thumbPos, thumbSize := styles.ScrollbarThumb(total, visible, offset)
	if localPos >= thumbPos && localPos < thumbPos+thumbSize {
		d.Start(base, localPos-thumbPos)
		return 0
	}
	if localPos < thumbPos {
		return -visible
	}
	return visible
}

// DragOffset computes the thumb position from a mouse coordinate during drag.
// mousePos is the current mouse screen coordinate (X or Y).
func (d ScrollDrag) DragOffset(mousePos, trackSize, totalSize int) int {
	thumbPos := mousePos - d.base - d.grabOffset
	return styles.DragToOffset(thumbPos, trackSize, totalSize)
}

// HandleClickAndClamp processes a scrollbar click and returns the new clamped
// scroll offset. It handles thumb drag initiation and page-up/page-down jumps.
// Returns the new offset and true if the offset changed; (0, false) if a drag
// was started (caller should not change offset).
func (d *ScrollDrag) HandleClickAndClamp(localPos, base, total, visible, offset int) (int, bool) {
	delta := d.HandleClick(localPos, base, total, visible, offset)
	if delta == 0 {
		return 0, false
	}
	newOff := offset + delta
	if newOff < 0 {
		newOff = 0
	}
	if maxOff := total - visible; newOff > maxOff {
		newOff = maxOff
	}
	return newOff, true
}
