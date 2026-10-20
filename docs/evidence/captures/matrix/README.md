# Phase 9 size × theme capture matrix

These 22 ANSI captures are the required Phase 9 matrix, copied from
`internal/tui/testdata/golden/` and verified byte-for-byte by
`TestEvidenceCaptureMatrixMatchesGoldens`.

Regeneration command:

```sh
QUOTARY_GOLDEN_UPDATE=1 go test ./internal/tui -run TestGoldenMatrix -count=1
cp internal/tui/testdata/golden/*.ansi docs/evidence/captures/matrix/
```

Profile: forced TrueColor (`colorprofile.TrueColor`), no NO_COLOR, ANSI SGR
sequences retained. Matrix: Green light/dark and monochrome at 160×48, 120×40,
100×32, 80×24, 60×20; Blue, Amber, Monokai light/dark at 120×40; plus Green dark
page 2 at 160×48.

Mockup comparison: overview captures correspond to scenes 01–05 and 10–11 at the
same dimensions. The Go implementation intentionally differs where terminal-safe
wrapping, availability labels, and live/offline distinctions require it; scene
captures under `docs/evidence/captures/` remain the per-mockup comparison record.
