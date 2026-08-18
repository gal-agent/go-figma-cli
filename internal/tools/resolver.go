// Package tools maps stable capability names onto whatever tool names the
// live server exposes (Figma renamed tools at least once: get_code ->
// get_design_context, get_image -> get_screenshot).
package tools

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gal-agent/go-figma-cli/internal/mcp"
)

// Capability names used across the CLI. Never call raw tool names directly.
const (
	CapMetadata   = "get_metadata"
	CapDesignCtx  = "get_design_context"
	CapVariables  = "get_variable_defs"
	CapScreenshot = "get_screenshot"
)

// aliases lists acceptable tool names per capability, most current first.
var aliases = map[string][]string{
	CapMetadata:   {"get_metadata"},
	CapDesignCtx:  {"get_design_context", "get_code"},
	CapVariables:  {"get_variable_defs"},
	CapScreenshot: {"get_screenshot", "get_image"},
}

// Resolver picks concrete tool names from a live tools/list snapshot.
type Resolver struct {
	present map[string]bool
	all     []mcp.Tool
}

// NewResolver snapshots the tool list.
func NewResolver(list []mcp.Tool) *Resolver {
	r := &Resolver{present: map[string]bool{}, all: list}
	for _, t := range list {
		r.present[t.Name] = true
	}
	return r
}

// Resolve returns the concrete tool name for a capability.
func (r *Resolver) Resolve(capability string) (string, error) {
	for _, name := range aliases[capability] {
		if r.present[name] {
			return name, nil
		}
	}
	return "", fmt.Errorf("server exposes none of %s (looked for %s); available: %s",
		capability, strings.Join(aliases[capability], " / "), strings.Join(r.Names(), ", "))
}

// Has reports whether a capability is available.
func (r *Resolver) Has(capability string) bool {
	_, err := r.Resolve(capability)
	return err == nil
}

// Names returns all tool names, sorted.
func (r *Resolver) Names() []string {
	names := make([]string, 0, len(r.all))
	for _, t := range r.all {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names
}

// Tools returns the snapshot.
func (r *Resolver) Tools() []mcp.Tool { return r.all }

// DriftReport lists capabilities that cannot be resolved on this server.
func (r *Resolver) DriftReport() []string {
	var missing []string
	for cap := range aliases {
		if !r.Has(cap) {
			missing = append(missing, cap)
		}
	}
	sort.Strings(missing)
	return missing
}
