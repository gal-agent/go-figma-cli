---
name: go-figma-tocode
description: Implement Figma designs as UI code via go-figma-cli. Load when user mentions or agent finds task highly matches figma to code, implement design, 按稿实现, 实现这个页面/组件, design handoff, UI 还原.
---

# Figma design to code (go-figma-cli)

End-to-end: turn a Figma link into working UI code **in the project's own
stack**. The CLI is self-documenting for syntax (`figma --help`, and every
command has `--help` with examples); this skill teaches the process
discipline that makes the output accurate and cheap.

## Pre-flight (once per session)

1. `figma doctor` must pass. If it fails, its output names the remediation
   (login / desktop mode / renamed tools); full OAuth walkthrough lives in
   the go-figma-cli repo README. Do not proceed before it is green.
2. Detect the project stack from the repo (package.json, components dir,
   tailwind config, vue/svelte markers) BEFORE calling `code`. The whole
   point of this workflow is landing code in the project's stack, so know
   the target first.

## Workflow

### 1. Orient, never blind-convert
Run `pages` / `tree` to locate the exact target frame and its child frames.
**Never convert a whole page or a huge frame**: official guidance is that
large selections return truncated, low-quality output. Target child-frame
granularity (Card / Header / Sidebar size).

### 2. Extract per child frame
Default: `pipeline` on the frame (does tree -> child frames -> code+vars in
one process; intermediates stay out of context). Use `code` per frame when
you only need some children. If a child frame is itself huge, go one level
deeper with `tree` on that child and split again.

### 3. Treat output as a design representation
The generated React + Tailwind (or `--set clientFrameworks=...` variant) is
**not the deliverable**. It encodes layout, hierarchy, spacing and variants.
Translate it to the project's components, utilities and conventions — see
resources/translation-guide.md for mapping heuristics.

### 4. Align design tokens
`vars` lists the design tokens (colors/spacing/type) the frame uses. Map
them to the project's token system (CSS variables, theme file, tailwind
config). Do not hardcode a hex value that exists as a token; if the project
lacks a token, define one instead of scattering literals.

### 5. Verify visually
`shot` the frame to a file, look at it (image tools), and compare against
what you wrote. Iterate on mismatches before declaring done.

### 6. Designer updated the file?
Re-run the affected commands with `--fresh` **on those nodes only**. Blanket
`--fresh` wastes quota; cache hits are free and instant.

## Discipline

- **Quota-aware**: every MCP call counts against Figma rate limits. Cache
  is on by default — exploit it within and across sessions. `--fresh` is a
  deliberate act, not a default.
- **Node-id hygiene**: take ids from `tree`/`pages` output; pasting the
  Figma link as-is is equally fine. `12-34` and `12:34` are the same node.
- **Screenshots are files**: image payloads are written to disk and only
  the path is printed. Reference them by path; never inline base64.
- **Icons/raster assets**: not exported by these commands yet — export via
  the Figma UI meanwhile.

## Boundaries

- Quick lookups without implementing (check a token, screenshot a node)?
  That is the `go-figma-inspect` skill — lighter workflow, same CLI.
- Setup/login troubleshooting is NOT a skill: `figma doctor` and the CLI
  README carry it.
