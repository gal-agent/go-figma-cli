package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

// ErrUnauthorized is returned on HTTP 401/402 from the server. Callers can
// inspect the message for the WWW-Authenticate header (RFC 9728 resource
// metadata) to kick off OAuth discovery.
var ErrUnauthorized = errors.New("mcp: unauthorized (HTTP 401/402)")

// ErrSessionLost is returned when the server rejects our session id; the
// client retries initialize once before surfacing it.
var ErrSessionLost = errors.New("mcp: session lost or expired")

// Client is a Streamable-HTTP MCP client bound to one server endpoint.
type Client struct {
	BaseURL string
	Token   string // bearer token, optional (desktop mode needs none)

	HTTP *http.Client

	sessionID string
	nextID    int64

	ServerInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
}

// NewClient builds a client for baseURL using the default http client.
func NewClient(baseURL, token string) *Client {
	return &Client{BaseURL: baseURL, Token: token, HTTP: http.DefaultClient}
}

// Initialize performs the MCP handshake: initialize request, capture session
// id, then send the initialized notification.
func (c *Client) Initialize(ctx context.Context) (*InitializeResult, error) {
	params := map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "figma-cli", "version": "0.1.0"},
	}
	resp, hdr, err := c.post(ctx, &Request{JSONRPC: "2.0", Method: "initialize", Params: params})
	if err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("initialize: %s", resp.Error.Message)
	}
	var result InitializeResult
	if len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return nil, fmt.Errorf("initialize: decode result: %w", err)
		}
	}
	c.ServerInfo = result.ServerInfo
	if sid := hdr.Get("mcp-session-id"); sid != "" {
		c.sessionID = sid
	}
	if _, _, err := c.post(ctx, &Notification{JSONRPC: "2.0", Method: "notifications/initialized"}); err != nil {
		return nil, fmt.Errorf("initialized notification: %w", err)
	}
	return &result, nil
}

// SessionID exposes the negotiated session id (empty when the server did not
// issue one).
func (c *Client) SessionID() string { return c.sessionID }

// ListTools returns the full tool list, following cursor pagination.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var all []Tool
	cursor := ""
	for {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		resp, _, err := c.post(ctx, &Request{JSONRPC: "2.0", Method: "tools/list", Params: params})
		if err != nil {
			return nil, fmt.Errorf("tools/list: %w", err)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("tools/list: %s", resp.Error.Message)
		}
		var page ListToolsResult
		if err := json.Unmarshal(resp.Result, &page); err != nil {
			return nil, fmt.Errorf("tools/list: decode: %w", err)
		}
		all = append(all, page.Tools...)
		if page.NextCursor == "" {
			return all, nil
		}
		cursor = page.NextCursor
	}
}

// CallTool invokes a tool by name.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	if args == nil {
		args = map[string]any{}
	}
	resp, err := c.callToolWithRetry(ctx, name, args)
	if err != nil {
		return nil, err
	}
	var result CallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("tools/call %s: decode: %w", name, err)
	}
	if result.IsError {
		msg := result.TextParts()
		if msg == "" {
			msg = fmt.Sprintf("tool %s returned isError", name)
		}
		return nil, fmt.Errorf("tool %s failed: %s", name, truncate(msg, 400))
	}
	return &result, nil
}

func (c *Client) callToolWithRetry(ctx context.Context, name string, args map[string]any) (*Response, error) {
	resp, _, err := c.callToolOnce(ctx, name, args)
	if errors.Is(err, ErrSessionLost) && c.sessionID != "" {
		// One transparent re-handshake, then retry the call once.
		c.sessionID = ""
		if _, ierr := c.Initialize(ctx); ierr != nil {
			return nil, fmt.Errorf("re-initialize after session loss: %w", ierr)
		}
		resp, _, err = c.callToolOnce(ctx, name, args)
	}
	return resp, err
}

func (c *Client) callToolOnce(ctx context.Context, name string, args map[string]any) (*Response, http.Header, error) {
	return c.post(ctx, &Request{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params:  CallToolParams{Name: name, Arguments: args},
	})
}

// post sends one JSON-RPC message and returns the matching response.
// Request IDs are assigned here (monotonic) and used to pick the right
// message out of an SSE stream (which may interleave notifications).
func (c *Client) post(ctx context.Context, msg any) (*Response, http.Header, error) {
	sentID := int64(0)
	if req, ok := msg.(*Request); ok {
		sentID = atomic.AddInt64(&c.nextID, 1)
		req.ID = sentID
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", "figma-cli/0.1.0")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.sessionID != "" {
		req.Header.Set("mcp-session-id", c.sessionID)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusPaymentRequired:
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, resp.Header, fmt.Errorf("%w: www-authenticate=%q", ErrUnauthorized, resp.Header.Get("WWW-Authenticate"))
	case resp.StatusCode == http.StatusNotFound && c.sessionID != "":
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, resp.Header, ErrSessionLost
	case resp.StatusCode >= 400:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, resp.Header, fmt.Errorf("mcp: HTTP %d: %s", resp.StatusCode, truncate(string(snippet), 300))
	}

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, resp.Header, err
	}
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		// Accepted notification (202) or empty 200: nothing to match.
		return &Response{JSONRPC: "2.0"}, resp.Header, nil
	}

	ctype := resp.Header.Get("Content-Type")
	var rpcResp *Response
	if strings.Contains(ctype, "text/event-stream") {
		rpcResp, err = matchSSE(payload, sentID)
		if err != nil {
			return nil, resp.Header, fmt.Errorf("mcp: sse: %w", err)
		}
	} else {
		var decoded Response
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return nil, resp.Header, fmt.Errorf("mcp: decode response: %w", err)
		}
		rpcResp = &decoded
	}
	if rpcResp.Error != nil {
		return nil, resp.Header, rpcResp.Error
	}
	return rpcResp, resp.Header, nil
}

// matchSSE picks the JSON-RPC response with the given id out of an SSE body,
// ignoring interleaved notifications (no id) and unrelated messages.
func matchSSE(payload []byte, wantID int64) (*Response, error) {
	var matched *Response
	scanErr := scanSSE(bytes.NewReader(payload), func(data []byte) {
		if matched != nil {
			return
		}
		var candidate Response
		if json.Unmarshal(data, &candidate) != nil {
			return
		}
		if candidate.ID == wantID {
			matched = &candidate
		}
	})
	if scanErr != nil {
		return nil, scanErr
	}
	if matched == nil {
		return nil, fmt.Errorf("stream carried no response for request id %d", wantID)
	}
	return matched, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
