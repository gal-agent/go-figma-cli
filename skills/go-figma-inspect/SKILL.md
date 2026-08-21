---
name: go-figma-inspect
description: Inspect Figma design files. Load when user mentions or agent finds task highly matches Figma, inspect design, read Figma file, list pages, node tree, design structure, component variants, or explore Figma layout.
---

# go-figma-inspect

Inspect Figma design files via `go-figma-cli` (PAT-based REST API).

## PAT setup (first run or after 401/403)

Run `go-figma-cli doctor`. If it fails:

1. Tell the user: "Open https://www.figma.com/settings -> Security ->
   Personal access tokens -> Generate new token. Check BOTH:
   **File content - read-only** AND **Variables - read-only**.
   Copy the token (starts with figd_)."
2. Run: `go-figma-cli login --token figd_xxxxxxxx`
3. Re-run: `go-figma-cli doctor` to confirm.

Token stored at `<user config>/figma-cli/config.json`, reused automatically.

## Commands

```bash
go-figma-cli pages <url|key>          # list pages (tiny output)
go-figma-cli tree <url>               # node tree (XML, ~1-3KB)
go-figma-cli tree <url> --grep icon   # filter tree to matching nodes
go-figma-cli code <url>               # design context (structured text)
go-figma-cli vars <url>               # design tokens (local + published)
go-figma-cli shot <url> -o file.png   # screenshot to file (no base64)
go-figma-cli pipeline <url>           # tree->children->code+vars in one call
go-figma-cli doctor                    # verify token + connection
```

URL: any Figma link with `?node-id=...` (right-click frame -> Copy link to
selection). Or `fileKey nodeId` as two args.

## Workflow

1. `pages` or `tree` to locate frame IDs - never guess.
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
- `code` output is structured text, not raw JSON - typically 2-10KB per
  frame.
- `shot` writes to file - only the path enters context, never base64.
- Cache hits are free - exploit within and across sessions.
- Use `tree --grep <pattern>` instead of `tree --raw | grep` to find
  specific nodes without loading full JSON into context.

## Parallel calls (MANDATORY)

**You MUST issue independent commands in the same round whenever possible.**
Each round re-processes the full conversation history, so serializing
independent calls wastes tokens proportional to context size.

**Always parallelize these (zero data dependency):**
- `doctor` + `tree <url>` - run both at once.
- `tree <url>` + `vars <url>` - no dependency between them.
- Multiple `code <nodeA>` / `code <nodeB>` on sibling nodes - independent.
- Multiple `pipeline` on different top-level frames - independent.

**Never parallelize these (data dependency - prior output needed):**
- `pages` -> `tree` - tree needs page's node ID.
- `tree` -> `code <specific-child>` - code needs node ID from tree output.

**When in doubt:** if call B needs any value from call A's stdout, they
are dependent - serialize. Otherwise, parallelize.
