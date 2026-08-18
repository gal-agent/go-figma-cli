package output

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gal-agent/go-figma-cli/internal/mcp"
)

func TestWriteResultText(t *testing.T) {
	res := &mcp.CallToolResult{Content: []mcp.Content{
		{Type: "text", Text: "line1\n"},
		{Type: "text", Text: "line2"},
	}}
	var buf bytes.Buffer
	if _, err := WriteResult(res, Options{Out: &buf}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if buf.String() != "line1\nline2\n" {
		t.Fatalf("output = %q", buf.String())
	}
}

func TestWriteResultImageToFile(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("pngbytes"))
	res := &mcp.CallToolResult{Content: []mcp.Content{
		{Type: "image", MimeType: "image/png", Data: b64},
	}}
	dir := t.TempDir()
	var buf bytes.Buffer
	files, err := WriteResult(res, Options{Out: &buf, ImageDir: dir, ImagePrefix: "shot"})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %v", files)
	}
	want := filepath.Join(dir, "shot-0.png")
	if files[0] != want {
		t.Fatalf("path = %q, want %q", files[0], want)
	}
	raw, err := os.ReadFile(want)
	if err != nil || string(raw) != "pngbytes" {
		t.Fatalf("file content = %q err=%v", raw, err)
	}
	if strings.Contains(buf.String(), b64) {
		t.Fatal("base64 leaked to stdout")
	}
	if !strings.Contains(buf.String(), "[image saved:") {
		t.Fatalf("missing saved notice: %q", buf.String())
	}
}

func TestWriteResultResource(t *testing.T) {
	res := &mcp.CallToolResult{Content: []mcp.Content{
		{Type: "resource", Resource: &mcp.ResourceContent{URI: "file://x.svg", Text: "<svg/>"}},
	}}
	var buf bytes.Buffer
	if _, err := WriteResult(res, Options{Out: &buf}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(buf.String(), "file://x.svg") || !strings.Contains(buf.String(), "<svg/>") {
		t.Fatalf("output = %q", buf.String())
	}
}
