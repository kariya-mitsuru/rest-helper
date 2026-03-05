// SPDX-License-Identifier: MIT

package http

import (
	"bytes"
	"fmt"
	"io"
	gohttp "net/http"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ResponseMsg is sent when an HTTP response is received.
type ResponseMsg struct {
	Response *Response
	Err      error
}

// Send executes an HTTP request asynchronously as a tea.Cmd.
func Send(req Request) tea.Cmd {
	return func() tea.Msg {
		url := req.URL
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = "https://" + url
		}

		var bodyReader io.Reader
		if req.Body != "" {
			bodyReader = bytes.NewBufferString(req.Body)
		}

		httpReq, err := gohttp.NewRequest(req.Method, url, bodyReader)
		if err != nil {
			return ResponseMsg{Err: fmt.Errorf("request creation failed: %w", err)}
		}

		for k, v := range req.Headers {
			httpReq.Header.Set(k, v)
		}

		if req.Body != "" && httpReq.Header.Get("Content-Type") == "" {
			httpReq.Header.Set("Content-Type", "application/json")
		}

		client := &gohttp.Client{
			Timeout: 30 * time.Second,
		}

		start := time.Now()
		resp, err := client.Do(httpReq)
		duration := time.Since(start)

		if err != nil {
			return ResponseMsg{Err: fmt.Errorf("request failed: %w", err)}
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return ResponseMsg{Err: fmt.Errorf("reading response body: %w", err)}
		}

		headers := make(map[string][]string)
		for k, v := range resp.Header {
			headers[k] = v
		}

		return ResponseMsg{
			Response: &Response{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Proto:      resp.Proto,
				Headers:    headers,
				Body:       string(body),
				Duration:   duration,
				Size:       int64(len(body)),
			},
		}
	}
}
