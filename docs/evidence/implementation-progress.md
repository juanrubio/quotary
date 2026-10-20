# Implementation progress

External review: separate Codex CLI process using GPT-6 Astra. See
[the implementation prompt](../prompts/implement-all-phases.md) for the gate.

| Phase | Status | Commit | Checks | Review verdict | Open items |
| --- | --- | --- | --- | --- | --- |
| 0 — Repository and tooling | complete | e605f7b | make check, make race, docs-check, PTY restoration | PASS (4 rounds) | none |
| 1 — Domain and seams | complete | af9bc54 | make check, make race | PASS (11 rounds) | none |
| 2 — Demo backend | complete | 6a7d40f | make check, make race, PTY isolation | PASS (14 rounds) | none |
| 3 — TUI and themes | complete | 726c15e | make check, make race, 15 captures | PASS (24 rounds) | ptrace isolation skipped in review sandbox; Phase 2 evidence retained |
| 4 — Interaction and goldens | complete | fe8b5c9 | make check, make race, 22 goldens, 15 captures | PASS (20 rounds) | ptrace isolation skipped in review sandbox; Phase 2 evidence retained |
| 5 — CodexBar adapter | complete | d1e49af | make check, make race, adapter contract suite | PASS (Muse muse-wonder-full-1.0 substitution; Codex quota exhausted) | Live validation deferred to Phase 8; Phase 6 must wire NativeOnly |
| 6 — Refresh scheduler | complete | ce66a06 | make check, make race, repeated scheduler/live races | PASS WITH DOCUMENTED OPEN ITEMS (Muse substitution; 9 rounds) | Phase 9 deferrals: catalog before first paint, interval timer reset, dormant RetryAfter |
| 7 — Catalog and preferences | not started | — | — | — | — |
| 8 — Live validation | not started | — | — | — | — |
| 9 — Hardening and closeout | not started | — | — | — | — |
