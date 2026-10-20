package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/juanrubio/quotary/internal/backend/demo"
	"github.com/juanrubio/quotary/internal/config"
)

// This file tests the canvas-background invariant: every cell of a rendered
// frame must carry the canvas background, so a terminal whose default
// background is black (kitty, for example) never paints dark bands beside
// the content. See docs/evidence/kitty-rep-compatibility.md.

// canvasBgPrefixes lists the full style resets lipgloss emits between
// fragments; after each one the frame must immediately re-assert the canvas
// background.
var canvasBgPrefixes = []string{"\x1b[m", "\x1b[0m"}

// assertCanvasSequence checks that s begins with the canvas style re-asserted
// as a truecolor foreground (palette Text) immediately followed by a
// truecolor background (palette Canvas). The canvas style is both colours;
// asserting only the background lets unstyled text after a reset fall back
// to the terminal's default foreground.
func assertCanvasSequence(t *testing.T, s, context string) {
	t.Helper()
	if !strings.HasPrefix(s, "\x1b[38;2;") {
		t.Fatalf("%s: not followed by a canvas foreground SGR: %q", context, s)
	}
	end := strings.Index(s, "m")
	if end < 0 || !strings.HasPrefix(s[end+1:], "\x1b[48;2;") {
		t.Fatalf("%s: canvas foreground not followed by a canvas background SGR: %q", context, s)
	}
}

// assertCanvasBackground checks that every line of the rendered frame starts
// with the canvas style, that every reset is immediately followed by it
// again, and that each line is exactly width cells wide.
func assertCanvasBackground(t *testing.T, view string, width int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		if got := lipgloss.Width(line); got != width {
			t.Fatalf("line %d width = %d, want %d", i, got, width)
		}
		assertCanvasSequence(t, line, "line "+itoa(i)+" start")
		for _, reset := range canvasBgPrefixes {
			index := 0
			for {
				j := strings.Index(line[index:], reset)
				if j < 0 {
					break
				}
				at := index + j
				rest := line[at+len(reset):]
				assertCanvasSequence(t, rest, "line "+itoa(i)+" reset at "+itoa(at))
				index = at + len(reset)
			}
		}
	}
}

func TestCanvasBackgroundSurvivesEveryReset(t *testing.T) {
	old := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	defer func() { lipgloss.Writer.Profile = old }()

	cases := []struct {
		name   string
		build  func() *model
		width  int
		height int
		mono   bool
	}{
		{"setup", func() *model {
			m := newModel(Options{SetupError: "The backend returned unreadable data."})
			m.width, m.height = 158, 98
			m.applyTheme()
			return m
		}, 158, 98, false},
		{"overview", func() *model {
			m := demoModel(t, demo.ScenarioSubscription)
			m.width, m.height = 120, 40
			m.applyTheme()
			return m
		}, 120, 40, false},
		{"details", func() *model {
			m := demoModel(t, demo.ScenarioSubscription)
			m.screen = screenDetails
			m.selected = 0
			m.selectedKey = m.visible[0].Key.String()
			m.width, m.height = 120, 40
			m.applyTheme()
			return m
		}, 120, 40, false},
		{"settings", func() *model {
			m := demoModel(t, demo.ScenarioSubscription)
			m.screen = screenSettings
			m.width, m.height = 120, 40
			m.applyTheme()
			return m
		}, 120, 40, false},
		{"help", func() *model {
			m := demoModel(t, demo.ScenarioSubscription)
			m.screen = screenHelp
			m.width, m.height = 120, 40
			m.applyTheme()
			return m
		}, 120, 40, false},
		{"mono overview", func() *model {
			m := demoModel(t, demo.ScenarioSubscription)
			m.prefs.Monochrome = true
			m.noColor = true
			m.applyTheme()
			m.width, m.height = 120, 40
			return m
		}, 120, 40, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.build()
			view := m.View().Content
			if tc.mono {
				// The monochrome theme deliberately has no colors or canvas
				// background; the bold-only resets are the whole story.
				return
			}
			assertCanvasBackground(t, view, tc.width)
		})
	}
}

// TestGoldenMatrixBackgrounds ensures every stored golden also upholds the
// invariant, so a future renderer change cannot silently reintroduce the
// shadow bands.
func TestGoldenMatrixBackgrounds(t *testing.T) {
	for _, size := range [][2]int{{160, 48}, {120, 40}, {100, 32}, {80, 24}, {60, 20}} {
		m := demoModel(t, demo.ScenarioSubscription)
		m.prefs.Theme = config.ThemeGreen
		m.applyTheme()
		m.width, m.height = size[0], size[1]
		assertCanvasBackground(t, m.View().Content, size[0])
	}
}
