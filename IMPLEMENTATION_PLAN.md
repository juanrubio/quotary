# Quotary — implementation plan

Status: draft for execution. Source of truth remains [SPEC.md](SPEC.md) and
[UX.md](UX.md); this plan sequences the work and names concrete deliverables.
Visual references live in [docs/mockups](docs/mockups/README.md).

## 1. Strategy

Build the product in the order the specs require:

1. Make the **demo TUI visually convincing first**, backed by fictional data.
2. Put a **strict normalized domain model** between the UI and every backend.
3. Add the **CodexBar process adapter** and prove it with captured fixtures.
4. Add **scheduling, persistence, broad catalog tracking, and source variants**.
5. Validate priority providers against live CodexBar where credentials exist.
6. Harden, measure, capture, and close out.

Do not start with provider-specific screens or live credential handling. Claude,
Codex, Muse, and Google are validation cases inside a generic presentation model,
not hardcoded branches in the UI.

Repository conventions follow the sibling Go CLIs `../gh-boardwalk` and
`../trailmark`: thin `cmd/<name>/` entrypoints, Cobra in `internal/cli/`, a
Bubble Tea/Lip Gloss v2 TUI in `internal/tui/`, normalized domain data in
`internal/domain/` (their `internal/model/`), backend adapters under
`internal/` (their `internal/adapter/` or `internal/githubx/`), a shared
`internal/theme/`, a root `Makefile`, `.golangci.yml`, `.github/workflows/ci.yml`,
`testdata/`, `tapes/`, and an `AGENTS.md` front door with `CLAUDE.md` pointing to
it. Every document other than the root `README.md` lives under `docs/`.

## 2. Workstreams

| Workstream | Owns |
| --- | --- |
| Domain | Normalized types, invariants, source keys, optionality, freshness |
| Demo | Deterministic fictional scenarios and demo clock |
| TUI | Shell, layouts, screens, key maps, rendering budget |
| Theme | Semantic tokens, Green/Blue/Amber/Monokai palettes, light/dark, monochrome |
| Docs | Root README/AGENTS, all specifications and plans under `docs/`, evidence, decisions |
| Backend | Executable resolution, process runner, DTOs, normalization |
| Catalog | Capability manifest, classification, source variants, bulk eligibility |
| Refresh | Queueing, concurrency, cancellation, backoff, generations |
| Config | Versioned preferences, atomic writes, privacy and path rules |
| Verification | Fixtures, unit/integration tests, visual captures, live evidence |

## 3. Milestones and phases

### Phase 0 — Repository, documentation, and tooling foundation

**Goal:** a buildable Go module laid out like the sibling projects, with all
project documentation under `docs/`.

Tasks:

- Initialize `go.mod` with the Go version pinned in `../gh-boardwalk` and
  `../trailmark` (currently 1.27.2), and a thin `cmd/quotary/main.go` that calls
  `internal/cli.Execute()`.
- Pin Cobra plus Bubble Tea v2 and Lip Gloss v2, using the same `charm.land/...`
  v2 import paths and `charmbracelet/x/ansi`, `colorprofile`, and `teatest`
  support libraries as the sibling projects. Record exact versions.
- Add the house root files: `Makefile` (`all`, `build`, `test`, `vet`, `lint`,
  `check`, `run`, `clean`), `.golangci.yml`, `.github/workflows/ci.yml` running
  `make check`, `AGENTS.md`, a pointer-only `CLAUDE.md`, `CONTRIBUTING.md`, and
  `tapes/` for VHS demos.
- Use Boardwalk's lint setup (`go tool golangci-lint` with errcheck, govet,
  ineffassign, staticcheck, unused, misspell, unconvert, unparam; gofumpt).
- **Documentation migration:** move `SPEC.md`, `UX.md`, and
  `IMPLEMENTATION_PLAN.md` into `docs/` rather than leaving specifications at the
  repository root. Suggested layout:
  - `docs/specs/product.md` (from `SPEC.md`)
  - `docs/specs/tui.md` (from `UX.md`)
  - `docs/implementation-plan.md` (this file)
  - `docs/decisions/NNNN-*.md` for ADRs (stack, CodexBar boundary, config format)
  - existing `docs/mockups/` and `docs/reference/` stay in place
  - `docs/evidence/` for live checks, coverage, captures, and measurements
  - `docs/README.md` as the documentation index
  - root `README.md` remains the only project document outside `docs/`
- Update every cross-reference after the move; add a docs index and link it from
  the root README. Do not leave duplicate copies of moved documents.
- Create the package skeleton from SPEC §5 plus the house packages below.
- Add `testdata/codexbar/` policy: synthetic or redacted fixtures only; no
  credentials, cookies, emails, project paths, or raw live payloads.
- Add `internal/cli/` with Cobra root, `--version`, `--demo`, terminal check,
  and a `Version = "dev"` stamp suitable for release `-ldflags`.

Exit gate: `make check` passes; `go build ./...` succeeds; an empty program can
open and restore a Bubble Tea terminal cleanly; all documentation is reachable
from `docs/README.md` and no spec/plan remains at the root.

### Phase 1 — Domain model and invariant tests

**Goal:** one backend-agnostic language for the whole application.

Implement in `internal/domain/`:

- `SourceDescriptor`: source ID, backend provider ID, display name, variant,
  service kind, capabilities, tracked/pinned state.
- `SourceKey` encoding: `provider:configured`, `provider:web`, `provider:api`, etc.
  Include account key where established; never infer account identity.
- `UsageSnapshot`: source/account/plan, reported source, sample and receipt times,
  windows, balances, detail sections, quality, scoped error.
- `AllowanceWindow`: stable key, semantic label, optional used/remaining percent,
  optional used/limit + unit, duration, reset instant, group, quality.
- `Balance`, `DetailSection`, `HistorySnapshot`, `RefreshState`, and error
  categories.
- Explicit optional/null semantics: distinguish missing/null, measured zero,
  exhausted, over-limit, stale, partial, and unsupported.
- Pure presentation helpers: remaining percentage derivation, over-limit label,
  gauge clamping, freshness/staleness, reset countdown, reset-due state, source
  state aggregation, sorting, and filter predicates.

Tests:

- `usedPercent` omitted/negative/NaN/Inf → unavailable.
- `usedPercent = 0/100/100+` → 100% left / 0% exhausted / 0% plus over-limit.
- Numeric zero is not missing for credits and history.
- Staleness uses configured interval and last success; old sample time never
  becomes “updated now.”
- Source keys and account changes are stable and distinct.
- Default ordering is pinned then name with source-ID tie-break.

Exit gate: domain unit tests cover SPEC §6 meaning/freshness rules and §10
correctness bullets that do not require a process.

### Phase 2 — Deterministic demo backend and fixtures

**Goal:** the full visual product can run with zero backend, credentials, or
network.

Tasks:

- Implement `internal/backend/demo/` behind the same backend interface as live
  CodexBar.
- Port the fictional fixtures from `docs/mockups/fixtures.js` into typed Go
  fixtures, keeping the demo clock fixed and all identities fictional.
- Build scenarios:
  - **Subscription desk:** 24 sources, priority families, varied quota shapes,
    plans, resets, pins, and two pages of cards.
  - **Attention:** low, exhausted, missing window, token-only history, expired
    login, retained stale data, partial/unsupported detail.
  - **Full catalog:** 106 public catalog IDs plus Gemini app/CLI variants, with
    non-subscription kinds represented honestly.
  - **Empty/setup:** no tracked sources, no results, unknown account, missing
    backend.
- Demo refresh simulates queued → fetching → current without real I/O.
- Demo preferences mutate memory only.
- Label every screen `DEMO · fictional data`; never read the user's config,
  credentials, sessions, or network.

Exit gate: `quotary --demo` runs offline, all scenarios switch deterministically,
and a test asserts no backend process or real filesystem/user-config access.

### Phase 3 — TUI shell, layouts, and core screens

**Goal:** reproduce the mockup hierarchy across required sizes.

Implement in `internal/tui/`:

- Persistent shell: brand/status, navigation/filter, content, footer; demo label.
- Adaptive card grid: 4/3/2 columns at ≥160/120/88; minimum 36 cells per card and
  two-cell gutter; compact at 80–87 columns or under 32 rows; single-column
  fallback below 80 columns.
- Card rendering: plan, up to two windows, ten-cell remaining gauge, right-aligned
  percentage, reset line, freshness, `+N more`, stable row heights.
- Compact rows: one line per source, selected-row expansion only, position count.
- Screens: Overview, Details/Allowances, History, source picker/library, Settings
  sheet, Help, first launch/empty, backend missing, empty search, failure states.
- Details: grouped windows, balances, detail rows, source metadata, account
  masking/reveal, absolute reset time/timezone, refresh outcome.
- History: 7/30-day selector, daily histogram/sparkline, textual equivalent,
  totals, model table, coverage/provenance/pricing labels, gaps for unavailable
  days, token-only Muse path.
- Shared theming through `internal/theme/`, never raw colors in TUI components.
- Display-width-aware truncation and sanitization of all external text, including
  Unicode and control sequences.
- Render only visible rows/cards; no per-source spinner allocation for 106 sources.

#### Theme system

Adopt Trailmark's theme model and palette choices:

- Provide four named themes: **Green** (default), **Blue**, **Amber**, and
  **Monokai**. Each has a dark palette and a light palette.
- Choose light or dark from the terminal background report
  (`tea.BackgroundColorMsg`), falling back to dark when the terminal does not
  report a background. Retain Quotary's optional manual appearance override
  (auto/dark/light) from UX.md.
- Port Trailmark's dark/light palettes into `internal/theme/` and map them onto
  Quotary's semantic tokens: Canvas, Surface, Border, BorderFocus, Text, Muted,
  Accent, Available, Low, Exhausted/Error, and Stale. Green uses Quotary's
  existing dark/light tokens as its anchor; add equivalent Blue, Amber, and
  Monokai palettes without changing metric meaning.
- Source values are `../trailmark/internal/tui/theme.go` (`darkPalette`,
  `lightPalette`, and `themePalette`). Copy the structural colours (panel, text,
  muted, accent, selection, path, separator, raised), but extend each palette
  with explicit Quotary state tokens instead of overloading Trailmark's `gold`,
  `warning`, or `path` fields: Available stays a healthy green-family cue, Low a
  warning hue, Error a danger hue, and Stale a distinct retained-data hue. Verify
  those choices against Quotary's Canvas/Surface rather than assuming Trailmark's
  field names carry the same semantics.
- Keep monochrome as an orthogonal rendering mode, not a fifth palette: honor
  `NO_COLOR`/`TERM=dumb`, provide an explicit monochrome setting, use `>` and
  border/weight cues for selection, and provide ASCII borders and gauges.
- Add a Settings → Theme chooser listing Green, Blue, Amber, and Monokai with the
  current choice marked; `↑↓/jk` selects, `Enter` saves, `Esc` cancels, and a save
  failure leaves the active theme unchanged while showing a concise error.
- Persist the theme name in Quotary config; normalize absent or unknown names to
  Green. Theme changes must not trigger a backend refresh or rescan.
- Cache resolved styles and invalidate on theme, light/dark, monochrome, or
  terminal-background changes. Downsample color profiles; never rely on color as
  the only state cue.
- Add contrast tests equivalent to Trailmark's: every text/muted/accent/state
  token against its Canvas and Surface backgrounds meets WCAG AA (4.5:1) in every
  theme, in both light and dark variants; Monokai gets an explicit check.

Exit gate: all 15 reference screens are representable at their dimensions, the
UX requirements for 80×24 and 60×20 pass, and all four themes render correctly in
light/dark and monochrome with no color-only information.

### Phase 4 — Interaction, state machine, and visual calibration

**Goal:** the demo behaves like the intended app, not a static screen.

Tasks:

- Implement the key map from UX §5, including context-sensitive printable input
  while search/forms are focused; `Ctrl+C` always quits.
- Selection is by source identity; preserve it through refresh, filter, sort,
  resize, sheet open/close, and pagination where still in scope.
- Search/filter/sort over tracked sources; full-catalog search in the picker.
- Tracking, bulk “Track all subscriptions,” pins, layout toggle, settings.
- Picker displays catalog availability, tracking, kind, and last-read status as
  separate concepts.
- State transitions: queued, fetching with/without prior data, current, low,
  exhausted, sign-in, auth failure with retained data, offline/timeout stale,
  missing window, unavailable entitlement, partial history, backend incompatible.
- Scope operations to the selected source; closing an error returns to the same
  position.
- Add golden/snapshot tests for rendered text grids at 160×48, 120×40, 100×32,
  80×24, and 60×20, for all four themes in light/dark plus monochrome.

Exit gate: demo interactions match the mockup guide; golden renders show correct
alignment, no overflow, selection visibility across page boundaries, and stable
layout while values change width.

### Phase 5 — CodexBar backend boundary

**Goal:** real data enters only through a tested adapter.

Implement `internal/backend/codexbar/`:

- Executable resolution: `QUOTARY_CODEXBAR_PATH` → configured path → `codexbar` on
  `PATH`; explicit invalid path is a visible setup error. Resolve absolute path.
- Version detection via `--version` once at startup.
- Catalog via `config providers --format json`; preserve enabled/defaultEnabled.
- Usage via `usage --provider <id> --format json`, adding `--source` only for
  explicitly selected validated variants.
- History via `cost --provider <id> --format json --days <7|30>`; evaluate
  `--provider-native-only` for Codex/Claude and record scope honestly if absent.
- Process runner: direct args (no shell), concurrent stdout/stderr drain, output
  caps (4 MiB usage, 32 MiB history, 128 KiB diagnostics), deadlines (5/10 s
  version/catalog, 60 s usage, 120 s Antigravity, 90 s history).
- Accept top-level array and equivalent single object; never take only the first
  record from a multi-account response.
- Parse structured errors even on exit code 0; retain valid rows on nonzero exit.
- Distinguish malformed JSON, empty stdout, process failure, timeout, auth,
  unavailable quota, and unsupported shape.
- Context cancellation, owned process group on Linux, graceful termination,
  bounded kill/reap fallback; never kill unrelated processes by name.
- DTOs isolated under the adapter; normalization into `internal/domain` only.

Fixtures and tests:

- Captured/synthetic arrays for quota windows, credits, money, grouped/model
  windows, generic `usage.details`, source variants, multi-account, and history.
- Failure fixtures for every error category, oversized output, partial success,
  exit-zero-with-error, incompatible schema, Unicode/control text.
- Table tests for field optionality and normalization; no UI imports in adapter.

Exit gate: adapter contract suite passes without a live backend; fixture families
cover all SPEC §10 contract bullets.

### Phase 6 — Refresh scheduler and application state

**Goal:** many sources refresh predictably without freezing the UI.

Implement `internal/refresh/`:

- Queue coalesced by source/account/operation; max three backend jobs.
- Prioritize visible/selected/newly selected sources without starving others.
- One bulk sweep at a time; manual refresh joins existing work.
- Stagger starts; track last attempt separately from last success.
- Results applied on the Bubble Tea event loop with request generations; drop
  superseded, untracked, or changed-context results.
- Retain last success per source/account with age and stale badge; clear on
  identity/source-mode change; auth failures do not present retained data as
  current.
- Transient backoff with jitter to 30 minutes; honor backend retry time; auth /
  setup failures await explicit retry or config change.
- History loads on open, stays in memory, refreshes on request or after 15
  minutes while visible.
- Countdowns update locally without network traffic.
- Cancellation and quit stop owned child processes and restore the terminal.
- Bulk progress shows completed/total.

Tests: scheduler fairness, concurrency ceiling, deduplication, backoff, stale
handling, generation drops, cancellation, and race-detector runs.

Exit gate: 106-source stress mode stays responsive while fetching; no duplicate
in-flight work; no late result can overwrite a changed context.

### Phase 7 — Catalog capabilities, variants, and preferences

**Goal:** broad live coverage without hardcoded provider screens.

Tasks:

- Implement `internal/catalog/` with a small reviewed capability manifest:
  service kind, supported source variants, history support, safe metering, active
  probe classification, and presentation hints.
- Classify every subscription-like source in the 106-entry baseline; leave
  unknown fields unknown and unfamiliar providers visible through the picker.
- Exclude active/inference probes (e.g. Azure OpenAI-style probes) from bulk
  tracking and automatic polling; show them as outside the prototype until
  classified.
- Seed first-run tracking from enabled compatible CodexBar providers without
  rewriting CodexBar settings.
- Bulk tracking includes all classified subscription and mixed-quota sources
  across the catalog, independent of search text.
- Source variants: Gemini app (`gemini:web`), Gemini CLI (`gemini:api`),
  configured mode, Antigravity; replacing a generic provider entry with variants
  preserves stable keys and clears incompatible old readings.
- Preferences in `internal/config/`: versioned JSON at XDG path/fallback, atomic
  writes, theme (green/blue/amber/monokai), appearance override, layout, tracked
  sources, pins/order, interval (60 s minimum), privacy mode, backend/config
  paths. Unknown theme names normalize to green. Pass `CODEXBAR_CONFIG` only when
  configured.
- Keep snapshots in memory; diagnostics off by default and redacted when enabled.
- Setup/help copy covers backend installation, resource bundle, Linux credential
  limits, and the distinction between demo and live side effects.

Exit gate: all classified subscriptions are trackable in a stress fixture; bulk
tracking excludes probes; preferences round-trip and corrupt files fail safely.

### Phase 8 — Priority live validation

**Goal:** verify the adapter against real CodexBar behavior where credentials exist.

Validate, separately recording catalog coverage, fixture coverage, and live
verification:

- Claude Code: reported session/weekly/named windows; history scope.
- Codex: session/weekly/extra windows and credits; history scope.
- Muse Code: five-hour/weekly when present; omitted windows handled; token-only
  history with no invented dollars.
- Google Gemini app (`--source web`), Gemini CLI (`--source api`), and
  Antigravity: distinct entries, distinct readings/accounts, model groups.
- At least several additional subscription providers through generic components.
- Unavailable credentials are recorded as unavailable rather than “passed.”

For each validated case store in `docs/evidence/`: date, backend version, command,
fictional/redacted summary, observed states, and whether native-only history was
available. Never store credentials, cookies, emails, tokens, raw payloads, or
project paths.

Exit gate: live results match the CLI for the same source/account when
credentials exist; gaps and setup limits are documented honestly.

### Phase 9 — Hardening and prototype closeout

Tasks:

- Measure shell appearance (<100 ms target) and responsiveness during fetch,
  search, scroll, and resize; record methodology and results.
- Resize/fuzz tests for tiny terminals, long labels, Unicode, control sequences,
  malformed JSON, and oversized output.
- Verify terminal restoration on normal quit, `Ctrl+C`, timeout, and signal paths.
- Capture actual Go TUI at 160×48, 120×40, 100×32, 80×24, and 60×20 for Green,
  Blue, Amber, and Monokai in both light and dark variants, plus monochrome;
  compare against mockups and store under `docs/evidence/captures/`.
- Write build/run/setup instructions, including demo-first onboarding and
  CodexBar configuration.
- Complete a coverage record: catalog IDs, classification, fixture coverage, and
  live status.
- Re-check Bubble Tea/Lip Gloss v2 APIs and pinned versions before release.

Exit gate: a new user can launch the demo, configure the backend, track services,
and understand failures; all unchecked SPEC §10 criteria are either satisfied or
explicitly recorded as open with evidence.

## 4. Package map

```text
cmd/quotary/                 thin entrypoint: call internal/cli.Execute()
internal/cli/                Cobra root, flags (--demo), --version, terminal check
internal/domain/             sources, snapshots, windows, balances, history,
                             freshness, state aggregation, sorting/filtering
internal/backend/            backend interface (Catalog/Usage/History, context)
internal/backend/codexbar/   executable resolution, runner, DTOs, normalization
internal/backend/demo/       deterministic scenarios and demo clock
internal/catalog/            capability manifest and source-variant rules
internal/refresh/            queue, scheduler, backoff, generations, cancellation
internal/config/             versioned preferences, XDG resolution, atomic writes
internal/theme/              semantic tokens + Green/Blue/Amber/Monokai,
                             light/dark palettes, monochrome, contrast rules
internal/tui/                app model, screens, components, layouts, key maps,
                             text sanitization and width helpers
testdata/codexbar/           synthetic/redacted contract fixtures
docs/specs/                  product and TUI specifications
docs/implementation-plan.md  this plan
docs/decisions/              ADRs for stack, backend, config, and theme choices
docs/mockups/                existing browser reference and captures
docs/reference/              pinned CodexBar catalog snapshot
docs/evidence/               live checks, coverage, measurements, captures
docs/README.md               documentation index
tapes/                       VHS demo scripts
AGENTS.md / CLAUDE.md        contributor front door / pointer
```

Dependency rule: `cli` wires config, backend, refresh, and TUI; `tui` depends on
`domain` and `theme`, never on `backend/codexbar` DTOs; `backend/codexbar`
depends on `domain` and produces normalized values; `refresh` coordinates the
backend interface; `config` does not own runtime snapshots. Documentation lives
under `docs/`; only the root `README.md` is a top-level document.

## 5. Core interfaces (shape, not final signatures)

```go
// internal/backend
type Backend interface {
    Version(ctx context.Context) (VersionInfo, error)
    Catalog(ctx context.Context) (CatalogSnapshot, error)
    Usage(ctx context.Context, req UsageRequest) ([]domain.UsageSnapshot, error)
    History(ctx context.Context, req HistoryRequest) (domain.HistorySnapshot, error)
}
```

`UsageRequest` carries provider ID, optional validated source variant, and source
key. The adapter returns all records; the caller/domain decides how account
boundaries are represented. Errors are typed categories with safe user messages
and optional scoped detail, never raw stderr or secrets.

## 6. Test strategy

| Layer | Tests |
| --- | --- |
| Domain | Table tests for optionality, percentage derivation, staleness, reset states, ordering, aggregation |
| Adapter | Golden fixtures for every metric family and failure mode; argument construction; output caps; multi-record handling |
| Runner | Cancellation, timeout, stderr drain, process-group cleanup, oversize rejection |
| Scheduler | Concurrency, deduplication, fairness, backoff, generation drops, manual priority |
| Config | Round-trip, XDG fallback, atomic write, corrupt/migrated file, theme normalization and persistence |
| Theme | Four named palettes in light/dark; WCAG AA contrast; unknown-name fallback; NO_COLOR/monochrome; style-cache invalidation |
| TUI | Golden text grids at required sizes/themes; key-map tests; search focus behavior; resize; truncation; sanitization |
| Safety | Demo mode makes no subprocess/credential/network access; no secret-bearing diagnostics |
| Stress | 106 sources, 24 tracked, rapid refresh/resize, race detector |

Manual evidence complements tests: live provider checks and actual terminal
captures are required for acceptance, not replaced by unit tests.

## 7. Suggested commit/PR sequence

1. `chore: initialize Go module, Makefile, CI, lint, and contributor docs`
2. `docs: move specifications and plan under docs and add index`
3. `feat(cli): add Cobra root, version stamp, terminal check, and demo flag`
4. `feat(domain): normalized source, usage, window, history, and freshness model`
5. `feat(theme): add semantic tokens and Green/Blue/Amber/Monokai palettes`
6. `feat(demo): deterministic fictional scenarios and demo backend`
7. `feat(tui): shell, adaptive card grid, compact rows, theme chooser`
8. `feat(tui): details, history, picker, settings, help, failure states`
9. `feat(tui): full key map and state transitions`
10. `test(tui): golden renders, theme contrast, and responsive interaction coverage`
11. `feat(codexbar): executable resolution, runner, version and catalog`
12. `feat(codexbar): usage/history DTOs and normalization`
13. `feat(refresh): scheduler, generations, retention, backoff`
14. `feat(catalog): capability manifest, variants, bulk tracking eligibility`
15. `feat(config): versioned preferences, theme persistence, backend paths`
16. `test(codexbar): contract and failure fixture suite`
17. `docs(evidence): live priority validation, captures, and setup instructions`
18. `chore(release): measurements, coverage record, closeout`

Keep visual/demo work ahead of live integration. Adapter work can proceed in
parallel only after Phase 1 domain types are stable.

## 8. Dependencies and critical path

```text
P0 tooling
  └─ P1 domain
       ├─ P2 demo ── P3 TUI ── P4 interaction/goldens
       └─ P5 CodexBar adapter ── P6 scheduler ── P8 live validation
                                      └─ P7 catalog/config ── P8
P4 + P8 + P9 ── closeout
```

P3/P4 and P5/P6 can run in parallel once P1 lands, provided UI code imports only
the normalized domain and shared theme. P7 depends on adapter catalog output and
scheduler behavior. P8 depends on both the visual demo and live adapter paths. The
Phase 0 documentation move must land before any spec or plan links change.

## 9. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Terminal width/Unicode bugs break layout | Central width helpers, sanitization, golden grids, fuzz/truncation tests |
| Provider payload drift | Fixtures pinned to 0.77.0; generic rendering; visible compatibility state; record version |
| First prototype collapses into four providers | Capability manifest + 106-source stress fixture + generic components from day one |
| Live credentials unavailable on Linux | Demo-first acceptance; honest setup states; record live gaps separately |
| Multi-account data misattributed | Preserve account boundaries; stable source/account keys; clear on context change |
| Refresh storm or frozen UI | Coalescing, 3-job ceiling, generations, bounded rendering, race tests |
| Stale data presented as current | Separate sample/receipt times; freshness rules in domain; stale/retained UI states |
| Child processes survive quit | Owned process group, cancellation, bounded kill/reap tests |
| Mockup copied blindly into live classifications | Demo classification explicitly illustrative; live manifest reviewed per provider |
| Named palettes fail contrast or collapse into decoration | Shared `internal/theme` tokens, WCAG AA tests for all four palettes, monochrome structural cues |
| Documentation drifts between root and `docs/` | Move specs/plans once in Phase 0, add `docs/README.md`, require cross-reference updates in the same change |
| Scope creep into billing/login/collectors | Keep deferred list explicit; backend interface leaves room without implementing it |

## 10. Definition of done

- Demo runs fully offline and is visually accepted at all required sizes.
- Green, Blue, Amber, and Monokai work in light and dark; monochrome works with
  no color; all palette contrast tests pass.
- All project documentation lives under `docs/`; root `README.md` links the index.
- The 24-source and 106-source scenarios are navigable and responsive.
- Normalized domain tests and CodexBar contract fixtures pass.
- Scheduler, cancellation, persistence, and safety tests pass under `-race`.
- Priority live validation is recorded where credentials exist; unavailable cases
  remain explicitly open, not silently passed.
- Actual Go TUI captures and responsiveness measurements are stored.
- README/build/setup docs describe demo and live modes accurately.
- Every SPEC §10 checkbox is satisfied or listed as an open item with evidence.
