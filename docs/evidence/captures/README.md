# Phase 3 TUI captures

These are ANSI text captures rendered by the Go TUI test harness, not browser
mockups. Regenerate from the repository root:

```sh
QUOTARY_CAPTURE_DIR="$PWD/docs/evidence/captures" \
  go test ./internal/tui -run TestCapturePhase3 -count=1
```

The captures cover the 15 reference scenes at the required dimensions, including
light, dark, and monochrome. Phase 9 replaces or supplements these with final
acceptance captures.
