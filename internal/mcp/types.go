// Package mcp implements a minimal MCP (Model Context Protocol) client:
// JSON-RPC 2.0 over Streamable HTTP, covering initialize / tools.list /
// tools.call with both application/json and text/event-stream responses.
package mcp

import "encoding/json"

// ProtocolVersion is the MCP revision this client speaks.
const ProtocolVersion = "2025-06-18"

// ---- JSON-RPC 2.0 envelope ----

// Request is a JSON-RPC request with an id (expects a response).
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Notification is a JSON-RPC notification (no id, no response).
type Notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// RPCError is a JSON-RPC error object.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return e.Message }

// Response is a JSON-RPC response. Result or Error is set, not both.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// ---- MCP domain types ----

// InitializeResult is the result of the MCP initialize handshake.
type InitializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	Capabilities    struct {
		Tools *struct {
			ListChanged bool `json:"listChanged,omitempty"`
		} `json:"tools,omitempty"`
	} `json:"capabilities"`
	ServerInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

// Tool describes a single MCP tool as returned by tools/list.
type Tool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ListToolsResult is one page of tools/list.
type ListToolsResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// CallToolParams is the params object for tools/call.
type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ResourceContent is embedded in resource-typed content blocks.
type ResourceContent struct {
	URI      string `json:"uri,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // base64
}

// Content is one content block of a CallToolResult.
type Content struct {
	Type     string           `json:"type"` // "text" | "image" | "resource"
	Text     string           `json:"text,omitempty"`
	Data     string           `json:"data,omitempty"` // base64, image blocks
	MimeType string           `json:"mimeType,omitempty"`
	Resource *ResourceContent `json:"resource,omitempty"`
}

// CallToolResult is the result of tools/call.
type CallToolResult struct {
	Content           []Content       `json:"content,omitempty"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
}

// TextParts returns all text-block text concatenated, in order.
func (r *CallToolResult) TextParts() string {
	out := ""
	for _, c := range r.Content {
		if c.Type == "text" && c.Text != "" {
			if out != "" {
				out += "\n"
			}
			out += c.Text
		}
		if c.Type == "resource" && c.Resource != nil && c.Resource.Text != "" {
			if out != "" {
				out += "\n"
			}
			out += c.Resource.Text
		}
	}
	return out
}
