# Phase 0 external review

- Phase: 0 — Repository, documentation, and tooling foundation
- Date: 2026-10-19
- Reviewer: separate Codex CLI process, model `gpt-6-astra`
- Command: `codex exec --model gpt-6-astra --ephemeral --sandbox workspace-write -C .`
  with a writable `GOCACHE`/`TMPDIR`, plus the review prompt in
  `docs/prompts/implement-all-phases.md`
- Iterations: 4

## Round 1 — CHANGES REQUIRED

- `.gitignore` ignored `cmd/quotary/main.go`.
- `teatest` was not pinned in the module.
- `docs-check` did not cover reference links or anchors.
- Quit tests did not prove terminal restoration.
- Fixture and tape policy documents lived outside `docs/`.

## Round 2 — CHANGES REQUIRED

- Required `tapes/` and `testdata/codexbar/` directories had no tracked entries.
- `docs-check` missed titled and multiline links.
- Terminal restoration evidence did not check exit status or `stty` restoration.
- `.golangci.yml` used obsolete v2 keys.

## Round 3 — CHANGES REQUIRED

- `docs-check` missed nested outer link destinations around images.
- Residual `UX.md` references remained after the docs move.

## Round 4 — PASS

```text
VERDICT: PASS
PHASE: 0 — Repository, documentation, and tooling foundation
SUMMARY: Phase 0 requirements and exit gates are satisfied. Independent offline
checks passed, and PTY testing confirmed clean terminal restoration on both quit
paths. The shell preserves demo isolation and introduces no provider coupling;
later-phase behavior and visual acceptance remain correctly unclaimed.
FINDINGS:
- [P2] scripts/review-phase.sh:65 — Workspace protection compares only Git status —
  Editing an already-modified or untracked file can leave its status unchanged —
  Strengthen the guard with before/after content hashes.
EVIDENCE CHECKED:
- All 42 staged changes inspected; no unstaged or untracked files.
- Passed make check, make docs-check, go build ./..., make race, uncached unit and
  race tests, go mod verify, shell syntax checks, and whitespace checks.
- Verified release version stamping, help/version output, and nonterminal rejection.
- Independent 80x24 PTY checks confirmed demo painting, successful exits, restored
  terminal settings, and alternate-screen restoration for q and Ctrl+C.
OPEN ITEMS:
- none
REQUIRED FIXES:
- none
```

The P2 workspace guard was fixed after round 4 by hashing staged and unstaged
content in addition to `git status`.

Phase checks and PTY evidence: `docs/evidence/phase-0-checks.txt` and
`docs/evidence/terminal-restoration.txt`.
