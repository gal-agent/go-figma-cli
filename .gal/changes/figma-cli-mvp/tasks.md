# Tasks

## Setup
- [x] go mod init, cobra dependency verified
- [x] proposal.md

## Core libs
- [x] internal/mcp: client (initialize/session/tools.list/tools.call), SSE parser, types
- [x] internal/auth: PKCE + DCR + token store
- [x] internal/figmaurl: parse fileKey/nodeId from design/file URLs
- [x] internal/tools: alias resolver + typed wrappers
- [x] internal/cache: TTL disk cache
- [x] internal/output: content filtering (images->files)
- [x] internal/xmlscan: direct-child frame extraction from sparse XML

## CLI
- [x] login, doctor, pages, tree, code, vars, shot, pipeline

## Verify
- [x] mock-server tests: mcp handshake/SSE, alias, cache, url, xmlscan, pipeline e2e
- [x] go build && go vet && gofmt && go test ./... green

## Findings during implementation (live, corrected 2026-08-19)
- Figma Remote MCP is at `https://mcp.figma.com/mcp` and requires OAuth.
- Figma's Remote MCP documentation limits access to clients listed in the Figma
  MCP Catalog. The published dynamic-registration endpoint returned HTTP 403
  during a real CLI test, so dynamic registration is not a self-service
  onboarding path for this project.
- A future Remote integration requires Figma-provisioned MCP client credentials;
  ordinary Figma REST OAuth app credentials cannot request `mcp:connect`.
- The local Desktop MCP server remains supported at
  `http://127.0.0.1:3845/mcp`. Enable it in an open design file through Dev Mode
  -> Inspect -> MCP server -> Enable desktop MCP server. It requires a paid plan
  with a Dev or Full seat.

## Remaining (post-MVP)
- [ ] Register go-figma-cli as a Figma Remote MCP client and verify its
      Figma-provisioned OAuth credentials end-to-end.
- [ ] Confirm exact hosted-mode argument names for get_metadata without nodeId
      (pages) after the approved Remote integration is authorized; `--set` is
      the escape hatch.
- [ ] Consider a Figma REST API backend for headless/server-side use.
