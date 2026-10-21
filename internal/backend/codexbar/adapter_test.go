package codexbar

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/domain"
)

type call struct {
	path string
	args []string
	cap  int
}

type fakeRunner struct {
	calls []call
	fn    func(ctx context.Context, path string, args []string, max int) ([]byte, []byte, error)
}

func (f *fakeRunner) Run(ctx context.Context, path string, args []string, max int) ([]byte, []byte, error) {
	f.calls = append(f.calls, call{path: path, args: append([]string(nil), args...), cap: max})
	if f.fn != nil {
		return f.fn(ctx, path, args, max)
	}
	return nil, nil, nil
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "codexbar", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func adapterWith(data []byte) (*Adapter, *fakeRunner) {
	runner := &fakeRunner{fn: func(context.Context, string, []string, int) ([]byte, []byte, error) {
		return data, nil, nil
	}}
	return &Adapter{path: "/fixture/codexbar", runner: runner}, runner
}

func categoryOf(t *testing.T, err error) domain.ErrorCategory {
	t.Helper()
	scoped, ok := domain.AsScopedError(err)
	if !ok {
		t.Fatalf("error is not scoped: %v", err)
	}
	return scoped.Category
}

func TestVersionUsesDirectArgsAndDeadlineSeam(t *testing.T) {
	a, runner := adapterWith([]byte("CodexBar 0.77.0\n"))
	info, err := a.Version(context.Background())
	if err != nil || info.Version != "CodexBar 0.77.0" {
		t.Fatalf("version=%+v err=%v", info, err)
	}
	if len(runner.calls) != 1 || strings.Join(runner.calls[0].args, " ") != "--version" {
		t.Fatalf("calls=%+v", runner.calls)
	}
}

func TestCatalogPreservesEnabledDefaultsAndConservativeUnknowns(t *testing.T) {
	a, _ := adapterWith(fixture(t, "catalog.json"))
	catalog, err := a.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Providers) != 3 {
		t.Fatalf("providers=%d", len(catalog.Providers))
	}
	codex := catalog.Providers[0]
	if codex.ProviderID != "codex" || !codex.Available || codex.Kind != domain.KindUnknown {
		t.Fatalf("codex=%+v", codex)
	}
	if codex.Capabilities.Usage.Supported() || codex.Capabilities.ActiveProbe != domain.CapabilityUnknown {
		t.Fatal("catalog invented capabilities")
	}
	gemini := catalog.Providers[1]
	if gemini.Available != true {
		t.Fatal("defaultEnabled provider not available")
	}
}

func TestUsageWindowsCreditsDetailsAndMultiAccount(t *testing.T) {
	a, _ := adapterWith(fixture(t, "usage_windows.json"))
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex", SourceKey: "codex@account-one"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshots=%d", len(snapshots))
	}
	codex := snapshots[0]
	if codex.Key.String() != "codex@account-one" || codex.Plan != "Pro" || codex.ReportedSource != "web" {
		t.Fatalf("codex identity=%+v", codex)
	}
	if len(codex.Windows) != 2 || codex.Windows[0].Label != "Session" || codex.Windows[0].UsedPercent == nil || *codex.Windows[0].UsedPercent != 20 {
		t.Fatalf("windows=%+v", codex.Windows)
	}
	if codex.Windows[1].UsedPercent == nil || *codex.Windows[1].UsedPercent != 40 {
		t.Fatalf("derived percentage=%+v", codex.Windows[1])
	}
	if len(codex.Balances) != 1 || codex.Balances[0].Value == nil || *codex.Balances[0].Value != 12.34 || codex.Balances[0].Unit != "USD" {
		t.Fatalf("balances=%+v", codex.Balances)
	}
	if len(codex.Details) != 1 || len(codex.Details[0].Rows) != 1 || len(codex.Details[0].Bars) != 1 || len(codex.Details[0].Points) != 1 {
		t.Fatalf("details=%+v", codex.Details)
	}
	claude := snapshots[1]
	if claude.Key.Provider != "claude" || claude.Quality != domain.QualityPartial {
		t.Fatalf("second record=%+v", claude)
	}
	if claude.Windows[0].RemainingPercent() == nil || *claude.Windows[0].RemainingPercent() != 0 {
		t.Fatal("fractional over-limit remaining not clamped to zero")
	}
}

func TestUsageExpandsEveryAccount(t *testing.T) {
	a, _ := adapterWith(fixture(t, "usage_accounts.json"))
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "muse"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshots=%d", len(snapshots))
	}
	if snapshots[0].Key.Account != "alpha" || snapshots[1].Key.Account != "beta" {
		t.Fatalf("accounts=%s,%s", snapshots[0].Key, snapshots[1].Key)
	}
}

func TestUsageZeroIsMeasuredAndMissingIsNil(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"label":"Zero","used":0,"limit":0},"secondary":{"label":"Missing"}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	windows := snapshots[0].Windows
	if windows[0].Used == nil || *windows[0].Used != 0 {
		t.Fatal("measured zero was lost")
	}
	if windows[1].Used != nil || windows[1].UsedPercent != nil || windows[1].Limit != nil {
		t.Fatal("missing values became zero")
	}
}

func TestUsageRejectsInvalidVariantBeforeSpawning(t *testing.T) {
	a, runner := adapterWith(nil)
	_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "gemini", Variant: domain.Variant("bogus"), SourceKey: "gemini:bogus"})
	if err == nil || categoryOf(t, err) != domain.ErrUnsupported {
		t.Fatalf("err=%v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("backend spawned for invalid variant: %+v", runner.calls)
	}
}

func TestUsagePassesOnlyExplicitValidatedSource(t *testing.T) {
	a, runner := adapterWith([]byte(`{"provider":"gemini","primary":{"label":"Web","usedPercent":1}}`))
	_, _ = a.Usage(context.Background(), backend.UsageRequest{ProviderID: "gemini", Variant: domain.VariantWeb})
	_, _ = a.Usage(context.Background(), backend.UsageRequest{ProviderID: "gemini", Variant: domain.VariantConfigured})
	if got := strings.Join(runner.calls[0].args, " "); !strings.Contains(got, "--source web") {
		t.Fatalf("web args=%q", got)
	}
	if got := strings.Join(runner.calls[1].args, " "); strings.Contains(got, "--source") {
		t.Fatalf("configured args added source: %q", got)
	}
}

func TestStructuredErrorOnExitZeroAndFailureCategories(t *testing.T) {
	a, _ := adapterWith(fixture(t, "error_exit0.json"))
	_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || categoryOf(t, err) != domain.ErrAuth {
		t.Fatalf("err=%v", err)
	}
	runner := &fakeRunner{fn: func(context.Context, string, []string, int) ([]byte, []byte, error) {
		return nil, fixture(t, "error_unavailable.txt"), &ProcessError{Code: 1}
	}}
	a = &Adapter{path: "/fixture/codexbar", runner: runner}
	_, err = a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || categoryOf(t, err) != domain.ErrUnavailable {
		t.Fatalf("err=%v", err)
	}
}

func TestNonzeroExitRetainsValidRows(t *testing.T) {
	runner := &fakeRunner{fn: func(context.Context, string, []string, int) ([]byte, []byte, error) {
		return fixture(t, "partial_success.json"), []byte("warning"), &ProcessError{Code: 2}
	}}
	a := &Adapter{path: "/fixture/codexbar", runner: runner}
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if len(snapshots) != 2 || err == nil || categoryOf(t, err) != domain.ErrProcessFailure {
		t.Fatalf("snapshots=%d err=%v", len(snapshots), err)
	}
}

func TestMalformedEmptyUnsupportedAndCaps(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		err  error
		want domain.ErrorCategory
	}{
		{"malformed", fixture(t, "malformed.json"), nil, domain.ErrMalformedJSON},
		{"empty", []byte("  \n"), nil, domain.ErrEmptyStdout},
		{"unsupported", fixture(t, "unsupported.json"), nil, domain.ErrUnsupported},
		{"process", nil, &ProcessError{Code: 1}, domain.ErrProcessFailure},
		{"timeout", nil, ErrTimeout, domain.ErrTimeout},
		{"cap", nil, ErrOutputCap, domain.ErrOutputCap},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{fn: func(context.Context, string, []string, int) ([]byte, []byte, error) {
				return tc.data, nil, tc.err
			}}
			a := &Adapter{path: "/fixture/codexbar", runner: runner}
			_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
			if err == nil || categoryOf(t, err) != tc.want {
				t.Fatalf("err=%v want=%s", err, tc.want)
			}
		})
	}
}

func TestOversizedStdoutIsRejectedEvenIfRunnerReturnsIt(t *testing.T) {
	runner := &fakeRunner{fn: func(context.Context, string, []string, int) ([]byte, []byte, error) {
		return []byte(strings.Repeat("x", usageCap+1)), nil, nil
	}}
	a := &Adapter{path: "/fixture/codexbar", runner: runner}
	_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || categoryOf(t, err) != domain.ErrOutputCap {
		t.Fatalf("err=%v", err)
	}
}

func TestHistoryParsesDaysModelsCostAndPartial(t *testing.T) {
	a, runner := adapterWith(fixture(t, "history.json"))
	a.nativeOnly = true
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", SourceKey: "codex", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Buckets) != 3 || history.TotalTokens == nil || *history.TotalTokens != 3000 {
		t.Fatalf("history=%+v", history)
	}
	if len(history.Models) != 2 || history.Cost == nil || history.Cost.Amount != 1.25 || history.Cost.Currency != "USD" {
		t.Fatalf("models/cost=%+v %+v", history.Models, history.Cost)
	}
	if !history.Partial || history.Incomplete != 1 || len(history.MissingDays) != 1 {
		t.Fatalf("partial=%+v", history)
	}
	if history.Provenance != "provider-native local records" {
		t.Fatalf("provenance=%q", history.Provenance)
	}
	if got := strings.Join(runner.calls[0].args, " "); !strings.Contains(got, "--provider-native-only") {
		t.Fatalf("native-only flag missing: %q", got)
	}
}

func TestHistoryFallsBackWhenNativeOnlyUnavailable(t *testing.T) {
	calls := 0
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args []string, _ int) ([]byte, []byte, error) {
		calls++
		if len(args) > 0 && args[len(args)-1] == "--provider-native-only" {
			return nil, []byte("unknown option"), &ProcessError{Code: 2}
		}
		return fixture(t, "history.json"), nil, nil
	}}
	a := &Adapter{path: "/fixture/codexbar", runner: runner, nativeOnly: true}
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "claude", Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !strings.Contains(history.Provenance, "unavailable") {
		t.Fatalf("calls=%d provenance=%q", calls, history.Provenance)
	}
}

func TestHistoryRejectsInvalidRange(t *testing.T) {
	a, runner := adapterWith(nil)
	_, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", Days: 14})
	if err == nil || categoryOf(t, err) != domain.ErrUnsupported || len(runner.calls) != 0 {
		t.Fatalf("err=%v calls=%d", err, len(runner.calls))
	}
}

func TestExecutableResolution(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "codexbar")
	if err := os.WriteFile(explicit, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a, err := New(Options{Path: explicit, Runner: &fakeRunner{}})
	if err != nil || a.Path() != explicit {
		t.Fatalf("explicit=%v err=%v", a, err)
	}
	if _, err := New(Options{Path: filepath.Join(dir, "missing"), Runner: &fakeRunner{}}); err == nil || categoryOf(t, err) != domain.ErrConfig {
		t.Fatalf("invalid explicit path err=%v", err)
	}
	env := filepath.Join(dir, "env-codexbar")
	if err := os.WriteFile(env, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a, err = New(Options{EnvPath: env, Path: explicit, Runner: &fakeRunner{}})
	if err != nil || a.Path() != env {
		t.Fatalf("env order path=%v err=%v", a.Path(), err)
	}
	a, err = New(Options{Runner: &fakeRunner{}, LookPath: func(name string) (string, error) { return explicit, nil }})
	if err != nil || a.Path() != explicit {
		t.Fatalf("lookpath=%v err=%v", a.Path(), err)
	}
}

func TestContextCancellationDoesNotSpawn(t *testing.T) {
	a, runner := adapterWith(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.Usage(ctx, backend.UsageRequest{ProviderID: "codex"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatal("cancelled context still spawned backend")
	}
}

func TestDiagnosticsAreBoundedAndRedacted(t *testing.T) {
	process := &ProcessError{Code: 1, Stderr: []byte("Authorization: Bearer super-secret-token\nCookie: first=one; second=two\ncontact person@example.com\napi_key=abc123\npath /home/person/project\n\x1b[31mred\x1b[0m\n" + strings.Repeat("x", 20<<10))}
	diagnostic := process.Diagnostic()
	for _, secret := range []string{"super-secret-token", "person@example.com", "abc123", "second=two", "/home/person/project", "\x1b[31m"} {
		if strings.Contains(diagnostic, secret) {
			t.Fatalf("diagnostic leaked %q: %s", secret, diagnostic)
		}
	}
	if len(diagnostic) > 4096 {
		t.Fatalf("diagnostic cap not bounded: %d", len(diagnostic))
	}
}

func TestSyntheticFixturesAreValidJSON(t *testing.T) {
	for _, name := range []string{"catalog.json", "usage_windows.json", "usage_accounts.json", "history.json", "history_array.json", "usage_monetary.json", "history_baseline.json", "usage_baseline.json", "error_exit0.json", "partial_success.json", "mixed_results.json"} {
		var value any
		if err := json.Unmarshal(fixture(t, name), &value); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	_ = time.Now
	_ = errors.New
}

func TestCatalogAcceptsBareProviderList(t *testing.T) {
	// CodexBar 0.78.0 answers `config providers --format json` with a bare
	// JSON array rather than a {"providers": [...]} envelope.
	body := []byte(`[{"provider":"codex","enabled":true,"defaultEnabled":false,"displayName":"Codex"},{"provider":"gemini","enabled":false,"defaultEnabled":true,"displayName":"Gemini"}]`)
	a, _ := adapterWith(body)
	catalog, err := a.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Providers) != 2 {
		t.Fatalf("providers=%d", len(catalog.Providers))
	}
	codex := catalog.Providers[0]
	if codex.ProviderID != "codex" || !codex.Enabled || codex.DefaultEnabled || !codex.Available {
		t.Fatalf("codex=%+v", codex)
	}
	gemini := catalog.Providers[1]
	if gemini.Enabled || !gemini.DefaultEnabled || !gemini.Available {
		t.Fatalf("gemini=%+v", gemini)
	}
}

func TestCatalogBareEmptyProviderListIsUnsupported(t *testing.T) {
	a, _ := adapterWith([]byte(`[]`))
	if _, err := a.Catalog(context.Background()); categoryOf(t, err) != domain.ErrUnsupported {
		t.Fatalf("category=%v err=%v", categoryOf(t, err), err)
	}
}

func TestCatalogKeepsEnabledAndDefaultEnabledSeparate(t *testing.T) {
	a, _ := adapterWith(fixture(t, "catalog.json"))
	catalog, err := a.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	gemini := catalog.Providers[1]
	if gemini.Enabled || !gemini.DefaultEnabled {
		t.Fatalf("gemini flags enabled=%v default=%v", gemini.Enabled, gemini.DefaultEnabled)
	}
	if !gemini.Available {
		t.Fatal("catalog membership should imply availability")
	}
}

func TestUsageMissingIdentityBecomesUnknownSentinel(t *testing.T) {
	a, _ := adapterWith([]byte(`{"provider":"codex","primary":{"label":"Session","usedPercent":10}}`))
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex", SourceKey: "codex@old-account"})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshots[0].Key.AccountUnknown || snapshots[0].Key.Account != domain.UnknownAccount || snapshots[0].Key.String() != "codex@unknown" {
		t.Fatalf("identity inherited request: %s", snapshots[0].Key)
	}
}

func TestUsageNestedArrayEnvelopeIsFlattened(t *testing.T) {
	body := []byte(`[{"results":[{"provider":"codex","accountKey":"one","primary":{"label":"Session","usedPercent":10}}]}]`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil || len(snapshots) != 1 || snapshots[0].Key.Account != "one" {
		t.Fatalf("snapshots=%+v err=%v", snapshots, err)
	}
}

func TestMixedResultsRetainSuccessAndScopedFailure(t *testing.T) {
	a, _ := adapterWith(fixture(t, "mixed_results.json"))
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshots=%d", len(snapshots))
	}
	if snapshots[0].Error != nil || snapshots[0].Key.Account != "healthy" {
		t.Fatalf("success row=%+v", snapshots[0])
	}
	if snapshots[1].Error == nil || snapshots[1].Error.Category != domain.ErrAuth || snapshots[1].Key.Account != "expired" {
		t.Fatalf("failure row=%+v", snapshots[1])
	}
}

func TestHistoryArrayEnvelopeEqualsObjectShape(t *testing.T) {
	a, _ := adapterWith(fixture(t, "history_array.json"))
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Buckets) != 3 || history.Buckets[0].Day != "2026-10-14" || !history.Partial {
		t.Fatalf("history=%+v", history)
	}
}

func TestHistoryRepeatedModelsAccumulate(t *testing.T) {
	body := []byte(`{"days":[
		{"date":"2026-10-14","tokens":30,"models":[{"name":"m","input":10,"output":5}]},
		{"date":"2026-10-15","tokens":60,"models":[{"name":"m","input":20,"output":10}]}
	]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Models) != 1 || history.Models[0].Input == nil || *history.Models[0].Input != 30 || *history.Models[0].Output != 15 {
		t.Fatalf("models=%+v", history.Models)
	}
}

func TestHistoryStructuredAuthErrorPreserved(t *testing.T) {
	a, _ := adapterWith(fixture(t, "error_exit0.json"))
	_, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", Days: 7})
	if err == nil || categoryOf(t, err) != domain.ErrAuth {
		t.Fatalf("err=%v", err)
	}
}

func TestFatalRunnerErrorsAreNotMaskedByValidJSON(t *testing.T) {
	for _, fatal := range []error{ErrTimeout, ErrOutputCap} {
		runner := &fakeRunner{fn: func(context.Context, string, []string, int) ([]byte, []byte, error) {
			return fixture(t, "partial_success.json"), nil, fatal
		}}
		a := &Adapter{path: "/fixture/codexbar", runner: runner}
		_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
		if err == nil {
			t.Fatalf("%v was masked as success", fatal)
		}
		if categoryOf(t, err) != domain.ErrTimeout && categoryOf(t, err) != domain.ErrOutputCap {
			t.Fatalf("%v mapped to %v", fatal, err)
		}
	}
}

func TestReceiptTimestampUsesInjectedClock(t *testing.T) {
	fixed := time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
	a, _ := adapterWith([]byte(`{"provider":"codex","primary":{"label":"Session","usedPercent":10}}`))
	a.now = func() time.Time { return fixed }
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshots[0].ReceivedAt.Equal(fixed) || snapshots[0].SampleAt != nil {
		t.Fatalf("received=%v sample=%v", snapshots[0].ReceivedAt, snapshots[0].SampleAt)
	}
}

func TestSemanticWindowLabelsAndExtraWindows(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"usedPercent":10},"secondary":{"usedPercent":20},"extraWindows":[{"key":"model-x","label":"Model X","usedPercent":30}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	windows := snapshots[0].Windows
	if len(windows) != 3 || windows[0].Label != "Primary" || windows[1].Label != "Secondary" || windows[2].Label != "Model X" {
		t.Fatalf("windows=%+v", windows)
	}
}

func TestUnsupportedDetailShapeIsDisclosed(t *testing.T) {
	body := []byte(`{"provider":"codex","details":[{"title":"Mystery","opaque":{"new":true}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots[0].Details) != 1 || !snapshots[0].Details[0].Unsupported {
		t.Fatalf("details=%+v", snapshots[0].Details)
	}
}

func TestProviderSpecificVariantAndConsistencyValidation(t *testing.T) {
	a, runner := adapterWith([]byte(`{"provider":"gemini","primary":{"label":"Web","usedPercent":1}}`))
	if _, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "gemini", Variant: domain.VariantWeb, SourceKey: "gemini:web"}); err != nil {
		t.Fatalf("gemini web rejected: %v", err)
	}
	if _, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex", Variant: domain.VariantWeb}); err == nil || categoryOf(t, err) != domain.ErrUnsupported {
		t.Fatalf("codex web err=%v", err)
	}
	if _, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex", SourceKey: "claude@x"}); err == nil || categoryOf(t, err) != domain.ErrUnsupported {
		t.Fatalf("mismatched key err=%v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("invalid requests spawned backend: %d", len(runner.calls))
	}
}

func TestNativeOnlyFallbackDoesNotRetryFatalErrors(t *testing.T) {
	runner := &fakeRunner{fn: func(context.Context, string, []string, int) ([]byte, []byte, error) {
		return nil, []byte("sign in required"), &ProcessError{Code: 1}
	}}
	a := &Adapter{path: "/fixture/codexbar", runner: runner, nativeOnly: true}
	_, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", Days: 7})
	if err == nil || categoryOf(t, err) != domain.ErrAuth {
		t.Fatalf("err=%v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("fatal error triggered fallback: %d calls", len(runner.calls))
	}
}

func TestTrailingJSONIsRejected(t *testing.T) {
	a, _ := adapterWith([]byte(`{"provider":"codex","primary":{"label":"Session","usedPercent":10}} {"extra":true}`))
	_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || categoryOf(t, err) != domain.ErrMalformedJSON {
		t.Fatalf("err=%v", err)
	}
}

func TestAdapterAppliesOperationDeadlines(t *testing.T) {
	runner := &fakeRunner{fn: func(ctx context.Context, _ string, _ []string, _ int) ([]byte, []byte, error) {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}}
	a := &Adapter{path: "/fixture/codexbar", runner: runner}
	start := time.Now()
	_, err := a.run(context.Background(), 30*time.Millisecond, 1024, "codex", "usage")
	if err == nil || categoryOf(t, err) != domain.ErrTimeout {
		t.Fatalf("err=%v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("deadline was not applied")
	}
}

func TestReportedRateWindowLabelsAreHonored(t *testing.T) {
	body := []byte(`{"provider":"codex","rateWindowLabels":{"primary":"Five hour","secondary":"Weekly"},"primary":{"usedPercent":10},"secondary":{"usedPercent":20}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshots[0].Windows[0].Label != "Five hour" || snapshots[0].Windows[1].Label != "Weekly" {
		t.Fatalf("windows=%+v", snapshots[0].Windows)
	}
}

func TestUnknownStructuredFailureIsNotCurrent(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"error":{"code":"rate_limit","message":"Try later"}}`)
	a, _ := adapterWith(body)
	_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || categoryOf(t, err) != domain.ErrUnknown {
		t.Fatalf("err=%v", err)
	}
}

func TestObjectResultsEnvelopeUsesRecursiveUsageNormalization(t *testing.T) {
	body := []byte(`{"results":[{"provider":"codex","accountKey":"nested","usage":{"primary":{"usedPercent":10}}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil || len(snapshots) != 1 || snapshots[0].Key.Account != "nested" || len(snapshots[0].Windows) != 1 {
		t.Fatalf("snapshots=%+v err=%v", snapshots, err)
	}
}

func TestHistoryArrayProcessesEveryEnvelopeAndRejectsErrors(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":10}]},
		{"days":[{"date":"2026-10-15","tokens":20}]}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil || len(history.Buckets) != 2 {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	a, _ = adapterWith([]byte(`[{"error":{"code":"auth","message":"expired"}}]`))
	_, err = a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err == nil || categoryOf(t, err) != domain.ErrAuth {
		t.Fatalf("array error err=%v", err)
	}
}

func TestPayloadReceivedAtCannotOverwriteLocalClock(t *testing.T) {
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	body := []byte(`{"provider":"codex","receivedAt":"` + future + `","primary":{"label":"Plan","usedPercent":10}}`)
	a, _ := adapterWith(body)
	fixed := time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return fixed }
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshots[0].ReceivedAt.Equal(fixed) {
		t.Fatalf("received=%v", snapshots[0].ReceivedAt)
	}
}

func TestStructuredErrorCollectionsAreRejected(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"errors":[{"code":"auth","message":"expired"}]}`)
	a, _ := adapterWith(body)
	_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || categoryOf(t, err) != domain.ErrAuth {
		t.Fatalf("err=%v", err)
	}
}

func TestAccountExpansionNormalizesNestedUsage(t *testing.T) {
	body := []byte(`{"provider":"muse","accounts":[{"accountKey":"one","usage":{"primary":{"usedPercent":10}}},{"accountKey":"two","usage":{"primary":{"usedPercent":20}}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "muse"})
	if err != nil || len(snapshots) != 2 || len(snapshots[0].Windows) != 1 || len(snapshots[1].Windows) != 1 {
		t.Fatalf("snapshots=%+v err=%v", snapshots, err)
	}
}

func TestWindowKeysAreStableAndUnique(t *testing.T) {
	body := []byte(`{"provider":"codex","windows":[{"label":"Five hour","usedPercent":10},{"label":"Five hour","usedPercent":20},{"label":"Weekly","usedPercent":30}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, window := range snapshots[0].Windows {
		if window.Key == "" || seen[window.Key] {
			t.Fatalf("unstable/duplicate key %q in %+v", window.Key, snapshots[0].Windows)
		}
		seen[window.Key] = true
	}
}

func TestNullHistoryMeasurementsAreIncomplete(t *testing.T) {
	body := []byte(`{"days":[{"date":"2026-10-14","tokens":0},{"date":"2026-10-15","tokens":null}]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !history.Partial || history.Incomplete != 1 || len(history.MissingDays) != 1 || history.Buckets[0].Tokens == nil || *history.Buckets[0].Tokens != 0 {
		t.Fatalf("history=%+v", history)
	}
}

func TestHistorySiblingErrorRetainsValidRecords(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":10}]},
		{"error":{"code":"auth","message":"expired"}}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err == nil || categoryOf(t, err) != domain.ErrAuth {
		t.Fatalf("err=%v", err)
	}
	if len(history.Buckets) != 1 || history.TotalTokens == nil || *history.TotalTokens != 10 {
		t.Fatalf("valid history discarded: %+v", history)
	}
}

func TestHistoryEnvelopeSummariesReconcile(t *testing.T) {
	body := []byte(`[
		{"accountKey":"one","days":[{"date":"2026-10-14","tokens":10}],"totalTokens":10,"cost":{"amount":1,"currency":"USD"},"coverage":"one"},
		{"accountKey":"two","partial":true,"days":[{"date":"2026-10-15","tokens":20}],"totalTokens":20,"cost":{"amount":2,"currency":"USD"},"coverage":"two"}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.TotalTokens == nil || *history.TotalTokens != 30 || history.Cost == nil || history.Cost.Amount != 3 {
		t.Fatalf("totals/cost=%+v %+v", history.TotalTokens, history.Cost)
	}
	if !history.Partial || !strings.Contains(history.Coverage, "one") || !strings.Contains(history.Coverage, "two") {
		t.Fatalf("summary metadata=%+v", history)
	}
	if !history.Key.AccountUnknown || history.Key.Account != domain.UnknownAccount {
		t.Fatalf("multi-account scope collapsed to %s", history.Key)
	}
}

func TestJSONDiagnosticSecretsAreRedacted(t *testing.T) {
	process := &ProcessError{Code: 1, Stderr: []byte(`{"token":"SYNTHETIC-TOKEN","api_key":"SYNTHETIC-KEY","message":"contact person@example.com"}`)}
	diagnostic := process.Diagnostic()
	for _, secret := range []string{"SYNTHETIC-TOKEN", "SYNTHETIC-KEY", "person@example.com"} {
		if strings.Contains(diagnostic, secret) {
			t.Fatalf("diagnostic leaked %q: %s", secret, diagnostic)
		}
	}
}

func TestHistoryDailyRecordsDoNotInheritRequestAccount(t *testing.T) {
	body := []byte(`{"days":[{"date":"2026-10-14","tokens":10}]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", SourceKey: "muse@selected-account", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !history.Key.AccountUnknown || history.Key.Account != domain.UnknownAccount || history.Key.String() != "muse@unknown" {
		t.Fatalf("history inherited request account: %s", history.Key)
	}
}

func TestStringErrorCollectionsAreRejected(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"errors":["authentication expired"]}`)
	a, _ := adapterWith(body)
	_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || categoryOf(t, err) != domain.ErrAuth {
		t.Fatalf("err=%v", err)
	}
}

func TestHistoryModelsAreNotDoubleCountedAcrossSummaryAndDetail(t *testing.T) {
	body := []byte(`{"days":[{"date":"2026-10-14","tokens":10,"models":[{"name":"m","input":10,"output":5}]}],"models":[{"name":"m","input":10,"output":5}],"totalTokens":10}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Models) != 1 || *history.Models[0].Input != 10 || *history.Models[0].Output != 5 {
		t.Fatalf("models double counted: %+v", history.Models)
	}
}

func TestHistoryTotalsReconcileEnvelopeSummariesWithDailyDetail(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":10}],"totalTokens":10},
		{"days":[{"date":"2026-10-15","tokens":20}]}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.TotalTokens == nil || *history.TotalTokens != 30 {
		t.Fatalf("total=%v", history.TotalTokens)
	}
}

func TestReportedIncompleteWithTokensStaysPartial(t *testing.T) {
	body := []byte(`{"days":[{"date":"2026-10-14","tokens":10,"incomplete":true}]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !history.Partial || history.Incomplete != 1 || history.Buckets[0].Tokens == nil {
		t.Fatalf("history=%+v", history)
	}
}

func TestObjectResultsMixedWithErrorsRetainsSuccesses(t *testing.T) {
	body := []byte(`{"results":[{"provider":"codex","accountKey":"ok","plan":"Pro","primary":{"label":"Plan","usedPercent":10}}],"errors":[{"code":"auth","message":"expired"}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshots=%+v", snapshots)
	}
	if snapshots[0].Error != nil || snapshots[1].Error == nil {
		t.Fatalf("mixed error retention=%+v", snapshots)
	}
}

func TestWindowKeyGenerationAvoidsExplicitCollisions(t *testing.T) {
	body := []byte(`{"provider":"codex","windows":[{"key":"x","label":"A","usedPercent":1},{"key":"x","label":"B","usedPercent":2},{"key":"x-2","label":"C","usedPercent":3}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, window := range snapshots[0].Windows {
		if window.Key == "" || seen[window.Key] {
			t.Fatalf("duplicate key %q in %+v", window.Key, snapshots[0].Windows)
		}
		seen[window.Key] = true
	}
	if !seen["x-2"] {
		t.Fatalf("explicit x-2 key was displaced: %+v", snapshots[0].Windows)
	}
}

func TestJSONAccessAndRefreshTokensAreRedacted(t *testing.T) {
	process := &ProcessError{Code: 1, Stderr: []byte(`{"access_token":"A-SECRET\"-TAIL","refresh_token":"R-SECRET"}`)}
	diagnostic := process.Diagnostic()
	for _, secret := range []string{"A-SECRET", "-TAIL", "R-SECRET"} {
		if strings.Contains(diagnostic, secret) {
			t.Fatalf("diagnostic leaked %q: %s", secret, diagnostic)
		}
	}
}

func TestIncompatibleHistoryCurrenciesAreNotSummed(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":10}],"cost":{"amount":1,"currency":"USD"}},
		{"days":[{"date":"2026-10-15","tokens":20}],"cost":{"amount":2,"currency":"EUR"}}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.Cost != nil || !strings.Contains(history.Coverage, "incompatible currencies") {
		t.Fatalf("cost=%+v coverage=%q", history.Cost, history.Coverage)
	}
}

func TestInvalidTokenCountsAreNotMeasured(t *testing.T) {
	for _, value := range []string{"-1", "1.5", "1e30"} {
		body := []byte(`{"days":[{"date":"2026-10-14","tokens":` + value + `}]}`)
		a, _ := adapterWith(body)
		history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
		if err != nil {
			t.Fatal(err)
		}
		if history.Buckets[0].Tokens != nil || history.TotalTokens != nil || !history.Partial {
			t.Fatalf("value %s became measured: %+v", value, history)
		}
	}
}

func TestHistoryEnvelopeErrorRetainsValidDays(t *testing.T) {
	body := []byte(`[{"days":[{"date":"2026-10-14","tokens":10}],"error":{"code":"auth","message":"expired"}}]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err == nil || categoryOf(t, err) != domain.ErrAuth {
		t.Fatalf("err=%v", err)
	}
	if len(history.Buckets) != 1 || !history.Partial || history.TotalTokens == nil || *history.TotalTokens != 10 {
		t.Fatalf("valid days discarded: %+v", history)
	}
}

func TestAllFailedUsageAccountsRetainScopedSnapshots(t *testing.T) {
	body := []byte(`[
		{"accountKey":"one","error":{"code":"auth","message":"expired"}},
		{"accountKey":"two","error":{"code":"unavailable","message":"no subscription"}}
	]`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 2 {
		t.Fatalf("snapshots=%d err=%v", len(snapshots), err)
	}
	if snapshots[0].Error == nil || snapshots[0].Error.Category != domain.ErrAuth || snapshots[1].Error == nil || snapshots[1].Error.Category != domain.ErrUnavailable {
		t.Fatalf("scoped failures=%+v", snapshots)
	}
}

func TestWindowKeyReservationWorksInBothOrders(t *testing.T) {
	bodies := [][]byte{
		[]byte(`{"provider":"codex","windows":[{"label":"x","usedPercent":1},{"key":"x","label":"Explicit","usedPercent":2}]}`),
		[]byte(`{"provider":"codex","windows":[{"key":"x","label":"Explicit","usedPercent":1},{"label":"x","usedPercent":2}]}`),
	}
	for _, body := range bodies {
		a, _ := adapterWith(body)
		snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, window := range snapshots[0].Windows {
			if window.Key == "" || seen[window.Key] {
				t.Fatalf("duplicate key %q in %+v", window.Key, snapshots[0].Windows)
			}
			seen[window.Key] = true
		}
		if !seen["x"] {
			t.Fatalf("explicit key x missing: %+v", snapshots[0].Windows)
		}
	}
}

func TestUnsupportedSubstructureDisclosedAlongsideSupportedRows(t *testing.T) {
	body := []byte(`{"provider":"codex","details":[{"title":"Mixed","rows":[{"label":"A","value":"B"}],"chart":{"new":"shape"}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	section := snapshots[0].Details[0]
	if len(section.Rows) != 1 || !section.Unsupported {
		t.Fatalf("section=%+v", section)
	}
}

func TestUsageEnvelopeErrorRetainsAccountsAndWindows(t *testing.T) {
	body := []byte(`{"provider":"muse","accounts":[{"accountKey":"one","primary":{"label":"Plan","usedPercent":10}},{"accountKey":"two","primary":{"label":"Plan","usedPercent":20}}],"error":{"code":"auth","message":"partial login"}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "muse"})
	if err == nil {
		t.Fatal("envelope error missing")
	}
	if len(snapshots) != 2 || len(snapshots[0].Windows) != 1 || snapshots[0].Error == nil {
		t.Fatalf("snapshots=%+v", snapshots)
	}
}

func TestUsageWindowAlongsideErrorRetainsMeasurement(t *testing.T) {
	body := []byte(`[{"provider":"codex","accountKey":"one","primary":{"label":"Plan","usedPercent":10},"error":{"code":"auth","message":"expired"}}]`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 1 || len(snapshots[0].Windows) != 1 || snapshots[0].Error == nil {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
}

func TestHistoryIncompatibleSiblingRetainsValidGroup(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":10}]},
		{"unfamiliar":{"shape":true}}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err == nil {
		t.Fatal("incompatible sibling not disclosed")
	}
	if len(history.Buckets) != 1 || history.TotalTokens == nil || *history.TotalTokens != 10 || !history.Partial {
		t.Fatalf("valid history discarded: %+v", history)
	}
}

func TestMissingCurrencyIsNotInventedAndPricingCoverageDisclosed(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":10}],"cost":{"amount":1}},
		{"days":[{"date":"2026-10-15","tokens":20}]}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.Cost != nil && history.Cost.Currency == "USD" {
		t.Fatalf("currency invented: %+v", history.Cost)
	}
	if !history.Partial || !strings.Contains(history.Coverage, "pricing incomplete") || !strings.Contains(history.Coverage, "unknown currency") {
		t.Fatalf("pricing coverage not disclosed: %+v", history)
	}
}

func TestInt64OverflowTokenRejected(t *testing.T) {
	body := []byte(`{"days":[{"date":"2026-10-14","tokens":9223372036854775808}]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.TotalTokens != nil || history.Buckets[0].Tokens != nil || !history.Partial {
		t.Fatalf("overflow became measurement: %+v", history)
	}
}

func TestInvalidHistoryBucketDateIsRejectedWithoutFabrication(t *testing.T) {
	body := []byte(`{"days":[{"unknown":10}]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err == nil || categoryOf(t, err) != domain.ErrUnsupported {
		t.Fatalf("err=%v history=%+v", err, history)
	}
	if !history.Start.IsZero() || !history.End.IsZero() {
		t.Fatal("history fabricated a date range")
	}
}

func TestNativeOnlyFallbackRetainsValidPartialAndFailure(t *testing.T) {
	calls := 0
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args []string, _ int) ([]byte, []byte, error) {
		calls++
		if len(args) > 0 && args[len(args)-1] == "--provider-native-only" {
			return nil, []byte("unknown option"), &ProcessError{Code: 2}
		}
		return fixture(t, "history.json"), []byte("warning"), &ProcessError{Code: 1}
	}}
	a := &Adapter{path: "/fixture/codexbar", runner: runner, nativeOnly: true}
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", Days: 7})
	if calls != 2 || err == nil || len(history.Buckets) != 3 {
		t.Fatalf("calls=%d err=%v buckets=%d", calls, err, len(history.Buckets))
	}
	if !strings.Contains(history.Provenance, "unavailable") {
		t.Fatalf("provenance=%q", history.Provenance)
	}
}

func TestChartPointsAreNormalized(t *testing.T) {
	body := []byte(`{"provider":"codex","details":[{"title":"Chart","chartPoints":[{"label":"Mon","value":3}]}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	section := snapshots[0].Details[0]
	if len(section.Points) != 1 || section.Unsupported || section.Points[0].Value == nil || *section.Points[0].Value != 3 {
		t.Fatalf("section=%+v", section)
	}
}

func TestSingleObjectErrorWithMeasurementRetainsReading(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"error":{"code":"auth","message":"expired"}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 1 || len(snapshots[0].Windows) != 1 || snapshots[0].Error == nil {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
}

func TestIncompatibleUsageSiblingIsDisclosed(t *testing.T) {
	body := []byte(`[
		{"provider":"codex","accountKey":"healthy","primary":{"label":"Plan","usedPercent":10}},
		{"provider":"codex","accountKey":"mystery","unfamiliar":{"shape":true}}
	]`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 2 {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
	if snapshots[0].Error != nil || snapshots[1].Error == nil || snapshots[1].Error.Category != domain.ErrUnsupported {
		t.Fatalf("sibling disclosure=%+v", snapshots)
	}
}

func TestUnknownCurrencyIsExcludedFromKnownTotal(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":10}],"cost":{"amount":1}},
		{"days":[{"date":"2026-10-15","tokens":20}],"cost":{"amount":2,"currency":"USD"}}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.Cost == nil || history.Cost.Currency != "USD" || history.Cost.Amount != 2 {
		t.Fatalf("unknown currency was merged: %+v", history.Cost)
	}
	if !strings.Contains(history.Coverage, "unknown currency") {
		t.Fatalf("unknown currency not disclosed: %q", history.Coverage)
	}
}

func TestAggregateTokenOverflowBecomesUnavailablePartial(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":9223372036854775807}],"totalTokens":9223372036854775807},
		{"days":[{"date":"2026-10-15","tokens":1}],"totalTokens":1}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.TotalTokens != nil || !history.Partial || !strings.Contains(history.Coverage, "overflow") {
		t.Fatalf("overflow presented as complete: %+v", history)
	}
}

func TestFallbackFatalErrorTakesPrecedenceOverPartialAuth(t *testing.T) {
	calls := 0
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args []string, _ int) ([]byte, []byte, error) {
		calls++
		if len(args) > 0 && args[len(args)-1] == "--provider-native-only" {
			return nil, []byte("unknown option"), &ProcessError{Code: 2}
		}
		return fixture(t, "history.json"), []byte(`{"error":{"code":"auth","message":"expired"}}`), ErrOutputCap
	}}
	a := &Adapter{path: "/fixture/codexbar", runner: runner, nativeOnly: true}
	_, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", Days: 7})
	if calls != 2 || err == nil || categoryOf(t, err) != domain.ErrOutputCap {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestCamelCaseAndCookieDiagnosticsRedacted(t *testing.T) {
	process := &ProcessError{Code: 1, Stderr: []byte(`accessToken=CAMEL-SECRET refreshToken: REFRESH-SECRET cookie=SESSION-SECRET`)}
	diagnostic := process.Diagnostic()
	for _, secret := range []string{"CAMEL-SECRET", "REFRESH-SECRET", "SESSION-SECRET"} {
		if strings.Contains(diagnostic, secret) {
			t.Fatalf("diagnostic leaked %q: %s", secret, diagnostic)
		}
	}
}

func TestUnfamiliarStructuredErrorObjectIsUnknownFailure(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"error":{"status":401}}`)
	a, _ := adapterWith(body)
	_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || categoryOf(t, err) != domain.ErrUnknown {
		t.Fatalf("err=%v", err)
	}
}

func TestNestedUsageWithEnvelopeErrorRetainsMeasurement(t *testing.T) {
	body := []byte(`{"usage":{"primary":{"usedPercent":10}},"error":{"code":"auth"}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 1 || len(snapshots[0].Windows) != 1 || snapshots[0].Error == nil {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
}

func TestModelOverflowAndNullCompleteness(t *testing.T) {
	body := []byte(`{"days":[
		{"date":"2026-10-14","tokens":10,"models":[{"name":"m","input":9223372036854775807}]},
		{"date":"2026-10-15","tokens":20,"models":[{"name":"m","input":1}]},
		{"date":"2026-10-16","tokens":30,"models":[{"name":"m","input":5}]}
	]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Models) != 1 || history.Models[0].Input != nil || !history.Partial {
		t.Fatalf("model overflow=%+v partial=%v", history.Models, history.Partial)
	}
	body = []byte(`{"days":[
		{"date":"2026-10-14","tokens":10,"models":[{"name":"m","input":10}]},
		{"date":"2026-10-15","tokens":20,"models":[{"name":"m","input":null}]}
	]}`)
	a, _ = adapterWith(body)
	history, err = a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Models) != 1 || history.Models[0].Input == nil || *history.Models[0].Input != 10 || !history.Partial {
		t.Fatalf("model null completeness=%+v partial=%v", history.Models, history.Partial)
	}
}

func TestNonObjectUsageArrayMemberIsDisclosed(t *testing.T) {
	body := []byte(`[{"provider":"codex","accountKey":"ok","primary":{"label":"Plan","usedPercent":10}},42]`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 2 || snapshots[1].Error == nil || snapshots[1].Error.Category != domain.ErrUnsupported {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
}

func TestUnsupportedBarsObjectDisclosedAlongsideRows(t *testing.T) {
	body := []byte(`{"provider":"codex","details":[{"title":"Mixed","rows":[{"label":"A","value":"B"}],"bars":{"new":"shape"}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshots[0].Details[0].Unsupported || len(snapshots[0].Details[0].Rows) != 1 {
		t.Fatalf("section=%+v", snapshots[0].Details[0])
	}
}

func TestGroupedWindowKeysAreStableAcrossOrder(t *testing.T) {
	first := []byte(`{"provider":"codex","windows":[{"group":"A","label":"Daily","usedPercent":1},{"group":"B","label":"Daily","usedPercent":2}]}`)
	second := []byte(`{"provider":"codex","windows":[{"group":"B","label":"Daily","usedPercent":2},{"group":"A","label":"Daily","usedPercent":1}]}`)
	keys := func(body []byte) map[string]bool {
		a, _ := adapterWith(body)
		snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
		if err != nil {
			t.Fatal(err)
		}
		result := map[string]bool{}
		for _, window := range snapshots[0].Windows {
			result[window.Key] = true
		}
		return result
	}
	a, b := keys(first), keys(second)
	if len(a) != len(b) {
		t.Fatalf("key sets differ: %v vs %v", a, b)
	}
	for key := range a {
		if !b[key] {
			t.Fatalf("key %q not stable across order: %v vs %v", key, a, b)
		}
	}
}

func TestUnknownErrorObjectCollectionIsUnknownFailure(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"errors":[{"reason":"provider failed"}]}`)
	a, _ := adapterWith(body)
	_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || categoryOf(t, err) != domain.ErrUnknown {
		t.Fatalf("err=%v", err)
	}
}

func TestMixedReportedAndEstimatedCostProvenanceIsNotAggregated(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":10}],"cost":{"amount":1,"currency":"USD","reported":true}},
		{"days":[{"date":"2026-10-15","tokens":20}],"cost":{"amount":2,"currency":"USD"}}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.Cost != nil || history.CostReported || !strings.Contains(history.Coverage, "mixed reported and estimated") {
		t.Fatalf("cost=%+v reported=%v coverage=%q", history.Cost, history.CostReported, history.Coverage)
	}
}

func TestDeclaredNullWindowPreservedAsUnavailable(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"usedPercent":10},"secondary":null}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	windows := snapshots[0].Windows
	if len(windows) != 2 || windows[1].Quality != domain.QualityUnavailable || windows[1].UsedPercent != nil {
		t.Fatalf("windows=%+v", windows)
	}
	if snapshots[0].State != domain.StatusPartial {
		t.Fatalf("state=%v", snapshots[0].State)
	}
}

func TestUnlabeledNullWindowsAreSchemaNoise(t *testing.T) {
	// CodexBar 0.78.0 emits fixed primary/secondary/tertiary slots, null for
	// windows the provider does not have, and lists the real ones in
	// rateWindowLabels. A null slot the backend does not label is not a
	// missing allowance; it must not become a phantom unavailable window.
	body := []byte(`{"provider":"codex","rateWindowLabels":{"secondary":"Weekly"},"primary":null,"secondary":{"usedPercent":3},"tertiary":null}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	windows := snapshots[0].Windows
	if len(windows) != 1 || windows[0].Label != "Weekly" {
		t.Fatalf("windows=%+v", windows)
	}
	if snapshots[0].State != domain.StatusCurrent {
		t.Fatalf("state=%v", snapshots[0].State)
	}
}

func TestLabeledNullWindowPreservedAsUnavailable(t *testing.T) {
	// A null slot the backend does label is a declared missing allowance for
	// a window that exists, so it keeps the unavailable placeholder.
	body := []byte(`{"provider":"muse","rateWindowLabels":{"primary":"5 hours","secondary":"Weekly"},"primary":null,"secondary":{"usedPercent":44},"tertiary":null}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "muse"})
	if err != nil {
		t.Fatal(err)
	}
	windows := snapshots[0].Windows
	if len(windows) != 2 || windows[0].Label != "5 hours" || windows[0].Quality != domain.QualityUnavailable || windows[0].UsedPercent != nil {
		t.Fatalf("windows=%+v", windows)
	}
	if snapshots[0].State != domain.StatusPartial {
		t.Fatalf("state=%v", snapshots[0].State)
	}
}

func TestPlanPromotedFromDetailsRows(t *testing.T) {
	// CodexBar 0.78.0 reports Muse's plan only as a details row, not as a
	// plan field; the snapshot plan should still surface it.
	body := []byte(`{"provider":"muse","primary":{"usedPercent":1},"details":[{"title":"Muse Code subscription","rows":[{"label":"Plan","value":"Muse Code Max Usage"}]}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "muse"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshots[0].Plan != "Muse Code Max Usage" {
		t.Fatalf("plan=%q", snapshots[0].Plan)
	}
	if len(snapshots[0].Details) != 1 {
		t.Fatalf("details=%+v", snapshots[0].Details)
	}
}

func TestUnsupportedDetailContainersAndMembersDisclosed(t *testing.T) {
	body := []byte(`{"provider":"codex","details":[42,{"title":"Mixed","rows":[{"label":"A","value":"B"}],"chart":[42]}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	sections := snapshots[0].Details
	if len(sections) != 2 || !sections[0].Unsupported || !sections[1].Unsupported || len(sections[1].Rows) != 1 {
		t.Fatalf("sections=%+v", sections)
	}
}

func TestIncompatibleHistoryMembersAreDisclosed(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`[{"days":[{"date":"2026-10-14","tokens":10}]},42]`),
		[]byte(`{"days":[{"date":"2026-10-14","tokens":10},42]}`),
	} {
		a, _ := adapterWith(body)
		history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
		if err == nil || len(history.Buckets) != 1 || !history.Partial {
			t.Fatalf("err=%v history=%+v", err, history)
		}
	}
}

func TestFallbackStructuredErrorWithNoBucketsPreserved(t *testing.T) {
	calls := 0
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args []string, _ int) ([]byte, []byte, error) {
		calls++
		if len(args) > 0 && args[len(args)-1] == "--provider-native-only" {
			return nil, []byte("unknown option"), &ProcessError{Code: 2}
		}
		return fixture(t, "error_exit0.json"), nil, nil
	}}
	a := &Adapter{path: "/fixture/codexbar", runner: runner, nativeOnly: true}
	_, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", Days: 7})
	if calls != 2 || err == nil || categoryOf(t, err) != domain.ErrAuth {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestCacheAgeBecomesSampleTime(t *testing.T) {
	body := []byte(`{"provider":"codex","cacheAgeSeconds":3600,"primary":{"label":"Plan","usedPercent":10}}`)
	a, _ := adapterWith(body)
	fixed := time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return fixed }
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshots[0].SampleAt == nil || !snapshots[0].SampleAt.Equal(fixed.Add(-time.Hour)) {
		t.Fatalf("sample=%v", snapshots[0].SampleAt)
	}
}

func TestUnsupportedUsageContainersAndMalformedWindowsDisclosed(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"secondary":42,"details":"new shape"}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := snapshots[0]
	if len(snapshot.Windows) != 2 || snapshot.Windows[1].Quality != domain.QualityUnsupported {
		t.Fatalf("windows=%+v", snapshot.Windows)
	}
	if len(snapshot.Details) == 0 || !snapshot.Details[0].Unsupported {
		t.Fatalf("details=%+v", snapshot.Details)
	}
}

func TestNonObjectAccountMemberIsDisclosed(t *testing.T) {
	body := []byte(`{"provider":"muse","accounts":[{"accountKey":"ok","primary":{"usedPercent":10}},42]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "muse"})
	if err == nil || len(snapshots) != 2 || snapshots[1].Error == nil || snapshots[1].Error.Category != domain.ErrUnsupported {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
}

func TestDuplicateDailyBucketsAcrossAccountsReconcile(t *testing.T) {
	body := []byte(`[
		{"accountKey":"one","days":[{"date":"2026-10-14","tokens":10}]},
		{"accountKey":"two","days":[{"date":"2026-10-14","tokens":20}]}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Buckets) != 1 || history.Buckets[0].Tokens == nil || *history.Buckets[0].Tokens != 30 {
		t.Fatalf("buckets=%+v", history.Buckets)
	}
	if history.TotalTokens == nil || *history.TotalTokens != 30 || history.Partial {
		t.Fatalf("total=%v partial=%v", history.TotalTokens, history.Partial)
	}
}

func TestProviderComponentsValidatedBeforeSpawn(t *testing.T) {
	a, runner := adapterWith([]byte(`{"provider":"codex","primary":{"usedPercent":1}}`))
	for _, request := range []backend.UsageRequest{
		{ProviderID: ""},
		{ProviderID: "bad@account"},
		{ProviderID: "bad:variant"},
		{ProviderID: "Bad Provider"},
	} {
		if _, err := a.Usage(context.Background(), request); err == nil || categoryOf(t, err) != domain.ErrUnsupported {
			t.Fatalf("request %+v err=%v", request, err)
		}
	}
	if len(runner.calls) != 0 {
		t.Fatalf("invalid provider spawned backend: %d", len(runner.calls))
	}
	if _, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "bad@account", Days: 7}); err == nil || categoryOf(t, err) != domain.ErrUnsupported {
		t.Fatalf("history invalid provider err=%v", err)
	}
}

func TestMonetaryOverflowBecomesUnavailablePartial(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":10}],"cost":{"amount":1e308,"currency":"USD"}},
		{"days":[{"date":"2026-10-15","tokens":20}],"cost":{"amount":1e308,"currency":"USD"}}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.Cost != nil || !history.Partial || !strings.Contains(history.Coverage, "monetary overflow") {
		t.Fatalf("cost=%+v partial=%v coverage=%q", history.Cost, history.Partial, history.Coverage)
	}
}

func TestMalformedMetricCollectionsDisclosed(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"windows":[42]}`),
		[]byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"windows":{"new":"shape"}}`),
		[]byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"extraWindows":[42]}`),
		[]byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"balances":[42]}`),
	} {
		a, _ := adapterWith(body)
		snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
		if err != nil {
			t.Fatal(err)
		}
		snapshot := snapshots[0]
		disclosed := len(snapshot.Details) > 0 && snapshot.Details[len(snapshot.Details)-1].Unsupported
		for _, window := range snapshot.Windows {
			if window.Quality == domain.QualityUnsupported {
				disclosed = true
			}
		}
		if len(snapshot.Windows) < 1 || !disclosed {
			t.Fatalf("valid measurement or disclosure missing: windows=%+v details=%+v", snapshot.Windows, snapshot.Details)
		}
	}
}

func TestScalarHistoryCostOverflowDisclosed(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":10}],"cost":1e308,"currency":"USD"},
		{"days":[{"date":"2026-10-15","tokens":20}],"cost":1e308,"currency":"USD"}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.Cost != nil || !history.Partial || !strings.Contains(history.Coverage, "monetary overflow") {
		t.Fatalf("cost=%+v partial=%v coverage=%q", history.Cost, history.Partial, history.Coverage)
	}
}

func TestProviderGrammarRejectsUnderscoreDotAndLeadingHyphen(t *testing.T) {
	a, runner := adapterWith(nil)
	for _, provider := range []string{"bad_name", "bad.name", "-bad"} {
		if _, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: provider}); err == nil || categoryOf(t, err) != domain.ErrUnsupported {
			t.Fatalf("%s err=%v", provider, err)
		}
	}
	if len(runner.calls) != 0 {
		t.Fatal("invalid provider reached backend")
	}
}

func TestArrayWrappedNestedHistoryEqualsObject(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"history":{"days":[{"date":"2026-10-14","tokens":10},{"date":"2026-10-15","tokens":20}]}}`),
		[]byte(`[{"history":{"days":[{"date":"2026-10-14","tokens":10},{"date":"2026-10-15","tokens":20}]}}]`),
	} {
		a, _ := adapterWith(body)
		history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
		if err != nil || len(history.Buckets) != 2 || history.TotalTokens == nil || *history.TotalTokens != 30 {
			t.Fatalf("history=%+v err=%v", history, err)
		}
	}
}

func TestCatalogStructuredErrorIsPreserved(t *testing.T) {
	body := []byte(`{"providers":[{"provider":"codex","enabled":true}],"error":{"code":"auth","message":"expired"}}`)
	a, _ := adapterWith(body)
	_, err := a.Catalog(context.Background())
	if err == nil || categoryOf(t, err) != domain.ErrAuth {
		t.Fatalf("err=%v", err)
	}
}

func TestNestedNullErrorDoesNotEraseEnvelopeError(t *testing.T) {
	body := []byte(`{"error":{"code":"auth"},"usage":{"error":null,"primary":{"usedPercent":10}}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 1 || snapshots[0].Error == nil || snapshots[0].Error.Category != domain.ErrAuth {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
}

func TestHistoryRejectsCrossProviderAggregation(t *testing.T) {
	body := []byte(`[
		{"provider":"codex","days":[{"date":"2026-10-14","tokens":10}]},
		{"provider":"claude","days":[{"date":"2026-10-15","tokens":20}]}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", Days: 7})
	if err == nil || len(history.Buckets) != 1 || history.TotalTokens == nil || *history.TotalTokens != 10 || !history.Partial {
		t.Fatalf("err=%v history=%+v", err, history)
	}
}

func TestReturnedVariantValidatedAgainstCanonicalScope(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"provider":"gemini","variant":"api:bad","primary":{"usedPercent":10}}`),
		[]byte(`{"provider":"gemini","variant":"api","primary":{"usedPercent":10}}`),
	} {
		a, _ := adapterWith(body)
		snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "gemini", Variant: domain.VariantWeb, SourceKey: "gemini:web"})
		if err == nil || len(snapshots) != 1 {
			t.Fatalf("err=%v snapshots=%+v", err, snapshots)
		}
		if snapshots[0].Key.String() != "gemini:web@unknown" || snapshots[0].Error == nil || snapshots[0].Error.Category != domain.ErrUnsupported {
			t.Fatalf("identity=%s error=%v", snapshots[0].Key, snapshots[0].Error)
		}
	}
}

func TestUnsupportedBalanceMapDisclosed(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"balances":{"new":"shape"}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := snapshots[0]
	if len(snapshot.Windows) != 1 || len(snapshot.Details) == 0 || !snapshot.Details[len(snapshot.Details)-1].Unsupported {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestNestedIdentitySeparatesAccounts(t *testing.T) {
	body := []byte(`[
		{"provider":"muse","usage":{"identity":{"accountID":"alpha"},"primary":{"usedPercent":10}}},
		{"provider":"muse","usage":{"identity":{"accountID":"beta"},"primary":{"usedPercent":20}}}
	]`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "muse"})
	if err != nil || len(snapshots) != 2 {
		t.Fatalf("snapshots=%d err=%v", len(snapshots), err)
	}
	if snapshots[0].Key.Account != "alpha" || snapshots[1].Key.Account != "beta" {
		t.Fatalf("accounts=%s,%s", snapshots[0].Key, snapshots[1].Key)
	}
}

func TestNestedNamedWindowAndUsageKnownFalse(t *testing.T) {
	body := []byte(`{"provider":"codex","windows":[{"label":"Nested","window":{"usedPercent":40}},{"label":"Unknown","usageKnown":false,"usedPercent":0}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	windows := snapshots[0].Windows
	if len(windows) != 2 || windows[0].UsedPercent == nil || *windows[0].UsedPercent != 40 {
		t.Fatalf("nested window=%+v", windows)
	}
	if windows[1].UsedPercent != nil || windows[1].RemainingPercent() != nil || windows[1].Quality != domain.QualityUnavailable {
		t.Fatalf("unknown usage=%+v", windows[1])
	}
}

func TestMismatchedVariantQuarantinesMeasurements(t *testing.T) {
	body := []byte(`{"provider":"gemini","variant":"api","primary":{"usedPercent":10}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "gemini", Variant: domain.VariantWeb, SourceKey: "gemini:web"})
	if err == nil || len(snapshots) != 1 || len(snapshots[0].Windows) != 0 || snapshots[0].Error == nil {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
	if snapshots[0].Key.String() != "gemini:web@unknown" {
		t.Fatalf("identity=%s", snapshots[0].Key)
	}
}

func TestNestedHistoryNullErrorDoesNotEraseOuterError(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"error":{"code":"auth"},"history":{"error":null,"days":[{"date":"2026-10-14","tokens":10}]}}`),
		[]byte(`[{"error":{"code":"auth"},"history":{"error":null,"days":[{"date":"2026-10-14","tokens":10}]}}]`),
	} {
		a, _ := adapterWith(body)
		history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
		if err == nil || categoryOf(t, err) != domain.ErrAuth || len(history.Buckets) != 1 || !history.Partial {
			t.Fatalf("err=%v history=%+v", err, history)
		}
	}
}

func TestMonetaryUsageNormalizedWithBillingMeaning(t *testing.T) {
	a, _ := adapterWith(fixture(t, "usage_monetary.json"))
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, section := range snapshots[0].Details {
		if section.Title == "Monetary usage" {
			found = true
			seen := map[string]string{}
			for _, row := range section.Rows {
				seen[row.Label] = row.Value
			}
			if seen["Used"] != "12.5" || seen["Limit"] != "100" || seen["Currency"] != "USD" || seen["Period"] != "monthly" {
				t.Fatalf("rows=%+v", section.Rows)
			}
		}
	}
	if !found {
		t.Fatalf("monetary section missing: %+v", snapshots[0].Details)
	}
}

func TestSingleObjectFailedAccountRetainsScope(t *testing.T) {
	body := []byte(`{"provider":"codex","accountKey":"alpha","error":{"code":"auth","message":"expired"}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 1 || snapshots[0].Key.Account != "alpha" || snapshots[0].Error == nil {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
}

func TestReturnedVariantUsesProviderPolicy(t *testing.T) {
	body := []byte(`{"provider":"codex","variant":"web","primary":{"usedPercent":10}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 1 || len(snapshots[0].Windows) != 0 || snapshots[0].Error == nil {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
	if snapshots[0].Key.String() == "codex:web@unknown" && snapshots[0].Error == nil {
		t.Fatalf("policy variant accepted: %s", snapshots[0].Key)
	}
}

func TestMonetaryUsageQuarantinedWithMismatchedVariant(t *testing.T) {
	body := []byte(`{"provider":"gemini","variant":"api","primary":{"usedPercent":10},"cost":{"used":1,"limit":10,"currency":"USD","period":"monthly"}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "gemini", Variant: domain.VariantWeb, SourceKey: "gemini:web"})
	if err == nil || len(snapshots) != 1 || len(snapshots[0].Details) != 0 || len(snapshots[0].Windows) != 0 {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
}

func TestMissingModelFieldMarksHistoryPartial(t *testing.T) {
	body := []byte(`{"days":[
		{"date":"2026-10-14","tokens":10,"models":[{"name":"m","input":10,"output":5}]},
		{"date":"2026-10-15","tokens":20,"models":[{"name":"m","output":5}]}
	]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !history.Partial || len(history.Models) != 1 || history.Models[0].Input == nil || *history.Models[0].Input != 10 {
		t.Fatalf("models=%+v partial=%v", history.Models, history.Partial)
	}
}

func TestReportedIncompleteCountPreserved(t *testing.T) {
	body := []byte(`{"days":[{"date":"2026-10-14","tokens":10}],"incomplete":2}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !history.Partial || history.Incomplete != 2 {
		t.Fatalf("partial=%v incomplete=%d", history.Partial, history.Incomplete)
	}
}

func TestGroupedExplicitWindowKeysStableAcrossOrder(t *testing.T) {
	first := []byte(`{"provider":"codex","windows":[{"key":"daily","group":"A","label":"Daily","usedPercent":1},{"key":"daily","group":"B","label":"Daily","usedPercent":2}]}`)
	second := []byte(`{"provider":"codex","windows":[{"key":"daily","group":"B","label":"Daily","usedPercent":2},{"key":"daily","group":"A","label":"Daily","usedPercent":1}]}`)
	mapping := func(body []byte) map[string]string {
		a, _ := adapterWith(body)
		snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
		if err != nil {
			t.Fatal(err)
		}
		result := map[string]string{}
		for _, window := range snapshots[0].Windows {
			result[window.Group] = window.Key
		}
		return result
	}
	a, b := mapping(first), mapping(second)
	if a["A"] != b["A"] || a["B"] != b["B"] || a["A"] == a["B"] {
		t.Fatalf("group mappings differ: %v vs %v", a, b)
	}
}

func TestEmptyAccountsPreserveEnvelopeError(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"provider":"muse","accounts":[],"error":{"code":"auth"}}`),
		[]byte(`[{"provider":"muse","accounts":[],"error":{"code":"auth"}}]`),
	} {
		a, _ := adapterWith(body)
		_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "muse"})
		if err == nil || categoryOf(t, err) != domain.ErrAuth {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestBaselineHistoryContractNormalizes(t *testing.T) {
	a, _ := adapterWith(fixture(t, "history_baseline.json"))
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Buckets) != 3 || history.Buckets[0].Tokens == nil || *history.Buckets[0].Tokens != 1000 {
		t.Fatalf("buckets=%+v", history.Buckets)
	}
	if history.TotalTokens == nil || *history.TotalTokens != 3000 {
		t.Fatalf("total=%v", history.TotalTokens)
	}
	if history.Cost == nil || history.Cost.Amount != 1.25 || history.Cost.Currency != "USD" {
		t.Fatalf("cost=%+v", history.Cost)
	}
	if len(history.Models) != 2 || history.Models[0].Input == nil {
		t.Fatalf("models=%+v", history.Models)
	}
	if !history.Partial || history.Incomplete != 1 || history.Coverage != "Local machine baseline" {
		t.Fatalf("coverage=%+v", history)
	}
}

func TestBaselineUsageExtraWindowsProviderCostAndCharts(t *testing.T) {
	a, _ := adapterWith(fixture(t, "usage_baseline.json"))
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := snapshots[0]
	if len(snapshot.Windows) != 2 || snapshot.Windows[0].UsedPercent == nil || snapshot.Windows[1].UsedPercent == nil || *snapshot.Windows[1].UsedPercent != 30 {
		t.Fatalf("windows=%+v", snapshot.Windows)
	}
	if len(snapshot.Details) < 2 {
		t.Fatalf("details=%+v", snapshot.Details)
	}
	foundPoints, foundBars := 0, 0
	for _, section := range snapshot.Details {
		foundPoints += len(section.Points)
		foundBars += len(section.Bars)
	}
	if foundPoints != 2 || foundBars != 1 {
		t.Fatalf("points=%d bars=%d details=%+v", foundPoints, foundBars, snapshot.Details)
	}
	monetary := false
	for _, section := range snapshot.Details {
		if section.Title == "Monetary usage" {
			monetary = true
		}
	}
	if !monetary {
		t.Fatalf("providerCost missing: %+v", snapshot.Details)
	}
}

func TestGroupedWindowKeyCollisionWithReservedExplicit(t *testing.T) {
	body := []byte(`{"provider":"codex","windows":[{"key":"daily","group":"A","usedPercent":1},{"key":"daily","group":"B","usedPercent":2},{"key":"daily-a","label":"Explicit","usedPercent":3}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, window := range snapshots[0].Windows {
		if seen[window.Key] {
			t.Fatalf("duplicate key %q: %+v", window.Key, snapshots[0].Windows)
		}
		seen[window.Key] = true
	}
	if !seen["daily-a"] {
		t.Fatalf("explicit key displaced: %+v", snapshots[0].Windows)
	}
}

func TestReturnedSourceModeValidatedAgainstRequestedVariant(t *testing.T) {
	body := []byte(`{"provider":"gemini","source":"api","usage":{"primary":{"usedPercent":25}}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "gemini", Variant: domain.VariantWeb, SourceKey: "gemini:web"})
	if err == nil || len(snapshots) != 1 || len(snapshots[0].Windows) != 0 || snapshots[0].Error == nil {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
}

func TestTopLevelIDIsProviderNotAccount(t *testing.T) {
	body := []byte(`{"id":"codex","primary":{"usedPercent":10}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshots[0].Key.AccountUnknown || snapshots[0].Key.Account != domain.UnknownAccount || snapshots[0].Key.String() != "codex@unknown" {
		t.Fatalf("identity=%s", snapshots[0].Key)
	}
}

func TestNestedHistoryTotalsCostsAggregatePerEnvelope(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":10}],"totals":{"totalTokens":10,"totalCost":1,"currency":"USD"}},
		{"days":[{"date":"2026-10-15","tokens":20}],"totals":{"totalTokens":20,"totalCost":2,"currency":"USD"}}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.Cost == nil || history.Cost.Amount != 3 || history.TotalTokens == nil || *history.TotalTokens != 30 {
		t.Fatalf("cost=%+v total=%v", history.Cost, history.TotalTokens)
	}
}

func TestZeroIncompleteCountDoesNotMarkPartial(t *testing.T) {
	body := []byte(`{"days":[{"date":"2026-10-14","tokens":10}],"totals":{"totalTokens":10,"incompleteCount":0}}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.Partial || history.Incomplete != 0 {
		t.Fatalf("partial=%v incomplete=%d", history.Partial, history.Incomplete)
	}
}

func TestSupportedChartObjectIsNotMarkedUnsupported(t *testing.T) {
	body := []byte(`{"provider":"codex","details":[{"title":"Chart","chart":{"points":[{"label":"Mon","value":3}]}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	section := snapshots[0].Details[0]
	if section.Unsupported || len(section.Points) != 1 || snapshots[0].State != domain.StatusCurrent {
		t.Fatalf("section=%+v state=%v", section, snapshots[0].State)
	}
}

func TestHistoryCostNotDoubleCountedAcrossTopAndTotals(t *testing.T) {
	body := []byte(`{"days":[{"date":"2026-10-14","tokens":10}],"cost":{"amount":1.25,"currency":"USD"},"totals":{"totalCost":1.25,"currency":"USD"}}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.Cost == nil || history.Cost.Amount != 1.25 {
		t.Fatalf("cost double counted: %+v", history.Cost)
	}
}

func TestNestedScalarCostPreservesReportedProvenance(t *testing.T) {
	body := []byte(`{"days":[{"date":"2026-10-14","tokens":10}],"costReported":true,"totals":{"totalCost":1.25,"currency":"USD"}}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if history.Cost == nil || !history.CostReported {
		t.Fatalf("cost=%+v reported=%v", history.Cost, history.CostReported)
	}
}

func TestReturnedProviderGrammarRejected(t *testing.T) {
	body := []byte(`{"provider":"bad@account","primary":{"usedPercent":10}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 1 || snapshots[0].Key.Provider != "codex" || snapshots[0].Error == nil {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
}

func TestConfiguredModeAcceptsBackendSelectedSource(t *testing.T) {
	for _, source := range []string{"web", "api"} {
		body := []byte(`{"provider":"gemini","source":"` + source + `","primary":{"usedPercent":25}}`)
		a, _ := adapterWith(body)
		snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "gemini", Variant: domain.VariantConfigured, SourceKey: "gemini:configured"})
		if err != nil || len(snapshots) != 1 || len(snapshots[0].Windows) != 1 || snapshots[0].Key.String() != "gemini:configured@unknown" {
			t.Fatalf("source=%s err=%v snapshots=%+v", source, err, snapshots)
		}
	}
}

func TestUnsupportedHistoryModelStructuresDisclosed(t *testing.T) {
	body := []byte(`{"days":[{"date":"2026-10-14","tokens":10,"models":{"new":"shape"}}]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err == nil || len(history.Buckets) != 1 || !history.Partial {
		t.Fatalf("err=%v history=%+v", err, history)
	}
}

func TestFlattenedResultsInheritEnvelopeSourceAndSample(t *testing.T) {
	body := []byte(`{"provider":"gemini","source":"api","sampleAt":"2026-10-16T12:00:00Z","results":[{"primary":{"usedPercent":25}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "gemini", Variant: domain.VariantWeb, SourceKey: "gemini:web"})
	if err == nil || len(snapshots) != 1 || len(snapshots[0].Windows) != 0 {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
	if snapshots[0].ReportedSource != "api" {
		t.Fatalf("reported source=%q", snapshots[0].ReportedSource)
	}
	wantSample := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	if snapshots[0].SampleAt == nil || !snapshots[0].SampleAt.Equal(wantSample) {
		t.Fatalf("sample time=%v", snapshots[0].SampleAt)
	}
}

func TestObjectErrorWithExtraRateWindowsRetainsMeasurement(t *testing.T) {
	body := []byte(`{"provider":"codex","extraRateWindows":[{"label":"Extra","usedPercent":30}],"error":{"code":"auth","message":"expired"}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 1 || len(snapshots[0].Windows) != 1 || snapshots[0].Error == nil {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
}

func TestNamedWindowArrayDoesNotAddUnsupportedFallback(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":[{"label":"One","usedPercent":10},{"label":"Two","usedPercent":20}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots[0].Windows) != 2 || snapshots[0].State == domain.StatusBackendIncompatible {
		t.Fatalf("windows=%+v state=%v", snapshots[0].Windows, snapshots[0].State)
	}
}

func TestProviderCostWithErrorRetainsMonetaryDetails(t *testing.T) {
	body := []byte(`{"provider":"codex","providerCost":{"used":1,"limit":10,"currency":"USD","period":"monthly"},"error":{"code":"auth","message":"expired"}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 1 {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
	found := false
	for _, section := range snapshots[0].Details {
		if section.Title == "Monetary usage" {
			found = true
		}
	}
	if !found || snapshots[0].Error == nil {
		t.Fatalf("monetary details=%+v error=%v", snapshots[0].Details, snapshots[0].Error)
	}
}

func TestInvalidNamedWindowMemberDisclosed(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"secondary":[42]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	windows := snapshots[0].Windows
	disclosed := false
	for _, window := range windows {
		if window.Quality == domain.QualityUnsupported {
			disclosed = true
		}
	}
	if len(windows) != 2 || !disclosed || snapshots[0].State != domain.StatusBackendIncompatible {
		t.Fatalf("windows=%+v state=%v", windows, snapshots[0].State)
	}
}

func TestUnsupportedRowProgressDisclosed(t *testing.T) {
	body := []byte(`{"provider":"codex","details":[{"title":"Mixed","rows":[{"label":"A","value":"B","progress":[42]}]}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	section := snapshots[0].Details[0]
	if !section.Unsupported || len(section.Rows) != 1 {
		t.Fatalf("section=%+v", section)
	}
}

func TestUnknownCurrencyContributionMarksPartial(t *testing.T) {
	body := []byte(`[
		{"days":[{"date":"2026-10-14","tokens":10}],"cost":{"amount":1}},
		{"days":[{"date":"2026-10-15","tokens":20}],"cost":{"amount":2,"currency":"USD"}}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !history.Partial || !strings.Contains(history.Coverage, "unknown currency") {
		t.Fatalf("partial=%v coverage=%q", history.Partial, history.Coverage)
	}
}

func TestAccountMembersOverrideEnvelopeIdentity(t *testing.T) {
	body := []byte(`{"provider":"codex","identity":{"accountID":"parent"},"accounts":[{"accountKey":"alpha","accountLabel":"Alpha","primary":{"usedPercent":10}},{"accountKey":"beta","accountLabel":"Beta","primary":{"usedPercent":20}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil || len(snapshots) != 2 {
		t.Fatalf("snapshots=%d err=%v", len(snapshots), err)
	}
	if snapshots[0].Key.Account != "alpha" || snapshots[1].Key.Account != "beta" || snapshots[0].AccountLabel == snapshots[1].AccountLabel {
		t.Fatalf("identity collision: %+v %+v", snapshots[0], snapshots[1])
	}
}

func TestCoexistingBalanceCollectionsAreRetained(t *testing.T) {
	body := []byte(`{"provider":"codex","balances":[{"label":"USD","value":1,"unit":"USD"}],"credits":[{"label":"Credits","value":2,"unit":"credits"}],"primary":{"usedPercent":10}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots[0].Balances) != 2 {
		t.Fatalf("balances=%+v", snapshots[0].Balances)
	}
}

func TestEmptyAccountsInMixedArrayScopedFailure(t *testing.T) {
	body := []byte(`[
		{"provider":"codex","accountKey":"ok","primary":{"usedPercent":10}},
		{"provider":"codex","accounts":[]}
	]`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err == nil || len(snapshots) != 2 || snapshots[1].Error == nil || snapshots[1].Error.Category != domain.ErrUnsupported {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
}

func TestCatalogRejectsInvalidProviderGrammar(t *testing.T) {
	body := []byte(`{"providers":[{"provider":"bad@account","enabled":true}]}`)
	a, _ := adapterWith(body)
	_, err := a.Catalog(context.Background())
	if err == nil || categoryOf(t, err) != domain.ErrUnsupported {
		t.Fatalf("err=%v", err)
	}
}

func TestAccountIdentityAliasesDoNotOverrideChildren(t *testing.T) {
	body := []byte(`{"provider":"codex","accountID":"parent","identity":{"accountID":"parent"},"accounts":[{"accountId":"alpha","primary":{"usedPercent":10}},{"accountID":"beta","primary":{"usedPercent":20}},{"primary":{"usedPercent":30}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil || len(snapshots) != 3 {
		t.Fatalf("snapshots=%d err=%v", len(snapshots), err)
	}
	if snapshots[0].Key.Account != "alpha" || snapshots[1].Key.Account != "beta" || !snapshots[2].Key.AccountUnknown {
		t.Fatalf("accounts=%s,%s,%s", snapshots[0].Key, snapshots[1].Key, snapshots[2].Key)
	}
}

func TestNestedAccountUsageDoesNotInheritEnvelopeMeasurement(t *testing.T) {
	body := []byte(`{"provider":"codex","usage":{"identity":{"accountID":"parent"},"primary":{"usedPercent":90}},"accounts":[{"accountId":"alpha","usage":{"primary":{"usedPercent":10}}},{"accountId":"beta","usage":{"primary":{"usedPercent":20}}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil || len(snapshots) != 2 {
		t.Fatalf("snapshots=%d err=%v", len(snapshots), err)
	}
	for index, expected := range []float64{10, 20} {
		if snapshots[index].Key.Account != []string{"alpha", "beta"}[index] || len(snapshots[index].Windows) != 1 || snapshots[index].Windows[0].UsedPercent == nil || *snapshots[index].Windows[0].UsedPercent != expected {
			t.Fatalf("snapshot=%+v", snapshots[index])
		}
	}
}

func TestEmptyDailyHistoryPreservesMeasuredZeroTotals(t *testing.T) {
	body := []byte(`{"daily":[],"totals":{"totalTokens":0,"totalCost":0,"currency":"USD"},"coverage":"empty baseline"}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Buckets) != 0 || history.TotalTokens == nil || *history.TotalTokens != 0 || history.Cost == nil || history.Cost.Amount != 0 || history.Coverage != "empty baseline" {
		t.Fatalf("history=%+v", history)
	}
	if !history.Start.IsZero() || !history.End.IsZero() {
		t.Fatalf("empty history fabricated dates: %v-%v", history.Start, history.End)
	}
}

func TestResultsEnvelopeIdentityDoesNotOverrideChildren(t *testing.T) {
	body := []byte(`{"provider":"codex","identity":{"accountID":"parent"},"results":[{"accountKey":"alpha","primary":{"usedPercent":10}},{"accountKey":"beta","primary":{"usedPercent":20}},{"primary":{"usedPercent":30}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil || len(snapshots) != 3 {
		t.Fatalf("snapshots=%d err=%v", len(snapshots), err)
	}
	if snapshots[0].Key.Account != "alpha" || snapshots[1].Key.Account != "beta" || !snapshots[2].Key.AccountUnknown {
		t.Fatalf("accounts=%s,%s,%s", snapshots[0].Key, snapshots[1].Key, snapshots[2].Key)
	}
}

func TestUnsupportedCreditScalarDisclosed(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10},"credits":42}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := snapshots[0]
	disclosed := len(snapshot.Details) > 0 && snapshot.Details[len(snapshot.Details)-1].Unsupported
	for _, balance := range snapshot.Balances {
		if balance.Kind == "unsupported" {
			disclosed = true
		}
	}
	if !disclosed || len(snapshot.Windows) != 1 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestEmptyHistoryWithErrorRetainsMeasuredZeroSummary(t *testing.T) {
	body := []byte(`{"daily":[],"totals":{"totalTokens":0,"totalCost":0,"currency":"USD"},"coverage":"empty baseline","error":{"code":"auth"}}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err == nil || categoryOf(t, err) != domain.ErrAuth {
		t.Fatalf("err=%v", err)
	}
	if history.TotalTokens == nil || *history.TotalTokens != 0 || history.Cost == nil || history.Cost.Amount != 0 || history.Coverage != "empty baseline" {
		t.Fatalf("summary discarded: %+v", history)
	}
}

func TestParentMeasurementsDoNotLeakIntoAccountsOrResults(t *testing.T) {
	bodies := [][]byte{
		[]byte(`{"provider":"codex","primary":{"usedPercent":90},"credits":{"label":"Credits","value":123,"unit":"USD"},"accounts":[{"accountKey":"alpha","secondary":{"usedPercent":10}}]}`),
		[]byte(`{"provider":"codex","identity":{"accountID":"parent"},"primary":{"usedPercent":90},"credits":{"label":"Credits","value":123,"unit":"USD"},"results":[{"accountKey":"alpha","secondary":{"usedPercent":10}}]}`),
	}
	for _, body := range bodies {
		a, _ := adapterWith(body)
		snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
		if err != nil || len(snapshots) != 1 {
			t.Fatalf("snapshots=%d err=%v", len(snapshots), err)
		}
		snapshot := snapshots[0]
		if len(snapshot.Windows) != 1 || snapshot.Windows[0].Label != "Secondary" || len(snapshot.Balances) != 0 {
			t.Fatalf("parent measurements leaked: windows=%+v balances=%+v", snapshot.Windows, snapshot.Balances)
		}
	}
}

func TestInvalidExplicitPercentageIsUnavailableNotGauge(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"usedPercent":"NaN","used":0,"limit":100}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	window := snapshots[0].Windows[0]
	if window.UsedPercent != nil || window.RemainingPercent() != nil || window.Quality != domain.QualityUnavailable {
		t.Fatalf("window=%+v remaining=%v", window, window.RemainingPercent())
	}
}

func TestHistoryIncompatibleTimezonesNotAggregated(t *testing.T) {
	body := []byte(`[
		{"timezone":"UTC","days":[{"date":"2026-10-14","tokens":10}]},
		{"timezone":"Europe/Madrid","days":[{"date":"2026-10-14","tokens":20}]}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err == nil || len(history.Buckets) != 1 || history.Buckets[0].Tokens == nil || *history.Buckets[0].Tokens != 10 || !history.Partial {
		t.Fatalf("err=%v history=%+v", err, history)
	}
}

func TestDiagnosticRedactsArbitraryProjectPath(t *testing.T) {
	process := &ProcessError{Code: 1, Stderr: []byte("failure at /mnt/example-project/sessions/log.json")}
	if strings.Contains(process.Diagnostic(), "/mnt/example-project") {
		t.Fatalf("diagnostic leaked project path: %s", process.Diagnostic())
	}
}

func TestChildAliasSourceAndTimestampTakePrecedence(t *testing.T) {
	body := []byte(`{"provider":"gemini","reportedSource":"web","sampleAt":"2026-10-16T14:00:00Z","results":[{"accountKey":"alpha","source":"api","updatedAt":"2026-10-15T14:00:00Z","primary":{"usedPercent":25}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "gemini", Variant: domain.VariantWeb, SourceKey: "gemini:web"})
	if err == nil || len(snapshots) != 1 || len(snapshots[0].Windows) != 0 || snapshots[0].Error == nil {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
	if snapshots[0].ReportedSource != "api" {
		t.Fatalf("reported source=%q", snapshots[0].ReportedSource)
	}
	want := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)
	if snapshots[0].SampleAt == nil || !snapshots[0].SampleAt.Equal(want) {
		t.Fatalf("sample=%v", snapshots[0].SampleAt)
	}
}

func TestParentNestedUsageCostDoesNotLeakIntoAccounts(t *testing.T) {
	body := []byte(`{"provider":"codex","usage":{"cost":{"used":99,"limit":100,"currency":"USD","period":"monthly"}},"accounts":[{"accountKey":"alpha","primary":{"usedPercent":10}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("snapshots=%d err=%v", len(snapshots), err)
	}
	for _, section := range snapshots[0].Details {
		if section.Title == "Monetary usage" {
			t.Fatalf("parent monetary usage leaked: %+v", section)
		}
	}
	if len(snapshots[0].Windows) != 1 {
		t.Fatalf("child measurement lost: %+v", snapshots[0])
	}
}

func TestDecodeErrorsCarryCanonicalScope(t *testing.T) {
	for _, body := range [][]byte{nil, []byte("{bad")} {
		a, _ := adapterWith(body)
		_, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex", SourceKey: "codex@alpha"})
		scopedErr, ok := domain.AsScopedError(err)
		if !ok || scopedErr.SourceKey != "codex@alpha" {
			t.Fatalf("usage err=%v", err)
		}
		_, err = a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", SourceKey: "codex@alpha", Days: 7})
		scopedErr, ok = domain.AsScopedError(err)
		if !ok || scopedErr.SourceKey != "codex@alpha" {
			t.Fatalf("history err=%v", err)
		}
	}
}

func TestAccountAliasSourceAndTimestampPrecedence(t *testing.T) {
	body := []byte(`{"provider":"gemini","reportedSource":"web","sampleAt":"2026-10-16T14:00:00Z","accounts":[{"accountKey":"alpha","source":"api","updatedAt":"2026-10-15T14:00:00Z","primary":{"usedPercent":25}}]}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "gemini", Variant: domain.VariantWeb, SourceKey: "gemini:web"})
	if err == nil || len(snapshots) != 1 || len(snapshots[0].Windows) != 0 || snapshots[0].ReportedSource != "api" {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
	want := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)
	if snapshots[0].SampleAt == nil || !snapshots[0].SampleAt.Equal(want) {
		t.Fatalf("sample=%v", snapshots[0].SampleAt)
	}
}

func TestCookieAssignmentAndPathAssignmentRedacted(t *testing.T) {
	process := &ProcessError{Code: 1, Stderr: []byte(`cookie="first=one; second=SYNTHETIC-COOKIE-SECRET"` + "\n" + `path=/srv/fictional-project/log.json`)}
	diagnostic := process.Diagnostic()
	if strings.Contains(diagnostic, "SYNTHETIC-COOKIE-SECRET") || strings.Contains(diagnostic, "/srv/fictional-project") {
		t.Fatalf("diagnostic leaked: %s", diagnostic)
	}
}

func TestNullPercentageKeepsRawQuantities(t *testing.T) {
	body := []byte(`{"provider":"codex","primary":{"usedPercent":null,"used":3,"limit":10,"unit":"requests"}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	window := snapshots[0].Windows[0]
	if window.Used == nil || *window.Used != 3 || window.Limit == nil || *window.Limit != 10 || window.UsedPercent != nil {
		t.Fatalf("window=%+v", window)
	}
}

func TestNestedUsageChildIdentityAndLabelPrecedence(t *testing.T) {
	body := []byte(`{"provider":"codex","identity":{"accountID":"parent","accountLabel":"Parent"},"usage":{"accountKey":"child","primary":{"usedPercent":10}}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("err=%v snapshots=%+v", err, snapshots)
	}
	if snapshots[0].Key.Account != "child" || snapshots[0].AccountLabel == "Parent" {
		t.Fatalf("identity=%s label=%q", snapshots[0].Key, snapshots[0].AccountLabel)
	}
}

func TestChildCacheAgeOverridesParentSample(t *testing.T) {
	body := []byte(`{"provider":"codex","sampleAt":"2026-10-16T14:00:00Z","results":[{"accountKey":"alpha","cacheAgeSeconds":3600,"primary":{"usedPercent":10}}]}`)
	a, _ := adapterWith(body)
	fixed := time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return fixed }
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	want := fixed.Add(-time.Hour)
	if snapshots[0].SampleAt == nil || !snapshots[0].SampleAt.Equal(want) {
		t.Fatalf("sample=%v want=%v", snapshots[0].SampleAt, want)
	}
}

func TestQuotedPasswordWithWhitespaceRedacted(t *testing.T) {
	process := &ProcessError{Code: 1, Stderr: []byte(`password="FIRST SYNTHETIC-SECRET-TAIL"`)}
	diagnostic := process.Diagnostic()
	if strings.Contains(diagnostic, "SYNTHETIC-SECRET-TAIL") || strings.Contains(diagnostic, "FIRST") {
		t.Fatalf("diagnostic leaked: %s", diagnostic)
	}
}

func TestHistoryDifferentReadersNotAggregated(t *testing.T) {
	body := []byte(`[
		{"provider":"codex","source":"web","provenance":"web reader","days":[{"date":"2026-10-14","tokens":10}]},
		{"provider":"codex","source":"api","provenance":"api reader","days":[{"date":"2026-10-15","tokens":20}]}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", Days: 7})
	if err == nil || len(history.Buckets) != 1 || history.TotalTokens == nil || *history.TotalTokens != 10 || !history.Partial {
		t.Fatalf("err=%v history=%+v", err, history)
	}
	if !strings.Contains(history.Provenance, "web reader") || !strings.Contains(history.Provenance, "provider-native-only not requested") {
		t.Fatalf("provenance=%q", history.Provenance)
	}
}

func TestFlatHistoryRecordsPreserveReaderCompatibility(t *testing.T) {
	body := []byte(`[
		{"date":"2026-10-14","tokens":10,"source":"web","timezone":"UTC"},
		{"date":"2026-10-15","tokens":20,"source":"api","timezone":"UTC"}
	]`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err == nil || len(history.Buckets) != 1 || !history.Partial || history.TotalTokens == nil || *history.TotalTokens != 10 {
		t.Fatalf("err=%v history=%+v", err, history)
	}
}

func TestObjectBalanceAndUsageDetailsWithErrorRetained(t *testing.T) {
	bodies := [][]byte{
		[]byte(`{"provider":"codex","balance":{"value":3,"unit":"USD"},"error":{"code":"auth"}}`),
		[]byte(`{"provider":"codex","usageDetails":[{"title":"Rows","rows":[{"label":"A","value":"B"}]}],"error":{"code":"auth"}}`),
	}
	for _, body := range bodies {
		a, _ := adapterWith(body)
		snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
		if err == nil || len(snapshots) != 1 || snapshots[0].Error == nil {
			t.Fatalf("err=%v snapshots=%+v", err, snapshots)
		}
		if len(snapshots[0].Balances) == 0 && len(snapshots[0].Details) == 0 {
			t.Fatalf("measurement lost: %+v", snapshots[0])
		}
	}
}

func TestHistoryScopeDisclosedAlongsideBackendProvenance(t *testing.T) {
	body := []byte(`{"source":"local","provenance":"local","days":[{"date":"2026-10-14","tokens":10}]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", Days: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(history.Provenance, "local") || !strings.Contains(history.Provenance, "provider-native-only not requested") {
		t.Fatalf("provenance=%q", history.Provenance)
	}
}

func TestQuotedSecretsWithColonAndTokenAliasesRedacted(t *testing.T) {
	process := &ProcessError{Code: 1, Stderr: []byte("password: \"FIRST SYNTHETIC-SECRET-TAIL\"\naccessToken: \"ACCESS-SECRET-TAIL\"")}
	diagnostic := process.Diagnostic()
	for _, secret := range []string{"SYNTHETIC-SECRET-TAIL", "ACCESS-SECRET-TAIL"} {
		if strings.Contains(diagnostic, secret) {
			t.Fatalf("diagnostic leaked %q: %s", secret, diagnostic)
		}
	}
}

func TestParentDetailAliasesDoNotLeakIntoChildren(t *testing.T) {
	bodies := [][]byte{
		[]byte(`{"provider":"codex","usageDetails":[{"title":"Spend","rows":[{"label":"A","value":"B"}]}],"accounts":[{"accountKey":"child","primary":{"usedPercent":10}}]}`),
		[]byte(`{"provider":"codex","usageDetails":[{"title":"Spend","rows":[{"label":"A","value":"B"}]}],"results":[{"accountKey":"child","primary":{"usedPercent":10}}]}`),
	}
	for _, body := range bodies {
		a, _ := adapterWith(body)
		snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
		if err != nil || len(snapshots) != 1 || len(snapshots[0].Details) != 0 {
			t.Fatalf("err=%v snapshots=%+v", err, snapshots)
		}
	}
}

func TestHistoryRecordLevelSourceValidation(t *testing.T) {
	body := []byte(`{"provider":"codex","days":[
		{"date":"2026-10-14","tokens":10,"provider":"codex"},
		{"date":"2026-10-15","tokens":20,"provider":"claude"}
	]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", Days: 7})
	if err == nil || len(history.Buckets) != 1 || history.TotalTokens == nil || *history.TotalTokens != 10 || !history.Partial {
		t.Fatalf("err=%v history=%+v", err, history)
	}
}

func TestNullSiblingPreservesKnownDailySubtotal(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`[{"accountKey":"one","days":[{"date":"2026-10-14","tokens":10}]},{"accountKey":"two","days":[{"date":"2026-10-14","tokens":null}]}]`),
		[]byte(`[{"accountKey":"two","days":[{"date":"2026-10-14","tokens":null}]},{"accountKey":"one","days":[{"date":"2026-10-14","tokens":10}]}]`),
	} {
		a, _ := adapterWith(body)
		history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
		if err != nil {
			t.Fatal(err)
		}
		if len(history.Buckets) != 1 || history.Buckets[0].Tokens == nil || *history.Buckets[0].Tokens != 10 || !history.Partial {
			t.Fatalf("buckets=%+v partial=%v", history.Buckets, history.Partial)
		}
	}
}

func TestAuthorizationSessionAndIDTokenQuotedRedacted(t *testing.T) {
	process := &ProcessError{Code: 1, Stderr: []byte("authorization=\"Bearer AUTH-SECRET\" `session=\"SESSION-SECRET\" idToken: \"ID-SECRET\"`")}
	diagnostic := process.Diagnostic()
	for _, secret := range []string{"AUTH-SECRET", "SESSION-SECRET", "ID-SECRET"} {
		if strings.Contains(diagnostic, secret) {
			t.Fatalf("diagnostic leaked %q: %s", secret, diagnostic)
		}
	}
}

func TestRecordLevelReaderWithoutEnvelopeMetadata(t *testing.T) {
	body := []byte(`{"days":[
		{"date":"2026-10-14","tokens":10,"source":"web"},
		{"date":"2026-10-15","tokens":20,"source":"api"}
	]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err == nil || len(history.Buckets) != 1 || history.TotalTokens == nil || *history.TotalTokens != 10 || !history.Partial {
		t.Fatalf("err=%v history=%+v", err, history)
	}
}

func TestRecordLevelTimezoneWithoutEnvelopeMetadata(t *testing.T) {
	body := []byte(`{"days":[
		{"date":"2026-10-14","tokens":10,"timezone":"UTC"},
		{"date":"2026-10-15","tokens":20,"timezone":"Europe/Madrid"}
	]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err == nil || len(history.Buckets) != 1 || history.TotalTokens == nil || *history.TotalTokens != 10 || !history.Partial {
		t.Fatalf("err=%v history=%+v", err, history)
	}
}

func TestRejectedRecordsDoNotContributeModelsOrEnvelopeTotals(t *testing.T) {
	body := []byte(`{"provider":"codex","totals":{"totalTokens":30},"days":[
		{"date":"2026-10-14","tokens":10,"provider":"codex","models":[{"name":"codex-model","input":10}]},
		{"date":"2026-10-15","tokens":20,"provider":"claude","models":[{"name":"claude-model","input":20}]}
	]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "codex", Days: 7})
	if err == nil || history.TotalTokens == nil || *history.TotalTokens != 10 {
		t.Fatalf("err=%v total=%v", err, history.TotalTokens)
	}
	for _, model := range history.Models {
		if model.Name == "claude-model" {
			t.Fatalf("rejected model included: %+v", history.Models)
		}
	}
}

func TestMidRunCancellationIsDistinct(t *testing.T) {
	runner := &fakeRunner{fn: func(ctx context.Context, _ string, _ []string, _ int) ([]byte, []byte, error) {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}}
	a := &Adapter{path: "/fixture/codexbar", runner: runner}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := a.Usage(ctx, backend.UsageRequest{ProviderID: "codex"})
		result <- err
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestCatalogAndVersionRetainOutputOnNonzeroExit(t *testing.T) {
	catalogBody := fixture(t, "catalog.json")
	runner := &fakeRunner{fn: func(_ context.Context, _ string, args []string, _ int) ([]byte, []byte, error) {
		if len(args) > 0 && args[0] == "--version" {
			return []byte("CodexBar 0.77.0\n"), nil, &ProcessError{Code: 1}
		}
		return catalogBody, nil, &ProcessError{Code: 1}
	}}
	a := &Adapter{path: "/fixture/codexbar", runner: runner}
	version, err := a.Version(context.Background())
	if version.Version != "CodexBar 0.77.0" || err == nil {
		t.Fatalf("version=%+v err=%v", version, err)
	}
	catalog, err := a.Catalog(context.Background())
	if len(catalog.Providers) != 3 || err == nil {
		t.Fatalf("catalog=%d err=%v", len(catalog.Providers), err)
	}
}

func TestUnreportedHistoryTimezoneStaysUnknown(t *testing.T) {
	body := []byte(`{"days":[{"date":"2026-10-14","tokens":10}]}`)
	a, _ := adapterWith(body)
	history, err := a.History(context.Background(), backend.HistoryRequest{ProviderID: "muse", Days: 7})
	if err != nil || history.Timezone != "" {
		t.Fatalf("history=%+v err=%v", history, err)
	}
}

func TestTopLevelDisplayNameIsNotAccountLabel(t *testing.T) {
	body := []byte(`{"provider":"codex","name":"Display Name","primary":{"usedPercent":10}}`)
	a, _ := adapterWith(body)
	snapshots, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"})
	if err != nil || snapshots[0].AccountLabel != "" {
		t.Fatalf("label=%q err=%v", snapshots[0].AccountLabel, err)
	}
}

func TestUnkeywordedJWTIsRedacted(t *testing.T) {
	process := &ProcessError{Code: 1, Stderr: []byte("debug eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signaturepart")}
	if strings.Contains(process.Diagnostic(), "eyJhbGci") {
		t.Fatalf("JWT leaked: %s", process.Diagnostic())
	}
}

func TestOperationDeadlineConstants(t *testing.T) {
	if versionTimeout != 5*time.Second || catalogTimeout != 10*time.Second || usageTimeout != 60*time.Second || probeTimeout != 120*time.Second || historyTimeout != 90*time.Second {
		t.Fatal("operation deadline constants changed")
	}
}

type envRunner struct {
	fakeRunner
	env []string
}

func (e *envRunner) RunEnv(ctx context.Context, path string, args []string, max int, env []string) ([]byte, []byte, error) {
	e.env = append([]string(nil), env...)
	return []byte(`{"provider":"codex","primary":{"label":"Plan","usedPercent":10}}`), nil, nil
}

func TestConfiguredEnvironmentReachesRunner(t *testing.T) {
	runner := &envRunner{}
	a := &Adapter{path: "/fixture/codexbar", runner: runner, env: []string{"CODEXBAR_CONFIG=/tmp/config.json"}}
	if _, err := a.Usage(context.Background(), backend.UsageRequest{ProviderID: "codex"}); err != nil {
		t.Fatal(err)
	}
	if len(runner.env) != 1 || runner.env[0] != "CODEXBAR_CONFIG=/tmp/config.json" {
		t.Fatalf("env=%v", runner.env)
	}
}
