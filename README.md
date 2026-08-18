# figma-cli

Read Figma designs from the command line through the **official Figma MCP
server**, wrapped for AI coding agents.

## Why

Driving the official MCP directly from an agent client costs:

- **~6-12k resident tokens per request** - all ~30 tool definitions are
  registered into every LLM call, even for sessions that never touch Figma.
- **Intermediate payloads in context** - the recommended drill-down
  (page list -> sparse tree -> per-frame code) leaves every step in the
  conversation.
- **No caching** - repeat reads of the same node hit the API and the
  rate limit again.

`figma-cli` fixes all three: zero resident definitions (it is just a bash
call), drill-down intermediates stay inside the process, and results are
disk-cached. Image payloads are written to files - base64 never hits stdout.

## Install

    go install github.com/gal-agent/go-figma-cli/cmd/figma@latest

## Auth

Two modes:

- **Remote** (default, `https://mcp.figma.com/mcp`): one-time OAuth2 login.
  Figma does not allow dynamic client registration (403), so register an
  OAuth app in your Figma account (redirect URI `http://localhost`) and:

      figma login --client-id <ID> [--client-secret <SECRET>]
      # or FIGMA_CLIENT_ID / FIGMA_CLIENT_SECRET

  Tokens are cached in `~/.config/figma-cli/auth.json` and refreshed
  automatically. Available on all plans, including free Starter (limited
  monthly calls; Dev/Full seats get hundreds/day).

- **Desktop** (`--desktop`, `http://127.0.0.1:3845/mcp`): no OAuth; open the
  Figma desktop app, switch to Dev Mode and enable the MCP server. Requires
  a paid plan seat.

## Usage

    figma doctor                # connectivity, tool inventory, alias drift
    figma pages  <url|fileKey>  # page list
    figma tree   <url>          # sparse node tree (--set depth=2)
    figma code   <url>          # design context / codegen (--set clientFrameworks=vue)
    figma vars   <url>          # design tokens
    figma shot   <url> -o x.png # screenshot to file
    figma pipeline <url>        # one-shot: tree -> child frames -> code + vars

A URL is any Figma link with `?node-id=...` (right-click a frame ->
*Copy link to selection*), or pass `<fileKey> <nodeId>` as two arguments.

Global flags: `--desktop`, `--url`, `--ttl`, `--fresh`, `--no-cache`,
`--raw`, `--image-dir`, `-v`.

`pipeline` runs the recommended drill-down in one process: the metadata and
child-frame extraction happen off-screen; only sectioned code and variables
are printed. If the frame has no children it falls back to single-node code.

## Agent guidance (for the skill layer)

1. `figma pages` or `figma tree` to locate frame ids - never guess them.
2. `figma pipeline <frame-url>` for a full screen; `figma code` for one node.
3. Treat `code` output as a *design representation* (default React+Tailwind)
   and translate it to the project stack; align tokens with `vars`.
4. `figma shot` gives a visual reference for verification.
5. Results are cached (`--ttl`); use `--fresh` after the designer pushes an
   update.

## Robustness against upstream drift

Figma renames tools occasionally (`get_code` -> `get_design_context`,
`get_image` -> `get_screenshot`). The CLI resolves capabilities through an
alias table against a live `tools/list` at connect time; `figma doctor`
reports drift explicitly.

## Development

    go build ./... && go test ./...

Layout: `internal/mcp` (JSON-RPC/Streamable-HTTP client + SSE),
`internal/auth` (OAuth2 PKCE + token store), `internal/figmaurl`,
`internal/tools` (alias resolver), `internal/cache`, `internal/output`
(content filtering), `internal/xmlscan` (sparse-XML tree extraction),
`internal/cli` (cobra commands), `internal/mcptest` (mock server used by
the test suite).
