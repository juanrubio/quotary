# Quotary visual references

Open [the interactive reference](index.html) in a browser. No server, build,
account, or network connection is required. Start with the **Subscription desk**
at **160×48**, then inspect **120×40** and **80×24**. Page Down shows the rest of
the collection. The controls above the terminal switch screens, sizes, and themes.

These are implementation references, not captures of a running Go application.
All usage, plans, entitlements, freshness, capabilities, and history are fictional.
The source library uses the real names and IDs in the pinned public
[CodexBar catalog](../reference/codexbar-0.77.0-providers.json); those entries do not
establish working integrations or active subscriptions on this machine.

The product and data requirements remain in [SPEC.md](../../SPEC.md). Interaction
and accessibility requirements remain in [UX.md](../../UX.md). Use these mockups
to make that design concrete. Where an illustrative reading differs from a
provider's actual output, preserve the real data's meaning and adapt the layout.

## Start here

![Quotary subscription desk with twelve of twenty-four fictional sources](screens/01-subscription-desk.png)

The main design decision is that a large subscription collection should feel
ordinary. Twelve cards fit on the large screen, nine on the standard screen,
and the collection remains easy to navigate in compact mode. This is one visual
system across sizes, rather than a set of unrelated design alternatives.

Do not reduce the prototype to four fixed providers. Claude Code, Codex, Muse,
and the Google sources receive explicit coverage within a wider collection.
The second page demonstrates the remaining twelve sources, with no new layout.

## Screen index

Each PNG has a matching `.txt` cell layout in [screens/](screens/manifest.json).
The manifest records scene, terminal dimensions, theme, and page for each export.

| Reference | Size | What the implementer should reproduce |
| --- | --- | --- |
| [01 · Subscription desk](screens/01-subscription-desk.png) | 160×48 | Four columns, twelve visible sources, stable focus, visible page count |
| [02 · Standard overview](screens/02-overview.png) | 120×40 | Three columns, nine cards, consistent quota and reset alignment |
| [03 · Medium terminal](screens/03-medium.png) | 100×32 | Two columns; the same information hierarchy survives resizing |
| [04 · Compact](screens/04-compact.png) | 80×24 | Dense rows, selected source's secondary allowance, persistent shortcuts |
| [05 · Small terminal](screens/05-small-terminal.png) | 60×20 | Essential fields and navigation remain usable |
| [06 · Attention](screens/06-attention.png) | 120×40 | Exhausted, stale, sign-in, missing, low, and unavailable states |
| [07 · Google allowances](screens/07-google-allowances.png) | 120×40 | Separate model groups, full reset timestamps, reading metadata |
| [08 · Muse history](screens/08-muse-history.png) | 120×40 | Token chart, missing day, known subtotal, no dollar column |
| [09 · Source library](screens/09-source-library.png) | 120×40 | Tracking, kinds, reading state, search, broad catalog coverage |
| [10 · Daylight](screens/10-daylight.png) | 120×40 | Light theme with equivalent hierarchy and contrast |
| [11 · Monochrome](screens/11-monochrome.png) | 80×24 | State text and `>` selection work without color |
| [12 · First launch](screens/12-first-launch.png) | 100×32 | Choose sources or explore demo |
| [13 · Backend missing](screens/13-backend-missing.png) | 100×32 | Actionable setup state without losing navigation |
| [14 · Keyboard help](screens/14-keyboard-help.png) | 100×32 | Discoverable navigation and shortcuts |
| [15 · More subscriptions](screens/15-more-subscriptions.png) | 160×48 | Sources 13–24 use the same generic cards |

## Translating this into the Go TUI

Build the terminal surface only. The web page title, screen selector, theme
selector, preview scaling, explanatory notes, and window chrome are review tools.
They are not product UI. The Go app should open directly into the overview.

Use the terminal's font; do not bundle a web font or require Nerd Fonts. These
exports use DejaVu Sans Mono at 15px with an 18px cell height. PNG pixels are
illustrative; character-cell geometry, hierarchy, and data semantics are the
reference. The Go renderer must use terminal display width and handle Unicode,
terminal color profiles, `NO_COLOR`, and the ASCII fallback described in UX.md.

| Piece | Cell geometry and behavior |
| --- | --- |
| Shell | Two-column outer margin; brand on row 1, status on row 3, separator on row 4; coordinates start at zero |
| Navigation | Row 6; search uses row 7 without shifting the cards |
| Card grid | Starts at row 8; two-column gutter, one-row gutter; 4/3/2 columns at 160/120/88 columns |
| Cards | At least 36 cells wide; 9 rows high normally, 11 at 48-row height; two windows maximum on the overview |
| Selection | Accent border, `>` prefix, bold title; preserve source identity through refresh and sorting |
| Allowance | Label, ten-cell remaining gauge, right-aligned percentage; reset sits immediately below |
| Extra windows | `+N more` in the bottom-right of the card; Enter opens all windows |
| Compact rows | One primary row per source; only the selected row expands; keep it inside the viewport |
| Footer | Separator at height minus 3, shortcuts and position at height minus 2 |
| Details | Dedicated view; metadata pane at 100+ columns, scrollable allowances below that |
| History | Chart and model table at 120×40; concise textual totals in short terminals |
| Library | Keep tracked selection, provider identity, source kind, and reading status separate |

The first twelve sources are deliberately pinned in a curated order in the
demo. This makes the priority families visible together. It is not a hardcoded
live-provider order. Live ordering follows UX.md; the source IDs remain stable.

Use the tokens in [styles.css](styles.css), which match UX.md:
canvas, surface, border, text, muted, accent, available (`good` in the reference
renderer), low, error, and stale. Card surfaces and gutters create most of the
hierarchy. Keep provider names in the same palette rather than assigning every
provider a competing brand color.

Treat empty space as intentional. Avoid filling single-window cards with fake
metrics, model suggestions, or aggregate totals just to match two-window cards.

## Data meaning that must survive implementation

- A fuller bar always means more remaining capacity. Zero has an exhausted label;
  an unknown allowance has no gauge. Stale values retain a stale label and age.
- `Current` counts fresh readings, including low or exhausted readings. The
  attention count can overlap with current; it includes low, exhausted, and
  failed/partial readings. Do not imply that these are disjoint totals.
- Do not sum percentages across providers or across Google sources. Gemini app,
  Gemini CLI, and Antigravity have separate identities and readings.
- The reference catalog has 106 provider IDs. Replacing the generic Gemini entry
  with its app and CLI variants produces 107 selectable sources. This is not a
  count of 107 subscriptions. The live catalog must be discovered dynamically.
- Bulk subscription tracking applies to eligible subscription sources across
  the whole catalog, independent of the search text. API, credit, unclassified,
  and active-probe sources are not silently included.
- The classification of these 24 demo sources is illustrative. **Do not copy it
  into a live capability manifest.** Validate the backend's capabilities and
  source behavior as specified in SPEC.md before assigning live classifications.
- The history clock is fixed at `2026-10-16T14:00:00Z`. The Muse example has
  4.74M known tokens across six recorded days, with 12 October unavailable.
  The missing day contributes no invented zero. Models A/B/C are fictional.
- Local token history is not measured subscription consumption. Muse has no
  dollar column. The Go implementation must preserve coverage and pricing scope.
- Unknown accounts stay unknown. Mask account identifiers by default. The demo
  never reads the user's actual provider configuration or credentials.

## What works in the interactive reference

Click the terminal to focus it. Arrow keys and `j`/`k` move selection; Enter opens
details; Esc returns. Tab switches Allowances/History where available. `/` enters
search, `f` cycles filters, `s` cycles sorting, Space pins, `l` changes layout,
`p` opens the library, and Page Up/Down moves through results. In the library,
Space toggles tracking and `a` tracks the demo subscriptions. Search accepts text
without firing global shortcuts. `?` opens help; `,` opens sample settings.

`r`/`R` simulate refresh while preserving selection. `q`/Ctrl+C pause this
reference; Enter restarts it. The browser tab remains open. All changes are
in-memory and reset when the page reloads or Reset reference is pressed.

The reference deliberately has no backend integration, login, real history scan,
saved preferences, real refresh scheduler, diagnostics pipeline, or Go terminal
event handling. Settings illustrate layout and theme choice only. History for
the three sample history-enabled sources reuses the fictional token series;
the dedicated exported history reference is Muse. Browser mouse hit regions
are conveniences, not a substitute for the keyboard contract.

## Files and reproducible exports

- [index.html](index.html): offline review page and screen controls.
- [styles.css](styles.css): review framing, terminal typography, and theme tokens.
- [mockup.js](mockup.js): character-grid renderer and reference interactions.
- [fixtures.js](fixtures.js): serializable fictional domain fixtures and the
  public catalog snapshot. These are not raw CodexBar response fixtures.
- [capture.py](capture.py): exports the terminal only and exercises core review
  interactions in an isolated headless browser.
- [screens/manifest.json](screens/manifest.json): exported image and text inventory.

To regenerate from the repository root:

```sh
uv run --with playwright==1.58.0 python docs/mockups/capture.py
```

The exporter uses `/usr/bin/google-chrome`; set `QUOTARY_CHROME` to another
installed Chromium executable if needed. It uses a temporary browser context,
does not attach to an existing browser profile, and closes its browser on exit.
`uv` may download the Python dependencies on first use. Viewing the HTML itself
requires none of these dependencies. Browser font substitution can change the
pixel dimensions; the terminal row/column count stays fixed.

Useful direct review URLs append `?scene=history&size=120x40&theme=dark` or
`?scene=overview&size=160x48&page=2` to `index.html`. Add `&capture=1` to hide the
review controls and window chrome.

## Acceptance and handoff

Build the Go demo with fictional domain data first. Match the wide and standard
overview, then the compact layouts, Google details, Muse history, and library.
Connect CodexBar through the adapter after the visual shell is convincing.
Keep the public catalog snapshot separate from curated backend-response fixtures.

The reference has been checked for all specified terminal grids, browser
overflow at desktop and narrow viewport widths, JavaScript errors, selection
and return navigation, search, tracking, and simulated refresh. The PNGs have
also been visually inspected. These checks establish the reference artifact,
not the correctness of a Go renderer or any provider integration.

The implementer must still capture the actual Go TUI at 160×48, 120×40, 100×32,
80×24, and 60×20, in dark, light, and monochrome modes. Verify selection visibility
across page boundaries, real terminal resize behavior, Unicode cell widths,
all returned metric families, and error transitions. Product acceptance remains
the unchecked criteria in SPEC.md and UX.md.
