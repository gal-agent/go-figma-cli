# go-figma-inspect

Inspect Figma design files via `go-figma-cli` (PAT-based REST API).

## Auto-install

```bash
# Check if CLI exists
go-figma-cli version 2>/dev/null || {
  # macOS/Linux: download from GitHub releases
  # Windows: scoop install or download exe
  # Ask user to run: /sudo go install github.com/gal-agent/go-figma-cli/cmd/go-figma-cli@latest
}
```

If `go-figma-cli` is not on PATH, ask the user to install it (requires /sudo
for system PATH changes). Do NOT proceed with inspection until it works.

## PAT setup (first run or after 401/403)

Run `go-figma-cli doctor`. If it fails:

1. Tell the user: "Open https://www.figma.com/settings → Security →
   Personal access tokens → Generate new token. Scope: **File content -
   read-only**. Copy the token (starts with figd_)."
2. Run: `go-figma-cli login --token figd_xxxxxxxx`
3. Re-run: `go-figma-cli doctor` to confirm.

The token is stored at `<user config>/figma-cli/config.json` and reused
automatically. Reconfigure only when expired/revoked.

## Commands (token-efficient)

```bash
go-figma-cli pages <url|key>          # list pages (tiny output)
go-figma-cli tree <url>               # node tree (XML, ~1-3KB)
go-figma-cli code <url>               # design context (structured text)
go-figma-cli vars <url>               # design tokens (local + published)
go-figma-cli shot <url> -o file.png   # screenshot to file (no base64)
go-figma-cli pipeline <url>           # tree→children→code+vars in one call
go-figma-cli doctor                    # verify token + connection
```

URL: any Figma link with `?node-id=...` (right-click frame → Copy link to
selection). Or `fileKey nodeId` as two args.

## Workflow

1. `pages` or `tree` to locate frame IDs — never guess.
2. `pipeline <frame-url>` for a full screen (one call, intermediates
   off-screen). Use `--max N` to limit children.
3. `code <node-url>` for a single node's detail.
4. `vars <url>` to align design tokens with project tokens.
5. `shot <url> -o ref.png` for visual reference.

Results are disk-cached (`--ttl`, default 10min). Use `--fresh` only after
the designer updates the file. `--no-cache` for one-shot reads.

## Token efficiency

- `pipeline` is the most efficient: one command does tree + per-child code
  + vars, intermediates never hit stdout.
- `pages`/`tree` outputs are compact XML (~1-3KB).
- `code` output is structured text, not raw JSON — typically 2-10KB per
  frame.
- `shot` writes to file — only the path enters context, never base64.
- Cache hits are free — exploit within and across sessions.
- Avoid `--raw` unless debugging — it dumps full JSON.
