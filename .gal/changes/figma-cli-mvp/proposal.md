# figma-cli MVP

## Intent

Wrap the official Figma MCP server (`https://mcp.figma.com/mcp`, or desktop
`http://127.0.0.1:3845/mcp`) in a Go CLI so that AI coding agents can consume
Figma designs via bash without paying the MCP "connection tax" (all 30 tool
definitions resident in every LLM request, ~6-12k tokens) and without dumping
raw intermediate artifacts into the conversation.

Measured rationale (session 2026-08-17):
- MCP tool definitions: ~150-680 tokens/tool (wire-format measurement of two
  real MCP servers), 30 official Figma tools => ~6-12k resident tokens/request.
- A drill-down session (pages -> sparse tree -> per-frame code) leaves every
  intermediate response in context when driven through an MCP client directly.
- CLI: zero resident cost; only final (optionally trimmed) output enters
  context; disk cache removes repeat calls.

## Scope (MVP)

Commands:
- `figma login`        one-time OAuth2 PKCE authorization for the remote server
- `figma doctor`       handshake + tools/list + drift report vs expected aliases
- `figma pages <url>`  top-level page list (get_metadata without nodeId)
- `figma tree <url>`   sparse node tree (get_metadata with nodeId)
- `figma code <url>`   design context / codegen (get_design_context)
- `figma vars <url>`   design tokens (get_variable_defs)
- `figma shot <url>`   screenshot to file (get_screenshot)
- `figma pipeline <url>` tree -> direct child frames -> per-frame code + vars,
                       one command, intermediate steps never printed

Global flags: `--desktop` (localhost:3845, no auth), `--no-cache`, `--fresh`,
`--ttl`, `--raw` (full JSON-RPC result), `--set k=v` passthrough args.

## Non-goals (MVP)

- Write/canvas tools (use_figma etc.) - read-only pipeline only.
- The GAL skill that drives this CLI (separate project).
- Multi-account auth, team management.
- REST API fallback (PAT mode) - the MCP path already covers MVP needs.

## Approach

- Go + cobra. MCP client implemented on stdlib net/http: JSON-RPC 2.0 over
  Streamable HTTP, handles both `application/json` and `text/event-stream`
  responses, session id propagation (`mcp-session-id` header).
- Auth: RFC 9728 protected-resource discovery -> AS metadata -> dynamic client
  registration (RFC 7591) when available -> PKCE loopback flow. Tokens cached
  in ~/.config/figma-cli/auth.json (0600).
- Tool drift defense: alias table (get_design_context|get_code,
  get_screenshot|get_image) resolved against a live tools/list; `doctor`
  reports drift. No hardcoded assumptions beyond documented param names.
- Cache: sha256(mode|fileKey|nodeId|tool|args) under ~/.cache/figma-cli/,
  TTL-based, stores full CallToolResult.
- Output policy: text content printed; image content decoded to files (path
  printed, base64 never hits stdout).

## Verification

- Unit tests against an in-process mock MCP server (httptest): handshake,
  SSE parsing, alias resolution, cache TTL, URL parsing, XML child extraction,
  pipeline end-to-end.
- `go build`, `go vet`, `go test ./...` green.
- Live smoke vs real endpoints is manual (OAuth needs a browser): `doctor`
  against desktop mode; 401 handling against remote without login.
