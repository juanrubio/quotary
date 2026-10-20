# TUI captures and acceptance matrix

These are ANSI text captures rendered by the Go TUI test harness, not browser
mockups. Regenerate the 15 scene captures from the repository root:

```sh
QUOTARY_CAPTURE_DIR="$PWD/docs/evidence/captures" \
  go test ./internal/tui -run TestCapturePhase3 -count=1
```

Regenerate/verify the size × theme acceptance matrix (22 goldens under
`internal/tui/testdata/golden/`) with:

```sh
go test ./internal/tui -run TestGoldenMatrix -count=1
```

The golden matrix is the Phase 9 capture artifact for the full required matrix:
Green light/dark and monochrome at 160×48, 120×40, 100×32, 80×24, and 60×20; Blue,
Amber, and Monokai at 120×40 in light and dark; plus a second-page selection
scene. It is generated with forced TrueColor (`colorprofile.TrueColor`), so files
contain ANSI SGR sequences rather than plain text. The 15 scene captures cover the
numbered browser mockups; the golden matrix covers the theme/size grid. Static
browser PNG `screens/09-source-library.png` predates the 108-source configured
variant extension; the interactive mockup text and Go captures are authoritative.

Comparison note: scene captures map to mockup scenes 01–15 by filename. Captures
may differ from the browser reference where the Go implementation intentionally
improves terminal-safe wrapping, filtering, or availability labels; differences
are reviewed through `TestGoldenMatrix`, `TestCapturePhase3`, and the TUI tests.
