package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gal-agent/go-figma-cli/internal/cache"
	"github.com/gal-agent/go-figma-cli/internal/config"
)

const patHelp = `Create a personal access token:
  1. Open https://www.figma.com/settings
  2. Security -> Personal access tokens -> Generate new token
  3. Scope: File content - read-only (enough for all read commands)
  4. Copy the token (starts with "figd_"; shown only once)`

func newLoginCmd(app *App) *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Save a Figma personal access token (PAT) to the config file",
		Long: `Saves a Figma personal access token for all later commands.

` + patHelp + `

The token is stored at ` + "`<user config>/figma-cli/config.json`" + ` (0600).
$FIGMA_TOKEN overrides the config file when set.
If commands ever return 401/403, the token was revoked or expired:
generate a new one and run login again.`,
		Example: `  go-figma-cli login --token figd_xxxxxxxx
  go-figma-cli doctor`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if token == "" {
				token = os.Getenv("FIGMA_TOKEN")
			}
			if token == "" {
				return fmt.Errorf("pass the token with --token (or set FIGMA_TOKEN)\n\n%s", patHelp)
			}
			if !strings.HasPrefix(token, "figd_") {
				fmt.Fprintln(cmd.ErrOrStderr(), "[warn] token does not start with figd_; continuing anyway")
			}
			if err := config.Save(&config.Store{PAT: token}); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Fprintf(out, "token saved to %s\n", config.DefaultPath())
			fmt.Fprintln(out, "verify with: go-figma-cli doctor")
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Figma personal access token (figd_...)")
	_ = token
	return cmd
}

func newLogoutCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored Figma token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.Clear(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "token removed")
			return nil
		},
	}
}

func newDoctorCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "doctor",
		Short:   "Verify the REST API connection and token",
		Example: `  go-figma-cli doctor`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.restDoctor(cmd.Context(), cmd.OutOrStdout())
		},
	}
}

var _ = cache.DefaultDir
