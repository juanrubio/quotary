package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestCleanTextRemovesControlSequences(t *testing.T) {
	value := cleanText("\x1b[31mDanger\x1b[0m\x07\x00 ok\tvalue")
	if strings.ContainsAny(value, "\x1b\x07\x00") {
		t.Fatalf("control bytes survived: %q", value)
	}
	if value != "Danger ok value" {
		t.Fatalf("value = %q", value)
	}
}

func TestTruncateUsesDisplayWidth(t *testing.T) {
	value := truncateWidth("日本語テスト", 5)
	if ansi.StringWidth(value) > 5 {
		t.Fatalf("width = %d for %q", ansi.StringWidth(value), value)
	}
	if !strings.HasSuffix(value, "…") {
		t.Fatalf("truncation marker missing: %q", value)
	}
}

func TestEmptySourceTextIsSafe(t *testing.T) {
	if got := truncateWidth("", 10); got != "" {
		t.Fatalf("empty = %q", got)
	}
	if got := padLeft("x", 4); got != "   x" {
		t.Fatalf("padLeft = %q", got)
	}
}
