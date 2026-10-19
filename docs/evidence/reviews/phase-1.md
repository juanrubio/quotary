# Phase 1 external review

- Phase: 1 — Domain model, backend seam, and invariant tests
- Date: 2026-10-19
- Reviewer: separate Codex CLI process, model `gpt-6-astra`
- Command: `codex exec --model gpt-6-astra --ephemeral --sandbox workspace-write`
  with writable `GOCACHE`/`TMPDIR` and the review prompt from
  `docs/prompts/implement-all-phases.md`
- Iterations: 11

## Blocking findings fixed

- Canonical source keys: malformed percent escapes, empty variants, invalid
  UTF-8, raw controls, C1 controls, and known `"unknown"` accounts colliding with
  the missing-identity sentinel.
- Freshness: sample and receipt ages separated; missing sample time stays
  `time unknown`; staleness boundary at two intervals.
- Optionality: nil versus measured zero for balances, history totals, model
  totals, cost, detail bars, and chart points.
- State aggregation: error/quality/completeness precedence across windows,
  balances, and supported/unsupported details; retained auth failures; queued
  and fetching states; partial mixed families; unsupported extra details remain
  disclosures.
- Config store: slices copied on construction, load, and save; rejected saves
  preserve prior state.
- Import boundaries now cover the TUI and prove that a forbidden import fails.
- Stable ordering: case-insensitive names fall through to a source-key tie-break.
- Traceability: Phase 1 obligations name concrete tests and evidence.

## Round 11 — PASS

```text
VERDICT: PASS
PHASE: 1 — Domain model, backend seam, and invariant tests
SUMMARY: Phase 1 requirements and exit-gate coverage are implemented, with no
blocking findings. Independent offline build, vet, lint, docs-check, unit, and
race checks passed. One non-blocking Unicode search issue remains.
FINDINGS:
- [P2] Search lowercases only ASCII; éclair did not match Éclair.
OPEN ITEMS:
- none
REQUIRED FIXES:
- none
```

The P2 was fixed after round 11 with Unicode-aware search and
`TestFilterIsUnicodeCaseInsensitive`.

Verification: `docs/evidence/phase-1-checks.txt`.
