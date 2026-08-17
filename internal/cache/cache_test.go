package cache

import (
	"testing"
	"time"

	"github.com/piratecoder/go-figma-cli/internal/mcp"
)

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, time.Minute)

	res := &mcp.CallToolResult{Content: []mcp.Content{{Type: "text", Text: "hello"}}}
	key := Key("remote", "file", "1:2", "get_metadata")

	if err := c.Put(key, res); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected hit")
	}
	if got.TextParts() != "hello" {
		t.Fatalf("got %q", got.TextParts())
	}

	if _, ok := c.Get(Key("other")); ok {
		t.Fatal("expected miss")
	}
}

func TestCacheExpiry(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, time.Nanosecond)
	res := &mcp.CallToolResult{Content: []mcp.Content{{Type: "text", Text: "x"}}}
	key := Key("k")
	if err := c.Put(key, res); err != nil {
		t.Fatalf("put: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, ok := c.Get(key); ok {
		t.Fatal("expected expiry miss")
	}
}

func TestCacheClean(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, time.Nanosecond)
	_ = c.Put(Key("a"), &mcp.CallToolResult{})
	_ = c.Put(Key("b"), &mcp.CallToolResult{})
	time.Sleep(2 * time.Millisecond)
	if n := c.Clean(); n < 2 {
		t.Fatalf("clean removed %d, want >= 2", n)
	}
}
