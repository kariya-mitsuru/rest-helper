// SPDX-License-Identifier: MIT

package http

// Request represents an outgoing HTTP request.
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
}
