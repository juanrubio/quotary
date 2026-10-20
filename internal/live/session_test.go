package live

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/domain"
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
	if s.BackendError() == "" {
		t.Fatal("catalog failure not surfaced")
	}
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
}

func (b *blockingHistoryBackend) History(ctx context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
	close(b.started)
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
	select {
	case <-s.Events():
	case <-time.After(2 * time.Second):
		t.Fatal("failure did not signal event")
	}
	snapshots := s.Snapshots(context.Background())
	if len(snapshots) != 1 || snapshots[0].Error == nil || snapshots[0].State == domain.StatusQueued {
		t.Fatalf("failure state not applied: %+v", snapshots)
	}
}

func TestLiveCallsVersionExactlyOnce(t *testing.T) {
	be := &versionCountingBackend{fakeBackend: &fakeBackend{}}
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, be, prefs)
	defer func() { _ = s.Close() }()
	if be.versionCalls != 1 || s.BackendVersion() != "test" {
		t.Fatalf("version calls=%d version=%q", be.versionCalls, s.BackendVersion())
	}
}

type versionCountingBackend struct {
	*fakeBackend
	versionCalls int
}

func (b *versionCountingBackend) Version(context.Context) (backend.VersionInfo, error) {
	b.versionCalls++
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
	waitFor(t, func() bool { be.mu.Lock(); defer be.mu.Unlock(); return be.historyCalls == 1 })
	if _, err := s.HistoryWithForce(context.Background(), request, false); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	be.mu.Lock()
	afterCache := be.historyCalls
	be.mu.Unlock()
	if afterCache != 1 {
		t.Fatalf("cached history called backend %d times", afterCache)
	}
	if _, err := s.HistoryWithForce(context.Background(), request, true); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { be.mu.Lock(); defer be.mu.Unlock(); return be.historyCalls == 2 })
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

type partialHistoryBackend struct{ *fakeBackend }

func (b *partialHistoryBackend) History(ctx context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
	key, _ := domain.ParseSourceKey(req.SourceKey)
	total := int64(10)
	return domain.HistorySnapshot{Key: key, Days: req.Days, TotalTokens: &total, Partial: true},
		domain.NewScopedError(domain.ErrProcessFailure, req.SourceKey, "partial")
}

func TestLivePartialHistoryIsCached(t *testing.T) {
	prefs := config.Defaults()
	prefs.Tracked = []string{"codex"}
	s := newLive(t, &partialHistoryBackend{fakeBackend: &fakeBackend{}}, prefs)
	defer func() { _ = s.Close() }()
	request := backend.HistoryRequest{ProviderID: "codex", SourceKey: "codex", Days: 7}
	if _, err := s.HistoryWithForce(context.Background(), request, false); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		history, err := s.HistoryWithForce(context.Background(), request, false)
		return err == nil && history.TotalTokens != nil && history.Partial
	})
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
