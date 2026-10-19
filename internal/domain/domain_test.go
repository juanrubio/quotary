package domain

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestSourceKeyRoundTrip(t *testing.T) {
	cases := []string{
		"codex:configured",
		"gemini:web",
		"gemini:api@account%40example.com",
		"claude:configured@unknown",
		"codex:configured@opaque%01id",
	}
	for _, raw := range cases {
		key, err := ParseSourceKey(raw)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if got := key.String(); got != raw {
			t.Fatalf("%s round-trips as %s", raw, got)
		}
	}
}

func TestSourceKeyEscaping(t *testing.T) {
	key := SourceKey{Provider: "codex", Variant: VariantConfigured, Account: "a:b@c%"}
	raw := key.String()
	if raw != "codex:configured@a%3Ab%40c%25" {
		t.Fatalf("raw = %s", raw)
	}
	parsed, err := ParseSourceKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Account != key.Account {
		t.Fatalf("account = %q", parsed.Account)
	}
}

func TestSourceKeyRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"", "Codex", "codex:WEB", "codex:@", "codex:configured@", "codex:", "codex@a%1G", "codex@a%2", "codex@a b", "codex@a:b", "codex@a/b", "codex@%FF", "codex@%FE"} {
		if _, err := ParseSourceKey(raw); err == nil {
			t.Fatalf("%q should fail", raw)
		}
	}
}

func TestSourceKeyAccountChangesAreDistinct(t *testing.T) {
	a := SourceKey{Provider: "codex", Variant: VariantConfigured, Account: "a"}
	b := SourceKey{Provider: "codex", Variant: VariantConfigured, Account: "b"}
	unknown := SourceKey{Provider: "codex", Variant: VariantConfigured, Account: UnknownAccount, AccountUnknown: true}
	if a.String() == b.String() || a.String() == unknown.String() || b.String() == unknown.String() {
		t.Fatal("account keys must be distinct")
	}
}

func TestSourceKeyCoalescing(t *testing.T) {
	known := SourceKey{Provider: "codex", Variant: VariantConfigured, Account: "a"}
	if !known.CoalescesWith(SourceKey{Provider: "codex", Variant: VariantConfigured, Account: "a"}) {
		t.Fatal("same key should coalesce")
	}
	unknown := SourceKey{Provider: "codex", Variant: VariantConfigured, Account: UnknownAccount, AccountUnknown: true}
	unspecified := SourceKey{Provider: "codex", Variant: VariantConfigured}
	if known.CoalescesWith(unknown) || unknown.CoalescesWith(unspecified) {
		t.Fatal("known, unknown, and unspecified identity must not coalesce")
	}
}

func TestVariantValidation(t *testing.T) {
	key := SourceKey{Provider: "gemini", Variant: VariantWeb}
	if err := key.ValidateVariant(map[Variant]bool{VariantWeb: true}); err != nil {
		t.Fatal(err)
	}
	if err := key.ValidateVariant(map[Variant]bool{VariantAPI: true}); err == nil {
		t.Fatal("unsupported variant should fail")
	}
}

func percent(value float64) *float64 { return &value }

func TestRemainingPercent(t *testing.T) {
	cases := []struct {
		used *float64
		want *float64
	}{
		{nil, nil},
		{percent(-1), nil},
		{percent(math.NaN()), nil},
		{percent(math.Inf(1)), nil},
		{percent(0), percent(100)},
		{percent(100), percent(0)},
		{percent(137), percent(0)},
	}
	for _, tc := range cases {
		got := (AllowanceWindow{UsedPercent: tc.used}).RemainingPercent()
		if tc.want == nil {
			if got != nil {
				t.Fatalf("used=%v got %v", tc.used, *got)
			}
			continue
		}
		if got == nil || *got != *tc.want {
			t.Fatalf("used=%v got %v want %v", tc.used, got, *tc.want)
		}
	}
}

func TestOverLimit(t *testing.T) {
	if !(AllowanceWindow{UsedPercent: percent(101)}).OverLimit() {
		t.Fatal("101 should be over limit")
	}
	if (AllowanceWindow{UsedPercent: percent(100)}).OverLimit() {
		t.Fatal("100 is exactly exhausted, not over limit")
	}
	if (AllowanceWindow{UsedPercent: percent(math.Inf(1))}).OverLimit() {
		t.Fatal("non-finite is unavailable")
	}
}

func TestGaugeClamps(t *testing.T) {
	if got := GaugeCells(nil, 10); got != 0 {
		t.Fatalf("nil gauge = %d", got)
	}
	if got := GaugeCells(percent(72), 10); got != 7 {
		t.Fatalf("72%% = %d", got)
	}
	if got := GaugeCells(percent(200), 10); got != 10 {
		t.Fatalf("over-limit gauge = %d", got)
	}
	if got := GaugeText(percent(30), 10, true); got != "###-------" {
		t.Fatalf("ascii gauge = %q", got)
	}
	nan := math.NaN()
	if got := GaugeCells(&nan, 10); got != 0 {
		t.Fatalf("NaN gauge = %d", got)
	}
	inf := math.Inf(1)
	if got := GaugeCells(&inf, 10); got != 0 {
		t.Fatalf("Inf gauge = %d", got)
	}
}

func TestNumericZeroIsMeasured(t *testing.T) {
	zero, zeroFloat := int64(0), 0.0
	if (Balance{Value: &zeroFloat}).Value == nil {
		t.Fatal("measured zero credits must be available")
	}
	if (Balance{}).Value != nil {
		t.Fatal("missing credits must be unavailable")
	}
	history := HistorySnapshot{
		Buckets:     []HistoryDay{{Day: "2026-10-10", Tokens: &zero}, {Day: "2026-10-11"}},
		TotalTokens: &zero,
		Models:      []ModelTotal{{Name: "A", Input: &zero}, {Name: "B"}},
		Cost:        &Money{Amount: 0, Currency: "USD"},
	}
	if history.Buckets[0].Tokens == nil || *history.Buckets[0].Tokens != 0 {
		t.Fatal("zero tokens should stay measured")
	}
	if history.Buckets[1].Tokens != nil {
		t.Fatal("missing tokens must be nil")
	}
	if history.TotalTokens == nil || *history.TotalTokens != 0 {
		t.Fatal("zero total must stay measured")
	}
	if history.Models[0].Input == nil || history.Models[1].Input != nil {
		t.Fatal("model optionality is wrong")
	}
	if history.Cost == nil || history.Cost.Amount != 0 || history.Cost.Currency != "USD" {
		t.Fatal("measured zero cost must stay available")
	}
}

func TestFreshness(t *testing.T) {
	now := time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
	sample := now.Add(-30 * time.Second)
	received := now.Add(-10 * time.Second)
	info := CheckFreshness(&sample, &received, now, 5*time.Minute, false)
	if info.SampleAge != 30*time.Second || info.Stale || !info.SampleKnown {
		t.Fatalf("info = %+v", info)
	}
	if info.AgeLabel() != "30s ago" {
		t.Fatalf("label = %q", info.AgeLabel())
	}
	unknown := CheckFreshness(nil, &received, now, 5*time.Minute, false)
	if unknown.SampleKnown || unknown.AgeLabel() != "time unknown" || unknown.ReceiptAge != 10*time.Second {
		t.Fatalf("unknown = %+v label=%q", unknown, unknown.AgeLabel())
	}
	old := now.Add(-11 * time.Minute)
	if !CheckFreshness(&old, &received, now, 5*time.Minute, false).Stale {
		t.Fatal("old sample should be stale")
	}
	boundary := now.Add(-10 * time.Minute)
	if CheckFreshness(&boundary, &received, now, 5*time.Minute, false).Stale {
		t.Fatal("exactly two intervals should not be stale")
	}
	if !CheckFreshness(&sample, &received, now, 5*time.Minute, true).Stale {
		t.Fatal("failed refresh should be stale")
	}
}

func TestResetLabels(t *testing.T) {
	now := time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
	reset := now.Add(2*time.Hour + 14*time.Minute)
	if got := ResetLabel(&reset, now); got != "Resets in 2h 14m" {
		t.Fatalf("label = %q", got)
	}
	due := now.Add(-time.Minute)
	if got := ResetLabel(&due, now); got != "Reset due · awaiting reading" {
		t.Fatalf("due = %q", got)
	}
	if got := ResetLabel(nil, now); got != "Reset not reported" {
		t.Fatalf("unknown = %q", got)
	}
}

func TestDeriveState(t *testing.T) {
	now := time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		snap UsageSnapshot
		want SourceStatus
	}{
		{"current", UsageSnapshot{Windows: []AllowanceWindow{{UsedPercent: percent(28)}}}, StatusCurrent},
		{"low", UsageSnapshot{Windows: []AllowanceWindow{{UsedPercent: percent(82)}}}, StatusLow},
		{"exhausted", UsageSnapshot{Windows: []AllowanceWindow{{UsedPercent: percent(100)}}}, StatusExhausted},
		{"missing", UsageSnapshot{Windows: []AllowanceWindow{{Label: "5h"}}}, StatusMissingWindow},
		{"no windows", UsageSnapshot{}, StatusMissingWindow},
		{"partial", UsageSnapshot{Quality: QualityPartial, Windows: []AllowanceWindow{{UsedPercent: percent(50)}}}, StatusPartial},
		{"unsupported", UsageSnapshot{Quality: QualityUnsupported}, StatusBackendIncompatible},
		{"quality stale", UsageSnapshot{Quality: QualityStale, Windows: []AllowanceWindow{{UsedPercent: percent(50)}}}, StatusStale},
		{"quality unavailable", UsageSnapshot{Quality: QualityUnavailable, Windows: []AllowanceWindow{{UsedPercent: percent(50)}}}, StatusUnavailable},
		{"balance only measured", UsageSnapshot{Balances: []Balance{{Label: "Credits", Value: percent(0)}}}, StatusCurrent},
		{"balance only unavailable", UsageSnapshot{Balances: []Balance{{Label: "Credits"}}}, StatusUnavailable},
		{"unit allowance", UsageSnapshot{Windows: []AllowanceWindow{{Used: percent(2), Limit: percent(10), Unit: "requests"}}}, StatusCurrent},
		{"non-finite unit", UsageSnapshot{Windows: []AllowanceWindow{{Used: percent(math.NaN()), Unit: "requests"}}}, StatusMissingWindow},
		{"infinite balance", UsageSnapshot{Balances: []Balance{{Value: percent(math.Inf(1))}}}, StatusUnavailable},
		{"measured zero balance", UsageSnapshot{Balances: []Balance{{Value: percent(0)}}}, StatusCurrent},
		{"stale window", UsageSnapshot{Windows: []AllowanceWindow{{UsedPercent: percent(50), Quality: QualityStale}}}, StatusStale},
		{"auth no prior", UsageSnapshot{Error: &ScopedError{Category: ErrAuth}}, StatusSignIn},
		{"auth retained", UsageSnapshot{SampleAt: &now, Windows: []AllowanceWindow{{UsedPercent: percent(50)}}, Error: &ScopedError{Category: ErrAuth}}, StatusAuthFailed},
		{"timeout retained", UsageSnapshot{SampleAt: &now, Windows: []AllowanceWindow{{UsedPercent: percent(50)}}, Error: &ScopedError{Category: ErrTimeout}}, StatusStale},
		{"timeout no prior", UsageSnapshot{Error: &ScopedError{Category: ErrTimeout}}, StatusError},
	}
	for _, tc := range cases {
		if got := DeriveState(tc.snap, false); got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}

func TestCountStatesOverlap(t *testing.T) {
	states := []SourceStatus{StatusCurrent, StatusLow, StatusExhausted, StatusStale, StatusSignIn, StatusUnavailable}
	var snapshots []UsageSnapshot
	for _, state := range states {
		snap := UsageSnapshot{State: state}
		switch state {
		case StatusCurrent:
			snap.Windows = []AllowanceWindow{{UsedPercent: percent(40)}}
		case StatusLow:
			snap.Windows = []AllowanceWindow{{UsedPercent: percent(90)}}
		case StatusExhausted:
			snap.Windows = []AllowanceWindow{{UsedPercent: percent(100)}}
		}
		snapshots = append(snapshots, snap)
	}
	counts := CountStates(snapshots)
	if counts.Total != 6 || counts.Current != 3 || counts.Attention != 5 || counts.Low != 1 {
		t.Fatalf("counts = %+v", counts)
	}
}

func TestSourceOrderingUsesStableTieBreak(t *testing.T) {
	a := []SourceDescriptor{{DisplayName: "Codex", Key: SourceKey{Provider: "z"}}, {DisplayName: "codex", Key: SourceKey{Provider: "a"}}}
	b := []SourceDescriptor{a[1], a[0]}
	SortSources(a)
	SortSources(b)
	if a[0].Key.String() != b[0].Key.String() || a[1].Key.String() != b[1].Key.String() {
		t.Fatalf("orders differ: %v vs %v", a, b)
	}
	if a[0].DisplayName != "Codex" || a[0].Key.Provider != "z" {
		t.Fatalf("tie-break order = %+v", a)
	}
}

func TestSortSnapshotsPinnedThenName(t *testing.T) {
	snapshots := []UsageSnapshot{
		{Key: SourceKey{Provider: "zed"}, DisplayName: "Zed"},
		{Key: SourceKey{Provider: "codex"}, DisplayName: "Codex"},
		{Key: SourceKey{Provider: "claude"}, DisplayName: "Claude"},
	}
	SortSnapshots(snapshots, map[string]int{"codex": 1})
	if snapshots[0].DisplayName != "Codex" {
		t.Fatalf("first = %s", snapshots[0].DisplayName)
	}
	if snapshots[1].DisplayName != "Claude" || snapshots[2].DisplayName != "Zed" {
		t.Fatalf("order = %v", snapshots)
	}
}

func TestFilter(t *testing.T) {
	snapshot := UsageSnapshot{State: StatusLow, DisplayName: "Google · Gemini app", Key: SourceKey{Provider: "gemini", Variant: VariantWeb}}
	if !Filter(snapshot, "attention", "gemini", false) {
		t.Fatal("attention/search should match")
	}
	if Filter(snapshot, "attention", "codex", false) {
		t.Fatal("query mismatch should not match")
	}
	if Filter(snapshot, "pinned", "", false) {
		t.Fatal("unpinned should not match pinned")
	}
}

func TestErrorCategoriesMapToUXStates(t *testing.T) {
	cases := map[ErrorCategory]SourceStatus{
		ErrAuth:           StatusSignIn,
		ErrTimeout:        StatusStale,
		ErrProcessFailure: StatusStale,
		ErrUnavailable:    StatusUnavailable,
		ErrUnsupported:    StatusBackendIncompatible,
		ErrMalformedJSON:  StatusError,
		ErrEmptyStdout:    StatusError,
		ErrOutputCap:      StatusError,
	}
	for category, want := range cases {
		if got := category.UXState(); got != want {
			t.Fatalf("%s maps to %s want %s", category, got, want)
		}
		if category.UserMessage() == "" {
			t.Fatalf("%s has no user message", category)
		}
	}
}

func TestHistoryOptionalConcepts(t *testing.T) {
	zero := int64(0)
	priced := HistorySnapshot{
		Timezone:    "UTC",
		TotalTokens: &zero,
		Cost:        &Money{Amount: 0, Currency: "USD"},
		Priced:      true,
		Coverage:    "partial",
		Partial:     true,
		Incomplete:  2,
	}
	tokenOnly := HistorySnapshot{Timezone: "UTC", TotalTokens: &zero, Cost: nil, Priced: false}
	if priced.Cost == nil || !priced.Priced || priced.Timezone != "UTC" || priced.Incomplete != 2 {
		t.Fatalf("priced history lost concepts: %+v", priced)
	}
	if tokenOnly.Cost != nil || tokenOnly.Priced {
		t.Fatalf("token-only history invented a cost: %+v", tokenOnly)
	}
}

func TestAttentionFilterAgreesWithCounts(t *testing.T) {
	snapshot := UsageSnapshot{DisplayName: "Claude", Windows: []AllowanceWindow{{UsedPercent: percent(90)}}}
	counts := CountStates([]UsageSnapshot{snapshot})
	if counts.Attention != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	if !Filter(snapshot, "attention", "", false) {
		t.Fatal("attention filter disagrees with counts")
	}
}

func TestScopedErrorThroughBackendSeam(t *testing.T) {
	err := error(NewScopedError(ErrAuth, "codex:configured", "secret raw stderr"))
	var scoped *ScopedError
	if !errors.As(err, &scoped) {
		t.Fatal("typed error not recoverable")
	}
	if scoped.Category != ErrAuth || scoped.SourceKey != "codex:configured" {
		t.Fatalf("scoped = %+v", scoped)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked detail: %q", err.Error())
	}
	recovered, ok := AsScopedError(err)
	if !ok || recovered.Category != ErrAuth {
		t.Fatal("AsScopedError failed")
	}
}

func TestExplicitStateCannotHideErrorPrecedence(t *testing.T) {
	now := nowForTest()
	snapshot := UsageSnapshot{
		State:    StatusCurrent,
		SampleAt: &now,
		Windows:  []AllowanceWindow{{UsedPercent: percent(40)}},
		Error:    &ScopedError{Category: ErrAuth},
	}
	if got := EffectiveState(snapshot); got != StatusAuthFailed {
		t.Fatalf("state = %s", got)
	}
	counts := CountStates([]UsageSnapshot{snapshot})
	if counts.Attention != 1 || counts.Current != 0 {
		t.Fatalf("counts = %+v", counts)
	}
	if !Filter(snapshot, "attention", "", false) {
		t.Fatal("retained auth failure must appear in attention")
	}
}

func TestCapabilityUnknownIsConservative(t *testing.T) {
	unknown := Capabilities{}
	if unknown.Usage.Supported() || unknown.BulkTrackable(KindSubscription) {
		t.Fatal("unknown capability must not imply support")
	}
	supported := Capabilities{Usage: CapabilitySupported, ActiveProbe: CapabilityUnsupported}
	if !supported.BulkTrackable(KindSubscription) || supported.BulkTrackable(KindMeteredAPI) {
		t.Fatal("bulk eligibility is wrong")
	}
	unknownProbe := Capabilities{Usage: CapabilitySupported}
	if unknownProbe.BulkTrackable(KindSubscription) {
		t.Fatal("unknown probe behavior must not be bulk-trackable")
	}
	probe := Capabilities{Usage: CapabilitySupported, ActiveProbe: CapabilitySupported}
	if probe.BulkTrackable(KindSubscription) {
		t.Fatal("active probe must not be bulk-trackable")
	}
	unsupported := Capabilities{Usage: CapabilityUnsupported}
	if unsupported.BulkTrackable(KindMixed) {
		t.Fatal("confirmed unsupported usage must not be bulk-trackable")
	}
}

func nowForTest() time.Time {
	return time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC)
}

func TestWindowQualityOverridesExplicitCurrent(t *testing.T) {
	cases := map[Quality]SourceStatus{
		QualityStale:       StatusStale,
		QualityUnavailable: StatusUnavailable,
		QualityUnsupported: StatusBackendIncompatible,
		QualityPartial:     StatusPartial,
	}
	for quality, want := range cases {
		snapshot := UsageSnapshot{
			State:   StatusCurrent,
			Windows: []AllowanceWindow{{UsedPercent: percent(40), Quality: quality}},
		}
		if got := EffectiveState(snapshot); got != want {
			t.Fatalf("%s: effective=%s want=%s", quality, got, want)
		}
		counts := CountStates([]UsageSnapshot{snapshot})
		if want.NeedsAttention() && (counts.Attention != 1 || !Filter(snapshot, "attention", "", false)) {
			t.Fatalf("%s: counts=%+v filter=false", quality, counts)
		}
	}
}

func TestC1ControlAccountEscaping(t *testing.T) {
	raw := "codex:configured@a%C2%9Bb"
	key, err := ParseSourceKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := key.String(); got != raw {
		t.Fatalf("round trip = %q", got)
	}
	if _, err := ParseSourceKey("codex:configured@a\u009bb"); err == nil {
		t.Fatal("raw C1 control must be rejected")
	}
}

func TestExplicitMeasurementStateCannotHideWindowQuality(t *testing.T) {
	cases := []struct {
		state  SourceStatus
		window AllowanceWindow
		want   SourceStatus
	}{
		{StatusLow, AllowanceWindow{UsedPercent: percent(80), Quality: QualityStale}, StatusStale},
		{StatusExhausted, AllowanceWindow{UsedPercent: percent(100), Quality: QualityUnavailable}, StatusUnavailable},
		{StatusLow, AllowanceWindow{UsedPercent: percent(80), Quality: QualityPartial}, StatusPartial},
	}
	for _, tc := range cases {
		snapshot := UsageSnapshot{State: tc.state, Windows: []AllowanceWindow{tc.window}}
		if got := EffectiveState(snapshot); got != tc.want {
			t.Fatalf("%s got %s want %s", tc.state, got, tc.want)
		}
		counts := CountStates([]UsageSnapshot{snapshot})
		if counts.Attention != 1 || !Filter(snapshot, "attention", "", false) {
			t.Fatalf("%s: counts=%+v filter=false", tc.state, counts)
		}
	}
}

func TestSchedulingStatesSurviveWithoutData(t *testing.T) {
	for _, state := range []SourceStatus{StatusQueued, StatusFetching} {
		snapshot := UsageSnapshot{State: state}
		if got := EffectiveState(snapshot); got != state {
			t.Fatalf("%s became %s", state, got)
		}
		counts := CountStates([]UsageSnapshot{snapshot})
		if state == StatusQueued && counts.Queued != 1 {
			t.Fatalf("queued counts = %+v", counts)
		}
		if state == StatusFetching && counts.Fetching != 1 {
			t.Fatalf("fetching counts = %+v", counts)
		}
		if Filter(snapshot, "attention", "", false) {
			t.Fatalf("%s should not be attention", state)
		}
	}
}

func TestMissingSiblingWindowMarksPartial(t *testing.T) {
	snapshot := UsageSnapshot{Windows: []AllowanceWindow{
		{Label: "5h", UsedPercent: percent(40)},
		{Label: "Weekly"},
	}}
	if got := EffectiveState(snapshot); got != StatusPartial {
		t.Fatalf("state = %s", got)
	}
	if !Filter(snapshot, "attention", "", false) || CountStates([]UsageSnapshot{snapshot}).Attention != 1 {
		t.Fatal("missing sibling should be attention")
	}
}

func TestCompletenessChecksBeforeExplicitState(t *testing.T) {
	snapshot := UsageSnapshot{State: StatusLow, Windows: []AllowanceWindow{
		{Label: "5h", UsedPercent: percent(80)},
		{Label: "Weekly"},
	}}
	if got := EffectiveState(snapshot); got != StatusPartial {
		t.Fatalf("effective = %s", got)
	}
	if got := DeriveState(snapshot, false); got != StatusPartial {
		t.Fatalf("derived = %s", got)
	}
	if counts := CountStates([]UsageSnapshot{snapshot}); counts.Attention != 1 {
		t.Fatalf("counts = %+v", counts)
	}
}

func TestMixedBalancesArePartial(t *testing.T) {
	zero := 0.0
	cases := []UsageSnapshot{
		{Balances: []Balance{{Value: &zero}, {Label: "Unavailable"}}},
		{Windows: []AllowanceWindow{{UsedPercent: percent(40)}}, Balances: []Balance{{Value: &zero}, {Label: "Unavailable"}}},
	}
	for _, snapshot := range cases {
		if got := EffectiveState(snapshot); got != StatusPartial {
			t.Fatalf("state = %s for %+v", got, snapshot)
		}
		if !Filter(snapshot, "attention", "", false) {
			t.Fatal("partial mixture should need attention")
		}
	}
}

func TestEmptyDetailsAreNotReadings(t *testing.T) {
	empty := UsageSnapshot{Details: []DetailSection{{Title: "Extras"}}}
	if got := EffectiveState(empty); got != StatusMissingWindow {
		t.Fatalf("empty details state = %s", got)
	}
	auth := UsageSnapshot{Details: []DetailSection{{Title: "Extras"}}, Error: &ScopedError{Category: ErrAuth}}
	if got := EffectiveState(auth); got != StatusSignIn {
		t.Fatalf("empty details auth state = %s", got)
	}
	populated := UsageSnapshot{Details: []DetailSection{{Title: "Extras", Rows: []DetailRow{{Label: "A", Value: "B"}}}}}
	if got := EffectiveState(populated); got != StatusCurrent {
		t.Fatalf("populated details state = %s", got)
	}
	retained := UsageSnapshot{Details: populated.Details, Error: &ScopedError{Category: ErrAuth}}
	if got := EffectiveState(retained); got != StatusAuthFailed {
		t.Fatalf("retained details auth state = %s", got)
	}
	unsupported := UsageSnapshot{Details: []DetailSection{{Title: "Chart", Unsupported: true}}}
	if got := EffectiveState(unsupported); got != StatusBackendIncompatible {
		t.Fatalf("unsupported details state = %s", got)
	}
}

func TestExplicitStateCompletenessRegression(t *testing.T) {
	zero := 0.0
	mixedBalances := []Balance{{Value: &zero}, {Label: "Unavailable"}}
	lowWithMixed := UsageSnapshot{
		State:    StatusLow,
		Windows:  []AllowanceWindow{{UsedPercent: percent(80)}},
		Balances: mixedBalances,
	}
	if got := EffectiveState(lowWithMixed); got != StatusPartial {
		t.Fatalf("low with mixed balances = %s", got)
	}
	emptyExhausted := UsageSnapshot{State: StatusExhausted}
	if got := EffectiveState(emptyExhausted); got != StatusMissingWindow {
		t.Fatalf("empty exhausted = %s", got)
	}
	partialWindow := UsageSnapshot{Windows: []AllowanceWindow{{Quality: QualityPartial}}}
	if got := EffectiveState(partialWindow); got != StatusPartial {
		t.Fatalf("partial window effective = %s", got)
	}
	if got := DeriveState(partialWindow, false); got != StatusPartial {
		t.Fatalf("partial window derived = %s", got)
	}
}

func TestUnsupportedDetailFamiliesDoNotHideSupportedReadings(t *testing.T) {
	supported := DetailSection{Title: "Extras", Rows: []DetailRow{{Label: "A", Value: "B"}}}
	unsupported := DetailSection{Title: "Chart", Unsupported: true}
	mixed := UsageSnapshot{Details: []DetailSection{supported, unsupported}}
	if got := EffectiveState(mixed); got != StatusCurrent {
		t.Fatalf("mixed details = %s", got)
	}
	onlyUnsupported := UsageSnapshot{Details: []DetailSection{unsupported}}
	if got := EffectiveState(onlyUnsupported); got != StatusBackendIncompatible {
		t.Fatalf("only unsupported = %s", got)
	}
	withWindow := UsageSnapshot{Windows: []AllowanceWindow{{UsedPercent: percent(40)}}, Details: []DetailSection{unsupported}}
	if got := EffectiveState(withWindow); got != StatusCurrent {
		t.Fatalf("window with unsupported detail = %s", got)
	}
}

func TestMixedMetricFamiliesArePartial(t *testing.T) {
	zero := 0.0
	detail := DetailSection{Title: "Extras", Rows: []DetailRow{{Label: "A", Value: "B"}}}
	cases := map[string]UsageSnapshot{
		"supported detail plus unavailable balance": {
			Details:  []DetailSection{detail},
			Balances: []Balance{{Label: "Credits"}},
		},
		"missing window plus measured credits": {
			Windows:  []AllowanceWindow{{Label: "5h"}},
			Balances: []Balance{{Label: "Credits", Value: &zero}},
		},
		"unavailable window plus supported detail": {
			Windows: []AllowanceWindow{{Label: "5h", Quality: QualityUnavailable}},
			Details: []DetailSection{detail},
		},
	}
	for name, snapshot := range cases {
		if got := DeriveState(snapshot, false); got != StatusPartial {
			t.Fatalf("%s: derived = %s", name, got)
		}
		if got := EffectiveState(snapshot); got != StatusPartial {
			t.Fatalf("%s: effective = %s", name, got)
		}
		if counts := CountStates([]UsageSnapshot{snapshot}); counts.Attention != 1 || counts.Current != 0 {
			t.Fatalf("%s: counts = %+v", name, counts)
		}
		if !Filter(snapshot, "attention", "", false) {
			t.Fatalf("%s: not in attention", name)
		}
	}
}

func TestDetailNumericOptionality(t *testing.T) {
	zero, nan := 0.0, math.NaN()
	zeroBar := DetailBar{Label: "A", Value: &zero}
	missingBar := DetailBar{Label: "B"}
	nanPoint := ChartPoint{Label: "C", Value: &nan}
	zeroPoint := ChartPoint{Label: "D", Value: &zero}
	cases := map[string]struct {
		snapshot UsageSnapshot
		want     SourceStatus
	}{
		"measured zero bar":  {UsageSnapshot{Details: []DetailSection{{Title: "Bars", Bars: []DetailBar{zeroBar}}}}, StatusCurrent},
		"missing bar only":   {UsageSnapshot{Details: []DetailSection{{Title: "Bars", Bars: []DetailBar{missingBar}}}}, StatusUnavailable},
		"invalid point only": {UsageSnapshot{Details: []DetailSection{{Title: "Chart", Points: []ChartPoint{nanPoint}}}}, StatusUnavailable},
		"valid row plus invalid bar": {
			UsageSnapshot{Details: []DetailSection{{Title: "Mixed", Rows: []DetailRow{{Label: "A", Value: "B"}}, Bars: []DetailBar{missingBar}}}},
			StatusPartial,
		},
		"valid row plus measured zero": {
			UsageSnapshot{Details: []DetailSection{{Title: "Mixed", Rows: []DetailRow{{Label: "A", Value: "B"}}, Points: []ChartPoint{zeroPoint}}}},
			StatusCurrent,
		},
	}
	for name, tc := range cases {
		if got := DeriveState(tc.snapshot, false); got != tc.want {
			t.Fatalf("%s: derived = %s want %s", name, got, tc.want)
		}
		if got := EffectiveState(tc.snapshot); got != tc.want {
			t.Fatalf("%s: effective = %s want %s", name, got, tc.want)
		}
		counts := CountStates([]UsageSnapshot{tc.snapshot})
		if tc.want.NeedsAttention() {
			if counts.Attention != 1 || !Filter(tc.snapshot, "attention", "", false) {
				t.Fatalf("%s: counts=%+v filter=false", name, counts)
			}
		}
	}
}

func TestUnknownAccountSentinelDoesNotCollide(t *testing.T) {
	sentinel, err := ParseSourceKey("codex:configured@unknown")
	if err != nil {
		t.Fatal(err)
	}
	if !sentinel.AccountUnknown || sentinel.String() != "codex:configured@unknown" {
		t.Fatalf("sentinel = %+v key=%q", sentinel, sentinel.String())
	}
	known, err := ParseSourceKey("codex:configured@%75nknown")
	if err != nil {
		t.Fatal(err)
	}
	if known.AccountUnknown || known.Account != "unknown" {
		t.Fatalf("known = %+v", known)
	}
	if known.String() != "codex:configured@%75nknown" {
		t.Fatalf("known canonical = %q", known.String())
	}
	if known.CoalescesWith(sentinel) {
		t.Fatal("known account unknown must not coalesce with missing identity")
	}
	constructed := SourceKey{Provider: "codex", Variant: VariantConfigured, Account: "unknown"}
	if constructed.String() != "codex:configured@%75nknown" {
		t.Fatalf("constructed canonical = %q", constructed.String())
	}
}

func TestUnavailableDetailsWithWindowsArePartial(t *testing.T) {
	snapshot := UsageSnapshot{
		Windows: []AllowanceWindow{{UsedPercent: percent(40)}},
		Details: []DetailSection{{Title: "Bars", Bars: []DetailBar{{Label: "Missing"}}}},
	}
	if got := DeriveState(snapshot, false); got != StatusPartial {
		t.Fatalf("derived = %s", got)
	}
	if counts := CountStates([]UsageSnapshot{snapshot}); counts.Attention != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	if !Filter(snapshot, "attention", "", false) {
		t.Fatal("not in attention")
	}
}

func TestFilterIsUnicodeCaseInsensitive(t *testing.T) {
	snapshot := UsageSnapshot{State: StatusCurrent, DisplayName: "Éclair"}
	if !Filter(snapshot, "", "éclair", false) {
		t.Fatal("unicode case-insensitive search failed")
	}
}
