# Translation guide: design representation -> project stack

`code` output (default React + Tailwind) is a *description of the design*,
not the code to ship. How to read it depends on the target stack.

## General rules

- Layout structure (flex/grid, direction, gaps) is the ground truth — keep
  it. Class syntax and component names are incidental — replace them.
- Variants/states (hover, active, selected, disabled) describe required
  behavior. If the target component library has a variant prop, map to it;
  otherwise implement state with the project's styling convention.
- Repeated structures in the output (identical card N times) mean "one
  component, data-driven" — write the component once and map the sample
  data to props/state, do not copy-paste N blocks.
- Auto-layout padding/gap values are real design values; keep exact
  numbers unless a project token exists for them (then use the token).

## React + Tailwind project

- Closest to raw output; still route through project components: if a
  `<button>` in the output matches an existing `Button` component, use it
  with equivalent props instead of raw markup with utility classes.
- Replace one-off hex classes with the project's Tailwind tokens where they
  exist.

## Vue / Svelte / Solid

- Same structure; component boundaries translate 1:1. Convert JSX
  expressions to template syntax; keep prop names semantic (not Tailwind
  class strings as props).
- Tailwind classes survive if the project uses Tailwind; otherwise convert
  spacing/color values into the project's CSS approach (scoped CSS, CSS
  modules, style tokens) — values from the output, syntax from the project.

## Plain CSS / CSS-modules projects

- Read utility classes as a compact styling spec: `p-4` = 16px padding,
  `gap-2` = 8px, `text-sm` = 14px/20px, `rounded-lg` = 8px. Materialize
  them into classes that follow the project naming convention.
- Colors: take hex from output or token name from `vars` (prefer `vars`).

## Non-web (SwiftUI/Flutter) targets

- Pass `--set clientFrameworks=swiftui` (where supported) to get closer to
  the target idiom first, then apply the same rules: structure is truth,
  names are incidental, tokens beat literals.

## Tokens over literals (all stacks)

`vars` gives the authoritative token names/values. Output code may show
resolved values (hex, px). Always check `vars` before committing to a
literal: token `brand/primary` + hex in code = use the project's
equivalent token or create one.
