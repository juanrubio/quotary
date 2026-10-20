package tui

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func FuzzWrapTextNeverPanicsOrOverflows(f *testing.F) {
	for _, seed := range []string{"", "hello world", strings.Repeat("界", 20), "a\x1b[2Jb", "Label: abcdefghijklmnopqrstuvwxyz", "multi\nline\tvalue"} {
		f.Add(seed, 11)
	}
	f.Fuzz(func(t *testing.T, value string, width int) {
		if width < 1 || width > 200 {
			return
		}
		lines := wrapText(value, width)
		for _, line := range lines {
			if !utf8.ValidString(line) {
				t.Fatalf("invalid UTF-8 for %q", line)
			}
			if ansi.StringWidth(line) > width {
				t.Fatalf("line width %d exceeds %d", ansi.StringWidth(line), width)
			}
		}
	})
}

func FuzzCleanTextStripsControls(f *testing.F) {
	for _, seed := range []string{"", "safe", "\x1b]0;title\a", "line\nbreak", "界"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		cleaned := cleanText(value)
		if strings.Contains(cleaned, "\x1b") {
			t.Fatalf("escape survived: %q", cleaned)
		}
		for _, r := range cleaned {
			if unicode.IsControl(r) {
				t.Fatalf("control survived: %q", cleaned)
			}
		}
	})
}
