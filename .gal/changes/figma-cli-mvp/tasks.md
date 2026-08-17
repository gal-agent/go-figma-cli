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

## Findings during implementation (live, 2026-08-17)
- OAuth discovery chain works against the real server:
  `https://mcp.figma.com/.well-known/oauth-protected-resource` ->
  authorization endpoint `https://www.figma.com/oauth/mcp`.
- Figma rejects dynamic client registration (HTTP 403) -> login requires a
  pre-registered OAuth app client id (`--client-id` / FIGMA_CLIENT_ID),
  implemented with a clear actionable error otherwise.
- Remote endpoint answers 401 with RFC 9728 metadata before login; handled.

## Remaining (post-MVP)
- [ ] Full live OAuth round-trip once a client id is registered (browser step)
- [ ] Confirm exact remote-mode arg names for get_metadata without nodeId
      (pages) against the live server; `--set` is the escape hatch
- [ ] The GAL skill that drives this CLI (separate project)
