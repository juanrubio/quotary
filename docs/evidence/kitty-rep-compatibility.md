# Kitty `REP` escape-sequence compatibility report

- Date: 2026-10-20 (UTC)
- Status: fixed — the REP-expansion writer landed in `internal/tui/repexpand.go`
  and is installed in `Run` (see [Implementation](#implementation)); the
  verification plan below is complete
- Affects: Quotary TUI rendering in kitty terminals that do not implement the
  `REP` escape sequence (confirmed on kitty 0.16.0; kitty 0.45.0 implements
  `REP`. The exact kitty version that introduced `REP` was not established by
  this report, so no version threshold is claimed)
- Severity: visual only — no data loss, no crash, no incorrect values
- Related: [Phase 9 verification](../implementation-plan.md#711-phase-9-verification),
  [terminal restoration record](terminal-restoration.txt),
  [visual and interaction specification](../specs/tui.md)

## Summary

Quotary's compact table layout renders with collapsed column padding in kitty
0.16.0, the kitty version installed at `~/.local/kitty.app/bin/kitty` and used by
all kitty windows in the review environment. Values are drawn immediately after
their column headers instead of in aligned columns.

The cause is not in Quotary's layout code. The terminal renderer under Bubble Tea
v2 assumes every kitty terminal implements the `REP` escape sequence
(`CSI Ps b`, repeat the preceding character). kitty 0.16.0 does not implement
`REP` and silently discards it, so every run of characters that the renderer
compressed with `REP` is lost.

The recommended fix is to expand `REP` sequences into literal characters before
they reach the terminal. This preserves every capability the terminal *does*
support and compensates only for the one it lacks. That fix is now implemented;
see [Implementation](#implementation).

## Symptom

The compact overview table is the most visible case, because its column padding
is rendered as runs of identical characters. The card layout is largely
unaffected because its borders are drawn as box-drawing characters rather than
padded with spaces.

Rendering at 100x30 in kitty 0.16.0 (incorrect — columns collapsed):

```
  Source                Allowance Left Resets State
> Claude Code 5h 72%                              2h 14m Current
```

The same application rendered in tmux and in kitty 0.45.0 (correct):

```
  Source                Allowance Left            Resets      State
> Claude Code           5h        72%             2h 14m      Current
```

The header row is truncated in a different place in each case, and data cells are
pulled left against the wrong header. Nothing is missing from the model — the
content is complete but misaligned, so a reader can misattribute a value to the
adjacent column.

## Environment

| Property | Value |
| --- | --- |
| Terminal under review | kitty 0.16.0 (`~/.local/kitty.app/bin/kitty`) |
| Comparison terminal, same app | kitty 0.45.0 (`/usr/bin/kitty`) |
| Comparison terminal, same app | tmux (`xterm-256color`) |
| `TERM` | `xterm-kitty` |
| `COLORTERM` | `truecolor` |
| Bubble Tea | `charm.land/bubbletea/v2 v2.0.2` |
| Renderer | `github.com/charmbracelet/ultraviolet v0.0.0-20260811164956-006e29f97886` |
| Platform | Linux 7.0.0-38-generic, x86-64 |
| Reported by | the user running Quotary in a kitty window |

## Reproduction

### 1. The terminal drops `REP`

`REP` (`CSI Ps b`) repeats the preceding graphic character `Ps` times. A one-line
probe run inside each terminal:

```
printf 'X\033[4bY|test\n'
```

| Terminal | Output | Meaning |
| --- | --- | --- |
| kitty 0.16.0 | `XY` | `REP` discarded, no repeat |
| kitty 0.45.0 | `XXXXXY` | `REP` honoured |

### 2. The application emits `REP`

Capturing the bytes Quotary writes to the terminal (`kitty --dump-bytes`) shows
the renderer compressing runs of identical characters with `REP`. From the
overview header row:

```
...mSource\x1b[16X\x1b[5;25HAllowance Left \x1b[11bResets \x1b[5bState\x1b[m
```

| Sequence | Meaning | kitty 0.16.0 |
| --- | --- | --- |
| `CSI 16 X` | `ECH` — erase 16 characters | honoured |
| `CSI 11 b` | `REP` — repeat 11 times | **discarded** |
| `CSI 5 b` | `REP` — repeat 5 times | **discarded** |

Across one captured session the renderer emitted 67 `REP` sequences covering 547
repeated cells.

### 3. The rendered result matches exactly what `REP` loss predicts

Removing precisely the two `REP`-rendered gaps from the correct output yields the
kitty 0.16.0 output byte for byte:

```
tmux/kitty 0.45.0 : '  Source                Allowance Left            Resets      State'
expected          : '  Source                Allowance Left Resets State'
kitty 0.16.0      : '  Source                Allowance Left Resets State'
expected == actual: True
```

The gap after `Source` survives because it was rendered with `ECH`; the gaps
after `Allowance Left` and `Resets` vanish because they were rendered with
`REP`. This confirms the mechanism end to end and rules out a layout or
measurement bug in Quotary.

## Root cause

`ultraviolet` derives terminal capabilities from `TERM` alone, in
`xtermCaps()`:

```go
case "contour", "foot", "ghostty", "kitty", "rio", "st", "tmux", "wezterm":
    v = allCaps
case "xterm":
    switch {
    case len(parts) > 1 && parts[1] == "kitty":
        ...
        v = allCaps
```

`allCaps` includes `capREP`:

```go
allCaps = capVPA | capHPA | capCHA | capCHT | capCBT | capREP | capECH | capICH | capSD | capSU
```

The assumption is that kitty supports `REP`. That holds for kitty 0.45.0 but
not for kitty 0.16.0, so the renderer compresses output with a sequence the
terminal throws away.

Two further points make this worth fixing at the application layer rather than
waiting for a dependency bump:

- The same unconditional assumption exists in the newer `ultraviolet` in the
  module cache (`v0.0.0-20261008173134-6b8d4baf91b4`, used by
  `bubbletea/v2 v2.1.0`), so upgrading Bubble Tea does not resolve it.
- `capREP` is not exposed through any public API, and the capability set is
  computed inside `ultraviolet` from `TERM`, so an application cannot switch it
  off without changing the environment the program runs in.

`kitty`'s own `xterm-kitty` terminfo entry advertises no `rep` capability in
either version, so terminfo cannot be used to detect the gap.

### Capabilities that are not affected

Verified working in kitty 0.16.0 by direct probe: `ECH` (`CSI Ps X`), `ICH`
(`CSI Ps @`), `CBT` (`CSI Ps Z`), `SU` (`CSI Ps S`), `SD` (`CSI Ps T`), `HPA`
(`CSI Pn G`), `VPA` (`CSI Pn d`), and truecolor SGR. An independent review
re-probed all of these side by side in kitty 0.16.0 and 0.45.0 with identical
results; the only parse error logged by 0.16.0 was for `CSI … b`, so `REP` is
the only gap in the `allCaps` set. Two capabilities in `allCaps` are not
listed here: `CHA` is the same `CSI Pn G` sequence as `HPA`, and the `CHT`
path in the pinned `ultraviolet` is disabled in code (`if false && …`), so
neither can affect rendering. `kitty` 0.16.0 also does
not implement the kitty keyboard protocol (`CSI = flags ; mode u` and the
`CSI ? u` query are unsupported), so no keyboard-enhancement negotiation occurs.
That degrades gracefully — keys are reported in the classic xterm form and
Quotary's key handling still works — and is not part of this defect.

## Impact

- **Scope:** any kitty terminal that does not implement `REP`.
- **User-visible effect:** column padding is lost wherever the renderer used
  `REP` to compress a run of identical characters. The compact overview table
  degrades most visibly; the card layout degrades less because it uses
  box-drawing characters for borders.
- **Not affected:** data correctness, values, ordering, selection, key handling,
  terminal restoration on quit, and the demo/live distinction. The model is
  rendered completely; only inter-column whitespace is lost.
- **Not detected by the existing test suite:** the golden captures
  (`docs/evidence/captures/`) assert the model's own rendered frame, not the
  byte stream sent to a terminal, so a terminal-side sequence drop is invisible
  to them. The existing terminal checks run under tmux and `script`, whose
  emulators do implement `REP`.

## Recommended fix

Expand `REP` sequences into literal characters in the byte stream on its way to
the terminal, so the terminal never has to interpret them.

**Where:** a small writer wrapper applied to the program's output, installed
before the Bubble Tea program starts, in `internal/tui`. There is exactly one
such installation point: `Run` constructs the only `tea.NewProgram` in the
repository, and no other code in `internal/` writes escape sequences to the
terminal, so one wrapper covers every byte the application emits.

**How it works:**

1. Scan the outgoing byte stream for `ESC [ Ps b` where `Ps` is a decimal count.
2. Replace each such sequence with `Ps` copies of the preceding graphic
   character, which the wrapper tracks as it scans. (The character itself has
   already been emitted once, so the total run is `Ps + 1` characters, matching
   `REP` semantics on a terminal that supports it.)
3. Everything else passes through byte for byte.

**Wrapper requirements.** These are load-bearing; a wrapper that misses any of
them trades this defect for a worse one:

- **Preserve `Fd()`.** Bubble Tea type-asserts the program output to
  `term.File` (an interface requiring `Fd() uintptr`) to populate its TTY
  output handle, which gates the initial window size, resize listening, and
  terminal restore; `colorprofile.Detect` (called by both Bubble Tea and
  `ultraviolet`) uses the same assertion to decide truecolor support. A
  wrapper that only implements `io.Writer` silently breaks resizing and
  downgrades colour — the same collateral damage cited below for rejecting
  the `TERM` rewrite. Embedding `*os.File` in the wrapper type satisfies the
  assertion, since `Fd()` is promoted.
- **Be stateful across `Write` calls.** Bubble Tea writes startup sequences,
  frames, and cursor movements as separate writes. The tracked preceding
  character must persist across writes, a trailing partial escape sequence
  must be buffered for the next write rather than emitted or dropped, and an
  escape sequence (for example SGR) intervening between a graphic character
  and its `REP` must not reset the tracked character. The pinned renderer
  happens to emit the cell immediately before `REP`, but the wrapper is
  specified generically and must not rely on that.
- **Honour the `io.Writer` contract.** `Write` must return the length of the
  *input*, not the expanded output, and must not modify the input slice.
- **Handle the parameter forms the scanner can meet.** An omitted parameter
  (`ESC [ b`) means 1, and a count of 0 is treated as the default (1),
  matching terminals that implement `REP`. The pinned renderer never emits
  either form — its `count > len(rep)` guard keeps counts well above 1 — so
  this is defensive, but it must be defined rather than left to chance. Only
  a bare single-parameter `CSI … b` is `REP`: sequences with private or
  intermediate markers (for example `CSI ? … b`) are not, and payloads of
  OSC, DCS, and APC sequences are never scanned.

**Why this shape:**

- It is terminal-agnostic. It does not need to know which terminal it is
  running under, and it cannot be wrong about a terminal's capabilities.
- It preserves every capability the terminal *does* have. Nothing is disabled;
  only the one unsupported sequence is emulated.
- It is self-contained, so it does not require a dependency fork, a `replace`
  directive, or an upstream patch to land first.
- The cost is bounded. `REP` exists to shorten output; expanding it makes the
  stream marginally longer for the affected runs only. In the captured session
  the 67 `REP` sequences covered 547 cells; replacing those sequences
  (4–6 bytes each) with literal repeats adds roughly 150–280 bytes net
  against a 4225-byte capture.

**Alternatives considered and rejected:**

| Alternative | Why rejected |
| --- | --- |
| Rewrite `TERM` to a value that maps to a capability set without `capREP` | `TERM` also drives colour-profile detection, terminfo key lookups, and the kitty keyboard protocol negotiation. Faking it would degrade unrelated behaviour and still not cover terminals that set `xterm-kitty` directly. |
| Upgrade Bubble Tea / `ultraviolet` | The newer `ultraviolet` in the module cache carries the identical unconditional `capREP` assumption for kitty. An upgrade does not fix the defect. |
| Fork or patch `ultraviolet` | Requires vendoring or a `replace` directive, and takes the fix out of this repository's control. The application-level fix is smaller and reviewable. |
| Detect kitty version and branch on it | Kitty reports no version in a form the app can read before the first frame, and version sniffing is brittle. The `REP` expansion is correct regardless of version. |
| Restrict the compact layout so no padding runs are long enough to be compressed | Only changes which runs get compressed; the terminal would still drop the `REP`-encoded ones. Does not address the defect. |

**Detection note.** The wrapper may be applied unconditionally rather than
gated on a terminal probe. Probing is unreliable here: kitty implements no
capability query for `REP`, and its terminfo entry advertises no `rep`
capability in any version, so there is no positive signal to probe for. The
expansion is semantically transparent — a terminal that does implement `REP`
receives the same characters it would have received anyway, just spelled out.

## Implementation

The recommended fix is implemented and reviewed within this repository:

- `internal/tui/repexpand.go` defines `repExpandWriter`, the output wrapper
  that expands bare `REP` sequences (`CSI Ps b` with a single decimal
  parameter) into literal repetitions of the preceding graphic character.
  Its incremental scanner is stateful across `Write` calls (a partial escape
  sequence or UTF-8 rune at the end of a write is buffered, never emitted or
  dropped), tracks the preceding graphic character across intervening SGR,
  simple escape, and control bytes, never scans OSC/DCS/APC payloads, treats
  an omitted or zero parameter as 1, leaves marker or multi-parameter forms
  alone, honours the `io.Writer` contract (returns the input length, never
  modifies the input), is safe for concurrent use, and bounds both its
  buffered partial sequences and its expansion counts so a malformed stream
  cannot grow memory without limit.
- `Run` in `internal/tui/app.go` installs the wrapper with
  `tea.WithOutput(newRepExpandWriter(os.Stdout))` at the program's single
  construction site, so every escape sequence the application emits passes
  through it. The embedded `*os.File` keeps the output satisfying
  `term.File` (asserted at compile time), which preserves the `Fd()`-gated
  behaviours Bubble Tea and colour detection depend on.
- `internal/tui/repexpand_test.go` unit-tests the wrapper byte for byte:
  expansion cases, pass-through identity cases, split-write cases, and a
  round-trip identity check on a representative frame without `REP`.
- `internal/tui/repexpand_terminal_test.go` is the terminal-level regression
  test. It runs the real demo program with `TERM=xterm-kitty` so the
  renderer's kitty capability set — and therefore its `REP` compression — is
  active, captures the emitted byte stream, and interprets it on a minimal
  screen emulator that either honours `REP` or parses and discards it exactly
  as kitty 0.16.0 does. It asserts that the unwrapped stream reproduces the
  defect (collapsed header, `Allowance Left Resets State`), that the wrapped
  stream contains zero `REP` sequences and renders the identical screen on
  both terminal models, that the emulated screen equals the model's own
  rendered view, that the initial window size is picked up (the pager sits on
  the last row), that truecolor SGR survives the wrapper, and that a resize
  repaint passes through the wrapper's retained state correctly.

Two verification limits are recorded honestly:

- kitty is not available in the automated test environment (the same limit
  noted under reproduction), so the regression test simulates the kitty 0.16
  `REP`-dropping behaviour in its emulator rather than driving a real kitty
  process.
- The `Fd()`-protected behaviours are verified structurally (compile-time
  `term.File` assertion; the wrapper's methods delegate to the same file, so
  `Fd()`, `IsTerminal`, resize listening, and terminal restoration see the
  same descriptor they would without the wrapper) and behaviourally through
  the program's non-tty path (window size, resize repaint, truecolor). A
  manual pass in a real kitty 0.16.0 window — resize, truecolor, quit-restore
  — remains the final confirmation.

## Verification plan

1. **Unit test** the wrapper: `CSI 5 b` after a space expands to five spaces;
   the sequence is not expanded when it is the first thing in the stream (no
   preceding character); an omitted parameter and a count of `0` both expand
   as 1; a `CSI` with a private or intermediate marker ending in `b` (for
   example `CSI ? … b`) is not mistaken for `REP` — note that `b` is itself a
   CSI final byte, so it can never appear as a parameter byte, and the cases
   to exclude are marker and multi-parameter forms; `REP` inside an OSC, DCS,
   or APC payload is not touched; non-`b` CSI final bytes are untouched; the
   return value is the input length; input is not modified. Split-stream
   cases are also covered: a `REP` sequence split across two `Write` calls, a
   preceding character written by an earlier `Write`, a `Write` ending in a
   partial escape sequence, and an SGR sequence between the character and its
   `REP` (which must not reset the tracked character).
2. **Round-trip test** that the wrapper is byte-for-byte identity on output that
   contains no `REP`.
3. **Terminal-level test** that renders the compact overview into a pty whose
   emulator drops `REP`, and asserts the resulting screen still has the column
   gaps. This is the regression test that would have caught the defect; it must
   simulate the kitty 0.16 behaviour rather than trusting the current terminal.
   The same test (or a manual pass in a real terminal) must confirm the
   wrapper has not regressed the behaviour its `Fd()` requirement protects:
   the initial window size is picked up, resizing the window re-renders, and
   truecolor output is preserved.
4. **Golden and matrix captures** re-verified unchanged, since the model's
   rendered frame is unaffected by the fix.
5. `make check`, `make race`, and the external review gate re-run per the
   delivery rules in [the implementation prompt](../prompts/implement-all-phases.md).

## Traceability

| Requirement | Where handled |
| --- | --- |
| Phase 9 hardening and prototype closeout | [implementation plan §7.11](../implementation-plan.md#711-phase-9-verification) |
| Layout and small-terminal requirements | [TUI specification](../specs/tui.md) |
| Terminal restoration on quit | [terminal restoration record](terminal-restoration.txt) |
| Stored capture and matrix verification | [captures README](captures/README.md) |

## Notes for the reviewer

- The defect is in the interaction between Quotary's renderer and a terminal
  that lacks `REP`; Quotary's layout and measurement code are correct and are
  unchanged by the fix.
- The fix is implemented as a Phase 9 follow-up, not a new phase: it hardens
  an already-delivered phase rather than adding scope. `make check` and
  `make race` pass, and the golden and capture matrices are unchanged, as
  the fix touches only the byte stream between renderer and terminal.
- An independent review on 2026-10-20 corroborated the mechanism end to end:
  it re-ran the `REP` probe and the remaining `allCaps` probes in both kitty
  versions (only `REP` differed), re-derived the header-gap arithmetic from
  the captured sequences, and confirmed the resolved module versions with
  `go list -m`. Its additions are the wrapper requirements and verification
  cases in the fix section, the `CHA`/`CHT` note above, and the corrections
  that follow in this list.
- The kitty version that introduced `REP` was not established; the header of
  this report claims no threshold. Only 0.16.0 (drops `REP`) and 0.45.0
  (honours it) are evidenced.
- The capture counts (67 `REP` sequences, 547 cells, 4225 bytes) come from
  the author's captured session; the byte capture itself is not stored in
  this repository, so those figures could not be re-derived independently.
  The mechanism does not depend on them.
- No credentials, cookies, provider payloads, or account identifiers were
  accessed to produce this report. The captured byte stream is synthetic demo
  output; the only path-like token in it is the `/tmp/quotary` build path.
- Reproducing this issue required running kitty, which is not available in the
  automated test environment. The recommended regression test therefore
  simulates the kitty 0.16 `REP`-dropping behaviour in a pty rather than
  requiring kitty to be installed.
