// SPDX-License-Identifier: MIT

package storage

import "time"

type HistoryEntry struct {
	ID              int64
	Method          string
	URL             string
	RequestHeaders  string // JSON
	RequestBody     string
	BodyFormat      string // "JSON" or "YAML"
	StatusCode      int
	ResponseProto   string // e.g. "HTTP/1.1"
	ResponseStatus  string // e.g. "200 OK"
	ResponseHeaders string // JSON
	ResponseBody    string
	ResponseTimeMs  int64
	ResponseSize    int64
	CreatedAt       time.Time
}
