# Canvas-background re-assertion report (dark "shadow" bands)

- Date: 2026-10-20 (UTC)
- Status: fixed — `model.paintLine` in `internal/tui/render.go` re-asserts the
  canvas background after every style reset; regression tests in
  `internal/tui/background_test.go` pass
- Severity: visual only — no data loss, no crash, no incorrect values
- Related: [TUI specification](../specs/tui.md),
  [terminal restoration record](terminal-restoration.txt)

## Symptom

On wide terminal sizes the rendered screens showed large pure-black bands
("shadows") hugging the content: a black band to the right of the setup
dialog, black areas right of the header text, and dark regions between
styled fragments. The content itself was complete; only the surrounding
background color was wrong.

## Environment

| Property | Value |
| --- | --- |
| Terminal | kitty (default background black) |
| Confirmed size | 158x98 cells; the defect also affected the top-right band of the header at smaller sizes |
| Reported by | the user running Quotary in a kitty window |

## Root cause

Quotary composes each frame line from concatenated Lip Gloss styled
fragments. Every fragment ends with an SGR reset (`ESC[m`), which also
returns the terminal to its **default** background. The frame then padded
each line to full width with plain spaces and wrapped it once in the Canvas
style. As a result:

- separator runs between fragments,
- trailing line padding, and
- blank cells the renderer skipped at line ends

carried no background. On kitty, whose default background is black, those
cells painted black instead of the canvas color, forming dark bands beside
and above the content. The canvas background was only ever asserted once at
the start of each line, so every reset that followed dropped the frame for
the rest of the line — exactly the staircase of black regions visible in the
screenshot.

## Fix

The repository already solved this for panels: `model.surface` prepends the
Surface background sequence and re-asserts it after every `ESC[m`, keeping
card interiors painted. `model.paintLine` (new) now does the same for the
canvas: every frame line is padded to width, prefixed with the canvas style
sequence (`ESC[38;2;…m` Text foreground + `ESC[48;2;…m` Canvas background),
and every `ESC[m` is followed by the same sequence. Both colours are
re-asserted, not just the background: the canvas style that `paintLine`
replaces (`Canvas.Render`) carried the Text foreground too, and re-asserting
only the background left unstyled text after a reset — the footer hints, for
example — in the terminal's default foreground. Combined with `surface()`,
cards keep their Surface background (the later SGR wins) while everything
else stays on the canvas color. The monochrome theme is untouched — it
deliberately has no background.

Verification:

- Re-playing the 158x98 setup frame on a background-tracking screen emulator
  shows zero cells with the terminal default background (previously ~45% of
  the header and right-of-dialog area).
- `internal/tui/background_test.go` asserts the invariant on every major
  screen and every golden size: each line is exactly `width` cells wide,
  starts with the canvas style, and every `ESC[m` is immediately followed by
  the canvas foreground and background SGRs.
- Goldens and stored evidence captures were regenerated; the only change to
  the stored bytes is the injected canvas style sequences — the rendered
  text grid is identical.
- `make check` and `make race` pass.

## Impact

- Not affected: text content, values, layout measurements, selection, key
  handling, terminal restoration, the demo/live distinction, and the REP
  expansion fix in `internal/tui/repexpand.go`.
- Terminals whose default background differs from black (tmux with a theme,
  xterm with a configured background) were also affected, but the dark bands
  were most visible on black-default terminals.