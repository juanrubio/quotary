// Package refresh schedules, coalesces, and cancels backend work.
package refresh

import (
	"context"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/domain"
)

// Operation identifies scheduler work.
type Operation string

const (
	OpUsage   Operation = "usage"
	OpHistory Operation = "history"
)

// Backend is the subset of backend.Backend required by the scheduler.
type Backend interface {
	Usage(context.Context, backend.UsageRequest) ([]domain.UsageSnapshot, error)
	History(context.Context, backend.HistoryRequest) (domain.HistorySnapshot, error)
}

// Store supplies live preference changes without restarting the scheduler.
type Store interface {
	Load() config.Preferences
}

// Request asks for one coalesced operation.
type Request struct {
	Key       domain.SourceKey
	Operation Operation
	Days      int
	Priority  int
	Context   string
	Manual    bool
	Reason    string
}

// Result is delivered after a scheduled operation. Results are only emitted
// from the worker loop and never update scheduler state when dropped.
type Result struct {
	Key        domain.SourceKey
	Operation  Operation
	Days       int
	Generation uint64
	Context    string
	Snapshots  []domain.UsageSnapshot
	History    domain.HistorySnapshot
	Err        error
	Dropped    bool
	AttemptAt  time.Time
	BulkID     string
	Completed  int
	Total      int
}

// Entry is retained per canonical source key.
type Entry struct {
	LastAttempt           *time.Time
	LastSuccess           *time.Time
	Snapshots             []domain.UsageSnapshot
	Err                   error
	ConsecutiveFailures   int
	BackoffUntil          *time.Time
	AwaitingExplicitRetry bool
}

type task struct {
	mu           sync.Mutex
	request      Request
	generation   uint64
	seq          uint64
	enqueuedAt   time.Time
	bulkID       string
	invalidation uint64
}

type taskSnapshot struct {
	request      Request
	generation   uint64
	bulkID       string
	invalidation uint64
}

func (t *task) snapshot() taskSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return taskSnapshot{request: t.request, generation: t.generation, bulkID: t.bulkID, invalidation: t.invalidation}
}

type historyEntry struct {
	history  domain.HistorySnapshot
	loadedAt time.Time
}

type historyLoad struct {
	done         chan struct{}
	history      domain.HistorySnapshot
	err          error
	invalidation uint64
}

type bulkProgress struct {
	id     string
	total  int
	done   int
	active bool
}

// Options tunes the scheduler. Zero values select documented defaults.
type Options struct {
	MaxJobs    int
	Stagger    time.Duration
	Now        func() time.Time
	Jitter     func(time.Duration) time.Duration
	Tracked    func(domain.SourceKey) bool
	RetryAfter func(error) *time.Time
	HistoryTTL time.Duration
	Aging      time.Duration
}

// Scheduler coalesces work by canonical source key and operation.
type Scheduler struct {
	backend Backend
	store   Store
	opts    Options

	startOnce sync.Once
	workers   sync.WaitGroup
	wake      chan struct{}
	results   chan Result
	sem       chan struct{}

	mu            sync.Mutex
	queue         []*task
	inflight      map[string]*task
	latest        map[string]uint64
	latestContext map[string]string
	state         map[string]*Entry
	history       map[string]historyEntry
	historyLoads  map[string]*historyLoad
	invalidation  map[string]uint64
	bulkKeys      map[string]bool
	bulk          *bulkProgress
	sequence      uint64
	lastStart     time.Time
}

// New constructs a scheduler. All defaults are safe for production.
func New(be Backend, store Store, opts Options) *Scheduler {
	if opts.MaxJobs <= 0 {
		opts.MaxJobs = 3
	}
	if opts.Stagger < 0 {
		opts.Stagger = 0
	}
	if opts.Stagger == 0 {
		opts.Stagger = 40 * time.Millisecond
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if opts.Jitter == nil {
		opts.Jitter = func(d time.Duration) time.Duration {
			if d <= 0 {
				return 0
			}
			return time.Duration(rand.Int63n(int64(d/4) + 1))
		}
	}
	if opts.HistoryTTL <= 0 {
		opts.HistoryTTL = 15 * time.Minute
	}
	if opts.Aging <= 0 {
		opts.Aging = 30 * time.Second
	}
	return &Scheduler{
		backend:       be,
		store:         store,
		opts:          opts,
		wake:          make(chan struct{}, 1),
		results:       make(chan Result, 256),
		sem:           make(chan struct{}, opts.MaxJobs),
		inflight:      map[string]*task{},
		latest:        map[string]uint64{},
		latestContext: map[string]string{},
		state:         map[string]*Entry{},
		history:       map[string]historyEntry{},
		historyLoads:  map[string]*historyLoad{},
		invalidation:  map[string]uint64{},
		bulkKeys:      map[string]bool{},
	}
}

// Run starts the worker pool once and returns the result stream.
func (s *Scheduler) Run(ctx context.Context) <-chan Result {
	s.startOnce.Do(func() {
		for i := 0; i < s.opts.MaxJobs; i++ {
			s.workers.Add(1)
			go func() {
				defer s.workers.Done()
				s.worker(ctx)
			}()
		}
	})
	return s.results
}

// Wait waits for scheduler workers to stop after cancellation.
func (s *Scheduler) Wait() { s.workers.Wait() }

// Interval reports the current configured refresh interval, enforcing 60 s.
func (s *Scheduler) Interval() time.Duration {
	prefs := s.store.Load()
	seconds := prefs.IntervalSeconds
	if seconds < 60 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

// Enqueue coalesces work with the same operation, context, and canonical key.
// Unknown-account work coalesces only with other unknown work for the same
// provider and variant.
func (s *Scheduler) Enqueue(req Request) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enqueueLocked(req, "")
}

func (s *Scheduler) enqueueLocked(req Request, bulkID string) uint64 {
	if req.Operation == "" {
		req.Operation = OpUsage
	}
	dedupe := dedupeKey(req)
	canonical := req.Key.String()
	if previous, ok := s.latestContext[canonical]; !ok || previous != req.Context {
		delete(s.state, canonical)
		for cacheKey := range s.history {
			if strings.HasPrefix(cacheKey, canonical+"|") {
				delete(s.history, cacheKey)
			}
		}
		s.latestContext[canonical] = req.Context
	}
	if entry := s.state[canonical]; entry != nil && !req.Manual {
		if entry.AwaitingExplicitRetry {
			return 0
		}
		if entry.BackoffUntil != nil && entry.BackoffUntil.After(s.opts.Now()) {
			return 0
		}
	}
	s.latest[dedupe]++
	generation := s.latest[dedupe]
	now := s.opts.Now()
	for _, existing := range s.queue {
		if existing.request.Operation == req.Operation && dedupeKey(existing.request) == dedupe {
			if req.Priority < existing.request.Priority {
				existing.request.Priority = req.Priority
			}
			existing.generation = generation
			if bulkID != "" {
				existing.bulkID = bulkID
			}
			return generation
		}
	}
	if existing := s.inflight[dedupe]; existing != nil {
		existing.mu.Lock()
		if req.Priority < existing.request.Priority {
			existing.request.Priority = req.Priority
		}
		existing.generation = generation
		if bulkID != "" {
			existing.bulkID = bulkID
		}
		existing.mu.Unlock()
		return generation
	}
	s.sequence++
	s.queue = append(s.queue, &task{
		request: req, generation: generation, seq: s.sequence,
		enqueuedAt: now, bulkID: bulkID, invalidation: s.invalidation[canonical],
	})
	s.signal()
	return generation
}

func (s *Scheduler) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func dedupeKey(req Request) string {
	key := req.Key
	coalesced := key.String()
	if key.AccountUnknown {
		coalesced = key.Provider + ":" + string(key.Variant) + "@unknown-group"
	}
	return string(req.Operation) + "|" + req.Context + "|" + coalesced + "|" + itoa(req.Days)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [12]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

func (s *Scheduler) worker(ctx context.Context) {
	for {
		task, ok := s.nextTask(ctx)
		if !ok {
			return
		}
		if !s.acquire(ctx) {
			return
		}
		snapshot := task.snapshot()
		result := s.execute(ctx, snapshot)
		s.release()
		s.finish(ctx, task, snapshot, result)
	}
}

func (s *Scheduler) acquire(ctx context.Context) bool {
	select {
	case s.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *Scheduler) release() { <-s.sem }

func (s *Scheduler) nextTask(ctx context.Context) (*task, bool) {
	for {
		s.mu.Lock()
		if len(s.queue) > 0 {
			now := s.opts.Now()
			effective := func(task *task) int {
				age := now.Sub(task.enqueuedAt)
				boost := int(age / s.opts.Aging)
				return task.request.Priority - boost
			}
			sort.SliceStable(s.queue, func(i, j int) bool {
				pi, pj := effective(s.queue[i]), effective(s.queue[j])
				if pi != pj {
					return pi < pj
				}
				return s.queue[i].seq < s.queue[j].seq
			})
			task := s.queue[0]
			s.queue = s.queue[1:]
			dedupe := dedupeKey(task.request)
			s.inflight[dedupe] = task
			delay := s.staggerDelayLocked()
			s.mu.Unlock()
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return nil, false
				}
			}
			return task, true
		}
		s.mu.Unlock()
		select {
		case <-s.wake:
		case <-ctx.Done():
			return nil, false
		}
	}
}

func (s *Scheduler) staggerDelayLocked() time.Duration {
	if s.opts.Stagger <= 0 {
		return 0
	}
	now := s.opts.Now()
	if s.lastStart.IsZero() || now.Sub(s.lastStart) >= s.opts.Stagger {
		s.lastStart = now
		return 0
	}
	delay := s.opts.Stagger - now.Sub(s.lastStart)
	s.lastStart = now.Add(delay)
	return delay
}

func (s *Scheduler) execute(ctx context.Context, snapshot taskSnapshot) Result {
	result := Result{
		Key: snapshot.request.Key, Operation: snapshot.request.Operation, Days: snapshot.request.Days,
		Generation: snapshot.generation, Context: snapshot.request.Context,
		AttemptAt: s.opts.Now(),
	}
	if snapshot.request.Operation == OpHistory {
		result.History, result.Err = s.backend.History(ctx, backend.HistoryRequest{
			ProviderID: snapshot.request.Key.Provider, Variant: snapshot.request.Key.Variant,
			SourceKey: snapshot.request.Key.String(), Days: snapshot.request.Days,
		})
		return result
	}
	result.Snapshots, result.Err = s.backend.Usage(ctx, backend.UsageRequest{
		ProviderID: snapshot.request.Key.Provider, Variant: snapshot.request.Key.Variant,
		SourceKey: snapshot.request.Key.String(),
	})
	return result
}

func (s *Scheduler) finish(ctx context.Context, task *task, snapshot taskSnapshot, result Result) {
	tracked := true
	if s.opts.Tracked != nil {
		tracked = s.opts.Tracked(snapshot.request.Key)
	}
	s.mu.Lock()
	current := task.snapshot()
	dedupe := dedupeKey(snapshot.request)
	delete(s.inflight, dedupe)
	canonical := snapshot.request.Key.String()
	latestGeneration := s.latest[dedupe]
	latestContext := s.latestContext[canonical]
	dropped := result.Context != latestContext || s.invalidation[canonical] != task.invalidation || !tracked
	if !dropped {
		result.Generation = latestGeneration
	}
	attempt := result.AttemptAt
	if !dropped {
		entry := s.state[canonical]
		if entry == nil {
			entry = &Entry{}
			s.state[canonical] = entry
		}
		entry.LastAttempt = &attempt
		if result.Err != nil {
			entry.Err = result.Err
			if snapshotsHaveData(result.Snapshots) {
				entry.Snapshots = cloneSnapshots(result.Snapshots)
				success := attempt
				entry.LastSuccess = &success
			}
			entry.ConsecutiveFailures++
			wait, retryAt, explicit := s.backoff(result.Err, entry.ConsecutiveFailures)
			if explicit {
				entry.AwaitingExplicitRetry = true
				entry.BackoffUntil = nil
			} else if retryAt != nil {
				entry.BackoffUntil = retryAt
			} else if wait > 0 {
				until := attempt.Add(wait)
				entry.BackoffUntil = &until
			}
		} else if result.Operation == OpUsage {
			entry.Err = nil
			entry.ConsecutiveFailures = 0
			entry.BackoffUntil = nil
			entry.AwaitingExplicitRetry = false
			entry.Snapshots = cloneSnapshots(result.Snapshots)
			success := attempt
			entry.LastSuccess = &success
		}
	}
	if !dropped && result.Operation == OpHistory && result.Err == nil {
		s.history[historyCacheKey(snapshot.request)] = historyEntry{history: result.History, loadedAt: attempt}
	}
	if current.bulkID != "" && s.bulk != nil && s.bulk.id == current.bulkID {
		s.bulk.done++
		result.Completed = s.bulk.done
		result.Total = s.bulk.total
		result.BulkID = current.bulkID
		if s.bulk.done >= s.bulk.total {
			s.bulk.active = false
		}
	}
	if dropped {
		result.Dropped = true
		result.Snapshots = nil
		result.History = domain.HistorySnapshot{}
	}
	s.mu.Unlock()
	select {
	case s.results <- result:
	case <-ctx.Done():
	}
}

func snapshotsHaveData(snapshots []domain.UsageSnapshot) bool {
	for _, snapshot := range snapshots {
		if len(snapshot.Windows) > 0 || len(snapshot.Balances) > 0 || len(snapshot.Details) > 0 || snapshot.Plan != "" {
			return true
		}
	}
	return false
}

func cloneSnapshots(snapshots []domain.UsageSnapshot) []domain.UsageSnapshot {
	result := make([]domain.UsageSnapshot, len(snapshots))
	copy(result, snapshots)
	return result
}

func (s *Scheduler) backoff(err error, failures int) (time.Duration, *time.Time, bool) {
	scopedErr, ok := domain.AsScopedError(err)
	if !ok {
		base := time.Minute
		if failures > 1 {
			base = time.Minute << min(failures-1, 5)
		}
		if base > 30*time.Minute {
			base = 30 * time.Minute
		}
		wait := base + s.opts.Jitter(base/4)
		if wait > 30*time.Minute {
			wait = 30 * time.Minute
		}
		return wait, nil, false
	}
	switch scopedErr.Category {
	case domain.ErrAuth, domain.ErrConfig, domain.ErrUnsupported:
		return 0, nil, true
	case domain.ErrUnavailable:
		return 0, nil, true
	}
	if s.opts.RetryAfter != nil {
		if retry := s.opts.RetryAfter(err); retry != nil {
			return 0, retry, false
		}
	}
	base := time.Minute
	if failures > 1 {
		base = time.Minute << min(failures-1, 5)
	}
	if base > 30*time.Minute {
		base = 30 * time.Minute
	}
	wait := base + s.opts.Jitter(base/4)
	if wait > 30*time.Minute {
		wait = 30 * time.Minute
	}
	return wait, nil, false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// State returns a copy of retained state for a canonical source key.
func (s *Scheduler) State(key domain.SourceKey) Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.state[key.String()]
	if entry == nil {
		return Entry{}
	}
	copyEntry := *entry
	copyEntry.Snapshots = cloneSnapshots(entry.Snapshots)
	return copyEntry
}

// SetPriority updates queued work priority for a canonical source key.
func (s *Scheduler) SetPriority(key domain.SourceKey, priority int) {
	canonical := key.String()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, task := range s.queue {
		if task.request.Key.String() == canonical {
			task.request.Priority = priority
		}
	}
	if task := s.inflight[dedupeKey(Request{Key: key, Operation: OpUsage, Context: s.latestContext[canonical]})]; task != nil {
		task.mu.Lock()
		task.request.Priority = priority
		task.mu.Unlock()
	}
}

// BulkProgress reports the active scheduler sweep.
func (s *Scheduler) BulkProgress() (done, total int, active bool, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bulk == nil {
		return 0, 0, false, ""
	}
	return s.bulk.done, s.bulk.total, s.bulk.active, s.bulk.id
}

// StartBulk starts one bulk sweep. If a sweep is active its id is returned.
func (s *Scheduler) StartBulk(keys []domain.SourceKey, contextID string) string {
	return s.StartBulkWithManual(keys, contextID, false)
}

// StartBulkWithManual starts a user-initiated sweep when manual is true.
func (s *Scheduler) StartBulkWithManual(keys []domain.SourceKey, contextID string, manual bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startBulkLocked("", keys, contextID, manual)
}

// StartBulkWithID starts a sweep using a caller-provided id, allowing live
// accounting to be installed before any worker can complete.
func (s *Scheduler) StartBulkWithID(id string, keys []domain.SourceKey, contextID string, manual bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startBulkLocked(id, keys, contextID, manual)
}

func (s *Scheduler) startBulkLocked(providedID string, keys []domain.SourceKey, contextID string, manual bool) string {
	if s.bulk != nil && s.bulk.active {
		for _, key := range keys {
			canonical := key.String()
			if !s.bulkKeys[canonical] {
				if s.enqueueLocked(Request{Key: key, Operation: OpUsage, Priority: 1, Context: contextID, Manual: manual}, s.bulk.id) == 0 {
					continue
				}
				s.bulkKeys[canonical] = true
				s.bulk.total++
			}
		}
		return s.bulk.id
	}
	s.sequence++
	id := providedID
	if id == "" {
		id = "bulk-" + itoa(int(s.sequence))
	}
	s.bulk = &bulkProgress{id: id, active: true}
	s.bulkKeys = map[string]bool{}
	for _, key := range keys {
		canonical := key.String()
		if s.bulkKeys[canonical] {
			continue
		}
		if s.enqueueLocked(Request{Key: key, Operation: OpUsage, Priority: 1, Context: contextID, Manual: manual}, id) == 0 {
			continue
		}
		s.bulkKeys[canonical] = true
		s.bulk.total++
	}
	if s.bulk.total == 0 {
		s.bulk.active = false
	}
	return id
}

// CachedHistory returns fresh cached history without starting work.
func (s *Scheduler) CachedHistory(req Request) (domain.HistorySnapshot, bool) {
	if req.Days == 0 {
		req.Days = 7
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cached, ok := s.history[historyCacheKey(req)]
	if !ok || s.opts.Now().Sub(cached.loadedAt) >= s.opts.HistoryTTL {
		return domain.HistorySnapshot{}, false
	}
	return cached.history, true
}

// CachedHistoryAny returns cached history regardless of TTL.
func (s *Scheduler) CachedHistoryAny(req Request) (domain.HistorySnapshot, bool) {
	if req.Days == 0 {
		req.Days = 7
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cached, ok := s.history[historyCacheKey(req)]
	return cached.history, ok
}

// LoadHistory returns cached history when fresh, otherwise fetches it. The
// scheduler's shared concurrency ceiling includes history jobs.
func (s *Scheduler) LoadHistory(ctx context.Context, req Request, force bool) (domain.HistorySnapshot, error) {
	if req.Days == 0 {
		req.Days = 7
	}
	cacheKey := historyCacheKey(req)
	canonical := req.Key.String()
	now := s.opts.Now()
	s.mu.Lock()
	cached, ok := s.history[cacheKey]
	if !force && ok && now.Sub(cached.loadedAt) < s.opts.HistoryTTL {
		s.mu.Unlock()
		return cached.history, nil
	}
	if inFlight := s.historyLoads[cacheKey]; inFlight != nil {
		s.mu.Unlock()
		select {
		case <-inFlight.done:
			return inFlight.history, inFlight.err
		case <-ctx.Done():
			return domain.HistorySnapshot{}, ctx.Err()
		}
	}
	load := &historyLoad{done: make(chan struct{}), invalidation: s.invalidation[canonical]}
	s.historyLoads[cacheKey] = load
	s.mu.Unlock()

	if !s.acquire(ctx) {
		s.mu.Lock()
		load.err = ctx.Err()
		delete(s.historyLoads, cacheKey)
		close(load.done)
		s.mu.Unlock()
		return domain.HistorySnapshot{}, load.err
	}
	history, err := s.backend.History(ctx, backend.HistoryRequest{
		ProviderID: req.Key.Provider, Variant: req.Key.Variant,
		SourceKey: req.Key.String(), Days: req.Days,
	})
	s.release()

	s.mu.Lock()
	if s.invalidation[canonical] == load.invalidation {
		if err == nil || hasHistorySummary(history) {
			s.history[cacheKey] = historyEntry{history: history, loadedAt: s.opts.Now()}
		}
	}
	load.history = history
	load.err = err
	delete(s.historyLoads, cacheKey)
	close(load.done)
	s.mu.Unlock()
	return history, err
}

func hasHistorySummary(history domain.HistorySnapshot) bool {
	return len(history.Buckets) > 0 || history.TotalTokens != nil || history.Cost != nil || len(history.Models) > 0
}

func historyCacheKey(req Request) string {
	return req.Key.String() + "|" + req.Context + "|" + itoa(req.Days)
}

// Invalidate clears retained state and history for a source identity/mode change.
func (s *Scheduler) Invalidate(key domain.SourceKey) {
	canonical := key.String()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invalidation[canonical]++
	delete(s.state, canonical)
	for cacheKey := range s.history {
		if strings.HasPrefix(cacheKey, canonical+"|") {
			delete(s.history, cacheKey)
		}
	}
	for index := len(s.queue) - 1; index >= 0; index-- {
		if s.queue[index].request.Key.String() == canonical {
			removed := s.queue[index]
			s.queue = append(s.queue[:index], s.queue[index+1:]...)
			if removed.bulkID != "" && s.bulk != nil && s.bulk.id == removed.bulkID {
				s.bulk.done++
				if s.bulk.done >= s.bulk.total {
					s.bulk.active = false
				}
			}
		}
	}
	delete(s.latestContext, canonical)
}
