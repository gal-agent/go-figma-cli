---
name: go-figma-tocode
description: Convert Figma designs to UI code. Load when user mentions or agent finds task highly matches Figma to code, implement Figma design, convert design to component, figma-to-react, figma-to-vue, design to HTML, or build UI from Figma.
---

# go-figma-tocode

Implement Figma designs as UI code via `go-figma-cli` (PAT-based REST API).

## PAT setup

Run `go-figma-cli doctor`. If it fails (401/403 or missing token):

1. Tell the user: "Open https://www.figma.com/settings -> Security ->
   Personal access tokens -> Generate new token. Check BOTH:
   **File content - read-only** AND **Variables - read-only**."
2. Run: `go-figma-cli login --token figd_xxxxxxxx`
3. Verify: `go-figma-cli doctor`

Token stored at `<user config>/figma-cli/config.json`, reused automatically.

## Commands

```bash
go-figma-cli pipeline <url>           # tree->children->code+vars (best for handoff)
go-figma-cli code <url>                # single node design context
go-figma-cli tree <url>                # node structure
go-figma-cli vars <url>                # design tokens (local + published)
go-figma-cli shot <url> -o ref.png     # screenshot to file
go-figma-cli pages <url|key>           # page list
```

URL: Figma link with `?node-id=...` (right-click frame -> Copy link).
Or `fileKey nodeId` as two args. Output is disk-cached; `--fresh` to refresh.

## Workflow

1. Detect project stack from repo (package.json, components/, tailwind.config,
   .vue/.svelte files) BEFORE calling `code`.
2. `pipeline <frame-url>` - one call gets tree + per-child design context +
   variables. Use `--max N` for large frames.
3. `code <node-url>` for individual nodes that need more detail.
4. `vars <url>` - map design tokens to project tokens (prefer tokens over
   literals).
5. `shot <url> -o ref.png` - visual reference for self-check.

## How to read `code` output

The `code` command outputs structured **design context** (indented text, not
JSON, not runnable code). Translate it to the project stack:

- **Layout** (flex/grid, direction, gap, padding) is ground truth - keep it.
- **Component properties/variants** are listed as `props: Size=Large|Small`
  - map to variant props in the target component library.
- **Constraints** (`constrain: left/top`) describe resize behavior - map to
  flex/grid sizing.
- **Layout grids** (`grid: columns stretch count=4 gutter=16`) map to CSS
  grid.
- **Colors** from `vars` take priority over hex literals in `code`.
- **Bound variables** (`bound: fill->Var:123`) link to design tokens -
  always resolve via `vars`.
- Repeated blocks = one component, data-driven. Write it once.
- The output is stack-neutral (no React/Vue/Tailwind bias) - map to the
  project's styling convention.

## Token efficiency

- `pipeline` is most efficient: intermediates off-screen, only sectioned
  code + vars printed.
- Cache hits are free - don't re-fetch within `--ttl`.
- `--fresh` only after designer updates the file.
- `shot` writes to file - only the path enters context.
- Avoid `--raw` unless debugging.

## Parallel calls

Independent commands can be issued in the same round to reduce round-trips:

- `tree` + `vars` - no dependency, run both at once.
- Multiple `code <nodeA>` / `code <nodeB>` on sibling nodes - independent.
- Multiple `pipeline` on different top-level frames - independent.

Do NOT parallelize calls with dependencies (`pages` -> `tree` -> `code`
each needs the prior output's node IDs).
