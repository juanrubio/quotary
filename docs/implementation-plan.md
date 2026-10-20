# Quotary — implementation plan

Status: draft for execution. Source of truth remains [product spec](specs/product.md) and
[TUI spec](specs/tui.md); this plan sequences the work and names concrete deliverables.
Visual references live in [docs/mockups](mockups/README.md).

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
it. The root `README.md`, `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`, and
`LICENSE` are the only top-level documents; every other document lives under
`docs/`.

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
  `docs-check`, `check`, `run`, `clean`), `.golangci.yml`,
  `.github/workflows/ci.yml` running `make check`, `AGENTS.md`, a pointer-only
  `CLAUDE.md`, `CONTRIBUTING.md`, and `tapes/` for VHS demos.
- Make `docs-check` verify that every relative Markdown link resolves and that
  no moved specification or plan is duplicated at the repository root.
- Use Boardwalk's lint setup (`go tool golangci-lint` with errcheck, govet,
  ineffassign, staticcheck, unused, misspell, unconvert, unparam; gofumpt).
- **Documentation migration:** move `specs/product.md`, `specs/tui.md`, and
  `docs/implementation-plan.md` into `docs/` rather than leaving specifications at the
  repository root. Suggested layout:
  - `docs/specs/product.md` (from `specs/product.md`)
  - `docs/specs/tui.md` (from `specs/tui.md`)
  - `docs/implementation-plan.md` (this file)
  - `docs/decisions/NNNN-*.md` for ADRs (stack, CodexBar boundary, config
    format, theme palettes)
  - existing `docs/mockups/` and `docs/reference/` stay in place
  - `docs/evidence/` for live checks, coverage, classification, captures, and
    measurements
  - `docs/README.md` as the documentation index; only the root collaboration
    files stay outside `docs/`
- Update every cross-reference after the move, including `specs/product.md` ↔ `specs/tui.md`,
  the root README, and the mockup guide. Do not leave duplicate copies of moved
  documents and do not add redirect stubs unless an external link cannot be
  avoided.
- Create the package skeleton from SPEC §5 plus the house packages below.
- Add the [fixture policy](fixtures-policy.md) for `testdata/codexbar/`: synthetic
  or redacted fixtures only; no credentials, cookies, emails, project paths, or
  raw live payloads.
- Add the [tapes guide](tapes.md); VHS demos use demo data only.
- Add `internal/cli/` with Cobra root, `--version`, `--demo`, terminal check,
  and a `Version = "dev"` stamp suitable for release `-ldflags`.

Exit gate: `make check` and `make docs-check` pass; `go build ./...` succeeds; an
empty program can open and restore a Bubble Tea terminal cleanly; all
documentation is reachable from `docs/README.md` and no spec/plan remains at the
root.

### Phase 1 — Domain model, backend seam, and invariant tests

**Goal:** one backend-agnostic language for the whole application, plus the two
seams every later phase compiles against.

Implement the domain in `internal/domain/` and the seams later phases compile
against:

- `SourceDescriptor`: source ID, backend provider ID, display name, variant,
  service kind, capabilities, and tracked/pinned presentation inputs. Tracking
  and pin values are merged from preferences at the application layer; the
  adapter never supplies them.
- `SourceKey` canonical grammar: `<provider-id>[:<variant>][@<account-key>]`,
  matching SPEC §3 and the mockup fixture IDs.
  - `provider-id` is the exact backend catalog ID (lowercase
    `[a-z0-9][a-z0-9-]*`).
  - `variant` is optional and drawn from the set the catalog manifest declares
    for that provider (at minimum `configured`, `web`, and `api`); reject
    unvalidated variants.
  - `account-key` is a backend-provided opaque identifier or stable label,
    percent-encoded for `%`, `:`, `@`, and control characters. When the backend
    supplies no account identity, use the literal sentinel `@unknown`.
  - Never derive an account key from an email, plan, display label, response
    index, or the previous reading. A snapshot with `@unknown` never inherits a
    previously observed account label.
  - The canonical key is the unit of refresh coalescing, stale retention, and
    selection persistence.
- `UsageSnapshot`: source/account/plan, reported source, sample and receipt times,
  windows, balances, detail sections, quality, scoped error.
- `AllowanceWindow`: stable key, semantic label, optional used/remaining percent,
  optional used/limit + unit, duration, reset instant, group, quality.
- `Balance`, `DetailSection`, `HistorySnapshot`, `RefreshState`, and a closed
  error taxonomy: malformed JSON, empty stdout, process failure, timeout,
  authentication, unavailable quota, unsupported shape, and output cap. Each
  category carries a safe user message, optional scoped detail, and a mapping to
  the UX §6 state it renders as.
- Explicit optional/null semantics: distinguish missing/null, measured zero,
  exhausted, over-limit, stale, partial, and unsupported.
- Pure presentation helpers: remaining percentage derivation, over-limit label,
  gauge clamping, freshness/staleness, reset countdown, reset-due state, source
  state aggregation, sorting, and filter predicates.
- Define the `Backend` interface in `internal/backend` now, with
  `CatalogSnapshot`, `UsageRequest`, and `HistoryRequest` owned by that package
  and producing `domain` values. Demo (Phase 2) and live CodexBar (Phase 5)
  implement the same contract; no consumer may wait for the other.
- Add a `config.Store` interface plus an in-memory implementation and documented
  defaults (5-minute interval, 60-second minimum, Green theme, auto appearance).
  The TUI, scheduler, and demo depend on this seam from day one; the versioned
  XDG/JSON implementation lands in Phase 7.
- Add an import-boundary test: `domain` imports neither Bubble Tea nor Lip Gloss;
  `backend/codexbar` imports neither TUI nor theme packages; `tui` never imports
  adapter DTOs.

Tests:

- `usedPercent` omitted/negative/NaN/Inf → unavailable.
- `usedPercent = 0/100/100+` → 100% left / 0% exhausted / 0% plus over-limit.
- Numeric zero is not missing for credits and history.
- Staleness uses configured interval and last success; old sample time never
  becomes “updated now.”
- Source-key grammar round-trips, percent-escapes reserved characters, rejects
  unsupported variants, and distinguishes an account change from the same
  provider. Omitted identity stays `@unknown` and never merges with a known
  account during coalescing or retention.
- Every error category maps to exactly one UX §6 state and never exposes raw
  stderr or secrets.
- Default ordering is pinned then name with source-ID tie-break.

Exit gate: domain, `SourceKey`, error-taxonomy, config-seam, and import-boundary
tests cover SPEC §6 meaning/freshness rules and §10 correctness bullets that do
not require a process.

### Phase 2 — Deterministic demo backend and fixtures

**Goal:** the full visual product can run with zero backend, credentials, or
network.

Tasks:

- Implement `internal/backend/demo/` behind the `backend.Backend` interface from
  Phase 1, using the in-memory `config.Store`.
- Port the fictional fixtures from `docs/mockups/fixtures.js` into typed Go
  fixtures, keeping the demo clock fixed and all identities fictional.
- Add a parity check so the Go fixtures and `docs/mockups/fixtures.js` cannot
  drift on provider IDs, the fixed clock, scenario membership, or headline
  values. Either generate both from one source or fail a parity test that names
  the differing field.
- Build scenarios:
  - **Subscription desk:** 24 sources, priority families, varied quota shapes,
    plans, resets, pins, and two pages of cards.
  - **Attention:** low, exhausted, missing window, token-only history, expired
    login, retained stale data, partial/unsupported detail.
  - **Full catalog:** the 106 reference catalog IDs with the generic `gemini`
    entry replaced by `gemini:configured`, `gemini:web`, and `gemini:api`, producing
    108 selectable sources. Include non-subscription kinds honestly; this is not a
    count of 108
    subscriptions. Demo kinds and capabilities are illustrative and must not be
    copied into the live manifest.
  - **Empty/setup:** no tracked sources, no results, unknown account, missing
    backend.
- Provide a demo catalog index with availability, kind, variant, and tracking
  metadata so the Phase 4 picker has a truthful source of catalog concepts before
  the live manifest exists.
- Demo refresh simulates queued → fetching → current without real I/O.
- Demo preferences mutate memory only.
- Inject fake executable, filesystem, and preference seams into demo mode; do not
  rely on “never call” discipline in production code.
- Label every screen `DEMO · fictional data`; never read the user's config,
  credentials, sessions, or network.

Exit gate: `quotary --demo` runs offline, all scenarios switch deterministically,
and a test injects fake exec/filesystem/config seams and asserts zero real
subprocess, credential, network, or user-config access.

### Phase 3 — TUI shell, layouts, and core screens

**Goal:** reproduce the mockup hierarchy across required sizes.

Implement in `internal/tui/`:

- Persistent shell: brand/status, navigation/filter, content, footer; demo label.
- Adaptive card grid: 4/3/2 columns at ≥160/120/88; minimum 36 cells per card and
  two-cell gutter; compact at 80–87 columns or 24–31 rows; reduced single-column
  list below 80 columns or below 24 rows.
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

Adopt Trailmark's theme model:

- The four-theme extension is already recorded in UX §3 and SPEC §8. Keep those
  documents and `docs/decisions/0004-theme-palettes.md` in sync with the
  implementation; a later palette or chooser change updates all three.
- Provide four named themes: **Green** (default), **Blue**, **Amber**, and
  **Monokai**. Each has a dark palette and a light palette.
- Choose light or dark from the terminal background report
  (`tea.BackgroundColorMsg`), falling back to dark when the terminal does not
  report a background. Retain Quotary's optional manual appearance override
  (auto/dark/light) from the TUI specification.
- Green uses Quotary's UX §3 dark/light tokens as its anchor. Blue, Amber, and
  Monokai port Trailmark's structural colours (panel, text, muted, accent,
  selection, path, separator, raised) from
  `../trailmark/internal/tui/theme.go` (`darkPalette`, `lightPalette`, and
  `themePalette`) and map them onto Quotary's semantic tokens: Canvas, Surface,
  Border, BorderFocus, Text, Muted, Accent, Available, Low, Exhausted/Error, and
  Stale.
- Extend every palette with explicit Quotary state tokens instead of overloading
  Trailmark's `gold`, `warning`, or `path` fields: Available stays a healthy
  green-family cue, Low a warning hue, Error a danger hue, and Stale a distinct
  retained-data hue. Verify those choices against Quotary's Canvas/Surface rather
  than assuming Trailmark's field names carry the same semantics.
- Trailmark is MIT-licensed by the same copyright holder; record the source and
  licence in the theme ADR and keep the copied structural values attributed.
- Keep monochrome as an orthogonal rendering mode, not a fifth palette: honor
  `NO_COLOR`/`TERM=dumb`, provide an explicit monochrome setting, use `>` and
  border/weight cues for selection, and provide ASCII borders and gauges.
- Add a Settings → Theme chooser listing Green, Blue, Amber, and Monokai with the
  current choice marked; `↑↓/jk` selects, `Enter` saves, `Esc` cancels, and a save
  failure leaves the active theme unchanged while showing a concise error.
- Persist the theme name through the Phase 1 `config.Store`; normalize absent or
  unknown names to Green. Theme changes must not trigger a backend refresh or
  rescan.
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
- Tracking, bulk “Track all subscriptions,” pins, layout toggle, and settings are
  written through the `config.Store` seam (memory in demo, JSON in live mode).
- Picker displays catalog availability, tracking, kind, and last-read status as
  separate concepts, reading the demo catalog index until the Phase 7 live
  manifest replaces it.
- State transitions: queued, fetching with/without prior data, current, low,
  exhausted, sign-in, auth failure with retained data, offline/timeout stale,
  missing window, unavailable entitlement, partial history, backend incompatible.
  Render each error through the Phase 1 taxonomy rather than ad-hoc strings.
- Scope operations to the selected source; closing an error returns to the same
  position.
- Add golden/snapshot tests for rendered text grids with a forced color profile at
  160×48, 120×40, 100×32, 80×24, and 60×20. Cover all four themes at 120×40 in
  light and dark, all five sizes in Green light/dark, and monochrome at all five
  sizes. Add a theme × size smoke assertion for dimensions and overflow instead
  of forty full goldens.

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
  unavailable quota, unsupported shape, and output cap, mapping every result to
  the Phase 1 taxonomy and its UX §6 state. Never expose raw stderr as product
  copy; retain it only as redacted diagnostics.
- Reject an unvalidated `--source` variant before spawning the process.
- Context cancellation, owned process group on Linux, graceful termination,
  bounded kill/reap fallback; never kill unrelated processes by name.
- DTOs isolated under the adapter; normalization into `internal/domain` only.

Fixtures and tests:

- Captured/synthetic arrays for quota windows, credits, money, grouped/model
  windows, generic `usage.details`, source variants, multi-account, and history.
- Failure fixtures for every error category, oversized output, partial success,
  exit-zero-with-error, incompatible schema, Unicode/control text.
- Table tests for field optionality and normalization; no UI imports in adapter.
  The Phase 1 import-boundary test also covers the adapter, runner, and DTOs.
- Invalid/unsupported source-variant requests fail with the documented error
  category without invoking the backend.

Exit gate: adapter contract suite passes without a live backend; fixture families
cover all SPEC §10 contract bullets.

### Phase 6 — Refresh scheduler and application state

**Goal:** many sources refresh predictably without freezing the UI.

Implement `internal/refresh/`:

- Queue coalesced by canonical `SourceKey`/operation; max three backend jobs.
  Unknown-account work coalesces only with other unknown-account work for the same
  provider.
- Read the refresh interval and 60-second minimum from `config.Store`, defaulting
  from Phase 1; apply a settings change without restarting the scheduler.
- Prioritize visible/selected/newly selected sources without starving others.
- One bulk sweep at a time; manual refresh joins existing work.
- Stagger starts; track last attempt separately from last success.
- Results applied on the Bubble Tea event loop with request generations; drop
  superseded, untracked, or changed-context results.
- Retain last success per canonical source key with age and stale badge; clear
  on identity/source-mode change; auth failures do not present retained data as
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

Exit gate: the 108-source stress mode stays responsive while fetching; no
duplicate in-flight work; no late result can overwrite a changed context.

### Phase 7 — Catalog capabilities, variants, and preferences

**Goal:** broad live coverage without hardcoded provider screens.

Tasks:

- Implement `internal/catalog/` with a reviewed capability manifest: service
  kind, supported source variants, history support, safe metering, active-probe
  classification, and presentation hints.
- Generate the manifest skeleton from
  `docs/reference/codexbar-0.77.0-providers.json`, then classify each provider
  explicitly. Record reviewer, date, CodexBar version, and evidence for every
  entry in a review log under `docs/evidence/`.
- Apply a written rubric per provider: one of subscription, credits, metered API,
  mixed, non-usage, or unknown; declared source variants; history support; safe
  metering; and probe status. The baseline is 106 catalog IDs, or 108 selectable
  sources after the Gemini variant split.
- Default conservatively: unknown kind, capability, or probe status stays unknown
  and is neither bulk-trackable nor automatically polled. Unfamiliar providers
  remain visible through the picker.
- Classify an active/inference probe as any read that triggers billable inference,
  an account-specific test request, or another active provider-side operation
  rather than a passive usage read. Document the reason per provider. Exclude
  probes from bulk tracking and automatic polling; show them as outside the
  prototype until reviewed.
- The 24-source demo classification is illustrative only and must not be copied
  into the live manifest.
- Seed first-run tracking from enabled compatible CodexBar providers without
  rewriting CodexBar settings.
- Bulk tracking includes all classified subscription and mixed-quota sources
  across the catalog, independent of search text; API, credit, unknown, and probe
  sources are included only by explicit selection.
- Source variants: Gemini app (`gemini:web`), Gemini CLI (`gemini:api`),
  configured mode, and Antigravity. Variant keys are stable across refreshes;
  replacing a generic provider entry retires the generic key, clears its
  incompatible readings, and preserves every already-established variant key.
- Extend the Phase 1 `config.Store` seam with versioned JSON at the XDG
  path/fallback and atomic writes: theme (green/blue/amber/monokai), appearance
  override, layout, tracked sources, pins/order, interval (60 s minimum), privacy
  mode, backend/config paths. Unknown theme names normalize to green. Pass
  `CODEXBAR_CONFIG` only when configured.
- Define the migration policy: a newer/unknown schema version refuses writes,
  leaves the file untouched, and runs on defaults with a setup message; a known
  older version migrates atomically; a corrupt file is backed up and replaced by
  defaults. Never silently discard a readable old file.
- Keep snapshots in memory; diagnostics off by default and redacted when enabled.
- Setup/help copy covers backend installation, resource bundle, Linux credential
  limits, and the distinction between demo and live side effects.

Exit gate: all classified subscriptions are trackable in a stress fixture; bulk
tracking excludes probes, API, credit, and unknown sources; preferences
round-trip; corrupt, older, and newer config files follow the migration policy;
and every manifest entry has a review-log record.

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
- Capture actual Go TUI at 160×48, 120×40, 100×32, 80×24, and 60×20 in Green
  light/dark and monochrome, plus Blue, Amber, and Monokai at 120×40 in light and
  dark. Compare against the closest mockups and store under
  `docs/evidence/captures/` with the capture command and terminal profile.
- Complete the §7 traceability matrix: every mockup screen and every SPEC §10
  checkbox names a test, golden, capture, or evidence file, or is explicitly
  recorded as open with a reason.
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
internal/domain/             sources, source keys, snapshots, windows, balances,
                             history, freshness, errors, state aggregation,
                             sorting/filtering
internal/backend/            Backend interface plus CatalogSnapshot and request
                             types; produces domain values
internal/backend/codexbar/   executable resolution, runner, DTOs, normalization
internal/backend/demo/       deterministic scenarios and demo clock
internal/catalog/            capability manifest, classification rubric, source
                             variants, bulk-tracking eligibility
internal/refresh/            queue, scheduler, backoff, generations, cancellation
internal/live/               live Session wiring: backend + scheduler + TUI seam
internal/config/             config.Store seam, versioned JSON, XDG resolution,
                             atomic writes, migration policy
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
docs/evidence/               live checks, coverage, classification log,
                             measurements, captures
docs/README.md               documentation index
tapes/                       VHS demo scripts
AGENTS.md / CLAUDE.md        contributor front door / pointer
```

Dependency rule: `cli` wires config, backend, refresh, and TUI; `tui` depends on
`domain`, `config.Store`, and `theme`, never on `backend/codexbar` DTOs;
`backend/codexbar` depends on `domain` and produces normalized values; `refresh`
coordinates the backend interface and reads interval policy from `config.Store`;
`config` does not own runtime snapshots. Documentation lives under `docs/`; only
the root collaboration files are top-level documents.

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

`UsageRequest` carries provider ID, optional validated source variant, and the
canonical source key. `CatalogSnapshot`, `UsageRequest`, and `HistoryRequest`
are owned by `internal/backend`; `CatalogSnapshot` contains
`[]domain.SourceDescriptor`. The adapter returns all records; the caller/domain
decides how account boundaries are represented. Errors use the closed Phase 1
taxonomy with safe user messages and optional scoped detail, never raw stderr or
secrets.

## 6. Test strategy

| Layer | Tests |
| --- | --- |
| Domain | Table tests for optionality, percentage derivation, staleness, reset states, ordering, aggregation, `SourceKey` grammar/escaping/unknown-account coalescing, and error-taxonomy → UX state mapping |
| Adapter | Golden fixtures for every metric family and failure mode; argument construction; output caps; multi-record handling; rejected source variants |
| Runner | Cancellation, timeout, stderr drain, process-group cleanup, oversize rejection |
| Scheduler | Concurrency, deduplication by canonical source key, fairness, backoff, generation drops, manual priority, live interval changes |
| Config | Round-trip, XDG fallback, atomic write, corrupt/older/newer migration policy, theme normalization and persistence |
| Theme | Four named palettes in light/dark; WCAG AA contrast; unknown-name fallback; NO_COLOR/monochrome; style-cache invalidation |
| TUI | Golden text grids at required sizes/themes; key-map tests; search focus behavior; resize; truncation; sanitization |
| Safety | Injected demo seams make no subprocess/credential/network access; no secret-bearing diagnostics |
| Import boundaries | `domain` and adapters do not import TUI/theme; `tui` does not import adapter DTOs |
| Stress | 108 selectable sources, 24 tracked, rapid refresh/resize, race detector |
| Documentation | `make docs-check` resolves links and rejects duplicated root specs |

Manual evidence complements tests: live provider checks and actual terminal
captures are required for acceptance, not replaced by unit tests.

## 7. Traceability

A phase exit gate is satisfied only when its rows name a concrete test, golden,
capture, or evidence file. Update this matrix in the same PR that changes scope;
Phase 9 closes every row.

### 7.1 Reference screens

| Mockup screen(s) | Primary phase | Verification |
| --- | --- | --- |
| 01–03, 15 · overview/cards | P3–P4 | `TestGoldenMatrix`; `docs/evidence/captures/01-subscription-desk.ansi`, `02-overview.ansi`, `03-medium.ansi`, `15-more-subscriptions.ansi` |
| 04 · compact 80×24, 05 · small 60×20 | P3–P4 | `TestGoldenMatrix`, compact interaction tests; `04-compact.ansi`, `05-small-terminal.ansi` |
| 06 · attention | P2, P4 | `TestAttentionScenarioStates`, `TestDemoRefreshTransitionsVisible`; `06-attention.ansi` |
| 07 · details/allowances | P3 | `TestDetailsShowRawAllowanceAlongsidePercent`, `TestLongBalanceRemainsAccessible`; `07-google-allowances.ansi` |
| 08 · history | P3 | `TestHistoryMissingDayAndTokenOnly`, `TestHistoryPricingSemantics`; `08-muse-history.ansi` |
| 09 · picker/library | P3–P4, P7 | `TestPickerAvailabilityVisibleAtBreakpoints`, `TestManifestCoversReferenceCatalog`; `09-source-library.ansi` |
| 10 · daylight | P3 | `TestGoldenMatrix` green-light, `TestFourThemesLightDarkAndMonochrome`; `10-daylight.ansi` |
| 11 · monochrome | P3 | `TestGoldenMatrix` mono cases, `TestNoColorEnvironmentDisablesColor`; `11-monochrome.ansi` |
| 12 · first launch | P3–P4, P7 | `TestEmptyScenarioHasNoTrackedSources`, `TestLiveSeedingRequiresGenuineFirstRun`; `12-first-launch.ansi` |
| 13 · backend missing | P3, P5 | `TestDemoSetupScenarioShowsBackendMissing`, `TestExecutableResolution`; refreshed `13-backend-missing.ansi` |
| 14 · help | P3–P4 | `TestHelpPagesCoverEveryRenderedShortcut`; `14-keyboard-help.ansi` |

### 7.2 SPEC §10 acceptance criteria

| Criterion | Phase | Verification |
| --- | --- | --- |
| Demo communicates product with many fictional sources, no setup | P2, P9 | Demo scenario tests, offline safety test, captures |
| Catalog entries discoverable; tracking independent of enablement | P7 | Picker/catalog tests with disabled providers |
| Bulk tracking includes subscriptions and excludes probes | P7 | Manifest eligibility and bulk-selection tests |
| 24 tracked and 108-source mixed collection navigable | P2, P4, P6 | `TestSubscriptionScenarioHasLowReadings`, `TestFullCatalogScenarioTracksSubscriptions`; UI stress in P4/P6 |
| Selection preserved through refresh/filter/sort/resize | P4 | Interaction tests by source identity |
| Screens meet the TUI specification | P3–P4 | Goldens, captures, manual visual review |
| Green/Blue/Amber/Monokai in light/dark; monochrome keeps state cues | P3–P4 | Palette contrast tests, monochrome golden, stored captures |
| Ordinary operations keyboard-accessible and explained | P4 | Key-map tests, help coverage |
| Fixtures cover quota, credit, money, grouped windows, details, optional history | P5 | Contract fixture suite |
| Missing/null/zero/exhausted/over-limit/stale/partial distinct | P1, P5 | `TestRemainingPercent`, `TestOverLimit`, `TestNumericZeroIsMeasured`, `TestDeriveState`, `TestHistoryOptionalConcepts`; adapter fixtures in P5 |
| Partial results survive mixed success/nonzero exits | P5 | Failure/partial fixtures |
| Additive fields tolerated; malformed/oversized/incompatible visible | P5 | Schema-drift and cap fixtures |
| Sample/receipt distinct; countdowns local | P1, P6 | `TestFreshness`, `TestResetLabels`; scheduler tests in P6 |
| Account/source changes cannot inherit readings or late results | P1, P6 | `TestSourceKey*`, `TestSourceKeyCoalescing`, `TestSourceOrderingUsesStableTieBreak`, `TestImportBoundaries`; generation tests in P6 |
| Unknown names/Unicode/control sequences cannot break or inject | P3, P9 | `TestRenderedExternalTextIsSanitized`, `TestWrapTextUnicodeAndOrder`, `FuzzWrapTextNeverPanicsOrOverflows`, `FuzzCleanTextStripsControls`, `FuzzNormalizeUsageNeverPanics` |
| Demo makes no backend/credential/session access | P2 | `TestRuntimeReadsOnlyInMemoryFixtures`, `TestRuntimeUsesInjectedRunner`, `TestDemoProductionSourcesHaveNoExternalIO` |
| Cancellation/timeout/quit clean up and restore terminal | P5, P6, P9 | `TestExecRunnerTimeoutAndCancellation`, `Scheduler.Wait`/cancellation tests, `docs/evidence/terminal-restoration.txt` (q, Ctrl+C, SIGTERM) |
| Claude/Codex extra windows and supported history | P8 | Open/unavailable: `docs/evidence/live-validation.md` |
| Muse omitted windows and token-only history | P8 | Open/unavailable: `docs/evidence/live-validation.md` |
| Gemini app, Gemini CLI, Antigravity distinct and honest | P8 | Open/unavailable: `docs/evidence/live-validation.md` |
| Additional subscription providers via shared components | P8 | Open/unavailable: `docs/evidence/live-validation.md` |
| Account-specific checks recorded separately | P8 | Open/unavailable: `docs/evidence/live-validation.md` |

Open rows need a dated evidence record with backend version and reason, not an
unchecked box without explanation.

### 7.3 Phase 1 verification

| Phase 1 requirement | Verification |
| --- | --- |
| Canonical provider/variant/account parsing, escaping, malformed input | `TestSourceKeyRoundTrip`, `TestSourceKeyEscaping`, `TestSourceKeyRejectsInvalid`, `TestVariantValidation` |
| Known, unknown, and unspecified accounts stay distinct; controls round-trip | `TestSourceKeyAccountChangesAreDistinct`, `TestSourceKeyCoalescing`, `TestC1ControlAccountEscaping` |
| Remaining percentage, over-limit, gauge clamping, missing/non-finite | `TestRemainingPercent`, `TestOverLimit`, `TestGaugeClamps` |
| Measured zero versus unavailable credits, history, models, and cost | `TestNumericZeroIsMeasured`, `TestHistoryOptionalConcepts` |
| Sample/receipt separation, unknown sample time, staleness boundary | `TestFreshness`, `TestResetLabels` |
| State precedence for errors, retained data, quality, balance-only and unit allowances | `TestDeriveState`, `TestCountStatesOverlap`, `TestErrorCategoriesMapToUXStates` |
| Attention filter agrees with state aggregation | `TestAttentionFilterAgreesWithCounts` |
| Typed scoped errors survive the Backend error seam | `TestScopedErrorThroughBackendSeam` |
| Explicit state cannot hide error or window-quality precedence | `TestExplicitStateCannotHideErrorPrecedence`, `TestWindowQualityOverridesExplicitCurrent`, `TestExplicitMeasurementStateCannotHideWindowQuality` |
| Queued/fetching states survive without readings | `TestSchedulingStatesSurviveWithoutData` |
| Completeness precedes explicit measurement states | `TestCompletenessChecksBeforeExplicitState` |
| Mixed available/unavailable balances are partial | `TestMixedBalancesArePartial`, `TestExplicitStateCompletenessRegression` |
| Mixed metric families (windows, balances, details) are partial | `TestMixedMetricFamiliesArePartial` |
| Empty detail sections are not readings | `TestEmptyDetailsAreNotReadings` |
| Unsupported detail families stay disclosures, not backend incompatibility | `TestUnsupportedDetailFamiliesDoNotHideSupportedReadings` |
| Detail bars/points preserve optionality and reject invalid numerics | `TestDetailNumericOptionality` |
| Unavailable details alongside windows are partial | `TestUnavailableDetailsWithWindowsArePartial` |
| Known account `unknown` never collides with missing identity | `TestUnknownAccountSentinelDoesNotCollide` |
| Missing sibling windows become partial, not current | `TestMissingSiblingWindowMarksPartial` |
| Unknown capabilities stay conservative | `TestCapabilityUnknownIsConservative` |
| Stable pinned/name/key ordering independent of input order | `TestSourceOrderingUsesStableTieBreak`, `TestSortSnapshotsPinnedThenName` |
| Filter/search predicates | `TestFilter` |
| Config defaults, normalization, interval, slice ownership, rejected save | `internal/config/store_test.go` |
| Domain/adapter/TUI import boundaries | `TestImportBoundaries`, `TestImportBoundaryDetectsViolation` |
| Backend contract seam | `internal/backend/backend_test.go` |

### 7.4 Phase 2 verification

| Phase 2 requirement | Verification |
| --- | --- |
| Fixtures agree with the browser reference and name the differing field | `TestFixtureParityWithMockups`, `TestFixtureParityNamesTheDifferingField` |
| Deterministic backend for every scenario | `TestScenariosAreDeterministic` |
| Subscription desk has 24 tracked sources and low readings | `TestSubscriptionScenarioHasLowReadings` |
| Attention states are distinct and truthful | `TestAttentionScenarioStates` |
| Full catalog has 106 IDs and 108 selectable sources | `TestCatalogIsBroadAndClassified`, `TestFullCatalogScenarioTracksSubscriptions` |
| Empty/setup has no tracked sources | `TestEmptyScenarioHasNoTrackedSources` |
| Muse history is token-only with a real gap | `TestMuseHistoryIsTokenOnly` |
| History capabilities match readers | `TestHistoryCapabilitiesMatchReaders` |
| Demo is offline and does not read user state or execute processes | `TestRuntimeReadsOnlyInMemoryFixtures`, `TestRuntimeUsesInjectedRunner`, `TestScenarioSwitchPreservesInjectedFixtures`, `TestDemoDepsUseInMemoryFixtureFS`, `TestDemoProductionSourcesHaveNoExternalIO`, `TestOfflineImportCheckDetectsViolation`, `TestDemoStartupMakesNoExternalAccess` (strace + negative control); successful trace in `docs/evidence/phase-2-isolation.txt`. Sandboxes without ptrace skip this test; static import checks still run. |
| Unknown scenario is rejected | `TestUnknownScenarioRejected` |
| History requests validate and canonicalize scope | `TestHistoryScopeValidation` |
| Refresh completion is bound to its original source and scenario | `TestRefreshCompletesOriginalSourceAfterNavigation`, `TestScenarioSwitchInvalidatesPendingRefresh` |
| No-results preset preserves the collection and query | `TestNoResultsPresetPreservesCollection`, `TestDemoNoResultsScenarioPreservesQuery` |
| Attention transforms and every tracked window match the shared browser scenario definitions; sibling-window, numeric, null, and membership mutations fail | `TestAttentionScenarioMatchesSharedReference` |
| CLI constructs demo with in-memory fixture and runner seams | `TestDemoDepsUseInMemoryFixtureFS`, `TestRuntimeReadsOnlyInMemoryFixtures`, `TestRuntimeUsesInjectedRunner` |
| Demo startup uses backend data, scenarios, memory preferences, and refresh stages | `TestDemoModelUsesBackendAndSwitchesScenarios`, `TestDemoRefreshTransitionsVisible`, `TestRuntimeRefreshTransitions` |
| Demo preferences persist in-session and reset between sessions | `TestRuntimeTrackingAndPinningAreInMemory`, `TestRuntimeResetBetweenSessions` |
| Setup/missing backend is actionable | `TestSetupScenarioHasNoBackend`, `TestDemoSetupScenarioShowsBackendMissing` |
| Non-subscription credit readings are honest | `TestCreditSourceHasBalanceOnlyReading` |
| Unsupported details are disclosed | `TestAttentionIncludesUnsupportedDetailDisclosure` |
| Accounts use the unknown sentinel | `TestDemoAccountsUseUnknownSentinel` |
| Window keys are stable and unique | `TestWindowKeysAreUnique` |
| Invalid variants and conflicting scopes are rejected | `TestUsageRejectsUnvalidatedVariantsAndScopes`, `TestUsageRejectsConflictingScopes` |
| Returned history owns its values | `TestHistoryFixturesAreOwnedByBackend` |

### 7.5 Phase 3 verification

| Phase 3 requirement | Verification |
| --- | --- |
| Overview renders at 160×48, 120×40, 100×32, 80×24, 60×20 without overflow | `TestOverviewAtRequiredSizes`, `checkFrame` |
| Details, History, picker, Settings, Help render at each required size | `TestAllScreensRenderAtRequiredSizes` |
| Card grid uses 4/3/2 columns and enforces 36 cells | `TestCardColumnBreakpoints` |
| Green, Blue, Amber, Monokai in light/dark and monochrome | `TestFourThemesLightDarkAndMonochrome` |
| Every palette meets WCAG AA on Canvas and Surface | `internal/theme/theme_test.go` |
| Theme chooser persists and applies without refresh | `TestThemeChooserSavesAndApplies` |
| Search/filter context and clearing | `TestSearchAndFilterContext` |
| Muse history totals, gap, and token-only copy | `TestHistoryMissingDayAndTokenOnly` |
| Picker tracking and bulk subscription action | `TestPickerTrackingAndBulkAction` |
| External text sanitization and display-width truncation | `internal/tui/text_test.go` |
| Styling survives layout; themes differ; selection has a structural cue | `TestForcedColorPreservesStylingAndSelection` |
| Footer shortcuts and position remain visible; tiny terminals do not panic | `TestFooterAndTinyTerminal` |
| 120×24 uses compact rows; 60×20 uses reduced compact columns | `TestResponsiveHeightUsesCompactLayout` |
| History is bound to the selected source and reloads on navigation | `TestHistoryBoundToSelectedSource` |
| Missing sample time and privacy masking | `TestFreshnessUnknownAndPrivacyMasking` |
| 7/30-day history range control | `TestHistoryRangeControl` |
| Settings cancel and explicit monochrome | `TestSettingsCancelAndMonochromeSetting` |
| Refresh-all advances the whole tracked collection | `TestRefreshAllAdvancesTrackedCollection` |
| Focused search accepts `q` | `TestFocusedSearchAcceptsQ` |
| Pin labels come from preferences | `TestUnpinnedCardOmitsPinnedLabel` |
| Unit and balance-only readings render | `TestUnitAllowanceAndBalanceOnlyRendering` |
| Actual Go ANSI captures for all 15 scenes, with a distinct second page | `TestCapturePhase3`, `docs/evidence/captures/` |
| Rendered external text is sanitized | `TestRenderedExternalTextIsSanitized` |
| Narrow Details exposes metadata through scrolling | `TestDetailsMetadataAtNarrowWidths` |
| Priced/unpriced history semantics at full and compact sizes | `TestHistoryPricingSemantics` |
| Theme chooser keys, save, and cancel | `TestSettingsThemeChooserWithKeys` |
| NO_COLOR and Canvas background behavior | `TestNoColorEnvironmentDisablesColor`, `TestCanvasBackgroundIsPainted` |
| Card geometry aligns at display-cell widths | `TestCardGeometryIsAligned` |
| Allowance units are sanitized | `TestAllowanceUnitSanitization` |
| Scenario switch clears stale history | `TestScenarioChangeClearsHistory` |
| Settings rows are reachable and save | `TestSettingsTabReachesEveryControl` |
| History disclosures survive required sizes | `TestHistoryDisclosuresAtSmallSizes` |
| Details pane stays within its allocation | `TestDetailsWidePaneFitsAllocation` |
| Compact keeps units and selected context | `TestCompactPreservesUnitsAndContext` |
| Over-limit keeps 0% left and its qualifier | `TestOverLimitKeepsZeroLeftAndQualifier` |
| Search query remains visible at compact width | `TestSearchQueryVisibleAtCompactWidth` |
| Successful refresh outcome is shown in Details | `TestRefreshOutcomeVisibleInDetails` |
| History scrolling reaches the final daily values | `TestHistoryScrollReachesLastDailyValue` |
| History tab shares range and scrolling | `TestHistoryTabKeysAndScrolling` |
| Cost provenance and coverage at full/compact sizes | `TestHistoryCostProvenanceLabels` |
| Long detail values wrap and remain scrollable | `TestLongDetailValuesWrapAndRemainAccessible` |
| Compact footer counts match rendered sources | `TestCompactRenderedSourceCountMatchesFooter` |
| Unicode wrapping and label order | `TestWrapTextUnicodeAndOrder` |
| Long allowance labels remain accessible | `TestLongAllowanceLabelAccessible` |
| Compact preserves long units | `TestCompactPreservesLongUnitAt80` |
| Retained auth data is labelled | `TestAuthFailureLabelsRetainedData` |
| Generic history coverage metadata is accurate | `TestGenericHistoryCoverageMetadata` |
| Narrow library preserves kind, availability, shortcuts | `TestNarrowLibraryKeepsDistinctionsAndShortcuts` |
| Tiny history and unavailable transition are safe/honest | `TestTinyHistoryAndUnavailableTransition` |
| Privacy masks sensitive normalized detail values | `TestPrivacyHidesSensitiveDetailValues` |
| Details metadata remains fully scrollable | `TestDetailMetadataFullyScrollable` |
| Absolute resets use local timezone | `TestResetAbsoluteUsesLocalTimezone` |
| Explicit cards survives a short terminal | `TestExplicitCardsLayoutOnShortTerminal` |
| Details show raw allowance alongside percentage | `TestDetailsShowRawAllowanceAlongsidePercent` |
| Long balances remain accessible | `TestLongBalanceRemainsAccessible` |
| Reduced compact preserves units at 60×20 | `TestCompactReducedPreservesUnitsAt60` |
| Long details headings and section titles remain accessible | `TestLongDetailsHeadingAndSectionTitlesAccessible` |
| History charts/models stay scrollable at required sizes | `TestHistoryChartsAndModelsRemainScrollable` |
| Compact rows label retained auth data | `TestCompactRowsLabelRetainedAuthData` |
| Unpinned card test detects pin mutations | `TestUnpinnedCardOmitsPinnedLabel` |
| Settings success clears previous error | `TestSettingsSuccessClearsPreviousError` |
| Over-limit reported percentage remains visible | `TestOverLimitPreservesReportedPercentage` |
| Retained balance/detail data without timestamps is labelled | `TestRetainedBalanceWithoutTimestampIsLabelled` |
| Complete source names are accessible in Details | `TestCompleteSourceNameAccessibleInDetails` |
| Empty pinned filter is not first-launch copy | `TestEmptyPinnedFilterIsNotFirstLaunchCopy` |
| No-results search preserves/restores tracked collection | `TestNoResultsScenarioRestoresCollectionAfterClearing` |
| Retained card age stays visible with extra windows | `TestCardRetainedAgeVisibleWithExtraWindows` |
| Long history model names/totals remain accessible | `TestLongHistoryModelNamesAndTotalsStayAccessible` |
| Narrow history range hint and help are present | `TestNarrowHistoryRangeHintAndHelp` |
| Compact left column shows remaining quantity | `TestCompactLeftColumnShowsRemainingQuantity` |
| History partial flag is independently disclosed | `TestHistoryPartialFlagDisclosedIndependentlyOfProse` |
| Reduced compact keeps reset countdown | `TestReducedCompactKeepsResetCountdown` |
| History coverage/provenance wrap to final suffix | `TestHistoryCoverageAndProvenanceWrapToSuffix` |
| History display reuses cached snapshot until refresh | `TestHistoryDisplayReusesSnapshotUntilRefresh` |
| Invalid quantities are not rendered as measurements | `TestInvalidQuantitiesAreNotRenderedAsMeasurements` |
| Narrow details keeps retained warning alongside allowances | `TestNarrowDetailsShowsRetainedWarningAlongsideAllowances` |
| Non-auth failures retain the prior-data label | `TestMalformedFailureRetainsPriorDataLabel` |
| Reduced compact aligns primary resets | `TestReducedCompactPrimaryResetAlignment` |
| Narrow details shows stale state without an error | `TestNarrowDetailsShowsStaleStateWithoutError` |
| 30-day chart fits 60-column history | `TestThirtyDayChartFitsNarrowWidth` |
| Settings exposes and marks auto layout | `TestSettingsShowsAutoLayoutChoice` |
| Used-only compact values do not imply remaining | `TestCompactUsedWithoutLimitShowsRemainingUnavailable` |
| Cards preserve realistic quantity units | `TestCardPreservesRealisticQuantityUnit` |
| Overlapping refresh does not strand sources | `TestOverlappingRefreshDoesNotStrandQueuedSource` |
| History boundary navigation keeps cache | `TestHistoryBoundaryNavigationKeepsCachedSnapshot` |
| Compact large quantities keep units/columns | `TestCompactLargeQuantitiesKeepUnitsAndColumns` |
| Narrow chart keeps early range and gaps | `TestNarrowChartKeepsEarlyRangeAndGaps` |
| Card balances keep amounts with long labels | `TestCardBalanceKeepsAmountWithLongLabel` |
| Colored navigation uses display-cell width | `TestColoredNavigationUsesDisplayCellWidth` |
| Card refresh shows scheduling state and status | `TestCardRefreshShowsSchedulingStateAndStatus` |
| Balance freshness uses its own sample time | `TestBalanceFreshnessUsesItsOwnSampleTime` |
| Refresh footer keeps essential hints at 80 | `TestRefreshFooterKeepsEssentialHintsAt80` |
| Leaving settings via Help restores committed theme | `TestLeavingSettingsThroughHelpRestoresCommittedTheme` |
| Leaving settings via picker restores committed theme | `TestLeavingSettingsThroughPickerRestoresCommittedTheme` |
| Canvas/surface carry semantic text foreground | `TestCanvasAndSurfaceCarrySemanticForeground` |
| Fractional over-limit values are preserved | `TestFractionalOverLimitPreservedAtRequiredSizes` |
| Compact pagination accounts for current-page rows | `TestCompactPaginationAccountsForCurrentPageRows` |
| Mixed compact pagination matches footer and context | `TestCompactMixedPaginationMatchesFooterAndKeepsContext` |
| Picker availability remains visible at breakpoints | `TestPickerAvailabilityVisibleAtBreakpoints` |
| Normalized fixtures match the reference fields | `TestNormalizedFixtureParity`, `TestFixtureParityNamesTheDifferingField` |
| Base fixture and scenario headline values reject mutations | `TestNormalizedFixtureParity`, `TestAttentionScenarioMatchesSharedReference` |

### 7.6 Phase 4 verification

| Phase 4 requirement | Verification |
| --- | --- |
| UX §5 key map, focused printable input, Ctrl+C | `TestQuitKeys`, `TestFocusedSearchAcceptsQ`, `TestSettingsThemeChooserWithKeys` |
| Selection survives sort, resize, pagination; arrows use grid, j/k sequential, reverse tab | `TestSelectionIdentitySurvivesSortResizeAndPagination`, `TestGridNavigationAndReverseTab` |
| Search/filter/sort, catalog search, tracking, pins, layout, settings through Store | `TestSearchAndFilterContext`, `TestPickerTrackingAndBulkAction`, `TestRuntimeTrackingAndPinningAreInMemory`, `TestThemeChooserSavesAndApplies` |
| Picker separates availability/tracking/kind/last-read; paging covers every row at all sizes and clamps | `TestPickerAvailabilityVisibleAtBreakpoints`, `TestPickerTrackingAndBulkAction`, `TestPickerPageKeysDoNotMoveOverviewSelection`, `TestPickerFilteredPagingClampsAndActivates`, `TestPickerPagingCoversEveryEntryAt60`, `TestPickerPageKeysCoverEveryEntryAtAllSizes` |
| Sheets restore originating screen, picker selection, tab, and scroll on cancel/save | `TestSheetsRestoreOriginScreenAndPosition`, `TestNestedSheetsRestoreFullOriginChain`, `TestNestedPickerSelectionPreserved`, `TestSettingsSaveReturnsToOrigin` |
| Settings labels, interval, and path editors match handlers and save through Store | `TestSettingsVisibleMappingMatchesHandlers`, `TestSettingsIntervalAndPathEditors`, `TestSettingsSuccessClearsPreviousError` |
| Settings masks/sanitizes paths and keeps editor cursor visible | `TestSettingsPrivacyMasksAndSanitizesPaths`, `TestSettingsEditingMasksOtherPath`, `TestSettingsPathEditorKeepsCursorVisible` |
| Settings paging is scoped to the sheet | `TestSettingsPageKeysDoNotMoveOverview` |
| Rejected path saves restore committed styling | `TestRejectedPathSaveRestoresCommittedTheme` |
| Cancelled Settings clears sheet-scoped validation errors | `TestCancellingSettingsClearsValidationError` |
| Settings saves preserve nested picker tracking changes | `TestSettingsSavePreservesNestedPickerTracking` |
| Overlapping bulk/selected requests are queued/completed without dropping distinct sources | `TestSelectedRefreshDuringBulkIsNotStranded`, `TestBulkRequestDuringSelectedRefreshCompletesAllSources`, `TestThreeRapidPendingSourceRefreshesComplete` |
| History refresh dispatches independently during usage refresh | `TestHistoryRefreshDispatchesDuringUsageRefresh` |
| Scenario changes clear bulk/refresh state and outcomes | `TestScenarioSwitchClearsBulkRefreshState`, `TestScenarioSwitchClearsRefreshOutcomes` |
| Search restart/empty submission restores full collection | `TestRestartingSearchRestoresFullCollection` |
| Nested sheets preserve original picker selection | `TestNestedSheetsPreserveOriginalPickerSelection` |
| Layout toggle uses effective automatic presentation | `TestLayoutToggleUsesEffectiveAutoLayout` |
| Settings draft/focus suspends through child sheets and cancels on Esc | `TestSettingsDraftSuspendsThroughHelpAndCancels` |
| Help paging clamps and covers every rendered shortcut | `TestHelpScrollClampsToViewport`, `TestHelpPagesCoverEveryRenderedShortcut` |
| Cancelled Settings draft is never saved from parent origins | `TestCancelledSettingsDraftNotSavedAfterPickerOrigin`, `TestUnwindingSettingsViaPickerCancelsDraft` |
| Interval saves recompute states/counts/filters both directions | `TestIntervalSaveRecomputesStatesCountsAndFilter` |
| Settings recomputation preserves explicit normalized states | `TestSettingsRecomputePreservesExplicitNormalizedStates` |
| Settings saves preserve active refresh stages | `TestSettingsSaveDuringRefreshPreservesSchedulingStates` |
| Explicit auth-failure retention remains labelled through refresh | `TestExplicitAuthFailureRetainedDuringRefreshIsLabelled` |
| Bulk tracking is discoverable in Help and picker footer | `TestBulkTrackingDiscoverableInHelpAndPicker` |
| Explicit stale state keeps prior warning/age | `TestExplicitStaleStateKeepsPriorWarning` |
| Picker search/backspace preserves selected canonical source identity | `TestPickerSearchPreservesSelectedSourceIdentity`, `TestPickerBackspacePreservesSourceIdentity` |
| Selection changes reconcile history/error/scroll | `TestSelectionChangeReconcilesHistory`, `TestUntrackingSelectedSourceReconcilesHistory` |
| Returning from sheets reconciles history and reuses valid cache | `TestReturningFromPickerReconcilesHistoryAfterUntrack`, `TestUnchangedSourceHistoryCacheSurvivesSheetRoundTrip` |
| Nested picker preserves unsaved settings drafts | `TestNestedPickerPreservesUnsavedSettingsDraft` |
| Space pins only on Overview | `TestSpaceOnlyPinsOnOverview` |
| Bulk tracking reports failed and partial outcomes | `TestBulkTrackingReportsFailureAndPartialSuccess` |
| Help restores its originating picker context | `TestHelpReturnsToPickerContext` |
| Attention refresh preserves selected identity within search/filter scope | `TestAttentionRefreshKeepsSelectedIdentity`, `TestBulkRefreshKeepsAttentionSelection`, `TestRefreshSelectionPreservationRespectsSearch` |
| Retained measurements keep prior-data cue while refreshing | `TestRefreshingRetainedCardShowsPriorInMonochrome`, `TestStaleRefreshDetailsKeepPriorAndAge`, `TestRetainedBalanceDuringFetchingShowsPriorEverywhere`, `TestReceiptTimeFallbackContributesToRetainedDetection` |
| Picker refresh targets its focused source | `TestPickerRefreshUsesFocusedSource` |
| Help paging is scoped and origin scroll is preserved | `TestHelpPagingDoesNotMoveUnderlyingSource` |
| Details History tab scrolls the history pane | `TestDetailsHistoryTabScrollsHistory` |
| Picker-help-picker round trip preserves selection | `TestPickerHelpRoundTripPreservesSelection` |
| Uppercase E edits path fields | `TestUppercaseEEditsPaths` |
| Settings navigation hints and Help are accurate | `TestSettingsHintsAndHelpExplainNavigation` |
| Layout and theme mutations are transactional on save failure | `TestLayoutSaveFailureIsTransactional`, `TestSettingsCancelAndMonochromeSetting` |
| Uppercase R queues all usage sources from History | `TestHistoryUppercaseRRefreshesAllSources` |
| Focused search preserves printable spaces | `TestFocusedSearchPreservesPrintableSpaces` |
| Failed refresh reports failure and visible transitions | `TestFailedRefreshReportsFailureAndTransitions` |
| Space toggles picker tracking and overview pins | `TestSpaceTogglesPickerTrackingAndOverviewPin` |
| Tab/Shift+Tab leave standalone History for Details | `TestHistoryTabMovesToDetailsScreen` |
| Active sort and soonest-reset rule are explained | `TestSortExplanationVisible` |
| State transitions render through the closed error taxonomy | `TestAttentionScenarioStates`, `TestRefreshCompletesOriginalSourceAfterNavigation`, `TestMalformedFailureRetainsPriorDataLabel`, `TestInvalidQuantitiesAreNotRenderedAsMeasurements` |
| Scope operations to selected source and preserve return position | `TestHistoryBoundToSelectedSource`, `TestHistoryDisplayReusesSnapshotUntilRefresh`, `TestHistoryBoundaryNavigationKeepsCachedSnapshot` |
| Golden text grids with forced TrueColor at all five sizes | `TestGoldenMatrix` with `internal/tui/testdata/golden/` |
| Four themes at 120×40 light/dark; Green at every size; monochrome at every size | `TestGoldenMatrix`, `TestFourThemesLightDarkAndMonochrome` |
| Theme × size smoke matrix for dimensions/overflow | `TestThemeSizeSmokeMatrix`, `TestAllScreensRenderAtRequiredSizes`, `TestOverviewAtRequiredSizes`, `TestGoldenMatrix` |
| No color-only information; structural selection cue | `TestForcedColorPreservesStylingAndSelection`, `TestCanvasBackgroundIsPainted` |

### 7.7 Phase 5 verification

| Phase 5 requirement | Verification |
| --- | --- |
| Executable resolution order and explicit invalid path setup error | `TestExecutableResolution` |
| Version/catalog command shape; operation deadlines; output/stderr caps; complete bounded process-group/pipe cleanup | `TestVersionUsesDirectArgsAndDeadlineSeam`, `TestCatalogAndVersionRetainOutputOnNonzeroExit`, `TestOperationDeadlineConstants`, `TestAdapterAppliesOperationDeadlines`, `TestExecRunnerOutputCap`, `TestExecRunnerStderrCapIsEnforced`, `TestExecRunnerTimeoutAndCancellation`, `TestExecRunnerKillsResistantGroupAfterGrace`, `TestExecRunnerCleansOwnedDescendantAfterParentExit`, `TestExecRunnerCancellationCleansDescendantAfterLeaderExits`, `TestExecRunnerBoundsWaitForEscapedInheritedPipe` |
| Catalog preserves `enabled` and `defaultEnabled` separately with conservative unknown capabilities | `TestCatalogPreservesEnabledDefaultsAndConservativeUnknowns`, `TestCatalogKeepsEnabledAndDefaultEnabledSeparate`, `TestCatalogStructuredErrorIsPreserved`, `TestCatalogRejectsInvalidProviderGrammar` |
| Usage baseline (`extraRateWindows`, `providerCost`, chart objects, row progress), percentages, raw quantities, credits, reported/neutral labels, collision-safe keys, named windows, details | `TestUsageWindowsCreditsDetailsAndMultiAccount`, `TestBaselineUsageExtraWindowsProviderCostAndCharts`, `TestUsageZeroIsMeasuredAndMissingIsNil`, `TestSemanticWindowLabelsAndExtraWindows`, `TestReportedRateWindowLabelsAreHonored`, `TestWindowKeysAreStableAndUnique`, `TestWindowKeyGenerationAvoidsExplicitCollisions`, `TestWindowKeyReservationWorksInBothOrders`, `TestGroupedWindowKeysAreStableAcrossOrder`, `TestGroupedExplicitWindowKeysStableAcrossOrder`, `TestGroupedWindowKeyCollisionWithReservedExplicit`, `TestUnsupportedDetailShapeIsDisclosed`, `TestUnsupportedSubstructureDisclosedAlongsideSupportedRows`, `TestUnsupportedBarsObjectDisclosedAlongsideRows`, `TestUnsupportedRowProgressDisclosed`, `TestUnsupportedDetailContainersAndMembersDisclosed`, `TestChartPointsAreNormalized`, `TestSupportedChartObjectIsNotMarkedUnsupported`, `TestMonetaryUsageNormalizedWithBillingMeaning` |
| Object/array envelopes and nested envelopes normalize consistently | `TestUsageNestedArrayEnvelopeIsFlattened`, `TestObjectResultsEnvelopeUsesRecursiveUsageNormalization`, `TestFlattenedResultsInheritEnvelopeSourceAndSample`, `TestResultsEnvelopeIdentityDoesNotOverrideChildren`, `TestChildAliasSourceAndTimestampTakePrecedence`, `TestAccountAliasSourceAndTimestampPrecedence`, `TestNestedUsageChildIdentityAndLabelPrecedence`, `TestParentMeasurementsDoNotLeakIntoAccountsOrResults`, `TestParentNestedUsageCostDoesNotLeakIntoAccounts`, `TestParentDetailAliasesDoNotLeakIntoChildren`, `TestHistoryArrayEnvelopeEqualsObjectShape`, `TestHistoryArrayProcessesEveryEnvelopeAndRejectsErrors`, `TestArrayWrappedNestedHistoryEqualsObject` |
| Every account retained, nested identity/nested usage normalized, missing identity becomes `@unknown`, daily history not attributed to a selected account | `TestUsageExpandsEveryAccount`, `TestCoexistingBalanceCollectionsAreRetained`, `TestAccountExpansionNormalizesNestedUsage`, `TestNestedAccountUsageDoesNotInheritEnvelopeMeasurement`, `TestNestedIdentitySeparatesAccounts`, `TestAccountMembersOverrideEnvelopeIdentity`, `TestAccountIdentityAliasesDoNotOverrideChildren`, `TestTopLevelIDIsProviderNotAccount`, `TestTopLevelDisplayNameIsNotAccountLabel`, `TestReturnedProviderGrammarRejected`, `TestUsageMissingIdentityBecomesUnknownSentinel`, `TestHistoryDailyRecordsDoNotInheritRequestAccount` |
| Mixed success/error envelopes, collections, all-failed accounts, envelope errors, and measurements alongside errors retain valid data/scoped failures | `TestMixedResultsRetainSuccessAndScopedFailure`, `TestObjectResultsMixedWithErrorsRetainsSuccesses`, `TestStructuredErrorCollectionsAreRejected`, `TestStringErrorCollectionsAreRejected`, `TestUnknownErrorObjectCollectionIsUnknownFailure`, `TestAllFailedUsageAccountsRetainScopedSnapshots`, `TestEmptyAccountsPreserveEnvelopeError`, `TestEmptyAccountsInMixedArrayScopedFailure`, `TestUsageEnvelopeErrorRetainsAccountsAndWindows`, `TestObjectErrorWithExtraRateWindowsRetainsMeasurement`, `TestProviderCostWithErrorRetainsMonetaryDetails`, `TestObjectBalanceAndUsageDetailsWithErrorRetained`, `TestDeclaredNullWindowPreservedAsUnavailable`, `TestNestedNamedWindowAndUsageKnownFalse`, `TestInvalidExplicitPercentageIsUnavailableNotGauge`, `TestNullPercentageKeepsRawQuantities`, `TestNamedWindowArrayDoesNotAddUnsupportedFallback`, `TestInvalidNamedWindowMemberDisclosed`, `TestUnsupportedUsageContainersAndMalformedWindowsDisclosed`, `TestMalformedMetricCollectionsDisclosed`, `TestUnsupportedBalanceMapDisclosed`, `TestUnsupportedCreditScalarDisclosed`, `TestNonObjectAccountMemberIsDisclosed`, `TestUsageWindowAlongsideErrorRetainsMeasurement`, `TestNestedNullErrorDoesNotEraseEnvelopeError`, `TestSingleObjectErrorWithMeasurementRetainsReading`, `TestSingleObjectFailedAccountRetainsScope`, `TestNestedUsageWithEnvelopeErrorRetainsMeasurement`, `TestIncompatibleUsageSiblingIsDisclosed`, `TestNonObjectUsageArrayMemberIsDisclosed`, `TestHistorySiblingErrorRetainsValidRecords`, `TestHistoryEnvelopeErrorRetainsValidDays`, `TestNestedHistoryNullErrorDoesNotEraseOuterError`, `TestHistoryIncompatibleSiblingRetainsValidGroup`, `TestHistoryRejectsCrossProviderAggregation`, `TestHistoryIncompatibleTimezonesNotAggregated`, `TestHistoryDifferentReadersNotAggregated`, `TestUnreportedHistoryTimezoneStaysUnknown`, `TestFlatHistoryRecordsPreserveReaderCompatibility`, `TestHistoryRecordLevelSourceValidation`, `TestRecordLevelReaderWithoutEnvelopeMetadata`, `TestRecordLevelTimezoneWithoutEnvelopeMetadata`, `TestIncompatibleHistoryMembersAreDisclosed` |
| Invalid/unsupported provider-specific variants, malformed provider components, and inconsistent source keys rejected before spawning | `TestUsageRejectsInvalidVariantBeforeSpawning`, `TestProviderSpecificVariantAndConsistencyValidation`, `TestProviderComponentsValidatedBeforeSpawn`, `TestProviderGrammarRejectsUnderscoreDotAndLeadingHyphen`, `TestReturnedVariantValidatedAgainstCanonicalScope`, `TestReturnedVariantUsesProviderPolicy`, `TestReturnedSourceModeValidatedAgainstRequestedVariant`, `TestConfiguredModeAcceptsBackendSelectedSource`, `TestMismatchedVariantQuarantinesMeasurements`, `TestMonetaryUsageQuarantinedWithMismatchedVariant` |
| `--source` only for explicit validated variants | `TestUsagePassesOnlyExplicitValidatedSource` |
| Structured error on exit zero including unknown categories/objects; nonempty collections; redacted structured/text diagnostics | `TestStructuredErrorOnExitZeroAndFailureCategories`, `TestUnknownStructuredFailureIsNotCurrent`, `TestUnfamiliarStructuredErrorObjectIsUnknownFailure`, `TestStructuredErrorCollectionsAreRejected`, `TestStringErrorCollectionsAreRejected`, `TestUnknownErrorObjectCollectionIsUnknownFailure`, `TestMalformedEmptyUnsupportedAndCaps`, `TestExecRunnerProcessFailureRetainsDiagnostic`, `TestDiagnosticsAreBoundedAndRedacted`, `TestJSONDiagnosticSecretsAreRedacted`, `TestJSONAccessAndRefreshTokensAreRedacted`, `TestCamelCaseAndCookieDiagnosticsRedacted`, `TestDiagnosticRedactsArbitraryProjectPath`, `TestCookieAssignmentAndPathAssignmentRedacted`, `TestQuotedPasswordWithWhitespaceRedacted`, `TestQuotedSecretsWithColonAndTokenAliasesRedacted`, `TestUnkeywordedJWTIsRedacted`, `TestAuthorizationSessionAndIDTokenQuotedRedacted` |
| Valid rows/measurements survive nonzero process exit together with the failure; fatal timeout/cap errors are not masked; incompatible history siblings retain valid groups | `TestNonzeroExitRetainsValidRows`, `TestFatalRunnerErrorsAreNotMaskedByValidJSON`, `TestHistoryIncompatibleSiblingRetainsValidGroup`, `TestHistoryRejectsCrossProviderAggregation`, `TestHistoryIncompatibleTimezonesNotAggregated`, `TestHistoryDifferentReadersNotAggregated`, `TestUnreportedHistoryTimezoneStaysUnknown`, `TestFlatHistoryRecordsPreserveReaderCompatibility`, `TestHistoryRecordLevelSourceValidation`, `TestRecordLevelReaderWithoutEnvelopeMetadata`, `TestRecordLevelTimezoneWithoutEnvelopeMetadata`, `TestIncompatibleHistoryMembersAreDisclosed` |
| Complete JSON consumption | `TestTrailingJSONIsRejected` |
| Receipt time stamped locally; cache-age metadata becomes sample age without replacing known sample times | `TestReceiptTimestampUsesInjectedClock`, `TestPayloadReceivedAtCannotOverwriteLocalClock`, `TestCacheAgeBecomesSampleTime`, `TestChildCacheAgeOverridesParentSample` |
| History baseline (`daily`, nested `totals`, `modelBreakdowns`) and per-envelope models/cost/totals, currency compatibility/unknown currency/pricing coverage, token bounds/overflow, bucket-date validation, partial/null/reported incompleteness, error taxonomy | `TestHistoryParsesDaysModelsCostAndPartial`, `TestHistoryScopeDisclosedAlongsideBackendProvenance`, `TestBaselineHistoryContractNormalizes`, `TestHistoryRejectsInvalidRange`, `TestHistoryRepeatedModelsAccumulate`, `TestHistoryModelsAreNotDoubleCountedAcrossSummaryAndDetail`, `TestModelOverflowAndNullCompleteness`, `TestMissingModelFieldMarksHistoryPartial`, `TestUnsupportedHistoryModelStructuresDisclosed`, `TestReportedIncompleteCountPreserved`, `TestZeroIncompleteCountDoesNotMarkPartial`, `TestHistoryTotalsReconcileEnvelopeSummariesWithDailyDetail`, `TestIncompatibleHistoryCurrenciesAreNotSummed`, `TestMixedReportedAndEstimatedCostProvenanceIsNotAggregated`, `TestMonetaryOverflowBecomesUnavailablePartial`, `TestScalarHistoryCostOverflowDisclosed`, `TestMissingCurrencyIsNotInventedAndPricingCoverageDisclosed`, `TestUnknownCurrencyIsExcludedFromKnownTotal`, `TestUnknownCurrencyContributionMarksPartial`, `TestInvalidTokenCountsAreNotMeasured`, `TestInt64OverflowTokenRejected`, `TestAggregateTokenOverflowBecomesUnavailablePartial`, `TestInvalidHistoryBucketDateIsRejectedWithoutFabrication`, `TestEmptyDailyHistoryPreservesMeasuredZeroTotals`, `TestEmptyHistoryWithErrorRetainsMeasuredZeroSummary`, `TestHistoryStructuredAuthErrorPreserved`, `TestHistoryEnvelopeSummariesReconcile`, `TestNestedHistoryTotalsCostsAggregatePerEnvelope`, `TestHistoryCostNotDoubleCountedAcrossTopAndTotals`, `TestNestedScalarCostPreservesReportedProvenance`, `TestDuplicateDailyBucketsAcrossAccountsReconcile`, `TestNullSiblingPreservesKnownDailySubtotal`, `TestRejectedRecordsDoNotContributeModelsOrEnvelopeTotals`, `TestNullHistoryMeasurementsAreIncomplete`, `TestReportedIncompleteWithTokensStaysPartial` |
| `--provider-native-only` evaluation, restricted fallback, and fallback partial/failure retention | `TestHistoryParsesDaysModelsCostAndPartial`, `TestHistoryScopeDisclosedAlongsideBackendProvenance`, `TestBaselineHistoryContractNormalizes`, `TestHistoryFallsBackWhenNativeOnlyUnavailable`, `TestNativeOnlyFallbackDoesNotRetryFatalErrors`, `TestNativeOnlyFallbackRetainsValidPartialAndFailure`, `TestFallbackFatalErrorTakesPrecedenceOverPartialAuth`, `TestFallbackStructuredErrorWithNoBucketsPreserved` |
| Context cancellation and owned process group | `TestContextCancellationDoesNotSpawn`, `TestMidRunCancellationIsDistinct`, `TestDecodeErrorsCarryCanonicalScope`, `TestExecRunnerTimeoutAndCancellation`, `TestExecRunnerKillsResistantGroupAfterGrace`, `process_linux.go` |
| Adapter DTOs stay inside adapter; no UI imports | `TestImportBoundaries` |
| Synthetic/redacted fixtures cover contract families | `TestSyntheticFixturesAreValidJSON`, `testdata/codexbar/` |

Phase 6 wiring note: construct the CodexBar adapter with provider-native-only enabled for Codex/Claude (or explicitly record an equivalent scope decision) when live mode is wired.

### 7.8 Phase 6 verification

| Phase 6 requirement | Verification |
| --- | --- |
| Queue coalesces by canonical key/operation; unknown accounts coalesce only within provider/variant scope | `TestCoalescesUnknownAndDeduplicatesInflight` |
| Maximum three backend jobs; 108-source stress stays responsive | `TestConcurrencyCeilingAndStress` |
| Interval reads Store changes without restart; 60 s minimum | `TestIntervalReadsStoreWithoutRestart` |
| Visible/selected priority without starvation, with aging and live selected-source ordering | `TestPrioritySelectedBeforeBackground`, `TestAgingPreventsBackgroundStarvation`, `TestSetPriorityReordersQueuedWork`, `TestLivePrioritizeSelectedAheadOfBackground` |
| One bulk sweep at a time; manual requests join existing in-flight work | `TestBulkProgressAndSingleSweep`, `TestCoalescesUnknownAndDeduplicatesInflight` |
| Staggered starts; last attempt tracked separately from last success | `TestStaggeredStarts`, `TestRetainsLastSuccessAndBackoff` |
| Superseded generations coalesce/join; changed-context, untracked, and invalidated results are dropped without overwriting state | `TestChangedContextDropsLateResult`, `TestUntrackedResultDroppedWithoutState`, `TestInvalidateDropsQueuedAndInflightResults`, `TestSupersededGenerationCoalescesAndUsesLatest` |
| Retained last success survives transient failure; auth failures await explicit retry; backoff suppresses automatic retries | `TestRetainsLastSuccessAndBackoff`, `TestAuthFailureAwaitsExplicitRetry`, `TestBackoffSuppressesAutomaticButNotManualRetry` |
| Transient backoff/jitter and provider retry hints | `TestRetainsLastSuccessAndBackoff`, `TestRetryAfterHookSetsBackoff` |
| History singleflight, cache/TTL/force refresh, invalidation, concurrent loads, and partial history retained with error | `TestHistoryCacheAndForceRefresh`, `TestHistoryTTLExpiryUsesClock`, `TestHistorySingleflightAndInvalidation`, `TestInvalidateClearsRetainedStateAndHistory`, `TestLivePartialHistoryIsCached` |
| Cancellation stops owned work | `TestCancellationStopsWorkers` |
| Bulk progress exposes completed/total, one sweep id, dropped-member accounting, empty sweeps, and new keys during a sweep | `TestBulkProgressAndSingleSweep`, `TestBulkAccountsForDroppedMembersAndEmptySweep`, `TestBulkAddsNewKeysDuringActiveSweep` |
| Reliable result delivery and blocked consumers | `TestBlockedResultConsumerDoesNotDropResults` |
| Live scheduler-backed CLI/TUI session, tracked refresh, retained failure, history, tracking/pinning | `TestLiveSessionRefreshesTrackedSources`, `TestLiveSessionRetainsStaleOnFailure`, `TestLiveSessionTrackingPinningAndHistory`, `TestLiveSessionBackendSetupError` |
| Live returned `@unknown`/account keys replace bare skeletons; multi-account refresh, sequential account changes, automatic failure overlays one identity, and partial rows+errors are applied; finish-during-untrack cannot resurrect snapshots | `TestLiveReconcilesBareTrackedKeyWithReturnedUnknownAccount`, `TestLiveMultiAccountReplacesBareSkeletonAndRefreshesAccount`, `TestLiveSequentialAccountChangePrunesOldSnapshot`, `TestLiveAutomaticFailureOverlaysEstablishedUnknownSnapshot`, `TestLiveAppliesPartialSnapshotsWithError`, `TestUntrackDuringFinishDoesNotResurrectSnapshot` |
| Live failure paths notify the TUI and render non-Queued state; adapter-shaped error snapshots replace bare skeletons | `TestLiveFailureSignalsEvents`, `TestLiveSessionRetainsStaleOnFailure`, `TestLiveAdapterShapedFailureReplacesSkeleton` |
| Live manual bulk retries auth-awaiting members, records final fast-backend progress, and completes after untracking a queued member | `TestLiveManualBulkRetriesAuthAwaitingSource`, `TestLiveFastBulkRecordsFinalProgress`, `TestLiveBulkProgressCompletesAfterUntrackingQueuedMember` |
| Live history is non-blocking, fills from cache, and explicit TUI refresh forces within TTL | `TestLiveHistoryLoadDoesNotBlockEventLoop`, `TestLiveHistoryForceReloadsWithinTTL` |
| Live interval loop re-reads Store and schedules automatically without restart | `TestIntervalChangeSchedulesWithoutRestart` |
| Bubble Tea event-loop result notification refreshes the rendered model | `TestLiveUpdateMessageReloadsRenderedModel` |
| Live bulk sweep uses scheduler bulk progress | `TestBulkProgressAndSingleSweep`, `TestBulkAccountsForDroppedMembersAndEmptySweep`, `TestBulkAddsNewKeysDuringActiveSweep` |
| 108-source scheduler-backed TUI responsiveness | `TestSchedulerBackedTUIStaysResponsiveWith108Sources` |
| Backend version is sanitized before header rendering | `TestBackendVersionHeaderIsSanitized` |
| Live local tick advances ages/countdowns without backend traffic and re-checks visible history through the cache | `TestLiveLocalTickAdvancesAgeWithoutBackendTraffic` |
| Bulk sweeps joining executing work complete and allow later sweeps | `TestBulkJoinsExecutingTaskAndCompletes` |
| Sequential account changes prune old live snapshots | `TestLiveSequentialAccountChangePrunesOldSnapshot` |
| Live startup calls Backend.Version exactly once and surfaces it in the shell | `TestLiveCallsVersionExactlyOnce` |
| Scheduler workers terminate and Wait returns after cancellation | `TestSchedulerWaitReturnsAfterCancellation` |
| Partial snapshots returned with operation errors remain last-success readings; error-only snapshots do not replace prior success | `TestSchedulerRetainsPartialSnapshotsWithError`, `TestSchedulerErrorOnlySnapshotsRetainPriorSuccess`, `TestLiveAppliesPartialSnapshotsWithError` |
| Import boundaries cover refresh/live | `TestImportBoundaries` |
| Race-detector coverage, including repeated scheduler/live races | `make race`, `go test -race -count=30 ./internal/refresh`, `go test -race -count=20 ./internal/live` |

Live mode constructs the CodexBar adapter and scheduler through `internal/live`, delivers scheduler completions to the Bubble Tea event loop as `liveUpdateMsg`, uses one manual scheduler bulk sweep with completed/total footer status, re-reads the configured interval for automatic work, and closes owned workers when the TUI exits. History is dispatched in the background through the scheduler's 15-minute cache; explicit refresh forces, and the event notification reloads the rendered history. Countdown rendering remains local and generates no network traffic.

Phase 9 deferrals: CodexBar currently reports no retry timestamp, so the `RetryAfter` seam is dormant; catalog discovery still occurs during live session construction rather than behind the first shell paint; an interval settings change takes effect when the current timer next fires. These are recorded rather than claimed as verified behavior.

### 7.9 Phase 7 verification

| Phase 7 requirement | Verification |
| --- | --- |
| Manifest ID set and review-log annotations match the parsed 106-provider reference; Azure is an active probe with reason | `TestManifestCoversReferenceCatalog`, `TestReviewLogCoversEveryManifestEntry`, `TestReviewLogMatchesManifestAnnotations`, `docs/evidence/catalog-review.md` |
| Gemini generic key retired/migrated; configured/web/api variants stable; established variants preserved; Antigravity distinct | `TestApplyRetiresGeminiGenericAndSplitsVariants`, `TestGeminiVariantPresentationNamesDistinct`, `TestLiveFirstRunSeedsOnlyCompatibleEnabledSources`, `TestLiveMigratesGenericGeminiTrackedKey`, `TestLiveMigratesGenericGeminiPins` |
| Conservative unknown defaults; bulk and automatic polling exclude API, credits, non-usage, probes, and unknown; active probes are labelled outside the prototype | `TestBulkTrackingExcludesUnknownAPICreditsAndProbes`, `TestLiveAutomaticPollingSkipsUnknownAndProbe`, `TestLiveBulkExcludesActiveProbe`, `TestActiveProbeSourceShowsOutsidePrototype` |
| Every classified subscription/mixed source is bulk-trackable in the stress fixture | `TestAllClassifiedSubscriptionsAreBulkTrackableStress` |
| First-run tracking seeds only enabled compatible sources and a genuine first run; explicitly empty tracking stays empty | `TestSeedTrackedOnlyEnabledCompatible`, `TestLiveFirstRunSeedsOnlyCompatibleEnabledSources`, `TestLiveSeedingRequiresGenuineFirstRun` |
| Versioned JSON preferences round-trip including backend config path; XDG and explicit env paths | `TestJSONStoreRoundTrip`, `TestDefaultJSONStorePathHonorsEnv`, `TestDefaultJSONStorePathUsesXDG` |
| Corrupt config backs up and restores defaults; older migrates; newer refuses writes even before Load | `TestJSONStoreCorruptBacksUpAndRestoresDefaults`, `TestJSONStoreOlderVersionMigrates`, `TestJSONStoreNewerVersionRefusesWrites`, `TestJSONStoreSaveBeforeLoadRefusesNewerSchema` |
| Unknown theme normalizes to Green | `TestJSONStoreUnknownThemeNormalizesGreen` |
| Live CLI uses the JSON store, passes CODEXBAR_CONFIG only when configured, and surfaces store warnings | `TestLiveFirstRunSeedsOnlyCompatibleEnabledSources`, `TestJSONStoreRoundTrip`, `TestConfiguredEnvironmentReachesRunner`, `TestLiveStoreWarningPropagates`, `TestLiveConfigWarningRendersInShell`, `TestLiveCodexbarOptionsOnlyPassesConfiguredEnv` |
| Catalog/live import boundaries | `TestImportBoundaries` |

### 7.10 Phase 8 verification

| Phase 8 requirement | Verification |
| --- | --- |
| Claude/Codex/Muse/Gemini web/Gemini API/Antigravity/additional provider cases are recorded separately | `docs/evidence/live-validation.md` case table |
| Unavailable cases share document-level date/version and per-case command/observed state/native-only availability | `docs/evidence/live-validation.md` |
| Unavailable credentials are recorded as unavailable, not passed | Environment finding and conclusion in `docs/evidence/live-validation.md` |
| No credentials, cookies, emails, tokens, raw payloads, or project paths stored | Manual evidence review; fixture policy `docs/fixtures-policy.md` |
| Fixture/contract coverage is separated from live verification | Phase 5 §7.7 tables and Phase 8 case table |

Live exit gate: no credential-backed case existed in this environment, so the
live-success clause is not asserted. The documented-unavailable path is complete;
all live cases remain open/unavailable pending an installed CodexBar and real
credentials.

### 7.11 Phase 9 verification

| Phase 9 requirement | Verification |
| --- | --- |
| Demo and live shell appearance measured; live discovery is asynchronous | `TestFirstPaintBudget`, `TestLiveFirstPaintBudgetDespiteSlowDiscovery`, `BenchmarkFirstPaint`, `docs/evidence/performance.md` |
| Responsiveness during fetch/search/scroll/resize | `TestSchedulerBackedTUIStaysResponsiveWith108Sources`, `docs/evidence/performance.md` |
| Resize/fuzz for tiny terminals, labels, Unicode, controls, malformed JSON, oversized output | `TestFooterAndTinyTerminal`, `FuzzWrapTextNeverPanicsOrOverflows`, `FuzzCleanTextStripsControls`, `FuzzNormalizeUsageNeverPanics`, `TestMalformedEmptyUnsupportedAndCaps` |
| Terminal restoration on quit, Ctrl+C, SIGTERM, and quit during a hanging backend | `scripts/terminal-check.sh`, `docs/evidence/terminal-restoration.txt` |
| Required scene captures and full size×theme matrix stored under evidence; stored scenes verified in CI | `TestStoredSceneCapturesMatch`, `TestEvidenceCaptureMatrixMatchesGoldens`, `TestCapturePhase3`, `TestGoldenMatrix`, `docs/evidence/captures/README.md`, `docs/evidence/captures/matrix/` |
| Traceability matrices complete with live rows open | §7.1, §7.2, §7.10, §7.11 |
| Build/run/setup instructions | root `README.md` |
| Coverage record: catalog, classification, fixtures, live status | `docs/evidence/coverage.md` |
| Pinned Bubble Tea/Lip Gloss v2 and tool versions rechecked | `docs/evidence/versions.txt` |

Phase 9 accepted prototype limitations (recorded, not deferred to another phase):

- Live `Catalog`/`Version` discovery is asynchronous; the shell paints first and updates on a live event. Demo and live first-paint guards are recorded in `docs/evidence/performance.md`.
- The full size × theme matrix is stored under `docs/evidence/captures/matrix/` and verified byte-for-byte by `TestEvidenceCaptureMatrixMatchesGoldens`; the 15 scene captures remain under `docs/evidence/captures/` and are checked by `TestStoredSceneCapturesMatch`.
- Terminal restoration covers q, Ctrl+C, SIGTERM, and quitting while a backend usage command is hanging; backend timeout/cancellation cleanup is additionally covered by runner tests.
- Manifest safe-metering/presentation hints remain kind/display-name-derived; the review log records that rubric. Review-log variant/display-name assertions are enforced by `TestReviewLogMatchesManifestAnnotations`.
- Bulk tracking of Gemini is configured-only (`SeedTracked` agrees), preventing duplicate configured/web/api account identities. Interval changes take effect on the next timer fire. `RetryAfter` remains dormant because CodexBar 0.77.0 reports no retry timestamp; these are accepted prototype limitations, not live claims.
- All Phase 8 live provider cases remain open/unavailable in `docs/evidence/live-validation.md`; no live success is claimed.

## 8. Suggested commit/PR sequence

1. `chore: initialize Go module, Makefile, docs-check, CI, lint, and contributor docs`
2. `docs: move specifications and plan under docs, add index, and check links`
3. `feat(cli): add Cobra root, version stamp, terminal check, and demo flag`
4. `feat(domain): normalized model, source keys, and error taxonomy`
5. `feat(backend): add Backend interface and in-memory config.Store seam`
6. `test(domain): key grammar, optionality, freshness, and import boundaries`
7. `docs(spec): finalize named-theme UX/SPEC text and theme ADR against the implementation`
8. `feat(theme): add semantic tokens and Green/Blue/Amber/Monokai palettes`
9. `feat(demo): deterministic fictional scenarios, catalog index, and demo backend`
10. `feat(tui): shell, adaptive card grid, compact rows, theme chooser`
11. `feat(tui): details, history, picker, settings, help, failure states`
12. `feat(tui): full key map and state transitions`
13. `test(tui): golden renders, theme contrast, and responsive interaction coverage`
14. `feat(codexbar): executable resolution, runner, version and catalog`
15. `feat(codexbar): usage/history DTOs and normalization`
16. `feat(refresh): scheduler, generations, retention, backoff`
17. `feat(catalog): classification rubric, manifest, variants, bulk eligibility`
18. `feat(config): versioned JSON, XDG paths, and migration policy`
19. `test(codexbar): contract and failure fixture suite`
20. `docs(evidence): live priority validation, captures, and setup instructions`
21. `chore(release): measurements, coverage, traceability closeout`

Keep visual/demo work ahead of live integration. Adapter work can proceed in
parallel only after Phase 1 domain types and the `Backend`/`config.Store` seams
are stable; the theme ADR and spec amendment must land with the theme commit, not
in closeout.

## 9. Dependencies and critical path

```text
P0 tooling
  └─ P1 domain + Backend/config.Store seams
       ├─ P2 demo ── P3 TUI ── P4 interaction/goldens
       └─ P5 CodexBar adapter ── P6 scheduler ── P8 live validation
                                      └─ P7 catalog/config ── P8
P4 + P8 + P9 ── closeout
```

P3/P4 and P5/P6 can run in parallel once P1 lands, provided UI code imports only
the normalized domain, `config.Store`, and shared theme. P7 depends on adapter
catalog output and scheduler behavior. P8 depends on both the visual demo and
live adapter paths. The Phase 0 documentation move must land before any spec or
plan links change.

Indicative sizes, for sequencing rather than estimation: P0 S, P1 M, P2 M, P3 L,
P4 L, P5 L, P6 M, P7 L, P8 M plus credential availability, P9 M. P3/P4 and P5/P6
are the intended parallel tracks.

## 10. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Terminal width/Unicode bugs break layout | Central width helpers, sanitization, golden grids, fuzz/truncation tests |
| Provider payload drift | Fixtures pinned to 0.77.0; generic rendering; visible compatibility state; record version |
| First prototype collapses into four providers | Capability manifest + 108-source stress fixture + generic components from day one |
| Source-key ambiguity merges accounts or splits one account | Canonical grammar, percent-encoding, `@unknown` sentinel, and coalescing tests in Phase 1 |
| Demo fixtures drift from the browser reference | Parity check or single-source generation for IDs, clock, scenarios, and headline values |
| Config migration loses preferences | Explicit newer/older/corrupt policy with backup, atomic migration, and round-trip tests |
| Live credentials unavailable on Linux | Demo-first acceptance; honest setup states; record live gaps separately |
| Multi-account data misattributed | Preserve account boundaries; stable source/account keys; clear on context change |
| Refresh storm or frozen UI | Coalescing, 3-job ceiling, generations, bounded rendering, race tests |
| Stale data presented as current | Separate sample/receipt times; freshness rules in domain; stale/retained UI states |
| Child processes survive quit | Owned process group, cancellation, bounded kill/reap tests |
| Mockup copied blindly into live classifications | Demo classification explicitly illustrative; live manifest reviewed per provider |
| Named palettes fail contrast or collapse into decoration | Shared `internal/theme` tokens, WCAG AA tests for all four palettes, monochrome structural cues |
| Four-theme extension drifts from the specs | UX/SPEC and theme ADR are updated in the same change as any palette, chooser, or persistence change |
| Documentation links break during the move | `make docs-check` resolves relative links and rejects duplicated root specs in Phase 0 |
| Documentation drifts between root and `docs/` | Move specs/plans once in Phase 0, add `docs/README.md`, require cross-reference updates in the same change |
| Scope creep into billing/login/collectors | Keep deferred list explicit; backend interface leaves room without implementing it |

## 11. Definition of done

- Demo runs fully offline and is visually accepted at all required sizes.
- Green, Blue, Amber, and Monokai work in light and dark; monochrome works with
  no color; all palette contrast tests pass.
- All project documentation lives under `docs/`; the root collaboration files link
  the index; `make docs-check` passes.
- The 24-source and 108-selectable-source scenarios are navigable and responsive.
- Normalized domain, `SourceKey`, error-taxonomy, and backend contract tests pass.
- Scheduler, cancellation, persistence, config migration, and safety tests pass
  under `-race`.
- Priority live validation is recorded where credentials exist; unavailable cases
  remain explicitly open, not silently passed.
- Actual Go TUI captures and responsiveness measurements are stored.
- The §7 traceability matrix names a verification artifact for every mockup
  screen and SPEC §10 criterion, or records it as open with evidence.
- README/build/setup docs describe demo and live modes accurately.
- Every SPEC §10 checkbox is satisfied or listed as an open item with evidence.
