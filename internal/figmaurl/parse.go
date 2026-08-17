package figmaurl

import (
	"fmt"
	"net/url"
	"strings"
)

// Ref identifies a Figma file (and optionally a node inside it).
// NodeID is stored in API form with colons ("123:456"), never dashes.
type Ref struct {
	FileKey string
	NodeID  string
}

// Parse accepts:
//   - https://www.figma.com/design/<fileKey>/<title>?node-id=12-34
//   - https://www.figma.com/file/<fileKey>/<title>?node-id=12%3A34
//   - https://www.figma.com/proto/... and /board/... variants
//   - a bare file key ("AbCdEf123")
//   - "fileKey nodeId" positional style handled by callers, not here
func Parse(raw string) (*Ref, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty Figma URL")
	}
	if !strings.Contains(raw, "/") && !strings.Contains(raw, "?") {
		// Bare file key.
		if !isFileKey(raw) {
			return nil, fmt.Errorf("%q does not look like a Figma file key", raw)
		}
		return &Ref{FileKey: raw}, nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	// Expect [design|file|proto|board, <fileKey>, <title>...]
	if len(segments) < 2 {
		return nil, fmt.Errorf("cannot find file key in %q", raw)
	}
	fileKey := segments[1]
	if !isFileKey(fileKey) {
		// Maybe the user pasted a path without the design/ segment.
		if isFileKey(segments[0]) {
			fileKey = segments[0]
		} else {
			return nil, fmt.Errorf("cannot find file key in %q", raw)
		}
	}
	ref := &Ref{FileKey: fileKey}
	if node := queryGet(u.RawQuery, "node-id"); node != "" {
		ref.NodeID = NormalizeNodeID(node)
	}
	return ref, nil
}

// queryGet extracts a query parameter without url.ParseQuery, which drops
// pairs containing semicolons (node ids like "1:2;3:4" use them).
func queryGet(rawQuery, key string) string {
	for _, pair := range strings.Split(rawQuery, "&") {
		k, v, ok := strings.Cut(pair, "=")
		if !ok || k != key {
			continue
		}
		if unescaped, err := url.QueryUnescape(v); err == nil {
			return unescaped
		}
		return v
	}
	return ""
}

// NormalizeNodeID converts the URL dash form ("12-34") into the API colon
// form ("12:34"). Values already containing colons pass through; URL-encoded
// colons (%3A) must be decoded by net/url already.
func NormalizeNodeID(id string) string {
	id = strings.TrimSpace(id)
	if strings.Contains(id, ":") {
		return id
	}
	return strings.ReplaceAll(id, "-", ":")
}

// Dashed returns the URL query form of the node id.
func (r *Ref) Dashed() string {
	return strings.ReplaceAll(r.NodeID, ":", "-")
}

func isFileKey(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// IsFileKey reports whether s looks like a bare Figma file key.
func IsFileKey(s string) bool { return isFileKey(s) }
