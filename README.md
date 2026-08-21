# go-figma-cli

Read Figma designs from the command line through the **Figma REST API**,
authenticated with a **personal access token (PAT)**. Built for AI coding
agents: zero resident tool schemas, drill-down intermediates stay out of
context, disk cache, image payloads written to files.

## Install

    go install github.com/gal-agent/go-figma-cli/cmd/go-figma-cli@latest

or download a prebuilt binary from
[Releases](https://github.com/gal-agent/go-figma-cli/releases/latest).

## AI agent setup

Copy the text below and paste it to your AI agent:

```
Install go-figma-cli and its skills:

Step 1 - Install the CLI binary:
- Go to https://github.com/gal-agent/go-figma-cli/releases/latest
- Download the file matching your OS and arch:
  - Linux:   go-figma-cli-linux-amd64   -> install to /usr/local/bin/go-figma-cli, run chmod +x on it
  - macOS:   go-figma-cli-darwin-arm64  (Apple Silicon) or go-figma-cli-darwin-amd64 (Intel) -> install to /usr/local/bin/go-figma-cli, run chmod +x on it
  - Windows: go-figma-cli-windows-amd64.exe -> install to a directory on your PATH (e.g. %LOCALAPPDATA%\Programs\go-figma-cli\go-figma-cli.exe, then add that directory to your User PATH if not already present)
- Verify: run `go-figma-cli --help` and confirm output is shown

Step 2 - Install the skills:
- The two skills live at https://github.com/gal-agent/go-figma-cli/tree/main/skills
  - go-figma-inspect/SKILL.md
  - go-figma-tocode/SKILL.md
- Download both directories and place them under your agent's skills directory so the agent can discover them at runtime

Step 3 - Configure the Figma PAT:
- Go to https://www.figma.com/settings -> Security -> Personal access tokens
- Create a token with "File content - read-only" scope
- Run: go-figma-cli login --token figd_xxxxxxxx
- Run: go-figma-cli doctor to verify
```

## Setup (once)

1. Create a PAT at <https://www.figma.com/settings>
   (**Security -> Personal access tokens**, scope: **File content - read-only**).
2. Save it:

       go-figma-cli login --token figd_xxxxxxxx

3. Verify:

       go-figma-cli doctor

The token is stored at `<user config>/figma-cli/config.json` (0600).
`$FIGMA_TOKEN` overrides the config file when set. If commands start
returning 401/403, the token was revoked or expired: generate a new one
and run `login` again.

## Usage

    go-figma-cli doctor                   # token + connectivity check
    go-figma-cli pages  <url|fileKey>     # page list
    go-figma-cli tree   <url>             # sparse node tree
    go-figma-cli code   <url>             # design context for one node
    go-figma-cli vars   <url>             # design tokens
    go-figma-cli shot   <url> -o x.png    # screenshot to file
    go-figma-cli pipeline <url>           # tree -> child frames -> code + vars

A URL is any Figma link with `?node-id=...` (right-click a frame ->
*Copy link to selection*), or pass `<fileKey> <nodeId>` as two arguments.

Global flags: `--ttl`, `--fresh`, `--no-cache`, `--raw`, `--image-dir`,
`-v`.

`pipeline` runs the recommended drill-down in one process: metadata and
child-frame extraction happen off-screen; only sectioned code and
variables are printed. If the frame has no children it falls back to
single-node code.

## Agent guidance

1. `pages` or `tree` to locate frame ids - never guess them.
2. `pipeline <frame-url>` for a full screen; `code` for one node.
3. Treat `code` output as a *design representation* and translate it to
   the project stack; align tokens with `vars`.
4. `shot` gives a visual reference for verification.
5. Results are cached (`--ttl`); use `--fresh` after the designer pushes
   an update.

## Error remediation

- **401/403**: token missing/expired/revoked or lacks file access ->
  regenerate at figma.com/settings and `go-figma-cli login --token ...`.
- **404**: file key or node id wrong -> re-check with `pages`/`tree`.

## Development

    go build ./... && go test ./...

Layout: `internal/restapi` (Figma REST client + renderers), `internal/config`
(PAT store), `internal/figmaurl`, `internal/cache`, `internal/output`,
`internal/xmlscan`, `internal/cli` (cobra commands), `internal/resttest`
(mock API used by the test suite), `internal/mcp` (result types only).
