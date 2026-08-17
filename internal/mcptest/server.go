// Package mcptest provides an in-process mock MCP server for tests.
package mcptest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
)

// Server is a minimal Streamable-HTTP MCP server exercising the parts of the
// protocol figma-cli relies on.
type Server struct {
	*httptest.Server

	// Tools pages: page 1 -> Tools[0:half] + nextCursor, page 2 -> rest.
	Tools []string
	// Calls records "name:argjson" per tools/call.
	Calls []string
	// MetadataFor / DesignFor / VarsFor stub tool outputs (nodeID -> text).
	MetadataFor map[string]string
	DesignFor   map[string]string
	VarsFor     map[string]string
	// RespondSSE makes responses text/event-stream instead of JSON.
	RespondSSE bool
	// Always401 forces 401 on every request.
	Always401 bool
	// DropSessions invalidates all known session ids (to test re-init).
	DropSessions func()

	mu         sync.Mutex
	sessions   map[string]bool
	sessionSeq int
}

// New starts a server whose MCP endpoint is s.URL + "/mcp".
func New() *Server {
	s := &Server{
		Tools:       []string{"get_metadata", "get_design_context", "get_variable_defs", "get_screenshot"},
		MetadataFor: map[string]string{},
		DesignFor:   map[string]string{},
		VarsFor:     map[string]string{},
		sessions:    map[string]bool{},
	}
	s.DropSessions = func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.sessions = map[string]bool{}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handle)
	s.Server = httptest.NewServer(mux)
	return s
}

// Endpoint returns the MCP endpoint URL.
func (s *Server) Endpoint() string { return s.URL + "/mcp" }

type rpcIn struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if s.Always401 {
		w.Header().Set("WWW-Authenticate",
			`Bearer resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource"`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var in rpcIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}

	switch in.Method {
	case "initialize":
		sid := s.newSession()
		w.Header().Set("mcp-session-id", sid)
		s.reply(w, in.ID, map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": true}},
			"serverInfo":      map[string]any{"name": "mock-figma", "version": "9.9"},
		})
	case "notifications/initialized":
		if !s.checkSession(r) {
			http.Error(w, `{"error":{"code":-32000,"message":"session lost"}}`, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	case "tools/list":
		if !s.checkSession(r) {
			s.sessionLost(w)
			return
		}
		var params struct {
			Cursor string `json:"cursor"`
		}
		_ = json.Unmarshal(in.Params, &params)
		half := len(s.Tools) / 2
		if params.Cursor == "" {
			page := s.Tools[:half]
			if len(s.Tools) == 1 {
				page = s.Tools
			}
			s.reply(w, in.ID, map[string]any{"tools": toolList(page), "nextCursor": "p2"})
			return
		}
		s.reply(w, in.ID, map[string]any{"tools": toolList(s.Tools[half:])})
	case "tools/call":
		if !s.checkSession(r) {
			s.sessionLost(w)
			return
		}
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(in.Params, &params); err != nil {
			http.Error(w, "bad params", http.StatusBadRequest)
			return
		}
		s.record(params.Name, params.Arguments)
		nodeID, _ := params.Arguments["nodeId"].(string)
		switch params.Name {
		case "get_metadata", "get_code": // accept legacy alias too
			if nodeID == "" {
				s.reply(w, in.ID, callText("<pages><page id=\"0:1\" name=\"Page 1\"/></pages>"))
				return
			}
			if xml, ok := s.MetadataFor[nodeID]; ok {
				s.reply(w, in.ID, callText(xml))
				return
			}
			s.replyError(w, in.ID, "node not found: "+nodeID)
		case "get_design_context":
			if code, ok := s.DesignFor[nodeID]; ok {
				s.reply(w, in.ID, callText(code))
				return
			}
			s.replyError(w, in.ID, "node not found: "+nodeID)
		case "get_variable_defs":
			if v, ok := s.VarsFor[nodeID]; ok {
				s.reply(w, in.ID, callText(v))
				return
			}
			s.reply(w, in.ID, callText("{}"))
		case "get_screenshot":
			s.reply(w, in.ID, map[string]any{"content": []map[string]any{
				{"type": "image", "mimeType": "image/png", "data": "aGVsbG8="},
			}})
		default:
			s.replyError(w, in.ID, "unknown tool "+params.Name)
		}
	default:
		s.replyError(w, in.ID, "method not found: "+in.Method)
	}
}

func (s *Server) record(name string, args map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := json.Marshal(args)
	s.Calls = append(s.Calls, fmt.Sprintf("%s:%s", name, raw))
}

// CallsSnapshot returns a copy of the call log.
func (s *Server) CallsSnapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.Calls))
	copy(out, s.Calls)
	return out
}

func (s *Server) newSession() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionSeq++
	sid := fmt.Sprintf("sess-%d", s.sessionSeq)
	s.sessions[sid] = true
	return sid
}

func (s *Server) checkSession(r *http.Request) bool {
	sid := r.Header.Get("mcp-session-id")
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sid]
}

func (s *Server) sessionLost(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":{"code":-32000,"message":"session lost"}}`))
}

func toolList(names []string) []map[string]any {
	out := []map[string]any{}
	for _, n := range names {
		out = append(out, map[string]any{
			"name":        n,
			"description": "mock tool " + n,
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		})
	}
	return out
}

func callText(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

func (s *Server) reply(w http.ResponseWriter, id int64, result any) {
	payload := map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
	s.write(w, payload)
}

func (s *Server) replyError(w http.ResponseWriter, id int64, msg string) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": -32000, "message": msg},
	}
	s.write(w, payload)
}

func (s *Server) write(w http.ResponseWriter, payload any) {
	raw, _ := json.Marshal(payload)
	if s.RespondSSE {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(": keepalive\n\n"))
		_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
		_, _ = w.Write([]byte("data: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/ignored\"}\n\n"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}
