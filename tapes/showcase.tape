# README showcase (read-only, fictional): open the offline demo, walk the
# cards, open a source, flip to its History, toggle the compact list, peek
# at the source library and the keyboard guide, then leave through the
# quit confirmation. Recorded against the built-in demo backend only
# (see docs/tapes.md) — no CodexBar, account, or network is touched.
Output ../docs/media/showcase.gif
Set Shell bash
Set Width 1440
Set Height 900
Set FontSize 18
Set FontFamily "DejaVu Sans Mono"
Set Padding 16
Set Framerate 12
Set CursorBlink false
Set Theme { "background": "#0b1020", "foreground": "#e6edf3", "cursor": "#7dd3fc" }

# Start on the populated subscription desk.
Hide
Type "../quotary --demo"
Enter
Sleep 3s
Show
Sleep 2s

# Walk the cards, then open the focused source.
Type "j"
Sleep 1200ms
Type "j"
Sleep 1200ms
Enter
Sleep 2s

# Flip to History and back, then return to the desk.
Tab
Sleep 2s
Tab
Sleep 1200ms
Escape
Sleep 1200ms

# Toggle cards to the compact list and back.
Type "l"
Sleep 1500ms
Type "l"
Sleep 1200ms

# Peek at the source library and the keyboard guide.
Type "p"
Sleep 1800ms
Escape
Sleep 800ms
Type "?"
Sleep 1800ms
Escape
Sleep 800ms

# Leave through the quit confirmation: ask, cancel, then confirm.
Type "q"
Sleep 1500ms
Escape
Sleep 800ms
Hide
Type "qy"
Sleep 500ms
