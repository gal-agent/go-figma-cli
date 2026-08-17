// Package cache implements a tiny TTL disk cache for CallToolResults,
// keyed by a sha256 of the call's identity parts.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/piratecoder/go-figma-cli/internal/mcp"
)

// Cache stores tool results under Dir with a per-entry TTL.
type Cache struct {
	Dir string
	TTL time.Duration
}

// DefaultDir returns ~/.cache/figma-cli
// (or $FIGMA_CLI_CACHE_DIR when that env var is set).
func DefaultDir() string {
	if env := os.Getenv("FIGMA_CLI_CACHE_DIR"); env != "" {
		return env
	}
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "figma-cli")
}

// New creates a cache with dir (created lazily) and the given TTL.
func New(dir string, ttl time.Duration) *Cache {
	return &Cache{Dir: dir, TTL: ttl}
}

type entry struct {
	Created time.Time           `json:"created"`
	TTL     time.Duration       `json:"ttlSeconds"`
	Result  *mcp.CallToolResult `json:"result"`
}

// Key hashes the identity parts of a call into a cache key.
func Key(parts ...string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Get returns a fresh result or (nil, false) on miss/expiry.
func (c *Cache) Get(key string) (*mcp.CallToolResult, bool) {
	if c == nil || c.Dir == "" {
		return nil, false
	}
	raw, err := os.ReadFile(c.path(key))
	if err != nil {
		return nil, false
	}
	var e entry
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, false
	}
	ttl := e.TTL
	if ttl <= 0 {
		ttl = c.TTL
	}
	if time.Since(e.Created) > ttl {
		_ = os.Remove(c.path(key))
		return nil, false
	}
	return e.Result, true
}

// Put stores a result. Errors are reported but non-fatal by design.
func (c *Cache) Put(key string, res *mcp.CallToolResult) error {
	if c == nil || c.Dir == "" {
		return errors.New("cache: disabled")
	}
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return fmt.Errorf("cache: mkdir: %w", err)
	}
	raw, err := json.Marshal(entry{Created: time.Now(), TTL: c.TTL, Result: res})
	if err != nil {
		return err
	}
	tmp := c.path(key) + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("cache: write: %w", err)
	}
	return os.Rename(tmp, c.path(key))
}

func (c *Cache) path(key string) string {
	return filepath.Join(c.Dir, key+".json")
}

// Clean removes expired entries; returns the number removed.
func (c *Cache) Clean() int {
	if c == nil || c.Dir == "" {
		return 0
	}
	n := 0
	_ = filepath.WalkDir(c.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var e entry
		if json.Unmarshal(raw, &e) != nil || time.Since(e.Created) > c.TTL {
			_ = os.Remove(path)
			n++
		}
		return nil
	})
	return n
}
