---
name: go-figma-inspect
description: Inspect Figma files via go-figma-cli: pages, node tree, variables, screenshots. Load when user mentions or agent finds task highly matches check go-figma-cli node, design tokens, go-figma-cli 截图, 看下设计稿, 变量查询.
---

# Figma inspection (go-figma-cli)

Answer questions about a Figma design **without generating code**: structure,
tokens, visual reference. Syntax lives in `go-figma-cli --help` (every command has
examples); this skill is about choosing the right probe and not wasting
quota.

## CLI prerequisite

Before using this skill, ensure `go-figma-cli` is available on `PATH`. If missing,
download the matching asset from the latest release:
`https://github.com/gal-agent/go-figma-cli/releases/latest`.

| Platform | Release asset | Install as |
|---|---|---|
| Windows x64 | `go-figma-cli-windows-amd64.exe` | `%LOCALAPPDATA%\Programs\go-figma-cli\go-figma-cli.exe` |
| Windows ARM64 | `go-figma-cli-windows-arm64.exe` | `%LOCALAPPDATA%\Programs\go-figma-cli\go-figma-cli.exe` |
| Linux x64 | `go-figma-cli-linux-amd64` | `~/.local/bin/go-figma-cli` |
| Linux ARM64 | `go-figma-cli-linux-arm64` | `~/.local/bin/go-figma-cli` |
| macOS Intel | `go-figma-cli-darwin-amd64` | `~/.local/bin/go-figma-cli` |
| macOS Apple Silicon | `go-figma-cli-darwin-arm64` | `~/.local/bin/go-figma-cli` |

Rename it exactly as shown. On Linux/macOS run
`chmod +x ~/.local/bin/go-figma-cli`; on every platform add the parent directory to
`PATH` if needed. Prefer the release binary—Go is only needed to build from
source. Verify it using `checksums.txt`, then run `go-figma-cli --help`.


## Intent -> command

| Question | Command |
|---|---|
| What pages exist in this file? | `pages` |
| How is this frame structured / where is X? | `tree` (ids from its output feed follow-ups) |
| Which color / spacing / font does it use? | `vars` (token names + values) |
| Show me what it looks like / visual reference | `shot` (writes a file, prints the path; view with image tools) |

`code` also exists but belongs to the `go-figma-tocode` workflow — for pure
lookups, prefer `tree` + `vars` + `shot`: they are cheap and precise.

## Caching discipline (this is where tokens are saved)

- Reads are disk-cached (default TTL). Re-checking a node you already
  looked at — even in a later session — is free. Exploit that.
- `--fresh` re-fetches from Figma and spends quota. Use it only when the
  user says the design changed, and only on the affected nodes.
- If `doctor` fails mid-session, fix per its output (remediation is
  printed; OAuth details in the go-figma-cli repo README).

## Reading the outputs

- `tree` prints a sparse node tree: ids, names, types, sizes. Use the ids
  verbatim in follow-up commands (`12-34` == `12:34`; the Figma link
  pasted as-is also works).
- `vars` output is the token source of truth — prefer token names over
  hardcoded values when reporting or writing code.
- `shot` writes the image under the project (or `--image-dir`); reference
  it by path in notes/comparisons, never inline base64.
