# go-figma-cli

Read Figma designs from the command line through the **Figma REST API**,
authenticated with a **personal access token (PAT)**. Built for AI coding
agents: zero resident tool schemas, drill-down intermediates stay out of
context, disk cache, image payloads written to files.

## Install

    go install github.com/gal-agent/go-figma-cli/cmd/go-figma-cli@latest

or download a prebuilt binary from
[Releases](https://github.com/gal-agent/go-figma-cli/releases/latest).

## AI agent setup (copy-paste)

Paste the one-liner for your platform into your AI agent (GAL, Claude Code,
etc.) to install the CLI + skills:

**macOS / Linux:**

```
Run: curl -sL https://github.com/gal-agent/go-figma-cli/releases/latest/download/go-figma-cli-$(uname -s | tr A-Z a-z)-amd64 -o /usr/local/bin/go-figma-cli && chmod +x /usr/local/bin/go-figma-cli && curl -sL https://github.com/gal-agent/go-figma-cli/archive/refs/heads/main.tar.gz | tar xz && cp -r go-figma-cli-main/skills/* ~/.gal/skills/
```

**Windows (PowerShell):**

```
Run: $r="https://github.com/gal-agent/go-figma-cli/releases/latest/download"; $d="$env:LOCALAPPDATA\Programs\go-figma-cli"; New-Item -ItemType Directory -Force $d | Out-Null; Invoke-WebRequest "$r/go-figma-cli-windows-amd64.exe" -OutFile "$d\go-figma-cli.exe"; $p=[Environment]::GetEnvironmentVariable("PATH","User"); if($p -notlike "*$d*"){[Environment]::SetEnvironmentVariable("PATH","$p;$d","User")}; git clone --depth 1 https://github.com/gal-agent/go-figma-cli.git "$env:TEMP\go-figma-cli"; Copy-Item -Recurse -Force "$env:TEMP\go-figma-cli\skills\*" "$env:USERPROFILE\.gal\skills\"
```

Then set up the PAT:

```
Run: go-figma-cli login --token figd_xxxxxxxx && go-figma-cli doctor
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
