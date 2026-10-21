package tui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/backend/demo"
	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/domain"
	"github.com/juanrubio/quotary/internal/theme"
)

func demoModel(t *testing.T, scenario demo.Scenario) *model {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	runtime, err := demo.NewRuntime(scenario, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	return newModel(Options{Demo: true, Session: runtime})
}

func checkFrame(t *testing.T, m *model, width, height int) string {
	t.Helper()
	m.width, m.height = width, height
	view := m.View().Content
	if got := lipgloss.Width(view); got != width {
		t.Fatalf("%v/%dx%d width = %d", m.screen, width, height, got)
	}
	if got := lipgloss.Height(view); got != height {
		t.Fatalf("%v/%dx%d height = %d", m.screen, width, height, got)
	}
	return view
}

func TestOverviewAtRequiredSizes(t *testing.T) {
	sizes := [][2]int{{160, 48}, {120, 40}, {100, 32}, {80, 24}, {60, 20}}
	for _, size := range sizes {
		m := demoModel(t, demo.ScenarioSubscription)
		view := checkFrame(t, m, size[0], size[1])
		if !strings.Contains(view, "QUOTARY") || !strings.Contains(view, "DEMO") {
			t.Fatalf("%dx%d missing shell labels", size[0], size[1])
		}
		if !strings.Contains(view, "Claude Code") {
			t.Fatalf("%dx%d missing source", size[0], size[1])
		}
	}
}

func TestAllScreensRenderAtRequiredSizes(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {100, 32}, {80, 24}, {60, 20}} {
		for _, screen := range []screen{screenDetails, screenHistory, screenPicker, screenSettings, screenHelp} {
			m := demoModel(t, demo.ScenarioSubscription)
			m.screen = screen
			m.selectMuse()
			if screen == screenHistory || screen == screenDetails {
				m.loadHistory()
			}
			checkFrame(t, m, size[0], size[1])
		}
	}
}

func (m *model) selectMuse() {
	for index, snapshot := range m.visible {
		if snapshot.Key.Provider == "muse" {
			m.selected = index
			m.selectedKey = snapshot.Key.String()
			return
		}
	}
}

func TestFourThemesLightDarkAndMonochrome(t *testing.T) {
	for _, name := range []config.ThemeName{config.ThemeGreen, config.ThemeBlue, config.ThemeAmber, config.ThemeMonokai} {
		for _, light := range []bool{false, true} {
			m := demoModel(t, demo.ScenarioAttention)
			m.prefs.Theme = name
			m.light = light
			m.applyTheme()
			view := checkFrame(t, m, 120, 40)
			if !strings.Contains(view, "Google · Antigravity") {
				t.Fatalf("%s/%v missing attention source", name, light)
			}
			if !strings.Contains(view, "Exhausted") {
				t.Fatalf("%s/%v lost state text", name, light)
			}
		}
	}
	m := demoModel(t, demo.ScenarioAttention)
	m.prefs.Theme = config.ThemeMonokai
	m.noColor = true
	m.applyTheme()
	view := checkFrame(t, m, 80, 24)
	if !strings.Contains(view, "> ") || !strings.Contains(view, "Exhausted") {
		t.Fatalf("monochrome lost selection/state cues:\n%s", view)
	}
}

func TestThemeChooserSavesAndApplies(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	if m.screen != screenSettings {
		t.Fatal("settings did not open")
	}
	m.changeSetting(1)
	if m.settingsDraft.Theme != config.ThemeBlue {
		t.Fatalf("theme draft = %s", m.settingsDraft.Theme)
	}
	m.saveSettings()
	if m.prefs.Theme != config.ThemeBlue {
		t.Fatalf("theme = %s", m.prefs.Theme)
	}
	if m.session.Preferences().Theme != config.ThemeBlue {
		t.Fatal("theme was not persisted through the session store")
	}
}

func TestSearchAndFilterContext(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, r := range "codex" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if len(m.visible) == 0 || !strings.Contains(strings.ToLower(m.visible[0].DisplayName), "codex") {
		t.Fatalf("search results = %v", m.visible)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.query != "" {
		t.Fatalf("query = %q", m.query)
	}
	m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	if m.filter != "attention" {
		t.Fatalf("filter = %s", m.filter)
	}
}

func TestHistoryMissingDayAndTokenOnly(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.loadHistory()
	if m.historyErr != "" {
		t.Fatal(m.historyErr)
	}
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "4.74M") || !strings.Contains(view, "dollar") {
		t.Fatalf("history view missing totals/token-only copy:\n%s", view)
	}
}

func TestPickerTrackingAndBulkAction(t *testing.T) {
	m := demoModel(t, demo.ScenarioEmpty)
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if m.screen != screenPicker {
		t.Fatal("picker did not open")
	}
	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	tracked := 0
	for _, descriptor := range m.catalog.Providers {
		if descriptor.Tracked {
			tracked++
		}
	}
	if tracked == 0 {
		t.Fatal("bulk tracking tracked nothing")
	}
	_ = checkFrame(t, m, 120, 40)
}

func TestCardColumnBreakpoints(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	cases := map[int]int{160: 4, 120: 3, 100: 2, 88: 2, 80: 1}
	for width, want := range cases {
		if got := m.cardColumns(width); got != want {
			t.Fatalf("width %d columns = %d want %d", width, got, want)
		}
		if want > 1 {
			cardWidth := (width - 4 - (want-1)*2) / want
			if cardWidth < 36 {
				t.Fatalf("width %d card width = %d", width, cardWidth)
			}
		}
	}
}

func TestForcedColorPreservesStylingAndSelection(t *testing.T) {
	old := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	defer func() { lipgloss.Writer.Profile = old }()
	green := demoModel(t, demo.ScenarioAttention)
	green.prefs.Theme = config.ThemeGreen
	green.applyTheme()
	greenView := checkFrame(t, green, 120, 40)
	monokai := demoModel(t, demo.ScenarioAttention)
	monokai.prefs.Theme = config.ThemeMonokai
	monokai.applyTheme()
	monokaiView := checkFrame(t, monokai, 120, 40)
	if !strings.Contains(greenView, "\x1b[") {
		t.Fatal("styles were stripped from the rendered frame")
	}
	if greenView == monokaiView {
		t.Fatal("theme palettes render identically")
	}
	if !strings.Contains(greenView, "> ") {
		t.Fatal("selection marker missing")
	}
}

func TestFooterAndTinyTerminal(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	view := checkFrame(t, m, 80, 24)
	if !strings.Contains(view, "q Quit") || !strings.Contains(view, "1–") {
		t.Fatalf("footer shortcuts/position missing:\n%s", view)
	}
	tiny := demoModel(t, demo.ScenarioSubscription)
	_ = checkFrame(t, tiny, 1, 10)
}

func TestResponsiveHeightUsesCompactLayout(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	view := checkFrame(t, m, 120, 24)
	if strings.Contains(view, "╭─") {
		t.Fatalf("120x24 should use compact rows:\n%s", view)
	}
	if !strings.Contains(view, "Source") || !strings.Contains(view, "Current") {
		t.Fatalf("compact layout missing columns:\n%s", view)
	}
}

func TestHistoryBoundToSelectedSource(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.loadHistory()
	if m.history.Key.Provider != "muse" {
		t.Fatalf("history = %s", m.history.Key)
	}
	m.move(1)
	if m.history.Key.Provider != "" {
		t.Fatal("history survived a selection change")
	}
	view := checkFrame(t, m, 120, 40)
	if strings.Contains(view, "Muse Code") && strings.Contains(view, "4.74M") {
		t.Fatal("stale Muse history rendered under another source")
	}
}

func TestFreshnessUnknownAndPrivacyMasking(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	snapshot.SampleAt = nil
	snapshot.ReceivedAt = m.now()
	if got := ageLabel(snapshot, m.now()); got != "time unknown" {
		t.Fatalf("age = %q", got)
	}
	key := snapshot.Key
	key.Account = "private-account"
	key.AccountUnknown = false
	snapshot.Key = key
	m.prefs.Privacy = true
	if got := m.displayKey(snapshot); strings.Contains(got, "private-account") {
		t.Fatalf("privacy leaked key: %s", got)
	}
	if got := m.accountLabel(snapshot); strings.Contains(got, "private-account") {
		t.Fatalf("privacy leaked label: %s", got)
	}
}

func TestHistoryRangeControl(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.historyDays = 30
	m.loadHistory()
	if m.history.Days != 30 || len(m.history.Buckets) != 30 {
		t.Fatalf("history range = %d/%d", m.history.Days, len(m.history.Buckets))
	}
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "[30 days]") {
		t.Fatalf("range control missing:\n%s", view)
	}
}

func TestSettingsCancelAndMonochromeSetting(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	original := m.prefs.Theme
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.changeSetting(1)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if m.prefs.Theme != original {
		t.Fatalf("cancel kept draft theme %s", m.prefs.Theme)
	}
	if m.session.Preferences().Theme != original {
		t.Fatal("cancel wrote to the store")
	}
	m.prefs.Monochrome = true
	m.noColor = false
	m.applyTheme()
	if !m.theme.Mono {
		t.Fatal("explicit monochrome setting was ignored")
	}
}

func TestRefreshAllAdvancesTrackedCollection(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	for i := 0; i < 100 && m.refreshing; i++ {
		m.Update(refreshTickMsg{generation: m.refreshGen})
	}
	for _, snapshot := range m.all {
		if snapshot.State == domain.StatusQueued || snapshot.State == domain.StatusFetching {
			t.Fatalf("%s stranded in %s", snapshot.Key, snapshot.State)
		}
	}
}

func TestFocusedSearchAcceptsQ(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if m.query != "q" {
		t.Fatalf("query = %q", m.query)
	}
}

func TestUnpinnedCardOmitsPinnedLabel(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	for index, snapshot := range m.visible {
		if contains(m.prefs.Pins, snapshot.Key.String()) {
			continue
		}
		card := strings.Join(m.card(index, 48, 10), "\n")
		if !strings.Contains(card, snapshot.Plan) {
			t.Fatalf("card did not render plan %q:\n%s", snapshot.Plan, card)
		}
		if strings.Contains(card, " · pinned") {
			t.Fatalf("unpinned card claimed to be pinned:\n%s", card)
		}
		return
	}
	t.Fatal("fixture has no unpinned source")
}

func TestUnitAllowanceAndBalanceOnlyRendering(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	used, limit, zero := 2.0, 10.0, 0.0
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "unit"}, DisplayName: "Unit Source", State: domain.StatusCurrent,
		Windows: []domain.AllowanceWindow{{Label: "Requests", Used: &used, Limit: &limit, Unit: "requests"}},
	}}
	view := strings.Join(m.renderCards(120, 40), "\n")
	if !strings.Contains(view, "2/10 requests") {
		t.Fatalf("unit allowance missing:\n%s", view)
	}
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "credits"}, DisplayName: "Credits Source", State: domain.StatusCurrent,
		Balances: []domain.Balance{{Label: "Credits", Value: &zero, Unit: "USD"}},
	}}
	view = strings.Join(m.renderCards(120, 40), "\n")
	if !strings.Contains(view, "0.00 USD") {
		t.Fatalf("balance-only reading missing:\n%s", view)
	}
}

func TestRenderedExternalTextIsSanitized(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	snapshot.DisplayName = "Evil\x1b]8;;https://example.com\aClick\x1b]8;;\a"
	snapshot.Windows[0].Label = "5h\x07"
	snapshot.Details = []domain.DetailSection{{Title: "chart\x1b[2J", Rows: []domain.DetailRow{{Label: "x\x00", Value: "y\x1b]0;pwned\a"}}}}
	m.visible[0] = sanitizeSnapshot(snapshot)
	m.screen = screenDetails
	view := m.View().Content
	if strings.Contains(view, "\x1b]8") || strings.Contains(view, "\x07") || strings.Contains(view, "\x00") {
		t.Fatalf("terminal control sequence survived:\n%q", view)
	}
}

func TestDetailsMetadataAtNarrowWidths(t *testing.T) {
	m := demoModel(t, demo.ScenarioAttention)
	m.selectProvider("antigravity")
	m.screen = screenDetails
	for _, size := range [][2]int{{80, 24}, {60, 20}} {
		m.detailScroll = 999
		view := checkFrame(t, m, size[0], size[1])
		if !strings.Contains(view, "Account") && !strings.Contains(view, "Unknown") {
			t.Fatalf("%dx%d details metadata missing:\n%s", size[0], size[1], view)
		}
	}
}

func TestHistoryPricingSemantics(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.history = domain.HistorySnapshot{
		Key: m.visible[m.selected].Key, Days: 7, Timezone: "UTC", Provenance: "local",
		Coverage: "partial", Partial: true, TotalTokens: int64Ptr(1000),
		Cost: &domain.Money{Amount: 1.25, Currency: "USD"}, Priced: true,
		Buckets: []domain.HistoryDay{{Day: "2026-10-16", Tokens: int64Ptr(1000)}},
	}
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "1.25 USD") {
		t.Fatalf("priced history missing amount:\n%s", view)
	}
	view = checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "1.25 USD") {
		t.Fatalf("compact priced history missing amount:\n%s", view)
	}
	m.history.Cost = nil
	m.history.Priced = true
	view = checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "Pricing unavailable") {
		t.Fatalf("unavailable pricing not disclosed:\n%s", view)
	}
}

func int64Ptr(value int64) *int64 { return &value }

func TestSettingsThemeChooserWithKeys(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	original := m.prefs.Theme
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.settingsDraft.Theme == original {
		t.Fatal("j did not select another theme")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.prefs.Theme == original || m.session.Preferences().Theme != m.prefs.Theme {
		t.Fatalf("theme chooser did not save: %s", m.prefs.Theme)
	}
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.prefs.Theme != m.session.Preferences().Theme {
		t.Fatal("cancelled theme chooser diverged from the store")
	}
}

func TestNoColorEnvironmentDisablesColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm-256color")
	runtime, err := demo.NewRuntime(demo.ScenarioSubscription, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	view := checkFrame(t, m, 80, 24)
	if strings.Contains(view, "38;2;") || strings.Contains(view, "48;2;") {
		t.Fatal("NO_COLOR frame contains truecolor sequences")
	}
}

func TestCanvasBackgroundIsPainted(t *testing.T) {
	old := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	defer func() { lipgloss.Writer.Profile = old }()
	m := demoModel(t, demo.ScenarioSubscription)
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "48;2;") {
		t.Fatal("canvas background was not painted")
	}
	if !strings.Contains(view, theme.BackgroundSequence(theme.For("green", false).Surface)) {
		t.Fatal("surface background was not painted")
	}
}

func TestCardGeometryIsAligned(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	card := m.card(0, 37, 10)
	for index, line := range card {
		if got := ansiWidth(line); got != 37 {
			t.Fatalf("card line %d width = %d", index, got)
		}
	}
}

func TestAllowanceUnitSanitization(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	snapshot.Windows = []domain.AllowanceWindow{{Label: "Requests", Used: floatPtr(2), Limit: floatPtr(10), Unit: "req\x1b]0;pwned\a"}}
	m.visible[0] = sanitizeSnapshot(snapshot)
	m.screen = screenDetails
	view := m.View().Content
	if strings.Contains(view, "\x1b]0") {
		t.Fatal("allowance unit injection survived")
	}
}

func floatPtr(value float64) *float64 { return &value }

func TestScenarioChangeClearsHistory(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.loadHistory()
	if m.history.TotalTokens == nil {
		t.Fatal("history did not load")
	}
	m.switchScenario("2")
	if m.history.Key.Provider == "muse" && m.history.TotalTokens != nil {
		t.Fatal("scenario change retained old Muse history")
	}
	view := m.View().Content
	if strings.Contains(view, "4.74M") {
		t.Fatal("old history rendered after scenario change")
	}
}

func TestSettingsTabReachesEveryControl(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	for m.settingsIdx != 5 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	before := m.settingsDraft.Monochrome
	m.changeSetting(1)
	if m.settingsDraft.Monochrome == before {
		t.Fatal("monochrome control did not change")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.prefs.Monochrome == before {
		t.Fatal("monochrome setting was not saved")
	}
}

func TestHistoryDisclosuresAtSmallSizes(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.loadHistory()
	for _, size := range [][2]int{{80, 24}, {100, 32}, {60, 20}} {
		view := checkFrame(t, m, size[0], size[1])
		for _, want := range []string{"known tokens", "DAILY", "Token history only"} {
			if !strings.Contains(view, want) {
				t.Fatalf("%dx%d missing %q:\n%s", size[0], size[1], want, view)
			}
		}
	}
}

func TestDetailsWidePaneFitsAllocation(t *testing.T) {
	m := demoModel(t, demo.ScenarioAttention)
	m.selectProvider("antigravity")
	m.screen = screenDetails
	m.width, m.height = 120, 40
	for index, line := range strings.Split(m.View().Content, "\n") {
		if ansiWidth(line) > 120 {
			t.Fatalf("line %d width = %d", index, ansiWidth(line))
		}
	}
	view := m.View().Content
	if !strings.Contains(view, "SOURCE KEY") || !strings.Contains(view, "Reading") {
		t.Fatalf("details metadata pane incomplete:\n%s", view)
	}
}

func TestCompactPreservesUnitsAndContext(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	zero := 0.0
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "credits"}, DisplayName: "Credits", Plan: "Pro", State: domain.StatusCurrent,
		Balances: []domain.Balance{{Label: "Credits", Value: &zero, Unit: "USD"}},
	}}
	view := checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "USD") || !strings.Contains(view, "Credits") || !strings.Contains(view, "Pro") {
		t.Fatalf("compact context/unit missing:\n%s", view)
	}
}

func TestOverLimitKeepsZeroLeftAndQualifier(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	used := 137.0
	remaining := 0.0
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "over"}, DisplayName: "Over", State: domain.StatusExhausted,
		Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &used}},
	}}
	card := strings.Join(m.card(0, 37, 10), "\n")
	if !strings.Contains(card, "0% left") || !strings.Contains(card, "Over") {
		t.Fatalf("over-limit card lost meaning:\n%s", card)
	}
	_ = remaining
	compact := strings.Join(m.renderCompact(60, 20), "\n")
	if !strings.Contains(compact, "Over") {
		t.Fatalf("compact over-limit qualifier missing:\n%s", compact)
	}
}

func TestSearchQueryVisibleAtCompactWidth(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, r := range "codex" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	view := checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "/ codex") {
		t.Fatalf("search context not visible:\n%s", view)
	}
}

func TestRefreshOutcomeVisibleInDetails(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	for i := 0; i < 10 && m.refreshing; i++ {
		m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	}
	m.screen = screenDetails
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "Success") {
		t.Fatalf("successful refresh outcome missing:\n%s", view)
	}
}

func TestHistoryScrollReachesLastDailyValue(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.historyDays = 30
	m.loadHistory()
	found := false
	for offset := 0; offset < 400 && !found; offset += 5 {
		m.historyScroll = offset
		found = strings.Contains(checkFrame(t, m, 60, 20), "10-16")
	}
	if !found {
		t.Fatal("scrolled history never exposed the final day")
	}
}

func TestHistoryTabKeysAndScrolling(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenDetails
	m.detailTab = 1
	m.loadHistory()
	m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	if m.historyDays != 30 || m.history.Days != 30 {
		t.Fatalf("range = %d/%d", m.historyDays, m.history.Days)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if m.historyScroll == 0 {
		t.Fatal("history tab did not scroll history")
	}
	if m.detailScroll != 0 {
		t.Fatal("history tab scrolled allowances instead")
	}
}

func TestHistoryCostProvenanceLabels(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.history = domain.HistorySnapshot{
		Key: m.visible[m.selected].Key, Days: 7, Timezone: "UTC", Coverage: "Local machine scope",
		TotalTokens: int64Ptr(1000), Cost: &domain.Money{Amount: 2.5, Currency: "USD"}, Priced: true,
		Buckets: []domain.HistoryDay{{Day: "2026-10-16", Tokens: int64Ptr(1000)}},
	}
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "Estimated API-price cost") || !strings.Contains(view, "Local machine scope") {
		t.Fatalf("estimated cost/scope missing:\n%s", view)
	}
	m.history.CostReported = true
	view = checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "Reported cost") {
		t.Fatalf("reported cost label missing:\n%s", view)
	}
}

func TestLongDetailValuesWrapAndRemainAccessible(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	snapshot.Details = []domain.DetailSection{{Title: "Long", Rows: []domain.DetailRow{{
		Label: "Description", Value: strings.Repeat("accessible-value-", 12),
	}}}}
	m.visible[0] = sanitizeSnapshot(snapshot)
	m.screen = screenDetails
	found := false
	for offset := 0; offset < 80 && !found; offset++ {
		m.detailScroll = offset
		view := checkFrame(t, m, 80, 24)
		found = strings.Contains(view, "accessible-value-")
	}
	if !found {
		t.Fatal("wrapped detail value inaccessible at every scroll position")
	}
}

func TestCompactRenderedSourceCountMatchesFooter(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	view := checkFrame(t, m, 80, 24)
	expected := fmt.Sprintf("1–%d / 24", m.pageSize())
	if !strings.Contains(view, expected) {
		t.Fatalf("compact footer count does not match viewport %s:\n%s", expected, view)
	}
}

func TestWrapTextUnicodeAndOrder(t *testing.T) {
	lines := wrapText("Label: abcdefghijklmnopqrstuvwxyz", 11)
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "Label:") {
		t.Fatalf("order = %v", lines)
	}
	unicodeLines := wrapText(strings.Repeat("界", 12), 11)
	for _, line := range unicodeLines {
		if !utf8.ValidString(line) || ansiWidth(line) > 11 {
			t.Fatalf("invalid wrap %q width=%d", line, ansiWidth(line))
		}
	}
}

func TestLongAllowanceLabelAccessible(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	snapshot.Windows[0].Label = strings.Repeat("allowance-label-", 10)
	m.visible[0] = sanitizeSnapshot(snapshot)
	m.screen = screenDetails
	found := false
	for offset := 0; offset < 40 && !found; offset++ {
		m.detailScroll = offset
		found = strings.Contains(m.View().Content, "allowance-label-")
	}
	if !found {
		t.Fatal("long allowance label is not accessible")
	}
}

func TestCompactPreservesLongUnitAt80(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	used, limit := 2.0, 10.0
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "unit"}, DisplayName: "Units", State: domain.StatusCurrent,
		Windows: []domain.AllowanceWindow{{Label: "Requests", Used: &used, Limit: &limit, Unit: "requests"}},
	}}
	view := checkFrame(t, m, 80, 24)
	if !strings.Contains(view, "8/10 requests") {
		t.Fatalf("unit truncated:\n%s", view)
	}
}

func TestAuthFailureLabelsRetainedData(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	snapshot.Error = domain.NewScopedError(domain.ErrAuth, snapshot.Key.String(), "expired")
	snapshot.State = domain.StatusAuthFailed
	snapshot.SampleAt = nil
	m.visible[0] = sanitizeSnapshot(snapshot)
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "prior") {
		t.Fatalf("retained data not labelled:\n%s", view)
	}
}

func TestGenericHistoryCoverageMetadata(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioSubscription, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	history, err := runtime.History(context.Background(), backend.HistoryRequest{ProviderID: "claude", SourceKey: "claude:configured@unknown", Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	available := 0
	for _, bucket := range history.Buckets {
		if bucket.Tokens != nil {
			available++
		}
	}
	if history.Incomplete != len(history.MissingDays) || history.Incomplete != len(history.Buckets)-available {
		t.Fatalf("coverage metadata = incomplete:%d missing:%d available:%d", history.Incomplete, len(history.MissingDays), available)
	}
}

func TestNarrowLibraryKeepsDistinctionsAndShortcuts(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	view := checkFrame(t, m, 80, 24)
	if !strings.Contains(view, "subscription") || !strings.Contains(view, "available") {
		t.Fatalf("narrow library lost kind/availability:\n%s", view)
	}
	if !strings.Contains(view, "? q") {
		t.Fatalf("narrow library lost quit/help shortcuts:\n%s", view)
	}
}

func TestTinyHistoryAndUnavailableTransition(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.loadHistory()
	_ = checkFrame(t, m, 60, 8)
	m.history = domain.HistorySnapshot{}
	m.historyErr = ""
	view := checkFrame(t, m, 80, 24)
	if !strings.Contains(view, "History unavailable") {
		t.Fatalf("missing history not rendered honestly:\n%s", view)
	}
}

func TestPrivacyHidesSensitiveDetailValues(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	snapshot.Details = []domain.DetailSection{{Title: "Account", Rows: []domain.DetailRow{
		{Label: "Email", Value: "person@example.com"},
		{Label: "Home", Value: "/home/person/private"},
		{Label: "Note", Value: "safe"},
	}}}
	m.visible[0] = sanitizeSnapshot(snapshot)
	m.prefs.Privacy = true
	m.screen = screenDetails
	m.detailScroll = 0
	found := false
	for offset := 0; offset < 60; offset++ {
		m.detailScroll = offset
		view := m.View().Content
		if strings.Contains(view, "person@example.com") || strings.Contains(view, "/home/person/private") {
			t.Fatalf("privacy leaked sensitive value at offset %d:\n%s", offset, view)
		}
		if strings.Contains(view, "Hidden · privacy mode") {
			found = true
		}
	}
	if !found {
		t.Fatal("privacy mask not shown")
	}
}

func TestDetailMetadataFullyScrollable(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	snapshot.Plan = strings.Repeat("VeryLongPlanName", 8)
	snapshot.ReportedSource = strings.Repeat("https://example.test/reported/"+"path/", 6)
	m.visible[0] = sanitizeSnapshot(snapshot)
	m.screen = screenDetails
	seen := ""
	for offset := 0; offset < 80; offset++ {
		m.detailScroll = offset
		seen += strings.ReplaceAll(cleanText(m.View().Content), " ", "")
	}
	if !strings.Contains(seen, "VeryLongPlanName") || !strings.Contains(seen, "example.test/reported/") {
		t.Fatal("long metadata never became fully accessible")
	}
}

func TestResetAbsoluteUsesLocalTimezone(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	reset := m.now().Add(time.Hour)
	snapshot.Windows = []domain.AllowanceWindow{{Label: "Plan", UsedPercent: floatPtr(20), ResetAt: &reset}}
	m.visible[0] = sanitizeSnapshot(snapshot)
	m.location = time.FixedZone("CEST", 2*60*60)
	m.screen = screenDetails
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "CEST") {
		t.Fatalf("absolute reset not in local timezone:\n%s", view)
	}
}

func TestExplicitCardsLayoutOnShortTerminal(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Layout = config.LayoutCards
	view := checkFrame(t, m, 120, 24)
	if !strings.Contains(view, "╭─") || !strings.Contains(view, "Max · pinned") {
		t.Fatalf("explicit cards layout ignored at 120x24:\n%s", view)
	}
}

func TestDetailsShowRawAllowanceAlongsidePercent(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	used, limit := 246.0, 1230.0
	percent := 20.0
	snapshot.Windows = []domain.AllowanceWindow{{Label: "Requests", Used: &used, Limit: &limit, UsedPercent: &percent, Unit: "requests"}}
	m.visible[0] = sanitizeSnapshot(snapshot)
	m.screen = screenDetails
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "80% left") || !strings.Contains(view, "246/1230 requests") {
		t.Fatalf("raw allowance fields hidden:\n%s", view)
	}
}

func TestLongBalanceRemainsAccessible(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	amount := 12.34
	snapshot.Balances = []domain.Balance{{Label: strings.Repeat("very-long-balance-label-", 8), Value: &amount, Unit: "USD"}}
	m.visible[0] = sanitizeSnapshot(snapshot)
	m.screen = screenDetails
	found := false
	for offset := 0; offset < 80 && !found; offset++ {
		m.detailScroll = offset
		seen := strings.ReplaceAll(cleanText(m.View().Content), " ", "")
		found = strings.Contains(seen, "12.34USD")
	}
	if !found {
		t.Fatal("balance amount and unit remained inaccessible")
	}
}

func TestCompactReducedPreservesUnitsAt60(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	used, limit := 2.0, 10.0
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "unit"}, DisplayName: "Units", State: domain.StatusCurrent,
		Windows: []domain.AllowanceWindow{{Label: "Requests", Used: &used, Limit: &limit, Unit: "requests"}},
	}}
	view := checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "8/10 requests") {
		t.Fatalf("unit truncated at 60x20:\n%s", view)
	}
}

func TestLongDetailsHeadingAndSectionTitlesAccessible(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	snapshot.Plan = strings.Repeat("VeryLongReportedPlan", 12)
	snapshot.Details = []domain.DetailSection{{Title: strings.Repeat("LongSectionTitle", 10), Rows: []domain.DetailRow{{Label: "A", Value: "B"}}}}
	m.visible[0] = sanitizeSnapshot(snapshot)
	m.screen = screenDetails
	first := checkFrame(t, m, 60, 20)
	if !strings.Contains(first, "Resets in") {
		t.Fatalf("long heading hid allowances:\n%s", first)
	}
	seen := ""
	for offset := 0; offset < 120; offset++ {
		m.detailScroll = offset
		seen += strings.ReplaceAll(cleanText(m.View().Content), " ", "")
	}
	if !strings.Contains(seen, "LongSectionTitle") {
		t.Fatal("long section title never fully accessible")
	}
}

func TestHistoryChartsAndModelsRemainScrollable(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.loadHistory()
	if len(m.history.Models) == 0 {
		t.Fatal("fixture has no model breakdown")
	}
	modelName := m.history.Models[0].Name
	for _, size := range [][2]int{{100, 32}, {80, 24}} {
		foundChart, foundModel := false, false
		for offset := 0; offset < 200 && (!foundChart || !foundModel); offset++ {
			m.historyScroll = offset
			view := checkFrame(t, m, size[0], size[1])
			foundChart = foundChart || strings.Contains(view, "TOKENS / DAY")
			foundModel = foundModel || strings.Contains(view, modelName)
		}
		if !foundChart || !foundModel {
			t.Fatalf("%dx%d history missing chart=%v models=%v", size[0], size[1], foundChart, foundModel)
		}
	}
}

func TestCompactRowsLabelRetainedAuthData(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[1]
	snapshot.Error = domain.NewScopedError(domain.ErrAuth, snapshot.Key.String(), "expired")
	snapshot.State = domain.StatusAuthFailed
	m.visible = []domain.UsageSnapshot{m.visible[0], sanitizeSnapshot(snapshot)}
	m.selected = 0
	view := checkFrame(t, m, 80, 24)
	if !strings.Contains(view, "prior") || !strings.Contains(view, "Auth failed") {
		t.Fatalf("retained auth data unlabelled in compact rows:\n%s", view)
	}
}

type failingSaveSession struct {
	Session
	fail bool
}

func (f *failingSaveSession) SavePreferences(prefs config.Preferences) error {
	if f.fail {
		return errors.New("save failed")
	}
	return f.Session.SavePreferences(prefs)
}

func TestSettingsSuccessClearsPreviousError(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	session := &failingSaveSession{Session: m.session, fail: true}
	m.session = session
	m.screen = screenSettings
	m.settingsDraft = m.prefs
	m.saveSettings()
	if m.errorText == "" {
		t.Fatal("failed save did not expose an error")
	}
	session.fail = false
	m.saveSettings()
	if m.errorText != "" {
		t.Fatalf("resolved save retained error %q", m.errorText)
	}
	if m.status != "Settings saved" || m.screen != screenOverview {
		t.Fatalf("success not reflected: status=%q screen=%v", m.status, m.screen)
	}
}

func TestOverLimitPreservesReportedPercentage(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	usedPercent := 137.0
	snapshot.Windows = []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &usedPercent}}
	m.visible[0] = sanitizeSnapshot(snapshot)
	m.screen = screenDetails
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "137% used") || !strings.Contains(view, "exceeds 100") {
		t.Fatalf("over-limit reported percentage lost:\n%s", view)
	}
}

func TestRetainedBalanceWithoutTimestampIsLabelled(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	amount := 12.34
	snapshot.Windows = nil
	snapshot.Balances = []domain.Balance{{Label: "Credits", Value: &amount, Unit: "USD"}}
	snapshot.SampleAt = nil
	snapshot.Error = domain.NewScopedError(domain.ErrAuth, snapshot.Key.String(), "expired")
	snapshot.State = domain.StatusAuthFailed
	m.visible = []domain.UsageSnapshot{sanitizeSnapshot(snapshot), m.visible[1]}
	m.selected = 0
	details := checkFrame(t, m, 120, 40)
	if !strings.Contains(details, "prior") {
		t.Fatalf("balance-only retained data unlabelled in details:\n%s", details)
	}
	m.screen = screenDetails
	details = checkFrame(t, m, 120, 40)
	if !strings.Contains(details, "prior") {
		t.Fatalf("balance-only retained data unlabelled in metadata:\n%s", details)
	}
	compact := checkFrame(t, m, 80, 24)
	if !strings.Contains(compact, "prior") {
		t.Fatalf("balance-only retained data unlabelled in compact rows:\n%s", compact)
	}
}

func TestCompleteSourceNameAccessibleInDetails(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	suffix := "UNIQUE-NAME-SUFFIX"
	snapshot.DisplayName = strings.Repeat("Long Source Name ", 8) + suffix
	m.visible[0] = sanitizeSnapshot(snapshot)
	m.screen = screenDetails
	found := false
	for offset := 0; offset < 120 && !found; offset++ {
		m.detailScroll = offset
		found = strings.Contains(cleanText(m.View().Content), suffix)
	}
	if !found {
		t.Fatal("full source name never accessible")
	}
}

func TestEmptyPinnedFilterIsNotFirstLaunchCopy(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Pins = nil
	m.filter = "pinned"
	m.filterVisible()
	view := checkFrame(t, m, 80, 24)
	if strings.Contains(view, "No tracked sources") || !strings.Contains(view, "No pinned sources") || !strings.Contains(view, "clear the filter") {
		t.Fatalf("empty pinned filter rendered incorrectly:\n%s", view)
	}
}

func TestNoResultsScenarioRestoresCollectionAfterClearing(t *testing.T) {
	m := demoModel(t, demo.ScenarioNoResults)
	if len(m.all) != 24 || len(m.visible) != 0 {
		t.Fatalf("all=%d visible=%d", len(m.all), len(m.visible))
	}
	if !strings.Contains(checkFrame(t, m, 80, 24), "No results") {
		t.Fatal("no-results state not rendered")
	}
	m.query = ""
	m.filterVisible()
	if len(m.visible) != 24 {
		t.Fatalf("clearing search restored %d sources", len(m.visible))
	}
}

func TestCardRetainedAgeVisibleWithExtraWindows(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	percent := 20.0
	snapshot.Windows = []domain.AllowanceWindow{{Label: "A", UsedPercent: &percent}, {Label: "B", UsedPercent: &percent}, {Label: "C", UsedPercent: &percent}}
	sample := m.now().Add(-42 * time.Minute)
	snapshot.SampleAt = &sample
	snapshot.Error = domain.NewScopedError(domain.ErrAuth, snapshot.Key.String(), "expired")
	snapshot.State = domain.StatusAuthFailed
	m.visible = []domain.UsageSnapshot{sanitizeSnapshot(snapshot)}
	m.selected = 0
	for _, size := range [][2]int{{120, 40}, {160, 48}} {
		view := checkFrame(t, m, size[0], size[1])
		if !strings.Contains(view, "42m ago") || !strings.Contains(view, "prior") || !strings.Contains(view, "+1 more") {
			t.Fatalf("%dx%d lost retained age/extra windows:\n%s", size[0], size[1], view)
		}
	}
}

func TestLongHistoryModelNamesAndTotalsStayAccessible(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.loadHistory()
	input, output := int64(900), int64(800)
	m.history.Models = []domain.ModelTotal{{Name: strings.Repeat("模型-very-long-name-", 6) + "TAIL", Input: &input, Output: &output}}
	for _, size := range [][2]int{{120, 40}, {100, 32}, {80, 24}} {
		seen := ""
		for offset := 0; offset < 160; offset++ {
			m.historyScroll = offset
			seen += strings.ReplaceAll(cleanText(checkFrame(t, m, size[0], size[1])), " ", "")
		}
		if !strings.Contains(seen, "TAIL") || !strings.Contains(seen, "1.7k") {
			t.Fatalf("%dx%d model name/total inaccessible", size[0], size[1])
		}
	}
}

func TestNarrowHistoryRangeHintAndHelp(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.loadHistory()
	view := checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "[7d]") || !strings.Contains(view, "30d") {
		t.Fatalf("narrow range hint clipped:\n%s", view)
	}
	m.screen = screenHelp
	help := ""
	for page := 0; page < 10; page++ {
		help += checkFrame(t, m, 60, 20)
		if m.helpScroll >= m.helpMaxScroll() {
			break
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if !strings.Contains(help, "7-day / 30-day history") {
		t.Fatalf("help does not document range shortcuts:\n%s", help)
	}
}

func TestCompactLeftColumnShowsRemainingQuantity(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	used, limit := 2.0, 10.0
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "unit"}, DisplayName: "Units", State: domain.StatusCurrent,
		Windows: []domain.AllowanceWindow{{Label: "Requests", Used: &used, Limit: &limit, Unit: "requests"}},
	}}
	for _, size := range [][2]int{{80, 24}, {60, 20}} {
		view := checkFrame(t, m, size[0], size[1])
		if !strings.Contains(view, "8/10 requests") || strings.Contains(view, "2/10 requests") {
			t.Fatalf("%dx%d left column misrepresents quantity:\n%s", size[0], size[1], view)
		}
	}
}

func TestHistoryPartialFlagDisclosedIndependentlyOfProse(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.loadHistory()
	m.history.Partial = true
	m.history.Coverage = "This machine only"
	for _, size := range [][2]int{{120, 40}, {80, 24}, {60, 20}} {
		found := false
		for offset := 0; offset < 80 && !found; offset++ {
			m.historyScroll = offset
			found = strings.Contains(checkFrame(t, m, size[0], size[1]), "Partial coverage")
		}
		if !found {
			t.Fatalf("%dx%d did not disclose HistorySnapshot.Partial", size[0], size[1])
		}
	}
}

func TestReducedCompactKeepsResetCountdown(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	reset := m.now().Add(2*time.Hour + 14*time.Minute)
	for i := range m.visible {
		for j := range m.visible[i].Windows {
			m.visible[i].Windows[j].ResetAt = &reset
		}
	}
	view := checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "2h 14m") || !strings.Contains(view, "Resets") {
		t.Fatalf("reduced compact lost reset countdown:\n%s", view)
	}
}

func TestHistoryCoverageAndProvenanceWrapToSuffix(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.loadHistory()
	m.history.Coverage = strings.Repeat("coverage qualifier ", 8) + "COVERAGE-TAIL"
	m.history.Provenance = strings.Repeat("provenance source ", 8) + "PROVENANCE-TAIL"
	for _, size := range [][2]int{{80, 24}, {60, 20}} {
		seen := ""
		for offset := 0; offset < 80; offset++ {
			m.historyScroll = offset
			seen += strings.ReplaceAll(cleanText(checkFrame(t, m, size[0], size[1])), " ", "")
		}
		if !strings.Contains(seen, "COVERAGE-TAIL") || !strings.Contains(seen, "PROVENANCE-TAIL") {
			t.Fatalf("%dx%d truncated coverage/provenance", size[0], size[1])
		}
	}
}

type countingSession struct {
	Session
	historyCalls int
}

func (c *countingSession) History(ctx context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
	c.historyCalls++
	return c.Session.History(ctx, req)
}

func TestHistoryDisplayReusesSnapshotUntilRefresh(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	session := &countingSession{Session: m.session}
	m.session = session
	m.screen = screenHistory
	m.loadHistoryIfNeeded()
	m.loadHistoryIfNeeded()
	if session.historyCalls != 1 {
		t.Fatalf("display actions made %d history calls", session.historyCalls)
	}
	m.onRefresh(false)
	if session.historyCalls != 2 {
		t.Fatalf("explicit refresh made %d total history calls", session.historyCalls)
	}
}

func TestInvalidQuantitiesAreNotRenderedAsMeasurements(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	negUsed, limit, inf := -2.0, 10.0, math.Inf(1)
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "bad"}, DisplayName: "Bad", State: domain.StatusCurrent,
		Windows:  []domain.AllowanceWindow{{Label: "Requests", Used: &negUsed, Limit: &limit, Unit: "requests"}},
		Balances: []domain.Balance{{Label: "Credits", Value: &inf, Unit: "USD"}},
	}}
	overview := checkFrame(t, m, 80, 24)
	if strings.Contains(overview, "12/10") || strings.Contains(overview, "+Inf") || strings.Contains(overview, "NaN") {
		t.Fatalf("invalid numbers rendered as measurements:\n%s", overview)
	}
	m.screen = screenDetails
	details := checkFrame(t, m, 120, 40)
	if strings.Contains(details, "12/10") || strings.Contains(details, "+Inf") || strings.Contains(details, "NaN") {
		t.Fatalf("invalid numbers rendered in details:\n%s", details)
	}
}

func TestNarrowDetailsShowsRetainedWarningAlongsideAllowances(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	sample := m.now().Add(-34 * time.Second)
	snapshot.SampleAt = &sample
	snapshot.Error = domain.NewScopedError(domain.ErrAuth, snapshot.Key.String(), "expired")
	snapshot.State = domain.StatusAuthFailed
	m.visible = []domain.UsageSnapshot{sanitizeSnapshot(snapshot)}
	m.screen = screenDetails
	m.detailScroll = 0
	view := checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "prior") || !strings.Contains(view, "34s ago") {
		t.Fatalf("narrow details hid retained warning:\n%s", view)
	}
}

func TestMalformedFailureRetainsPriorDataLabel(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	sample := m.now().Add(-34 * time.Second)
	snapshot.SampleAt = &sample
	snapshot.Error = domain.NewScopedError(domain.ErrMalformedJSON, snapshot.Key.String(), "bad json")
	snapshot.State = domain.StatusError
	m.visible = []domain.UsageSnapshot{sanitizeSnapshot(snapshot)}
	m.selected = 0
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "prior") || !strings.Contains(view, "34s ago") {
		t.Fatalf("non-auth retained reading unlabelled:\n%s", view)
	}
}

func TestReducedCompactPrimaryResetAlignment(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	reset := m.now().Add(65 * time.Minute)
	percent := 25.0
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "single"}, DisplayName: "Single", State: domain.StatusCurrent,
		Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent, ResetAt: &reset}},
	}}
	view := checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "1h 05m") {
		t.Fatalf("primary row reset missing:\n%s", view)
	}
}

func TestNarrowDetailsShowsStaleStateWithoutError(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	sample := m.now().Add(-42 * time.Minute)
	snapshot.SampleAt = &sample
	snapshot.Error = nil
	snapshot.Quality = domain.QualityStale
	for i := range snapshot.Windows {
		snapshot.Windows[i].Quality = domain.QualityStale
	}
	m.visible = []domain.UsageSnapshot{sanitizeSnapshot(snapshot)}
	m.screen = screenDetails
	m.detailScroll = 0
	view := checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "Stale") || !strings.Contains(view, "42m ago") {
		t.Fatalf("narrow details hid stale state/age:\n%s", view)
	}
}

func TestThirtyDayChartFitsNarrowWidth(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.historyDays = 30
	m.screen = screenHistory
	m.loadHistory()
	lines := m.historyLines(60, 200)
	for _, line := range lines {
		if ansiWidth(line) > 60 {
			t.Fatalf("history line exceeds 60 cells (%d): %q", ansiWidth(line), cleanText(line))
		}
	}
}

func TestSettingsShowsAutoLayoutChoice(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Layout = config.LayoutAuto
	m.settingsDraft = m.prefs
	m.screen = screenSettings
	m.settingsIdx = 2
	view := checkFrame(t, m, 80, 24)
	if !strings.Contains(view, "auto") || !strings.Contains(view, "[auto]") {
		t.Fatalf("auto layout choice/marker missing:\n%s", view)
	}
}

func TestCompactUsedWithoutLimitShowsRemainingUnavailable(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	used := 1000.0
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "used"}, DisplayName: "Used", State: domain.StatusCurrent,
		Windows: []domain.AllowanceWindow{{Label: "Requests", Used: &used, Unit: "requests"}},
	}}
	for _, size := range [][2]int{{80, 24}, {60, 20}} {
		view := checkFrame(t, m, size[0], size[1])
		if strings.Contains(view, "1000 requests") || !strings.Contains(view, "—") {
			t.Fatalf("%dx%d presented used quantity as remaining:\n%s", size[0], size[1], view)
		}
	}
}

func TestCardPreservesRealisticQuantityUnit(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	used, limit := 1000.0, 2000.0
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "quota"}, DisplayName: "Quota", State: domain.StatusCurrent,
		Windows: []domain.AllowanceWindow{{Label: "Requests", Used: &used, Limit: &limit, Unit: "requests"}},
	}}
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "1000/2000 requests") {
		t.Fatalf("card truncated quantity unit:\n%s", view)
	}
}

func TestOverlappingRefreshDoesNotStrandQueuedSource(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	first := m.refreshKey
	m.move(1)
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	for i := 0; i < 100 && m.refreshing; i++ {
		m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	}
	for _, snapshot := range m.all {
		if snapshot.State == domain.StatusQueued || snapshot.State == domain.StatusFetching {
			t.Fatalf("%s stranded after overlapping refresh", snapshot.Key)
		}
	}
	if first == "" {
		t.Fatal("first refresh was not started")
	}
}

func TestHistoryBoundaryNavigationKeepsCachedSnapshot(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selected = 0
	m.selectedKey = m.visible[0].Key.String()
	session := &countingSession{Session: m.session}
	m.session = session
	m.screen = screenHistory
	m.loadHistoryIfNeeded()
	if session.historyCalls != 1 {
		t.Fatalf("initial history calls = %d", session.historyCalls)
	}
	m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if session.historyCalls != 1 {
		t.Fatalf("boundary navigation reloaded history (%d calls)", session.historyCalls)
	}
}

func TestCompactLargeQuantitiesKeepUnitsAndColumns(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	used, limit := 1_000_000.0, 2_000_000.0
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "large"}, DisplayName: "Large", State: domain.StatusCurrent,
		Windows: []domain.AllowanceWindow{{Label: "Tokens", Used: &used, Limit: &limit, Unit: "tokens"}},
	}}
	for _, size := range [][2]int{{80, 24}, {60, 20}} {
		view := checkFrame(t, m, size[0], size[1])
		if !strings.Contains(view, "1M/2M tokens") || strings.Contains(view, "1e+06") {
			t.Fatalf("%dx%d large quantity/unit lost:\n%s", size[0], size[1], view)
		}
	}
}

func TestNarrowChartKeepsEarlyRangeAndGaps(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.loadHistory()
	first := int64(500)
	buckets := make([]domain.HistoryDay, 30)
	for i := range buckets {
		buckets[i].Day = m.history.End.AddDate(0, 0, i-29).Format("2006-01-02")
		if i == 0 {
			buckets[i].Tokens = &first
		}
	}
	m.history.Buckets = buckets
	lines := m.chartLines(m.history, 60)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "█") {
		t.Fatalf("early-range activity omitted from narrow chart:\n%s", joined)
	}
	if !strings.Contains(joined, "?") {
		t.Fatalf("missing-day gaps not disclosed in narrow chart:\n%s", joined)
	}
	for _, line := range lines {
		if ansiWidth(line) > 60 {
			t.Fatalf("chart line exceeds 60 cells (%d)", ansiWidth(line))
		}
	}
}

func TestCardBalanceKeepsAmountWithLongLabel(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	amount := 12.34
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "credit"}, DisplayName: "Credit", State: domain.StatusCurrent,
		Balances: []domain.Balance{{Label: "Remaining prepaid credit balance", Value: &amount, Unit: "USD"}},
	}}
	view := checkFrame(t, m, 160, 48)
	if !strings.Contains(view, "12.34 USD") {
		t.Fatalf("long balance label hid amount:\n%s", view)
	}
}

func TestColoredNavigationUsesDisplayCellWidth(t *testing.T) {
	previous := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	defer func() { lipgloss.Writer.Profile = previous }()
	m := demoModel(t, demo.ScenarioSubscription)
	view := checkFrame(t, m, 100, 32)
	if !strings.Contains(view, "[ All tracked ]") {
		t.Fatalf("colored navigation clipped:\n%s", view)
	}
}

func TestCardRefreshShowsSchedulingStateAndStatus(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Layout = config.LayoutCards
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "Fetching") || !strings.Contains(view, "Fetching…") {
		t.Fatalf("card refresh stage/status missing:\n%s", view)
	}
	for i := 0; i < 100 && m.refreshing; i++ {
		m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	}
	for _, snapshot := range m.all {
		if snapshot.State == domain.StatusQueued || snapshot.State == domain.StatusFetching {
			t.Fatalf("%s stranded after card refresh", snapshot.Key)
		}
	}
}

func TestBalanceFreshnessUsesItsOwnSampleTime(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	amount := 12.34
	balanceSample := m.now().Add(-48 * time.Hour)
	snapshotSample := m.now().Add(-34 * time.Second)
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "credit"}, DisplayName: "Credit", State: domain.StatusCurrent,
		SampleAt: &snapshotSample,
		Balances: []domain.Balance{{Label: "Credits", Value: &amount, Unit: "USD", SampleAt: &balanceSample}},
	}}
	cards := checkFrame(t, m, 120, 40)
	if !strings.Contains(cards, "sampled 2d") {
		t.Fatalf("card ignored balance sample time:\n%s", cards)
	}
	compact := checkFrame(t, m, 80, 24)
	if !strings.Contains(compact, "2d") {
		t.Fatalf("compact ignored balance sample time:\n%s", compact)
	}
	m.screen = screenDetails
	details := checkFrame(t, m, 120, 40)
	if !strings.Contains(details, "sampled 2d") {
		t.Fatalf("details ignored balance sample time:\n%s", details)
	}
}

func TestRefreshFooterKeepsEssentialHintsAt80(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	view := checkFrame(t, m, 80, 24)
	for _, hint := range []string{"Details", "Help", "q Quit"} {
		if !strings.Contains(view, hint) {
			t.Fatalf("refresh footer lost %q:\n%s", hint, view)
		}
	}
}

func TestLeavingSettingsThroughHelpRestoresCommittedTheme(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Theme = config.ThemeGreen
	m.applyTheme()
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.settingsDraft.Theme == config.ThemeGreen {
		t.Fatal("theme preview did not change")
	}
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenSettings {
		t.Fatalf("Help did not return to Settings: %v", m.screen)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if m.prefs.Theme != config.ThemeGreen {
		t.Fatalf("persisted theme changed to %s", m.prefs.Theme)
	}
	want := theme.New(string(config.ThemeGreen), m.light, m.noColor || m.prefs.Monochrome)
	if !reflect.DeepEqual(m.theme, want) {
		t.Fatal("committed theme not restored after leaving Settings via Help")
	}
}

func TestLeavingSettingsThroughPickerRestoresCommittedTheme(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Theme = config.ThemeGreen
	m.applyTheme()
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if m.screen != screenPicker {
		t.Fatalf("p did not open picker from settings: %v", m.screen)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenSettings {
		t.Fatalf("picker did not return to Settings: %v", m.screen)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if m.prefs.Theme != config.ThemeGreen {
		t.Fatalf("persisted theme changed to %s", m.prefs.Theme)
	}
	want := theme.New(string(config.ThemeGreen), m.light, m.noColor || m.prefs.Monochrome)
	if !reflect.DeepEqual(m.theme, want) {
		t.Fatal("committed theme not restored after leaving Settings via picker")
	}
}

func TestFractionalOverLimitPreservedAtRequiredSizes(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	snapshot := m.visible[0]
	for _, percent := range []float64{100.1, 137.25} {
		p := percent
		snapshot.Windows = []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &p}}
		m.visible = []domain.UsageSnapshot{sanitizeSnapshot(snapshot)}
		m.screen = screenDetails
		want := fmt.Sprintf("%g%% used", percent)
		for _, size := range [][2]int{{120, 40}, {80, 24}, {60, 20}} {
			seen := ""
			for offset := 0; offset < 40; offset++ {
				m.detailScroll = offset
				seen += cleanText(checkFrame(t, m, size[0], size[1]))
			}
			if !strings.Contains(seen, want) || !strings.Contains(seen, "0% left") || !strings.Contains(seen, "exceeds 100") {
				t.Fatalf("%dx%d lost fractional over-limit %s", size[0], size[1], want)
			}
		}
	}
}

func TestCompactPaginationAccountsForCurrentPageRows(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	percent := 20.0
	sample := m.now().Add(-42 * time.Minute)
	for i := range m.visible {
		m.visible[i].Windows = []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}
		m.visible[i].SampleAt = &sample
		m.visible[i].Quality = domain.QualityStale
		m.visible[i] = sanitizeSnapshot(m.visible[i])
	}
	m.all = append([]domain.UsageSnapshot(nil), m.visible...)
	m.selected = 0
	m.selectedKey = m.visible[0].Key.String()
	m.width, m.height = 80, 24
	if size := m.pageSize(); size < 2 {
		t.Fatalf("retained rows collapsed page to %d source(s)", size)
	}
}

func TestPickerAvailabilityVisibleAtBreakpoints(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	if len(m.catalog.Providers) < 2 {
		t.Fatal("fixture lacks catalog entries")
	}
	m.catalog.Providers[0].Available = true
	m.catalog.Providers[1].Available = false
	m.screen = screenPicker
	entries := m.pickerEntries()
	checked := map[bool]bool{}
	for index, descriptor := range entries {
		if checked[descriptor.Available] {
			continue
		}
		m.pickerIndex = index
		view := checkFrame(t, m, 100, 32)
		want := "available"
		if !descriptor.Available {
			want = "unavailable"
		}
		if !strings.Contains(view, want) {
			t.Fatalf("availability %q missing for %s at 100x32", want, descriptor.DisplayName)
		}
		checked[descriptor.Available] = true
	}
	if !checked[true] || !checked[false] {
		t.Fatal("fixture lacks both availability states")
	}
}

func TestCompactMixedPaginationMatchesFooterAndKeepsContext(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	percent := 20.0
	sample := m.now().Add(-42 * time.Minute)
	m.visible = nil
	for i := 0; i < 39; i++ {
		snapshot := domain.UsageSnapshot{
			Key: domain.SourceKey{Provider: fmt.Sprintf("source%02d", i)}, DisplayName: fmt.Sprintf("S%02d", i), State: domain.StatusCurrent,
			Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}},
		}
		if i%2 == 0 {
			snapshot.SampleAt = &sample
			snapshot.Quality = domain.QualityStale
		}
		m.visible = append(m.visible, sanitizeSnapshot(snapshot))
	}
	m.all = append([]domain.UsageSnapshot(nil), m.visible...)
	m.selected = 16
	m.selectedKey = m.visible[16].Key.String()
	for _, size := range [][2]int{{80, 24}, {60, 20}} {
		m.width, m.height = size[0], size[1]
		start, end := pageStart(m), pageEnd(m)
		view := checkFrame(t, m, size[0], size[1])
		if end-start <= 1 {
			t.Fatalf("%dx%d page collapsed to one source", size[0], size[1])
		}
		for i := start; i < end; i++ {
			if !strings.Contains(view, fmt.Sprintf("S%02d", i)) {
				t.Fatalf("%dx%d footer range %d-%d omitted S%02d", size[0], size[1], start, end, i)
			}
		}
		if !strings.Contains(view, "S16") || !strings.Contains(view, "prior data") {
			t.Fatalf("%dx%d lost selected context/retained age:\n%s", size[0], size[1], view)
		}
	}
}

func TestThemeSizeSmokeMatrix(t *testing.T) {
	previous := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	defer func() { lipgloss.Writer.Profile = previous }()
	themes := []config.ThemeName{config.ThemeGreen, config.ThemeBlue, config.ThemeAmber, config.ThemeMonokai}
	sizes := [][2]int{{160, 48}, {120, 40}, {100, 32}, {80, 24}, {60, 20}}
	screens := []screen{screenOverview, screenDetails, screenHistory, screenPicker, screenSettings, screenHelp}
	for _, themeName := range themes {
		for _, light := range []bool{false, true} {
			for _, mono := range []bool{false, true} {
				for _, size := range sizes {
					for _, target := range screens {
						m := demoModel(t, demo.ScenarioSubscription)
						m.prefs.Theme = themeName
						m.prefs.Monochrome = mono
						m.light = light
						m.noColor = mono
						m.applyTheme()
						m.screen = target
						if target == screenHistory {
							m.selectMuse()
							m.loadHistory()
						}
						_ = checkFrame(t, m, size[0], size[1])
					}
				}
			}
		}
	}
}

func TestActiveProbeSourceShowsOutsidePrototype(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.catalog.Providers = []domain.SourceDescriptor{{
		Key: domain.SourceKey{Provider: "azureopenai"}, ProviderID: "azureopenai", DisplayName: "Azure OpenAI",
		Kind: domain.KindMeteredAPI, Available: true,
		Capabilities: domain.Capabilities{Usage: domain.CapabilityUnknown, ActiveProbe: domain.CapabilitySupported},
	}}
	m.visible = nil
	m.screen = screenPicker
	m.pickerIndex = 0
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "Outside prototype · active probe") {
		t.Fatalf("active probe not labelled:\n%s", view)
	}
}
