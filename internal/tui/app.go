// Package tui implements Quotary's terminal interface.
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/domain"
	"github.com/juanrubio/quotary/internal/theme"
)

// Session is the stateful application seam implemented by the demo runtime and
// later by live wiring. The TUI never imports adapter DTOs.
type Session interface {
	ScenarioName() string
	Clock() string
	SetScenarioName(string) error
	Snapshots(context.Context) []domain.UsageSnapshot
	AllSnapshots(context.Context) []domain.UsageSnapshot
	BackendError() string
	Query() string
	StartRefresh(string)
	AdvanceRefresh(string) bool
	RefreshOutcome(string) string
	Catalog(context.Context) (backend.CatalogSnapshot, error)
	History(context.Context, backend.HistoryRequest) (domain.HistorySnapshot, error)
	Preferences() config.Preferences
	SavePreferences(config.Preferences) error
	Track(string, bool) error
	Pin(string, bool) error
}

// Options configures a TUI session.
type Options struct {
	Demo       bool
	Session    Session
	SetupError string
}

type screen int

const (
	screenOverview screen = iota
	screenDetails
	screenHistory
	screenPicker
	screenSettings
	screenHelp
)

type liveUpdateMsg struct{}

type localTickMsg struct{}

func localTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return localTickMsg{} })
}

type liveEventSession interface {
	Events() <-chan struct{}
}

type warningSession interface {
	ConfigWarning() string
}

type versionedSession interface {
	BackendVersion() string
}

type prioritySession interface {
	Prioritize(visible []string, selected string)
}

type liveBulkSession interface {
	StartBulk([]string) string
	BulkProgress() (int, int, bool)
}

type liveHistorySession interface {
	HistoryWithForce(context.Context, backend.HistoryRequest, bool) (domain.HistorySnapshot, error)
}

func waitLiveEvent(events <-chan struct{}) tea.Cmd {
	if events == nil {
		return nil
	}
	return func() tea.Msg {
		_, ok := <-events
		if !ok {
			return nil
		}
		return liveUpdateMsg{}
	}
}

type refreshTickMsg struct {
	key        string
	generation uint64
}

type model struct {
	width, height int
	demo          bool
	session       Session

	all     []domain.UsageSnapshot
	visible []domain.UsageSnapshot

	selected        int
	selectedKey     string
	screen          screen
	detailTab       int
	historyDays     int
	pickerIndex     int
	pickerQuery     string
	query           string
	searching       bool
	filter          string
	sortMode        int
	settingsIdx     int
	settingsDraft   config.Preferences
	settingsEditing bool
	settingsInput   string
	settingsDirty   bool
	returnStack     []screen
	helpScroll      int
	detailScroll    int
	historyScroll   int

	prefs          config.Preferences
	theme          theme.Styles
	liveEvents     <-chan struct{}
	setupError     string
	backendVersion string
	location       *time.Location
	light          bool
	noColor        bool

	catalog    backend.CatalogSnapshot
	history    domain.HistorySnapshot
	historyErr string
	baseStates map[string]domain.SourceStatus
	status     string
	errorText  string

	refreshing         bool
	refreshKey         string
	refreshGen         uint64
	refreshAll         bool
	refreshAllIdx      int
	refreshAllKeys     []string
	pendingRefreshAll  bool
	pendingRefreshKeys []string
	scenarioGen        uint64
}

// Run starts the interactive application.
func Run(ctx context.Context, opts Options) error {
	m := newModel(opts)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := p.Run()
	if closer, ok := opts.Session.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
	return err
}

func newModel(opts Options) *model {
	m := &model{demo: opts.Demo, session: opts.Session, filter: "all", location: time.Local, baseStates: map[string]domain.SourceStatus{}, setupError: opts.SetupError}
	m.noColor = os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
	if m.session != nil {
		m.prefs = m.session.Preferences()
		m.query = m.session.Query()
	}
	if m.prefs.Theme == "" {
		m.prefs = config.Defaults()
	}
	m.prefs = config.Normalize(m.prefs)
	if source, ok := opts.Session.(liveEventSession); ok {
		m.liveEvents = source.Events()
	}
	if versioned, ok := opts.Session.(versionedSession); ok {
		m.backendVersion = versioned.BackendVersion()
	}
	if warning, ok := opts.Session.(warningSession); ok {
		m.status = cleanText(warning.ConfigWarning())
	}
	m.load()
	m.applyTheme()
	m.syncLivePriorities()
	return m
}

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor, waitLiveEvent(m.liveEvents)}
	if m.liveEvents != nil {
		cmds = append(cmds, localTick())
	}
	return tea.Batch(cmds...)
}

func (m *model) load() {
	if m.session == nil {
		return
	}
	raw := m.session.AllSnapshots(context.Background())
	if m.baseStates == nil {
		m.baseStates = map[string]domain.SourceStatus{}
	}
	for _, snapshot := range raw {
		if _, exists := m.baseStates[snapshot.Key.String()]; !exists ||
			(snapshot.State != domain.StatusQueued && snapshot.State != domain.StatusFetching) {
			m.baseStates[snapshot.Key.String()] = snapshot.State
		}
	}
	m.all = raw
	for index := range m.all {
		m.all[index] = sanitizeSnapshot(m.all[index])
		m.all[index].State = m.effectiveState(m.all[index])
	}
	catalog, _ := m.session.Catalog(context.Background())
	m.catalog = sanitizeCatalog(catalog)
	m.filterVisible()
}

func (m *model) applyTheme() {
	light := m.light
	switch m.prefs.Appearance {
	case config.AppearanceLight:
		light = true
	case config.AppearanceDark:
		light = false
	}
	m.theme = theme.New(string(m.prefs.Theme), light, m.noColor || m.prefs.Monochrome)
}

func (m *model) filterVisible() {
	pins := map[string]int{}
	for index, key := range m.prefs.Pins {
		pins[key] = index
	}
	filtered := make([]domain.UsageSnapshot, 0, len(m.all))
	for _, snapshot := range m.all {
		pinned := false
		if _, ok := pins[snapshot.Key.String()]; ok {
			pinned = true
		}
		if !domain.Filter(snapshot, m.filter, m.query, pinned) {
			continue
		}
		filtered = append(filtered, snapshot)
	}
	switch m.sortMode {
	case 1:
		sort.SliceStable(filtered, func(i, j int) bool {
			return strings.ToLower(filtered[i].DisplayName) < strings.ToLower(filtered[j].DisplayName)
		})
	case 2:
		sort.SliceStable(filtered, func(i, j int) bool {
			ai, aj := filtered[i].State.NeedsAttention(), filtered[j].State.NeedsAttention()
			if ai != aj {
				return ai
			}
			return strings.ToLower(filtered[i].DisplayName) < strings.ToLower(filtered[j].DisplayName)
		})
	case 3:
		sort.SliceStable(filtered, func(i, j int) bool {
			ri, rj := resetTime(filtered[i]), resetTime(filtered[j])
			if ri == nil || rj == nil {
				return ri != nil
			}
			if !ri.Equal(*rj) {
				return ri.Before(*rj)
			}
			return strings.ToLower(filtered[i].DisplayName) < strings.ToLower(filtered[j].DisplayName)
		})
	default:
		domain.SortSnapshots(filtered, pins)
	}
	if m.selectedKey != "" && m.refreshing {
		found := false
		for _, snapshot := range filtered {
			if snapshot.Key.String() == m.selectedKey {
				found = true
				break
			}
		}
		if !found {
			for _, snapshot := range m.all {
				if snapshot.Key.String() != m.selectedKey ||
					(snapshot.State != domain.StatusQueued && snapshot.State != domain.StatusFetching) {
					continue
				}
				pinned := false
				if _, ok := pins[snapshot.Key.String()]; ok {
					pinned = true
				}
				filterForSelection := m.filter
				if filterForSelection == "attention" {
					filterForSelection = "all"
				}
				if domain.Filter(snapshot, filterForSelection, m.query, pinned) {
					filtered = append(filtered, snapshot)
				}
				break
			}
		}
	}
	previousKey := m.selectedKey
	m.visible = filtered
	if len(filtered) == 0 {
		m.selected = 0
		m.selectedKey = ""
		m.history = domain.HistorySnapshot{}
		m.historyErr = ""
		m.historyScroll = 0
		return
	}
	found := false
	if m.selectedKey != "" {
		for index, snapshot := range filtered {
			if snapshot.Key.String() == m.selectedKey {
				m.selected = index
				found = true
				break
			}
		}
	}
	if !found {
		if m.selected >= len(filtered) {
			m.selected = len(filtered) - 1
		}
		if m.selected < 0 {
			m.selected = 0
		}
		m.selectedKey = filtered[m.selected].Key.String()
	}
	if previousKey != m.selectedKey {
		m.history = domain.HistorySnapshot{}
		m.historyErr = ""
		m.historyScroll = 0
		if m.historyVisible() {
			m.loadHistoryIfNeeded()
		}
	}
}

func (m *model) selectedSnapshot() (domain.UsageSnapshot, bool) {
	if len(m.visible) == 0 {
		return domain.UsageSnapshot{}, false
	}
	if m.selected < 0 || m.selected >= len(m.visible) {
		m.selected = 0
	}
	return m.visible[m.selected], true
}

func (m *model) syncLivePriorities() {
	session, ok := m.session.(prioritySession)
	if !ok || len(m.visible) == 0 {
		return
	}
	start, end := pageStart(m), min(pageEnd(m), len(m.visible))
	keys := make([]string, 0, end-start)
	for _, snapshot := range m.visible[start:end] {
		keys = append(keys, snapshot.Key.String())
	}
	session.Prioritize(keys, m.selectedKey)
}

func (m *model) move(delta int) {
	if len(m.visible) == 0 {
		return
	}
	next := m.selected + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.visible) {
		next = len(m.visible) - 1
	}
	previousKey := m.selectedKey
	m.selected = next
	m.selectedKey = m.visible[next].Key.String()
	if previousKey != m.selectedKey {
		m.history = domain.HistorySnapshot{}
		m.historyErr = ""
	}
	m.detailScroll = 0
	m.syncLivePriorities()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.light = isLight(msg.Color)
		m.applyTheme()
	case liveUpdateMsg:
		m.load()
		if m.historyVisible() {
			m.loadHistoryForce(false)
		}
		return m, waitLiveEvent(m.liveEvents)
	case localTickMsg:
		if m.liveEvents == nil {
			return m, nil
		}
		m.load()
		if m.historyVisible() {
			m.loadHistoryForce(false)
		}
		return m, localTick()
	case refreshTickMsg:
		return m.onRefreshTick(msg)
	case tea.KeyPressMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m *model) onRefreshTick(msg refreshTickMsg) (tea.Model, tea.Cmd) {
	if m.session == nil || msg.generation != m.refreshGen {
		return m, nil
	}
	if m.refreshAll {
		if bulk, ok := m.session.(liveBulkSession); ok {
			if done, total, active := bulk.BulkProgress(); active {
				m.status = fmt.Sprintf("Refreshing %d/%d", done, total)
			}
		}
		if m.refreshAllIdx >= len(m.refreshAllKeys) {
			return m.finishRefresh()
		}
		key := m.refreshAllKeys[m.refreshAllIdx]
		if !m.session.AdvanceRefresh(key) {
			m.refreshAllIdx++
		}
		m.load()
		return m, tea.Tick(60*time.Millisecond, func(time.Time) tea.Msg {
			return refreshTickMsg{generation: msg.generation}
		})
	}
	if msg.key != m.refreshKey {
		return m, nil
	}
	done := m.session.AdvanceRefresh(msg.key)
	m.load()
	if !done {
		return m.finishRefresh()
	}
	m.status = "Fetching…"
	return m, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return refreshTickMsg{key: msg.key, generation: msg.generation}
	})
}

func (m *model) onKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	wasSettings := m.screen == screenSettings
	defer func() {
		suspended := (m.screen == screenPicker || m.screen == screenHelp) &&
			len(m.returnStack) > 0 && m.returnStack[len(m.returnStack)-1] == screenSettings
		if wasSettings && m.screen != screenSettings && !suspended {
			m.applyTheme()
			m.settingsDraft = m.prefs
			m.settingsDirty = false
		}
	}()
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.settingsEditing {
		return m.onSettingsEdit(msg)
	}
	if m.searching {
		return m.onSearchKey(msg)
	}
	if key == "q" {
		return m, tea.Quit
	}
	switch key {
	case "j", "down":
		switch m.screen {
		case screenOverview:
			if key == "j" {
				m.move(1)
			} else {
				m.move(m.overviewStep())
			}
		case screenPicker:
			m.movePicker(1)
		case screenSettings:
			if m.settingsIdx == 0 {
				m.changeSetting(1)
			} else {
				m.moveSettings(1)
			}
		case screenHelp:
			m.helpScroll = min(m.helpScroll+1, m.helpMaxScroll())
		case screenDetails:
			if m.detailTab == 1 {
				m.historyScroll++
			} else {
				m.detailScroll++
			}
		case screenHistory:
			m.move(1)
			m.loadHistoryIfNeeded()
		default:
			m.move(1)
		}
	case "k", "up":
		switch m.screen {
		case screenOverview:
			if key == "k" {
				m.move(-1)
			} else {
				m.move(-m.overviewStep())
			}
		case screenPicker:
			m.movePicker(-1)
		case screenSettings:
			if m.settingsIdx == 0 {
				m.changeSetting(-1)
			} else {
				m.moveSettings(-1)
			}
		case screenHelp:
			if m.helpScroll > 0 {
				m.helpScroll--
			}
		case screenDetails:
			if m.detailTab == 1 {
				if m.historyScroll > 0 {
					m.historyScroll--
				}
			} else if m.detailScroll > 0 {
				m.detailScroll--
			}
		case screenHistory:
			m.move(-1)
			m.loadHistoryIfNeeded()
		default:
			m.move(-1)
		}
	case "left", "h":
		switch m.screen {
		case screenOverview:
			m.move(-1)
		case screenSettings:
			m.changeSetting(-1)
		}
	case "right", "l":
		switch m.screen {
		case screenOverview:
			if key == "l" {
				m.toggleLayout()
			} else {
				m.move(1)
			}
		case screenSettings:
			m.changeSetting(1)
		}
	case "enter":
		return m.onEnter()
	case "esc":
		if m.screen == screenSettings {
			m.settingsDraft = m.prefs
			m.settingsDirty = false
			m.errorText = ""
			m.applyTheme()
		}
		if m.screen == screenPicker || m.screen == screenSettings || m.screen == screenHelp {
			m.closeSheet()
			m.status = ""
		} else if m.screen != screenOverview {
			m.screen = screenOverview
			m.status = ""
		} else if m.query != "" {
			m.query = ""
			m.filterVisible()
		}
	case "tab", "shift+tab":
		delta := 1
		if key == "shift+tab" {
			delta = -1
		}
		switch m.screen {
		case screenHistory:
			m.screen = screenDetails
			m.detailTab = 0
		case screenDetails:
			m.detailTab = (m.detailTab + delta + 2) % 2
			m.detailScroll = 0
			if m.detailTab == 1 {
				m.loadHistoryIfNeeded()
			}
		case screenSettings:
			m.moveSettings(delta)
		}
	case "/":
		m.searching = true
		if m.screen == screenPicker {
			m.setPickerQuery("")
		} else {
			m.query = ""
			m.filterVisible()
		}
	case "f":
		switch m.filter {
		case "all":
			m.filter = "attention"
		case "attention":
			m.filter = "pinned"
		default:
			m.filter = "all"
		}
		m.filterVisible()
	case "s":
		m.sortMode = (m.sortMode + 1) % 4
		m.filterVisible()
	case "p":
		if m.screen == screenPicker {
			m.closeSheet()
		} else if m.returnToPicker() {
		} else {
			m.openSheet(screenPicker)
			m.pickerIndex = 0
		}
	case " ", "space":
		return m.onSpace()
	case "a":
		if m.screen == screenPicker {
			m.trackAllSubscriptions()
		}
	case "e", "E":
		if m.screen == screenSettings && m.settingsIdx >= 6 {
			m.beginSettingsEdit()
		}
	case "r", "R":
		return m.onRefresh(key == "R")
	case "?":
		m.openSheet(screenHelp)
	case ",":
		if !m.settingsDirty {
			m.settingsIdx = 0
			m.settingsDraft = m.prefs
		}
		m.openSheet(screenSettings)
	case "d":
		m.switchScenario("1")
	case "1", "2", "3", "4", "5", "6":
		m.switchScenario(key)
	case "[":
		if m.historyVisible() {
			m.historyDays = 7
			m.loadHistoryIfNeeded()
		}
	case "]":
		if m.historyVisible() {
			m.historyDays = 30
			m.loadHistoryIfNeeded()
		}
	case "pgdown":
		switch {
		case m.screen == screenPicker:
			start, end := m.pickerViewport(max(1, m.height-6))
			m.movePicker(max(1, end-start))
		case m.screen == screenSettings:
			m.moveSettings(1)
		case m.screen == screenHelp:
			m.helpScroll = min(m.helpScroll+max(1, m.height-8), m.helpMaxScroll())
		case m.historyVisible():
			m.historyScroll += m.pageSize()
		case m.screen == screenDetails:
			m.detailScroll += m.pageSize()
		default:
			m.move(m.pageSize())
		}
	case "pgup":
		switch {
		case m.screen == screenPicker:
			start, end := m.pickerViewport(max(1, m.height-6))
			m.movePicker(-max(1, end-start))
		case m.screen == screenSettings:
			m.moveSettings(-1)
		case m.screen == screenHelp:
			m.helpScroll = max(0, m.helpScroll-max(1, m.height-8))
		case m.historyVisible():
			m.historyScroll = max(0, m.historyScroll-m.pageSize())
		case m.screen == screenDetails:
			m.detailScroll = max(0, m.detailScroll-m.pageSize())
		default:
			m.move(-m.pageSize())
		}
	}
	return m, nil
}

func (m *model) onSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		if m.screen == screenPicker {
			m.setPickerQuery("")
		} else {
			m.query = ""
			m.filterVisible()
		}
	case "enter":
		m.searching = false
		if m.screen != screenPicker {
			m.filterVisible()
		}
	case "backspace":
		if m.screen == screenPicker {
			if m.pickerQuery != "" {
				r := []rune(m.pickerQuery)
				m.setPickerQuery(string(r[:len(r)-1]))
			}
		} else if m.query != "" {
			r := []rune(m.query)
			m.query = string(r[:len(r)-1])
			m.filterVisible()
		}
	default:
		text := cleanInput(msg.Text)
		if text != "" {
			if m.screen == screenPicker {
				m.setPickerQuery(m.pickerQuery + text)
			} else {
				m.query += text
				m.filterVisible()
			}
		}
	}
	return m, nil
}

func (m *model) onEnter() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenOverview, screenHistory:
		if _, ok := m.selectedSnapshot(); ok {
			m.screen = screenDetails
			m.detailTab = 0
		}
	case screenPicker:
		m.togglePickerTracking()
		return m, nil
	case screenSettings:
		if m.settingsIdx >= 6 {
			m.beginSettingsEdit()
		} else {
			m.saveSettings()
		}
	case screenDetails:
		if m.detailTab == 0 {
			m.screen = screenHistory
			m.loadHistoryIfNeeded()
		} else {
			m.screen = screenDetails
		}
	}
	return m, nil
}

func (m *model) onSpace() (tea.Model, tea.Cmd) {
	if m.session == nil {
		return m, nil
	}
	if m.screen == screenPicker {
		m.togglePickerTracking()
		return m, nil
	}
	if m.screen != screenOverview {
		return m, nil
	}
	snapshot, ok := m.selectedSnapshot()
	if !ok {
		return m, nil
	}
	key := snapshot.Key.String()
	pinned := contains(m.prefs.Pins, key)
	if err := m.session.Pin(key, !pinned); err != nil {
		m.errorText = safeError(err)
		return m, nil
	}
	m.prefs = m.session.Preferences()
	m.filterVisible()
	return m, nil
}

func (m *model) startRefreshAll() (tea.Model, tea.Cmd) {
	m.refreshGen++
	m.refreshing = true
	m.refreshAll = true
	m.refreshAllIdx = 0
	m.refreshAllKeys = nil
	for _, snapshot := range m.all {
		m.refreshAllKeys = append(m.refreshAllKeys, snapshot.Key.String())
	}
	if bulk, ok := m.session.(liveBulkSession); ok {
		bulk.StartBulk(m.refreshAllKeys)
	} else {
		for _, key := range m.refreshAllKeys {
			m.session.StartRefresh(key)
		}
	}
	m.status = "Queued all tracked sources"
	m.load()
	generation := m.refreshGen
	return m, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return refreshTickMsg{generation: generation}
	})
}

func (m *model) startRefreshKey(key string) (tea.Model, tea.Cmd) {
	m.refreshGen++
	m.refreshKey = key
	m.refreshing = true
	m.status = "Queueing refresh…"
	m.session.StartRefresh(key)
	m.load()
	generation := m.refreshGen
	return m, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return refreshTickMsg{key: key, generation: generation}
	})
}

func (m *model) finishRefresh() (tea.Model, tea.Cmd) {
	m.refreshing = false
	m.refreshAll = false
	m.refreshAllIdx = 0
	m.refreshAllKeys = nil
	m.refreshKey = ""
	m.status = ""
	if m.pendingRefreshAll {
		m.pendingRefreshAll = false
		m.pendingRefreshKeys = nil
		return m.startRefreshAll()
	}
	if len(m.pendingRefreshKeys) > 0 {
		key := m.pendingRefreshKeys[0]
		m.pendingRefreshKeys = m.pendingRefreshKeys[1:]
		return m.startRefreshKey(key)
	}
	m.load()
	return m, nil
}

func (m *model) onRefresh(all bool) (tea.Model, tea.Cmd) {
	if m.session == nil {
		return m, nil
	}
	if !all && m.historyVisible() {
		m.loadHistoryForce(true)
		return m, nil
	}
	var key string
	if !all {
		if m.screen == screenPicker {
			entries := m.pickerEntries()
			if m.pickerIndex < 0 || m.pickerIndex >= len(entries) {
				return m, nil
			}
			key = entries[m.pickerIndex].Key.String()
		} else {
			snapshot, ok := m.selectedSnapshot()
			if !ok {
				return m, nil
			}
			key = snapshot.Key.String()
		}
	}
	if m.refreshing {
		if all {
			m.pendingRefreshAll = true
		} else if key != m.refreshKey && !contains(m.pendingRefreshKeys, key) {
			m.pendingRefreshKeys = append(m.pendingRefreshKeys, key)
		}
		m.status = "Refresh queued"
		return m, nil
	}
	if all {
		return m.startRefreshAll()
	}
	return m.startRefreshKey(key)
}

func (m *model) toggleLayout() {
	next := m.prefs
	effectiveCompact := next.Layout == config.LayoutCompact ||
		(next.Layout != config.LayoutCards && (m.width < 88 || m.height < 32))
	if effectiveCompact && m.width >= 88 {
		next.Layout = config.LayoutCards
	} else {
		next.Layout = config.LayoutCompact
	}
	if m.session != nil {
		if err := m.session.SavePreferences(next); err != nil {
			m.errorText = safeError(err)
			m.status = ""
			return
		}
	}
	m.prefs = next
	m.errorText = ""
}

func (m *model) switchScenario(key string) {
	if m.session == nil {
		return
	}
	scenario := map[string]string{
		"1": "subscription", "2": "attention", "3": "catalog",
		"4": "empty", "5": "setup", "6": "noresults",
	}[key]
	if err := m.session.SetScenarioName(scenario); err != nil {
		m.errorText = safeError(err)
		return
	}
	m.scenarioGen++
	m.refreshGen++
	m.refreshing = false
	m.refreshAll = false
	m.refreshAllIdx = 0
	m.refreshAllKeys = nil
	m.refreshKey = ""
	m.pendingRefreshAll = false
	m.pendingRefreshKeys = nil
	m.selected = 0
	m.selectedKey = ""
	m.history = domain.HistorySnapshot{}
	m.historyErr = ""
	m.detailScroll = 0
	m.baseStates = map[string]domain.SourceStatus{}
	m.query = m.session.Query()
	m.status = "Scenario: " + scenario
	m.prefs = m.session.Preferences()
	m.load()
}

func (m *model) loadHistoryIfNeeded() {
	snapshot, ok := m.selectedSnapshot()
	days := m.historyDays
	if days != 30 {
		days = 7
	}
	if ok && m.historyErr == "" && m.history.Key.String() == snapshot.Key.String() && m.history.Days == days {
		return
	}
	m.loadHistory()
}

func (m *model) loadHistory() {
	m.loadHistoryForce(false)
}

func (m *model) loadHistoryForce(force bool) {
	if m.session == nil {
		return
	}
	snapshot, ok := m.selectedSnapshot()
	if !ok {
		return
	}
	days := m.historyDays
	if days != 30 {
		days = 7
	}
	request := backend.HistoryRequest{
		ProviderID: snapshot.ProviderID,
		Variant:    snapshot.Key.Variant,
		SourceKey:  snapshot.Key.String(),
		Days:       days,
	}
	var history domain.HistorySnapshot
	var err error
	if session, ok := m.session.(liveHistorySession); ok {
		history, err = session.HistoryWithForce(context.Background(), request, force)
	} else {
		history, err = m.session.History(context.Background(), request)
	}
	if err != nil {
		m.history = domain.HistorySnapshot{}
		m.historyErr = safeError(err)
		return
	}
	previousScroll := m.historyScroll
	previousKey := m.history.Key.String()
	previousDays := m.history.Days
	m.history = sanitizeHistory(history)
	m.historyErr = ""
	if !force && previousKey == m.history.Key.String() && previousDays == m.history.Days {
		m.historyScroll = previousScroll
	} else {
		m.historyScroll = 0
	}
}

func (m *model) helpPageSize() int { return max(1, m.height-6-5) }

func (m *model) helpMaxScroll() int { return max(0, len(helpRows)-m.helpPageSize()) }

func (m *model) overviewStep() int {
	if m.prefs.Layout == config.LayoutCards || (m.prefs.Layout != config.LayoutCompact && m.width >= 88 && m.height >= 32) {
		return max(1, m.cardColumns(m.width))
	}
	return 1
}

func (m *model) pickerViewport(height int) (int, int) {
	entries := m.pickerEntries()
	if len(entries) == 0 {
		return 0, 0
	}
	budget := max(1, height-13)
	start := 0
	for start < len(entries) {
		end, rows := start, 0
		for end < len(entries) {
			cost := 1
			if m.width < 110 {
				cost++
			}
			if rows+cost > budget && end > start {
				break
			}
			rows += cost
			end++
		}
		if end == start {
			end++
		}
		if m.pickerIndex >= start && m.pickerIndex < end {
			return start, end
		}
		start = end
	}
	return start, len(entries)
}

func (m *model) returnToPicker() bool {
	for index := len(m.returnStack) - 1; index >= 0; index-- {
		if m.returnStack[index] == screenPicker {
			cancelledSettings := false
			for _, frame := range m.returnStack[index+1:] {
				if frame == screenSettings {
					cancelledSettings = true
				}
			}
			m.returnStack = m.returnStack[:index]
			m.screen = screenPicker
			if cancelledSettings {
				m.applyTheme()
				m.settingsDraft = m.prefs
				m.settingsDirty = false
			}
			return true
		}
	}
	return false
}

func (m *model) openSheet(next screen) {
	if m.screen != next {
		m.returnStack = append(m.returnStack, m.screen)
	}
	m.screen = next
}

func (m *model) closeSheet() {
	if len(m.returnStack) == 0 {
		m.screen = screenOverview
		return
	}
	m.screen = m.returnStack[len(m.returnStack)-1]
	m.returnStack = m.returnStack[:len(m.returnStack)-1]
	if m.historyVisible() {
		m.loadHistoryIfNeeded()
	}
}

func (m *model) beginSettingsEdit() {
	if m.settingsIdx == 6 {
		m.settingsInput = m.settingsDraft.BackendPath
	} else {
		m.settingsInput = m.settingsDraft.BackendConfigPath
	}
	m.settingsEditing = true
}

func (m *model) onSettingsEdit(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.settingsEditing = false
	case "enter":
		if m.settingsIdx == 6 {
			m.settingsDraft.BackendPath = m.settingsInput
		} else {
			m.settingsDraft.BackendConfigPath = m.settingsInput
		}
		m.settingsEditing = false
		m.settingsDirty = true
		m.saveSettings()
	case "backspace":
		runes := []rune(m.settingsInput)
		if len(runes) > 0 {
			m.settingsInput = string(runes[:len(runes)-1])
		}
	default:
		if msg.Text != "" && len(m.settingsInput) < 1024 {
			m.settingsInput += msg.Text
		}
	}
	return m, nil
}

func (m *model) moveSettings(delta int) {
	m.settingsIdx = (m.settingsIdx + delta + settingsRows) % settingsRows
}

func (m *model) movePicker(delta int) {
	entries := m.pickerEntries()
	if len(entries) == 0 {
		m.pickerIndex = 0
		return
	}
	next := m.pickerIndex + delta
	if next < 0 {
		next = 0
	}
	if next >= len(entries) {
		next = len(entries) - 1
	}
	m.pickerIndex = next
}

func (m *model) setPickerQuery(query string) {
	selectedKey := ""
	entries := m.pickerEntries()
	if m.pickerIndex >= 0 && m.pickerIndex < len(entries) {
		selectedKey = entries[m.pickerIndex].Key.String()
	}
	m.pickerQuery = query
	entries = m.pickerEntries()
	for index, descriptor := range entries {
		if descriptor.Key.String() == selectedKey {
			m.pickerIndex = index
			return
		}
	}
	m.pickerIndex = 0
}

func (m *model) pickerEntries() []domain.SourceDescriptor {
	if m.session == nil {
		return nil
	}
	query := strings.ToLower(m.pickerQuery)
	var entries []domain.SourceDescriptor
	for _, descriptor := range m.catalog.Providers {
		if query == "" || strings.Contains(strings.ToLower(descriptor.DisplayName), query) ||
			strings.Contains(strings.ToLower(descriptor.Key.String()), query) {
			entries = append(entries, descriptor)
		}
	}
	return entries
}

func (m *model) togglePickerTracking() {
	if m.session == nil {
		return
	}
	entries := m.pickerEntries()
	if len(entries) == 0 {
		return
	}
	if m.pickerIndex < 0 || m.pickerIndex >= len(entries) {
		return
	}
	descriptor := entries[m.pickerIndex]
	if err := m.session.Track(descriptor.Key.String(), !descriptor.Tracked); err != nil {
		m.errorText = safeError(err)
		return
	}
	m.prefs = m.session.Preferences()
	m.load()
}

func (m *model) trackAllSubscriptions() {
	if m.session == nil {
		return
	}
	count, failed := 0, 0
	for _, descriptor := range m.catalog.Providers {
		if descriptor.Kind != domain.KindSubscription && descriptor.Kind != domain.KindMixed {
			continue
		}
		if !descriptor.Capabilities.BulkTrackable(descriptor.Kind) {
			continue
		}
		if descriptor.ProviderID == "gemini" && descriptor.Variant != domain.VariantConfigured {
			continue
		}
		if !descriptor.Tracked {
			if err := m.session.Track(descriptor.Key.String(), true); err != nil {
				failed++
			} else {
				count++
			}
		}
	}
	m.prefs = m.session.Preferences()
	m.load()
	switch {
	case failed > 0 && count > 0:
		m.errorText = fmt.Sprintf("Tracked %d subscriptions; %d could not be saved", count, failed)
		m.status = ""
	case failed > 0:
		m.errorText = fmt.Sprintf("Tracking failed for %d subscriptions", failed)
		m.status = ""
	default:
		m.status = fmt.Sprintf("Tracked %d subscriptions", count)
	}
}

func (m *model) historyVisible() bool {
	return m.screen == screenHistory || (m.screen == screenDetails && m.detailTab == 1)
}

func hasMeasurements(snapshot domain.UsageSnapshot) bool {
	if len(snapshot.Windows) > 0 || len(snapshot.Balances) > 0 {
		return true
	}
	for _, detail := range snapshot.Details {
		if len(detail.Rows) > 0 || len(detail.Bars) > 0 || len(detail.Points) > 0 {
			return true
		}
	}
	return false
}

func (m *model) pageSize() int {
	if m.prefs.Layout == config.LayoutCompact || (m.prefs.Layout != config.LayoutCards && (m.width < 88 || m.height < 32)) {
		start, end := m.compactViewport(max(1, m.height-6))
		return max(1, end-start)
	}
	header := 4
	footer := 2
	contentHeight := max(1, m.height-header-footer)
	columns := m.cardColumns(m.width)
	cardHeight := 9
	if contentHeight >= 40 {
		cardHeight = 10
	}
	rows := max(1, (contentHeight+1)/(cardHeight+1))
	return columns * rows
}

func (m *model) compactViewport(height int) (int, int) {
	if len(m.visible) == 0 {
		return 0, 0
	}
	start := 0
	for start < len(m.visible) {
		end := start
		rows := 1
		for end < len(m.visible) {
			cost := 1
			snapshot := m.visible[end]
			state := m.effectiveState(snapshot)
			_ = state
			if m.retainedPrior(snapshot) && hasMeasurements(snapshot) {
				cost++
			}
			if end == m.selected && len(snapshot.Windows) > 1 {
				cost++
			}
			if rows+cost+2 > height && end > start {
				break
			}
			rows += cost
			end++
		}
		if end == start {
			end++
		}
		if m.selected >= start && m.selected < end {
			return start, end
		}
		start = end
	}
	return start, len(m.visible)
}

func isLight(color interface {
	RGBA() (uint32, uint32, uint32, uint32)
},
) bool {
	r, g, b, _ := color.RGBA()
	return 0.299*float64(r>>8)+0.587*float64(g>>8)+0.114*float64(b>>8) > 153
}

func resetTime(snapshot domain.UsageSnapshot) *time.Time {
	var earliest *time.Time
	for _, window := range snapshot.Windows {
		if window.ResetAt == nil {
			continue
		}
		if earliest == nil || window.ResetAt.Before(*earliest) {
			value := *window.ResetAt
			earliest = &value
		}
	}
	return earliest
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (m *model) now() time.Time {
	if m.session != nil {
		if parsed, err := time.Parse(time.RFC3339, m.session.Clock()); err == nil {
			return parsed
		}
	}
	return time.Now().UTC()
}

func (m *model) clock() string {
	if m.session != nil {
		return m.session.Clock()
	}
	return ""
}

func (m *model) View() tea.View {
	width, height := m.width, m.height
	if width <= 0 {
		width, height = 80, 24
	}
	content := m.render(width, height)
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

const settingsRows = 8

func (m *model) changeSetting(delta int) {
	draft := m.settingsDraft
	if draft.Theme == "" {
		draft = m.prefs
	}
	switch m.settingsIdx {
	case 0:
		themes := []config.ThemeName{config.ThemeGreen, config.ThemeBlue, config.ThemeAmber, config.ThemeMonokai}
		index := 0
		for i, name := range themes {
			if name == draft.Theme {
				index = i
			}
		}
		index = (index + delta + len(themes)) % len(themes)
		draft.Theme = themes[index]
	case 1:
		values := []config.Appearance{config.AppearanceAuto, config.AppearanceDark, config.AppearanceLight}
		index := 0
		for i, value := range values {
			if value == draft.Appearance {
				index = i
			}
		}
		index = (index + delta + len(values)) % len(values)
		draft.Appearance = values[index]
	case 2:
		layouts := []config.Layout{config.LayoutAuto, config.LayoutCards, config.LayoutCompact}
		index := 0
		for i, value := range layouts {
			if value == draft.Layout {
				index = i
			}
		}
		index = (index + delta + len(layouts)) % len(layouts)
		draft.Layout = layouts[index]
	case 3:
		draft.IntervalSeconds += delta * 60
		if draft.IntervalSeconds < 60 {
			draft.IntervalSeconds = 60
		}
	case 4:
		draft.Privacy = !draft.Privacy
	case 5:
		draft.Monochrome = !draft.Monochrome
	}
	m.settingsDraft = draft
	m.settingsDirty = true
	m.applyThemePreview(draft)
}

func (m *model) applyThemePreview(prefs config.Preferences) {
	light := m.light
	switch prefs.Appearance {
	case config.AppearanceLight:
		light = true
	case config.AppearanceDark:
		light = false
	}
	m.theme = theme.New(string(prefs.Theme), light, m.noColor || prefs.Monochrome)
}

func (m *model) recomputePresentationStates() {
	for index := range m.all {
		if m.all[index].State == domain.StatusQueued || m.all[index].State == domain.StatusFetching {
			continue
		}
		base := domain.StatusCurrent
		if value, ok := m.baseStates[m.all[index].Key.String()]; ok {
			base = value
		}
		m.all[index].State = base
		m.all[index].State = m.effectiveState(m.all[index])
	}
	m.filterVisible()
}

func (m *model) saveSettings() {
	draft := config.Normalize(m.settingsDraft)
	if draft.BackendPath != "" && !filepath.IsAbs(draft.BackendPath) {
		m.errorText = "Backend path must be absolute"
		m.applyTheme()
		return
	}
	if draft.BackendConfigPath != "" && !filepath.IsAbs(draft.BackendConfigPath) {
		m.errorText = "Backend config path must be absolute"
		m.applyTheme()
		return
	}
	if m.session != nil {
		current := m.session.Preferences()
		draft.Tracked = current.Tracked
		draft.Pins = current.Pins
	}
	m.settingsDraft = draft
	if m.session != nil {
		if err := m.session.SavePreferences(m.settingsDraft); err != nil {
			m.errorText = safeError(err)
			m.applyTheme()
			return
		}
	}
	m.prefs = m.settingsDraft
	m.settingsDirty = false
	m.errorText = ""
	m.recomputePresentationStates()
	m.applyTheme()
	m.status = "Settings saved"
	m.closeSheet()
}
