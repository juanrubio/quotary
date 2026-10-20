# Phase 9 fuzz campaign record

Date: 2026-10-20
Go: 1.27.2 linux/amd64

These are representative campaign results from independent timed runs; execution
counts vary by host load. `docs/evidence/phase-9-checks.txt` records a separate
campaign from the same day.

Commands and results:

- `go test ./internal/tui -run '^$' -fuzz FuzzWrapTextNeverPanicsOrOverflows -fuzztime=20s`
  — PASS, 112,037 execs. Regression seed retained at
  `internal/tui/testdata/fuzz/FuzzWrapTextNeverPanicsOrOverflows/1b4be214af2ae3b5`.
- `go test ./internal/tui -run '^$' -fuzz FuzzCleanTextStripsControls -fuzztime=20s`
  — PASS, 119,788 execs.
- `go test ./internal/backend/codexbar -run '^$' -fuzz FuzzNormalizeUsageNeverPanics -fuzztime=20s`
  — PASS, 225,942 execs.

The wrap fuzzer drops/replaces a rune wider than the target cell width with `?`;
this handles the retained CJK-at-width-1 regression seed. Fuzz invariants check
UTF-8 validity, display-cell width, `unicode.IsControl`, and finite/non-negative
percentages.
