package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/gal-agent/go-figma-cli/internal/mcptest"
)

func newTestClient(t *testing.T, s *mcptest.Server) *Client {
	t.Helper()
	c := NewClient(s.Endpoint(), "")
	if _, err := c.Initialize(context.Background()); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return c
}

func TestClientJSONResponses(t *testing.T) {
	s := mcptest.New()
	s.MetadataFor["1:2"] = "<frame id=\"1:2\"/>"
	s.DesignFor["1:2"] = "<div/>"
	defer s.Close()

	c := newTestClient(t, s)
	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools across pages, got %d", len(tools))
	}
	res, err := c.CallTool(context.Background(), "get_design_context", map[string]any{"nodeId": "1:2"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if len(res.Content) == 0 {
		t.Fatal("no content")
	}
}

func TestClientSSEResponses(t *testing.T) {
	s := mcptest.New()
	s.MetadataFor["1:2"] = "<frame id=\"1:2\"/>"
	s.RespondSSE = true
	defer s.Close()

	c := newTestClient(t, s)
	tools, err := c.ListTools(context.Background())
	if err != nil {
		t.Fatalf("list over SSE: %v", err)
	}
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}
	res, err := c.CallTool(context.Background(), "get_metadata", map[string]any{"nodeId": "1:2"})
	if err != nil {
		t.Fatalf("call over SSE: %v", err)
	}
	if res.TextParts() == "" {
		t.Fatal("empty text over SSE")
	}
}

func TestClientUnauthorized(t *testing.T) {
	s := mcptest.New()
	s.Always401 = true
	defer s.Close()

	c := NewClient(s.Endpoint(), "")
	_, err := c.Initialize(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestClientSessionReInit(t *testing.T) {
	s := mcptest.New()
	s.MetadataFor["1:2"] = "<frame id=\"1:2\"/>"
	defer s.Close()

	c := newTestClient(t, s)
	s.DropSessions() // server forgets our session -> next call 404

	res, err := c.CallTool(context.Background(), "get_metadata", map[string]any{"nodeId": "1:2"})
	if err != nil {
		t.Fatalf("expected transparent re-init, got %v", err)
	}
	if res.TextParts() == "" {
		t.Fatal("no content after re-init")
	}
}

func TestCallToolRPCError(t *testing.T) {
	s := mcptest.New()
	defer s.Close()

	c := newTestClient(t, s)
	_, err := c.CallTool(context.Background(), "get_design_context", map[string]any{"nodeId": "999:999"})
	if err == nil {
		t.Fatal("expected RPC error for unknown node")
	}
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *RPCError, got %T: %v", err, err)
	}
}
