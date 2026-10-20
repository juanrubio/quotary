# Goal: implement Quotary through all planned phases

> Reviewer substitution: if the mandated Codex GPT-6 Astra quota is exhausted,
> and the user explicitly authorizes a replacement, run the same external review
> prompt through `scripts/review-phase-muse.sh` using the Muse CLI model
> `muse-wonder-full-1.0`. Record the substitution and quota reason in the phase
> review evidence. Do not substitute silently or relax the review gate.


Use this file as the session objective. Read it fully, then execute it without
stopping at planning.

> Suggested invocation: `/goal Follow docs/prompts/implement-all-phases.md exactly.`

## Role

You are the implementation lead for Quotary. Build the Go application described
by `docs/implementation-plan.md`, `docs/specs/product.md`, and `docs/specs/tui.md`, phase by phase. The plan is
the execution sequence; the specs remain the source of truth. Do not replace
required behavior with a smaller interpretation, and do not skip a phase because
a later phase looks easier.

Work in `/home/joni/repos/quotary`. The repository currently contains documentation
and mockups only; the Go application has not been implemented.

## Required reading before Phase 0

Read these completely, in this order:

1. `docs/implementation-plan.md`
2. `docs/specs/product.md`
3. `docs/specs/tui.md`
4. `README.md`
5. `docs/mockups/README.md`, `docs/mockups/fixtures.js`, and the screen manifest
6. `docs/reference/codexbar-0.77.0-providers.json`
7. `docs/decisions/0004-theme-palettes.md`
8. The relevant sibling conventions in `../gh-boardwalk` and `../trailmark`

Re-read the applicable phase section, SPEC sections, and UX sections at the start
of every phase. Do not rely on memory or on this summary.

## Non-negotiable working rules

- Implement **one phase at a time**, in order: Phase 0 through Phase 9.
- A phase is not complete until **both** its plan exit gate passes and an
  external Codex review returns `PASS` (or `PASS WITH DOCUMENTED OPEN ITEMS`
  where the plan explicitly permits unavailable external evidence, such as live
  credentials in Phase 8).
- Never start Phase N+1 while Phase N is unreviewed, failing, or uncommitted.
- Do not weaken, skip, rename away, or `t.Skip` a required test to obtain a pass.
- Do not change an exit gate, acceptance criterion, or definition of done to match
  a weaker implementation. Fix the implementation.
- Keep demo mode fully offline. No subprocess, credential, session, network, or
  user-config access in demo mode.
- Keep adapter DTOs out of the TUI. Preserve the dependency rules in the plan.
- Preserve the canonical source-key grammar, optional/null distinctions, error
  taxonomy, freshness rules, and config migration policy from the plan.
- Update specifications, ADRs, mockup references, and cross-links in the same
  change whenever behavior or documented scope changes.
- Store only synthetic or redacted data in fixtures and evidence. Never store
  credentials, cookies, tokens, emails, raw provider payloads, or project paths.
- Run commands from the repository root. Commit only after the external review
  gate passes for that phase.
- If the same external-review blocker fails three consecutive fix/review rounds or
  goal turns, stop and report it with exact evidence instead of making unrelated
  changes.

## Mandatory external review gate

Every phase must be reviewed by a **separate external Codex CLI process** running
the **GPT-6 Astra** model. This is not a Pi subagent, not a self-review, and not a
summary written by you alone.

Use this command shape from the repository root. The custom review prompt must
run in `codex exec` mode because `codex exec review` rejects a prompt together
with `--uncommitted`/`--base`. The reviewer still runs as a separate external
Codex CLI process with GPT-6 Astra. It gets a writable Go cache and temporary
directory so it can independently run checks; the repository itself must remain
unedited:

```bash
mkdir -p /tmp/quotary-reviews
cd /home/joni/repos/quotary

mkdir -p /tmp/quotary-review-gocache
GOCACHE=/tmp/quotary-review-gocache GOTMPDIR=/tmp TMPDIR=/tmp \
codex exec \
  --model gpt-6-astra \
  --ephemeral \
  --sandbox workspace-write \
  -C /home/joni/repos/quotary \
  --output-last-message "/tmp/quotary-reviews/phase-<N>-iter-<M>.md" \
  - < /tmp/quotary-reviews/phase-<N>-review-prompt.md
```

Rules for this gate:

- `--model gpt-6-astra` is mandatory. If the model or Codex CLI is unavailable,
  stop and report; do not substitute another model or reviewer.
- Review the uncommitted phase diff before committing.
- Write the review prompt to `/tmp/quotary-reviews/phase-<N>-review-prompt.md`
  using the template below, with the phase number and exit gate filled in.
- Save the reviewer's final message outside the repository first, so the review
  artifact does not appear in the diff being reviewed.
- The reviewer must not edit tracked files. Record `git status --porcelain`
  before and after. If the reviewer changes the workspace, stop, inspect the
  changes, and discard only reviewer-created edits after saving evidence. Never
  discard pre-existing user work.
- Iterate at most **three review rounds** per phase. Fix every P0/P1 finding and
  re-run the review. If the third round still fails, stop and report the blocker.
- Reviewer nits marked P2 are not blockers unless they contradict the specs or
  hide a correctness/safety issue.
- After a passing review, copy the final passing message plus the iteration
  history into `docs/evidence/reviews/phase-<N>.md`, and commit that evidence
  with the phase.
- Never edit review evidence to claim a verdict the reviewer did not give.

### Reviewer prompt template

Write this to `/tmp/quotary-reviews/phase-<N>-review-prompt.md` before invoking
Codex. Replace `<N>` and the phase name. Preserve the required output structure.

```text
You are an independent external code reviewer for the Quotary repository. You are
running as a separate Codex CLI process using GPT-6 Astra. You did not write the
changes under review.

Review Phase <N> — <phase name> of `docs/implementation-plan.md`.

First read, in full:
- The implementation plan: `IMPLEMENTATION_PLAN.md` before the Phase 0 docs move,
  or `docs/implementation-plan.md` after it. Focus on Phase <N>, its exit gate,
  the traceability matrix, and the definition of done.
- The product and visual specs: `SPEC.md`/`UX.md` before the Phase 0 docs move,
  or `docs/specs/product.md`/`docs/specs/tui.md` after it. Read the sections
  referenced by Phase <N>.
- `README.md`, `docs/mockups/README.md`, and
  `docs/decisions/0004-theme-palettes.md` where relevant to this phase.

Then inspect all uncommitted changes: staged, unstaged, and untracked files. Use
`git diff --stat`, `git diff`, and read untracked files directly. Do not edit,
create, delete, stage, commit, or revert any repository file. Do not access
network resources, credentials, cookies, or live provider accounts. Do not run
commands that mutate the workspace.

Verify, with evidence:
1. Every task and exit-gate requirement for Phase <N> is actually implemented.
2. The implementation preserves SPEC and UX invariants, including optional/null
   distinctions, freshness, source identity, demo isolation, and error handling.
3. Tests genuinely exercise the required behavior and were not weakened, skipped,
   duplicated to inflate coverage, or made vacuous.
4. Golden/capture/fixture evidence is real and matches the required sizes,
   themes, scenarios, and data meanings.
5. The phase does not implement forbidden scope or leak adapter details into the
   TUI.
6. Build, vet, lint, docs-check, unit tests, and race tests required by the plan
   pass. Run safe read-only verification commands if useful; do not modify files.
7. Documentation and evidence are accurate, including any explicitly recorded
   open items. Unavailable live credentials may be recorded as open only where
   the plan permits it; they are not a pass.

Blocking findings are P0 (unsafe/incorrect, violates a spec invariant, or breaks
the build) and P1 (missing phase requirement, fake/weak evidence, weakened test,
or exit gate not met). P2 findings are non-blocking improvements.

Return exactly this structure:

VERDICT: PASS | PASS WITH DOCUMENTED OPEN ITEMS | CHANGES REQUIRED | BLOCKED
PHASE: <N> — <name>
SUMMARY: <2–5 sentences>
FINDINGS:
- [P0|P1|P2] <file>:<line> — <problem> — <why it matters> — <required fix>
EVIDENCE CHECKED:
- <commands run, files read, tests inspected, diff coverage>
OPEN ITEMS:
- <only items the plan permits; otherwise write "none">
REQUIRED FIXES:
- <ordered, concrete, testable actions; "none" only when verdict is PASS or
  PASS WITH DOCUMENTED OPEN ITEMS>

Do not return PASS unless every phase exit-gate item is evidenced. Do not return
PASS WITH DOCUMENTED OPEN ITEMS unless the plan explicitly allows that item to
remain open. Do not accept your own assumptions as evidence.
```

### Review iteration loop

For phase `<N>`:

1. Implement the phase.
2. Run all required checks locally.
3. Write the reviewer prompt template to `/tmp`.
4. Invoke `codex exec` with `gpt-6-astra`, the writable cache env, and the review prompt.
5. If verdict is `CHANGES REQUIRED` or `BLOCKED`, fix and repeat from step 2.
6. On `PASS` or an allowed `PASS WITH DOCUMENTED OPEN ITEMS`, save the review
   record, update progress, commit the phase, then continue.

Do not ask the same Codex session to review again after you changed the code.
Start a fresh `codex exec` invocation for each round.

## Phase checklist

Follow the full plan; this is the required order and the minimum review focus.

### Phase 0 — Repository, documentation, and tooling

- Initialize the Go module and house layout, pin dependencies, add Makefile,
  `.golangci.yml`, CI, `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`, and `tapes/`.
- Add `make docs-check` and wire it into `make check`/CI.
- Move `SPEC.md`, `UX.md`, and `IMPLEMENTATION_PLAN.md` under `docs/` per the plan,
  add `docs/README.md`, and update every cross-reference.
- Create the package skeleton and `testdata/codexbar/` policy.
- Add the Cobra root with `--version`, `--demo`, terminal check, and version stamp.
- Exit gate: checks pass, empty program restores the terminal, docs are reachable,
  no root spec/plan duplicates.

Review focus: layout matches sibling conventions; docs move is complete and
link-clean; no functionality is claimed before it exists; CI actually runs the
same checks a developer runs.

### Phase 1 — Domain model, backend seam, config seam, invariants

- Implement the normalized domain, `SourceKey` grammar and `@unknown` sentinel,
  optional/null semantics, freshness, state aggregation, sorting/filtering, and
  the closed error taxonomy mapped to UX states.
- Define `internal/backend.Backend` and its request/response ownership.
- Add `config.Store` plus an in-memory implementation and documented defaults.
- Add import-boundary and invariant tests.
- Exit gate: domain, key, error-taxonomy, config-seam, and import tests cover the
  applicable SPEC §6/§10 correctness rules.

Review focus: source identity cannot merge accounts; omission is not zero; no TUI
or adapter DTO imports; seams are usable by demo and live backends.

### Phase 2 — Deterministic demo backend and fixtures

- Implement the demo backend behind `backend.Backend` with the in-memory config
  store and injected exec/filesystem seams.
- Port fixtures, keep the fixed demo clock, add the parity check, and build the
  subscription desk, attention, full-catalog (106 IDs, 108 selectable sources),
  and empty/setup scenarios.
- Add the demo catalog index used by the picker.
- Exit gate: offline demo, deterministic scenarios, zero real subprocess/config/
  credential/network access.

Review focus: fictional identities only; demo classification is not copied into
live classification; picker metadata distinguishes availability, tracking, kind,
and last read; refresh simulation is not real I/O.

### Phase 3 — TUI shell, layouts, screens, theme system

- Implement the persistent shell, adaptive grid (including the reduced
  single-column fallback below 80 columns or below 24 rows), cards, compact rows,
  Overview, Details, History, picker, Settings, Help, first-launch, backend-missing,
  empty-search, and failure states.
- Implement `internal/theme` with Green/Blue/Amber/Monokai, light/dark selection,
  monochrome, style caching, and WCAG AA contrast tests.
- Keep the theme chooser and persistence behind `config.Store`.
- Exit gate: all 15 reference screens are representable, 80×24 and 60×20 pass,
  and all themes render correctly without color-only state cues.

Review focus: no raw colors in TUI components; semantic tokens only; no overflow
or unstable card heights; sanitization and display-width handling; theme changes
do not refresh the backend.

### Phase 4 — Interaction, state machine, and visual calibration

- Implement the full UX §5 key map, including focused input behavior and
  `Ctrl+C`; selection by source identity across refresh/filter/sort/resize/sheets.
- Add tracking, bulk tracking, pins, layout, settings, and full-catalog picker via
  the config store and demo catalog index.
- Implement all UX §6 transitions and route errors through the Phase 1 taxonomy.
- Add golden/snapshot coverage using the plan's representative theme/size matrix.
- Exit gate: interactions match the mockup guide, goldens show correct alignment,
  no overflow, visible selection, stable layout.

Review focus: tests exercise actual key handling and state transitions, not just
rendered strings; selection is not index-based; empty states retain context.

### Phase 5 — CodexBar backend boundary

- Implement executable resolution, version/catalog discovery, usage and history
  commands, direct-argument process runner, concurrent drain, output caps,
  deadlines, multi-record handling, structured errors, cancellation, and owned
  process groups.
- Keep DTOs in the adapter; normalize into the domain. Reject unvalidated source
  variants before spawning.
- Add fixtures for every metric family and failure category.
- Exit gate: adapter contract suite passes without a live backend and covers all
  SPEC §10 contract bullets.

Review focus: no shell invocation; no first-account truncation; partial results
survive nonzero exits; oversized output is a failure, not a truncated success;
raw stderr never becomes product copy.

### Phase 6 — Refresh scheduler and application state

- Implement canonical-key coalescing, the three-job ceiling, fairness/priority,
  staggered starts, generations, retention/clearing, backoff/jitter, history
  refresh rules, local countdowns, cancellation, terminal restoration, and bulk
  progress.
- Read interval/minimum from `config.Store` and apply changes without restart.
- Exit gate: 108-source stress stays responsive; no duplicate in-flight work; no
  late result overwrites a changed context.

Review focus: deduplication is by canonical source key; auth failures do not
present retained data as current; race detector and cancellation coverage.

### Phase 7 — Catalog capabilities, variants, and preferences

- Implement the reviewed catalog manifest generated from the pinned snapshot,
  the classification rubric and review log, conservative unknown defaults,
  active-probe exclusion, bulk-tracking eligibility, and variant retirement.
- Extend `config.Store` with versioned JSON, XDG paths, atomic writes, theme
  persistence, and the newer/older/corrupt migration policy.
- Exit gate: classified subscriptions are trackable, probes/API/credit/unknown
  are excluded from bulk, preferences round-trip, migrations are safe, and every
  manifest entry has a review record.

Review focus: no demo classifications copied into live; unknown is not silently
treated as subscription; probe criteria are documented; migration never silently
discards a readable file.

### Phase 8 — Priority live validation

- Validate Claude Code, Codex, Muse Code, Gemini app, Gemini CLI, Antigravity, and
  additional subscription providers where credentials exist.
- Store dated, redacted evidence with backend version, command, observed states,
  and native-only history availability.
- Record unavailable credentials as unavailable, not passed.
- Exit gate: live results match the CLI for the same source/account where possible;
  gaps and setup limits are documented honestly.

Review focus: evidence cannot be fabricated or inferred from fixtures; privacy
redaction is real; unavailable cases remain open.

### Phase 9 — Hardening and prototype closeout

- Measure shell appearance and responsiveness; add resize/fuzz coverage; verify
  terminal restoration on quit, `Ctrl+C`, timeout, and signal paths.
- Capture the required TUI sizes/themes/monochrome combinations; compare with
  mockups and store under `docs/evidence/captures/`.
- Complete build/run/setup docs, the coverage record, dependency/API re-check, and
  the traceability matrix.
- Exit gate: a new user can launch the demo, configure the backend, track
  services, and understand failures; every SPEC §10 criterion is satisfied or
  recorded as open with evidence.

Review focus: evidence files are real and reproducible; captures match the actual
Go TUI, not the browser mockups; the definition of done is not claimed while any
gate or traceability row is unresolved.

## Progress and evidence

Maintain `docs/evidence/implementation-progress.md` with one row per phase:

```text
Phase | Status | Commit | Checks | Review verdict | Open items
```

Maintain `docs/evidence/reviews/phase-<N>.md` containing:

- phase number and name
- date
- reviewer: external Codex CLI, model `gpt-6-astra`
- exact command shape and iteration count
- diff summary / commit under review
- every verdict and finding from each iteration
- fixes applied in response
- final verdict and open items

Only mark a phase `complete` when the review verdict permits it and the phase
commit exists. Keep failed or superseded review rounds; do not overwrite history.

## Completion

After Phase 9 has passed external review:

1. Run the full required verification suite one final time.
2. Confirm every traceability row names a real artifact or a permitted open item.
3. Confirm no uncommitted phase work or review evidence remains.
4. Call `goal_complete` with the exact active goal id and an evidence-based summary
   naming the final commit, checks, review records, and any permitted open items.

If the same external blocker recurs across at least three turns with concrete
evidence and cannot be resolved without user or external action, call
`goal_blocked` with that evidence. Do not call either goal tool for ordinary
incomplete work, uncertainty, or a single failed attempt.

## Forbidden shortcuts

- Starting a phase without the previous phase's passing external review.
- Using any reviewer other than an external `codex exec` process with `--model gpt-6-astra` and the review prompt.
- Treating this agent's own analysis, a local test run, or a plan restatement as
  the external review.
- Weakening tests, exit gates, acceptance criteria, or evidence to obtain `PASS`.
- Committing review evidence that was not produced by the external reviewer.
- Letting the reviewer modify the workspace or access secrets/live accounts.
- Claiming live validation from fixtures, mockups, or unverified assumptions.
- Skipping `make docs-check`, race tests, or the required capture/evidence steps.
