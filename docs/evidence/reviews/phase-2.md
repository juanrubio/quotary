# Phase 2 external review

- Phase: 2 — Deterministic demo backend and fixtures
- Date: 2026-10-19
- Reviewer: separate Codex CLI process, model `gpt-6-astra`
- Iterations: 14

## Blocking findings fixed

- Wired `--demo` to the demo backend, six deterministic scenarios, memory
  preferences, and queued → fetching → current refresh transitions.
- Added unknown-account keys, unique grouped window keys, validated usage and
  history scopes, fixture ownership, no-results/setup/credit/unsupported-detail
  coverage, and scenario-switch refresh generations.
- Replaced inert isolation hooks with an in-memory fixture `fs.FS`, an injected
  offline runner, blocked production access, static import checks, a PTY/strace
  startup trace, and negative controls.
- Made browser scenario transforms a shared `scenarios.js` source consumed by the
  reference and Go parity tests; parity covers every source/window, labels,
  groups, resets, ages, states, membership, and mutations.
- Added field-specific fixture diagnostics and deterministic map ordering.

## Round 14 — PASS

```text
VERDICT: PASS
PHASE: 2 — Deterministic demo backend and fixtures
SUMMARY: Phase 2's backend, fixtures, scenarios, memory preferences, and isolation
seams are implemented. Build, vet, lint, docs-check, unit, and race checks passed
offline; all six scenarios launched and quit successfully in a PTY.
FINDINGS:
- [P2] scenario-switch fixture-read assertion also counted construction reads.
OPEN ITEMS:
- none
REQUIRED FIXES:
- none
```

The P2 was fixed by requiring an additional fixture read after the scenario
switch. Verification: `docs/evidence/phase-2-checks.txt` and
`docs/evidence/phase-2-isolation.txt`.
