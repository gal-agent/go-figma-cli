// Package cli wires the cobra commands onto the internal packages.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gal-agent/go-figma-cli/internal/auth"
	"github.com/gal-agent/go-figma-cli/internal/cache"
	"github.com/gal-agent/go-figma-cli/internal/figmaurl"
	"github.com/gal-agent/go-figma-cli/internal/mcp"
	"github.com/gal-agent/go-figma-cli/internal/output"
	"github.com/gal-agent/go-figma-cli/internal/tools"
)

const (
	defaultRemoteURL  = "https://mcp.figma.com/mcp"
	defaultDesktopURL = "http://127.0.0.1:3845/mcp"
)

// version is set at build time via -ldflags.
var version = "0.1.0"

// verbose mirrors the global -v flag (progress goes to stderr).
var verbose bool

// App carries global options and shared plumbing.
type App struct {
	desktop  bool
	baseURL  string
	ttl      time.Duration
	noCache  bool
	fresh    bool
	raw      bool
	imageDir string

	client   *mcp.Client
	resolver *tools.Resolver
}

// NewRoot builds the command tree.
func NewRoot() *cobra.Command {
	app := &App{}
	root := &cobra.Command{
		Use:   "figma",
		Short: "Figma official MCP server, wrapped for agents (context-friendly output)",
		Long: `Read Figma designs through the official MCP server
(https://mcp.figma.com/mcp or a local desktop server) from the command line.

Designed to be driven by AI coding agents: zero resident tool definitions,
intermediate drill-down steps stay out of the conversation, disk-cached
results, and image payloads are written to files instead of stdout.

ARGUMENT FORMS (accepted by every read command unless noted):
  figma code "https://www.figma.com/design/<fileKey>/<name>?node-id=12-34"
      Paste a link copied in Figma (right-click frame -> Copy link to
      selection) as-is; design/file/proto URLs all work, quoted because
      of the shell-special characters.
  figma code <fileKey> 12:34
      Two-arg form. Node ids "12-34" and "12:34" are equivalent.
  figma pages <fileKey>
      pages also accepts a bare file key (no node id needed).

Typical workflow (drill down instead of converting whole pages):
  figma pages <file>                  # 1. what pages exist
  figma tree  <frame-url>             # 2. frame structure -> pick child frame ids
  figma code  <frame-url>             # 3. code per child frame (small = accurate)
  figma pipeline <frame-url>          #    ...or steps 2+3 for all children at once
  figma vars  <frame-url>             # 4. design tokens used by the frame
  figma shot  <frame-url> -o ref.png  # 5. visual reference for self-check

Reads are disk-cached for --ttl; use --fresh only after the designer
updated the file. Auth: ` + "`figma login`" + ` once for remote mode, or
` + "`--desktop`" + ` against the Figma desktop app (Dev Mode MCP enabled).
` + "`figma doctor`" + ` verifies the setup. Non-zero exit means failure;
error messages name the remediation.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	pf := root.PersistentFlags()
	pf.BoolVar(&app.desktop, "desktop", false, "use the local Figma desktop MCP server (127.0.0.1:3845, no OAuth; needs Dev Mode MCP enabled)")
	pf.StringVar(&app.baseURL, "url", "", fmt.Sprintf("MCP endpoint override (default %s, or %s with --desktop)", defaultRemoteURL, defaultDesktopURL))
	pf.DurationVar(&app.ttl, "ttl", 10*time.Minute, "cache TTL")
	pf.BoolVar(&app.noCache, "no-cache", false, "disable the disk cache")
	pf.BoolVar(&app.fresh, "fresh", false, "bypass cache for reads (still writes)")
	pf.BoolVar(&app.raw, "raw", false, "print the raw JSON-RPC tool result instead of rendered text")
	pf.StringVar(&app.imageDir, "image-dir", ".", "directory for image payloads decoded from tool results")
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

// ---- shared plumbing ----

func (a *App) endpoint() string {
	if a.baseURL != "" {
		return a.baseURL
	}
	if a.desktop {
		return defaultDesktopURL
	}
	return defaultRemoteURL
}

func (a *App) mode() string {
	if a.desktop {
		return "desktop"
	}
	return "remote"
}

var refreshHTTP = &http.Client{Timeout: 60 * time.Second}

func (a *App) token() (string, error) {
	if a.desktop {
		return "", nil
	}
	store := &auth.Store{Path: auth.DefaultStorePath()}
	tok, err := store.Load()
	if err != nil {
		return "", fmt.Errorf("read stored token: %w (run `figma login`)", err)
	}
	if tok == nil {
		return "", errors.New("no stored token for the remote server; run `figma login` (or use --desktop)")
	}
	if tok.Valid() {
		return tok.AccessToken, nil
	}
	if ep, _ := store.LoadEndpoint(); ep != nil {
		refreshed, rerr := auth.Refresh(context.Background(), refreshHTTP, ep, store, tok)
		if rerr == nil {
			return refreshed.AccessToken, nil
		}
	}
	return "", errors.New("token expired and refresh failed; run `figma login`")
}

func (a *App) connect(ctx context.Context) (*mcp.Client, *tools.Resolver, error) {
	if a.client == nil {
		tok, err := a.token()
		if err != nil {
			return nil, nil, err
		}
		client := mcp.NewClient(a.endpoint(), tok)
		if _, err := client.Initialize(ctx); err != nil {
			if errors.Is(err, mcp.ErrUnauthorized) {
				return nil, nil, fmt.Errorf("server rejected the token; run `figma login`: %w", err)
			}
			if a.desktop {
				return nil, nil, fmt.Errorf("cannot reach the desktop MCP server: %w\nIs the Figma desktop app running with Dev Mode + 'Enable MCP server' turned on?", err)
			}
			return nil, nil, err
		}
		list, err := client.ListTools(ctx)
		if err != nil {
			return nil, nil, err
		}
		a.client = client
		a.resolver = tools.NewResolver(list)
	}
	return a.client, a.resolver, nil
}

// callTool resolves a capability, consults the cache, and invokes the tool.
// ref participates in the cache key (may be nil for file-less calls).
func (a *App) callTool(ctx context.Context, capability string, args map[string]any, ref *figmaurl.Ref) (*mcp.CallToolResult, error) {
	client, resolver, err := a.connect(ctx)
	if err != nil {
		return nil, err
	}
	name, err := resolver.Resolve(capability)
	if err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}

	var c *cache.Cache
	if !a.noCache {
		c = cache.New(cache.DefaultDir(), a.ttl)
	}
	key := cacheKey(a.endpoint(), ref, capability, args)
	if c != nil && !a.fresh {
		if res, ok := c.Get(key); ok {
			if verbose {
				fmt.Fprintf(os.Stderr, "[cache] hit %s\n", shortKey(key))
			}
			return res, nil
		}
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "[mcp] %s %s\n", name, mustJSON(args))
	}
	res, err := client.CallTool(ctx, name, args)
	if err != nil {
		return nil, err
	}
	if c != nil {
		_ = c.Put(key, res)
	}
	return res, nil
}

func cacheKey(endpoint string, ref *figmaurl.Ref, capability string, args map[string]any) string {
	parts := []string{endpoint, capability}
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

// printResult renders a result honoring --raw; tool-level errors exit non-zero.
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
