// SPDX-License-Identifier: MIT

package http

import "time"

// Response represents an HTTP response.
type Response struct {
	StatusCode int
	Status     string
	Proto      string
	Headers    map[string][]string
	Body       string
	Duration   time.Duration
	Size       int64
}
