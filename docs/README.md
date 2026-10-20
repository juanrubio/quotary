# Quotary documentation

Source-of-truth documents for the Quotary prototype.

## Specifications

- [Product and technical specification](specs/product.md): scope, coverage,
  backend contract, data meaning, stages, and acceptance criteria.
- [Visual and interaction specification](specs/tui.md): layouts, keyboard
  behavior, states, themes, and small-terminal requirements.

## Delivery

- [Implementation plan](implementation-plan.md): phases, exit gates, package
  map, test strategy, traceability, and definition of done.
- [Implementation prompt](prompts/implement-all-phases.md): the phase loop and
  mandatory external Codex/GPT-6 Astra review gate.
- [Evidence](evidence): live checks, review records, captures, and measurements.
- [Phase 8 live validation record](evidence/live-validation.md): unavailable
  credentials and setup limits, recorded honestly.
- [Fixture policy](fixtures-policy.md) and [VHS tape guide](tapes.md).

## Decisions

- [0004 — Four named theme palettes](decisions/0004-theme-palettes.md)

## References

- [Interactive mockups](mockups/index.html) and
  [implementer guide](mockups/README.md): 15 reference screens, fixtures, and
  reproducible exports.
- [CodexBar 0.77.0 provider catalog](reference/codexbar-0.77.0-providers.json):
  pinned public catalog metadata.

## Conventions

The root `README.md`, `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`, and `LICENSE`
are the only top-level documents. Everything else lives under `docs/`.
