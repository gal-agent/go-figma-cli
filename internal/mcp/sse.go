package mcp

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// scanSSE reads a text/event-stream body and invokes handle once per
// dispatched event with the event's accumulated data payload (multi-line
// data fields are joined with "\n" per the SSE spec). Comment lines
// (starting with ":") and unknown fields are ignored.
func scanSSE(body io.Reader, handle func(data []byte)) error {
	sc := bufio.NewScanner(body)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	var dataLines []string
	dispatch := func() {
		if len(dataLines) == 0 {
			return
		}
		handle([]byte(strings.Join(dataLines, "\n")))
		dataLines = nil
	}

	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			dispatch()
		case strings.HasPrefix(line, ":"):
			// keep-alive comment
		case strings.HasPrefix(line, "data:"):
			v := strings.TrimPrefix(line, "data:")
			v = strings.TrimPrefix(v, " ")
			dataLines = append(dataLines, v)
		default:
			// event:/id:/retry: and vendor fields - not needed here
		}
	}
	dispatch()
	return sc.Err()
}

// extractJSONEvent is a helper for tests: pull the first data payload that
// looks like a JSON object from an SSE stream.
func extractJSONEvent(body io.Reader) ([]byte, bool) {
	var found []byte
	_ = scanSSE(body, func(data []byte) {
		if found == nil && len(data) > 0 && data[0] == '{' {
			found = bytes.TrimSpace(data)
		}
	})
	return found, found != nil
}
