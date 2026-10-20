package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/juanrubio/quotary/internal/backend/demo"
	"github.com/juanrubio/quotary/internal/config"
)

type goldenCase struct {
	name          string
	width, height int
	theme         config.ThemeName
	light, mono   bool
}

func TestGoldenMatrix(t *testing.T) {
	old := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	defer func() { lipgloss.Writer.Profile = old }()

	sizes := [][2]int{{160, 48}, {120, 40}, {100, 32}, {80, 24}, {60, 20}}
	themes := []config.ThemeName{config.ThemeGreen, config.ThemeBlue, config.ThemeAmber, config.ThemeMonokai}
	var cases []goldenCase
	for _, size := range sizes {
		for _, light := range []bool{false, true} {
			name := "green-dark"
			if light {
				name = "green-light"
			}
			cases = append(cases, goldenCase{name + "-" + itoa(size[0]) + "x" + itoa(size[1]), size[0], size[1], config.ThemeGreen, light, false})
		}
		cases = append(cases, goldenCase{"mono-" + itoa(size[0]) + "x" + itoa(size[1]), size[0], size[1], config.ThemeGreen, false, true})
	}
	for _, theme := range themes {
		for _, light := range []bool{false, true} {
			name := string(theme) + "-dark"
			if light {
				name = string(theme) + "-light"
			}
			cases = append(cases, goldenCase{name + "-120x40", 120, 40, theme, light, false})
		}
	}
	cases = append(cases, goldenCase{"page2-green-dark-160x48", 160, 48, config.ThemeGreen, false, false})

	update := os.Getenv("QUOTARY_GOLDEN_UPDATE") == "1"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := demoModel(t, demo.ScenarioSubscription)
			m.prefs.Theme = tc.theme
			m.prefs.Monochrome = tc.mono
			m.light = tc.light
			m.noColor = tc.mono
			m.applyTheme()
			if tc.name == "page2-green-dark-160x48" {
				m.selected = 12
				m.selectedKey = m.visible[12].Key.String()
			}
			view := checkFrame(t, m, tc.width, tc.height)
			path := filepath.Join("testdata", "golden", tc.name+".ansi")
			if update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(view), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden %s (run QUOTARY_GOLDEN_UPDATE=1 go test ./internal/tui -run TestGoldenMatrix): %v", path, err)
			}
			if view != string(want) {
				t.Fatalf("golden mismatch for %s", tc.name)
			}
		})
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [8]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}

func TestEvidenceCaptureMatrixMatchesGoldens(t *testing.T) {
	goldenDir := filepath.Join("testdata", "golden")
	evidenceDir := filepath.Join("..", "..", "docs", "evidence", "captures", "matrix")
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ansi") {
			continue
		}
		seen[entry.Name()] = true
		want, err := os.ReadFile(filepath.Join(goldenDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(evidenceDir, entry.Name()))
		if err != nil {
			t.Fatalf("missing evidence capture %s: %v", entry.Name(), err)
		}
		if string(want) != string(got) {
			t.Fatalf("evidence capture drift: %s", entry.Name())
		}
		count++
	}
	if count != 22 {
		t.Fatalf("matrix capture count=%d, want 22", count)
	}
	evidenceEntries, err := os.ReadDir(evidenceDir)
	if err != nil {
		t.Fatal(err)
	}
	extra := 0
	for _, entry := range evidenceEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ansi") {
			continue
		}
		if !seen[entry.Name()] {
			extra++
		}
	}
	if extra != 0 {
		t.Fatalf("evidence matrix has %d extra captures", extra)
	}
}
