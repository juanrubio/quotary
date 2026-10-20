package catalog

import (
	"sort"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/domain"
)

// Apply overlays the reviewed manifest on a backend catalog. Gemini becomes two
// stable variant sources and the generic provider key is retired.
func Apply(snapshot backend.CatalogSnapshot) backend.CatalogSnapshot {
	var result []domain.SourceDescriptor
	for _, descriptor := range snapshot.Providers {
		entry, ok := manifest[descriptor.ProviderID]
		if !ok {
			entry = Entry{ProviderID: descriptor.ProviderID, DisplayName: descriptor.DisplayName, Kind: domain.KindUnknown, History: domain.CapabilityUnknown, ActiveProbe: domain.CapabilityUnknown}
		}
		variants := entry.Variants
		if len(variants) == 0 {
			variants = []domain.Variant{""}
		}
		for _, variant := range variants {
			key := domain.SourceKey{Provider: descriptor.ProviderID, Variant: variant}
			displayName := descriptor.DisplayName
			if entry.DisplayName != "" {
				displayName = entry.DisplayName
			}
			if descriptor.ProviderID == "gemini" {
				switch variant {
				case domain.VariantConfigured:
					displayName = "Google · Gemini (configured)"
				case domain.VariantWeb:
					displayName = "Google · Gemini app"
				case domain.VariantAPI:
					displayName = "Google · Gemini CLI"
				}
			}
			capabilities := domain.Capabilities{
				Usage:          domain.CapabilityUnknown,
				History:        entry.History,
				Balances:       domain.CapabilityUnknown,
				Details:        domain.CapabilityUnknown,
				SourceVariants: append([]domain.Variant(nil), entry.Variants...),
				ActiveProbe:    entry.ActiveProbe,
			}
			if entry.Kind == domain.KindSubscription || entry.Kind == domain.KindMixed {
				capabilities.Usage = domain.CapabilitySupported
			}
			result = append(result, domain.SourceDescriptor{
				Key: key, ProviderID: descriptor.ProviderID,
				DisplayName: displayName, Variant: variant,
				Kind: entry.Kind, Capabilities: capabilities,
				Available: descriptor.Available,
				Enabled:   descriptor.Enabled, DefaultEnabled: descriptor.DefaultEnabled,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key.String() < result[j].Key.String() })
	return backend.CatalogSnapshot{Providers: result}
}

// BulkTrackable reports whether a descriptor may be included by bulk tracking.
func BulkTrackable(descriptor domain.SourceDescriptor) bool {
	return descriptor.Available && descriptor.Capabilities.BulkTrackable(descriptor.Kind)
}

// SeedTracked returns enabled, compatible subscription/mixed sources for a
// first run. It never includes API, credit, unknown, non-usage, or probe sources.
func SeedTracked(snapshot backend.CatalogSnapshot) []string {
	var keys []string
	for _, descriptor := range snapshot.Providers {
		if !descriptor.Enabled || !BulkTrackable(descriptor) {
			continue
		}
		if descriptor.ProviderID == "gemini" && descriptor.Variant != domain.VariantConfigured {
			continue
		}
		keys = append(keys, descriptor.Key.String())
	}
	sort.Strings(keys)
	return keys
}
