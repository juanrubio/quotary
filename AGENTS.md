# Quotary contributor guide

Quotary is a Go terminal dashboard for AI subscription usage. Start with the
documentation index at [docs/README.md](docs/README.md).

## Working agreements

- Product behavior: [docs/specs/product.md](docs/specs/product.md)
- Visual and interaction behavior: [docs/specs/tui.md](docs/specs/tui.md)
- Delivery sequence: [docs/implementation-plan.md](docs/implementation-plan.md)
- Architecture decisions: [docs/decisions](docs/decisions)
- Visual reference: [docs/mockups](docs/mockups)

## Commands

```sh
make build
make test
make race
make check
make run ARGS="--demo"
```

`make check` runs vet, lint, docs-check, and tests. CI runs the same target.

## Conventions

- Thin `cmd/quotary`; Cobra in `internal/cli`.
- Bubble Tea v2 and Lip Gloss v2 in `internal/tui`.
- Backend-agnostic values in `internal/domain`; provider DTOs never escape
  `internal/backend/codexbar`.
- Demo mode is fully offline and uses fictional data only.
- Tests use synthetic or redacted fixtures ([policy](docs/fixtures-policy.md)).
  Never commit credentials, cookies, raw provider payloads, account emails, or
  project paths.
- Every document except the root collaboration files lives under `docs/`.

## Phase reviews

Implementation phases are reviewed by an external Codex process using GPT-6
Astra before they are committed. The exact gate is documented in
[docs/prompts/implement-all-phases.md](docs/prompts/implement-all-phases.md).
Review records live under [docs/evidence/reviews](docs/evidence/reviews).
