# Implementation progress

External review: separate Codex CLI process using GPT-6 Astra. See
[the implementation prompt](../prompts/implement-all-phases.md) for the gate.

| Phase | Status | Commit | Checks | Review verdict | Open items |
| --- | --- | --- | --- | --- | --- |
| 0 — Repository and tooling | complete | e605f7b | make check, make race, docs-check, PTY restoration | PASS (4 rounds) | none |
| 1 — Domain and seams | complete | af9bc54 | make check, make race | PASS (11 rounds) | none |
| 2 — Demo backend | complete | pending | make check, make race, PTY isolation | PASS (14 rounds) | none |
| 3 — TUI and themes | not started | — | — | — | — |
| 4 — Interaction and goldens | not started | — | — | — | — |
| 5 — CodexBar adapter | not started | — | — | — | — |
| 6 — Refresh scheduler | not started | — | — | — | — |
| 7 — Catalog and preferences | not started | — | — | — | — |
| 8 — Live validation | not started | — | — | — | — |
| 9 — Hardening and closeout | not started | — | — | — | — |
