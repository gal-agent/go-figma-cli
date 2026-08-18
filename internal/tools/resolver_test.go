package tools

import (
	"strings"
	"testing"

	"github.com/gal-agent/go-figma-cli/internal/mcp"
)

func list(names ...string) []mcp.Tool {
	out := make([]mcp.Tool, len(names))
	for i, n := range names {
		out[i] = mcp.Tool{Name: n}
	}
	return out
}

func TestResolveCurrentNames(t *testing.T) {
	r := NewResolver(list("get_metadata", "get_design_context", "get_variable_defs", "get_screenshot"))
	for _, cap := range []string{CapMetadata, CapDesignCtx, CapVariables, CapScreenshot} {
		if _, err := r.Resolve(cap); err != nil {
			t.Fatalf("%s: %v", cap, err)
		}
	}
	if len(r.DriftReport()) != 0 {
		t.Fatalf("unexpected drift: %v", r.DriftReport())
	}
}

func TestResolveLegacyNames(t *testing.T) {
	// Server predates the 2025-09 rename.
	r := NewResolver(list("get_code", "get_image", "get_variable_defs"))
	if name, _ := r.Resolve(CapDesignCtx); name != "get_code" {
		t.Fatalf("design ctx -> %q, want get_code", name)
	}
	if name, _ := r.Resolve(CapScreenshot); name != "get_image" {
		t.Fatalf("screenshot -> %q, want get_image", name)
	}
	drift := r.DriftReport()
	if len(drift) != 1 || drift[0] != CapMetadata {
		t.Fatalf("drift = %v, want [get_metadata]", drift)
	}
}

func TestResolveMissingListsAvailable(t *testing.T) {
	r := NewResolver(list("unrelated_tool"))
	_, err := r.Resolve(CapDesignCtx)
	if err == nil || !strings.Contains(err.Error(), "unrelated_tool") {
		t.Fatalf("error should list available tools, got: %v", err)
	}
}
