// Package auth implements the OAuth2 dance the remote Figma MCP server
// requires: RFC 9728 protected-resource discovery, RFC 7591 dynamic client
// registration when offered, and an authorization-code + PKCE (S256)
// loopback flow. Tokens are cached on disk.
package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Endpoint holds the discovered OAuth endpoints and client credentials.
type Endpoint struct {
	AuthorizationURL string `json:"authorization_endpoint"`
	TokenURL         string `json:"token_endpoint"`
	RegistrationURL  string `json:"registration_endpoint,omitempty"`
	ClientID         string `json:"client_id"`
	ClientSecret     string `json:"client_secret,omitempty"` // DCR "none" servers leave this empty
}

// Token is the persisted credential.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

// Valid reports whether the token can still be used without refresh.
func (t *Token) Valid() bool {
	return t != nil && t.AccessToken != "" &&
		(t.ExpiresAt.IsZero() || time.Now().Add(30*time.Second).Before(t.ExpiresAt))
}

// Store persists a token as JSON with 0600 permissions.
type Store struct{ Path string }

// DefaultStorePath returns ~/.config/figma-cli/auth.json
// (or $FIGMA_CLI_CONFIG_HOME/auth.json when that env var is set).
func DefaultStorePath() string {
	if env := os.Getenv("FIGMA_CLI_CONFIG_HOME"); env != "" {
		return filepath.Join(env, "auth.json")
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "figma-cli", "auth.json")
}

// Load reads the stored token, if any.
func (s *Store) Load() (*Token, error) {
	raw, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var t Token
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("auth store corrupt: %w", err)
	}
	return &t, nil
}

// Save writes the token with tight permissions.
func (s *Store) Save(t *Token) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, raw, 0o600)
}

// Clear removes the stored token.
func (s *Store) Clear() error { return os.Remove(s.Path) }

// ---- discovery ----

type protectedResource struct {
	AuthorizationServers []string `json:"authorization_servers"`
}

// ServerMetadata is the OAuth authorization-server metadata we need.
type ServerMetadata struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	RegistrationEndpoint  string `json:"registration_endpoint"`
}

// Discover walks: resource server -> authorization servers[0] -> AS metadata.
// baseURL is the MCP endpoint (e.g. https://mcp.figma.com/mcp).
func Discover(ctx context.Context, hc *http.Client, baseURL string) (*ServerMetadata, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	prURL := fmt.Sprintf("%s://%s/.well-known/oauth-protected-resource", u.Scheme, u.Host)
	var pr protectedResource
	if err := getJSON(ctx, hc, prURL, &pr); err != nil {
		return nil, fmt.Errorf("protected resource metadata: %w", err)
	}
	if len(pr.AuthorizationServers) == 0 {
		return nil, fmt.Errorf("no authorization_servers at %s", prURL)
	}
	as := strings.TrimRight(pr.AuthorizationServers[0], "/")
	var meta ServerMetadata
	// RFC 8414: well-known directly on the issuer...
	wellKnown := as + "/.well-known/oauth-authorization-server"
	err = getJSON(ctx, hc, wellKnown, &meta)
	if err != nil {
		// ...or path-inserted before the issuer's path component.
		if u2, perr := url.Parse(as); perr == nil && u2.Path != "" && u2.Path != "/" {
			inserted := fmt.Sprintf("%s://%s/.well-known/oauth-authorization-server%s", u2.Scheme, u2.Host, u2.Path)
			if ierr := getJSON(ctx, hc, inserted, &meta); ierr != nil {
				return nil, fmt.Errorf("authorization server metadata: %w", err)
			}
		} else {
			return nil, fmt.Errorf("authorization server metadata: %w", err)
		}
	}
	if meta.AuthorizationEndpoint == "" || meta.TokenEndpoint == "" {
		return nil, fmt.Errorf("incomplete AS metadata from %s", wellKnown)
	}
	return &meta, nil
}

// Register performs dynamic client registration when the AS offers it.
// Returns the endpoint with ClientID filled in, or an error explaining why.
func Register(ctx context.Context, hc *http.Client, meta *ServerMetadata) (*Endpoint, error) {
	if meta.RegistrationEndpoint == "" {
		return nil, fmt.Errorf("AS offers no dynamic client registration; figma-cli needs a pre-issued client_id for %s", meta.AuthorizationEndpoint)
	}
	body := map[string]any{
		"client_name":                "figma-cli",
		"redirect_uris":              []string{"http://localhost"},
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.RegistrationEndpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dynamic registration failed: HTTP %d: %s", resp.StatusCode, truncate(string(payload), 300))
	}
	var reg struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(payload, &reg); err != nil || reg.ClientID == "" {
		return nil, fmt.Errorf("dynamic registration: unexpected response: %s", truncate(string(payload), 300))
	}
	return &Endpoint{
		AuthorizationURL: meta.AuthorizationEndpoint,
		TokenURL:         meta.TokenEndpoint,
		RegistrationURL:  meta.RegistrationEndpoint,
		ClientID:         reg.ClientID,
		ClientSecret:     reg.ClientSecret,
	}, nil
}

// ---- PKCE loopback login ----

// Login runs the loopback authorization-code + PKCE flow. It prints the
// authorization URL, tries to open the browser, and waits for the callback.
func Login(ctx context.Context, hc *http.Client, ep *Endpoint, store *Store) (*Token, error) {
	verifier, err := randomToken(64)
	if err != nil {
		return nil, err
	}
	challenge := s256(verifier)
	state, err := randomToken(24)
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("loopback listener: %w", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	authURL := ep.AuthorizationURL + "?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {ep.ClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {"mcp:connect offline_access"},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode()

	fmt.Printf("Authorize figma-cli:\n\n  %s\n\n", authURL)
	if uerr := openBrowser(authURL); uerr != nil {
		fmt.Println("Open the URL above in your browser manually.")
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			fmt.Fprintf(w, "authorization failed: %s", e)
			errCh <- fmt.Errorf("authorization failed: %s: %s", e, q.Get("error_description"))
			return
		}
		if q.Get("state") != state {
			http.Error(w, "bad state", http.StatusBadRequest)
			errCh <- errors.New("state mismatch in callback")
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- errors.New("callback missing code")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h3>figma-cli authorized. You can close this tab.</h3></body></html>")
		codeCh <- code
	})}
	go func() { _ = srv.Serve(ln) }()
	defer func() { shutdown(srv) }()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, errors.New("timed out waiting for browser callback (5m)")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	tok, err := exchangeCode(ctx, hc, ep, code, verifier, redirectURI)
	if err != nil {
		return nil, err
	}
	if err := store.Save(tok); err != nil {
		return nil, fmt.Errorf("persist token: %w", err)
	}
	return tok, nil
}

// Refresh exchanges a refresh token for a fresh access token.
func Refresh(ctx context.Context, hc *http.Client, ep *Endpoint, store *Store, t *Token) (*Token, error) {
	if t == nil || t.RefreshToken == "" {
		return nil, errors.New("no refresh token stored; run figma login")
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {t.RefreshToken},
		"client_id":     {ep.ClientID},
	}
	if ep.ClientSecret != "" {
		form.Set("client_secret", ep.ClientSecret)
	}
	raw, err := postForm(ctx, hc, ep.TokenURL, form)
	if err != nil {
		return nil, err
	}
	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, fmt.Errorf("refresh: decode: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("refresh: no access_token in response")
	}
	nt := &Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: firstNonEmpty(tr.RefreshToken, t.RefreshToken),
		TokenType:    tr.TokenType,
	}
	if tr.ExpiresIn > 0 {
		nt.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	if err := store.Save(nt); err != nil {
		return nil, err
	}
	return nt, nil
}

func exchangeCode(ctx context.Context, hc *http.Client, ep *Endpoint, code, verifier, redirectURI string) (*Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {ep.ClientID},
		"code_verifier": {verifier},
	}
	if ep.ClientSecret != "" {
		form.Set("client_secret", ep.ClientSecret)
	}
	raw, err := postForm(ctx, hc, ep.TokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, fmt.Errorf("token exchange: decode: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("token exchange: no access_token (error=%s description=%s)", tr.Error, tr.ErrorDescription)
	}
	t := &Token{AccessToken: tr.AccessToken, RefreshToken: tr.RefreshToken, TokenType: tr.TokenType}
	if tr.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	return t, nil
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	TokenType        string `json:"token_type,omitempty"`
	ExpiresIn        int    `json:"expires_in,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func postForm(ctx context.Context, hc *http.Client, tokenURL string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	return raw, nil
}

func getJSON(ctx context.Context, hc *http.Client, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	return json.Unmarshal(raw, v)
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func s256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func shutdown(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
