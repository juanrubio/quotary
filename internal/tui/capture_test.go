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

func TestCapturePhase3(t *testing.T) {
	dir := os.Getenv("QUOTARY_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set QUOTARY_CAPTURE_DIR to write review captures")
	}
	old := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	defer func() { lipgloss.Writer.Profile = old }()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, m *model, width, height int) {
		m.width, m.height = width, height
		if err := os.WriteFile(filepath.Join(dir, name), []byte(m.View().Content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := func(scenario demo.Scenario) *model { return demoModel(t, scenario) }

	m := base(demo.ScenarioSubscription)
	write("01-subscription-desk.ansi", m, 160, 48)
	write("02-overview.ansi", m, 120, 40)
	write("03-medium.ansi", m, 100, 32)
	write("04-compact.ansi", m, 80, 24)
	write("05-small-terminal.ansi", m, 60, 20)
	page2 := base(demo.ScenarioSubscription)
	if len(page2.visible) > 12 {
		page2.selected = 12
		page2.selectedKey = page2.visible[12].Key.String()
	}
	write("15-more-subscriptions.ansi", page2, 160, 48)

	attention := base(demo.ScenarioAttention)
	write("06-attention.ansi", attention, 120, 40)

	detail := base(demo.ScenarioAttention)
	detail.selectProvider("antigravity")
	detail.screen = screenDetails
	write("07-google-allowances.ansi", detail, 120, 40)

	history := base(demo.ScenarioSubscription)
	history.selectMuse()
	history.screen = screenHistory
	history.loadHistory()
	write("08-muse-history.ansi", history, 120, 40)

	picker := base(demo.ScenarioSubscription)
	picker.screen = screenPicker
	write("09-source-library.ansi", picker, 120, 40)

	daylight := base(demo.ScenarioSubscription)
	daylight.prefs.Theme = config.ThemeGreen
	daylight.light = true
	daylight.applyTheme()
	write("10-daylight.ansi", daylight, 120, 40)

	mono := base(demo.ScenarioAttention)
	mono.prefs.Monochrome = true
	mono.applyTheme()
	write("11-monochrome.ansi", mono, 80, 24)

	write("12-first-launch.ansi", base(demo.ScenarioEmpty), 100, 32)
	write("13-backend-missing.ansi", base(demo.ScenarioSetup), 100, 32)

	help := base(demo.ScenarioSubscription)
	help.screen = screenHelp
	write("14-keyboard-help.ansi", help, 100, 32)

	first, err := os.ReadFile(filepath.Join(dir, "01-subscription-desk.ansi"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "15-more-subscriptions.ansi"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) {
		t.Fatal("scene 15 capture duplicates scene 01")
	}
	if len(page2.visible) > 12 && !strings.Contains(string(second), page2.visible[12].DisplayName) {
		t.Fatal("scene 15 does not show the second page")
	}
}

func (m *model) selectProvider(provider string) {
	for index, snapshot := range m.visible {
		if snapshot.Key.Provider == provider {
			m.selected = index
			m.selectedKey = snapshot.Key.String()
			return
		}
	}
}
