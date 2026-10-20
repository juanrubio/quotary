// Package live wires the CodexBar backend and refresh scheduler into the TUI
// Session seam. It never imports adapter DTOs into the TUI.
package live

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/catalog"
	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/domain"
	"github.com/juanrubio/quotary/internal/refresh"
)

// Options supplies the live backend and preference store.
type Options struct {
	Backend      backend.Backend
	Store        config.Store
	Interval     func() time.Duration
	StoreWarning string
}

type historyFailure struct {
	err error
	at  time.Time
}

// Session implements tui.Session using the refresh scheduler.
type Session struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	backend   backend.Backend
	store     config.Store
	scheduler *refresh.Scheduler
	interval  func() time.Duration

	mu             sync.RWMutex
	prefs          config.Preferences
	catalog        backend.CatalogSnapshot
	snapshots      map[string]domain.UsageSnapshot
	done           map[string]bool
	events         chan struct{}
	eventsClosed   bool
	bulkID         string
	bulkKeys       map[string]bool
	bulkDone       int
	bulkTotal      int
	bulkSeq        int
	priorities     map[string]int
	historyErrors  map[string]historyFailure
	historyLoading map[string]bool
	storeWarning   string
	backendErr     string
	backendVersion string
}

// New builds a live session and starts background refresh work.
func New(parent context.Context, opts Options) (*Session, error) {
	if opts.Backend == nil || opts.Store == nil {
		return nil, fmt.Errorf("live session requires backend and store")
	}
	ctx, cancel := context.WithCancel(parent)
	s := &Session{
		ctx:            ctx,
		cancel:         cancel,
		backend:        opts.Backend,
		store:          opts.Store,
		prefs:          config.Normalize(opts.Store.Load()),
		snapshots:      map[string]domain.UsageSnapshot{},
		done:           map[string]bool{},
		events:         make(chan struct{}, 1),
		historyErrors:  map[string]historyFailure{},
		historyLoading: map[string]bool{},
		bulkKeys:       map[string]bool{},
		priorities:     map[string]int{},
		storeWarning:   opts.StoreWarning,
	}
	s.scheduler = refresh.New(opts.Backend, opts.Store, refresh.Options{
		MaxJobs: 3,
		Tracked: func(key domain.SourceKey) bool {
			s.mu.RLock()
			defer s.mu.RUnlock()
			return s.trackedLocked(key)
		},
	})
	s.interval = opts.Interval
	results := s.scheduler.Run(ctx)
	s.wg.Add(1)
	go s.consume(results)

	rawCatalog, err := opts.Backend.Catalog(ctx)
	if err != nil {
		s.backendErr = safeError(err)
	} else {
		s.catalog = catalog.Apply(rawCatalog)
		migrated := migrateGeminiTracked(s.prefs.Tracked)
		migratedPins := migrateGeminiPins(s.prefs.Pins)
		changed := !sameStrings(migrated, s.prefs.Tracked) || !sameStrings(migratedPins, s.prefs.Pins)
		s.prefs.Tracked = migrated
		s.prefs.Pins = migratedPins
		firstRun := false
		if provider, ok := opts.Store.(interface{ FirstRun() bool }); ok {
			firstRun = provider.FirstRun()
		}
		if len(s.prefs.Tracked) == 0 && firstRun {
			seeded := catalog.SeedTracked(s.catalog)
			if len(seeded) > 0 {
				s.prefs.Tracked = seeded
				changed = true
			}
		}
		if changed {
			if saveErr := opts.Store.Save(s.prefs); saveErr != nil {
				s.storeWarning = safeError(saveErr)
			}
		}
	}
	if version, versionErr := opts.Backend.Version(ctx); versionErr == nil {
		s.backendVersion = version.Version
	} else if s.backendErr == "" {
		s.backendErr = safeError(versionErr)
	}
	for _, key := range s.prefs.Tracked {
		parsed, parseErr := domain.ParseSourceKey(key)
		if parseErr != nil || !s.tracked(parsed) {
			continue
		}
		s.ensureSkeleton(parsed)
		if s.autoPollAllowed(parsed) {
			s.enqueueAutomatic(parsed, 0)
		} else {
			s.markAutoDisabled(parsed)
		}
	}
	s.wg.Add(1)
	go s.intervalLoop()
	return s, nil
}

// Close cancels owned work and waits for the result consumer.
func (s *Session) Close() error {
	s.cancel()
	s.mu.Lock()
	if !s.eventsClosed {
		s.eventsClosed = true
		close(s.events)
	}
	s.mu.Unlock()
	s.wg.Wait()
	s.scheduler.Wait()
	return nil
}

func (s *Session) consume(results <-chan refresh.Result) {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case result, ok := <-results:
			if !ok {
				return
			}
			s.apply(result)
		}
	}
}

func (s *Session) apply(result refresh.Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if result.Operation == refresh.OpHistory {
		s.signalEventLocked()
		return
	}
	defer s.signalEventLocked()
	key := result.Key.String()
	s.done[key] = true
	if result.BulkID != "" && result.BulkID == s.bulkID && result.Total > 0 {
		s.bulkDone = result.Completed
		s.bulkTotal = result.Total
	}
	if result.Dropped || !s.trackedLocked(result.Key) {
		return
	}
	if len(result.Snapshots) > 0 {
		s.applyReturnedLocked(result)
		return
	}
	if result.Err != nil {
		targetKey := s.snapshotKeyLocked(result.Key)
		snapshot, ok := s.snapshots[targetKey]
		if !ok {
			targetKey = key
			snapshot = domain.UsageSnapshot{Key: result.Key, ProviderID: result.Key.Provider, DisplayName: result.Key.Provider}
		}
		snapshot.Error = asScoped(result.Err)
		if snapshot.Error != nil {
			snapshot.State = domain.DeriveState(snapshot, false)
		} else {
			snapshot.State = domain.StatusError
		}
		if snapshot.ReceivedAt.IsZero() {
			snapshot.ReceivedAt = result.AttemptAt
		}
		s.snapshots[targetKey] = snapshot
		return
	}
	delete(s.snapshots, key)
}

func (s *Session) applyReturnedLocked(result refresh.Result) {
	returned := map[string]bool{}
	for _, snapshot := range result.Snapshots {
		returned[snapshot.Key.String()] = true
	}
	for existingKey, existing := range s.snapshots {
		if trackMatches(result.Key, existing.Key) && !returned[existingKey] {
			delete(s.snapshots, existingKey)
		}
	}
	for _, snapshot := range result.Snapshots {
		if existing, ok := s.snapshots[snapshot.Key.String()]; ok && snapshot.Error != nil &&
			len(snapshot.Windows) == 0 && len(snapshot.Balances) == 0 && len(snapshot.Details) == 0 {
			snapshot.Windows = existing.Windows
			snapshot.Balances = existing.Balances
			snapshot.Details = existing.Details
			if snapshot.Plan == "" {
				snapshot.Plan = existing.Plan
			}
			if snapshot.SampleAt == nil {
				snapshot.SampleAt = existing.SampleAt
			}
			if snapshot.ReceivedAt.IsZero() {
				snapshot.ReceivedAt = existing.ReceivedAt
			}
		}
		if result.Err != nil && snapshot.Error == nil {
			snapshot.Error = asScoped(result.Err)
		}
		if snapshot.ReceivedAt.IsZero() {
			snapshot.ReceivedAt = result.AttemptAt
		}
		snapshot.State = domain.EffectiveState(snapshot)
		s.snapshots[snapshot.Key.String()] = snapshot
	}
}

func (s *Session) refreshTargetsLocked(tracked domain.SourceKey) []domain.SourceKey {
	var targets []domain.SourceKey
	for _, snapshot := range s.snapshots {
		if trackMatches(tracked, snapshot.Key) {
			targets = append(targets, snapshot.Key)
		}
	}
	if len(targets) == 0 {
		targets = append(targets, tracked)
	}
	return targets
}

func (s *Session) snapshotKeyLocked(key domain.SourceKey) string {
	if _, ok := s.snapshots[key.String()]; ok {
		return key.String()
	}
	for snapshotKey, snapshot := range s.snapshots {
		if trackMatches(key, snapshot.Key) {
			return snapshotKey
		}
	}
	return key.String()
}

func (s *Session) descriptorFor(key domain.SourceKey) (domain.SourceDescriptor, bool) {
	for _, descriptor := range s.catalog.Providers {
		if descriptor.ProviderID != key.Provider {
			continue
		}
		if descriptor.Key.Variant == key.Variant {
			return descriptor, true
		}
		if key.Variant == "" && descriptor.Variant == domain.VariantConfigured {
			return descriptor, true
		}
	}
	return domain.SourceDescriptor{}, false
}

func (s *Session) autoPollAllowed(key domain.SourceKey) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	descriptor, ok := s.descriptorFor(key)
	if !ok || !descriptor.Available || descriptor.Capabilities.ActiveProbe != domain.CapabilityUnsupported {
		return false
	}
	switch descriptor.Kind {
	case domain.KindSubscription, domain.KindMixed, domain.KindCredits, domain.KindMeteredAPI:
		return true
	default:
		return false
	}
}

func (s *Session) activeProbe(key domain.SourceKey) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	descriptor, ok := s.descriptorFor(key)
	return ok && descriptor.Capabilities.ActiveProbe == domain.CapabilitySupported
}

func (s *Session) markAutoDisabled(key domain.SourceKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := s.snapshots[key.String()]
	snapshot.Key = key
	snapshot.ProviderID = key.Provider
	if snapshot.DisplayName == "" {
		snapshot.DisplayName = s.displayNameLocked(key)
	}
	snapshot.State = domain.StatusUnavailable
	snapshot.Error = domain.NewScopedError(domain.ErrUnavailable, key.String(), "Automatic polling is disabled for this source")
	s.snapshots[key.String()] = snapshot
	s.signalEventLocked()
}

func migrateGeminiTracked(tracked []string) []string {
	hasGeminiVariant := false
	generic := false
	for _, value := range tracked {
		parsed, err := domain.ParseSourceKey(value)
		if err != nil || parsed.Provider != "gemini" {
			continue
		}
		if parsed.Variant == "" {
			generic = true
		} else {
			hasGeminiVariant = true
		}
	}
	if !generic {
		return tracked
	}
	result := make([]string, 0, len(tracked)+1)
	for _, value := range tracked {
		parsed, err := domain.ParseSourceKey(value)
		if err == nil && parsed.Provider == "gemini" && parsed.Variant == "" {
			continue
		}
		result = append(result, value)
	}
	if !hasGeminiVariant {
		result = append(result, (domain.SourceKey{Provider: "gemini", Variant: domain.VariantConfigured}).String())
	}
	return result
}

func migrateGeminiPins(pins []string) []string {
	result := make([]string, 0, len(pins))
	hasVariant := false
	generic := false
	for _, value := range pins {
		parsed, err := domain.ParseSourceKey(value)
		if err == nil && parsed.Provider == "gemini" {
			if parsed.Variant == "" {
				generic = true
			} else {
				hasVariant = true
			}
		}
	}
	if !generic {
		return pins
	}
	for _, value := range pins {
		parsed, err := domain.ParseSourceKey(value)
		if err == nil && parsed.Provider == "gemini" && parsed.Variant == "" {
			continue
		}
		result = append(result, value)
	}
	if !hasVariant {
		result = append(result, (domain.SourceKey{Provider: "gemini", Variant: domain.VariantConfigured}).String())
	}
	return result
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *Session) trackedLocked(key domain.SourceKey) bool {
	for _, tracked := range s.prefs.Tracked {
		parsed, err := domain.ParseSourceKey(tracked)
		if err != nil {
			continue
		}
		if trackMatches(parsed, key) {
			return true
		}
	}
	return false
}

func trackMatches(tracked, candidate domain.SourceKey) bool {
	if tracked.Provider != candidate.Provider || tracked.Variant != candidate.Variant {
		return false
	}
	if tracked.AccountUnknown || tracked.Account == "" {
		return true
	}
	return tracked.Account == candidate.Account && !candidate.AccountUnknown
}

func (s *Session) signalEventLocked() {
	if s.eventsClosed {
		return
	}
	select {
	case s.events <- struct{}{}:
	default:
	}
}

// Events lets the TUI refresh its rendered model when live state changes.
func (s *Session) Events() <-chan struct{} { return s.events }

func (s *Session) intervalLoop() {
	defer s.wg.Done()
	for {
		interval := s.scheduler.Interval()
		if s.interval != nil {
			interval = s.interval()
		}
		timer := time.NewTimer(interval)
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.mu.RLock()
			keys := append([]string(nil), s.prefs.Tracked...)
			s.mu.RUnlock()
			for _, key := range keys {
				parsed, err := domain.ParseSourceKey(key)
				if err != nil {
					continue
				}
				s.mu.RLock()
				targets := s.refreshTargetsLocked(parsed)
				s.mu.RUnlock()
				for _, target := range targets {
					if s.autoPollAllowed(target) {
						s.enqueueAutomatic(target, 1)
					}
				}
			}
		}
	}
}

func asScoped(err error) *domain.ScopedError {
	if scoped, ok := domain.AsScopedError(err); ok {
		return scoped
	}
	return domain.NewScopedError(domain.ErrUnknown, "", "backend request failed")
}

func (s *Session) ensureSkeleton(key domain.SourceKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.snapshots[key.String()]; ok {
		return
	}
	s.snapshots[key.String()] = domain.UsageSnapshot{
		Key: key, ProviderID: key.Provider, DisplayName: s.displayNameLocked(key),
		State: domain.StatusQueued,
	}
}

func (s *Session) displayNameLocked(key domain.SourceKey) string {
	for _, descriptor := range s.catalog.Providers {
		if descriptor.ProviderID == key.Provider {
			return descriptor.DisplayName
		}
	}
	return key.Provider
}

func (s *Session) tracked(key domain.SourceKey) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return contains(s.prefs.Tracked, key.String())
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *Session) enqueue(key domain.SourceKey, priority int) {
	s.scheduler.Enqueue(refresh.Request{Key: key, Operation: refresh.OpUsage, Priority: priority, Context: "live", Manual: true})
}

func (s *Session) enqueueAutomatic(key domain.SourceKey, priority int) {
	s.mu.RLock()
	if configured, ok := s.priorities[key.String()]; ok {
		priority = configured
	}
	s.mu.RUnlock()
	s.scheduler.Enqueue(refresh.Request{Key: key, Operation: refresh.OpUsage, Priority: priority, Context: "live"})
}

// Prioritize marks the visible page and selected source ahead of background work.
func (s *Session) Prioritize(visible []string, selected string) {
	s.mu.Lock()
	s.priorities = map[string]int{}
	for _, key := range s.prefs.Tracked {
		s.priorities[key] = 1
	}
	for _, key := range visible {
		s.priorities[key] = 0
	}
	if selected != "" {
		s.priorities[selected] = 0
	}
	for key, priority := range s.priorities {
		if parsed, err := domain.ParseSourceKey(key); err == nil {
			s.scheduler.SetPriority(parsed, priority)
		}
	}
	s.mu.Unlock()
}

// StartBulk begins a single scheduler sweep and marks members pending.
func (s *Session) StartBulk(keys []string) string {
	parsedKeys := make([]domain.SourceKey, 0, len(keys))
	var blocked []domain.SourceKey
	for _, key := range keys {
		if parsed, err := domain.ParseSourceKey(key); err == nil {
			if s.activeProbe(parsed) {
				blocked = append(blocked, parsed)
				continue
			}
			parsedKeys = append(parsedKeys, parsed)
		}
	}
	for _, key := range blocked {
		s.mu.Lock()
		s.done[key.String()] = true
		s.snapshots[key.String()] = domain.UsageSnapshot{Key: key, ProviderID: key.Provider, DisplayName: s.displayNameLocked(key), State: domain.StatusBackendIncompatible, Error: domain.NewScopedError(domain.ErrUnsupported, key.String(), "Active probe is outside the prototype")}
		s.signalEventLocked()
		s.mu.Unlock()
	}
	_, _, active, existingID := s.scheduler.BulkProgress()
	s.mu.Lock()
	if active && existingID != "" {
		s.bulkID = existingID
		if s.bulkKeys == nil {
			s.bulkKeys = map[string]bool{}
		}
		for _, parsed := range parsedKeys {
			key := parsed.String()
			if !s.bulkKeys[key] {
				s.bulkKeys[key] = true
				s.done[key] = false
			}
		}
		for _, parsed := range blocked {
			s.bulkKeys[parsed.String()] = true
			s.done[parsed.String()] = true
		}
		s.bulkTotal = len(s.bulkKeys)
		s.mu.Unlock()
		s.scheduler.StartBulkWithID(existingID, parsedKeys, "live", true)
		return existingID
	}
	s.bulkSeq++
	id := "live-bulk-" + itoa(s.bulkSeq)
	s.bulkID = id
	s.bulkDone = 0
	s.bulkKeys = map[string]bool{}
	for _, parsed := range parsedKeys {
		s.bulkKeys[parsed.String()] = true
		s.done[parsed.String()] = false
	}
	for _, parsed := range blocked {
		s.bulkKeys[parsed.String()] = true
		s.done[parsed.String()] = true
	}
	s.bulkTotal = len(s.bulkKeys)
	s.mu.Unlock()
	s.scheduler.StartBulkWithID(id, parsedKeys, "live", true)
	return id
}

// BulkProgress reports live sweep progress when a sweep is active.
func (s *Session) BulkProgress() (int, int, bool) {
	s.mu.RLock()
	id := s.bulkID
	s.mu.RUnlock()
	done, total, active, sweepID := s.scheduler.BulkProgress()
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id == "" || len(s.bulkKeys) == 0 {
		return 0, 0, false
	}
	completed := 0
	for key := range s.bulkKeys {
		if s.done[key] {
			completed++
		}
	}
	if completed >= len(s.bulkKeys) {
		return completed, len(s.bulkKeys), false
	}
	if id != "" && sweepID == id {
		return done, total, active
	}
	return completed, len(s.bulkKeys), completed < len(s.bulkKeys)
}

// ScenarioName implements tui.Session.
func (s *Session) ScenarioName() string { return "live" }

// Clock implements tui.Session.
func (s *Session) Clock() string { return time.Now().UTC().Format(time.RFC3339) }

// SetScenarioName is a no-op in live mode.
func (s *Session) SetScenarioName(string) error { return nil }

// Snapshots returns a stable copy of the live collection.
func (s *Session) Snapshots(context.Context) []domain.UsageSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domain.UsageSnapshot, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots {
		result = append(result, snapshot)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key.String() < result[j].Key.String() })
	return result
}

// AllSnapshots is identical to Snapshots for live mode.
func (s *Session) AllSnapshots(ctx context.Context) []domain.UsageSnapshot {
	return s.Snapshots(ctx)
}

// BackendError reports startup discovery failures.
func (s *Session) BackendError() string { return s.backendErr }

// BackendVersion reports the detected CodexBar version.
func (s *Session) BackendVersion() string { return s.backendVersion }

// ConfigWarning reports a migration/corruption/newer-schema setup message.
func (s *Session) ConfigWarning() string { return s.storeWarning }

// Query returns the current search query (managed by the TUI).
func (s *Session) Query() string { return "" }

// StartRefresh queues a manual refresh and marks it pending.
func (s *Session) StartRefresh(key string) {
	parsed, err := domain.ParseSourceKey(key)
	if err != nil {
		return
	}
	if s.activeProbe(parsed) {
		s.mu.Lock()
		s.done[key] = true
		s.snapshots[key] = domain.UsageSnapshot{Key: parsed, ProviderID: parsed.Provider, DisplayName: s.displayNameLocked(parsed), State: domain.StatusBackendIncompatible, Error: domain.NewScopedError(domain.ErrUnsupported, key, "Active probe is outside the prototype")}
		s.signalEventLocked()
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	s.done[key] = false
	if _, ok := s.snapshots[key]; !ok {
		s.snapshots[key] = domain.UsageSnapshot{Key: parsed, ProviderID: parsed.Provider, DisplayName: s.displayNameLocked(parsed)}
	}
	s.snapshots[key] = withState(s.snapshots[key], domain.StatusQueued)
	s.mu.Unlock()
	s.enqueue(parsed, 0)
}

func withState(snapshot domain.UsageSnapshot, state domain.SourceStatus) domain.UsageSnapshot {
	snapshot.State = state
	return snapshot
}

// AdvanceRefresh reports whether the requested refresh is still pending.
func (s *Session) AdvanceRefresh(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	done, ok := s.done[key]
	if !ok {
		return false
	}
	return !done
}

// RefreshOutcome summarizes the scheduler's retained result.
func (s *Session) RefreshOutcome(key string) string {
	parsed, err := domain.ParseSourceKey(key)
	if err != nil {
		return "Unavailable"
	}
	entry := s.scheduler.State(parsed)
	if entry.Err != nil {
		if scoped, ok := domain.AsScopedError(entry.Err); ok {
			return scoped.Category.UserMessage()
		}
		return "Refresh failed"
	}
	if entry.LastSuccess != nil {
		return "Success"
	}
	return "Current"
}

// Catalog returns the discovered provider catalog.
func (s *Session) Catalog(context.Context) (backend.CatalogSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	catalog := s.catalog
	catalog.Providers = append([]domain.SourceDescriptor(nil), catalog.Providers...)
	for index := range catalog.Providers {
		catalog.Providers[index].Tracked = contains(s.prefs.Tracked, catalog.Providers[index].Key.String())
		catalog.Providers[index].Pinned = contains(s.prefs.Pins, catalog.Providers[index].Key.String())
	}
	return catalog, nil
}

// History loads history through the scheduler cache.
func (s *Session) History(ctx context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
	return s.HistoryWithForce(ctx, req, false)
}

// HistoryWithForce returns cached history immediately and refreshes in the
// background so the Bubble Tea event loop is never blocked by a backend read.
func (s *Session) HistoryWithForce(ctx context.Context, req backend.HistoryRequest, force bool) (domain.HistorySnapshot, error) {
	key, err := domain.ParseSourceKey(req.SourceKey)
	if err != nil {
		return domain.HistorySnapshot{}, err
	}
	request := refresh.Request{Key: key, Operation: refresh.OpHistory, Days: req.Days, Context: "live", Manual: force}
	cacheKey := key.String() + "|" + itoa(req.Days)
	now := time.Now()
	cached, hasCached := s.scheduler.CachedHistory(request)
	if hasCached && !force {
		return cached, nil
	}
	s.mu.RLock()
	failure, hasFailure := s.historyErrors[cacheKey]
	loading := s.historyLoading[cacheKey]
	s.mu.RUnlock()
	if hasFailure && now.Sub(failure.at) < s.scheduler.Interval() && !force {
		if hasCached {
			return cached, nil
		}
		return domain.HistorySnapshot{}, failure.err
	}
	if loading {
		return cached, nil
	}
	stale, hasStale := s.scheduler.CachedHistoryAny(request)
	if !hasCached && hasStale {
		cached = stale
		hasCached = true
	}
	s.mu.Lock()
	if s.historyLoading[cacheKey] {
		s.mu.Unlock()
		return cached, nil
	}
	s.historyLoading[cacheKey] = true
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		history, loadErr := s.scheduler.LoadHistory(s.ctx, request, force)
		s.mu.Lock()
		if loadErr != nil {
			s.historyErrors[cacheKey] = historyFailure{err: loadErr, at: time.Now()}
		} else {
			delete(s.historyErrors, cacheKey)
		}
		delete(s.historyLoading, cacheKey)
		s.signalEventLocked()
		s.mu.Unlock()
		_ = history
	}()
	if hasCached {
		return cached, nil
	}
	return domain.HistorySnapshot{}, nil
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [12]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[i:])
}

// Preferences returns the current normalized preferences.
func (s *Session) Preferences() config.Preferences {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clonePrefs(s.prefs)
}

func clonePrefs(prefs config.Preferences) config.Preferences {
	prefs.Tracked = append([]string(nil), prefs.Tracked...)
	prefs.Pins = append([]string(nil), prefs.Pins...)
	return prefs
}

// SavePreferences persists preferences and reconciles tracked work.
func (s *Session) SavePreferences(prefs config.Preferences) error {
	prefs = config.Normalize(prefs)
	if err := s.store.Save(prefs); err != nil {
		return err
	}
	s.mu.Lock()
	oldTracked := map[string]bool{}
	var invalidated []domain.SourceKey
	for _, key := range s.prefs.Tracked {
		oldTracked[key] = true
	}
	s.prefs = clonePrefs(prefs)
	for key := range oldTracked {
		if !contains(prefs.Tracked, key) {
			parsed, err := domain.ParseSourceKey(key)
			if err == nil {
				invalidated = append(invalidated, parsed)
				for snapshotKey, snapshot := range s.snapshots {
					if trackMatches(parsed, snapshot.Key) {
						delete(s.snapshots, snapshotKey)
						s.done[snapshotKey] = true
					}
				}
			} else {
				delete(s.snapshots, key)
			}
			s.done[key] = true
			for cacheKey := range s.historyErrors {
				if strings.HasPrefix(cacheKey, key+"|") {
					delete(s.historyErrors, cacheKey)
				}
			}
		}
	}
	for _, key := range prefs.Tracked {
		if oldTracked[key] {
			continue
		}
		if parsed, err := domain.ParseSourceKey(key); err == nil {
			s.snapshots[key] = domain.UsageSnapshot{Key: parsed, ProviderID: parsed.Provider, DisplayName: s.displayNameLocked(parsed), State: domain.StatusQueued}
		}
	}
	s.mu.Unlock()
	for _, key := range invalidated {
		s.scheduler.Invalidate(key)
	}
	for _, key := range prefs.Tracked {
		if oldTracked[key] {
			continue
		}
		if parsed, err := domain.ParseSourceKey(key); err == nil {
			if s.autoPollAllowed(parsed) {
				s.enqueueAutomatic(parsed, 0)
			} else {
				s.markAutoDisabled(parsed)
			}
		}
	}
	return nil
}

// Track updates tracking through preferences and queues newly tracked work.
func (s *Session) Track(key string, tracked bool) error {
	parsed, err := domain.ParseSourceKey(key)
	if err != nil {
		return err
	}
	s.mu.Lock()
	prefs := clonePrefs(s.prefs)
	prefs.Tracked = remove(prefs.Tracked, key)
	if tracked {
		prefs.Tracked = append(prefs.Tracked, key)
	}
	s.mu.Unlock()
	if err := s.SavePreferences(prefs); err != nil {
		return err
	}
	if !tracked {
		s.scheduler.Invalidate(parsed)
	}
	return nil
}

// Pin updates persisted pin ordering.
func (s *Session) Pin(key string, pinned bool) error {
	if _, err := domain.ParseSourceKey(key); err != nil {
		return err
	}
	s.mu.Lock()
	prefs := clonePrefs(s.prefs)
	prefs.Pins = remove(prefs.Pins, key)
	if pinned {
		prefs.Pins = append(prefs.Pins, key)
	}
	s.mu.Unlock()
	return s.SavePreferences(prefs)
}

func remove(values []string, target string) []string {
	result := values[:0]
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func safeError(err error) string {
	if scoped, ok := domain.AsScopedError(err); ok {
		return scoped.Error()
	}
	return "Backend setup failed"
}
