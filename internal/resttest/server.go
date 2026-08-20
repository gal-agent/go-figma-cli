// Package resttest is an in-process mock Figma REST API for tests.
package resttest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// Server is a mock api.figma.com.
type Server struct {
	*httptest.Server

	// Token is the only accepted X-Figma-Token value ("" = accept any).
	Token string

	// FileFor maps fileKey -> File JSON (served by /v1/files/{key}).
	FileFor map[string]string
	// NodesFor maps fileKey -> NodesResponse JSON (/v1/files/{key}/nodes).
	NodesFor map[string]string
	// VarsFor maps fileKey -> VariablesResponse JSON.
	VarsFor map[string]string
	// ImageURLFor maps fileKey -> image URL (/v1/images/{key}).
	ImageURLFor map[string]string

	mu    sync.Mutex
	calls []string
}

// New starts a mock server.
func New() *Server {
	s := &Server{
		Token:       "test-pat",
		FileFor:     map[string]string{},
		NodesFor:    map[string]string{},
		VarsFor:     map[string]string{},
		ImageURLFor: map[string]string{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/me", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			http.Error(w, `{"status":403}`, 403)
			return
		}
		fmt.Fprint(w, `{"id":"1","email":"t@example.com","handle":"tester"}`)
	})
	mux.HandleFunc("/v1/files/", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			http.Error(w, `{"status":403}`, 403)
			return
		}
		rest := r.URL.Path[len("/v1/files/"):]
		key := rest
		kind := "file"
		for _, suffix := range []string{"/nodes", "/variables/local", "/variables/published"} {
			if strings.HasSuffix(rest, suffix) {
				key = strings.TrimSuffix(rest, suffix)
				kind = strings.TrimPrefix(suffix, "/")
				break
			}
		}
		s.record(kind + ":" + key)
		varsKey := key
		switch kind {
		case "file":
			if s.FileFor[key] != "" {
				fmt.Fprint(w, s.FileFor[key])
				return
			}
		case "nodes":
			if s.NodesFor[key] != "" && r.URL.Query().Get("ids") != "" {
				fmt.Fprint(w, s.NodesFor[key])
				return
			}
		default: // variables
			_ = varsKey
			if s.VarsFor[key] != "" {
				fmt.Fprint(w, s.VarsFor[key])
				return
			}
		}
		http.Error(w, `{"status":404,"err":"not found"}`, 404)
	})
	mux.HandleFunc("/v1/images/", func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			http.Error(w, `{"status":403}`, 403)
			return
		}
		key := r.URL.Path[len("/v1/images/"):]
		s.record("images:" + key)
		url, ok := s.ImageURLFor[key]
		if !ok {
			http.Error(w, `{"status":404}`, 404)
			return
		}
		fmt.Fprintf(w, `{"images":{"%s":"%s"}}`, r.URL.Query().Get("ids"), url)
	})
	mux.HandleFunc("/img/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		fmt.Fprint(w, "pngbytes")
	})
	s.Server = httptest.NewServer(mux)
	return s
}

func (s *Server) authorized(r *http.Request) bool {
	if s.Token == "" {
		return true
	}
	return r.Header.Get("X-Figma-Token") == s.Token
}

func (s *Server) record(call string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call)
}

// CallsSnapshot returns a copy of the recorded API calls.
func (s *Server) CallsSnapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calls))
	copy(out, s.calls)
	return out
}

// ResetCalls clears the call log (e.g. between cache assertions).
func (s *Server) ResetCalls() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = nil
}
