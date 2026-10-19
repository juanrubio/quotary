package demo

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/domain"
)

// Runner is the executable seam. The demo uses an in-memory offline runner; an
// operating-system runner is never part of demo startup.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// OfflineRunner returns fictional version output without executing anything.
type OfflineRunner struct{}

// Run implements Runner without external access.
func (OfflineRunner) Run(context.Context, string, ...string) ([]byte, error) {
	return []byte("demo-0.1.0"), nil
}

// Deps are the isolation seams available to the demo. Store is the in-memory
// preference store; Fixtures is an in-memory filesystem; Runner is an in-memory
// command seam. The demo has no process or network dependency.
type Deps struct {
	Store    config.Store
	Fixtures fs.FS
	Runner   Runner
}

type refreshEntry struct {
	stage      int
	generation uint64
}

// Runtime is the stateful demo application: backend, in-memory preferences, and
// deterministic queued -> fetching -> current refresh transitions.
type Runtime struct {
	scenario Scenario
	backend  *Backend
	store    config.Store
	runner   Runner
	deps     Deps
	refresh  map[string]*refreshEntry
	outcome  map[string]string
	query    string
}

// NewRuntime builds a demo runtime. Preferences are seeded from the scenario,
// then mutate in memory only.
func NewRuntime(scenario Scenario, deps Deps) (*Runtime, error) {
	backendUnderTest, err := newFromFS(scenario, deps.Fixtures)
	if err != nil {
		return nil, err
	}
	runtime := &Runtime{
		scenario: scenario,
		backend:  backendUnderTest,
		store:    deps.Store,
		deps:     deps,
		refresh:  map[string]*refreshEntry{},
		outcome:  map[string]string{},
	}
	if runtime.store == nil {
		runtime.store = config.NewMemoryStore(config.Defaults())
	}
	runtime.runner = deps.Runner
	if runtime.runner == nil {
		runtime.runner = OfflineRunner{}
	}
	if _, err := runtime.runner.Run(context.Background(), "demo", "--version"); err != nil {
		return nil, fmt.Errorf("demo runner: %w", err)
	}
	if err := runtime.seed(); err != nil {
		return nil, err
	}
	if scenario == ScenarioNoResults {
		runtime.query = "zzz-no-matching-source"
	}
	return runtime, nil
}

// ScenarioName returns the active scenario.
func (r *Runtime) ScenarioName() string { return string(r.scenario) }

// Clock returns the fixed demo clock.
func (r *Runtime) Clock() string { return r.backend.clock.Format("2006-01-02T15:04:05Z07:00") }

// Preferences returns the current in-memory preferences.
func (r *Runtime) Preferences() config.Preferences { return r.store.Load() }

// Catalog applies current tracking and pin preferences.
func (r *Runtime) Catalog(ctx context.Context) (backend.CatalogSnapshot, error) {
	catalog, err := r.backend.Catalog(ctx)
	if err != nil {
		return backend.CatalogSnapshot{}, err
	}
	prefs := r.store.Load()
	tracked := stringSet(prefs.Tracked)
	pins := map[string]int{}
	for index, key := range prefs.Pins {
		pins[key] = index
	}
	for i := range catalog.Providers {
		descriptor := &catalog.Providers[i]
		key := descriptor.Key.String()
		descriptor.Tracked = tracked[key]
		if order, ok := pins[key]; ok {
			descriptor.Pinned = true
			descriptor.PinOrder = order
		} else {
			descriptor.Pinned = false
		}
	}
	return catalog, nil
}

// Snapshots returns all tracked readings in default order, with refresh stages
// applied and unavailable scoped errors preserved.
func (r *Runtime) Snapshots(ctx context.Context) []domain.UsageSnapshot {
	all := r.AllSnapshots(ctx)
	if r.query == "" {
		return all
	}
	query := lower(r.query)
	var filtered []domain.UsageSnapshot
	for _, snapshot := range all {
		if containsText(snapshot.DisplayName, query) || containsText(snapshot.Key.String(), query) {
			filtered = append(filtered, snapshot)
		}
	}
	return filtered
}

// AllSnapshots returns the tracked collection without query filtering.
func (r *Runtime) AllSnapshots(ctx context.Context) []domain.UsageSnapshot {
	prefs := r.store.Load()
	catalog, _ := r.backend.Catalog(ctx)
	names := map[string]string{}
	for _, descriptor := range catalog.Providers {
		names[descriptor.Key.String()] = descriptor.DisplayName
	}
	var snapshots []domain.UsageSnapshot
	for _, key := range prefs.Tracked {
		result, err := r.backend.Usage(ctx, backend.UsageRequest{SourceKey: key})
		if err != nil {
			parsed, _ := domain.ParseSourceKey(key)
			scoped, _ := domain.AsScopedError(err)
			snapshot := domain.UsageSnapshot{Key: parsed, ProviderID: parsed.Provider, DisplayName: names[key], Error: scoped}
			if scoped != nil {
				snapshot.State = scoped.Category.UXState()
			} else {
				snapshot.State = domain.StatusError
			}
			if entry := r.refresh[key]; entry != nil {
				switch entry.stage {
				case 1:
					snapshot.State = domain.StatusQueued
				case 2:
					snapshot.State = domain.StatusFetching
				}
			}
			snapshots = append(snapshots, snapshot)
			continue
		}
		for _, snapshot := range result {
			if entry := r.refresh[key]; entry != nil {
				switch entry.stage {
				case 1:
					snapshot.State = domain.StatusQueued
				case 2:
					snapshot.State = domain.StatusFetching
				}
			}
			snapshots = append(snapshots, snapshot)
		}
	}
	pins := map[string]int{}
	for index, key := range prefs.Pins {
		pins[key] = index
	}
	domain.SortSnapshots(snapshots, pins)
	return snapshots
}

// StartRefresh queues a deterministic refresh for key.
func (r *Runtime) StartRefresh(key string) {
	entry := r.refresh[key]
	if entry == nil {
		entry = &refreshEntry{}
		r.refresh[key] = entry
	}
	entry.generation++
	entry.stage = 1
	r.outcome[key] = "Queued"
}

// AdvanceRefresh moves queued -> fetching -> current. It reports whether another
// tick is needed to finish the transition.
func (r *Runtime) AdvanceRefresh(key string) bool {
	entry := r.refresh[key]
	if entry == nil {
		return false
	}
	switch entry.stage {
	case 1:
		entry.stage = 2
		r.outcome[key] = "Fetching"
		return true
	case 2:
		entry.stage = 0
		snapshots, err := r.backend.Usage(context.Background(), backend.UsageRequest{SourceKey: key})
		if err != nil {
			scoped, ok := domain.AsScopedError(err)
			if ok {
				r.outcome[key] = scoped.Category.UserMessage()
			} else {
				r.outcome[key] = "Refresh failed"
			}
			return false
		}
		for _, snapshot := range snapshots {
			if snapshot.Error != nil {
				r.outcome[key] = snapshot.Error.Category.UserMessage()
				return false
			}
		}
		r.outcome[key] = "Success"
		return false
	default:
		return false
	}
}

// RefreshOutcome reports the last simulated refresh result for key.
func (r *Runtime) RefreshOutcome(key string) string {
	if outcome := r.outcome[key]; outcome != "" {
		return outcome
	}
	return "Current"
}

// RefreshStage reports the simulated stage for tests and UI copy.
func (r *Runtime) RefreshStage(key string) string {
	entry := r.refresh[key]
	if entry == nil {
		return "current"
	}
	switch entry.stage {
	case 1:
		return "queued"
	case 2:
		return "fetching"
	default:
		return "current"
	}
}

// SetScenario switches the deterministic scenario and reseeds demo preferences.
// Theme, appearance, and layout preferences are preserved.
func (r *Runtime) SetScenario(scenario Scenario) error {
	next, err := newFromFS(scenario, r.deps.Fixtures)
	if err != nil {
		return err
	}
	r.scenario = scenario
	r.backend = next
	r.refresh = map[string]*refreshEntry{}
	r.outcome = map[string]string{}
	prefs := r.store.Load()
	prefs.Tracked = nil
	prefs.Pins = nil
	r.query = ""
	if err := r.store.Save(prefs); err != nil {
		return err
	}
	if err := r.seed(); err != nil {
		return err
	}
	if scenario == ScenarioNoResults {
		r.query = "zzz-no-matching-source"
	}
	return nil
}

// Track toggles a source in the in-memory demo preferences.
func (r *Runtime) Track(key string, tracked bool) error {
	prefs := r.store.Load()
	prefs.Tracked = toggle(prefs.Tracked, key, tracked)
	if !tracked {
		prefs.Pins = remove(prefs.Pins, key)
	}
	if err := r.store.Save(prefs); err != nil {
		return err
	}
	delete(r.refresh, key)
	return nil
}

// Pin toggles a pinned source in the in-memory demo preferences.
func (r *Runtime) Pin(key string, pinned bool) error {
	prefs := r.store.Load()
	if !contains(prefs.Tracked, key) {
		return fmt.Errorf("%s is not tracked", key)
	}
	prefs.Pins = toggle(prefs.Pins, key, pinned)
	return r.store.Save(prefs)
}

// History delegates to the demo backend.
func (r *Runtime) History(ctx context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
	return r.backend.History(ctx, req)
}

// Query returns the demo search query.
func (r *Runtime) Query() string { return r.query }

// SetQuery changes the demo query without touching the underlying collection.
func (r *Runtime) SetQuery(query string) { r.query = query }

func lower(value string) string { return strings.ToLower(value) }

func containsText(value, query string) bool {
	return strings.Contains(strings.ToLower(value), query)
}

func (r *Runtime) seed() error {
	catalog, err := r.backend.Catalog(context.Background())
	if err != nil {
		return nil
	}
	prefs := r.store.Load()
	if len(prefs.Tracked) == 0 && r.scenario != ScenarioEmpty && r.scenario != ScenarioSetup {
		for _, descriptor := range catalog.Providers {
			if descriptor.Tracked {
				prefs.Tracked = append(prefs.Tracked, descriptor.Key.String())
			}
		}
		sort.Strings(prefs.Tracked)
	}
	if len(prefs.Pins) == 0 && r.scenario != ScenarioEmpty && r.scenario != ScenarioSetup {
		var pinned []domain.SourceDescriptor
		for _, descriptor := range catalog.Providers {
			if descriptor.Pinned {
				pinned = append(pinned, descriptor)
			}
		}
		sort.Slice(pinned, func(i, j int) bool { return pinned[i].PinOrder < pinned[j].PinOrder })
		for _, descriptor := range pinned {
			prefs.Pins = append(prefs.Pins, descriptor.Key.String())
		}
	}
	return r.store.Save(prefs)
}

func stringSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func toggle(values []string, target string, on bool) []string {
	if on && !contains(values, target) {
		return append(values, target)
	}
	if !on {
		return remove(values, target)
	}
	return values
}

func remove(values []string, target string) []string {
	var result []string
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

// SetScenarioName is the string-based scenario switch used by the TUI seam.
func (r *Runtime) SetScenarioName(name string) error {
	return r.SetScenario(Scenario(name))
}

// BackendError reports the simulated setup failure, if any.
func (r *Runtime) BackendError() string { return r.backend.BackendError() }

// SavePreferences persists through the injected store. Demo uses memory only.
func (r *Runtime) SavePreferences(prefs config.Preferences) error {
	return r.store.Save(prefs)
}
