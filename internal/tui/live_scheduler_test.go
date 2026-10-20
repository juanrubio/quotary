package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/backend/demo"
	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/domain"
	"github.com/juanrubio/quotary/internal/live"
)

type stressBackend struct{}

func (stressBackend) Version(context.Context) (backend.VersionInfo, error) {
	return backend.VersionInfo{Version: "stress"}, nil
}

func (stressBackend) Catalog(context.Context) (backend.CatalogSnapshot, error) {
	providers := make([]domain.SourceDescriptor, 0, 108)
	for i := 0; i < 108; i++ {
		name := fmt.Sprintf("provider-%03d", i)
		providers = append(providers, domain.SourceDescriptor{
			Key: domain.SourceKey{Provider: name}, ProviderID: name, DisplayName: name, Available: true,
		})
	}
	return backend.CatalogSnapshot{Providers: providers}, nil
}

func (stressBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	select {
	case <-time.After(2 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	key, _ := domain.ParseSourceKey(req.SourceKey)
	percent := 20.0
	return []domain.UsageSnapshot{{
		Key: key, ProviderID: key.Provider, DisplayName: key.Provider, State: domain.StatusCurrent,
		Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}},
	}}, nil
}

func (stressBackend) History(context.Context, backend.HistoryRequest) (domain.HistorySnapshot, error) {
	return domain.HistorySnapshot{}, nil
}

func TestSchedulerBackedTUIStaysResponsiveWith108Sources(t *testing.T) {
	prefs := config.Defaults()
	for i := 0; i < 108; i++ {
		prefs.Tracked = append(prefs.Tracked, fmt.Sprintf("provider-%03d", i))
	}
	store := config.NewMemoryStore(prefs)
	session, err := live.New(context.Background(), live.Options{Backend: stressBackend{}, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	m := newModel(Options{Session: session})
	for _, snapshot := range session.Snapshots(context.Background()) {
		session.StartRefresh(snapshot.Key.String())
	}
	start := time.Now()
	for i := 0; i < 50; i++ {
		m.width, m.height = 80, 24
		_ = m.View().Content
		m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("TUI blocked for %v with 108 sources", elapsed)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		snapshots := session.Snapshots(context.Background())
		current := 0
		for _, snapshot := range snapshots {
			if snapshot.State == domain.StatusCurrent {
				current++
			}
		}
		if current == 108 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	snapshots := session.Snapshots(context.Background())
	current := 0
	for _, snapshot := range snapshots {
		if snapshot.State == domain.StatusCurrent {
			current++
		}
	}
	t.Fatalf("only %d/108 sources completed under scheduler-backed TUI", current)
}

func TestLiveUpdateMessageReloadsRenderedModel(t *testing.T) {
	prefs := config.Defaults()
	prefs.Tracked = []string{"provider-000"}
	store := config.NewMemoryStore(prefs)
	session, err := live.New(context.Background(), live.Options{Backend: stressBackend{}, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	m := newModel(Options{Session: session})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snapshots := session.Snapshots(context.Background())
		if len(snapshots) == 1 && snapshots[0].State == domain.StatusCurrent {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	m.Update(liveUpdateMsg{})
	view := checkFrame(t, m, 120, 40)
	if !strings.Contains(view, "provider-000") {
		t.Fatalf("live event did not refresh rendered model:\n%s", view)
	}
}

func TestBackendVersionHeaderIsSanitized(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	m.backendVersion = "CodexBar\x1b[2Jevil"
	view := checkFrame(t, m, 120, 40)
	if strings.Contains(view, "\x1b[2J") {
		t.Fatal("control sequence from backend version reached header")
	}
}

func TestHeaderVersionLabelNotDuplicated(t *testing.T) {
	for _, tc := range []struct{ version, want string }{
		{"CodexBar 0.78.2", "CodexBar 0.78.2"},
		{"0.78.2", "CodexBar 0.78.2"},
		{"codexbar 1.2.3 beta", "codexbar 1.2.3 beta"},
	} {
		m := demoModel(t, demo.ScenarioSubscription)
		m.backendVersion = tc.version
		view := checkFrame(t, m, 120, 40)
		if !strings.Contains(view, tc.want) {
			t.Fatalf("version %q: header missing %q", tc.version, tc.want)
		}
		if strings.Contains(view, "CodexBar CodexBar") || strings.Contains(view, "CodexBar codexbar") {
			t.Fatalf("version %q: brand duplicated in header", tc.version)
		}
	}
}

func TestUnreadableDataScreenDistinguishesReachableBackend(t *testing.T) {
	m := demoModel(t, demo.ScenarioEmpty)
	m.backendVersion = "CodexBar 0.78.2"
	m.setupError = "The backend returned unreadable data."
	view := checkFrame(t, m, 100, 32)
	if !strings.Contains(view, "CodexBar data unreadable") {
		t.Fatalf("expected reachable-backend empty state:\n%s", view)
	}
	if !strings.Contains(view, "r Refresh") {
		t.Fatal("unreadable-data state should offer a refresh")
	}
	if strings.Contains(view, "Set the executable in Settings") {
		t.Fatal("unreadable-data state gave the not-found setup advice")
	}
}

type ageBackend struct {
	*stressBackend
	mu    sync.Mutex
	calls int
}

func (b *ageBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	key, _ := domain.ParseSourceKey(req.SourceKey)
	sample := time.Now().Add(-time.Second)
	percent := 20.0
	return []domain.UsageSnapshot{{Key: key, ProviderID: key.Provider, DisplayName: key.Provider, State: domain.StatusCurrent, SampleAt: &sample, Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}}}, nil
}

func TestLiveLocalTickAdvancesAgeWithoutBackendTraffic(t *testing.T) {
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	store := config.NewMemoryStore(prefs)
	be := &ageBackend{stressBackend: &stressBackend{}}
	session, err := live.New(context.Background(), live.Options{Backend: be, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	m := newModel(Options{Session: session})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshots := session.Snapshots(context.Background())
		if len(snapshots) == 1 && snapshots[0].State == domain.StatusCurrent {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	m.Update(localTickMsg{})
	before := checkFrame(t, m, 120, 40)
	be.mu.Lock()
	callsBefore := be.calls
	be.mu.Unlock()
	time.Sleep(1100 * time.Millisecond)
	m.Update(localTickMsg{})
	after := checkFrame(t, m, 120, 40)
	be.mu.Lock()
	callsAfter := be.calls
	be.mu.Unlock()
	if before == after {
		t.Fatal("local tick did not update rendered age")
	}
	if callsAfter != callsBefore {
		t.Fatal("local tick caused backend traffic")
	}
}

func TestLiveConfigWarningRendersInShell(t *testing.T) {
	prefs := config.Defaults()
	prefs.Tracked = []string{"provider-000"}
	store := config.NewMemoryStore(prefs)
	session, err := live.New(context.Background(), live.Options{Backend: stressBackend{}, Store: store, StoreWarning: "preferences migrated to current schema"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	m := newModel(Options{Session: session})
	view := checkFrame(t, m, 160, 48)
	if !strings.Contains(view, "preferences migrated") {
		t.Fatalf("store warning not rendered:\n%s", view)
	}
}

type blockedCatalogBackend struct {
	*stressBackend
	started chan struct{}
	release chan struct{}
}

func (b *blockedCatalogBackend) Catalog(ctx context.Context) (backend.CatalogSnapshot, error) {
	close(b.started)
	<-b.release
	return b.stressBackend.Catalog(ctx)
}

func (b *blockedCatalogBackend) Version(ctx context.Context) (backend.VersionInfo, error) {
	return b.stressBackend.Version(ctx)
}

func TestLiveFirstPaintBudgetDespiteSlowDiscovery(t *testing.T) {
	if raceEnabled {
		t.Skip("timing budget is not meaningful under the race detector")
	}
	prefs := config.Defaults()
	prefs.Tracked = []string{"provider-000"}
	store := config.NewMemoryStore(prefs)
	be := &blockedCatalogBackend{stressBackend: &stressBackend{}, started: make(chan struct{}), release: make(chan struct{})}
	start := time.Now()
	session, err := live.New(context.Background(), live.Options{Backend: be, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	m := newModel(Options{Session: session})
	m.width, m.height = 160, 48
	_ = m.View().Content
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("live first paint took %v while discovery was blocked", elapsed)
	}
	select {
	case <-be.started:
	default:
		t.Fatal("discovery did not start in the background")
	}
	close(be.release)
}
