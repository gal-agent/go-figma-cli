package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gal-agent/go-figma-cli/internal/auth"
	"github.com/gal-agent/go-figma-cli/internal/mcptest"
)

const metaXML = `<frame id="1:2" name="Dashboard" type="frame">
  <frame id="1:3" name="Header" type="frame"/>
  <frame id="1:4" name="CardList" type="frame"/>
</frame>`

// newTestRoot wires a mock MCP server behind the real cobra root command,
// with config/cache redirected into temp dirs.
func newTestRoot(t *testing.T) (*mcptest.Server, *bytes.Buffer, func(...string) error) {
	t.Helper()
	s := mcptest.New()
	s.MetadataFor["1:2"] = metaXML
	s.DesignFor["1:3"] = "<div>Header</div>"
	s.DesignFor["1:4"] = "<div>Cards</div>"
	s.VarsFor["1:2"] = `{"color/brand/red": "#FF0000"}`

	cfgDir := t.TempDir()
	cacheDir := t.TempDir()
	tok := auth.Token{AccessToken: "test-token", TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour)}
	raw, _ := json.Marshal(tok)
	if err := os.WriteFile(filepath.Join(cfgDir, "auth.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FIGMA_CLI_CONFIG_HOME", cfgDir)
	t.Setenv("FIGMA_CLI_CACHE_DIR", cacheDir)
	t.Cleanup(s.Close)

	buf := &bytes.Buffer{}
	run := func(args ...string) error {
		buf.Reset()
		root := NewRoot()
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs(append([]string{"--url", s.Endpoint()}, args...))
		return root.Execute()
	}
	return s, buf, run
}

func TestDoctorCommand(t *testing.T) {
	_, buf, run := newTestRoot(t)
	if err := run("doctor"); err != nil {
		t.Fatalf("doctor: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"mock-figma 9.9", "get_design_context", "get_metadata", "OK", "valid until"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
}

func TestPagesCommand(t *testing.T) {
	s, buf, run := newTestRoot(t)
	if err := run("pages", "https://www.figma.com/design/AbCdEf123/t"); err != nil {
		t.Fatalf("pages: %v", err)
	}
	if !strings.Contains(buf.String(), "Page 1") {
		t.Fatalf("output: %q", buf.String())
	}
	calls := s.CallsSnapshot()
	if len(calls) != 1 || !strings.HasPrefix(calls[0], "get_metadata:") {
		t.Fatalf("calls = %v", calls)
	}
}

func TestPipelineCommand(t *testing.T) {
	s, buf, run := newTestRoot(t)
	url := "https://www.figma.com/design/AbCdEf123/t?node-id=1-2"
	if err := run("pipeline", url); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"===== [1/2] Header (1:3) =====",
		"<div>Header</div>",
		"===== [2/2] CardList (1:4) =====",
		"===== variables =====",
		"#FF0000",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("pipeline output missing %q:\n%s", want, out)
		}
	}

	// Second run must be served entirely from cache: no new server calls.
	before := len(s.CallsSnapshot())
	if err := run("pipeline", url); err != nil {
		t.Fatalf("pipeline(cached): %v", err)
	}
	if after := len(s.CallsSnapshot()); after != before {
		t.Fatalf("cache miss: calls went %d -> %d", before, after)
	}
}

func TestPipelineFreshFlag(t *testing.T) {
	s, _, run := newTestRoot(t)
	url := "https://www.figma.com/design/AbCdEf123/t?node-id=1-2"
	if err := run("pipeline", url); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	before := len(s.CallsSnapshot())
	if err := run("--fresh", "pipeline", url); err != nil {
		t.Fatalf("pipeline fresh: %v", err)
	}
	if after := len(s.CallsSnapshot()); after <= before {
		t.Fatalf("--fresh should bypass cache (calls %d -> %d)", before, after)
	}
}

func TestCodeCommand(t *testing.T) {
	_, buf, run := newTestRoot(t)
	if err := run("code", "https://www.figma.com/design/AbCdEf123/t?node-id=1-3"); err != nil {
		t.Fatalf("code: %v", err)
	}
	if !strings.Contains(buf.String(), "<div>Header</div>") {
		t.Fatalf("output: %q", buf.String())
	}
}

func TestShotCommandWritesFileNotBase64(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "shot.png")
	_, buf, run := newTestRoot(t)
	if err := run("shot", "--out", out, "AbCdEf123", "1-3"); err != nil {
		t.Fatalf("shot: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil || len(raw) == 0 {
		t.Fatalf("screenshot file missing: %v", err)
	}
	if strings.Contains(buf.String(), "aGVsbG8=") {
		t.Fatal("base64 leaked into stdout")
	}
}

func TestNodeArgumentRequired(t *testing.T) {
	_, _, run := newTestRoot(t)
	err := run("code", "https://www.figma.com/design/AbCdEf123/t")
	if err == nil || !strings.Contains(err.Error(), "node-id") {
		t.Fatalf("expected node-id error, got %v", err)
	}
}

func TestMissingToken(t *testing.T) {
	s := mcptest.New()
	defer s.Close()
	dir := t.TempDir()
	t.Setenv("FIGMA_CLI_CONFIG_HOME", dir)
	t.Setenv("FIGMA_CLI_CACHE_DIR", t.TempDir())
	root := NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--url", s.Endpoint(), "pages", "AbCdEf123"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "figma login") {
		t.Fatalf("expected login hint, got %v", err)
	}
}
