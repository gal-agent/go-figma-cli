package mcp

import (
	"strings"
	"testing"
)

func TestScanSSE(t *testing.T) {
	stream := ": ping\n\n" +
		"event: message\n" +
		"data: {\"id\":1}\n\n" +
		"data: line1\n" +
		"data: line2\n\n" +
		"garbagefield: x\n\n"

	var got [][]byte
	err := scanSSE(strings.NewReader(stream), func(d []byte) { got = append(got, d) })
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 events, got %d", len(got))
	}
	if string(got[0]) != `{"id":1}` {
		t.Fatalf("event0 = %q", got[0])
	}
	if string(got[1]) != "line1\nline2" {
		t.Fatalf("multiline join failed: %q", got[1])
	}
}

func TestExtractJSONEvent(t *testing.T) {
	stream := "data: not json\n\ndata: {\"a\":1}\n\n"
	data, ok := extractJSONEvent(strings.NewReader(stream))
	if !ok || string(data) != `{"a":1}` {
		t.Fatalf("got %q ok=%v", data, ok)
	}
}
