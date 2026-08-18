package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gal-agent/go-figma-cli/internal/tools"
	"github.com/gal-agent/go-figma-cli/internal/xmlscan"
)

func newPipelineCmd(app *App) *cobra.Command {
	var (
		maxChildren int
		includeVars bool
		sets        []string
	)
	cmd := &cobra.Command{
		Use:   "pipeline <url-or-(fileKey nodeId)>",
		Short: "One-shot: tree -> direct child frames -> per-frame code (+ vars)",
		Long: `Runs the recommended drill-down in a single process so intermediate
payloads never hit the agent context:

  1. get_metadata on the frame        (kept out of stdout unless -v)
  2. direct child frames extracted    (kept out of stdout unless -v)
  3. get_design_context per child     (printed, sectioned)
  4. get_variable_defs on the frame   (printed last, optional)

If the frame has no child frames, falls back to calling get_design_context
on the frame itself.`,
		Example: `  figma pipeline "https://www.figma.com/design/ABC123/My-App?node-id=12-34"
  figma pipeline ABC123 12:34 --set clientFrameworks=vue --max 6
  figma pipeline ABC123 12:34 -v`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := refFromArgs(args, true)
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			metaRes, err := app.callTool(ctx, tools.CapMetadata, map[string]any{"nodeId": ref.NodeID}, ref)
			if err != nil {
				return fmt.Errorf("metadata: %w", err)
			}
			tree, err := xmlscan.Parse(metaRes.TextParts())
			if err != nil {
				return fmt.Errorf("parse metadata XML (server format drift? run `figma doctor`): %w", err)
			}
			root := xmlscan.Root(tree)
			children := root.DirectChildren()
			frames := children // every direct child is a section worth coding
			if verbose {
				fmt.Fprintf(cmd.ErrOrStderr(), "[verbose] root=%s children=%d\n", root.ID, len(frames))
			}
			if maxChildren > 0 && len(frames) > maxChildren {
				fmt.Fprintf(cmd.ErrOrStderr(), "[warn] limiting to first %d of %d children (--max)\n", maxChildren, len(frames))
				frames = frames[:maxChildren]
			}

			codeArgs, err := parseSets(sets)
			if err != nil {
				return err
			}

			targets := frames
			if len(targets) == 0 {
				targets = []*xmlscan.Node{{ID: ref.NodeID, Name: root.Name, Type: root.Type}}
			}

			for i, child := range targets {
				args := map[string]any{"nodeId": child.ID}
				for k, v := range codeArgs {
					args[k] = v
				}
				res, err := app.callTool(ctx, tools.CapDesignCtx, args, ref)
				if err != nil {
					return fmt.Errorf("design context for %s (%s): %w", child.ID, child.Name, err)
				}
				out := cmd.OutOrStdout()
				title := child.Name
				if title == "" {
					title = child.ID
				}
				fmt.Fprintf(out, "===== [%d/%d] %s (%s) =====\n", i+1, len(targets), title, child.ID)
				if err := app.printResult(cmd, res); err != nil {
					return err
				}
			}

			if includeVars {
				res, err := app.callTool(ctx, tools.CapVariables, map[string]any{"nodeId": ref.NodeID}, ref)
				if err != nil {
					return fmt.Errorf("variables: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "===== variables =====")
				if err := app.printResult(cmd, res); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&maxChildren, "max", 0, "limit number of child frames (0 = all)")
	cmd.Flags().BoolVar(&includeVars, "vars", true, "include get_variable_defs at the end")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "extra args passed to get_design_context, k=v")
	return cmd
}
