package live

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/domain"
	"github.com/juanrubio/quotary/internal/refresh"
)

type fakeBackend struct {
	mu           sync.Mutex
	usageCalls   int
	historyCalls int
	failUsage    bool
}

func (f *fakeBackend) Version(context.Context) (backend.VersionInfo, error) {
	return backend.VersionInfo{Version: "test"}, nil
}

func (f *fakeBackend) Catalog(context.Context) (backend.CatalogSnapshot, error) {
	return backend.CatalogSnapshot{Providers: []domain.SourceDescriptor{
		{Key: domain.SourceKey{Provider: "codex"}, ProviderID: "codex", DisplayName: "Codex", Available: true},
		{Key: domain.SourceKey{Provider: "claude"}, ProviderID: "claude", DisplayName: "Claude", Available: true},
	}}, nil
}

func (f *fakeBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	f.mu.Lock()
	f.usageCalls++
	fail := f.failUsage
	f.mu.Unlock()
	if fail {
		return nil, domain.NewScopedError(domain.ErrTimeout, req.SourceKey, "slow")
	}
	key, _ := domain.ParseSourceKey(req.SourceKey)
	percent := 20.0
	return []domain.UsageSnapshot{{Key: key, ProviderID: key.Provider, DisplayName: key.Provider, State: domain.StatusCurrent, Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}}}, nil
}

func (f *fakeBackend) History(ctx context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
	f.mu.Lock()
	f.historyCalls++
	f.mu.Unlock()
	key, _ := domain.ParseSourceKey(req.SourceKey)
	total := int64(10)
	return domain.HistorySnapshot{Key: key, Days: req.Days, TotalTokens: &total, Coverage: "test"}, nil
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}

func newLive(t *testing.T, be backend.Backend, prefs config.Preferences) *Session {
	t.Helper()
	store := config.NewMemoryStore(prefs)
	s, err := New(context.Background(), Options{Backend: be, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLiveSessionRefreshesTrackedSources(t *testing.T) {
	be := &fakeBackend{}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, be, prefs)
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool {
		snapshots := s.Snapshots(context.Background())
		return len(snapshots) == 1 && snapshots[0].State == domain.StatusCurrent
	})
	key := "codex"
	s.StartRefresh(key)
	if !s.AdvanceRefresh(key) {
		t.Fatal("refresh completed before backend result")
	}
	waitFor(t, func() bool { return !s.AdvanceRefresh(key) })
	if outcome := s.RefreshOutcome(key); outcome != "Success" {
		t.Fatalf("outcome=%q", outcome)
	}
}

func TestLiveSessionRetainsStaleOnFailure(t *testing.T) {
	be := &fakeBackend{}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, be, prefs)
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool { return s.Snapshots(context.Background())[0].State == domain.StatusCurrent })
	be.mu.Lock()
	be.failUsage = true
	be.mu.Unlock()
	s.StartRefresh("codex")
	waitFor(t, func() bool { return !s.AdvanceRefresh("codex") })
	snapshot := s.Snapshots(context.Background())[0]
	if snapshot.Error == nil || len(snapshot.Windows) != 1 {
		t.Fatalf("retained snapshot=%+v", snapshot)
	}
}

func TestLiveSessionTrackingPinningAndHistory(t *testing.T) {
	be := &fakeBackend{}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, be, prefs)
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool { return len(s.Snapshots(context.Background())) == 1 })
	if err := s.Track("claude", true); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(s.Snapshots(context.Background())) == 2 })
	if err := s.Pin("codex", true); err != nil {
		t.Fatal(err)
	}
	if !contains(s.Preferences().Pins, "codex") {
		t.Fatal("pin not persisted")
	}
	_, err := s.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", SourceKey: "codex", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		history, loadErr := s.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", SourceKey: "codex", Days: 7})
		return loadErr == nil && history.TotalTokens != nil
	})
	if err := s.Track("codex", false); err != nil {
		t.Fatal(err)
	}
	if len(s.Snapshots(context.Background())) != 1 {
		t.Fatal("untracking did not remove snapshot")
	}
}

func TestLiveSessionBackendSetupError(t *testing.T) {
	store := config.NewMemoryStore(config.Defaults())
	s, err := New(context.Background(), Options{Backend: &failingCatalogBackend{}, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool { return s.BackendError() != "" })
	if _, err := New(context.Background(), Options{}); err == nil {
		t.Fatal("missing backend accepted")
	}
}

type accountUnknownBackend struct{ fakeBackend }

func (b *accountUnknownBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	key, _ := domain.ParseSourceKey(req.SourceKey)
	key.Account = domain.UnknownAccount
	key.AccountUnknown = true
	percent := 20.0
	return []domain.UsageSnapshot{{Key: key, ProviderID: key.Provider, DisplayName: key.Provider, State: domain.StatusCurrent, Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}}}, nil
}

func TestLiveReconcilesBareTrackedKeyWithReturnedUnknownAccount(t *testing.T) {
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, &accountUnknownBackend{}, prefs)
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool {
		return len(s.Snapshots(context.Background())) == 1 && s.Snapshots(context.Background())[0].State == domain.StatusCurrent
	})
	if key := s.Snapshots(context.Background())[0].Key.String(); key != "codex@unknown" {
		t.Fatalf("reconciled key=%q", key)
	}
	s.StartRefresh("codex@unknown")
	waitFor(t, func() bool { return !s.AdvanceRefresh("codex@unknown") })
	if len(s.Snapshots(context.Background())) != 1 {
		t.Fatalf("skeleton key diverged: %+v", s.Snapshots(context.Background()))
	}
}

type blockingBackend struct {
	started chan struct{}
	release chan struct{}
	fakeBackend
}

func (b *blockingBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	close(b.started)
	<-b.release
	return b.fakeBackend.Usage(ctx, req)
}

func TestUntrackDuringFinishDoesNotResurrectSnapshot(t *testing.T) {
	be := &blockingBackend{started: make(chan struct{}), release: make(chan struct{})}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	store := config.NewMemoryStore(prefs)
	s, err := New(context.Background(), Options{Backend: be, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	<-be.started
	close(be.release)
	if err := s.Track("codex", false); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if len(s.Snapshots(context.Background())) != 0 {
		t.Fatalf("late result resurrected snapshot: %+v", s.Snapshots(context.Background()))
	}
}

func TestIntervalChangeSchedulesWithoutRestart(t *testing.T) {
	be := &fakeBackend{}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	prefs.IntervalSeconds = 300
	store := config.NewMemoryStore(prefs)
	s, err := New(context.Background(), Options{
		Backend: be,
		Store:   store,
		Interval: func() time.Duration {
			return time.Duration(store.Load().IntervalSeconds) * time.Millisecond
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool { be.mu.Lock(); defer be.mu.Unlock(); return be.usageCalls >= 2 })
	be.mu.Lock()
	before := be.usageCalls
	be.mu.Unlock()
	updated := prefs
	updated.IntervalSeconds = 60
	if err := store.Save(updated); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { be.mu.Lock(); defer be.mu.Unlock(); return be.usageCalls > before })
}

type failingCatalogBackend struct{ fakeBackend }

func (f *failingCatalogBackend) Catalog(context.Context) (backend.CatalogSnapshot, error) {
	return backend.CatalogSnapshot{}, domain.NewScopedError(domain.ErrConfig, "", "missing codexbar")
}

func TestLiveManualBulkRetriesAuthAwaitingSource(t *testing.T) {
	be := &fakeBackend{}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	store := config.NewMemoryStore(prefs)
	s, err := New(context.Background(), Options{Backend: &authOnceBackend{fakeBackend: be}, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool {
		entry := s.scheduler.State(domain.SourceKey{Provider: "codex"})
		return entry.AwaitingExplicitRetry
	})
	s.StartBulk([]string{"codex"})
	waitFor(t, func() bool {
		snapshots := s.Snapshots(context.Background())
		return len(snapshots) == 1 && snapshots[0].State == domain.StatusCurrent
	})
}

type authOnceBackend struct {
	*fakeBackend
	calls int
}

func (b *authOnceBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	b.calls++
	if b.calls == 1 {
		return nil, domain.NewScopedError(domain.ErrAuth, req.SourceKey, "expired")
	}
	return b.fakeBackend.Usage(ctx, req)
}

func TestLiveHistoryLoadDoesNotBlockEventLoop(t *testing.T) {
	be := &blockingHistoryBackend{fakeBackend: &fakeBackend{}, started: make(chan struct{}), release: make(chan struct{})}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, be, prefs)
	defer func() { _ = s.Close() }()
	start := time.Now()
	_, err := s.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", SourceKey: "codex", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("history load blocked for %v", elapsed)
	}
	<-be.started
	close(be.release)
	waitFor(t, func() bool {
		history, loadErr := s.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", SourceKey: "codex", Days: 7})
		return loadErr == nil && history.TotalTokens != nil
	})
}

type blockingHistoryBackend struct {
	*fakeBackend
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (b *blockingHistoryBackend) History(ctx context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
	b.once.Do(func() { close(b.started) })
	<-b.release
	return b.fakeBackend.History(ctx, req)
}

type multiAccountBackend struct{ *fakeBackend }

func (b *multiAccountBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	percent := 20.0
	return []domain.UsageSnapshot{
		{Key: domain.SourceKey{Provider: "codex", Account: "one"}, ProviderID: "codex", DisplayName: "Codex", State: domain.StatusCurrent, Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}},
		{Key: domain.SourceKey{Provider: "codex", Account: "two"}, ProviderID: "codex", DisplayName: "Codex", State: domain.StatusCurrent, Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}},
	}, nil
}

func TestLiveMultiAccountReplacesBareSkeletonAndRefreshesAccount(t *testing.T) {
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, &multiAccountBackend{fakeBackend: &fakeBackend{}}, prefs)
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool { return len(s.Snapshots(context.Background())) == 2 })
	for _, snapshot := range s.Snapshots(context.Background()) {
		if snapshot.Key.String() == "codex" {
			t.Fatalf("bare skeleton survived: %+v", s.Snapshots(context.Background()))
		}
	}
	s.StartRefresh("codex@one")
	waitFor(t, func() bool { return !s.AdvanceRefresh("codex@one") })
	if len(s.Snapshots(context.Background())) != 2 {
		t.Fatalf("account refresh changed collection: %+v", s.Snapshots(context.Background()))
	}
}

type authFailBackend struct{ *fakeBackend }

func (b *authFailBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	return nil, domain.NewScopedError(domain.ErrAuth, req.SourceKey, "expired")
}

func TestLiveFailureSignalsEvents(t *testing.T) {
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, &authFailBackend{fakeBackend: &fakeBackend{}}, prefs)
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool {
		snapshots := s.Snapshots(context.Background())
		return len(snapshots) == 1 && snapshots[0].Error != nil && snapshots[0].State != domain.StatusQueued
	})
	for {
		select {
		case <-s.Events():
			continue
		default:
		}
		break
	}
	s.StartRefresh("codex")
	select {
	case <-s.Events():
	case <-time.After(2 * time.Second):
		t.Fatal("failure did not signal Events")
	}
}

func TestLiveCallsVersionExactlyOnce(t *testing.T) {
	be := &versionCountingBackend{fakeBackend: &fakeBackend{}}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, be, prefs)
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool { return be.versionCalls.Load() == 1 && s.BackendVersion() == "test" })
}

type versionCountingBackend struct {
	*fakeBackend
	versionCalls atomic.Int32
}

func (b *versionCountingBackend) Version(context.Context) (backend.VersionInfo, error) {
	b.versionCalls.Add(1)
	return backend.VersionInfo{Version: "test"}, nil
}

func TestLiveAutomaticFailureOverlaysEstablishedUnknownSnapshot(t *testing.T) {
	be := &successThenFailBackend{fakeBackend: &fakeBackend{}}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, be, prefs)
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool {
		snapshots := s.Snapshots(context.Background())
		return len(snapshots) == 1 && snapshots[0].Key.String() == "codex@unknown" && snapshots[0].State == domain.StatusCurrent
	})
	s.enqueueAutomatic(domain.SourceKey{Provider: "codex"}, 0)
	waitFor(t, func() bool {
		snapshots := s.Snapshots(context.Background())
		return len(snapshots) == 1 && snapshots[0].Error != nil
	})
	snapshots := s.Snapshots(context.Background())
	if len(snapshots) != 1 || snapshots[0].Key.String() != "codex@unknown" || snapshots[0].State == domain.StatusCurrent {
		t.Fatalf("failure split identity: %+v", snapshots)
	}
}

type successThenFailBackend struct {
	*fakeBackend
	mu    sync.Mutex
	calls int
}

func (b *successThenFailBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	b.mu.Lock()
	b.calls++
	call := b.calls
	b.mu.Unlock()
	if call > 1 {
		return nil, domain.NewScopedError(domain.ErrTimeout, req.SourceKey, "slow")
	}
	key, _ := domain.ParseSourceKey(req.SourceKey)
	key.Account = domain.UnknownAccount
	key.AccountUnknown = true
	percent := 20.0
	return []domain.UsageSnapshot{{Key: key, ProviderID: key.Provider, DisplayName: key.Provider, State: domain.StatusCurrent, Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}}}, nil
}

func TestLiveFastBulkRecordsFinalProgress(t *testing.T) {
	be := &fakeBackend{}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex", "claude"}
	s := newLive(t, be, prefs)
	defer func() { _ = s.Close() }()
	id := s.StartBulk([]string{"codex", "claude"})
	if id == "" {
		t.Fatal("no bulk id")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		done, total, active := s.BulkProgress()
		if !active && done == total && total == 2 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	done, total, active := s.BulkProgress()
	t.Fatalf("bulk progress done=%d total=%d active=%v snapshots=%+v", done, total, active, s.Snapshots(context.Background()))
}

func TestLiveHistoryForceReloadsWithinTTL(t *testing.T) {
	be := &fakeBackend{}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, be, prefs)
	defer func() { _ = s.Close() }()
	request := backend.HistoryRequest{ProviderID: "codex", SourceKey: "codex", Days: 7}
	if _, err := s.HistoryWithForce(context.Background(), request, false); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		history, err := s.HistoryWithForce(context.Background(), request, false)
		return err == nil && history.Key.Provider != ""
	})
	be.mu.Lock()
	cachedCalls := be.historyCalls
	be.mu.Unlock()
	if _, err := s.HistoryWithForce(context.Background(), request, false); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	be.mu.Lock()
	afterCache := be.historyCalls
	be.mu.Unlock()
	if afterCache != cachedCalls {
		t.Fatalf("cached history called backend %d times, want %d", afterCache, cachedCalls)
	}
	if _, err := s.HistoryWithForce(context.Background(), request, true); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { be.mu.Lock(); defer be.mu.Unlock(); return be.historyCalls == cachedCalls+1 })
}

func TestLiveBulkProgressCompletesAfterUntrackingQueuedMember(t *testing.T) {
	be := &slowBackend{fakeBackend: &fakeBackend{}}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex", "claude"}
	s := newLive(t, be, prefs)
	defer func() { _ = s.Close() }()
	s.StartBulk([]string{"codex", "claude"})
	if err := s.Track("claude", false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		done, total, active := s.BulkProgress()
		if !active && done == total && total == 2 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	done, total, active := s.BulkProgress()
	t.Fatalf("bulk progress done=%d total=%d active=%v", done, total, active)
}

type slowBackend struct{ *fakeBackend }

func (b *slowBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	time.Sleep(20 * time.Millisecond)
	return b.fakeBackend.Usage(ctx, req)
}

type gatedOrderBackend struct {
	*fakeBackend
	mu      sync.Mutex
	started chan struct{}
	release chan struct{}
	order   []string
}

func (b *gatedOrderBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	b.mu.Lock()
	b.order = append(b.order, req.SourceKey)
	index := len(b.order)
	b.mu.Unlock()
	if index <= 3 {
		b.started <- struct{}{}
		<-b.release
	}
	return b.fakeBackend.Usage(ctx, req)
}

func TestLivePrioritizeSelectedAheadOfBackground(t *testing.T) {
	be := &gatedOrderBackend{fakeBackend: &fakeBackend{}, started: make(chan struct{}, 3), release: make(chan struct{})}
	prefs := config.Defaults()
	for i := 0; i < 10; i++ {
		prefs.Tracked = append(prefs.Tracked, fmt.Sprintf("provider-%02d", i))
	}
	s := newLive(t, be, prefs)
	defer func() { _ = s.Close() }()
	for i := 0; i < 10; i++ {
		s.StartRefresh(fmt.Sprintf("provider-%02d", i))
	}
	for i := 0; i < 3; i++ {
		select {
		case <-be.started:
		case <-time.After(2 * time.Second):
			t.Fatal("initial workers did not start")
		}
	}
	s.Prioritize([]string{"provider-09"}, "provider-09")
	close(be.release)
	waitFor(t, func() bool { be.mu.Lock(); defer be.mu.Unlock(); return len(be.order) >= 4 })
	be.mu.Lock()
	fourth := be.order[3]
	be.mu.Unlock()
	if fourth != "provider-09" {
		t.Fatalf("selected source not prioritized: order=%v", be.order)
	}
}

type accountSwitchBackend struct {
	*fakeBackend
	mu    sync.Mutex
	calls int
}

func (b *accountSwitchBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	b.mu.Lock()
	b.calls++
	call := b.calls
	b.mu.Unlock()
	account := "one"
	if call > 1 {
		account = "two"
	}
	percent := 20.0
	return []domain.UsageSnapshot{{Key: domain.SourceKey{Provider: "codex", Account: account}, ProviderID: "codex", DisplayName: "Codex", State: domain.StatusCurrent, Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}}}, nil
}

func TestLiveSequentialAccountChangePrunesOldSnapshot(t *testing.T) {
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, &accountSwitchBackend{fakeBackend: &fakeBackend{}}, prefs)
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool {
		snapshots := s.Snapshots(context.Background())
		return len(snapshots) == 1 && snapshots[0].Key.Account == "one"
	})
	s.StartRefresh("codex@one")
	waitFor(t, func() bool {
		snapshots := s.Snapshots(context.Background())
		return len(snapshots) == 1 && snapshots[0].Key.Account == "two"
	})
	if len(s.Snapshots(context.Background())) != 1 {
		t.Fatalf("old account retained: %+v", s.Snapshots(context.Background()))
	}
}

type partialUsageBackend struct{ *fakeBackend }

func (b *partialUsageBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	key, _ := domain.ParseSourceKey(req.SourceKey)
	percent := 20.0
	return []domain.UsageSnapshot{{Key: key, ProviderID: key.Provider, DisplayName: key.Provider, Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}}},
		domain.NewScopedError(domain.ErrProcessFailure, req.SourceKey, "nonzero")
}

func TestLiveAppliesPartialSnapshotsWithError(t *testing.T) {
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, &partialUsageBackend{fakeBackend: &fakeBackend{}}, prefs)
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool {
		snapshots := s.Snapshots(context.Background())
		return len(snapshots) == 1 && len(snapshots[0].Windows) == 1 && snapshots[0].Error != nil
	})
	snapshot := s.Snapshots(context.Background())[0]
	if snapshot.State == domain.StatusCurrent || snapshot.State == domain.StatusQueued {
		t.Fatalf("partial failure presented current: %+v", snapshot)
	}
}

type partialHistoryBackend struct {
	*fakeBackend
	calls atomic.Int32
}

func (b *partialHistoryBackend) History(ctx context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
	b.calls.Add(1)
	key, _ := domain.ParseSourceKey(req.SourceKey)
	total := int64(10)
	return domain.HistorySnapshot{Key: key, Days: req.Days, TotalTokens: &total, Partial: true},
		domain.NewScopedError(domain.ErrProcessFailure, req.SourceKey, "partial")
}

func TestLivePartialHistoryIsCached(t *testing.T) {
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	be := &partialHistoryBackend{fakeBackend: &fakeBackend{}}
	s := newLive(t, be, prefs)
	defer func() { _ = s.Close() }()
	request := backend.HistoryRequest{ProviderID: "codex", SourceKey: "codex", Days: 7}
	if _, err := s.HistoryWithForce(context.Background(), request, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		history, err := s.HistoryWithForce(context.Background(), request, false)
		if err == nil && history.TotalTokens != nil && history.Partial {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	history, err := s.HistoryWithForce(context.Background(), request, false)
	cached, ok := s.scheduler.CachedHistoryAny(refresh.Request{Key: domain.SourceKey{Provider: "codex"}, Operation: refresh.OpHistory, Days: 7, Context: "live"})
	t.Fatalf("partial history not cached: %+v err=%v cacheAny=%+v ok=%v loading=%v errors=%v", history, err, cached, ok, s.historyLoading, s.historyErrors)
}

type adapterShapedFailureBackend struct{ *fakeBackend }

func (b *adapterShapedFailureBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	key, _ := domain.ParseSourceKey(req.SourceKey)
	key.Account = domain.UnknownAccount
	key.AccountUnknown = true
	scoped := domain.NewScopedError(domain.ErrAuth, key.String(), "expired")
	return []domain.UsageSnapshot{{Key: key, ProviderID: key.Provider, Error: scoped, State: domain.StatusSignIn}}, scoped
}

func TestLiveAdapterShapedFailureReplacesSkeleton(t *testing.T) {
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, &adapterShapedFailureBackend{fakeBackend: &fakeBackend{}}, prefs)
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool {
		snapshots := s.Snapshots(context.Background())
		return len(snapshots) == 1 && snapshots[0].Error != nil
	})
	if key := s.Snapshots(context.Background())[0].Key.String(); key != "codex@unknown" {
		t.Fatalf("skeleton not reconciled: %s", key)
	}
}

type seedBackend struct{ *fakeBackend }

func (b *seedBackend) Catalog(context.Context) (backend.CatalogSnapshot, error) {
	return backend.CatalogSnapshot{Providers: []domain.SourceDescriptor{
		{Key: domain.SourceKey{Provider: "codex"}, ProviderID: "codex", DisplayName: "Codex", Available: true, Enabled: true},
		{Key: domain.SourceKey{Provider: "openai"}, ProviderID: "openai", DisplayName: "OpenAI", Available: true, Enabled: true},
		{Key: domain.SourceKey{Provider: "gemini"}, ProviderID: "gemini", DisplayName: "Gemini", Available: true, Enabled: true},
	}}, nil
}

func TestLiveFirstRunSeedsOnlyCompatibleEnabledSources(t *testing.T) {
	store := config.NewMemoryStore(config.Defaults())
	s, err := New(context.Background(), Options{Backend: &seedBackend{fakeBackend: &fakeBackend{}}, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool { return len(s.Preferences().Tracked) > 0 })
	prefs := s.Preferences()
	if !contains(prefs.Tracked, "codex") || contains(prefs.Tracked, "openai") || !contains(prefs.Tracked, "gemini:configured") || contains(prefs.Tracked, "gemini:web") || contains(prefs.Tracked, "gemini:api") || contains(prefs.Tracked, "gemini") {
		t.Fatalf("seeded tracked=%v", prefs.Tracked)
	}
	catalog, _ := s.Catalog(context.Background())
	seen := map[string]bool{}
	for _, descriptor := range catalog.Providers {
		seen[descriptor.Key.String()] = true
	}
	if seen["gemini"] || !seen["gemini:configured"] || !seen["gemini:web"] || !seen["gemini:api"] {
		t.Fatalf("catalog variants=%v", seen)
	}
}

type gatingBackend struct {
	*fakeBackend
	mu         sync.Mutex
	calls      int
	callsByKey map[string]int
}

func (b *gatingBackend) Catalog(context.Context) (backend.CatalogSnapshot, error) {
	return backend.CatalogSnapshot{Providers: []domain.SourceDescriptor{
		{Key: domain.SourceKey{Provider: "codex"}, ProviderID: "codex", DisplayName: "Codex", Available: true, Enabled: true},
		{Key: domain.SourceKey{Provider: "azureopenai"}, ProviderID: "azureopenai", DisplayName: "Azure", Available: true, Enabled: true},
		{Key: domain.SourceKey{Provider: "openrouter"}, ProviderID: "openrouter", DisplayName: "OpenRouter", Available: true, Enabled: true},
		{Key: domain.SourceKey{Provider: "unknown-provider"}, ProviderID: "unknown-provider", DisplayName: "Unknown", Available: true, Enabled: true},
	}}, nil
}

func (b *gatingBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	b.mu.Lock()
	b.calls++
	if b.callsByKey == nil {
		b.callsByKey = map[string]int{}
	}
	b.callsByKey[req.SourceKey]++
	b.mu.Unlock()
	return b.fakeBackend.Usage(ctx, req)
}

func (b *gatingBackend) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

func (b *gatingBackend) countFor(key string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.callsByKey[key]
}

func TestLiveAutomaticPollingSkipsUnknownAndProbe(t *testing.T) {
	be := &gatingBackend{fakeBackend: &fakeBackend{}}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex", "openrouter", "azureopenai", "unknown-provider"}
	store := config.NewMemoryStore(prefs)
	s, err := New(context.Background(), Options{Backend: be, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool { return be.count() >= 2 })
	time.Sleep(80 * time.Millisecond)
	if calls := be.count(); calls != 2 {
		t.Fatalf("automatic polling called backend %d times; want codex+openrouter only", calls)
	}
	s.StartRefresh("azureopenai")
	if calls := be.count(); calls != 2 {
		t.Fatal("manual active probe fired a backend call")
	}
	waitFor(t, func() bool {
		for _, snapshot := range s.Snapshots(context.Background()) {
			if snapshot.Key.Provider == "azureopenai" && snapshot.State == domain.StatusBackendIncompatible {
				return true
			}
		}
		return false
	})
}

func TestLiveSeedingRequiresGenuineFirstRun(t *testing.T) {
	store := config.NewMemoryStore(config.Defaults())
	if err := store.Save(config.Defaults()); err != nil {
		t.Fatal(err)
	}
	be := &seedBackend{fakeBackend: &fakeBackend{}}
	s, err := New(context.Background(), Options{Backend: be, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if tracked := s.Preferences().Tracked; len(tracked) != 0 {
		t.Fatalf("explicitly empty tracking reseeded: %v", tracked)
	}
}

func TestLiveMigratesGenericGeminiTrackedKey(t *testing.T) {
	for _, tracked := range [][]string{{"gemini"}, {"gemini", "gemini:web@unknown"}} {
		store := config.NewMemoryStore(config.Defaults())
		prefs := config.Defaults()
		prefs.Tracked = tracked
		if err := store.Save(prefs); err != nil {
			t.Fatal(err)
		}
		s, err := New(context.Background(), Options{Backend: &seedBackend{fakeBackend: &fakeBackend{}}, Store: store})
		if err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool { return !contains(s.Preferences().Tracked, "gemini") })
		got := s.Preferences().Tracked
		_ = s.Close()
		if contains(got, "gemini") {
			t.Fatalf("generic gemini retained: %v", got)
		}
		if len(tracked) == 1 && !contains(got, "gemini:configured") {
			t.Fatalf("configured variant not seeded: %v", got)
		}
		if len(tracked) > 1 && !contains(got, "gemini:web@unknown") {
			t.Fatalf("established variant lost: %v", got)
		}
	}
}

func TestLiveStoreWarningPropagates(t *testing.T) {
	store := config.NewMemoryStore(config.Defaults())
	s, err := New(context.Background(), Options{Backend: &fakeBackend{}, Store: store, StoreWarning: "preferences migrated"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if s.ConfigWarning() != "preferences migrated" {
		t.Fatalf("warning=%q", s.ConfigWarning())
	}
}

type knownAccountBackend struct {
	*fakeBackend
	mu    sync.Mutex
	calls int
}

func (b *knownAccountBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	key, _ := domain.ParseSourceKey(req.SourceKey)
	key.Account = "one"
	key.AccountUnknown = false
	percent := 20.0
	return []domain.UsageSnapshot{{Key: key, ProviderID: key.Provider, DisplayName: key.Provider, State: domain.StatusCurrent, Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}}}, nil
}

func TestLiveIntervalRefreshesEstablishedAccountSource(t *testing.T) {
	be := &knownAccountBackend{fakeBackend: &fakeBackend{}}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	prefs.IntervalSeconds = 300
	store := config.NewMemoryStore(prefs)
	s, err := New(context.Background(), Options{
		Backend: be,
		Store:   store,
		Interval: func() time.Duration {
			return 20 * time.Millisecond
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	time.Sleep(300 * time.Millisecond)
	be.mu.Lock()
	calls := be.calls
	be.mu.Unlock()
	if calls < 2 {
		t.Fatalf("interval calls=%d snapshots=%+v", calls, s.Snapshots(context.Background()))
	}
}

func TestLiveBulkExcludesActiveProbe(t *testing.T) {
	be := &gatingBackend{fakeBackend: &fakeBackend{}}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex", "azureopenai"}
	store := config.NewMemoryStore(prefs)
	s, err := New(context.Background(), Options{Backend: be, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool { return be.count() == 1 })
	s.StartBulk([]string{"codex", "azureopenai"})
	waitFor(t, func() bool {
		done, total, active := s.BulkProgress()
		return !active && done == total && total == 2
	})
	time.Sleep(40 * time.Millisecond)
	if calls := be.countFor("azureopenai"); calls != 0 {
		t.Fatalf("bulk fired active probe; calls=%d", calls)
	}
}

func TestLiveMigratesGenericGeminiPins(t *testing.T) {
	store := config.NewMemoryStore(config.Defaults())
	prefs := config.Defaults()
	prefs.Tracked = []string{"gemini"}
	prefs.Pins = []string{"gemini"}
	if err := store.Save(prefs); err != nil {
		t.Fatal(err)
	}
	s, err := New(context.Background(), Options{Backend: &seedBackend{fakeBackend: &fakeBackend{}}, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	waitFor(t, func() bool { return !contains(s.Preferences().Pins, "gemini") })
	pins := s.Preferences().Pins
	if contains(pins, "gemini") || !contains(pins, "gemini:configured") {
		t.Fatalf("pins=%v", pins)
	}
}

func TestLivePartialHistoryExpiresAndReloads(t *testing.T) {
	var ticks atomic.Int64
	ticks.Store(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC).UnixNano())
	be := &partialHistoryBackend{fakeBackend: &fakeBackend{}}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	store := config.NewMemoryStore(prefs)
	s, err := New(context.Background(), Options{
		Backend:    be,
		Store:      store,
		Now:        func() time.Time { return time.Unix(0, ticks.Load()) },
		HistoryTTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	request := backend.HistoryRequest{ProviderID: "codex", SourceKey: "codex", Days: 7}
	if _, err := s.HistoryWithForce(context.Background(), request, false); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		history, loadErr := s.HistoryWithForce(context.Background(), request, false)
		return loadErr == nil && history.TotalTokens != nil && history.Partial
	})
	before := be.calls.Load()
	ticks.Add(int64(2 * time.Minute))
	if _, err := s.HistoryWithForce(context.Background(), request, false); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return be.calls.Load() > before })
}
