// Package output renders CallToolResults for terminal/agent consumption:
// text blocks are printed, image blocks are decoded to files (base64 never
// reaches stdout), resource blocks print their URI.
package output

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/piratecoder/go-figma-cli/internal/mcp"
)

// Options controls rendering.
type Options struct {
	// ImageDir is where image blocks are written (default ".").
	ImageDir string
	// ImagePrefix names saved files (default "figma-image").
	ImagePrefix string
	// Out is the destination for text output (default os.Stdout).
	Out io.Writer
}

// WriteResult renders res and returns the paths of files written.
func WriteResult(res *mcp.CallToolResult, opts Options) ([]string, error) {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.ImageDir == "" {
		opts.ImageDir = "."
	}
	if opts.ImagePrefix == "" {
		opts.ImagePrefix = "figma-image"
	}
	var files []string
	imgN := 0
	for _, c := range res.Content {
		switch c.Type {
		case "text":
			fmt.Fprintln(opts.Out, strings.TrimRight(c.Text, "\n"))
		case "image":
			p, err := writeImage(opts, imgN, c.Data, c.MimeType)
			if err != nil {
				return files, err
			}
			files = append(files, p)
			fmt.Fprintf(opts.Out, "[image saved: %s]\n", p)
			imgN++
		case "resource":
			if c.Resource != nil {
				if c.Resource.URI != "" {
					fmt.Fprintf(opts.Out, "[resource: %s]\n", c.Resource.URI)
				}
				if c.Resource.Text != "" {
					fmt.Fprintln(opts.Out, strings.TrimRight(c.Resource.Text, "\n"))
				}
			}
		default:
			fmt.Fprintf(opts.Out, "[unhandled content type: %s]\n", c.Type)
		}
	}
	if len(res.Content) == 0 && len(res.StructuredContent) > 0 {
		fmt.Fprintln(opts.Out, string(res.StructuredContent))
	}
	return files, nil
}

func writeImage(opts Options, n int, data, mimeType string) (string, error) {
	if data == "" {
		return "", fmt.Errorf("image block %d has no data", n)
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("decode image %d: %w", n, err)
	}
	name := fmt.Sprintf("%s-%d.%s", opts.ImagePrefix, n, extFor(mimeType))
	if err := os.MkdirAll(opts.ImageDir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(opts.ImageDir, name)
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		return "", err
	}
	return p, nil
}

func extFor(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	case "image/svg+xml":
		return "svg"
	default:
		return "png"
	}
}
