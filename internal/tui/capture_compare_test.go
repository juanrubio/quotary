package tui

import (
	"os"
	"path/filepath"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/juanrubio/quotary/internal/backend/demo"
	"github.com/juanrubio/quotary/internal/config"
)

func TestStoredSceneCapturesMatch(t *testing.T) {
	old := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	defer func() { lipgloss.Writer.Profile = old }()
	scene := func(name string, m *model, width, height int) {
		m.width, m.height = width, height
		got := m.View().Content
		want, err := os.ReadFile(filepath.Join("..", "..", "docs", "evidence", "captures", name))
		if err != nil {
			t.Fatalf("missing stored capture %s: %v", name, err)
		}
		if got != string(want) {
			t.Fatalf("stored capture drift: %s", name)
		}
	}
	m := demoModel(t, demo.ScenarioSubscription)
	scene("01-subscription-desk.ansi", m, 160, 48)
	scene("02-overview.ansi", m, 120, 40)
	scene("03-medium.ansi", m, 100, 32)
	scene("04-compact.ansi", m, 80, 24)
	scene("05-small-terminal.ansi", m, 60, 20)
	page2 := demoModel(t, demo.ScenarioSubscription)
	if len(page2.visible) > 12 {
		page2.selected = 12
		page2.selectedKey = page2.visible[12].Key.String()
	}
	scene("15-more-subscriptions.ansi", page2, 160, 48)
	scene("06-attention.ansi", demoModel(t, demo.ScenarioAttention), 120, 40)
	detail := demoModel(t, demo.ScenarioAttention)
	detail.selectProvider("antigravity")
	detail.screen = screenDetails
	scene("07-google-allowances.ansi", detail, 120, 40)
	history := demoModel(t, demo.ScenarioSubscription)
	history.selectMuse()
	history.screen = screenHistory
	history.loadHistory()
	scene("08-muse-history.ansi", history, 120, 40)
	picker := demoModel(t, demo.ScenarioSubscription)
	picker.screen = screenPicker
	scene("09-source-library.ansi", picker, 120, 40)
	daylight := demoModel(t, demo.ScenarioSubscription)
	daylight.prefs.Theme = config.ThemeGreen
	daylight.light = true
	daylight.applyTheme()
	scene("10-daylight.ansi", daylight, 120, 40)
	mono := demoModel(t, demo.ScenarioAttention)
	mono.prefs.Monochrome = true
	mono.applyTheme()
	scene("11-monochrome.ansi", mono, 80, 24)
	scene("12-first-launch.ansi", demoModel(t, demo.ScenarioEmpty), 100, 32)
	scene("13-backend-missing.ansi", demoModel(t, demo.ScenarioSetup), 100, 32)
	help := demoModel(t, demo.ScenarioSubscription)
	help.screen = screenHelp
	scene("14-keyboard-help.ansi", help, 100, 32)
}
