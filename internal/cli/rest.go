package cli

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/gal-agent/go-figma-cli/internal/cache"
	"github.com/gal-agent/go-figma-cli/internal/config"
	"github.com/gal-agent/go-figma-cli/internal/figmaurl"
	"github.com/gal-agent/go-figma-cli/internal/mcp"
	"github.com/gal-agent/go-figma-cli/internal/restapi"
	"github.com/gal-agent/go-figma-cli/internal/tools"
)

func (a *App) restClient() (*restapi.Client, error) {
	tok, err := config.Token()
	if err != nil {
		return nil, err
	}
	return restapi.New("", tok, nil), nil
}

// callToolREST maps a capability + args to a REST API call and shapes the
// result as a CallToolResult, so the rest of the CLI (printResult, pipeline,
// cache) works unchanged.
func (a *App) callToolREST(ctx context.Context, capability string, args map[string]any, ref *figmaurl.Ref) (*mcp.CallToolResult, error) {
	c, err := a.restClient()
	if err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}

	switch capability {
	case tools.CapMetadata:
		if ref.NodeID == "" {
			// pages: file-level metadata
			f, err := c.File(ctx, ref.FileKey)
			if err != nil {
				return nil, patHint(err)
			}
			return textResult(restapi.RenderPagesXML(ref.FileKey, f)), nil
		}
		// tree: node subtree (shallow depth to match the metadata shape)
		doc, err := c.Nodes(ctx, ref.FileKey, ref.NodeID, 3)
		if err != nil {
			return nil, patHint(err)
		}
		return textResult(restapi.RenderTreeXML(&doc.Document)), nil

	case tools.CapDesignCtx:
		doc, err := c.Nodes(ctx, ref.FileKey, ref.NodeID, 0)
		if err != nil {
			return nil, patHint(err)
		}
		return textResult(restapi.RenderContext(&doc.Document, 6)), nil

	case tools.CapVariables:
		vr, err := c.Variables(ctx, ref.FileKey)
		if err != nil {
			return nil, patHint(err)
		}
		return textResult(restapi.RenderVariables(vr)), nil

	case tools.CapScreenshot:
		format := "png"
		scale := 2.0
		if v, ok := args["format"].(string); ok && v != "" {
			format = v
		}
		if v, ok := args["scale"].(float64); ok && v > 0 {
			scale = v
		}
		imgURL, err := c.ImageURL(ctx, ref.FileKey, ref.NodeID, format, scale)
		if err != nil {
			return nil, patHint(err)
		}
		data, mime, err := c.Fetch(ctx, imgURL)
		if err != nil {
			return nil, err
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{{
				Type:     "image",
				Data:     base64.StdEncoding.EncodeToString(data),
				MimeType: mime,
			}},
		}, nil

	default:
		return nil, fmt.Errorf("capability %q is not supported", capability)
	}
}

// patHint appends PAT remediation to 401/403 REST errors.
func patHint(err error) error {
	if e, ok := err.(*restapi.Error); ok && (e.Status == 401 || e.Status == 403) {
		return fmt.Errorf("%w\n%s\n%s", err,
			"Your Figma token is missing, expired or revoked (or lacks access to this file).",
			patHelp)
	}
	return err
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{{Type: "text", Text: text}},
	}
}

// restDoctor validates the token via /v1/me.
func (a *App) restDoctor(ctx context.Context, out io.Writer) error {
	c, err := a.restClient()
	if err != nil {
		return fmt.Errorf("%w\n\n%s", err, patHelp)
	}
	user, err := c.Me(ctx)
	if err != nil {
		return patHint(err)
	}
	fmt.Fprintln(out, "server       : Figma REST API (api.figma.com)")
	fmt.Fprintf(out, "auth         : PAT valid (user=%s)\n", firstNonEmpty(user.Handle, user.Email, user.ID))
	fmt.Fprintf(out, "token config : %s\n", config.DefaultPath())
	fmt.Fprintln(out, "capabilities : pages / tree / code / vars / shot")
	fmt.Fprintf(out, "cache        : %s", cacheDir())
	return nil
}

func cacheDir() string {
	return cache.DefaultDir()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
