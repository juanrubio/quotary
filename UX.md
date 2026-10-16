# Quotary — visual and interaction specification

Status: proposed design. The text screens below are fictional wireframes, not
captures of an implemented application. Product and data contract: [SPEC.md](SPEC.md).

Rendered references are available in the [interactive mockups](docs/mockups/index.html).
The [implementer guide](docs/mockups/README.md) links 15 exported screens, exact
cell layouts, fictional fixtures, and reproduction instructions. These browser
references illustrate the design; actual Go TUI acceptance is still required.

## 1. Experience

Opening Quotary should feel like opening a thoughtfully arranged instrument
panel: calm colors, crisp gauges, readable spacing, and an immediate sense of
which services have capacity. The application should feel complete when a person
has many subscriptions, not like a four-item demo stretched across the screen.

The visual hierarchy is:

1. Service and reported plan.
2. Remaining allowance and reset time.
3. Whether the reading is current and usable.
4. Additional allowances, account identity, and history on inspection.

The overview must never lead with raw JSON, provider IDs, implementation errors,
or authentication token vocabulary. Details and source setup can expose technical
information when it helps resolve a problem.

## 2. Screen structure

Use a small, persistent shell: brand and status line, navigation/filter line,
scrollable content, and contextual keyboard hints. Show fictional-data labeling
throughout demo mode. Keep titles, selection indicators, and shortcuts in stable
positions. Reserve room for state changes so a spinner or error does not shift
every card.

The default view is **Overview**, with adaptive cards on larger terminals.
`l` switches between cards and compact rows. `Enter` opens the selected source;
Details has **Allowances** and, when supported, **History** tabs. Settings and the
source picker are sheets; they return to the prior selection and scroll position.

### Large overview — fictional example

This wireframe illustrates composition at roughly 120 columns. Final rendering
must calculate exact terminal-cell widths; the wireframe is not a layout engine.
Percentages below are examples only.

```text
 QUOTARY   Your AI subscriptions, at a glance                                      DEMO · fictional data
 24 sources  ·  22 current  ·  1 needs sign-in  ·  1 unavailable                         Refresh complete

 Overview                  All  /  Attention  /  Pinned                              / Search  p Sources

 ╭─ > Claude Code ──────────────────╮  ╭─ Codex ──────────────────────────╮  ╭─ Muse Code ──────────────────────╮
 │ Max                              │  │ Pro                              │  │ Power Usage                      │
 │ 5h     ███████░░░  72% left      │  │ 5h     ████████░░  84% left      │  │ 5h     ██████░░░░  63% left      │
 │        Resets in 2h 14m          │  │        Resets in 3h 08m          │  │        Resets in 1h 46m          │
 │ Weekly ████░░░░░░  38% left      │  │ Weekly ██████░░░░  61% left      │  │ Weekly █████░░░░░  47% left      │
 │        Resets in 3d 05h          │  │        Resets in 4d 11h          │  │        Resets in 2d 03h          │
 │ Updated 34s ago          +1 more │  │ Updated 31s ago          +2 more │  │ Updated 30s ago                  │
 ╰──────────────────────────────────╯  ╰──────────────────────────────────╯  ╰──────────────────────────────────╯

 ╭─ Google · Gemini app ────────────╮  ╭─ Google · Antigravity ───────────╮  ╭─ Cursor ─────────────────────────╮
 │ AI Pro                           │  │ AI Pro · Gemini models           │  │ Pro                              │
 │ 5h     █████████░  92% left      │  │ 5h     ██░░░░░░░░  18% left      │  │ Plan allowance                   │
 │        Resets in 4h 12m          │  │        Resets in 56m             │  │        ██████░░░░  56% left      │
 │ Weekly ███████░░░  69% left      │  │ Weekly ███░░░░░░░  26% left      │  │        Resets in 12d             │
 │        Resets in 5d 01h          │  │        Resets in 2d 09h          │  │                                  │
 │ Updated 28s ago                  │  │ Low · 27s ago · +2 more          │  │ Updated 22s ago                  │
 ╰──────────────────────────────────╯  ╰──────────────────────────────────╯  ╰──────────────────────────────────╯

 ╭─ GitHub Copilot ─────────────────╮  ╭─ Kimi Code ──────────────────────╮  ╭─ Windsurf ───────────────────────╮
 │ Pro                              │  │ Subscription                     │  │ Pro                              │
 │ Plan allowance                   │  │ Allowance                        │  │ No current reading               │
 │        ████████░░  81% left      │  │        ███░░░░░░░  33% left      │  │                                  │
 │        Resets in 15d             │  │        Reset not reported        │  │ Sign-in needed                   │
 │                                  │  │                                  │  │ Enter for setup guidance         │
 │ Updated 20s ago                  │  │ Updated 18s ago                  │  │ Last attempt 18s ago             │
 ╰──────────────────────────────────╯  ╰──────────────────────────────────╯  ╰──────────────────────────────────╯

 ↑↓←→ Navigate  Enter Details  r Refresh  R Refresh all  l Layout  ? Help  q Quit                  1–9 / 24
```

Card contents are examples of supported metric families, not promises that a
provider exposes these exact allowances or plan names. The implemented demo must
use explicit fixture semantics, and live labels come from reported data.

### Responsive behavior

| Terminal | Default composition |
| --- | --- |
| 160+ columns | Four card columns; optional source detail pane when enough space remains |
| 120–159 columns | Three card columns; Details opens a dedicated view |
| 88–119 columns | Two card columns |
| 80–87 columns | Compact rows; Details opens a dedicated view |
| Below 80 columns or below 24 rows | Reduced single-column list, short labels, scrollable Details, persistent essential shortcuts |

Card columns must be calculated from available space, with a minimum practical
card width of 36 terminal cells and a two-cell gutter. If borders and padding
would violate that minimum, use fewer columns. Height below 32 rows defaults to
compact layout regardless of width; users can still choose cards.

At **80×24**, all core tasks must work with no clipped controls. At **60×20**, a
compact fallback must remain navigable; charts and secondary metadata can be
hidden. Tiny terminals may show a concise resize hint while retaining quit/help.
Never wrap the whole application into accidental horizontal scrolling.

### Compact overview — fictional example

```text
 QUOTARY                                                   DEMO · fictional data
 24 sources · 22 current · 1 sign-in · 1 unavailable             All tracked

 Source                  Allowance       Left       Resets              State
 > Claude Code           5h              72%        2h 14m              Current
                         Weekly          38%        3d 05h
   Codex                 5h              84%        3h 08m              Current
   Muse Code             5h              63%        1h 46m              Current
   Google · Gemini app   5h              92%        4h 12m              Current
   Google · Antigravity  Gemini 5h       18%        56m                 Low
   Cursor                Plan            56%        12d                 Current
   GitHub Copilot        Plan            81%        15d                 Current
   Windsurf              —                —         —                   Sign-in

 Claude Code · Max · 3 allowances                          Updated 34s ago

 ↑↓ Navigate  Enter Details  / Search  p Sources  r Refresh  ? Help  q Quit
```

Compact mode shows one primary line per source and expands the selected source
to reveal another key allowance. Do not make a 106-source collection 318 rows
long by expanding every source. Include a position/count indicator when scrolling.

## 3. Visual system

Use the terminal's default monospace typeface. Weight, spacing, alignment, and
restrained color establish hierarchy. Do not require Nerd Fonts, emoji, ligatures,
or private-use glyphs. Provide ASCII borders and gauges when Unicode rendering is
unavailable. Measure display cells, not UTF-8 byte length or rune count.

| Token | Dark | Light | Purpose |
| --- | --- | --- | --- |
| Canvas | `#0B1020` | `#F8FAFC` | Overall background |
| Surface | `#151D2C` | `#FFFFFF` | Card and sheet surfaces |
| Border | `#344155` | `#CBD5E1` | Unselected boundaries |
| Text | `#E6EDF3` | `#0F172A` | Main information |
| Muted | `#A6B2C3` | `#475569` | Labels and supporting text |
| Accent | `#7DD3FC` | `#0369A1` | Focus, current tab, selected border |
| Available | `#86EFAC` | `#166534` | Healthy capacity |
| Low | `#FCD34D` | `#92400E` | Low capacity |
| Exhausted/error | `#FDA4AF` | `#BE123C` | Exhaustion or failure |
| Stale | `#C4B5FD` | `#6D28D9` | Retained readings needing refresh |

These are design tokens, to be visually checked in the target terminals. Respect
`NO_COLOR`, support a monochrome mode, and downsample color profiles. All states
need readable text; color is supplementary. Monochrome selection uses `>` and a
strong border or reverse video, not color alone.

Use rounded outer card corners where supported. One accent color marks focus.
Avoid every provider getting a competing brand-colored panel. Numeric columns
align to the right; quota labels align to the left. Avoid decorative animation.
A single restrained refresh indicator is sufficient for a busy collection.

Remaining-quota bars use the same direction everywhere: more filled means more
available. Suggested presentation thresholds are over 20% available, 1–20% low,
and 0% exhausted. These are UI hints, not provider policies or a recommendation
about which model to run. Missing percentages get a textual state and no filled
gauge. Credits/money use their own unit and do not inherit percentage thresholds.

Cards have a stable height within a row. Show up to two allowance windows and
`+N more` when needed. Extra model groups, rows, and balances remain accessible in
Details. On small cards, omit secondary identity information before sacrificing
the metric label, unit, or reset meaning.

## 4. Details and history

Details shows the selected service, reported plan, and account label with privacy
masking. The Allowances tab lists every supported returned window, grouped by
reported model/service scope. It includes reset countdowns and absolute times,
credit balances, additional detail rows, and sample freshness.

A small source section reports whether the reading came from a CLI, OAuth, web,
or cache and gives the most recent refresh condition. Backend details belong here
or in setup diagnostics, not at the top of every overview card.

History presents tokens prominently, with optional API-price estimates or
reported spend explicitly labeled. Use a compact daily histogram or sparkline
plus readable totals and a model table. A chart always has a date range, unit,
and text equivalent. Unavailable days are gaps, not zero-height measured bars.
Partial coverage and local-machine scope remain visible above the chart.

Unsupported history gets a concise explanation; it is not an empty chart labeled
zero. Muse's history has no dollar column. No display action should force a fresh
scan when a suitable in-memory history snapshot is already available.

## 5. Navigation

| Input | Action |
| --- | --- |
| Arrow keys | Navigate cards or rows; use the natural grid directions |
| `j` / `k` | Move down/up through sources |
| `Enter` | Open the selected source or activate the focused control |
| `Esc` | Close sheet, clear active search, or return from Details |
| `Tab` / `Shift+Tab` | Move between visible tabs or form/control groups |
| `/` | Search tracked sources; in the picker, search the full catalog |
| `f` | Choose All tracked, Attention, or Pinned filter |
| `s` | Choose pinned/name, name, attention, or soonest reported reset sort |
| `p` | Open the source picker |
| `Space` | Toggle tracking in the picker; toggle a pin in Overview |
| `r` | Refresh the selected source/current history view |
| `R` | Queue one refresh of all tracked usage sources |
| `l` | Toggle card and compact overview layouts |
| `,` | Settings: appearance, interval, privacy, and backend paths |
| `?` | Contextual help |
| `q` / `Ctrl+C` | Quit, cancel owned work, and restore the terminal |

While a search or form field has focus, printable keys insert text rather than
triggering global letter shortcuts. `Esc` exits editing; `Ctrl+C` always quits.
Mouse support is optional and never required for an action.

Default ordering is pinned sources followed by names, with a stable source-ID
tie-breaker. Explicit sorts may reorder after refresh but preserve the selected
source by identity and keep it visible. “Soonest reset” states which reported
window is used; unknown resets sort after known resets. Never automatically
switch the active provider because a different service looks less depleted.

The source picker distinguishes catalog availability, tracking, source kind, and
last-read status. “Track all subscriptions” is a direct selection action. It
does not imply the user is signed in. Gemini app and Gemini CLI are separate
selectable variants; Antigravity is its own source. Filters and search remain
useful even when the backend catalog exceeds 100 entries.

## 6. State design

| State | Overview treatment | Detail/next action |
| --- | --- | --- |
| Queued | Stable skeleton, “Waiting to refresh” | Keep source navigable |
| Fetching, no prior data | Skeleton, restrained loading marker | Show elapsed work without a fake percentage |
| Fetching with prior data | Keep gauges and their real timestamps | “Refreshing” indicator |
| Current | Metrics and sample age | All returned supported data |
| Low/exhausted | Metrics plus text badge and reported reset | No provider switching or alarm dialog |
| Sign-in needed | “Sign-in needed”; no invented gauge | Setup guidance for the selected source |
| Auth failure after a prior success | Retained values clearly labeled previous/stale | Refresh after fixing the underlying login |
| Offline/timeout with prior data | Stale badge plus last successful sample age | Retry action; old value remains labeled |
| Missing window | “Allowance not reported” when relevant | Keep sibling windows and plan data |
| No subscription or unavailable entitlement | Precise backend-supported explanation | No fake free plan or 100% capacity |
| Partial history | Coverage note and known subtotals | Explain incomplete/unpriced records |
| Backend missing/incompatible | Setup sheet; keep app usable | Configure executable or open demo |
| Unsupported extra details | Supported metrics remain visible | Disclose the undisplayed data family |
| Empty search | Retain query and filter context | Clear search/filter; no blank whole screen |

Error messages should be concise and actionable, scoped to the affected source.
Never display raw stderr, secret-bearing URLs, or a stack trace as product copy.
An unknown failure gets a safe generic description and optional redacted
diagnostics. Closing an error detail returns to the same overview position.

## 7. Scale and polish requirements

- Render only visible cards/rows and keep filtering independent of network work.
- A collection of 106 sources must not allocate a process or spinner per source
  simultaneously; scheduling and rendering remain bounded.
- Preserve focus when a source changes from skeleton to data or from current to
  unavailable. Background updates must not take keyboard focus.
- Respect light and dark terminal backgrounds; settings can override detection.
- A number changing width must not move a neighboring card. Align gauges and
  reserve room for percentages, state labels, and countdowns.
- Long names, multi-byte text, multiple model groups, and narrow windows must
  truncate intentionally, with full labels available in Details.
- Footer hints adapt to width and context. At minimum, navigation, Details,
  help, and quit remain discoverable at 80×24.
- The overview contains no combined AI percentage or invented bill total. Count
  sources by state and show each service's actual units.

## 8. Demo and visual review

Provide these switchable scenarios inside demo mode:

1. **Full subscription desk:** at least 24 fictional active sources, including
   the priority services and a range of quota shapes, plans, and reset periods.
   This is the default demonstration of the visual design.
2. **Attention:** depleted windows, missing subscription windows, a token-only
   history, an expired login, and retained stale values.
3. **Full catalog:** at least the 106 entries in the reference catalog, plus
   supported source variants. Use appropriate metric families for API/credit
   services; do not pretend all catalog entries are subscriptions.
4. **Empty/setup:** no tracked sources, missing backend, no search results, and
   unknown account identity.

All names, accounts, plan examples, and numbers in demo fixtures must be
fictional or clearly labeled illustrative. Dates advance relative to a fixed demo
clock so screenshots are repeatable. Demo refresh simulates state transitions
without accessing the real backend. Demo preference changes stay in memory.

Visual acceptance needs actual rendered TUI captures during implementation at
160×48, 120×40, 100×32, 80×24, and 60×20, with dark, light, and monochrome samples.
Review Overview, selected focus, Details, History, picker, and representative
failure/empty states. Capture fictional demo data only.

Acceptance requires readable hierarchy, consistent cell alignment, no accidental
overflow, preserved navigation, and a convincing large subscription collection.
Text fixtures and passing parser checks alone do not establish visual quality.

## 9. Open design refinements

The implementation may refine border weight, spacing, color tokens, chart
glyphs, and card density after reviewing rendered screens. Changes must preserve
metric meaning, keyboard access, broad coverage, and the small-terminal contract.
Those visual refinements do not require changing the backend architecture.
