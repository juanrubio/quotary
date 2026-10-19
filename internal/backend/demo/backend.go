package demo

import (
	"context"
	"fmt"
	"hash/fnv"
	"io/fs"
	"sort"
	"time"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/domain"
)

// Scenario selects one deterministic demo collection.
type Scenario string

const (
	ScenarioSubscription Scenario = "subscription"
	ScenarioAttention    Scenario = "attention"
	ScenarioCatalog      Scenario = "catalog"
	ScenarioEmpty        Scenario = "empty"
	ScenarioNoResults    Scenario = "noresults"
	ScenarioSetup        Scenario = "setup"
)

// Backend is the offline fictional backend. It never reads user configuration,
// credentials, sessions, or network resources.
type Backend struct {
	scenario Scenario
	fixtures fixtures
	clock    time.Time
}

// New builds a demo backend for scenario.
func New(scenario Scenario) (*Backend, error) {
	return newFromFS(scenario, nil)
}

func newFromFS(scenario Scenario, fsys fs.FS) (*Backend, error) {
	switch scenario {
	case ScenarioSubscription, ScenarioAttention, ScenarioCatalog, ScenarioEmpty, ScenarioNoResults, ScenarioSetup:
	default:
		return nil, fmt.Errorf("unknown demo scenario %q", scenario)
	}
	if fsys == nil {
		fsys = fixtureFS
	}
	data, err := loadFixturesFrom(fsys)
	if err != nil {
		return nil, err
	}
	clock, err := data.clock()
	if err != nil {
		return nil, err
	}
	return &Backend{scenario: scenario, fixtures: data, clock: clock}, nil
}

// MustNew builds a demo backend or panics. It is for tests and the CLI wiring.
func MustNew(scenario Scenario) *Backend {
	value, err := New(scenario)
	if err != nil {
		panic(err)
	}
	return value
}

// Now returns the fixed demo clock.
func (b *Backend) Now() time.Time { return b.clock }

// Version reports the fictional demo backend version.
func (b *Backend) Version(context.Context) (backend.VersionInfo, error) {
	return backend.VersionInfo{Version: "demo-0.1.0"}, nil
}

// Catalog returns the full demo catalog with scenario tracking applied.
func (b *Backend) Catalog(context.Context) (backend.CatalogSnapshot, error) {
	if b.scenario == ScenarioSetup {
		return backend.CatalogSnapshot{}, domain.NewScopedError(domain.ErrConfig, "", "Demo setup: no backend connection")
	}
	var providers []domain.SourceDescriptor
	for _, entry := range b.fixtures.Catalog {
		variants := []domain.Variant{domain.VariantConfigured}
		names := []string{entry.DisplayName}
		if entry.Provider == "gemini" {
			variants = []domain.Variant{domain.VariantWeb, domain.VariantAPI}
			names = []string{"Google · Gemini app", "Google · Gemini CLI"}
		}
		for i, variant := range variants {
			key := domain.SourceKey{Provider: entry.Provider, Variant: variant, Account: domain.UnknownAccount, AccountUnknown: true}
			kind := demoKind(entry.Provider)
			descriptor := domain.SourceDescriptor{
				Key:          key,
				ProviderID:   entry.Provider,
				DisplayName:  names[i],
				Variant:      variant,
				Kind:         kind,
				Capabilities: demoCapabilities(entry.Provider, kind),
				Available:    true,
				Tracked:      b.tracks(key, kind),
				LastRead:     domain.StatusUnknown,
			}
			if source, ok := b.baseSource(key); ok {
				descriptor.Pinned = source.Pinned && b.scenario != ScenarioCatalog && b.scenario != ScenarioEmpty
				descriptor.PinOrder = source.PinOrder
			}
			if snapshot, ok := b.usageFor(key.String()); ok {
				descriptor.LastRead = snapshot.State
			}
			providers = append(providers, descriptor)
		}
	}
	sort.SliceStable(providers, func(i, j int) bool {
		if providers[i].Pinned != providers[j].Pinned {
			return providers[i].Pinned
		}
		if providers[i].Pinned && providers[i].PinOrder != providers[j].PinOrder {
			return providers[i].PinOrder < providers[j].PinOrder
		}
		return providers[i].Key.String() < providers[j].Key.String()
	})
	return backend.CatalogSnapshot{Providers: providers}, nil
}

// Usage returns all demo records for one requested source.
func (b *Backend) Usage(_ context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	if b.scenario == ScenarioSetup {
		return nil, domain.NewScopedError(domain.ErrConfig, req.SourceKey, "Demo setup: no backend connection")
	}
	parsed, err := b.requestKey(req)
	if err != nil {
		return nil, err
	}
	snapshot, ok := b.usageFor(parsed.String())
	if !ok {
		return nil, domain.NewScopedError(domain.ErrUnavailable, parsed.String(), "source is not tracked in this demo scenario")
	}
	return []domain.UsageSnapshot{snapshot}, nil
}

func (b *Backend) requestKey(req backend.UsageRequest) (domain.SourceKey, error) {
	if req.SourceKey == "" {
		variant := req.Variant
		if variant == "" {
			variant = domain.VariantConfigured
		}
		key := domain.SourceKey{Provider: req.ProviderID, Variant: variant, Account: domain.UnknownAccount, AccountUnknown: true}
		if !b.validCatalogKey(key) {
			return domain.SourceKey{}, domain.NewScopedError(domain.ErrConfig, key.String(), "invalid demo source")
		}
		return key, nil
	}
	key, err := domain.ParseSourceKey(req.SourceKey)
	if err != nil {
		return domain.SourceKey{}, domain.NewScopedError(domain.ErrConfig, req.SourceKey, "invalid demo source")
	}
	if key.Account == "" {
		key.Account = domain.UnknownAccount
		key.AccountUnknown = true
	}
	if !key.AccountUnknown {
		return domain.SourceKey{}, domain.NewScopedError(domain.ErrConfig, key.String(), "demo has no known account identities")
	}
	if req.ProviderID != "" && req.ProviderID != key.Provider {
		return domain.SourceKey{}, domain.NewScopedError(domain.ErrConfig, key.String(), "provider does not match source key")
	}
	if req.Variant != "" && req.Variant != key.Variant {
		return domain.SourceKey{}, domain.NewScopedError(domain.ErrConfig, key.String(), "variant does not match source key")
	}
	if !b.validCatalogKey(key) {
		return domain.SourceKey{}, domain.NewScopedError(domain.ErrConfig, key.String(), "demo source variant is not in the catalog")
	}
	return key, nil
}

func (b *Backend) validCatalogKey(key domain.SourceKey) bool {
	variants := b.variantsFor(key.Provider)
	if len(variants) == 0 {
		return false
	}
	for _, variant := range variants {
		if variant == key.Variant {
			return true
		}
	}
	return false
}

func (b *Backend) variantsFor(provider string) []domain.Variant {
	for _, entry := range b.fixtures.Catalog {
		if entry.Provider != provider {
			continue
		}
		if provider == "gemini" {
			return []domain.Variant{domain.VariantWeb, domain.VariantAPI}
		}
		return []domain.Variant{domain.VariantConfigured}
	}
	return nil
}

// History returns deterministic token history for sources with a demo history
// reader, or an unsupported error otherwise.
func (b *Backend) History(_ context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
	if b.scenario == ScenarioSetup {
		return domain.HistorySnapshot{}, domain.NewScopedError(domain.ErrConfig, req.SourceKey, "Demo setup: no backend connection")
	}
	key, err := b.historyKey(req)
	if err != nil {
		return domain.HistorySnapshot{}, err
	}
	days := req.Days
	if days != 30 {
		days = 7
	}
	if key.Provider == "muse" && days == 7 {
		history := b.fixtures.historySnapshot(b.clock)
		history.Key = key
		return history, nil
	}
	if !demoCapabilities(key.Provider, demoKind(key.Provider)).History.Supported() {
		return domain.HistorySnapshot{}, domain.NewScopedError(domain.ErrUnsupported, key.String(), "demo has no history reader for this source")
	}
	return genericHistory(key.String(), key.Provider, days, b.clock), nil
}

func (b *Backend) historyKey(req backend.HistoryRequest) (domain.SourceKey, error) {
	if req.SourceKey == "" {
		variant := req.Variant
		if variant == "" {
			variant = domain.VariantConfigured
		}
		key := domain.SourceKey{Provider: req.ProviderID, Variant: variant, Account: domain.UnknownAccount, AccountUnknown: true}
		if !b.validCatalogKey(key) {
			return domain.SourceKey{}, domain.NewScopedError(domain.ErrConfig, key.String(), "invalid demo history source")
		}
		return key, nil
	}
	key, err := domain.ParseSourceKey(req.SourceKey)
	if err != nil {
		return domain.SourceKey{}, domain.NewScopedError(domain.ErrConfig, req.SourceKey, "invalid demo history source")
	}
	if key.Account == "" {
		key.Account = domain.UnknownAccount
		key.AccountUnknown = true
	}
	if !key.AccountUnknown {
		return domain.SourceKey{}, domain.NewScopedError(domain.ErrConfig, key.String(), "demo has no known account identities")
	}
	if req.ProviderID != "" && req.ProviderID != key.Provider {
		return domain.SourceKey{}, domain.NewScopedError(domain.ErrConfig, key.String(), "provider does not match source key")
	}
	if req.Variant != "" && req.Variant != key.Variant {
		return domain.SourceKey{}, domain.NewScopedError(domain.ErrConfig, key.String(), "variant does not match source key")
	}
	if !b.validCatalogKey(key) {
		return domain.SourceKey{}, domain.NewScopedError(domain.ErrConfig, key.String(), "demo source variant is not in the catalog")
	}
	return key, nil
}

func (b *Backend) tracks(key domain.SourceKey, kind domain.ServiceKind) bool {
	switch b.scenario {
	case ScenarioEmpty:
		return false
	case ScenarioCatalog:
		return kind == domain.KindSubscription || kind == domain.KindMixed
	}
	_, ok := b.baseSource(key)
	return ok
}

func (b *Backend) baseSource(key domain.SourceKey) (fixtureSource, bool) {
	for _, source := range b.fixtures.Sources {
		parsed, err := domain.ParseSourceKey(source.ID)
		if err != nil {
			continue
		}
		if parsed.Provider == key.Provider && parsed.Variant == key.Variant {
			return source, true
		}
	}
	return fixtureSource{}, false
}

func (b *Backend) usageFor(key string) (domain.UsageSnapshot, bool) {
	if b.scenario == ScenarioEmpty {
		return domain.UsageSnapshot{}, false
	}
	if parsed, err := domain.ParseSourceKey(key); err == nil {
		if source, ok := b.baseSource(parsed); ok {
			snapshot := b.fixtures.snapshot(source, b.clock)
			if b.scenario == ScenarioAttention {
				snapshot = attentionVariant(snapshot, b.clock)
			}
			return snapshot, true
		}
	}
	if b.scenario != ScenarioCatalog {
		return domain.UsageSnapshot{}, false
	}
	parsed, err := domain.ParseSourceKey(key)
	if err != nil {
		return domain.UsageSnapshot{}, false
	}
	kind := demoKind(parsed.Provider)
	switch kind {
	case domain.KindSubscription, domain.KindMixed:
		return genericSnapshot(parsed, b.clock), true
	case domain.KindCredits:
		return genericCreditSnapshot(parsed, b.clock), true
	default:
		return domain.UsageSnapshot{}, false
	}
}

func genericCreditSnapshot(key domain.SourceKey, clock time.Time) domain.UsageSnapshot {
	amount := 12.5
	sample := clock.Add(-20 * time.Second)
	snapshot := domain.UsageSnapshot{
		Key:          key,
		ProviderID:   key.Provider,
		DisplayName:  demoName(key.Provider),
		AccountLabel: "Account unknown",
		AccountKey:   domain.UnknownAccount,
		Plan:         "Credits",
		SampleAt:     &sample,
		ReceivedAt:   clock,
		Balances:     []domain.Balance{{Label: "Credits", Value: &amount, Unit: "USD", Kind: "prepaid_money"}},
		Quality:      domain.QualityMeasured,
	}
	snapshot.State = domain.DeriveState(snapshot, false)
	return snapshot
}

func setSampleAge(snapshot *domain.UsageSnapshot, clock time.Time, age time.Duration) {
	sample := clock.Add(-age)
	snapshot.SampleAt = &sample
}

func attentionVariant(snapshot domain.UsageSnapshot, clock time.Time) domain.UsageSnapshot {
	switch snapshot.Key.Provider {
	case "antigravity":
		if len(snapshot.Windows) > 0 {
			used := 100.0
			snapshot.Windows[0].UsedPercent = &used
		}
		snapshot.State = domain.StatusExhausted
	case "kimi":
		snapshot.Windows = []domain.AllowanceWindow{{Key: "plan", Label: "Plan", Quality: domain.QualityPartial}}
		snapshot.Details = []domain.DetailSection{{Title: "Additional chart", Unsupported: true}}
		snapshot.State = domain.StatusPartial
		setSampleAge(&snapshot, clock, 18*time.Second)
	case "zed":
		snapshot.Windows = nil
		snapshot.Error = domain.NewScopedError(domain.ErrUnavailable, snapshot.Key.String(), "No entitlement reported")
		snapshot.State = domain.StatusUnavailable
		setSampleAge(&snapshot, clock, 16*time.Second)
	case "warp":
		if len(snapshot.Windows) > 0 {
			used := 88.0
			snapshot.Windows[0].UsedPercent = &used
		}
		snapshot.State = domain.StatusLow
	case "windsurf":
		snapshot.Windows = nil
		snapshot.Error = domain.NewScopedError(domain.ErrAuth, snapshot.Key.String(), "Demo login expired")
		snapshot.State = domain.StatusSignIn
		setSampleAge(&snapshot, clock, 18*time.Second)
	case "cursor":
		sample := clock.Add(-42 * time.Minute)
		snapshot.SampleAt = &sample
		snapshot.Quality = domain.QualityStale
		for i := range snapshot.Windows {
			snapshot.Windows[i].Quality = domain.QualityStale
		}
		snapshot.State = domain.StatusStale
	}
	return snapshot
}

func genericSnapshot(key domain.SourceKey, clock time.Time) domain.UsageSnapshot {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key.String()))
	left := 35 + float64(hash.Sum32()%60)
	used := 100 - left
	reset := clock.Add(time.Duration(5+hash.Sum32()%20) * 24 * time.Hour)
	sample := clock.Add(-20 * time.Second)
	window := domain.AllowanceWindow{
		Key:         "plan",
		Label:       "Plan",
		UsedPercent: &used,
		ResetAt:     &reset,
		Quality:     domain.QualityMeasured,
	}
	snapshot := domain.UsageSnapshot{
		Key:          key,
		ProviderID:   key.Provider,
		DisplayName:  demoName(key.Provider),
		AccountLabel: "Fictional account",
		Plan:         "Subscription",
		SampleAt:     &sample,
		ReceivedAt:   clock,
		Windows:      []domain.AllowanceWindow{window},
		Quality:      domain.QualityMeasured,
	}
	snapshot.State = domain.DeriveState(snapshot, false)
	return snapshot
}

func demoName(provider string) string {
	for _, entry := range demoCatalogNames {
		if entry.provider == provider {
			return entry.name
		}
	}
	return provider
}

type namedProvider struct {
	provider string
	name     string
}

var demoCatalogNames = []namedProvider{
	{"codex", "Codex"},
	{"openai", "OpenAI"},
	{"azureopenai", "Azure OpenAI"},
	{"claude", "Claude"},
	{"clinepass", "ClinePass"},
	{"cursor", "Cursor"},
	{"opencode", "OpenCode"},
	{"opencodego", "OpenCode Go"},
	{"alibaba", "Alibaba"},
	{"alibabatokenplan", "Alibaba Token Plan"},
	{"qwencloud", "Qwen Cloud"},
	{"factory", "Droid"},
	{"fireworks", "Fireworks"},
	{"gemini", "Gemini"},
	{"antigravity", "Antigravity"},
	{"copilot", "Copilot"},
	{"devin", "Devin"},
	{"zai", "z.ai / GLM"},
	{"minimax", "MiniMax"},
	{"manus", "Manus"},
	{"kimi", "Kimi Code"},
	{"kilo", "Kilo"},
	{"kiro", "Kiro"},
	{"vertexai", "Vertex AI"},
	{"augment", "Augment"},
	{"jetbrains", "JetBrains AI"},
	{"moonshot", "Moonshot / Kimi Open Platform"},
	{"amp", "Amp"},
	{"t3chat", "T3 Chat"},
	{"langdock", "Langdock"},
	{"ollama", "Ollama"},
	{"synthetic", "Synthetic"},
	{"openrouter", "OpenRouter"},
	{"elevenlabs", "ElevenLabs"},
	{"warp", "Warp"},
	{"windsurf", "Windsurf"},
	{"zed", "Zed"},
	{"perplexity", "Perplexity"},
	{"mimo", "Xiaomi MiMo"},
	{"doubao", "Doubao"},
	{"sakana", "Sakana AI"},
	{"abacus", "Abacus AI"},
	{"mistral", "Mistral"},
	{"deepseek", "DeepSeek"},
	{"deepinfra", "DeepInfra"},
	{"codebuff", "Codebuff"},
	{"venice", "Venice"},
	{"commandcode", "Command Code"},
	{"qoder", "Qoder"},
	{"stepfun", "StepFun"},
	{"bedrock", "AWS Bedrock"},
	{"grok", "Grok"},
	{"groq", "Groq"},
	{"llmproxy", "LLM Proxy"},
	{"litellm", "LiteLLM"},
	{"bifrost", "Bifrost"},
	{"aixy", "Aixy"},
	{"deepgram", "Deepgram"},
	{"poe", "Poe"},
	{"chutes", "Chutes"},
	{"neuralwatt", "Neuralwatt"},
	{"helmcode", "Helmcode"},
	{"clawrouter", "ClawRouter"},
	{"longcat", "LongCat"},
	{"sub2api", "sub2api"},
	{"wayfinder", "Wayfinder"},
	{"zenmux", "ZenMux"},
	{"aiand", "ai&"},
	{"zoommate", "ZoomMate"},
	{"xai", "xAI"},
	{"xapi", "X API"},
	{"notion", "Notion AI"},
	{"ibmbob", "IBM Bob"},
	{"nous", "Nous Portal"},
	{"muse", "Muse Code"},
	{"coderabbit", "CodeRabbit"},
	{"replicate", "Replicate"},
	{"huggingface", "Hugging Face"},
	{"raycast", "Raycast"},
	{"pi", "Pi"},
	{"v0", "v0"},
	{"typesafe", "TypeSafe"},
	{"hyper", "Charm Hyper"},
	{"gitkraken", "GitKraken AI"},
	{"futureagi", "Future AGI"},
	{"devpass", "DevPass"},
	{"atlascloud", "Atlas Cloud"},
	{"vercel", "Vercel AI Gateway"},
	{"llmman", "llmman"},
	{"xkiro", "xKiro"},
	{"museai", "Muse (muse.ai)"},
	{"lithosai", "LithosAI"},
	{"workbuddy", "WorkBuddy"},
	{"tavily", "Tavily"},
	{"linkup", "Linkup"},
	{"tinyapi", "TinyApi"},
	{"exa", "Exa"},
	{"cosmic", "Cosmic AI"},
	{"aerostack", "Aerostack"},
	{"sailresearch", "Sail Research"},
	{"sofya", "Sofya"},
	{"fal", "fal"},
	{"flora", "FLORA"},
	{"parallel", "Parallel"},
	{"claudeplatform", "Claude Platform"},
	{"munito", "Munito"},
}

var creditProviders = map[string]bool{
	"openrouter": true, "deepinfra": true, "replicate": true,
}

var apiProviders = map[string]bool{
	"openai": true, "azureopenai": true, "alibaba": true, "qwencloud": true,
	"fireworks": true, "vertexai": true, "moonshot": true, "ollama": true,
	"elevenlabs": true, "deepgram": true, "bedrock": true, "groq": true,
	"litellm": true, "llmproxy": true, "bifrost": true, "deepinfra": true,
	"replicate": true, "huggingface": true, "vercel": true, "deepseek": true,
	"tavily": true, "linkup": true, "exa": true, "fal": true, "parallel": true,
}

var subscriptionProviders = map[string]bool{
	"claude": true, "codex": true, "muse": true, "gemini": true, "antigravity": true,
	"cursor": true, "copilot": true, "windsurf": true, "kimi": true, "kiro": true,
	"jetbrains": true, "zed": true, "warp": true, "augment": true, "kilo": true,
	"factory": true, "opencodego": true, "zai": true, "minimax": true,
	"perplexity": true, "qoder": true, "clinepass": true, "devin": true,
	"manus": true, "amp": true, "t3chat": true, "langdock": true, "synthetic": true,
	"poe": true, "xai": true, "mistral": true, "v0": true,
	"raycast": true, "coderabbit": true, "hyper": true, "atlascloud": true,
	"museai": true, "workbuddy": true,
}

func demoKind(provider string) domain.ServiceKind {
	switch {
	case provider == "azureopenai":
		return domain.KindMeteredAPI
	case subscriptionProviders[provider]:
		return domain.KindSubscription
	case creditProviders[provider]:
		return domain.KindCredits
	case apiProviders[provider]:
		return domain.KindMeteredAPI
	default:
		return domain.KindUnknown
	}
}

func demoCapabilities(provider string, kind domain.ServiceKind) domain.Capabilities {
	caps := domain.Capabilities{
		Usage:       domain.CapabilityUnknown,
		History:     domain.CapabilityUnknown,
		Balances:    domain.CapabilityUnknown,
		Details:     domain.CapabilityUnknown,
		ActiveProbe: domain.CapabilityUnsupported,
	}
	if kind == domain.KindSubscription || kind == domain.KindMixed {
		caps.Usage = domain.CapabilitySupported
	}
	if kind == domain.KindCredits {
		caps.Balances = domain.CapabilitySupported
	}
	switch provider {
	case "claude", "codex", "muse", "antigravity":
		caps.History = domain.CapabilitySupported
	}
	if provider == "codex" {
		caps.Balances = domain.CapabilitySupported
	}
	if provider == "antigravity" {
		caps.Details = domain.CapabilitySupported
	}
	if provider == "azureopenai" {
		caps.ActiveProbe = domain.CapabilitySupported
	}
	return caps
}

func genericHistory(key, provider string, days int, clock time.Time) domain.HistorySnapshot {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	var total int64
	var buckets []domain.HistoryDay
	var missing []string
	for i := days - 1; i >= 0; i-- {
		day := clock.AddDate(0, 0, -i)
		if i%11 == 0 && i != 0 {
			buckets = append(buckets, domain.HistoryDay{Day: day.Format("2006-01-02")})
			missing = append(missing, day.Format("2006-01-02"))
			continue
		}
		tokens := int64(120000 + (hash.Sum32()+uint32(i*7919))%400000)
		total += tokens
		value := tokens
		buckets = append(buckets, domain.HistoryDay{Day: day.Format("2006-01-02"), Tokens: &value})
	}
	input, output := total*2/3, total/3
	result := domain.HistorySnapshot{
		Key:         domain.SourceKey{Provider: provider},
		Days:        days,
		Start:       clock.AddDate(0, 0, -(days - 1)),
		End:         clock,
		Timezone:    "UTC",
		Buckets:     buckets,
		TotalTokens: &total,
		Models:      []domain.ModelTotal{{Name: "Model A", Input: &input}, {Name: "Model B", Output: &output}},
		Currency:    "",
		Priced:      false,
		Provenance:  "Fictional local token history",
		Coverage:    "Partial coverage · demo data",
		Partial:     len(missing) > 0,
		Incomplete:  len(missing),
		MissingDays: missing,
	}
	if parsed, err := domain.ParseSourceKey(key); err == nil {
		result.Key = parsed
	}
	return result
}

// BackendError reports the simulated setup failure for the setup scenario.
func (b *Backend) BackendError() string {
	if b.scenario == ScenarioSetup {
		return "CodexBar could not be found. Set the executable in Settings, or add codexbar to PATH."
	}
	return ""
}
