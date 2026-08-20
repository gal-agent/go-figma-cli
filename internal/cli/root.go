// Package cli wires the cobra commands onto the internal packages.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gal-agent/go-figma-cli/internal/cache"
	"github.com/gal-agent/go-figma-cli/internal/config"
	"github.com/gal-agent/go-figma-cli/internal/figmaurl"
	"github.com/gal-agent/go-figma-cli/internal/mcp"
	"github.com/gal-agent/go-figma-cli/internal/output"
	"github.com/gal-agent/go-figma-cli/internal/tools"
)

// version is set at build time via -ldflags.
var version = "0.3.0"

// verbose mirrors the global -v flag (progress goes to stderr).
var verbose bool

// App carries global options and shared plumbing.
type App struct {
	ttl      time.Duration
	noCache  bool
	fresh    bool
	raw      bool
	imageDir string
}

// NewRoot builds the command tree.
func NewRoot() *cobra.Command {
	app := &App{}
	root := &cobra.Command{
		Use:   "go-figma-cli",
		Short: "Read Figma designs via the Figma REST API with a personal access token",
		Long: `Read Figma designs from the command line through the Figma REST API,
authenticated with a personal access token (PAT).

Designed to be driven by AI coding agents: intermediate drill-down steps
stay out of the conversation, disk-cached results, and image payloads are
written to files instead of stdout.

SETUP (once):
  1. Create a PAT at https://www.figma.com/settings
     (Security -> Personal access tokens, scope: File content - read-only).
  2. go-figma-cli login --token <PAT>
  3. go-figma-cli doctor

ARGUMENT FORMS (accepted by every read command unless noted):
  go-figma-cli code "https://www.figma.com/design/<fileKey>/<name>?node-id=12-34"
  go-figma-cli code <fileKey> 12:34
  go-figma-cli pages <fileKey>

Typical workflow (drill down instead of converting whole pages):
  go-figma-cli pages <file>                  # 1. what pages exist
  go-figma-cli tree  <frame-url>             # 2. frame structure -> pick child frame ids
  go-figma-cli code  <frame-url>             # 3. code per child frame (small = accurate)
  go-figma-cli pipeline <frame-url>          #    ...or steps 2+3 for all children at once
  go-figma-cli vars  <frame-url>             # 4. design tokens used by the frame
  go-figma-cli shot  <frame-url> -o ref.png  # 5. visual reference for self-check

Reads are disk-cached for --ttl; use --fresh only after the designer
updated the file. Non-zero exit means failure; error messages name the
remediation.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	pf := root.PersistentFlags()
	pf.DurationVar(&app.ttl, "ttl", 10*time.Minute, "cache TTL")
	pf.BoolVar(&app.noCache, "no-cache", false, "disable the disk cache")
	pf.BoolVar(&app.fresh, "fresh", false, "bypass cache for reads (still writes)")
	pf.BoolVar(&app.raw, "raw", false, "print the raw tool result JSON instead of rendered text")
	pf.StringVar(&app.imageDir, "image-dir", ".", "directory for image payloads")
	pf.BoolVarP(&verbose, "verbose", "v", false, "log drill-down progress to stderr")

	root.AddCommand(
		newLoginCmd(app),
		newDoctorCmd(app),
		newPagesCmd(app),
		newTreeCmd(app),
		newCodeCmd(app),
		newVarsCmd(app),
		newShotCmd(app),
		newPipelineCmd(app),
	)
	return root
}

// Execute runs the CLI and sets the process exit code.
func Execute() {
	if err := NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// callTool consults the cache, calls the REST API and renders an
// mcp.CallToolResult-shaped value (the CLI's internal result type).
func (a *App) callTool(ctx context.Context, capability string, args map[string]any, ref *figmaurl.Ref) (*mcp.CallToolResult, error) {
	var c *cache.Cache
	if !a.noCache {
		c = cache.New(cache.DefaultDir(), a.ttl)
	}
	key := cacheKey(capability, ref, args)
	if c != nil && !a.fresh {
		if res, ok := c.Get(key); ok {
			if verbose {
				fmt.Fprintf(os.Stderr, "[cache] hit %s\n", shortKey(key))
			}
			return res, nil
		}
	}
	res, err := a.callToolREST(ctx, capability, args, ref)
	if err != nil {
		return nil, err
	}
	if c != nil {
		_ = c.Put(key, res)
	}
	return res, nil
}

func cacheKey(capability string, ref *figmaurl.Ref, args map[string]any) string {
	parts := []string{"rest", capability}
	if ref != nil {
		parts = append(parts, ref.FileKey, ref.NodeID)
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, _ := json.Marshal(args[k])
		parts = append(parts, k+"="+string(v))
	}
	return cache.Key(parts...)
}

// outputOptions builds render options for a command invocation.
func (a *App) outputOptions(cmd *cobra.Command) output.Options {
	return output.Options{
		Out:         cmd.OutOrStdout(),
		ImageDir:    a.imageDir,
		ImagePrefix: "figma-image",
	}
}

// printResult renders a result honoring --raw.
func (a *App) printResult(cmd *cobra.Command, res *mcp.CallToolResult) error {
	if res.IsError {
		return fmt.Errorf("tool reported an error:\n%s", res.TextParts())
	}
	if a.raw {
		raw, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(raw))
		return nil
	}
	_, err := output.WriteResult(res, a.outputOptions(cmd))
	return err
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "?"
	}
	return string(raw)
}

func shortKey(k string) string {
	if len(k) > 12 {
		return k[:12]
	}
	return k
}

// parseSets turns ["k=v", ...] into a typed argument map.
func parseSets(values []string) (map[string]any, error) {
	out := map[string]any{}
	if len(values) == 0 {
		return out, nil
	}
	for _, v := range values {
		k, val, ok := strings.Cut(v, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("--set expects key=value, got %q", v)
		}
		switch {
		case val == "true":
			out[k] = true
		case val == "false":
			out[k] = false
		default:
			if n, err := strconv.ParseFloat(val, 64); err == nil {
				out[k] = n
			} else {
				out[k] = val
			}
		}
	}
	return out, nil
}

// ensure tools import is used (capability constants live there).
var _ = tools.CapMetadata
var _ = config.DefaultPath
