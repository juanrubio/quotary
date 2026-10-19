# 0004 — Four named theme palettes

- Status: Accepted
- Date: 2026-10-19
- Supersedes: none
- Related: docs/specs/tui.md §3, docs/specs/product.md §8

## Context

Quotary's original visual specification defines one semantic token set with dark
and light variants, plus monochrome. The sibling CLI Trailmark already ships four
named themes (Green, Blue, Amber, Monokai) with light and dark palettes, a
Settings chooser, persistence, and WCAG AA contrast tests. Reusing that model gives
Quotary a familiar, tested appearance without adding provider-specific styling.

The theme system is a deliberate extension of the existing specifications, not an
existing requirement, so it must be documented rather than introduced silently.

## Decision

Quotary ships four named palettes: Green (default), Blue, Amber, and Monokai.

- **Green** uses Quotary's UX §3 dark and light tokens as its anchor.
- **Blue**, **Amber**, and **Monokai** adopt Trailmark's structural colours
  (panel, text, muted, accent, selection, path, separator, raised) from
  `../trailmark/internal/tui/theme.go` (`darkPalette`, `lightPalette`, and
  `themePalette`), mapped onto Quotary's semantic tokens.
- Every palette defines explicit Quotary state tokens — Available, Low,
  Exhausted/Error, and Stale — instead of overloading Trailmark's decorative
  `gold`, `warning`, or `path` fields. State meaning is identical in all palettes.
- Light or dark is chosen from the terminal background report
  (`tea.BackgroundColorMsg`), falling back to dark. The manual auto/dark/light
  override remains available.
- Monochrome is an orthogonal rendering mode, not a fifth palette. `NO_COLOR`,
  `TERM=dumb`, and the explicit monochrome setting suppress colour and use `>`,
  border weight, and ASCII gauges for state cues.
- The theme name is persisted through `config.Store`; absent or unknown names
  normalize to Green. A failed save keeps the active theme and shows a concise
  error. Changing theme does not refresh backend data.
- Resolved styles are cached and invalidated on theme, light/dark, monochrome, or
  terminal-background changes.

## Attribution

Trailmark is MIT-licensed, copyright © 2026 Juan A. Rubio. Copied structural
palette values retain that attribution. Quotary's own state tokens and semantic
mapping are original to this project.

## Consequences

- docs/specs/tui.md and docs/specs/product.md record the named themes, persistence rule, and acceptance
  criterion.
- `internal/theme/` owns all palette values; TUI components use semantic tokens
  only and never raw colours.
- Tests cover all four palettes in light and dark, WCAG AA contrast for text,
  muted, accent, and state tokens against Canvas and Surface, unknown-name
  normalization, save failure, style-cache invalidation, and monochrome state
  cues.
- Adding or renaming a palette requires a config normalization update and a
  contrast test in the same change.
