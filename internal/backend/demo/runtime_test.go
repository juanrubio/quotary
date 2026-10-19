package demo

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/domain"
)

func TestRuntimeSeedsScenarioPreferences(t *testing.T) {
	runtime, err := NewRuntime(ScenarioSubscription, Deps{})
	if err != nil {
		t.Fatal(err)
	}
	prefs := runtime.Preferences()
	if len(prefs.Tracked) != 24 {
		t.Fatalf("tracked = %d", len(prefs.Tracked))
	}
	if len(prefs.Pins) == 0 {
		t.Fatal("pinned demo sources were not seeded")
	}
	snapshots := runtime.Snapshots(context.Background())
	counts := domain.CountStates(snapshots)
	if counts.Total != 24 || counts.Current != 24 || counts.Low != 2 {
		t.Fatalf("counts = %+v", counts)
	}
}

func TestRuntimeTrackingAndPinningAreInMemory(t *testing.T) {
	store := config.NewMemoryStore(config.Defaults())
	runtime, err := NewRuntime(ScenarioSubscription, Deps{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	key := "claude:configured@unknown"
	if err := runtime.Track(key, false); err != nil {
		t.Fatal(err)
	}
	if contains(runtime.Preferences().Tracked, key) {
		t.Fatal("untrack did not persist in memory")
	}
	if err := runtime.Pin(key, true); err == nil {
		t.Fatal("pinning an untracked source should fail")
	}
	if err := runtime.Track(key, true); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Pin(key, true); err != nil {
		t.Fatal(err)
	}
	if !contains(runtime.Preferences().Pins, key) {
		t.Fatal("pin did not persist in memory")
	}
	if !contains(store.Load().Tracked, key) {
		t.Fatal("runtime did not use the injected store")
	}
}

func TestRuntimeRefreshTransitions(t *testing.T) {
	runtime, err := NewRuntime(ScenarioAttention, Deps{})
	if err != nil {
		t.Fatal(err)
	}
	key := "claude:configured@unknown"
	runtime.StartRefresh(key)
	if got := snapshotState(runtime, key); got != domain.StatusQueued {
		t.Fatalf("queued state = %s", got)
	}
	if !runtime.AdvanceRefresh(key) {
		t.Fatal("queued should advance to fetching")
	}
	if got := snapshotState(runtime, key); got != domain.StatusFetching {
		t.Fatalf("fetching state = %s", got)
	}
	if runtime.AdvanceRefresh(key) {
		t.Fatal("fetching should finish")
	}
	if got := snapshotState(runtime, key); got != domain.StatusCurrent {
		t.Fatalf("current state = %s", got)
	}
}

func TestRuntimeScenarioSwitch(t *testing.T) {
	runtime, err := NewRuntime(ScenarioSubscription, Deps{})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.SetScenario(ScenarioAttention); err != nil {
		t.Fatal(err)
	}
	if runtime.ScenarioName() != string(ScenarioAttention) {
		t.Fatalf("scenario = %s", runtime.ScenarioName())
	}
	states := map[string]domain.SourceStatus{}
	for _, snapshot := range runtime.Snapshots(context.Background()) {
		states[snapshot.Key.String()] = snapshot.State
	}
	if states["antigravity:configured@unknown"] != domain.StatusExhausted {
		t.Fatalf("attention state = %v", states)
	}
	if err := runtime.SetScenario(ScenarioSetup); err != nil {
		t.Fatal(err)
	}
	if got := runtime.Snapshots(context.Background()); len(got) != 0 {
		t.Fatalf("setup snapshots = %d", len(got))
	}
}

type spyFS struct {
	base  fs.FS
	reads []string
}

func (s *spyFS) Open(name string) (fs.File, error) {
	s.reads = append(s.reads, name)
	return s.base.Open(name)
}

func TestRuntimeReadsOnlyInMemoryFixtures(t *testing.T) {
	spy := &spyFS{base: FixtureFS()}
	runtime, err := NewRuntime(ScenarioSubscription, Deps{Fixtures: spy})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := runtime.Catalog(ctx); err != nil {
		t.Fatal(err)
	}
	runtime.Snapshots(ctx)
	if _, err := runtime.History(ctx, backend.HistoryRequest{ProviderID: "muse", SourceKey: "muse:configured@unknown", Days: 7}); err != nil {
		t.Fatal(err)
	}
	runtime.StartRefresh("claude:configured@unknown")
	runtime.AdvanceRefresh("claude:configured@unknown")
	for _, path := range spy.reads {
		if path != "fixtures.json" {
			t.Fatalf("demo read unexpected fixture path %q", path)
		}
	}
	if len(spy.reads) == 0 {
		t.Fatal("injected fixture filesystem was not used")
	}
}

func TestRuntimeResetBetweenSessions(t *testing.T) {
	first, err := NewRuntime(ScenarioSubscription, Deps{})
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Track("claude:configured@unknown", false); err != nil {
		t.Fatal(err)
	}
	second, err := NewRuntime(ScenarioSubscription, Deps{})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(second.Preferences().Tracked, "claude:configured@unknown") {
		t.Fatal("new session did not reset demo preferences")
	}
}

func snapshotState(runtime *Runtime, key string) domain.SourceStatus {
	for _, snapshot := range runtime.Snapshots(context.Background()) {
		if snapshot.Key.String() == key {
			return snapshot.State
		}
	}
	return domain.StatusUnknown
}

func TestNoResultsPresetPreservesCollection(t *testing.T) {
	runtime, err := NewRuntime(ScenarioNoResults, Deps{})
	if err != nil {
		t.Fatal(err)
	}
	if len(runtime.Snapshots(context.Background())) != 0 {
		t.Fatal("no-results preset returned snapshots")
	}
	if len(runtime.AllSnapshots(context.Background())) != 24 {
		t.Fatalf("underlying collection = %d", len(runtime.AllSnapshots(context.Background())))
	}
	if runtime.Query() == "" {
		t.Fatal("no-results preset lost its query")
	}
	catalog, err := runtime.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tracked := 0
	for _, descriptor := range catalog.Providers {
		if descriptor.Tracked {
			tracked++
		}
	}
	if tracked != 24 {
		t.Fatalf("tracked = %d", tracked)
	}
}

type spyRunner struct{ calls int }

func (s *spyRunner) Run(context.Context, string, ...string) ([]byte, error) {
	s.calls++
	return []byte("demo"), nil
}

type failingRunner struct{}

func (failingRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return nil, errors.New("blocked external execution")
}

func TestRuntimeUsesInjectedRunner(t *testing.T) {
	runner := &spyRunner{}
	if _, err := NewRuntime(ScenarioSubscription, Deps{Runner: runner}); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want startup version probe", runner.calls)
	}
	if _, err := NewRuntime(ScenarioSubscription, Deps{Runner: failingRunner{}}); err == nil {
		t.Fatal("failing runner was not surfaced at startup")
	}
}

func TestScenarioSwitchPreservesInjectedFixtures(t *testing.T) {
	spy := &spyFS{base: FixtureFS()}
	runtime, err := NewRuntime(ScenarioSubscription, Deps{Fixtures: spy})
	if err != nil {
		t.Fatal(err)
	}
	before := len(spy.reads)
	if err := runtime.SetScenario(ScenarioAttention); err != nil {
		t.Fatal(err)
	}
	runtime.Snapshots(context.Background())
	if len(spy.reads) <= before {
		t.Fatal("scenario switch did not reload through the injected fixture filesystem")
	}
	if len(spy.reads) == 0 {
		t.Fatal("scenario switch bypassed the injected fixture filesystem")
	}
	for _, path := range spy.reads {
		if path != "fixtures.json" {
			t.Fatalf("scenario switch read %q", path)
		}
	}
}
