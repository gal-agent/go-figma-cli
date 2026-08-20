package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gal-agent/go-figma-cli/internal/resttest"
)

const fileJSON = `{
  "name": "TestFile",
  "lastModified": "2026-08-20T00:00:00Z",
  "document": {
    "id": "0:0", "name": "Document", "type": "DOCUMENT",
    "children": [
      {"id": "0:1", "name": "Page 1", "type": "CANVAS", "children": [
        {"id": "1:2", "name": "Dashboard", "type": "FRAME", "children": [
          {"id": "1:3", "name": "Header", "type": "FRAME"},
          {"id": "1:4", "name": "CardList", "type": "FRAME"}
        ]}
      ]}
    ]
  }
}`

const nodesJSON = `{
  "name": "TestFile",
  "nodes": {
    "1:2": {"id": "1:2", "name": "Dashboard", "type": "FRAME", "document": {
      "id": "1:2", "name": "Dashboard", "type": "FRAME",
      "absoluteBoundingBox": {"x": 0, "y": 0, "width": 800, "height": 600},
      "children": [
        {"id": "1:3", "name": "Header", "type": "FRAME",
         "absoluteBoundingBox": {"x": 0, "y": 0, "width": 800, "height": 64},
         "fills": [{"type": "SOLID", "color": {"r": 1, "g": 0, "b": 0, "a": 1}}]},
        {"id": "1:4", "name": "CardList", "type": "FRAME",
         "absoluteBoundingBox": {"x": 0, "y": 64, "width": 800, "height": 536}}
      ]
    }},
    "1:3": {"id": "1:3", "name": "Header", "type": "FRAME", "document": {
      "id": "1:3", "name": "Header", "type": "FRAME",
      "children": [{"id": "1:30", "name": "Logo", "type": "TEXT",
        "characters": "Hello", "style": {"fontFamily": "Inter", "fontWeight": 700, "fontSize": 16},
        "boundVariables": {"fills": [{"type": "VARIABLE_ALIAS", "id": "Var:1"}]}
      }]
    }}
  }
}`

const varsJSON = `{
  "status": 200,
  "meta": {
    "variables": {
      "Var:1": {"id": "Var:1", "name": "brand/red", "key": "k1", "variableCollectionId": "C:1",
        "resolvedType": "COLOR", "valuesByMode": {"m1": {"r": 1, "g": 0, "b": 0, "a": 1}}}
    },
    "variableCollections": {
      "C:1": {"id": "C:1", "name": "Core", "key": "kc", "defaultModeId": "m1",
        "modes": [{"modeId": "m1", "name": "Light"}]}
    }
  }
}`

// newTestRoot wires a mock REST server behind the real cobra root command,
// with config/cache redirected into temp dirs.
func newTestRoot(t *testing.T) (*resttest.Server, *bytes.Buffer, func(...string) error) {
	t.Helper()
	s := resttest.New()
	s.FileFor["AbCdEf123"] = fileJSON
	s.NodesFor["AbCdEf123"] = nodesJSON
	s.VarsFor["AbCdEf123"] = varsJSON
	s.ImageURLFor["AbCdEf123"] = s.URL + "/img/x.png"

	cfgDir := t.TempDir()
	cacheDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"pat":"test-pat"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FIGMA_CLI_CONFIG_HOME", cfgDir)
	t.Setenv("FIGMA_CLI_CACHE_DIR", cacheDir)
	t.Setenv("FIGMA_REST_BASE_URL", s.URL)
	t.Cleanup(s.Close)

	buf := &bytes.Buffer{}
	run := func(args ...string) error {
		buf.Reset()
		root := NewRoot()
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs(args)
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
	for _, want := range []string{"Figma REST API", "PAT valid", "tester"} {
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
	if len(calls) != 1 || !strings.HasPrefix(calls[0], "file:AbCdEf123") {
		t.Fatalf("calls = %v", calls)
	}
}

func TestTreeCommand(t *testing.T) {
	_, buf, run := newTestRoot(t)
	if err := run("tree", "https://www.figma.com/design/AbCdEf123/t?node-id=1-2"); err != nil {
		t.Fatalf("tree: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Dashboard", "Header", "CardList"} {
		if !strings.Contains(out, want) {
			t.Fatalf("tree output missing %q:\n%s", want, out)
		}
	}
}

func TestCodeCommand(t *testing.T) {
	_, buf, run := newTestRoot(t)
	if err := run("code", "https://www.figma.com/design/AbCdEf123/t?node-id=1-3"); err != nil {
		t.Fatalf("code: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Header", "Logo", "Hello", "Inter"} {
		if !strings.Contains(out, want) {
			t.Fatalf("code output missing %q:\n%s", want, out)
		}
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
		"===== [2/2] CardList (1:4) =====",
		"===== variables =====",
		"brand/red",
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
	if strings.Contains(buf.String(), "cG5nYnl0ZXM=") {
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
	s := resttest.New()
	defer s.Close()
	t.Setenv("FIGMA_CLI_CONFIG_HOME", t.TempDir())
	t.Setenv("FIGMA_CLI_CACHE_DIR", t.TempDir())
	t.Setenv("FIGMA_REST_BASE_URL", s.URL)
	root := NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"pages", "AbCdEf123"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "login --token") {
		t.Fatalf("expected login hint, got %v", err)
	}
}

func TestUnauthorizedGivesPATHint(t *testing.T) {
	s := resttest.New()
	s.FileFor["AbCdEf123"] = fileJSON
	defer s.Close()
	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"pat":"wrong"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FIGMA_CLI_CONFIG_HOME", cfgDir)
	t.Setenv("FIGMA_CLI_CACHE_DIR", t.TempDir())
	t.Setenv("FIGMA_REST_BASE_URL", s.URL)
	root := NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"pages", "AbCdEf123"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "figma.com/settings") {
		t.Fatalf("expected PAT remediation, got %v", err)
	}
}

func TestLoginSavesToken(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv("FIGMA_CLI_CONFIG_HOME", cfgDir)
	root := NewRoot()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"login", "--token", "figd_test123"})
	if err := root.Execute(); err != nil {
		t.Fatalf("login: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(cfgDir, "config.json"))
	if err != nil || !strings.Contains(string(raw), "figd_test123") {
		t.Fatalf("config not saved: %v %s", err, raw)
	}
}
