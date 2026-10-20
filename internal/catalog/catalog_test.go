package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/domain"
)

func TestManifestCoversReferenceCatalog(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "codexbar-0.77.0-providers.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Providers []struct {
			Provider string `json:"provider"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(data, &ref); err != nil {
		t.Fatal(err)
	}
	if len(ref.Providers) != 106 || len(manifest) != 106 {
		t.Fatalf("reference=%d manifest=%d", len(ref.Providers), len(manifest))
	}
	seen := map[string]bool{}
	for _, provider := range ref.Providers {
		seen[provider.Provider] = true
		if _, ok := manifest[provider.Provider]; !ok {
			t.Fatalf("missing manifest entry %s", provider.Provider)
		}
	}
	for id := range manifest {
		if !seen[id] {
			t.Fatalf("manifest contains unknown provider %s", id)
		}
	}
	if manifest["azureopenai"].ActiveProbe != domain.CapabilitySupported || manifest["azureopenai"].ProbeReason == "" {
		t.Fatal("azureopenai must remain an active probe with a reason")
	}
}

func TestApplyRetiresGeminiGenericAndSplitsVariants(t *testing.T) {
	catalog := backend.CatalogSnapshot{Providers: []domain.SourceDescriptor{
		{Key: domain.SourceKey{Provider: "gemini"}, ProviderID: "gemini", DisplayName: "Gemini", Available: true, Enabled: true},
		{Key: domain.SourceKey{Provider: "codex"}, ProviderID: "codex", DisplayName: "Codex", Available: true, Enabled: true},
	}}
	applied := Apply(catalog)
	geminiKeys := map[string]bool{}
	for _, descriptor := range applied.Providers {
		if descriptor.ProviderID == "gemini" {
			geminiKeys[descriptor.Key.String()] = true
		}
	}
	if geminiKeys["gemini"] || !geminiKeys["gemini:configured"] || !geminiKeys["gemini:web"] || !geminiKeys["gemini:api"] || len(geminiKeys) != 3 {
		t.Fatalf("gemini variants=%v", geminiKeys)
	}
}

func TestBulkTrackingExcludesUnknownAPICreditsAndProbes(t *testing.T) {
	entries := Manifest()
	cases := []struct {
		id   string
		kind domain.ServiceKind
		want bool
	}{
		{"codex", domain.KindSubscription, true},
		{"gemini", domain.KindSubscription, true},
		{"azureopenai", domain.KindMeteredAPI, false},
		{"openai", domain.KindMeteredAPI, false},
		{"openrouter", domain.KindCredits, false},
		{"ollama", domain.KindNonUsage, false},
		{"unknown-provider", domain.KindUnknown, false},
	}
	for _, tc := range cases {
		entry, ok := entries[tc.id]
		if !ok {
			entry = Entry{ProviderID: tc.id, Kind: domain.KindUnknown, ActiveProbe: domain.CapabilityUnknown}
		}
		descriptor := domain.SourceDescriptor{ProviderID: tc.id, Kind: entry.Kind, Available: true, Capabilities: domain.Capabilities{
			Usage:       domain.CapabilityUnknown,
			ActiveProbe: entry.ActiveProbe,
		}}
		if entry.Kind == domain.KindSubscription || entry.Kind == domain.KindMixed {
			descriptor.Capabilities.Usage = domain.CapabilitySupported
		}
		if got := BulkTrackable(descriptor); got != tc.want {
			t.Fatalf("%s bulk=%v want=%v", tc.id, got, tc.want)
		}
	}
}

func TestSeedTrackedOnlyEnabledCompatible(t *testing.T) {
	snapshot := backend.CatalogSnapshot{Providers: []domain.SourceDescriptor{
		{Key: domain.SourceKey{Provider: "codex"}, ProviderID: "codex", Kind: domain.KindSubscription, Enabled: true, Available: true, Capabilities: domain.Capabilities{Usage: domain.CapabilitySupported, ActiveProbe: domain.CapabilityUnsupported}},
		{Key: domain.SourceKey{Provider: "openai"}, ProviderID: "openai", Kind: domain.KindMeteredAPI, Enabled: true, Available: true, Capabilities: domain.Capabilities{Usage: domain.CapabilityUnknown}},
		{Key: domain.SourceKey{Provider: "claude"}, ProviderID: "claude", Kind: domain.KindSubscription, Enabled: false, Available: true, Capabilities: domain.Capabilities{Usage: domain.CapabilitySupported, ActiveProbe: domain.CapabilityUnsupported}},
	}}
	keys := SeedTracked(snapshot)
	if len(keys) != 1 || keys[0] != "codex" {
		t.Fatalf("seed=%v", keys)
	}
}

func TestReviewLogCoversEveryManifestEntry(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "evidence", "catalog-review.md"))
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	if !strings.Contains(log, "CodexBar version: 0.77.0") || !strings.Contains(log, "Reviewer:") || !strings.Contains(log, "Date:") {
		t.Fatal("review log header missing reviewer/date/version")
	}
	for id := range manifest {
		if !strings.Contains(log, "| `"+id+"` |") {
			t.Fatalf("manifest entry %s missing from review log", id)
		}
	}
}

func TestAllClassifiedSubscriptionsAreBulkTrackableStress(t *testing.T) {
	var providers []domain.SourceDescriptor
	for id, entry := range manifest {
		providers = append(providers, domain.SourceDescriptor{
			Key: domain.SourceKey{Provider: id}, ProviderID: id, DisplayName: entry.DisplayName,
			Kind: entry.Kind, Available: true, Enabled: true,
			Capabilities: domain.Capabilities{Usage: domain.CapabilityUnknown, ActiveProbe: entry.ActiveProbe},
		})
	}
	applied := Apply(backend.CatalogSnapshot{Providers: providers})
	subscriptions := 0
	for _, descriptor := range applied.Providers {
		if descriptor.Kind != domain.KindSubscription && descriptor.Kind != domain.KindMixed {
			continue
		}
		subscriptions++
		if !BulkTrackable(descriptor) {
			t.Fatalf("classified subscription %s is not bulk-trackable: %+v", descriptor.Key, descriptor.Capabilities)
		}
	}
	if subscriptions == 0 || subscriptions > len(applied.Providers) {
		t.Fatalf("subscription stress count=%d", subscriptions)
	}
}

func TestReviewLogMatchesManifestAnnotations(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "evidence", "catalog-review.md"))
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	for id, entry := range manifest {
		line := ""
		for _, candidate := range strings.Split(log, "\n") {
			if strings.HasPrefix(candidate, "| `"+id+"` |") {
				line = candidate
				break
			}
		}
		if line == "" {
			t.Fatalf("missing row %s", id)
		}
		if !strings.Contains(line, "| "+strings.ToLower(string(entry.Kind))+" |") ||
			!strings.Contains(line, "| "+strings.ToLower(string(entry.History))+" |") ||
			!strings.Contains(line, "| "+strings.ToLower(string(entry.ActiveProbe))+" |") {
			t.Fatalf("row annotations disagree with manifest for %s: %s", id, line)
		}
		if entry.ProbeReason != "" && !strings.Contains(line, entry.ProbeReason) {
			t.Fatalf("probe reason missing for %s", id)
		}
	}
}

func TestGeminiVariantPresentationNamesDistinct(t *testing.T) {
	applied := Apply(backend.CatalogSnapshot{Providers: []domain.SourceDescriptor{{Key: domain.SourceKey{Provider: "gemini"}, ProviderID: "gemini", DisplayName: "Gemini", Available: true, Enabled: true}}})
	names := map[string]bool{}
	for _, descriptor := range applied.Providers {
		names[descriptor.DisplayName] = true
	}
	if len(names) != 3 || !names["Google · Gemini app"] || !names["Google · Gemini CLI"] || !names["Google · Gemini (configured)"] {
		t.Fatalf("variant names=%v", names)
	}
}
