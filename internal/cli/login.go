package cli

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/piratecoder/go-figma-cli/internal/auth"
	"github.com/piratecoder/go-figma-cli/internal/cache"
	"github.com/piratecoder/go-figma-cli/internal/mcp"
	"github.com/piratecoder/go-figma-cli/internal/tools"
)

func newLoginCmd(app *App) *cobra.Command {
	var clientID, clientSecret string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authorize figma-cli with the remote Figma MCP server (one-time)",
		Long: `Runs the OAuth2 PKCE loopback flow against the Figma MCP authorization
server and caches the token.

Figma does not allow dynamic client registration (403), so you must pass a
client id from an OAuth app registered in your Figma account
(https://www.figma.com/developers/api - register redirect URI
http://localhost:<port>/callback, or fixed http://localhost). The client id
is remembered after the first login.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if app.desktop {
				fmt.Fprintln(out, "desktop mode talks to the local Figma app (127.0.0.1:3845) and needs no login.\n"+
					"Just open the Figma desktop app, switch to Dev Mode and enable the MCP server.")
				return nil
			}
			hc := &http.Client{Timeout: 60 * time.Second}
			fmt.Fprintf(out, "discovering OAuth endpoints for %s ...\n", app.endpoint())
			meta, err := auth.Discover(cmd.Context(), hc, app.endpoint())
			if err != nil {
				return fmt.Errorf("endpoint discovery failed: %w\n(if this persists, try --desktop mode)", err)
			}
			fmt.Fprintf(out, "authorization endpoint: %s\n", meta.AuthorizationEndpoint)

			endpoint, regErr := auth.Register(cmd.Context(), hc, meta)
			if regErr != nil {
				if clientID == "" {
					clientID = os.Getenv("FIGMA_CLIENT_ID")
					if clientSecret == "" {
						clientSecret = os.Getenv("FIGMA_CLIENT_SECRET")
					}
				}
				if clientID == "" {
					return fmt.Errorf(`Figma does not allow dynamic client registration (%v).
Register an OAuth app in your Figma account (redirect URI http://localhost) and re-run:
  figma login --client-id <YOUR_CLIENT_ID> [--client-secret <SECRET>]
or set FIGMA_CLIENT_ID / FIGMA_CLIENT_SECRET.`, regErr)
				}
				fmt.Fprintln(out, "dynamic registration unavailable; using provided client id")
				endpoint = &auth.Endpoint{
					AuthorizationURL: meta.AuthorizationEndpoint,
					TokenURL:         meta.TokenEndpoint,
					ClientID:         clientID,
					ClientSecret:     clientSecret,
				}
			}
			fmt.Fprintln(out, "client ready; starting browser authorization (PKCE)...")

			store := &auth.Store{Path: auth.DefaultStorePath()}
			tok, err := auth.Login(cmd.Context(), hc, endpoint, store)
			if err != nil {
				return err
			}
			if err := store.SaveEndpoint(endpoint); err != nil {
				return fmt.Errorf("saving endpoint: %w", err)
			}
			fmt.Fprintf(out, "authorized; token cached at %s (expires %s)\n",
				store.Path, tok.ExpiresAt.Format("2006-01-02 15:04 MST"))
			fmt.Fprintln(out, "verify with: figma doctor")
			return nil
		},
	}
	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth client id of your registered Figma app (also FIGMA_CLIENT_ID)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "OAuth client secret, if your app is confidential (also FIGMA_CLIENT_SECRET)")
	return cmd
}

// newDoctorCmd performs a handshake and prints a health/drift report.
func newDoctorCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check MCP connectivity, auth, tool inventory and alias drift",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			client, _, err := app.connect(cmd.Context())
			if err != nil {
				return err
			}
			init, err := client.Initialize(cmd.Context())
			if err != nil {
				if errors.Is(err, mcp.ErrUnauthorized) {
					return errors.New("server rejected our credentials; run `figma login` first")
				}
				return err
			}
			fmt.Fprintf(out, "server        : %s %s (protocol %s)\n",
				init.ServerInfo.Name, init.ServerInfo.Version, init.ProtocolVersion)
			fmt.Fprintf(out, "endpoint      : %s (mode=%s)\n", app.endpoint(), app.mode())

			list, err := client.ListTools(cmd.Context())
			if err != nil {
				return fmt.Errorf("tools/list: %w", err)
			}
			res := tools.NewResolver(list)
			fmt.Fprintf(out, "tools exposed : %d\n", len(list))
			for _, name := range res.Names() {
				fmt.Fprintf(out, "  - %s\n", name)
			}

			fmt.Fprintln(out, "core capabilities:")
			for _, cap := range []string{tools.CapMetadata, tools.CapDesignCtx, tools.CapVariables, tools.CapScreenshot} {
				status := "OK"
				if !res.Has(cap) {
					status = "MISSING (aliases exhausted - Figma likely renamed tools)"
				}
				fmt.Fprintf(out, "  %-20s %s\n", cap, status)
			}

			fmt.Fprintf(out, "cache         : %s\n", cache.DefaultDir())
			fmt.Fprint(out, "auth          : ")
			if app.desktop {
				fmt.Fprintln(out, "not required (desktop mode)")
			} else {
				store := &auth.Store{Path: auth.DefaultStorePath()}
				tok, err := store.Load()
				switch {
				case err != nil || tok == nil:
					fmt.Fprintln(out, "no token - run `figma login`")
				case !tok.Valid():
					fmt.Fprintln(out, "token expired - refresh is attempted automatically; else run `figma login`")
				default:
					fmt.Fprintf(out, "valid until %s\n", tok.ExpiresAt.Format("2006-01-02 15:04"))
				}
			}
			return nil
		},
	}
}
