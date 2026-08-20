package restapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// DefaultBaseURL is the Figma REST API root.
const DefaultBaseURL = "https://api.figma.com"

// Error is a non-2xx REST response.
type Error struct {
	Path   string
	Status int
	Body   string
}

func (e *Error) Error() string {
	return fmt.Sprintf("figma REST %s: HTTP %d: %s", e.Path, e.Status, truncate(e.Body, 300))
}

// Client talks to api.figma.com with a personal access token.
type Client struct {
	hc      *http.Client
	baseURL string
	token   string
}

// New builds a client. baseURL may be empty for DefaultBaseURL (or the
// FIGMA_REST_BASE_URL env override); hc may be nil.
func New(baseURL, token string, hc *http.Client) *Client {
	if baseURL == "" {
		if env := os.Getenv("FIGMA_REST_BASE_URL"); env != "" {
			baseURL = env
		} else {
			baseURL = DefaultBaseURL
		}
	}
	if hc == nil {
		hc = &http.Client{Timeout: 120 * time.Second}
	}
	return &Client{hc: hc, baseURL: strings.TrimRight(baseURL, "/"), token: token}
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Figma-Token", c.token)
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Error{Path: path, Status: resp.StatusCode, Body: string(raw)}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// File fetches the file document (pages as canvas children of the root).
func (c *Client) File(ctx context.Context, key string) (*File, error) {
	var f File
	if err := c.get(ctx, "/v1/files/"+key, nil, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// Nodes fetches one node subtree. depth <= 0 means "no depth limit".
func (c *Client) Nodes(ctx context.Context, key, nodeID string, depth int) (*NodeDoc, error) {
	q := url.Values{"ids": {nodeID}}
	if depth > 0 {
		q.Set("depth", fmt.Sprint(depth))
	}
	var nr NodesResponse
	if err := c.get(ctx, "/v1/files/"+key+"/nodes", q, &nr); err != nil {
		return nil, err
	}
	doc, ok := nr.Nodes[nodeID]
	if !ok {
		// Server may normalize the id (e.g. strip to integer form).
		for _, v := range nr.Nodes {
			doc = v
			ok = true
			break
		}
	}
	if !ok {
		return nil, fmt.Errorf("node %s not found in file %s", nodeID, key)
	}
	return &doc, nil
}

// ImageURL renders one node and returns the fetch URL of the image.
func (c *Client) ImageURL(ctx context.Context, key, nodeID, format string, scale float64) (string, error) {
	q := url.Values{"ids": {nodeID}, "format": {format}}
	if scale > 0 {
		q.Set("scale", fmt.Sprint(scale))
	}
	var ir ImagesResponse
	if err := c.get(ctx, "/v1/images/"+key, q, &ir); err != nil {
		return "", err
	}
	u, ok := ir.Images[nodeID]
	if !ok {
		for _, v := range ir.Images {
			u, ok = v, true
			break
		}
	}
	if !ok || u == "" {
		return "", fmt.Errorf("no image rendered for node %s in file %s", nodeID, key)
	}
	if strings.HasPrefix(u, "error:") {
		return "", fmt.Errorf("figma could not render node %s: %s", nodeID, strings.TrimPrefix(u, "error:"))
	}
	return u, nil
}

// Fetch downloads arbitrary bytes (image render URLs).
func (c *Client) Fetch(ctx context.Context, rawURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, "", &Error{Path: rawURL, Status: resp.StatusCode, Body: string(raw)}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	return body, resp.Header.Get("Content-Type"), err
}

// Variables fetches local variables merged with published library variables
// when available. Published variables carry collection-level "Remote: true"
// so the renderer can label them "(library)".
func (c *Client) Variables(ctx context.Context, key string) (*VariablesResponse, error) {
	var vr VariablesResponse
	err := c.get(ctx, "/v1/files/"+key+"/variables/local", nil, &vr)
	if err != nil {
		if e, ok := err.(*Error); !ok || (e.Status != 403 && e.Status != 404) {
			return nil, err
		}
	}

	// Merge published (library) variables on top of local.
	var pub VariablesResponse
	if perr := c.get(ctx, "/v1/files/"+key+"/variables/published", nil, &pub); perr == nil {
		// Merge collections.
		for id, coll := range pub.Meta.VariableCollections {
			if _, exists := vr.Meta.VariableCollections[id]; !exists {
				vr.Meta.VariableCollections[id] = coll
			}
		}
		// Merge variables.
		for id, v := range pub.Meta.Variables {
			if _, exists := vr.Meta.Variables[id]; !exists {
				vr.Meta.Variables[id] = v
			}
		}
	}
	return &vr, nil
}

// PublishedVariables fetches only published (library) variables.
func (c *Client) PublishedVariables(ctx context.Context, key string) (*VariablesResponse, error) {
	var vr VariablesResponse
	if err := c.get(ctx, "/v1/files/"+key+"/variables/published", nil, &vr); err != nil {
		return nil, err
	}
	return &vr, nil
}

// Me validates the token and returns the authenticated user.
func (c *Client) Me(ctx context.Context) (*User, error) {
	var u User
	if err := c.get(ctx, "/v1/me", nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
