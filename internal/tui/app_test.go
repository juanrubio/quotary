package tui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/juanrubio/quotary/internal/backend/demo"
	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/domain"
	"github.com/juanrubio/quotary/internal/theme"
)

func TestQuitKeys(t *testing.T) {
	// q asks for confirmation; Ctrl+C still quits immediately.
	m := &model{}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd != nil || !m.quitPrompt || m.quitSelected {
		t.Fatal("q should open the quit prompt with Cancel selected")
	}
	m = &model{}
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c should return a quit command")
	}
}

func TestDemoViewIsLabeled(t *testing.T) {
	m := &model{demo: true}
	view := m.View().Content
	if !strings.Contains(view, "DEMO") || !strings.Contains(view, "fictional data") {
		t.Fatalf("demo label missing: %q", view)
	}
}

func TestResize(t *testing.T) {
	m := &model{}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.width != 80 || m.height != 24 {
		t.Fatalf("size = %dx%d", m.width, m.height)
	}
}

func TestTeatestShellPaintsAndQuits(t *testing.T) {
	tm := teatest.NewTestModel(t, &model{demo: true}, teatest.WithInitialTermSize(80, 24))
	t.Cleanup(func() { _ = tm.Quit() })
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("DEMO"))
	}, teatest.WithCheckInterval(25*time.Millisecond), teatest.WithDuration(5*time.Second))
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.Send(tea.KeyPressMsg{Code: 'y', Text: "y"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
	if _, err := io.ReadAll(tm.FinalOutput(t)); err != nil {
		t.Fatal(err)
	}
}

func TestDemoModelUsesBackendAndSwitchesScenarios(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioSubscription, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	m.width, m.height = 120, 40
	view := m.View().Content
	for _, want := range []string{"DEMO · fictional data", "Scenario: subscription", "Claude Code", "24 sources"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	view = m.View().Content
	if !strings.Contains(view, "Scenario: attention") || !strings.Contains(view, "Exhausted") {
		t.Fatalf("attention scenario not painted:\n%s", view)
	}
	m.Update(tea.KeyPressMsg{Code: '4', Text: "4"})
	if !strings.Contains(m.View().Content, "No tracked sources") {
		t.Fatalf("empty scenario not painted:\n%s", m.View().Content)
	}
}

func TestDemoRefreshTransitionsVisible(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioSubscription, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if !strings.Contains(m.View().Content, "Queued") {
		t.Fatalf("queued state not visible:\n%s", m.View().Content)
	}
	m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	if !strings.Contains(m.View().Content, "Fetching") {
		t.Fatalf("fetching state not visible:\n%s", m.View().Content)
	}
	m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	if strings.Contains(m.View().Content, "Queued") || strings.Contains(m.View().Content, "Fetching") {
		t.Fatalf("refresh did not return to current:\n%s", m.View().Content)
	}
}

func TestDemoSetupScenarioShowsBackendMissing(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioSetup, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	m.width, m.height = 100, 32
	view := m.View().Content
	if !strings.Contains(view, "Setup · no backend connection") || !strings.Contains(view, "CodexBar could not be found") {
		t.Fatalf("setup view:\n%s", view)
	}
}

func TestRefreshCompletesOriginalSourceAfterNavigation(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioSubscription, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	m.width, m.height = 120, 40
	original := m.visible[0].Key.String()
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	generation := m.refreshGen
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.Update(refreshTickMsg{key: original, generation: generation})
	m.Update(refreshTickMsg{key: original, generation: generation})
	if stage := runtime.RefreshStage(original); stage != "current" {
		t.Fatalf("original source stranded in %s", stage)
	}
}

func TestScenarioSwitchInvalidatesPendingRefresh(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioSubscription, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	original := m.visible[0].Key.String()
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	generation := m.refreshGen
	m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	m.Update(refreshTickMsg{key: original, generation: generation})
	for _, snapshot := range runtime.Snapshots(context.Background()) {
		if snapshot.State == domain.StatusQueued || snapshot.State == domain.StatusFetching {
			t.Fatalf("stale refresh affected new scenario: %s", snapshot.Key)
		}
	}
}

func TestDemoNoResultsScenarioPreservesQuery(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioNoResults, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	view := m.View().Content
	if !strings.Contains(view, "No results for") || !strings.Contains(view, runtime.Query()) {
		t.Fatalf("no-results view:\n%s", view)
	}
}

func TestSelectionIdentitySurvivesSortResizeAndPagination(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selected = 12
	m.selectedKey = m.visible[m.selected].Key.String()
	want := m.selectedKey
	for _, size := range [][2]int{{160, 48}, {120, 40}, {100, 32}, {80, 24}, {60, 20}} {
		_ = checkFrame(t, m, size[0], size[1])
		if m.selectedKey != want {
			t.Fatalf("resize %v changed selection %s -> %s", size, want, m.selectedKey)
		}
		start, end := pageStart(m), pageEnd(m)
		if m.selected < start || m.selected >= end {
			t.Fatalf("resize %v lost selected source outside page %d-%d", size, start, end)
		}
	}
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	if m.selectedKey != want {
		t.Fatalf("sort changed selection %s -> %s", want, m.selectedKey)
	}
}

func TestGridNavigationAndReverseTab(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.width, m.height = 160, 48
	step := m.overviewStep()
	if step < 2 {
		t.Fatalf("card grid step = %d", step)
	}
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.selected != 1 {
		t.Fatalf("j selected %d, want 1", m.selected)
	}
	m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.selected != 0 {
		t.Fatalf("k selected %d", m.selected)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.selected != step {
		t.Fatalf("down arrow selected %d, want %d", m.selected, step)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.selected != 0 {
		t.Fatalf("up arrow selected %d", m.selected)
	}
	m.screen = screenDetails
	m.detailTab = 1
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.detailTab != 0 {
		t.Fatalf("shift+tab left tab %d", m.detailTab)
	}
}

func TestPickerPageKeysDoNotMoveOverviewSelection(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.selected = 3
	m.selectedKey = m.visible[3].Key.String()
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	before := m.selected
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if m.selected != before {
		t.Fatalf("picker paging moved overview selection %d -> %d", before, m.selected)
	}
	if m.pickerIndex == 0 {
		t.Fatal("picker paging did not move picker index")
	}
}

func TestSheetsRestoreOriginScreenAndPosition(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenDetails
	m.detailTab = 1
	m.loadHistory()
	m.detailScroll = 7
	m.historyScroll = 3
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenDetails || m.detailTab != 1 || m.detailScroll != 7 || m.historyScroll != 3 {
		t.Fatalf("settings lost origin: screen=%v tab=%d scroll=%d/%d", m.screen, m.detailTab, m.detailScroll, m.historyScroll)
	}
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenDetails {
		t.Fatalf("picker returned to %v", m.screen)
	}
}

func TestSettingsIntervalAndPathEditors(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	for m.settingsIdx != 3 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	before := m.settingsDraft.IntervalSeconds
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.settingsDraft.IntervalSeconds != before+60 {
		t.Fatalf("interval = %d", m.settingsDraft.IntervalSeconds)
	}
	for m.settingsIdx != 6 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	m.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.settingsDraft.BackendPath != "/tmp" {
		t.Fatalf("backend path = %q", m.settingsDraft.BackendPath)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.prefs.BackendPath != "/tmp" || m.prefs.IntervalSeconds != before+60 {
		t.Fatalf("settings not saved: %+v", m.prefs)
	}
}

func TestLayoutSaveFailureIsTransactional(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	session := &failingSaveSession{Session: m.session, fail: true}
	m.session = session
	before := m.prefs.Layout
	m.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if m.prefs.Layout != before {
		t.Fatal("failed layout save changed active layout")
	}
	if m.errorText == "" {
		t.Fatal("failed layout save did not report an error")
	}
}

func TestHistoryUppercaseRRefreshesAllSources(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.loadHistory()
	m.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	if !m.refreshAll || !m.refreshing {
		t.Fatalf("R in history did not queue all: all=%v refreshing=%v", m.refreshAll, m.refreshing)
	}
}

func TestSortExplanationVisible(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	wide := checkFrame(t, m, 120, 40)
	if !strings.Contains(wide, "Soonest reset") || !strings.Contains(wide, "earliest reported window") {
		t.Fatalf("sort rule not explained:\n%s", wide)
	}
	narrow := checkFrame(t, m, 80, 24)
	if !strings.Contains(narrow, "Reset↑") {
		t.Fatalf("narrow sort indicator missing:\n%s", narrow)
	}
}

func TestSettingsVisibleMappingMatchesHandlers(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	for m.settingsIdx != 4 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	privacy := m.settingsDraft.Privacy
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.settingsDraft.Privacy == privacy {
		t.Fatal("privacy row did not toggle privacy")
	}
	privacy = m.settingsDraft.Privacy
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	mono := m.settingsDraft.Monochrome
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.settingsDraft.Monochrome == mono || m.settingsDraft.Privacy != privacy {
		t.Fatal("monochrome row toggled the wrong control")
	}
	view := checkFrame(t, m, 80, 24)
	for _, label := range []string{"Interval", "Backend path", "Config path"} {
		if !strings.Contains(view, label) {
			t.Fatalf("settings missing %q control:\n%s", label, view)
		}
	}
}

func TestPickerFilteredPagingClampsAndActivates(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.catalog.Providers = m.catalog.Providers[:6]
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if len(m.pickerEntries()) == 0 || len(m.pickerEntries()) >= 26 {
		t.Fatalf("unexpected fixture size %d", len(m.pickerEntries()))
	}
	last := len(m.pickerEntries()) - 1
	m.pickerIndex = last
	m.movePicker(-100)
	before := m.pickerEntries()[0].Tracked
	if m.pickerIndex < 0 || m.pickerIndex >= len(m.pickerEntries()) {
		t.Fatalf("picker index out of range: %d", m.pickerIndex)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.pickerEntries()[m.pickerIndex].Tracked == before {
		t.Fatal("enter did not activate the clamped picker row")
	}
}

func TestSpaceTogglesPickerTrackingAndOverviewPin(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	descriptor := m.pickerEntries()[m.pickerIndex]
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	updated := m.pickerEntries()[m.pickerIndex]
	if descriptor.Tracked == updated.Tracked {
		t.Fatal("space did not toggle picker tracking")
	}
	m.screen = screenOverview
	m.filterVisible()
	if len(m.visible) == 0 {
		t.Fatal("no overview source after picker")
	}
	key := m.visible[0].Key.String()
	pinned := contains(m.prefs.Pins, key)
	m.selected = 0
	m.selectedKey = key
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if contains(m.prefs.Pins, key) == pinned {
		t.Fatal("space did not toggle overview pin")
	}
}

func TestNestedSheetsRestoreFullOriginChain(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenDetails
	m.detailTab = 1
	m.detailScroll = 5
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenSettings {
		t.Fatalf("picker closed to %v, want settings", m.screen)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenDetails || m.detailTab != 1 || m.detailScroll != 5 {
		t.Fatalf("nested sheets lost origin: %v tab=%d scroll=%d", m.screen, m.detailTab, m.detailScroll)
	}
}

func TestSettingsSaveReturnsToOrigin(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.screen = screenDetails
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.screen != screenDetails {
		t.Fatalf("settings save returned to %v", m.screen)
	}
}

func TestHistoryTabMovesToDetailsScreen(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenHistory
	m.loadHistory()
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.screen != screenDetails || m.detailTab != 0 {
		t.Fatalf("tab from history -> %v tab=%d", m.screen, m.detailTab)
	}
	m.screen = screenHistory
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if m.screen != screenDetails {
		t.Fatalf("shift+tab from history -> %v", m.screen)
	}
}

func TestSettingsPrivacyMasksAndSanitizesPaths(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Privacy = true
	m.prefs.BackendPath = "/secret/backend\x1b]0;pwned\a"
	m.prefs.BackendConfigPath = "/secret/config"
	m.settingsDraft = m.prefs
	m.screen = screenSettings
	view := checkFrame(t, m, 80, 24)
	if strings.Contains(view, "/secret") || strings.Contains(view, "pwned") || !strings.Contains(view, "Hidden · privacy mode") {
		t.Fatalf("settings leaked or failed to mask paths:\n%s", view)
	}
}

func TestSettingsPathEditorKeepsCursorVisible(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Privacy = false
	m.settingsDraft = m.prefs
	m.screen = screenSettings
	m.settingsIdx = 6
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.settingsInput = "/very/long/path/" + strings.Repeat("segment/", 20)
	m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	view := checkFrame(t, m, 80, 24)
	if !strings.Contains(view, "x▌") {
		t.Fatalf("path editor cursor/new character not visible:\n%s", view)
	}
}

func TestSettingsPageKeysDoNotMoveOverview(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenDetails
	m.detailScroll = 7
	beforeKey, beforeScroll := m.selectedKey, m.detailScroll
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.selectedKey != beforeKey || m.detailScroll != beforeScroll {
		t.Fatalf("settings paging changed origin: key=%s scroll=%d", m.selectedKey, m.detailScroll)
	}
}

func TestPickerPagingCoversEveryEntryAt60(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.width, m.height = 60, 20
	m.screen = screenPicker
	seen := map[int]bool{}
	for page := 0; page < 200; page++ {
		start, end := m.pickerViewport(max(1, m.height-6))
		for i := start; i < end; i++ {
			seen[i] = true
		}
		if end >= len(m.pickerEntries()) {
			break
		}
		m.pickerIndex = end - 1
		m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		if m.pickerIndex < end {
			break
		}
	}
	if len(seen) != len(m.pickerEntries()) {
		t.Fatalf("paging covered %d of %d entries", len(seen), len(m.pickerEntries()))
	}
}

func TestFocusedSearchPreservesPrintableSpaces(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.KeyPressMsg{Code: 'G', Text: "Google "})
	if m.query != "Google " {
		t.Fatalf("query = %q", m.query)
	}
	m.searching = false
	m.filterVisible()
	if len(m.visible) == 0 {
		t.Fatal("multiword/trailing-space search filtered everything")
	}
	m.screen = screenPicker
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.KeyPressMsg{Code: 'G', Text: "Gemini "})
	if m.pickerQuery != "Gemini " {
		t.Fatalf("picker query = %q", m.pickerQuery)
	}
}

func TestFailedRefreshReportsFailureAndTransitions(t *testing.T) {
	m := demoModel(t, demo.ScenarioAttention)
	m.selectProvider("windsurf")
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	if state := m.visible[m.selected].State; state != domain.StatusFetching {
		t.Fatalf("affected source state = %v, want fetching", state)
	}
	_ = checkFrame(t, m, 120, 40)
	m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	key := m.visible[m.selected].Key.String()
	if outcome := m.session.RefreshOutcome(key); outcome == "Success" {
		t.Fatal("failed refresh reported success")
	}
	if !strings.Contains(checkFrame(t, m, 120, 40), "Sign-in") {
		t.Fatal("failure state missing after refresh")
	}
}

func TestSettingsEditingMasksOtherPath(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Privacy = true
	m.prefs.BackendPath = "/secret/backend"
	m.prefs.BackendConfigPath = "/secret/config"
	m.settingsDraft = m.prefs
	m.screen = screenSettings
	m.settingsIdx = 6
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	view := checkFrame(t, m, 80, 24)
	if strings.Contains(view, "/secret/config") {
		t.Fatalf("unedited config path revealed during backend edit:\n%s", view)
	}
}

func TestNestedPickerSelectionPreserved(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.screen = screenPicker
	m.pickerIndex = 5
	want := m.pickerEntries()[m.pickerIndex].Key.String()
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if m.screen != screenPicker || m.pickerEntries()[m.pickerIndex].Key.String() != want {
		t.Fatalf("nested picker lost selection: screen=%v index=%d", m.screen, m.pickerIndex)
	}
}

func TestSettingsHintsAndHelpExplainNavigation(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.screen = screenSettings
	for _, size := range [][2]int{{80, 24}, {60, 20}} {
		view := checkFrame(t, m, size[0], size[1])
		if !strings.Contains(view, "Tab Field") || !strings.Contains(view, "E Edit") {
			t.Fatalf("%dx%d settings hints incomplete:\n%s", size[0], size[1], view)
		}
	}
	m.screen = screenHelp
	help := ""
	for page := 0; page < 10; page++ {
		help += checkFrame(t, m, 80, 24)
		if m.helpScroll >= m.helpMaxScroll() {
			break
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	for _, text := range []string{"Choose a theme", "Edit a backend path"} {
		if !strings.Contains(help, text) {
			t.Fatalf("help missing %q", text)
		}
	}
}

func TestRejectedPathSaveRestoresCommittedTheme(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Theme = config.ThemeGreen
	m.applyTheme()
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	for m.settingsIdx != 6 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.settingsInput = ""
	for _, r := range "relative/path" {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.prefs.Theme != config.ThemeGreen {
		t.Fatalf("committed theme changed to %s", m.prefs.Theme)
	}
	want := theme.New(string(config.ThemeGreen), m.light, m.noColor || m.prefs.Monochrome)
	if !reflect.DeepEqual(m.theme, want) {
		t.Fatal("failed path save kept preview theme active")
	}
	if m.errorText == "" || m.screen != screenSettings {
		t.Fatalf("rejected save error=%q screen=%v", m.errorText, m.screen)
	}
}

type failingTrackSession struct {
	Session
	failEvery int
	calls     int
}

func (f *failingTrackSession) Track(key string, tracked bool) error {
	f.calls++
	if f.failEvery > 0 && f.calls%f.failEvery == 0 {
		return errors.New("track save failed")
	}
	return f.Session.Track(key, tracked)
}

func TestSettingsSavePreservesNestedPickerTracking(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	untracked := -1
	for i, descriptor := range m.pickerEntries() {
		if !descriptor.Tracked {
			untracked = i
			break
		}
	}
	if untracked < 0 {
		t.Fatal("fixture has no untracked source")
	}
	m.pickerIndex = untracked
	key := m.pickerEntries()[untracked].Key.String()
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenSettings {
		t.Fatalf("picker returned to %v", m.screen)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !contains(m.prefs.Tracked, key) {
		t.Fatalf("settings save dropped nested tracking for %s", key)
	}
}

func TestNestedPickerPreservesUnsavedSettingsDraft(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	want := m.settingsDraft.Theme
	if want == m.prefs.Theme {
		t.Fatal("theme draft did not change")
	}
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenSettings || m.settingsDraft.Theme != want {
		t.Fatalf("settings draft lost: screen=%v theme=%s want=%s", m.screen, m.settingsDraft.Theme, want)
	}
}

func TestSpaceOnlyPinsOnOverview(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	key := m.visible[0].Key.String()
	before := contains(m.prefs.Pins, key)
	for _, target := range []screen{screenSettings, screenHelp, screenDetails} {
		m.screen = target
		m.selected = 0
		m.selectedKey = key
		m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
		if contains(m.prefs.Pins, key) != before {
			t.Fatalf("space changed pin on screen %v", target)
		}
	}
}

func TestBulkTrackingReportsFailureAndPartialSuccess(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	for i := range m.catalog.Providers {
		if m.catalog.Providers[i].Kind == domain.KindSubscription {
			m.catalog.Providers[i].Tracked = false
		}
	}
	session := &failingTrackSession{Session: m.session, failEvery: 1}
	m.session = session
	m.trackAllSubscriptions()
	if m.errorText == "" || strings.Contains(m.status, "Tracked") {
		t.Fatalf("all-failed bulk tracking reported status=%q error=%q", m.status, m.errorText)
	}
	m = demoModel(t, demo.ScenarioCatalog)
	for i := range m.catalog.Providers {
		if m.catalog.Providers[i].Kind == domain.KindSubscription {
			m.catalog.Providers[i].Tracked = false
		}
	}
	session = &failingTrackSession{Session: m.session, failEvery: 2}
	m.session = session
	m.trackAllSubscriptions()
	if !strings.Contains(m.errorText, "could not be saved") {
		t.Fatalf("partial bulk tracking error = %q", m.errorText)
	}
}

func TestHelpReturnsToPickerContext(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m.pickerIndex = 4
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if m.screen != screenHelp {
		t.Fatalf("help screen = %v", m.screen)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenPicker || m.pickerIndex != 4 {
		t.Fatalf("help returned to %v index=%d", m.screen, m.pickerIndex)
	}
}

func TestAttentionRefreshKeepsSelectedIdentity(t *testing.T) {
	m := demoModel(t, demo.ScenarioAttention)
	m.filter = "attention"
	m.filterVisible()
	m.selectProvider("warp")
	want := m.selectedKey
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	for i := 0; i < 100 && m.refreshing; i++ {
		m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	}
	if m.selectedKey != want {
		t.Fatalf("attention refresh selected %s, want %s", m.selectedKey, want)
	}
	found := false
	for _, snapshot := range m.visible {
		if snapshot.Key.String() == want {
			found = true
		}
	}
	if !found {
		t.Fatal("refreshed attention source vanished from the filter")
	}
}

func TestHelpPagingDoesNotMoveUnderlyingSource(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenDetails
	m.detailScroll = 7
	key := m.selectedKey
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenDetails || m.selectedKey != key || m.detailScroll != 7 || m.helpScroll == 0 {
		t.Fatalf("help paging leaked: screen=%v key=%s detail=%d help=%d", m.screen, m.selectedKey, m.detailScroll, m.helpScroll)
	}
}

func TestDetailsHistoryTabScrollsHistory(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectMuse()
	m.screen = screenDetails
	m.detailTab = 1
	m.loadHistory()
	before := m.historyScroll
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.historyScroll != before+1 || m.detailScroll != 0 {
		t.Fatalf("history tab scroll: history=%d detail=%d", m.historyScroll, m.detailScroll)
	}
	m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if m.historyScroll != before {
		t.Fatalf("history tab up scrolled to %d", m.historyScroll)
	}
}

func TestPickerHelpRoundTripPreservesSelection(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m.pickerIndex = 4
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if m.screen != screenPicker || m.pickerIndex != 4 {
		t.Fatalf("picker-help-picker lost context: screen=%v index=%d", m.screen, m.pickerIndex)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenOverview {
		t.Fatalf("picker did not close to overview, got %v", m.screen)
	}
}

func TestUppercaseEEditsPaths(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	for m.settingsIdx != 6 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m.Update(tea.KeyPressMsg{Code: 'E', Text: "E"})
	if !m.settingsEditing {
		t.Fatal("uppercase E did not start path editing")
	}
}

func TestBulkRefreshKeepsAttentionSelection(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.filter = "attention"
	m.filterVisible()
	m.selectProvider("warp")
	want := m.selectedKey
	m.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	for i := 0; i < 200 && m.refreshing; i++ {
		m.Update(refreshTickMsg{generation: m.refreshGen})
		if m.selectedKey != want {
			t.Fatalf("bulk refresh changed selection at tick %d: %s", i, m.selectedKey)
		}
	}
	if m.selectedKey != want {
		t.Fatalf("bulk refresh selected %s, want %s", m.selectedKey, want)
	}
}

func TestRefreshingRetainedCardShowsPriorInMonochrome(t *testing.T) {
	m := demoModel(t, demo.ScenarioAttention)
	m.prefs.Monochrome = true
	m.noColor = true
	m.applyTheme()
	m.selectProvider("cursor")
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "Fetching") || !strings.Contains(view, "prior") {
		t.Fatalf("retained refresh lost prior-data cue in monochrome:\n%s", view)
	}
}

func TestPickerRefreshUsesFocusedSource(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	index := -1
	var want string
	for i, descriptor := range m.pickerEntries() {
		if descriptor.Key.Provider == "codex" {
			index = i
			want = descriptor.Key.String()
			break
		}
	}
	if index < 0 {
		t.Fatal("fixture has no codex picker source")
	}
	m.pickerIndex = index
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if m.refreshKey != want {
		t.Fatalf("picker refresh targeted %s, want %s", m.refreshKey, want)
	}
}

func TestStaleRefreshDetailsKeepPriorAndAge(t *testing.T) {
	m := demoModel(t, demo.ScenarioAttention)
	m.selectProvider("cursor")
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	m.screen = screenDetails
	view := checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "prior") || !strings.Contains(view, "42m ago") {
		t.Fatalf("stale refresh lost prior/age in narrow details:\n%s", view)
	}
}

func TestRefreshSelectionPreservationRespectsSearch(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.query = "zzzz-does-not-exist"
	m.filterVisible()
	m.selectedKey = m.all[0].Key.String()
	m.selected = 0
	m.refreshing = true
	m.filterVisible()
	if len(m.visible) != 0 {
		t.Fatalf("refresh preservation bypassed search: %d visible", len(m.visible))
	}
}

func TestSelectedRefreshDuringBulkIsNotStranded(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectProvider("warp")
	m.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	m.Update(refreshTickMsg{generation: m.refreshGen})
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	for i := 0; i < 200 && m.refreshing; i++ {
		if m.refreshAll {
			m.Update(refreshTickMsg{generation: m.refreshGen})
		} else {
			m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
		}
	}
	for _, snapshot := range m.all {
		if snapshot.State == domain.StatusQueued || snapshot.State == domain.StatusFetching {
			t.Fatalf("%s stranded after overlapping bulk/selected refresh", snapshot.Key)
		}
	}
}

func TestSettingsDraftSuspendsThroughHelpAndCancels(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	want := m.settingsDraft.Theme
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	if m.screen != screenSettings || m.settingsDraft.Theme != want {
		t.Fatalf("settings draft lost after help round trip: screen=%v theme=%s", m.screen, m.settingsDraft.Theme)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenSettings || m.settingsDraft.Theme != m.prefs.Theme || m.settingsDirty {
		t.Fatalf("cancelled nested settings retained draft: screen=%v theme=%s dirty=%v", m.screen, m.settingsDraft.Theme, m.settingsDirty)
	}
}

func TestCancelledSettingsDraftNotSavedAfterPickerOrigin(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Theme = config.ThemeGreen
	m.applyTheme()
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.prefs.Theme != config.ThemeGreen {
		t.Fatalf("cancelled draft was saved as %s", m.prefs.Theme)
	}
}

func TestRetainedBalanceDuringFetchingShowsPriorEverywhere(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	amount := 12.34
	balanceSample := m.now().Add(-48 * time.Hour)
	sample := m.now().Add(-10 * time.Second)
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "credit"}, DisplayName: "Credit", State: domain.StatusFetching,
		SampleAt: &sample, Balances: []domain.Balance{{Label: "Credits", Value: &amount, Unit: "USD", SampleAt: &balanceSample}},
	}}
	m.selected = 0
	m.selectedKey = m.visible[0].Key.String()
	if !strings.Contains(checkFrame(t, m, 120, 40), "prior") {
		t.Fatal("card lost prior cue for old balance during fetching")
	}
	if !strings.Contains(checkFrame(t, m, 80, 24), "prior") {
		t.Fatal("compact lost prior cue for old balance during fetching")
	}
	m.screen = screenDetails
	if !strings.Contains(checkFrame(t, m, 120, 40), "prior") {
		t.Fatal("details lost prior cue for old balance during fetching")
	}
}

func TestReceiptTimeFallbackContributesToRetainedDetection(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	percent := 20.0
	received := m.now().Add(-48 * time.Hour)
	m.visible = []domain.UsageSnapshot{{
		Key: domain.SourceKey{Provider: "receipt"}, DisplayName: "Receipt", State: domain.StatusQueued,
		ReceivedAt: received, Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}},
	}}
	m.selected = 0
	m.selectedKey = m.visible[0].Key.String()
	if !strings.Contains(checkFrame(t, m, 120, 40), "prior") {
		t.Fatal("old receipt without sample time lost prior cue")
	}
}

func TestScenarioSwitchClearsBulkRefreshState(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	m.Update(refreshTickMsg{generation: m.refreshGen})
	m.switchScenario("2")
	if m.refreshing || m.refreshAll || len(m.refreshAllKeys) != 0 || m.refreshKey != "" {
		t.Fatalf("scenario switch kept refresh state: refreshing=%v all=%v keys=%d key=%q", m.refreshing, m.refreshAll, len(m.refreshAllKeys), m.refreshKey)
	}
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	for i := 0; i < 100 && m.refreshing; i++ {
		m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	}
	for _, snapshot := range m.all {
		if snapshot.State == domain.StatusQueued || snapshot.State == domain.StatusFetching {
			t.Fatalf("%s stranded after scenario switch", snapshot.Key)
		}
	}
}

func TestRestartingSearchRestoresFullCollection(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.KeyPressMsg{Code: 'C', Text: "Codex"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.visible) >= len(m.all) {
		t.Fatalf("search did not reduce collection: %d/%d", len(m.visible), len(m.all))
	}
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(m.visible) != len(m.all) {
		t.Fatalf("restarting search left %d/%d sources", len(m.visible), len(m.all))
	}
}

func TestNestedSheetsPreserveOriginalPickerSelection(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	index := -1
	for i, descriptor := range m.pickerEntries() {
		if descriptor.Key.Provider == "cursor" {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatal("fixture has no cursor picker source")
	}
	m.pickerIndex = index
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if m.screen != screenPicker || m.pickerIndex != index {
		t.Fatalf("nested sheet lost picker: screen=%v index=%d want=%d", m.screen, m.pickerIndex, index)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenOverview {
		t.Fatalf("picker closed to %v", m.screen)
	}
}

func TestLayoutToggleUsesEffectiveAutoLayout(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Layout = config.LayoutAuto
	m.width, m.height = 100, 24
	m.toggleLayout()
	if m.prefs.Layout != config.LayoutCards {
		t.Fatalf("first toggle from auto-compact chose %s", m.prefs.Layout)
	}
	view := checkFrame(t, m, 100, 24)
	if !strings.Contains(view, "╭─") {
		t.Fatalf("toggle did not render cards:\n%s", view)
	}
}

func TestEnterThenTabCyclesDetailsTabs(t *testing.T) {
	// Enter has one job: open the selected source. Tab has a different job,
	// and only inside Details: cycle between Allowances and History.
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.screen != screenOverview {
		t.Fatalf("tab on overview moved to screen %v", m.screen)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.screen != screenDetails || m.detailTab != 0 {
		t.Fatalf("enter: screen=%v tab=%d", m.screen, m.detailTab)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.screen != screenDetails || m.detailTab != 1 {
		t.Fatalf("tab to history: screen=%v tab=%d", m.screen, m.detailTab)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.screen != screenDetails || m.detailTab != 0 {
		t.Fatalf("tab back to allowances: screen=%v tab=%d", m.screen, m.detailTab)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.screen != screenOverview {
		t.Fatalf("escape from details returned to %v", m.screen)
	}
}

func TestTopBarNavigationContext(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*model)
		width   int
		want    []string
		notWant []string
	}{
		{
			name:    "overview shows no source tabs or key hints",
			setup:   func(*model) {},
			width:   120,
			want:    []string{"[Overview]"},
			notWant: []string{"Details", "History", "/ Search", "f Filter", "p Sources"},
		},
		{
			name:    "details allowances keeps only overview in top bar",
			setup:   func(m *model) { m.screen = screenDetails },
			width:   120,
			want:    []string{"[Overview]"},
			notWant: []string{"Details", "History"},
		},
		{
			name:    "details history keeps only overview in top bar",
			setup:   func(m *model) { m.screen, m.detailTab = screenDetails, 1 },
			width:   120,
			want:    []string{"[Overview]"},
			notWant: []string{"Details", "History"},
		},
		{
			name:    "history screen keeps only overview in top bar",
			setup:   func(m *model) { m.screen = screenHistory },
			width:   120,
			want:    []string{"[Overview]"},
			notWant: []string{"Details", "History"},
		},
		{
			name:    "picker keeps only overview in top bar",
			setup:   func(m *model) { m.screen = screenPicker },
			width:   120,
			want:    []string{"[Overview]"},
			notWant: []string{"Details", "History"},
		},
		{
			name:    "narrow history keeps only overview in top bar",
			setup:   func(m *model) { m.screen, m.detailTab = screenDetails, 1 },
			width:   50,
			want:    []string{"[Overview]"},
			notWant: []string{"Details", "History"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := demoModel(t, demo.ScenarioSubscription)
			tc.setup(m)
			got := cleanText(m.navLine(tc.width))
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("top bar %q missing %q", got, want)
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(got, notWant) {
					t.Fatalf("top bar %q contains %q", got, notWant)
				}
			}
		})
	}
}

func TestDetailsKeyHintsLiveOnlyInBottomBar(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.screen = screenDetails
	details := cleanText(strings.Join(m.renderDetails(120, 40), "\n"))
	if !strings.Contains(details, "[Allowances]") || !strings.Contains(details, "History") {
		t.Fatalf("details area lost its Allowances/History tabs:\n%s", details)
	}
	if strings.Contains(details, "Esc Overview") {
		t.Fatalf("details area repeats a key hint outside the bottom bar:\n%s", details)
	}
	frame := checkFrame(t, m, 120, 40)
	if !strings.Contains(frame, "Esc Overview") {
		t.Fatalf("details bottom bar lost its Esc hint:\n%s", frame)
	}
}

func TestLayoutToggleForcesCardsOnNarrowTerminal(t *testing.T) {
	// Below the auto-layout width the overview falls back to the list, but
	// pressing l must still bring the cards back; it used to just re-select
	// the compact layout the user was trying to leave.
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Layout = config.LayoutAuto
	m.width, m.height = 60, 20
	m.toggleLayout()
	if m.prefs.Layout != config.LayoutCards {
		t.Fatalf("toggle on a narrow terminal chose %s", m.prefs.Layout)
	}
	view := checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "╭─") {
		t.Fatalf("toggle did not render cards on a narrow terminal:\\n%s", view)
	}
	m.toggleLayout()
	if m.prefs.Layout != config.LayoutCompact {
		t.Fatalf("second toggle chose %s", m.prefs.Layout)
	}
}

func TestUnwindingSettingsViaPickerCancelsDraft(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Theme = config.ThemeGreen
	m.applyTheme()
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	if m.screen != screenPicker {
		t.Fatalf("did not return to picker: %v", m.screen)
	}
	want := theme.New(string(m.prefs.Theme), m.light, m.noColor || m.prefs.Monochrome)
	if !reflect.DeepEqual(m.theme, want) || m.settingsDirty {
		t.Fatal("discarded settings preview survived picker unwinding")
	}
}

func TestIntervalSaveRecomputesStatesCountsAndFilter(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	sample := m.now().Add(-3 * time.Minute)
	percent := 20.0
	m.visible[0].SampleAt = &sample
	m.visible[0].Windows = []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}
	m.visible[0] = sanitizeSnapshot(m.visible[0])
	m.all = append([]domain.UsageSnapshot(nil), m.visible...)
	m.prefs.IntervalSeconds = 300
	m.filterVisible()
	m.selectedKey = m.all[0].Key.String()
	m.selected = 0
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	for m.settingsIdx != 3 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	for m.settingsDraft.IntervalSeconds > 60 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.all[0].State != domain.StatusStale {
		t.Fatalf("saved interval left state %v", m.all[0].State)
	}
	if counts := domain.CountStates(m.all); counts.Attention == 0 {
		t.Fatal("interval save left attention count at zero")
	}
	m.filter = "attention"
	m.filterVisible()
	found := false
	for _, snapshot := range m.visible {
		if snapshot.Key.String() == m.selectedKey {
			found = true
		}
	}
	if !found {
		t.Fatal("interval save did not update Attention filter membership")
	}
	baseline := domain.CountStates(m.all).Attention
	m.filter = "all"
	m.filterVisible()
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	for m.settingsIdx != 3 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	for m.settingsDraft.IntervalSeconds < 300 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.all[0].State == domain.StatusStale {
		t.Fatal("restoring interval left stale state sticky")
	}
	if counts := domain.CountStates(m.all); counts.Attention >= baseline {
		t.Fatalf("restored interval left attention count %d, baseline %d", counts.Attention, baseline)
	}
}

func TestPickerSearchPreservesSelectedSourceIdentity(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.screen = screenPicker
	index := -1
	for i, descriptor := range m.pickerEntries() {
		if descriptor.Key.Provider == "codex" {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatal("fixture has no codex source")
	}
	m.pickerIndex = index
	want := m.pickerEntries()[index].Key.String()
	m.setPickerQuery("c")
	if m.pickerEntries()[m.pickerIndex].Key.String() != want {
		t.Fatalf("search changed selection to %s", m.pickerEntries()[m.pickerIndex].Key)
	}
	m.setPickerQuery("")
	if m.pickerEntries()[m.pickerIndex].Key.String() != want {
		t.Fatalf("clearing search changed selection to %s", m.pickerEntries()[m.pickerIndex].Key)
	}
}

func TestPickerBackspacePreservesSourceIdentity(t *testing.T) {
	m := demoModel(t, demo.ScenarioCatalog)
	m.screen = screenPicker
	index := -1
	for i, descriptor := range m.pickerEntries() {
		if descriptor.Key.Provider == "codex" {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatal("fixture has no codex source")
	}
	m.pickerIndex = index
	m.selectedKey = m.pickerEntries()[index].Key.String()
	want := m.pickerEntries()[index].Key.String()
	m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	m.Update(tea.KeyPressMsg{Code: 'c', Text: "codex"})
	for i := 0; i < 5; i++ {
		m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	if m.pickerEntries()[m.pickerIndex].Key.String() != want {
		t.Fatalf("backspace changed picker selection to %s", m.pickerEntries()[m.pickerIndex].Key)
	}
}

func TestSelectionChangeReconcilesHistory(t *testing.T) {
	m := demoModel(t, demo.ScenarioAttention)
	m.selectProvider("warp")
	m.screen = screenHistory
	m.loadHistory()
	m.query = "Codex"
	m.filterVisible()
	selected, ok := m.selectedSnapshot()
	if !ok {
		t.Fatal("search left no selection")
	}
	if m.history.Key.String() != "" && m.history.Key.String() != selected.Key.String() {
		t.Fatalf("history still belongs to old source: %s vs %s", m.history.Key, selected.Key)
	}
	if strings.Contains(m.historyErr, "unsupported") {
		t.Fatalf("old source error retained: %q", m.historyErr)
	}
}

func TestUntrackingSelectedSourceReconcilesHistory(t *testing.T) {
	m := demoModel(t, demo.ScenarioAttention)
	m.selectProvider("warp")
	m.screen = screenHistory
	m.loadHistory()
	oldKey := m.selectedKey
	m.screen = screenPicker
	for i, descriptor := range m.pickerEntries() {
		if descriptor.Key.String() == oldKey {
			m.pickerIndex = i
			break
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if m.history.Key.String() == oldKey {
		t.Fatal("untracking selected source kept its history")
	}
}

func TestReturningFromPickerReconcilesHistoryAfterUntrack(t *testing.T) {
	for _, detailsTab := range []bool{false, true} {
		m := demoModel(t, demo.ScenarioSubscription)
		m.selectProvider("codex")
		if detailsTab {
			m.screen = screenDetails
			m.detailTab = 1
		} else {
			m.screen = screenHistory
		}
		m.loadHistory()
		oldKey := m.selectedKey
		m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
		for i, descriptor := range m.pickerEntries() {
			if descriptor.Key.String() == oldKey {
				m.pickerIndex = i
				break
			}
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		selected, ok := m.selectedSnapshot()
		if !ok {
			t.Fatal("no selected source after returning")
		}
		if m.history.Key.String() != "" && m.history.Key.String() != selected.Key.String() {
			t.Fatalf("history not reconciled: %s vs %s", m.history.Key, selected.Key)
		}
		if detailsTab && (m.screen != screenDetails || m.detailTab != 1) {
			t.Fatalf("details history context lost: screen=%v tab=%d", m.screen, m.detailTab)
		}
	}
}

func TestUnchangedSourceHistoryCacheSurvivesSheetRoundTrip(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectProvider("codex")
	m.screen = screenHistory
	m.loadHistory()
	session := &countingSession{Session: m.session}
	m.session = session
	m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if session.historyCalls != 0 {
		t.Fatalf("unchanged history cache caused %d extra loads", session.historyCalls)
	}
}

func TestSettingsRecomputePreservesExplicitNormalizedStates(t *testing.T) {
	for _, state := range []domain.SourceStatus{domain.StatusSignIn, domain.StatusUnavailable, domain.StatusStale, domain.StatusAuthFailed} {
		m := demoModel(t, demo.ScenarioSubscription)
		snapshot := m.visible[0]
		snapshot.State = state
		snapshot.Error = nil
		snapshot.Quality = domain.QualityMeasured
		m.visible = []domain.UsageSnapshot{sanitizeSnapshot(snapshot)}
		m.all = append([]domain.UsageSnapshot(nil), m.visible...)
		m.all[0].State = state
		m.baseStates = map[string]domain.SourceStatus{snapshot.Key.String(): state}
		m.selected = 0
		m.selectedKey = snapshot.Key.String()
		m.settingsDraft = m.prefs
		m.saveSettings()
		if m.all[0].State != state {
			t.Fatalf("unchanged settings converted %v to %v", state, m.all[0].State)
		}
	}
}

func TestExplicitStaleStateKeepsPriorWarning(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	sample := m.now().Add(-10 * time.Second)
	percent := 20.0
	snapshot := m.visible[0]
	snapshot.State = domain.StatusStale
	snapshot.Quality = domain.QualityMeasured
	snapshot.SampleAt = &sample
	snapshot.Windows = []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}
	m.visible = []domain.UsageSnapshot{sanitizeSnapshot(snapshot)}
	m.all = append([]domain.UsageSnapshot(nil), m.visible...)
	m.baseStates = map[string]domain.SourceStatus{snapshot.Key.String(): domain.StatusStale}
	m.selected = 0
	m.selectedKey = snapshot.Key.String()
	for _, size := range [][2]int{{120, 40}, {80, 24}} {
		if view := checkFrame(t, m, size[0], size[1]); !strings.Contains(view, "prior") {
			t.Fatalf("%dx%d explicit stale state lost prior warning", size[0], size[1])
		}
	}
	m.screen = screenDetails
	if view := checkFrame(t, m, 60, 20); !strings.Contains(view, "prior") || !strings.Contains(view, "10s ago") {
		t.Fatalf("narrow details lost explicit stale warning/age:\n%s", view)
	}
}

func TestSettingsSaveDuringRefreshPreservesSchedulingStates(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	queued := 0
	for _, snapshot := range m.all {
		if snapshot.State == domain.StatusQueued {
			queued++
		}
	}
	if queued == 0 {
		t.Fatal("bulk refresh did not queue sources")
	}
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	for _, snapshot := range m.all {
		if snapshot.State != domain.StatusQueued && snapshot.State != domain.StatusFetching {
			t.Fatalf("%s lost scheduling state during settings save: %v", snapshot.Key, snapshot.State)
		}
	}
	for i := 0; i < 200 && m.refreshing; i++ {
		m.Update(refreshTickMsg{generation: m.refreshGen})
	}
	for _, snapshot := range m.all {
		if snapshot.State == domain.StatusQueued || snapshot.State == domain.StatusFetching {
			t.Fatalf("%s stranded after settings save", snapshot.Key)
		}
	}
}

func TestExplicitAuthFailureRetainedDuringRefreshIsLabelled(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.prefs.Monochrome = true
	m.noColor = true
	m.applyTheme()
	sample := m.now().Add(-34 * time.Second)
	percent := 20.0
	snapshot := m.visible[0]
	snapshot.State = domain.StatusFetching
	snapshot.Quality = domain.QualityMeasured
	snapshot.SampleAt = &sample
	snapshot.Windows = []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}
	m.visible = []domain.UsageSnapshot{sanitizeSnapshot(snapshot)}
	m.all = append([]domain.UsageSnapshot(nil), m.visible...)
	m.baseStates = map[string]domain.SourceStatus{snapshot.Key.String(): domain.StatusAuthFailed}
	m.selected = 0
	m.selectedKey = snapshot.Key.String()
	m.screen = screenDetails
	view := checkFrame(t, m, 60, 20)
	if !strings.Contains(view, "prior") || !strings.Contains(view, "34s ago") {
		t.Fatalf("explicit auth-failure retention missing in narrow monochrome details:\n%s", view)
	}
}

func TestBulkTrackingDiscoverableInHelpAndPicker(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.screen = screenHelp
	help := ""
	for page := 0; page < 10; page++ {
		help += checkFrame(t, m, 60, 20)
		if m.helpScroll >= m.helpMaxScroll() {
			break
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if !strings.Contains(help, "Track all subscriptions") {
		t.Fatalf("Help omits bulk tracking shortcut:\n%s", help)
	}
	m.screen = screenPicker
	if view := checkFrame(t, m, 60, 20); !strings.Contains(view, "a Track all") {
		t.Fatalf("picker footer omits bulk tracking shortcut:\n%s", view)
	}
}

func TestHistoryRefreshDispatchesDuringUsageRefresh(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectProvider("codex")
	m.screen = screenHistory
	m.loadHistory()
	session := &countingSession{Session: m.session}
	m.session = session
	m.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	before := session.historyCalls
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if session.historyCalls <= before {
		t.Fatal("history refresh was suppressed during usage refresh")
	}
	for i := 0; i < 300 && m.refreshing; i++ {
		if m.refreshAll {
			m.Update(refreshTickMsg{generation: m.refreshGen})
		} else {
			m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
		}
	}
}

func TestBulkRequestDuringSelectedRefreshCompletesAllSources(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selectProvider("warp")
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	for i := 0; i < 400 && m.refreshing; i++ {
		if m.refreshAll {
			m.Update(refreshTickMsg{generation: m.refreshGen})
		} else {
			m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
		}
	}
	if m.refreshing {
		t.Fatal("pending bulk refresh did not complete")
	}
	for _, snapshot := range m.all {
		if snapshot.State == domain.StatusQueued || snapshot.State == domain.StatusFetching {
			t.Fatalf("%s stranded after queued bulk refresh", snapshot.Key)
		}
	}
}

func TestThreeRapidPendingSourceRefreshesComplete(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.selected = 0
	m.selectedKey = m.visible[0].Key.String()
	keys := []string{m.visible[0].Key.String(), m.visible[1].Key.String(), m.visible[2].Key.String()}
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	for i := 1; i < 3; i++ {
		m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	}
	for i := 0; i < 400 && m.refreshing; i++ {
		if m.refreshAll {
			m.Update(refreshTickMsg{generation: m.refreshGen})
		} else {
			m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
		}
	}
	if m.refreshing {
		t.Fatal("pending source refreshes did not complete")
	}
	for _, key := range keys {
		if outcome := m.session.RefreshOutcome(key); outcome != "Success" {
			t.Fatalf("%s outcome = %q", key, outcome)
		}
	}
}

func TestPickerPageKeysCoverEveryEntryAtAllSizes(t *testing.T) {
	for _, size := range [][2]int{{160, 48}, {120, 40}, {100, 32}, {80, 24}, {60, 20}} {
		m := demoModel(t, demo.ScenarioCatalog)
		m.screen = screenPicker
		m.width, m.height = size[0], size[1]
		seen := map[int]bool{}
		lastIndex := -1
		for page := 0; page < 300; page++ {
			start, end := m.pickerViewport(max(1, m.height-6))
			for i := start; i < end; i++ {
				seen[i] = true
			}
			if m.pickerIndex == lastIndex || m.pickerIndex >= len(m.pickerEntries())-1 {
				break
			}
			lastIndex = m.pickerIndex
			m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		}
		start, end := m.pickerViewport(max(1, m.height-6))
		for i := start; i < end; i++ {
			seen[i] = true
		}
		if len(seen) != len(m.pickerEntries()) {
			t.Fatalf("%dx%d paging covered %d/%d entries", size[0], size[1], len(seen), len(m.pickerEntries()))
		}
	}
}

func TestHelpScrollClampsToViewport(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.screen = screenHelp
	m.width, m.height = 60, 20
	for i := 0; i < 10 && m.helpScroll < m.helpMaxScroll(); i++ {
		m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	if m.helpScroll != m.helpMaxScroll() {
		t.Fatalf("help scroll = %d, max %d", m.helpScroll, m.helpMaxScroll())
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	if m.helpScroll >= m.helpMaxScroll() {
		t.Fatalf("help page up did not move from clamp: %d", m.helpScroll)
	}
}

func TestScenarioSwitchClearsRefreshOutcomes(t *testing.T) {
	m := demoModel(t, demo.ScenarioAttention)
	m.selectProvider("windsurf")
	key := m.selectedKey
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	for i := 0; i < 10 && m.refreshing; i++ {
		m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	}
	if outcome := m.session.RefreshOutcome(key); outcome == "Success" {
		t.Fatal("failed refresh unexpectedly succeeded")
	}
	m.switchScenario("1")
	if outcome := m.session.RefreshOutcome(key); outcome != "Current" {
		t.Fatalf("scenario switch retained refresh outcome %q", outcome)
	}
}

func TestCancellingSettingsClearsValidationError(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	for m.settingsIdx != 6 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	m.settingsInput = "relative/path"
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.settingsInput = "relative/path"
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.errorText == "" {
		t.Fatal("relative path did not surface an error")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.errorText != "" {
		t.Fatalf("cancelled settings retained error %q", m.errorText)
	}
}

func TestHelpPagesCoverEveryRenderedShortcut(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.screen = screenHelp
	m.width, m.height = 60, 20
	seen := ""
	for page := 0; page < 10; page++ {
		seen += checkFrame(t, m, 60, 20)
		if m.helpScroll >= m.helpMaxScroll() {
			break
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	}
	for _, text := range []string{"Overview", "Switch between cards and list", "Open the source library", "Source library", "Track or stop tracking a source", "Settings", "Switch between Allowances and History"} {
		if !strings.Contains(seen, text) {
			t.Fatalf("help paging skipped %q", text)
		}
	}
}
