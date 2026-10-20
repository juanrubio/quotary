package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/domain"
)

func TestFixtureParityWithMockups(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "mockups", "fixtures.js"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "=")
	if start < 0 {
		t.Fatal("fixtures.js assignment not found")
	}
	var fromJS, embedded any
	if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimSpace(text[start+1:]), ";")), &fromJS); err != nil {
		t.Fatal(err)
	}
	raw, err := fs.ReadFile(FixtureFS(), "fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &embedded); err != nil {
		t.Fatal(err)
	}
	if failures := jsonDiff(fromJS, embedded, "$"); len(failures) > 0 {
		t.Fatalf("embedded demo fixtures drift from docs/mockups/fixtures.js:\n%s", strings.Join(failures, "\n"))
	}
}

func jsonDiff(want, got any, path string) []string {
	switch wantValue := want.(type) {
	case map[string]any:
		gotValue, ok := got.(map[string]any)
		if !ok {
			return []string{fmt.Sprintf("%s: type %T != %T", path, want, got)}
		}
		var failures []string
		keys := map[string]bool{}
		for key := range wantValue {
			keys[key] = true
		}
		for key := range gotValue {
			keys[key] = true
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			failures = append(failures, jsonDiff(wantValue[key], gotValue[key], path+"."+key)...)
		}
		return failures
	case []any:
		gotValue, ok := got.([]any)
		if !ok {
			return []string{fmt.Sprintf("%s: type %T != %T", path, want, got)}
		}
		if len(wantValue) != len(gotValue) {
			return []string{fmt.Sprintf("%s: length %d != %d", path, len(wantValue), len(gotValue))}
		}
		var failures []string
		for index := range wantValue {
			failures = append(failures, jsonDiff(wantValue[index], gotValue[index], fmt.Sprintf("%s[%d]", path, index))...)
		}
		return failures
	default:
		if !reflect.DeepEqual(want, got) {
			return []string{fmt.Sprintf("%s: %v != %v", path, want, got)}
		}
		return nil
	}
}

func TestFixtureParityNamesTheDifferingField(t *testing.T) {
	want := map[string]any{"clock": "2026-10-16T14:00:00Z", "sources": []any{map[string]any{"left": 72.0}}}
	got := map[string]any{"clock": "2026-10-16T14:00:01Z", "sources": []any{map[string]any{"left": 71.0}}}
	failures := jsonDiff(want, got, "$")
	if len(failures) != 2 || !strings.Contains(failures[0], "$.clock") || !strings.Contains(failures[1], "$.sources[0].left") {
		t.Fatalf("diagnostics = %v", failures)
	}
}

func TestScenariosAreDeterministic(t *testing.T) {
	for _, scenario := range []Scenario{ScenarioSubscription, ScenarioAttention, ScenarioCatalog, ScenarioEmpty} {
		first := MustNew(scenario)
		second := MustNew(scenario)
		catalogA, err := first.Catalog(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		catalogB, err := second.Catalog(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(catalogA, catalogB) {
			t.Fatalf("%s catalog is not deterministic", scenario)
		}
		for _, descriptor := range catalogA.Providers {
			if !descriptor.Tracked {
				continue
			}
			a, errA := first.Usage(context.Background(), backend.UsageRequest{ProviderID: descriptor.ProviderID, Variant: descriptor.Variant, SourceKey: descriptor.Key.String()})
			b, errB := second.Usage(context.Background(), backend.UsageRequest{ProviderID: descriptor.ProviderID, Variant: descriptor.Variant, SourceKey: descriptor.Key.String()})
			if (errA == nil) != (errB == nil) || !reflect.DeepEqual(a, b) {
				t.Fatalf("%s/%s is not deterministic", scenario, descriptor.Key.String())
			}
		}
	}
}

func TestCatalogIsBroadAndClassified(t *testing.T) {
	backendUnderTest := MustNew(ScenarioSubscription)
	catalog, err := backendUnderTest.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Providers) != 108 {
		t.Fatalf("catalog = %d, want 108 selectable sources", len(catalog.Providers))
	}
	byKey := map[string]domain.SourceDescriptor{}
	tracked := 0
	for _, descriptor := range catalog.Providers {
		byKey[descriptor.Key.String()] = descriptor
		if descriptor.Tracked {
			tracked++
		}
	}
	if tracked != 24 {
		t.Fatalf("tracked = %d, want 24", tracked)
	}
	if _, ok := byKey["gemini:web@unknown"]; !ok {
		t.Fatal("Gemini app variant missing")
	}
	if _, ok := byKey["gemini:api@unknown"]; !ok {
		t.Fatal("Gemini CLI variant missing")
	}
	azure := byKey["azureopenai:configured@unknown"]
	if azure.Kind != domain.KindMeteredAPI || azure.Capabilities.ActiveProbe != domain.CapabilitySupported {
		t.Fatalf("azure = %+v", azure)
	}
	if azure.Capabilities.BulkTrackable(azure.Kind) {
		t.Fatal("active probe must not be bulk-trackable")
	}
}

func TestAttentionScenarioStates(t *testing.T) {
	backendUnderTest := MustNew(ScenarioAttention)
	want := map[string]domain.SourceStatus{
		"antigravity:configured@unknown": domain.StatusExhausted,
		"kimi:configured@unknown":        domain.StatusPartial,
		"zed:configured@unknown":         domain.StatusUnavailable,
		"warp:configured@unknown":        domain.StatusLow,
		"windsurf:configured@unknown":    domain.StatusSignIn,
		"cursor:configured@unknown":      domain.StatusStale,
		"claude:configured@unknown":      domain.StatusCurrent,
	}
	for key, state := range want {
		snapshots, err := backendUnderTest.Usage(context.Background(), backend.UsageRequest{SourceKey: key})
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if len(snapshots) != 1 || snapshots[0].State != state {
			t.Fatalf("%s = %+v, want %s", key, snapshots, state)
		}
	}
}

func TestSubscriptionScenarioHasLowReadings(t *testing.T) {
	backendUnderTest := MustNew(ScenarioSubscription)
	catalog, err := backendUnderTest.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var snapshots []domain.UsageSnapshot
	for _, descriptor := range catalog.Providers {
		if !descriptor.Tracked {
			continue
		}
		result, err := backendUnderTest.Usage(context.Background(), backend.UsageRequest{SourceKey: descriptor.Key.String()})
		if err != nil {
			t.Fatal(err)
		}
		snapshots = append(snapshots, result...)
	}
	counts := domain.CountStates(snapshots)
	if counts.Total != 24 || counts.Current != 24 || counts.Low != 2 {
		t.Fatalf("counts = %+v", counts)
	}
}

func TestEmptyScenarioHasNoTrackedSources(t *testing.T) {
	backendUnderTest := MustNew(ScenarioEmpty)
	catalog, err := backendUnderTest.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Providers) != 108 {
		t.Fatalf("catalog = %d", len(catalog.Providers))
	}
	for _, descriptor := range catalog.Providers {
		if descriptor.Tracked {
			t.Fatal("empty scenario tracks a source")
		}
	}
	_, err = backendUnderTest.Usage(context.Background(), backend.UsageRequest{SourceKey: "codex:configured@unknown"})
	scoped, ok := domain.AsScopedError(err)
	if !ok || scoped.Category != domain.ErrUnavailable {
		t.Fatalf("empty usage error = %v", err)
	}
}

func TestMuseHistoryIsTokenOnly(t *testing.T) {
	backendUnderTest := MustNew(ScenarioSubscription)
	history, err := backendUnderTest.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", SourceKey: "muse:configured@unknown", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.TotalTokens == nil || *history.TotalTokens != 4740000 {
		t.Fatalf("total = %v", history.TotalTokens)
	}
	if history.Cost != nil || history.Priced {
		t.Fatal("Muse history must not invent dollars")
	}
	if len(history.MissingDays) != 1 || history.MissingDays[0] != "2026-10-12" {
		t.Fatalf("missing days = %v", history.MissingDays)
	}
	if history.Timezone != "UTC" || len(history.Models) != 3 {
		t.Fatalf("history = %+v", history)
	}
	if _, err := backendUnderTest.History(context.Background(), backend.HistoryRequest{ProviderID: "cursor", SourceKey: "cursor:configured@unknown"}); err == nil {
		t.Fatal("unsupported history should fail")
	}
}

func TestUnknownScenarioRejected(t *testing.T) {
	if _, err := New("production"); err == nil {
		t.Fatal("unknown scenario should fail")
	}
}

func TestHistoryCapabilitiesMatchReaders(t *testing.T) {
	backendUnderTest := MustNew(ScenarioSubscription)
	for _, provider := range []string{"claude", "codex", "muse", "antigravity"} {
		key := provider + ":configured@unknown"
		history, err := backendUnderTest.History(context.Background(), backend.HistoryRequest{ProviderID: provider, SourceKey: key, Days: 7})
		if err != nil {
			t.Fatalf("%s: %v", provider, err)
		}
		if history.TotalTokens == nil || history.Cost != nil || history.Priced {
			t.Fatalf("%s history = %+v", provider, history)
		}
	}
	thirty, err := backendUnderTest.History(context.Background(), backend.HistoryRequest{ProviderID: "claude", SourceKey: "claude:configured@unknown", Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if thirty.Days != 30 || len(thirty.Buckets) != 30 {
		t.Fatalf("30-day history = days:%d buckets:%d", thirty.Days, len(thirty.Buckets))
	}
}

func TestFullCatalogScenarioTracksSubscriptions(t *testing.T) {
	backendUnderTest := MustNew(ScenarioCatalog)
	catalog, err := backendUnderTest.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tracked := 0
	for _, descriptor := range catalog.Providers {
		if descriptor.Kind == domain.KindSubscription || descriptor.Kind == domain.KindMixed {
			if !descriptor.Tracked {
				t.Fatalf("%s should be tracked", descriptor.Key.String())
			}
			tracked++
			snapshots, err := backendUnderTest.Usage(context.Background(), backend.UsageRequest{SourceKey: descriptor.Key.String()})
			if err != nil || len(snapshots) != 1 {
				t.Fatalf("%s usage = %v, %v", descriptor.Key.String(), snapshots, err)
			}
		}
		if descriptor.Kind == domain.KindMeteredAPI && descriptor.Tracked {
			t.Fatalf("API source %s must not be bulk-tracked", descriptor.Key.String())
		}
	}
	if tracked <= 24 {
		t.Fatalf("full catalog tracked only %d subscriptions", tracked)
	}
}

func TestDemoAccountsUseUnknownSentinel(t *testing.T) {
	backendUnderTest := MustNew(ScenarioSubscription)
	for _, key := range []string{"claude:configured@unknown", "gemini:web@unknown"} {
		snapshots, err := backendUnderTest.Usage(context.Background(), backend.UsageRequest{SourceKey: key})
		if err != nil {
			t.Fatal(err)
		}
		snapshot := snapshots[0]
		if !snapshot.Key.AccountUnknown || snapshot.AccountKey != domain.UnknownAccount || snapshot.AccountLabel != "Account unknown" {
			t.Fatalf("%s account = %+v/%q", key, snapshot.Key, snapshot.AccountLabel)
		}
		if snapshot.Key.CoalescesWith(domain.SourceKey{Provider: "claude", Variant: domain.VariantConfigured, Account: "known"}) {
			t.Fatal("unknown account coalesced with a known account")
		}
	}
}

func TestWindowKeysAreUnique(t *testing.T) {
	backendUnderTest := MustNew(ScenarioAttention)
	catalog, err := backendUnderTest.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, descriptor := range catalog.Providers {
		if !descriptor.Tracked {
			continue
		}
		snapshots, err := backendUnderTest.Usage(context.Background(), backend.UsageRequest{SourceKey: descriptor.Key.String()})
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, window := range snapshots[0].Windows {
			if seen[window.Key] {
				t.Fatalf("%s duplicate window key %q", descriptor.Key, window.Key)
			}
			seen[window.Key] = true
		}
	}
}

func TestUsageRejectsUnvalidatedVariantsAndScopes(t *testing.T) {
	backendUnderTest := MustNew(ScenarioCatalog)
	for _, key := range []string{"claude:bogus@unknown", "claude:web@unknown", "notaprovider:configured@unknown"} {
		_, err := backendUnderTest.Usage(context.Background(), backend.UsageRequest{SourceKey: key})
		scoped, ok := domain.AsScopedError(err)
		if !ok || scoped.Category != domain.ErrConfig {
			t.Fatalf("%s error = %v", key, err)
		}
	}
	_, err := backendUnderTest.Usage(context.Background(), backend.UsageRequest{SourceKey: "azureopenai:configured@unknown"})
	scoped, ok := domain.AsScopedError(err)
	if !ok || scoped.Category != domain.ErrUnavailable {
		t.Fatalf("API usage error = %v", err)
	}
}

func TestHistoryFixturesAreOwnedByBackend(t *testing.T) {
	backendUnderTest := MustNew(ScenarioSubscription)
	first, err := backendUnderTest.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", SourceKey: "muse:configured@unknown", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if first.Buckets[0].Tokens == nil {
		t.Fatal("first bucket missing")
	}
	*first.Buckets[0].Tokens = 123
	first.Models[0].Name = "mutated"
	second, err := backendUnderTest.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", SourceKey: "muse:configured@unknown", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if *second.Buckets[0].Tokens != 420000 || second.Models[0].Name != "Model A" {
		t.Fatalf("fixture was mutated: %d %q", *second.Buckets[0].Tokens, second.Models[0].Name)
	}
}

func TestNormalizedFixtureParity(t *testing.T) {
	data, err := loadFixtures()
	if err != nil {
		t.Fatal(err)
	}
	backendUnderTest := MustNew(ScenarioSubscription)
	for _, source := range data.Sources {
		key := source.ID + "@unknown"
		snapshots, err := backendUnderTest.Usage(context.Background(), backend.UsageRequest{SourceKey: key})
		if err != nil {
			t.Fatalf("%s: %v", source.ID, err)
		}
		snapshot := snapshots[0]
		if snapshot.DisplayName != source.Name {
			t.Fatalf("%s name = %q want %q", source.ID, snapshot.DisplayName, source.Name)
		}
		if snapshot.Plan != source.Plan {
			t.Fatalf("%s plan = %q want %q", source.ID, snapshot.Plan, source.Plan)
		}
		if len(snapshot.Windows) != len(source.Windows) {
			t.Fatalf("%s windows = %d want %d", source.ID, len(snapshot.Windows), len(source.Windows))
		}
		for i, window := range source.Windows {
			remaining := snapshot.Windows[i].RemainingPercent()
			if window.Left == nil {
				if remaining != nil {
					t.Fatalf("%s/%d left should be unavailable", source.ID, i)
				}
				continue
			}
			if remaining == nil || *remaining != *window.Left {
				t.Fatalf("%s/%d left = %v want %v", source.ID, i, remaining, *window.Left)
			}
		}
	}
}

func TestSetupScenarioHasNoBackend(t *testing.T) {
	backendUnderTest := MustNew(ScenarioSetup)
	_, err := backendUnderTest.Catalog(context.Background())
	scoped, ok := domain.AsScopedError(err)
	if !ok || scoped.Category != domain.ErrConfig {
		t.Fatalf("catalog error = %v", err)
	}
	_, err = backendUnderTest.Usage(context.Background(), backend.UsageRequest{SourceKey: "claude:configured@unknown"})
	scoped, ok = domain.AsScopedError(err)
	if !ok || scoped.Category != domain.ErrConfig {
		t.Fatalf("usage error = %v", err)
	}
}

func TestCreditSourceHasBalanceOnlyReading(t *testing.T) {
	backendUnderTest := MustNew(ScenarioCatalog)
	snapshots, err := backendUnderTest.Usage(context.Background(), backend.UsageRequest{SourceKey: "openrouter:configured@unknown"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := snapshots[0]
	if len(snapshot.Windows) != 0 || len(snapshot.Balances) != 1 || snapshot.Balances[0].Value == nil {
		t.Fatalf("credit snapshot = %+v", snapshot)
	}
	if snapshot.State != domain.StatusCurrent {
		t.Fatalf("credit state = %s", snapshot.State)
	}
}

func TestAttentionIncludesUnsupportedDetailDisclosure(t *testing.T) {
	backendUnderTest := MustNew(ScenarioAttention)
	snapshots, err := backendUnderTest.Usage(context.Background(), backend.UsageRequest{SourceKey: "kimi:configured@unknown"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots[0].Details) != 1 || !snapshots[0].Details[0].Unsupported {
		t.Fatalf("kimi details = %+v", snapshots[0].Details)
	}
}

func TestSetupScenarioReportsBackendError(t *testing.T) {
	backendUnderTest := MustNew(ScenarioSetup)
	if backendUnderTest.BackendError() == "" {
		t.Fatal("setup scenario has no actionable backend error")
	}
}

func TestHistoryScopeValidation(t *testing.T) {
	backendUnderTest := MustNew(ScenarioSubscription)
	bad := []backend.HistoryRequest{
		{ProviderID: "claude", SourceKey: "claude:web@unknown"},
		{ProviderID: "claude", SourceKey: "claude:configured@known"},
		{ProviderID: "codex", SourceKey: "claude:configured@unknown"},
		{ProviderID: "claude", Variant: domain.VariantWeb, SourceKey: "claude:configured@unknown"},
		{ProviderID: "notaprovider", SourceKey: "notaprovider:configured@unknown"},
	}
	for _, req := range bad {
		_, err := backendUnderTest.History(context.Background(), req)
		scoped, ok := domain.AsScopedError(err)
		if !ok || scoped.Category != domain.ErrConfig {
			t.Fatalf("%+v error = %v", req, err)
		}
	}
	canonical, err := backendUnderTest.History(context.Background(), backend.HistoryRequest{ProviderID: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if canonical.Key.String() != "claude:configured@unknown" {
		t.Fatalf("canonical key = %s", canonical.Key.String())
	}
}

func TestUsageRejectsConflictingScopes(t *testing.T) {
	backendUnderTest := MustNew(ScenarioSubscription)
	bad := []backend.UsageRequest{
		{ProviderID: "codex", SourceKey: "claude:configured@unknown"},
		{ProviderID: "gemini", Variant: domain.VariantAPI, SourceKey: "gemini:web@unknown"},
		{ProviderID: "claude", Variant: domain.VariantWeb, SourceKey: "claude:configured@unknown"},
	}
	for _, req := range bad {
		_, err := backendUnderTest.Usage(context.Background(), req)
		scoped, ok := domain.AsScopedError(err)
		if !ok || scoped.Category != domain.ErrConfig {
			t.Fatalf("%+v error = %v", req, err)
		}
	}
}

type referenceScenarios struct {
	Attention map[string]referencePatch `json:"attention"`
}

type referencePatch struct {
	State       string           `json:"state"`
	Age         string           `json:"age"`
	Windows     *[]fixtureWindow `json:"windows"`
	WindowIndex *int             `json:"windowIndex"`
	WindowLeft  *float64         `json:"windowLeft"`
}

func loadReferenceScenarios(t *testing.T) referenceScenarios {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "mockups", "scenarios.js"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "=")
	if start < 0 {
		t.Fatal("scenarios.js assignment not found")
	}
	var scenarios referenceScenarios
	if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimSpace(text[start+1:]), ";")), &scenarios); err != nil {
		t.Fatal(err)
	}
	return scenarios
}

func TestAttentionScenarioMatchesSharedReference(t *testing.T) {
	data, err := loadFixtures()
	if err != nil {
		t.Fatal(err)
	}
	scenarios := loadReferenceScenarios(t)
	if failures := attentionScenarioFailures(data, scenarios); len(failures) > 0 {
		t.Fatal(strings.Join(failures, "\n"))
	}

	mutations := []struct {
		name   string
		mutate func(*referenceScenarios)
	}{
		{"shifted exhausted index", func(s *referenceScenarios) {
			index := 1
			s.Attention["antigravity"] = referencePatch{State: "exhausted", WindowIndex: &index, WindowLeft: floatPtr(0)}
		}},
		{"changed kimi value", func(s *referenceScenarios) {
			windows := []fixtureWindow{{Label: "Plan", Left: floatPtr(42)}}
			patch := s.Attention["kimi"]
			patch.Windows = &windows
			s.Attention["kimi"] = patch
		}},
		{"changed kimi reset", func(s *referenceScenarios) {
			windows := []fixtureWindow{{Label: "Plan", ResetMinutes: intPtr(10)}}
			patch := s.Attention["kimi"]
			patch.Windows = &windows
			s.Attention["kimi"] = patch
		}},
		{"missing cursor transform", func(s *referenceScenarios) {
			delete(s.Attention, "cursor")
		}},
		{"unknown provider", func(s *referenceScenarios) {
			s.Attention["unknown-provider"] = referencePatch{State: "current"}
		}},
	}
	for _, mutation := range mutations {
		copyScenarios := loadReferenceScenarios(t)
		mutation.mutate(&copyScenarios)
		if failures := attentionScenarioFailures(data, copyScenarios); len(failures) == 0 {
			t.Fatalf("parity check did not reject %s", mutation.name)
		}
		if failures := attentionScenarioFailures(data, loadReferenceScenarios(t)); len(failures) > 0 {
			t.Fatalf("control parity failed after %s: %v", mutation.name, failures)
		}
	}
}

func attentionScenarioFailures(data fixtures, scenarios referenceScenarios) []string {
	var failures []string
	attentionBackend := MustNew(ScenarioAttention)
	subscriptionBackend := MustNew(ScenarioSubscription)
	knownProviders := map[string]bool{}
	for _, source := range data.Sources {
		knownProviders[source.Provider] = true
	}
	for provider := range scenarios.Attention {
		if !knownProviders[provider] {
			failures = append(failures, fmt.Sprintf("scenarios.js unknown attention provider %q", provider))
		}
	}
	stateMap := map[string]domain.SourceStatus{
		"current": domain.StatusCurrent, "signin": domain.StatusSignIn,
		"stale": domain.StatusStale, "missing": domain.StatusPartial,
		"unavailable": domain.StatusUnavailable, "exhausted": domain.StatusExhausted,
	}
	for _, source := range data.Sources {
		key := source.ID + "@unknown"
		attentionSnapshots, err := attentionBackend.Usage(context.Background(), backend.UsageRequest{SourceKey: key})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", source.ID, err))
			continue
		}
		baseSnapshots, err := subscriptionBackend.Usage(context.Background(), backend.UsageRequest{SourceKey: key})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s base: %v", source.ID, err))
			continue
		}
		snapshot := attentionSnapshots[0]
		patch, patched := scenarios.Attention[source.Provider]
		wantWindows := append([]fixtureWindow(nil), source.Windows...)
		if patched && patch.Windows != nil {
			wantWindows = append([]fixtureWindow(nil), (*patch.Windows)...)
		}
		if patched && patch.WindowIndex != nil {
			if *patch.WindowIndex < 0 || *patch.WindowIndex >= len(wantWindows) {
				failures = append(failures, fmt.Sprintf("%s windowIndex %d out of range", source.ID, *patch.WindowIndex))
				continue
			}
			wantWindows[*patch.WindowIndex].Left = patch.WindowLeft
		}
		if len(snapshot.Windows) != len(wantWindows) {
			failures = append(failures, fmt.Sprintf("%s windows = %d, want %d", source.ID, len(snapshot.Windows), len(wantWindows)))
		} else {
			for index := range wantWindows {
				failures = append(failures, windowMismatches(source.ID, index, snapshot.Windows[index], wantWindows[index], attentionBackend.clock)...)
			}
		}
		wantState := baseSnapshots[0].State
		if patched && patch.State != "" {
			mapped, ok := stateMap[patch.State]
			if !ok {
				failures = append(failures, fmt.Sprintf("%s unknown reference state %q", source.ID, patch.State))
				continue
			}
			wantState = mapped
		}
		if snapshot.State != wantState {
			failures = append(failures, fmt.Sprintf("%s state = %s, want %s", source.ID, snapshot.State, wantState))
		}
		wantAge := parseAge(source.Age)
		if patched && patch.Age != "" {
			wantAge = parseAge(patch.Age)
		}
		if snapshot.SampleAt != nil {
			if gotAge := attentionBackend.clock.Sub(*snapshot.SampleAt); gotAge != wantAge {
				failures = append(failures, fmt.Sprintf("%s age = %s, want %s", source.ID, gotAge, wantAge))
			}
		}
	}
	return failures
}

func windowMismatches(source string, index int, got domain.AllowanceWindow, want fixtureWindow, clock time.Time) []string {
	label := fmt.Sprintf("%s window %d", source, index)
	var failures []string
	if got.Label != want.Label {
		failures = append(failures, fmt.Sprintf("%s label = %q, want %q", label, got.Label, want.Label))
	}
	wantGroup := ""
	if want.Group != nil {
		wantGroup = *want.Group
	}
	if got.Group != wantGroup {
		failures = append(failures, fmt.Sprintf("%s group = %q, want %q", label, got.Group, wantGroup))
	}
	if err := remainingMismatch(label, got.RemainingPercent(), want.Left); err != "" {
		failures = append(failures, err)
	}
	if want.ResetMinutes == nil {
		if got.ResetAt != nil {
			failures = append(failures, fmt.Sprintf("%s reset = %s, want unavailable", label, got.ResetAt))
		}
	} else {
		wantReset := clock.Add(time.Duration(*want.ResetMinutes) * time.Minute)
		if got.ResetAt == nil || !got.ResetAt.Equal(wantReset) {
			failures = append(failures, fmt.Sprintf("%s reset = %v, want %s", label, got.ResetAt, wantReset))
		}
	}
	return failures
}

func floatPtr(value float64) *float64 { return &value }
func intPtr(value int) *int           { return &value }

func remainingMismatch(label string, got, want *float64) string {
	if want == nil {
		if got != nil {
			return fmt.Sprintf("%s remaining = %.0f, want unavailable", label, *got)
		}
		return ""
	}
	if got == nil || *got != *want {
		return fmt.Sprintf("%s remaining = %v, want %.0f", label, got, *want)
	}
	return ""
}
