package cli

import (
	"fmt"
	"io"
	"strings"

	"os"

	"github.com/spf13/cobra"

	"github.com/piratecoder/go-figma-cli/internal/figmaurl"
	"github.com/piratecoder/go-figma-cli/internal/mcp"
	"github.com/piratecoder/go-figma-cli/internal/output"
	"github.com/piratecoder/go-figma-cli/internal/tools"
)

// newResolver is a tiny indirection so login/doctor can build resolvers too.
func newResolver(list []mcp.Tool) *tools.Resolver { return tools.NewResolver(list) }

// refFromArgs accepts "<url>" or "<fileKey> <nodeId>" (2 positional args).
func refFromArgs(args []string, requireNode bool) (*figmaurl.Ref, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing Figma URL argument")
	}
	if len(args) >= 2 {
		ref, err := figmaurl.Parse(args[0])
		if err != nil {
			return nil, err
		}
		if !figmaurl.IsFileKey(args[0]) {
			return nil, fmt.Errorf("first argument %q is not a file key", args[0])
		}
		ref.FileKey = args[0]
		ref.NodeID = figmaurl.NormalizeNodeID(args[1])
		return ref, nil
	}
	ref, err := figmaurl.Parse(args[0])
	if err != nil {
		return nil, err
	}
	if requireNode && ref.NodeID == "" {
		return nil, fmt.Errorf("URL has no node-id (right-click the frame in Figma and \"Copy link to selection\")")
	}
	return ref, nil
}

func newPagesCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "pages <url-or-fileKey>",
		Short: "List top-level pages of a file (get_metadata, no nodeId)",
		Example: `  figma pages https://www.figma.com/design/ABC123/My-App
  figma pages ABC123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := refFromArgs(args, false)
			if err != nil {
				return err
			}
			res, err := app.callTool(cmd.Context(), tools.CapMetadata, map[string]any{
				"fileKey": ref.FileKey,
			}, ref)
			if err != nil {
				return err
			}
			return app.printResult(cmd, res)
		},
	}
}

func newTreeCmd(app *App) *cobra.Command {
	var sets []string
	cmd := &cobra.Command{
		Use:   "tree <url-or-(fileKey nodeId)>",
		Short: "Sparse node tree for a node (get_metadata with nodeId)",
		Example: `  figma tree "https://www.figma.com/design/ABC123/My-App?node-id=12-34"
  figma tree ABC123 12:34`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := refFromArgs(args, true)
			if err != nil {
				return err
			}
			toolArgs, err := parseSets(sets)
			if err != nil {
				return err
			}
			toolArgs["nodeId"] = ref.NodeID
			res, err := app.callTool(cmd.Context(), tools.CapMetadata, toolArgs, ref)
			if err != nil {
				return err
			}
			return app.printResult(cmd, res)
		},
	}
	cmd.Flags().StringArrayVar(&sets, "set", nil, "extra tool args k=v")
	return cmd
}

func newCodeCmd(app *App) *cobra.Command {
	var sets []string
	cmd := &cobra.Command{
		Use:   "code <url-or-(fileKey nodeId)>",
		Short: "Design context / generated code for a node (get_design_context)",
		Long: `Calls get_design_context (aka get_code) for one node.

Default output is React + Tailwind; the server accepts extra params via --set
(known ones: clientFrameworks=react|swiftui|..., version, options).
Treat the output as a design representation and translate it to the
project's actual stack and components.
Screenshots embedded in the response are saved to --image-dir, base64 is
never printed.`,
		Example: `  figma code "https://www.figma.com/design/ABC123/My-App?node-id=12-34"
  figma code ABC123 12:34 --set clientFrameworks=vue
  figma code ABC123 12:34 --fresh`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := refFromArgs(args, true)
			if err != nil {
				return err
			}
			toolArgs, err := parseSets(sets)
			if err != nil {
				return err
			}
			toolArgs["nodeId"] = ref.NodeID
			res, err := app.callTool(cmd.Context(), tools.CapDesignCtx, toolArgs, ref)
			if err != nil {
				return err
			}
			return app.printResult(cmd, res)
		},
	}
	cmd.Flags().StringArrayVar(&sets, "set", nil, "extra tool args k=v (e.g. --set clientFrameworks=vue)")
	return cmd
}

func newVarsCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "vars <url-or-(fileKey nodeId)>",
		Short: "Design tokens used by a node (get_variable_defs)",
		Example: `  figma vars "https://www.figma.com/design/ABC123/My-App?node-id=12-34"
  figma vars ABC123 12:34`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := refFromArgs(args, true)
			if err != nil {
				return err
			}
			res, err := app.callTool(cmd.Context(), tools.CapVariables, map[string]any{
				"nodeId": ref.NodeID,
			}, ref)
			if err != nil {
				return err
			}
			return app.printResult(cmd, res)
		},
	}
}

func newShotCmd(app *App) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "shot <url-or-(fileKey nodeId)>",
		Short: "Screenshot a node to a file (get_screenshot); prints only the path",
		Example: `  figma shot "https://www.figma.com/design/ABC123/My-App?node-id=12-34" -o header.png
  figma shot ABC123 12:34`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := refFromArgs(args, true)
			if err != nil {
				return err
			}
			res, err := app.callTool(cmd.Context(), tools.CapScreenshot, map[string]any{
				"nodeId": ref.NodeID,
			}, ref)
			if err != nil {
				return err
			}
			opts := app.outputOptions(cmd)
			if out != "" {
				opts.ImageDir = dirOf(out)
			}
			files, err := output.WriteResult(res, opts)
			if err != nil {
				return err
			}
			if out != "" && len(files) > 0 {
				if err := moveFile(files[0], out); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), out)
				return nil
			}
			for _, f := range files {
				fmt.Fprintln(cmd.OutOrStdout(), f)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "out", "o", "", "output file path (default: shot-<n>.png in cwd)")
	return cmd
}

func dirOf(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[:i]
	}
	return "."
}

// moveFile renames src to dst, falling back to copy+delete across volumes.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(dirOf(dst), 0o755); err != nil {
		return err
	}
	outF, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(outF, in); err != nil {
		outF.Close()
		return err
	}
	if err := outF.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}
