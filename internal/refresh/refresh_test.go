package refresh

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

type fakeStore struct {
	mu    sync.Mutex
	prefs config.Preferences
}

func newStore() *fakeStore { return &fakeStore{prefs: config.Defaults()} }

func (s *fakeStore) Load() config.Preferences {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prefs
}

func (s *fakeStore) setInterval(seconds int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prefs.IntervalSeconds = seconds
}

type fakeBackend struct {
	mu           sync.Mutex
	usageCalls   int
	historyCalls int
	active       int
	maxActive    int
	order        []string
	usageFn      func(context.Context, backend.UsageRequest, int) ([]domain.UsageSnapshot, error)
	historyFn    func(context.Context, backend.HistoryRequest) (domain.HistorySnapshot, error)
}

func (f *fakeBackend) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	f.mu.Lock()
	f.usageCalls++
	call := f.usageCalls
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.order = append(f.order, req.SourceKey)
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.active--
		f.mu.Unlock()
	}()
	if f.usageFn != nil {
		return f.usageFn(ctx, req, call)
	}
	return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey), DisplayName: req.SourceKey, State: domain.StatusCurrent}}, nil
}

func (f *fakeBackend) History(ctx context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
	f.mu.Lock()
	f.historyCalls++
	call := f.historyCalls
	f.mu.Unlock()
	if f.historyFn != nil {
		return f.historyFn(ctx, req)
	}
	return domain.HistorySnapshot{Key: mustKey(req.SourceKey), Days: req.Days, Coverage: fmt.Sprintf("call-%d", call)}, nil
}

func (f *fakeBackend) counts() (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.usageCalls, f.historyCalls, f.maxActive
}

func mustKey(value string) domain.SourceKey {
	key, err := domain.ParseSourceKey(value)
	if err != nil {
		panic(err)
	}
	return key
}

func testScheduler(be Backend, store Store, opts Options) *Scheduler {
	if opts.Stagger == 0 {
		opts.Stagger = time.Nanosecond
	}
	return New(be, store, opts)
}

func waitResult(t *testing.T, results <-chan Result) Result {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for result")
		return Result{}
	}
}

func TestConcurrencyCeilingAndStress(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		time.Sleep(time.Millisecond)
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey), DisplayName: req.SourceKey, State: domain.StatusCurrent}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 3})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results := s.Run(ctx)
	for i := 0; i < 107; i++ {
		s.Enqueue(Request{Key: domain.SourceKey{Provider: fmt.Sprintf("provider-%03d", i)}, Context: "stress"})
	}
	seen := map[string]bool{}
	for i := 0; i < 107; i++ {
		result := waitResult(t, results)
		if result.Dropped {
			t.Fatalf("stress result dropped: %+v", result)
		}
		seen[result.Key.String()] = true
	}
	if len(seen) != 107 {
		t.Fatalf("completed %d/107", len(seen))
	}
	_, _, maxActive := be.counts()
	if maxActive > 3 {
		t.Fatalf("max concurrency=%d", maxActive)
	}
}

func TestCoalescesUnknownAndDeduplicatesInflight(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		time.Sleep(10 * time.Millisecond)
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey)}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 2})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results := s.Run(ctx)
	unknown := []domain.SourceKey{
		{Provider: "muse", Account: domain.UnknownAccount, AccountUnknown: true},
		{Provider: "muse", Account: domain.UnknownAccount, AccountUnknown: true},
	}
	for _, key := range unknown {
		s.Enqueue(Request{Key: key, Context: "same"})
	}
	s.Enqueue(Request{Key: domain.SourceKey{Provider: "codex", Account: "one"}, Context: "same"})
	s.Enqueue(Request{Key: domain.SourceKey{Provider: "codex", Account: "one"}, Context: "same"})
	_ = waitResult(t, results)
	_ = waitResult(t, results)
	calls, _, _ := be.counts()
	if calls != 2 {
		t.Fatalf("usage calls=%d, want 2 (unknown coalesced, known deduped)", calls)
	}
}

func TestPrioritySelectedBeforeBackground(t *testing.T) {
	be := &fakeBackend{}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1})
	s.Enqueue(Request{Key: domain.SourceKey{Provider: "background"}, Priority: 10, Context: "c"})
	s.Enqueue(Request{Key: domain.SourceKey{Provider: "selected"}, Priority: 0, Context: "c"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results := s.Run(ctx)
	first := waitResult(t, results)
	if first.Key.Provider != "selected" {
		t.Fatalf("first result=%s, want selected", first.Key)
	}
}

func TestChangedContextDropsLateResult(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		if call == 1 {
			close(started)
			<-release
			return []domain.UsageSnapshot{{DisplayName: "old"}}, nil
		}
		return []domain.UsageSnapshot{{DisplayName: "new"}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := s.Run(ctx)
	key := domain.SourceKey{Provider: "codex"}
	s.Enqueue(Request{Key: key, Context: "A"})
	<-started
	s.Enqueue(Request{Key: key, Context: "B"})
	close(release)
	first := waitResult(t, results)
	if !first.Dropped || first.Context != "A" {
		t.Fatalf("first result=%+v", first)
	}
	second := waitResult(t, results)
	if second.Dropped || second.Context != "B" || len(second.Snapshots) != 1 || second.Snapshots[0].DisplayName != "new" {
		t.Fatalf("second result=%+v", second)
	}
}

func TestUntrackedResultDroppedWithoutState(t *testing.T) {
	be := &fakeBackend{}
	s := testScheduler(be, newStore(), Options{Tracked: func(domain.SourceKey) bool { return false }})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results := s.Run(ctx)
	key := domain.SourceKey{Provider: "codex"}
	s.Enqueue(Request{Key: key, Context: "c"})
	result := waitResult(t, results)
	if !result.Dropped {
		t.Fatalf("result=%+v", result)
	}
	if entry := s.State(key); entry.LastSuccess != nil || entry.LastAttempt != nil {
		t.Fatalf("dropped result updated state: %+v", entry)
	}
}

func TestRetainsLastSuccessAndBackoff(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		if call == 1 {
			return []domain.UsageSnapshot{{DisplayName: "success"}}, nil
		}
		return nil, domain.NewScopedError(domain.ErrTimeout, req.SourceKey, "slow")
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1, Jitter: func(time.Duration) time.Duration { return 0 }})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := s.Run(ctx)
	key := domain.SourceKey{Provider: "codex", Account: "one"}
	s.Enqueue(Request{Key: key, Context: "c"})
	_ = waitResult(t, results)
	entry := s.State(key)
	if entry.LastSuccess == nil || len(entry.Snapshots) != 1 {
		t.Fatalf("success state=%+v", entry)
	}
	s.Enqueue(Request{Key: key, Context: "c"})
	_ = waitResult(t, results)
	entry = s.State(key)
	if entry.LastSuccess == nil || entry.Err == nil || entry.BackoffUntil == nil || !entry.BackoffUntil.After(time.Now()) {
		t.Fatalf("failure state=%+v", entry)
	}
}

func TestAuthFailureAwaitsExplicitRetry(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		return nil, domain.NewScopedError(domain.ErrAuth, req.SourceKey, "expired")
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results := s.Run(ctx)
	key := domain.SourceKey{Provider: "codex"}
	s.Enqueue(Request{Key: key, Context: "c"})
	_ = waitResult(t, results)
	entry := s.State(key)
	if !entry.AwaitingExplicitRetry || entry.BackoffUntil != nil {
		t.Fatalf("auth state=%+v", entry)
	}
}

func TestBulkProgressAndSingleSweep(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		time.Sleep(time.Millisecond)
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey)}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 3})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := s.Run(ctx)
	keys := make([]domain.SourceKey, 5)
	for i := range keys {
		keys[i] = domain.SourceKey{Provider: fmt.Sprintf("bulk-%d", i)}
	}
	id := s.StartBulk(keys, "bulk-context")
	if again := s.StartBulk(keys, "bulk-context"); again != id {
		t.Fatalf("second bulk id=%q, want %q", again, id)
	}
	completed := map[int]bool{}
	for i := 0; i < len(keys); i++ {
		result := waitResult(t, results)
		if result.BulkID != id || result.Total != len(keys) || result.Completed < 1 {
			t.Fatalf("bulk result=%+v", result)
		}
		completed[result.Completed] = true
	}
	for i := 1; i <= len(keys); i++ {
		if !completed[i] {
			t.Fatalf("missing completed=%d: %v", i, completed)
		}
	}
}

func TestHistoryCacheAndForceRefresh(t *testing.T) {
	be := &fakeBackend{}
	s := testScheduler(be, newStore(), Options{})
	ctx := context.Background()
	key := domain.SourceKey{Provider: "codex"}
	first, err := s.LoadHistory(ctx, Request{Key: key, Days: 7}, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.LoadHistory(ctx, Request{Key: key, Days: 7}, false)
	if err != nil || second.Coverage != first.Coverage {
		t.Fatalf("cache mismatch: %+v %+v %v", first, second, err)
	}
	_, historyCalls, _ := be.counts()
	if historyCalls != 1 {
		t.Fatalf("history calls=%d, want 1", historyCalls)
	}
	third, err := s.LoadHistory(ctx, Request{Key: key, Days: 7}, true)
	if err != nil || third.Coverage == first.Coverage {
		t.Fatalf("force refresh=%+v err=%v", third, err)
	}
}

func TestIntervalReadsStoreWithoutRestart(t *testing.T) {
	store := newStore()
	store.setInterval(300)
	s := New(&fakeBackend{}, store, Options{})
	if got := s.Interval(); got != 5*time.Minute {
		t.Fatalf("interval=%v", got)
	}
	store.setInterval(90)
	if got := s.Interval(); got != 90*time.Second {
		t.Fatalf("updated interval=%v", got)
	}
	store.setInterval(30)
	if got := s.Interval(); got != 5*time.Minute {
		t.Fatalf("minimum interval=%v", got)
	}
}

func TestCancellationStopsWorkers(t *testing.T) {
	started := make(chan struct{}, 1)
	finished := make(chan struct{})
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		started <- struct{}{}
		<-ctx.Done()
		close(finished)
		return nil, ctx.Err()
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1})
	ctx, cancel := context.WithCancel(context.Background())
	results := s.Run(ctx)
	s.Enqueue(Request{Key: domain.SourceKey{Provider: "codex"}, Context: "c"})
	<-started
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("backend call did not observe cancellation")
	}
	select {
	case <-results:
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRetryAfterHookSetsBackoff(t *testing.T) {
	retryAt := time.Now().Add(10 * time.Minute)
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		return nil, domain.NewScopedError(domain.ErrProcessFailure, req.SourceKey, "retry later")
	}}
	s := testScheduler(be, newStore(), Options{
		MaxJobs:    1,
		RetryAfter: func(error) *time.Time { return &retryAt },
		Jitter:     func(time.Duration) time.Duration { return 0 },
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results := s.Run(ctx)
	key := domain.SourceKey{Provider: "codex"}
	s.Enqueue(Request{Key: key, Context: "c"})
	_ = waitResult(t, results)
	entry := s.State(key)
	if entry.BackoffUntil == nil || !entry.BackoffUntil.Equal(retryAt) {
		t.Fatalf("backoff=%v want=%v", entry.BackoffUntil, retryAt)
	}
}

func TestInvalidateClearsRetainedStateAndHistory(t *testing.T) {
	be := &fakeBackend{}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results := s.Run(ctx)
	key := domain.SourceKey{Provider: "codex", Account: "one"}
	s.Enqueue(Request{Key: key, Context: "c"})
	_ = waitResult(t, results)
	if _, err := s.LoadHistory(ctx, Request{Key: key, Days: 7}, false); err != nil {
		t.Fatal(err)
	}
	s.Invalidate(key)
	if entry := s.State(key); entry.LastSuccess != nil || entry.LastAttempt != nil {
		t.Fatalf("state survived invalidation: %+v", entry)
	}
	_, err := s.LoadHistory(ctx, Request{Key: key, Days: 7}, false)
	if err != nil {
		t.Fatal(err)
	}
	_, historyCalls, _ := be.counts()
	if historyCalls != 2 {
		t.Fatalf("history cache survived invalidation: calls=%d", historyCalls)
	}
}

func TestHistoryTTLExpiryUsesClock(t *testing.T) {
	now := time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
	be := &fakeBackend{}
	s := testScheduler(be, newStore(), Options{Now: func() time.Time { return now }, HistoryTTL: time.Minute})
	ctx := context.Background()
	key := domain.SourceKey{Provider: "codex"}
	if _, err := s.LoadHistory(ctx, Request{Key: key, Days: 7}, false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Second)
	if _, err := s.LoadHistory(ctx, Request{Key: key, Days: 7}, false); err != nil {
		t.Fatal(err)
	}
	_, historyCalls, _ := be.counts()
	if historyCalls != 1 {
		t.Fatalf("fresh cache calls=%d", historyCalls)
	}
	now = now.Add(2 * time.Minute)
	if _, err := s.LoadHistory(ctx, Request{Key: key, Days: 7}, false); err != nil {
		t.Fatal(err)
	}
	_, historyCalls, _ = be.counts()
	if historyCalls != 2 {
		t.Fatalf("expired cache calls=%d", historyCalls)
	}
}

func TestKnownUnknownAccountDoesNotCoalesceWithSentinel(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		time.Sleep(5 * time.Millisecond)
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey)}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 2})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results := s.Run(ctx)
	s.Enqueue(Request{Key: domain.SourceKey{Provider: "codex", Account: "unknown"}, Context: "c"})
	s.Enqueue(Request{Key: domain.SourceKey{Provider: "codex", Account: domain.UnknownAccount, AccountUnknown: true}, Context: "c"})
	_ = waitResult(t, results)
	_ = waitResult(t, results)
	calls, _, _ := be.counts()
	if calls != 2 {
		t.Fatalf("known and sentinel unknown coalesced: calls=%d", calls)
	}
}

func TestInvalidateDropsQueuedAndInflightResults(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		if call == 1 {
			close(started)
			<-release
		}
		return []domain.UsageSnapshot{{DisplayName: "late"}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := s.Run(ctx)
	key := domain.SourceKey{Provider: "codex", Account: "one"}
	s.Enqueue(Request{Key: key, Context: "c"})
	<-started
	s.Invalidate(key)
	close(release)
	result := waitResult(t, results)
	if !result.Dropped {
		t.Fatalf("late result resurrected state: %+v", result)
	}
	if entry := s.State(key); entry.LastAttempt != nil || entry.LastSuccess != nil {
		t.Fatalf("state resurrected: %+v", entry)
	}
}

func TestBackoffSuppressesAutomaticButNotManualRetry(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		if call == 1 {
			return nil, domain.NewScopedError(domain.ErrTimeout, req.SourceKey, "slow")
		}
		return []domain.UsageSnapshot{{DisplayName: "manual"}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1, Jitter: func(time.Duration) time.Duration { return 0 }})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := s.Run(ctx)
	key := domain.SourceKey{Provider: "codex"}
	s.Enqueue(Request{Key: key, Context: "c"})
	_ = waitResult(t, results)
	if gen := s.Enqueue(Request{Key: key, Context: "c"}); gen != 0 {
		t.Fatalf("automatic retry was queued during backoff: gen=%d", gen)
	}
	time.Sleep(20 * time.Millisecond)
	calls, _, _ := be.counts()
	if calls != 1 {
		t.Fatalf("automatic retry invoked backend: calls=%d", calls)
	}
	s.Enqueue(Request{Key: key, Context: "c", Manual: true})
	result := waitResult(t, results)
	if result.Dropped || len(result.Snapshots) != 1 {
		t.Fatalf("manual retry=%+v", result)
	}
}

func TestAgingPreventsBackgroundStarvation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		if call == 1 {
			close(started)
			<-release
		}
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey)}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1, Aging: 5 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := s.Run(ctx)
	s.Enqueue(Request{Key: domain.SourceKey{Provider: "blocker"}, Priority: 0, Context: "c"})
	<-started
	s.Enqueue(Request{Key: domain.SourceKey{Provider: "background"}, Priority: 10, Context: "c"})
	time.Sleep(60 * time.Millisecond)
	for i := 0; i < 10; i++ {
		s.Enqueue(Request{Key: domain.SourceKey{Provider: fmt.Sprintf("high-%d", i)}, Priority: 0, Context: "c"})
	}
	close(release)
	_ = waitResult(t, results)
	next := waitResult(t, results)
	if next.Key.Provider != "background" {
		t.Fatalf("background starved; next=%s", next.Key.Provider)
	}
}

func TestBulkAccountsForDroppedMembersAndEmptySweep(t *testing.T) {
	be := &fakeBackend{}
	s := testScheduler(be, newStore(), Options{
		MaxJobs: 2,
		Tracked: func(domain.SourceKey) bool { return false },
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := s.Run(ctx)
	empty := s.StartBulk(nil, "c")
	keys := []domain.SourceKey{{Provider: "a"}, {Provider: "b"}, {Provider: "c"}}
	id := s.StartBulk(keys, "c")
	if id == empty {
		t.Fatal("empty sweep blocked the next bulk")
	}
	for i := 0; i < len(keys); i++ {
		result := waitResult(t, results)
		if result.BulkID != id || !result.Dropped {
			t.Fatalf("bulk dropped result=%+v", result)
		}
	}
	next := s.StartBulk([]domain.SourceKey{{Provider: "d"}}, "c")
	if next == id {
		t.Fatal("dropped bulk members left sweep active")
	}
}

func TestBulkAddsNewKeysDuringActiveSweep(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		time.Sleep(20 * time.Millisecond)
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey)}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 2})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := s.Run(ctx)
	id := s.StartBulk([]domain.SourceKey{{Provider: "a"}, {Provider: "b"}}, "c")
	if again := s.StartBulk([]domain.SourceKey{{Provider: "c"}, {Provider: "d"}}, "c"); again != id {
		t.Fatalf("new keys created another sweep: %q vs %q", again, id)
	}
	for i := 0; i < 4; i++ {
		result := waitResult(t, results)
		if result.BulkID != id || result.Total != 4 {
			t.Fatalf("bulk add result=%+v", result)
		}
	}
}

func TestHistorySingleflightAndInvalidation(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	be := &fakeBackend{historyFn: func(ctx context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
		started <- struct{}{}
		<-release
		return domain.HistorySnapshot{Key: mustKey(req.SourceKey), Days: req.Days, Coverage: "loaded"}, nil
	}}
	s := testScheduler(be, newStore(), Options{})
	key := domain.SourceKey{Provider: "codex"}
	type pair struct {
		history domain.HistorySnapshot
		err     error
	}
	results := make(chan pair, 2)
	for i := 0; i < 2; i++ {
		go func() {
			history, err := s.LoadHistory(context.Background(), Request{Key: key, Days: 7, Context: "c"}, false)
			results <- pair{history, err}
		}()
	}
	<-started
	close(release)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.history.Coverage != second.history.Coverage {
		t.Fatalf("singleflight=%+v %+v", first, second)
	}
	_, calls, _ := be.counts()
	if calls != 1 {
		t.Fatalf("duplicate history loads: %d", calls)
	}

	started2 := make(chan struct{})
	release2 := make(chan struct{})
	var once sync.Once
	be.historyFn = func(ctx context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
		once.Do(func() { close(started2) })
		<-release2
		return domain.HistorySnapshot{Key: mustKey(req.SourceKey), Days: req.Days, Coverage: "late"}, nil
	}
	loadDone := make(chan struct{})
	go func() {
		_, _ = s.LoadHistory(context.Background(), Request{Key: key, Days: 7, Context: "c"}, true)
		close(loadDone)
	}()
	<-started2
	s.Invalidate(key)
	close(release2)
	<-loadDone
	_, err := s.LoadHistory(context.Background(), Request{Key: key, Days: 7, Context: "c"}, false)
	if err != nil {
		t.Fatalf("history refetch after invalidation: %v", err)
	}
	_, calls, _ = be.counts()
	if calls != 3 {
		t.Fatalf("invalidated in-flight load did not refetch: calls=%d", calls)
	}
}

func TestBlockedResultConsumerDoesNotDropResults(t *testing.T) {
	be := &fakeBackend{}
	s := testScheduler(be, newStore(), Options{MaxJobs: 3})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results := s.Run(ctx)
	const total = 300
	for i := 0; i < total; i++ {
		s.Enqueue(Request{Key: domain.SourceKey{Provider: fmt.Sprintf("consumer-%d", i)}, Context: "c"})
	}
	for i := 0; i < total; i++ {
		result := waitResult(t, results)
		if result.Dropped || result.Err != nil {
			t.Fatalf("result %d=%+v", i, result)
		}
	}
}

func TestStaggeredStarts(t *testing.T) {
	var mu sync.Mutex
	var starts []time.Time
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey)}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 3, Stagger: 40 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := s.Run(ctx)
	for i := 0; i < 3; i++ {
		s.Enqueue(Request{Key: domain.SourceKey{Provider: fmt.Sprintf("stagger-%d", i)}, Context: "c"})
	}
	for i := 0; i < 3; i++ {
		_ = waitResult(t, results)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(starts) != 3 {
		t.Fatalf("starts=%d", len(starts))
	}
	for i := 1; i < len(starts); i++ {
		if starts[i].Sub(starts[i-1]) < 25*time.Millisecond {
			t.Fatalf("starts not staggered: %v", starts)
		}
	}
}

func TestSupersededGenerationCoalescesAndUsesLatest(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey)}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1})
	key := domain.SourceKey{Provider: "codex", Account: "one"}
	first := s.Enqueue(Request{Key: key, Context: "c"})
	second := s.Enqueue(Request{Key: key, Context: "c"})
	if second <= first {
		t.Fatalf("generation did not advance: %d -> %d", first, second)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results := s.Run(ctx)
	result := waitResult(t, results)
	if result.Generation != second || result.Dropped {
		t.Fatalf("result=%+v", result)
	}
	calls, _, _ := be.counts()
	if calls != 1 {
		t.Fatalf("superseded generation spawned duplicate work: %d", calls)
	}
}

func TestBulkWithAuthAwaitingKeyDoesNotWedge(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		if req.SourceKey == "auth" {
			return nil, domain.NewScopedError(domain.ErrAuth, req.SourceKey, "expired")
		}
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey)}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := s.Run(ctx)
	authKey := domain.SourceKey{Provider: "auth"}
	s.Enqueue(Request{Key: authKey, Context: "bulk", Manual: true})
	_ = waitResult(t, results)
	id := s.StartBulk([]domain.SourceKey{authKey, {Provider: "ok"}}, "bulk")
	for i := 0; i < 1; i++ {
		result := waitResult(t, results)
		if result.BulkID != id || result.Total != 1 || result.Completed != 1 {
			t.Fatalf("bulk suppressed membership result=%+v", result)
		}
	}
	next := s.StartBulk([]domain.SourceKey{{Provider: "next"}}, "bulk")
	if next == id {
		t.Fatal("suppressed bulk member left sweep active")
	}
}

func TestInvalidateQueuedBulkMemberAccountsCompletion(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		if req.SourceKey == "a" {
			close(started)
			<-release
		}
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey)}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := s.Run(ctx)
	id := s.StartBulk([]domain.SourceKey{{Provider: "a"}, {Provider: "b"}}, "bulk")
	<-started
	s.Invalidate(domain.SourceKey{Provider: "b"})
	close(release)
	_ = waitResult(t, results)
	next := s.StartBulk([]domain.SourceKey{{Provider: "c"}}, "bulk")
	if next == id {
		t.Fatal("invalidated bulk member left sweep active")
	}
}

func TestSchedulerWaitReturnsAfterCancellation(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 2})
	ctx, cancel := context.WithCancel(context.Background())
	s.Run(ctx)
	s.Enqueue(Request{Key: domain.SourceKey{Provider: "codex"}, Context: "c"})
	time.Sleep(10 * time.Millisecond)
	cancel()
	done := make(chan struct{})
	go func() { s.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler workers did not stop")
	}
}

func TestSetPriorityReordersQueuedWork(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey)}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1})
	bg := domain.SourceKey{Provider: "background"}
	selected := domain.SourceKey{Provider: "provider-010"}
	s.Enqueue(Request{Key: bg, Context: "c", Priority: 1})
	s.Enqueue(Request{Key: selected, Context: "c", Priority: 1})
	s.SetPriority(selected, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results := s.Run(ctx)
	first := waitResult(t, results)
	if first.Key.Provider != "provider-010" {
		t.Fatalf("selected not prioritized: %s", first.Key)
	}
}

func TestBulkJoinsExecutingTaskAndCompletes(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		if req.SourceKey == "a" && call == 1 {
			close(started)
			<-release
		}
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey)}}, nil
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	results := s.Run(ctx)
	s.Enqueue(Request{Key: domain.SourceKey{Provider: "a"}, Context: "bulk", Manual: true})
	<-started
	id := s.StartBulkWithManual([]domain.SourceKey{{Provider: "a"}, {Provider: "b"}}, "bulk", true)
	close(release)
	first := waitResult(t, results)
	second := waitResult(t, results)
	if first.BulkID != id || second.BulkID != id || second.Total != 2 {
		t.Fatalf("joined bulk results=%+v %+v", first, second)
	}
	next := s.StartBulkWithManual([]domain.SourceKey{{Provider: "a"}, {Provider: "b"}}, "bulk", true)
	if next == id {
		t.Fatal("joined sweep stayed active")
	}
}

func TestSchedulerRetainsPartialSnapshotsWithError(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		percent := 20.0
		return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey), Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}}},
			domain.NewScopedError(domain.ErrProcessFailure, req.SourceKey, "nonzero")
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1, Jitter: func(time.Duration) time.Duration { return 0 }})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results := s.Run(ctx)
	key := domain.SourceKey{Provider: "codex"}
	s.Enqueue(Request{Key: key, Context: "c"})
	result := waitResult(t, results)
	if result.Err == nil || len(result.Snapshots) != 1 {
		t.Fatalf("result=%+v", result)
	}
	entry := s.State(key)
	if entry.LastSuccess == nil || len(entry.Snapshots) != 1 || len(entry.Snapshots[0].Windows) != 1 || entry.Err == nil {
		t.Fatalf("partial state=%+v", entry)
	}
}

func TestSchedulerErrorOnlySnapshotsRetainPriorSuccess(t *testing.T) {
	be := &fakeBackend{usageFn: func(ctx context.Context, req backend.UsageRequest, call int) ([]domain.UsageSnapshot, error) {
		if call == 1 {
			percent := 20.0
			return []domain.UsageSnapshot{{Key: mustKey(req.SourceKey), Windows: []domain.AllowanceWindow{{Label: "Plan", UsedPercent: &percent}}}}, nil
		}
		key := mustKey(req.SourceKey)
		key.Account = domain.UnknownAccount
		key.AccountUnknown = true
		return []domain.UsageSnapshot{{Key: key, Error: domain.NewScopedError(domain.ErrAuth, key.String(), "expired")}}, domain.NewScopedError(domain.ErrAuth, key.String(), "expired")
	}}
	s := testScheduler(be, newStore(), Options{MaxJobs: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	results := s.Run(ctx)
	key := domain.SourceKey{Provider: "codex"}
	s.Enqueue(Request{Key: key, Context: "c"})
	_ = waitResult(t, results)
	before := s.State(key)
	s.Enqueue(Request{Key: key, Context: "c", Manual: true})
	_ = waitResult(t, results)
	after := s.State(key)
	if before.LastSuccess == nil || after.LastSuccess == nil || !after.LastSuccess.Equal(*before.LastSuccess) || len(after.Snapshots) != 1 || len(after.Snapshots[0].Windows) != 1 || after.Err == nil {
		t.Fatalf("prior success lost: before=%+v after=%+v", before, after)
	}
}
