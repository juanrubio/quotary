# Coverage record

Date: 2026-10-20

## Catalog

- Reference catalog IDs: 106 (CodexBar 0.77.0 pinned snapshot).
- Manifest rows: 106; review-log rows: 106.
- Selectable live descriptors after Gemini variant split: 108 (106 IDs + configured/web/api replacing one generic Gemini = +2). This is the amended plan, SPEC, mockup-guide, demo-catalog, and stress-fixture baseline; the rationale is the explicit configured mode plus the two product-required app/CLI variants.
- Classification: subscription 20, mixed 0, credits 1, metered API 1, non-usage 0, unknown 84.
- Active probes: Azure OpenAI only (outside prototype; excluded from bulk and auto-poll).
- Review evidence: `docs/evidence/catalog-review.md`; manifest: `internal/catalog/manifest.go`.

## Fixture coverage

Synthetic/redacted fixtures under `testdata/codexbar/` cover quota windows, credits, money, grouped/model windows, generic details, source variants, multi-account, history arrays/objects, partial success, mixed results, malformed JSON, unsupported shapes, and error-on-exit-zero. Tests live in `internal/backend/codexbar`.

## Live status

All live cases are **unavailable**, not passed. No CodexBar executable or credentials were present. See `docs/evidence/live-validation.md` and the open Phase 8 rows in the implementation plan.
