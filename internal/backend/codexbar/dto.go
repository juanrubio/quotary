package codexbar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/domain"
)

type catalogDTO struct {
	Providers []providerDTO `json:"providers"`
}

type providerDTO struct {
	Provider       string `json:"provider"`
	Enabled        bool   `json:"enabled"`
	DefaultEnabled bool   `json:"defaultEnabled"`
	DisplayName    string `json:"displayName"`
}

func parseCatalog(stdout []byte) (backend.CatalogSnapshot, error) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return backend.CatalogSnapshot{}, scoped(domain.ErrEmptyStdout, "", "catalog returned no data")
	}
	var dto catalogDTO
	if trimmed[0] == '[' {
		// CodexBar 0.78.0 answers with a bare provider array; accept that
		// alongside the {"providers": [...]} envelope form.
		if err := json.Unmarshal(trimmed, &dto.Providers); err != nil {
			return backend.CatalogSnapshot{}, scoped(domain.ErrMalformedJSON, "", "catalog JSON is malformed")
		}
	} else {
		var raw map[string]any
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			return backend.CatalogSnapshot{}, scoped(domain.ErrMalformedJSON, "", "catalog JSON is malformed")
		}
		if category := errorCategory(raw); category != "" {
			return backend.CatalogSnapshot{}, scoped(category, "", "catalog reported an error")
		}
		if err := json.Unmarshal(trimmed, &dto); err != nil {
			return backend.CatalogSnapshot{}, scoped(domain.ErrMalformedJSON, "", "catalog JSON is malformed")
		}
	}
	if len(dto.Providers) == 0 {
		return backend.CatalogSnapshot{}, scoped(domain.ErrUnsupported, "", "catalog has no provider list")
	}
	providers := make([]domain.SourceDescriptor, 0, len(dto.Providers))
	for _, provider := range dto.Providers {
		if provider.Provider == "" {
			continue
		}
		if !validProviderID(provider.Provider) {
			return backend.CatalogSnapshot{}, scoped(domain.ErrUnsupported, "", "catalog contains an invalid provider identifier")
		}
		name := provider.DisplayName
		if name == "" {
			name = provider.Provider
		}
		providers = append(providers, domain.SourceDescriptor{
			Key:            domain.SourceKey{Provider: provider.Provider},
			ProviderID:     provider.Provider,
			DisplayName:    strings.TrimSpace(name),
			Kind:           domain.KindUnknown,
			Available:      true,
			Enabled:        provider.Enabled,
			DefaultEnabled: provider.DefaultEnabled,
			Capabilities: domain.Capabilities{
				Usage:       domain.CapabilityUnknown,
				History:     domain.CapabilityUnknown,
				Balances:    domain.CapabilityUnknown,
				Details:     domain.CapabilityUnknown,
				ActiveProbe: domain.CapabilityUnknown,
			},
		})
	}
	if len(providers) == 0 {
		return backend.CatalogSnapshot{}, scoped(domain.ErrUnsupported, "", "catalog entries are missing provider IDs")
	}
	return backend.CatalogSnapshot{Providers: providers}, nil
}

func decodeDocument(stdout []byte) (any, error) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return nil, scoped(domain.ErrEmptyStdout, "", "backend returned no data")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, scoped(domain.ErrMalformedJSON, "", "backend JSON is malformed")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, scoped(domain.ErrMalformedJSON, "", "backend returned trailing JSON data")
	}
	return value, nil
}

func usageRecords(document any, key string) ([]map[string]any, error) {
	switch typed := document.(type) {
	case []any:
		return flattenUsageList(typed), nil
	case map[string]any:
		for _, field := range []string{"results", "snapshots", "readings", "data", "usages"} {
			if list := asList(pick(typed, field)); list != nil {
				records := flattenUsageList(list)
				for index := range records {
					records[index] = mergeUsageMetadata(typed, records[index])
				}
				if errorCategory(typed) != "" {
					records = append(records, typed)
				}
				return records, nil
			}
		}
		if accounts := asList(pick(typed, "accounts")); accounts != nil {
			if len(accounts) == 0 {
				if category := errorCategory(typed); category != "" {
					return nil, scoped(category, key, "backend reported an error")
				}
				return nil, scoped(domain.ErrUnsupported, key, "backend returned no accounts")
			}
			return expandAccounts(typed, accounts), nil
		}
		if usage := asMap(pick(typed, "usage")); usage != nil {
			return []map[string]any{mergeEnvelope(typed, usage)}, nil
		}
		if category := errorCategory(typed); category != "" && !hasUsageShape(typed) && !hasAccountIdentity(typed) {
			return nil, scoped(category, key, "backend reported an error")
		}
		return []map[string]any{typed}, nil
	default:
		return nil, scoped(domain.ErrUnsupported, key, "backend response is not an object or array")
	}
}

func accountIdentity(record map[string]any) (string, string) {
	identity := asMap(pick(record, "identity", "accountIdentity", "account_identity"))
	if identity == nil {
		identity = asMap(pick(record, "account"))
	}
	key := ""
	label := ""
	if identity != nil {
		key = stringValue(pick(identity, "accountKey", "account_key", "accountID", "accountId", "account_id", "id"))
		label = stringValue(pick(identity, "accountLabel", "account_label", "email", "user", "name"))
	}
	if key == "" {
		key = stringValue(pick(record, "accountKey", "account_key", "accountID", "accountId", "account_id"))
	}
	if label == "" {
		label = stringValue(pick(record, "accountLabel", "account_label", "email", "user"))
	}
	return key, label
}

func hasAccountIdentity(record map[string]any) bool {
	key, _ := accountIdentity(record)
	return key != ""
}

func hasUsageShape(mapped map[string]any) bool {
	for _, field := range []string{"primary", "secondary", "tertiary", "session", "weekly", "monthly", "windows", "quotas", "allowances", "limits", "extraWindows", "extra_windows", "extraRateWindows", "extra_rate_windows", "namedWindows", "named_windows", "balances", "balance", "credits", "wallets", "details", "usageDetails", "usage_details", "extraDetails", "extra_details", "plan", "usage", "providerCost", "provider_cost", "cost"} {
		if pick(mapped, field) != nil {
			return true
		}
	}
	return false
}

func mapsFromList(list []any) []map[string]any {
	records := make([]map[string]any, 0, len(list))
	for _, entry := range list {
		if mapped := asMap(entry); mapped != nil {
			records = append(records, mapped)
		}
	}
	return records
}

func flattenUsageList(list []any) []map[string]any {
	var records []map[string]any
	for _, entry := range list {
		mapped := asMap(entry)
		if mapped == nil {
			records = append(records, map[string]any{"__incompatible": entry})
			continue
		}
		if accounts := asList(pick(mapped, "accounts")); accounts != nil {
			if len(accounts) == 0 {
				if errorCategory(mapped) != "" {
					records = append(records, mapped)
				} else {
					records = append(records, map[string]any{"__incompatible": true, "provider": pick(mapped, "provider", "providerID", "provider_id")})
				}
				continue
			}
			records = append(records, expandAccounts(mapped, accounts)...)
			continue
		}
		nested := false
		for _, field := range []string{"results", "snapshots", "readings", "data", "usages"} {
			if child := asList(pick(mapped, field)); child != nil {
				children := flattenUsageList(child)
				for _, record := range children {
					records = append(records, mergeUsageMetadata(mapped, record))
				}
				nested = true
				break
			}
		}
		if nested {
			if errorCategory(mapped) != "" {
				records = append(records, mapped)
			}
			continue
		}
		if usage := asMap(pick(mapped, "usage")); usage != nil {
			records = append(records, mergeEnvelope(mapped, usage))
			continue
		}
		records = append(records, mapped)
	}
	return records
}

func expandAccounts(base map[string]any, accounts []any) []map[string]any {
	var records []map[string]any
	baseCopy := mergeMaps(base, nil)
	for _, key := range append([]string{"identity", "accountIdentity", "account_identity", "account", "accountKey", "account_key", "accountID", "accountId", "account_id", "accountLabel", "account_label", "email", "usage"}, []string{"primary", "secondary", "tertiary", "session", "weekly", "monthly", "windows", "quotas", "allowances", "limits", "extraWindows", "extra_windows", "extraRateWindows", "extra_rate_windows", "namedWindows", "named_windows", "balances", "credits", "wallets", "balance", "credit", "details", "usageDetails", "usage_details", "extraDetails", "extra_details", "providerCost", "provider_cost", "cost"}...) {
		delete(baseCopy, key)
	}
	for _, entry := range accounts {
		mapped := asMap(entry)
		if mapped == nil {
			records = append(records, map[string]any{"__incompatible": entry, "provider": pick(base, "provider", "providerID", "provider_id")})
			continue
		}
		record := mergeEnvelope(baseCopy, mapped)
		if usage := asMap(pick(mapped, "usage")); usage != nil {
			record = mergeEnvelope(mergeMaps(mapped, nil), usage)
			record = mergeEnvelope(baseCopy, record)
		}
		delete(record, "accounts")
		records = append(records, record)
	}
	return records
}

func mergeUsageMetadata(base, child map[string]any) map[string]any {
	baseCopy := mergeMaps(base, nil)
	for _, group := range aliasGroups() {
		if hasAnyKey(child, group...) {
			for _, key := range group {
				delete(baseCopy, key)
			}
		}
	}
	merged := mergeMaps(baseCopy, child)
	for _, key := range append([]string{"identity", "accountIdentity", "account_identity", "account", "accountKey", "account_key", "accountID", "accountId", "account_id", "accountLabel", "account_label", "email", "usage"}, []string{"primary", "secondary", "tertiary", "session", "weekly", "monthly", "windows", "quotas", "allowances", "limits", "extraWindows", "extra_windows", "extraRateWindows", "extra_rate_windows", "namedWindows", "named_windows", "balances", "credits", "wallets", "balance", "credit", "details", "usageDetails", "usage_details", "extraDetails", "extra_details", "providerCost", "provider_cost", "cost"}...) {
		if _, ok := child[key]; !ok {
			delete(merged, key)
		}
	}
	if _, ok := child["error"]; !ok {
		delete(merged, "error")
	}
	if _, ok := child["errors"]; !ok {
		delete(merged, "errors")
	}
	if _, ok := child["failure"]; !ok {
		delete(merged, "failure")
	}
	return merged
}

func hasAnyKey(mapped map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := mapped[key]; ok {
			return true
		}
	}
	return false
}

func mergeEnvelope(base, extra map[string]any) map[string]any {
	baseCopy := mergeMaps(base, nil)
	for _, group := range aliasGroups() {
		if hasAnyKey(extra, group...) {
			for _, key := range group {
				delete(baseCopy, key)
			}
		}
	}
	merged := mergeMaps(baseCopy, extra)
	if errorCategory(base) != "" && errorCategory(merged) == "" {
		merged["error"] = pick(base, "error", "errors", "failure")
	}
	return merged
}

func aliasGroups() [][]string {
	return [][]string{
		{"reportedSource", "reported_source", "source", "sourceMode", "source_mode"},
		{"sampleAt", "sample_at", "sampledAt", "sampled_at", "timestamp", "updatedAt", "updated_at", "cachedAt", "cached_at", "cacheTimestamp", "cache_timestamp", "cacheAge", "cache_age", "cacheAgeSeconds", "cache_age_seconds", "ageSeconds", "age_seconds"},
		{"provider", "providerID", "provider_id", "id"},
		{"variant", "sourceVariant", "source_variant"},
		{"identity", "accountIdentity", "account_identity", "account", "accountKey", "account_key", "accountID", "accountId", "account_id", "accountLabel", "account_label", "email"},
	}
}

func mergeMaps(base, extra map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(extra))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func errorCategory(mapped map[string]any) domain.ErrorCategory {
	found := false
	for _, field := range []string{"error", "errors", "failure"} {
		value := pick(mapped, field)
		if value == nil {
			continue
		}
		if text := stringValue(value); text != "" {
			found = true
			if category := categoryFromText(text); category != "" {
				return category
			}
		}
		if object := asMap(value); object != nil {
			code := stringValue(pick(object, "code", "type", "category", "kind"))
			message := stringValue(pick(object, "message", "detail", "description"))
			if len(object) > 0 {
				found = true
			}
			if category := categoryFromText(code + " " + message); category != "" {
				return category
			}
		}
		if list := asList(value); list != nil {
			for _, entry := range list {
				if text := stringValue(entry); text != "" {
					found = true
					if category := categoryFromText(text); category != "" {
						return category
					}
					continue
				}
				object := asMap(entry)
				if object == nil {
					continue
				}
				code := stringValue(pick(object, "code", "type", "category", "kind"))
				message := stringValue(pick(object, "message", "detail", "description"))
				if len(object) > 0 {
					found = true
				}
				if category := categoryFromText(code + " " + message); category != "" {
					return category
				}
			}
		}
	}
	if found {
		return domain.ErrUnknown
	}
	if success, ok := pick(mapped, "success", "ok").(bool); ok && !success {
		return domain.ErrProcessFailure
	}
	return ""
}

func normalizeUsage(document any, req backend.UsageRequest, now time.Time, allowed func(string, domain.Variant) bool) ([]domain.UsageSnapshot, error) {
	key := requestKey(req)
	records, err := usageRecords(document, key.String())
	if err != nil {
		return nil, err
	}
	var snapshots []domain.UsageSnapshot
	var firstErr error
	hasSuccess := false
	hasMeasuredFailure := false
	hasIncompatible := false
	for _, record := range records {
		category := errorCategory(record)
		snapshot := normalizeUsageRecord(record, key, now, allowed)
		if snapshot.Key.Provider == "" {
			continue
		}
		if category != "" {
			snapshot.Error = domain.NewScopedError(category, snapshot.Key.String(), "backend reported an error")
			snapshot.State = category.UXState()
			if firstErr == nil {
				firstErr = scoped(category, snapshot.Key.String(), "backend reported an error")
			}
		}
		hasMeasurements := len(snapshot.Windows) > 0 || len(snapshot.Balances) > 0 || len(snapshot.Details) > 0 || snapshot.Plan != ""
		if !hasMeasurements && category == "" {
			snapshot.Error = domain.NewScopedError(domain.ErrUnsupported, snapshot.Key.String(), "backend response shape is not supported")
			snapshot.State = domain.StatusBackendIncompatible
			if firstErr == nil {
				firstErr = scoped(domain.ErrUnsupported, snapshot.Key.String(), "backend response shape is not supported")
			}
			hasIncompatible = true
			snapshots = append(snapshots, snapshot)
			continue
		}
		if snapshot.Error != nil && category == "" {
			category = snapshot.Error.Category
			if firstErr == nil {
				firstErr = snapshot.Error
			}
		}
		if hasMeasurements && category == "" {
			hasSuccess = true
		}
		if hasMeasurements && category != "" {
			hasMeasuredFailure = true
		}
		snapshots = append(snapshots, snapshot)
	}
	if (hasMeasuredFailure || hasIncompatible) && firstErr != nil {
		return snapshots, firstErr
	}
	if !hasSuccess {
		if firstErr != nil {
			return snapshots, firstErr
		}
		return nil, scoped(domain.ErrUnsupported, key.String(), "backend returned no usable usage records")
	}
	return snapshots, nil
}

func requestKey(req backend.UsageRequest) domain.SourceKey {
	if req.SourceKey != "" {
		if parsed, err := domain.ParseSourceKey(req.SourceKey); err == nil {
			return parsed
		}
	}
	return domain.SourceKey{Provider: req.ProviderID, Variant: req.Variant}
}

func normalizeUsageRecord(record map[string]any, base domain.SourceKey, now time.Time, allowed func(string, domain.Variant) bool) domain.UsageSnapshot {
	accountKey, accountLabel := accountIdentity(record)
	key := base
	key.Account = domain.UnknownAccount
	key.AccountUnknown = true
	if accountKey != "" {
		key.Account = accountKey
		key.AccountUnknown = false
	}
	provider := stringValue(pick(record, "provider", "providerID", "provider_id", "id"))
	invalidProvider := provider != "" && !validProviderID(provider)
	if provider != "" && !invalidProvider {
		key.Provider = provider
	}
	reportedSource := stringValue(pick(record, "reportedSource", "reported_source", "source", "sourceMode", "source_mode"))
	variant := stringValue(pick(record, "variant", "sourceVariant", "source_variant"))
	variantError := domain.ErrorCategory("")
	if invalidProvider {
		variantError = domain.ErrUnsupported
	}
	if variant != "" {
		if !validVariantString(variant) {
			variantError = domain.ErrUnsupported
		} else if base.Variant == "" {
			key.Variant = domain.Variant(variant)
		} else if domain.Variant(variant) != base.Variant {
			variantError = domain.ErrUnsupported
		}
	}
	if allowed != nil && !allowed(key.Provider, key.Variant) {
		variantError = domain.ErrUnsupported
	}
	if reportedSource == string(domain.VariantWeb) || reportedSource == string(domain.VariantAPI) {
		if base.Variant == domain.VariantWeb || base.Variant == domain.VariantAPI {
			if domain.Variant(reportedSource) != base.Variant {
				variantError = domain.ErrUnsupported
			}
		}
	}
	display := stringValue(pick(record, "displayName", "display_name", "name"))
	if display == "" {
		display = key.Provider
	}
	snapshot := domain.UsageSnapshot{
		Key:            key,
		ProviderID:     key.Provider,
		DisplayName:    display,
		AccountKey:     key.Account,
		AccountLabel:   accountLabel,
		Plan:           stringValue(pick(record, "plan", "planName", "plan_name", "tier")),
		ReportedSource: reportedSource,
		Quality:        qualityValue(stringValue(pick(record, "quality", "statusQuality", "status_quality"))),
		ReceivedAt:     now,
	}
	snapshot.SampleAt = timeValue(pick(record, "sampleAt", "sample_at", "sampledAt", "sampled_at", "timestamp", "updatedAt", "updated_at"))
	if snapshot.SampleAt == nil {
		if cachedAt := timeValue(pick(record, "cachedAt", "cached_at", "cacheTimestamp", "cache_timestamp")); cachedAt != nil {
			snapshot.SampleAt = cachedAt
		} else if age, ok := cacheAgeValue(pick(record, "cacheAge", "cache_age", "cacheAgeSeconds", "cache_age_seconds", "ageSeconds", "age_seconds")); ok {
			value := now.Add(-age)
			snapshot.SampleAt = &value
		}
	}
	snapshot.Windows = normalizeWindowKeys(windowsFromRecord(record))
	snapshot.Balances = balancesFromRecord(record)
	snapshot.Details = detailsFromRecord(record)
	if malformedMetricCollections(record) {
		snapshot.Details = append(snapshot.Details, domain.DetailSection{Title: "Unsupported metrics", Unsupported: true})
	}
	if section := monetaryUsageSection(record); section != nil {
		snapshot.Details = append(snapshot.Details, *section)
	}
	if variantError != "" {
		snapshot.Windows = nil
		snapshot.Balances = nil
		snapshot.Details = nil
		snapshot.Error = domain.NewScopedError(variantError, key.String(), "backend returned an unsupported source variant")
	}
	snapshot.State = domain.DeriveState(snapshot, false)
	return snapshot
}

func monetaryUsageSection(record map[string]any) *domain.DetailSection {
	usage := asMap(pick(record, "usage"))
	cost := asMap(pick(record, "cost", "providerCost", "provider_cost"))
	if usage != nil {
		if nested := asMap(pick(usage, "cost", "providerCost", "provider_cost")); nested != nil {
			cost = nested
		}
	}
	if cost == nil {
		return nil
	}
	if pick(cost, "used", "limit", "period", "billingPeriod", "billing_period") == nil {
		return nil
	}
	section := &domain.DetailSection{Title: "Monetary usage"}
	add := func(label string, value any) {
		if text := stringValue(value); text != "" {
			section.Rows = append(section.Rows, domain.DetailRow{Label: label, Value: text})
		}
	}
	add("Used", pick(cost, "used", "usage", "spend"))
	add("Limit", pick(cost, "limit", "total", "budget"))
	add("Currency", pick(cost, "currency", "unit"))
	add("Period", pick(cost, "period", "billingPeriod", "billing_period"))
	if len(section.Rows) == 0 {
		return nil
	}
	return section
}

func malformedMetricCollections(record map[string]any) bool {
	for _, field := range []string{"windows", "quotas", "allowances", "limits", "extraWindows", "extra_windows", "extraRateWindows", "extra_rate_windows", "namedWindows", "named_windows", "balances", "credits", "wallets"} {
		value, present := lookup(record, field)
		if !present || value == nil {
			continue
		}
		if entries := asList(value); entries != nil {
			for _, entry := range entries {
				if asMap(entry) == nil {
					return true
				}
			}
			continue
		}
		if field == "balances" {
			return true
		}
		if asMap(value) == nil {
			return true
		}
	}
	return false
}

func validVariantString(value string) bool {
	switch domain.Variant(value) {
	case domain.VariantConfigured, domain.VariantWeb, domain.VariantAPI:
		return true
	}
	return false
}

func qualityValue(value string) domain.Quality {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "stale":
		return domain.QualityStale
	case "partial", "incomplete":
		return domain.QualityPartial
	case "unavailable", "missing":
		return domain.QualityUnavailable
	case "unsupported":
		return domain.QualityUnsupported
	default:
		return domain.QualityMeasured
	}
}

func windowsFromRecord(record map[string]any) []domain.AllowanceWindow {
	reported := reportedWindowLabels(record)
	var windows []domain.AllowanceWindow
	for _, field := range []string{"windows", "quotas", "allowances", "limits"} {
		value, present := lookup(record, field)
		if !present || value == nil {
			continue
		}
		if entries := asList(value); entries != nil {
			for index, entry := range entries {
				if mapped := asMap(entry); mapped != nil {
					windows = append(windows, normalizeWindow(mapped))
				} else {
					windows = append(windows, domain.AllowanceWindow{Key: fmt.Sprintf("%s-%d", field, index+1), Label: "Unsupported metric", Quality: domain.QualityUnsupported})
				}
			}
		} else {
			windows = append(windows, domain.AllowanceWindow{Key: field, Label: "Unsupported metric", Quality: domain.QualityUnsupported})
		}
	}
	labels := map[string]string{"primary": "Primary", "secondary": "Secondary", "tertiary": "Tertiary", "session": "Session", "weekly": "Weekly", "monthly": "Monthly"}
	for key, label := range reported {
		labels[key] = label
	}
	for _, field := range []string{"primary", "secondary", "tertiary", "session", "weekly", "monthly"} {
		value, present := lookup(record, field)
		if present && value == nil {
			windows = append(windows, domain.AllowanceWindow{Key: field, Label: labels[field], Quality: domain.QualityUnavailable})
			continue
		}
		for index, entry := range asList(value) {
			if mapped := asMap(entry); mapped != nil {
				window := normalizeWindow(mapped)
				if window.Key == "" {
					window.Key = field
				}
				if window.Label == "" {
					window.Label = labels[field]
				}
				windows = append(windows, window)
			} else {
				windows = append(windows, domain.AllowanceWindow{Key: fmt.Sprintf("%s-%d", field, index+1), Label: "Unsupported metric", Quality: domain.QualityUnsupported})
			}
		}
		if mapped := asMap(value); mapped != nil {
			window := normalizeWindow(mapped)
			if window.Key == "" {
				window.Key = field
			}
			if window.Label == "" {
				window.Label = labels[field]
			}
			windows = append(windows, window)
		} else if present && value != nil && asList(value) == nil {
			windows = append(windows, domain.AllowanceWindow{Key: field, Label: labels[field], Quality: domain.QualityUnsupported})
		}
	}
	for _, field := range []string{"extraWindows", "extra_windows", "extraRateWindows", "extra_rate_windows", "namedWindows", "named_windows"} {
		value, present := lookup(record, field)
		if !present || value == nil {
			continue
		}
		entries := asList(value)
		if entries == nil {
			windows = append(windows, domain.AllowanceWindow{Key: field, Label: "Unsupported metric", Quality: domain.QualityUnsupported})
			continue
		}
		for index, entry := range entries {
			if mapped := asMap(entry); mapped != nil {
				windows = append(windows, normalizeWindow(mapped))
			} else {
				windows = append(windows, domain.AllowanceWindow{Key: fmt.Sprintf("%s-%d", field, index+1), Label: "Unsupported metric", Quality: domain.QualityUnsupported})
			}
		}
	}
	return windows
}

func normalizeWindow(mapped map[string]any) domain.AllowanceWindow {
	if nested := asMap(pick(mapped, "window")); nested != nil {
		mapped = mergeMaps(nested, mapped)
	}
	window := domain.AllowanceWindow{
		Key:      stringValue(pick(mapped, "key", "id", "window", "windowKey")),
		Label:    stringValue(pick(mapped, "label", "name", "title", "windowLabel")),
		Group:    stringValue(pick(mapped, "group", "groupName", "modelGroup", "model_group")),
		Unit:     stringValue(pick(mapped, "unit", "units")),
		Quality:  qualityValue(stringValue(pick(mapped, "quality"))),
		ResetAt:  timeValue(pick(mapped, "resetAt", "reset_at", "resetsAt", "resets_at", "resetTime", "windowResetAt")),
		Duration: durationValue(pick(mapped, "windowMinutes", "window_minutes", "durationMinutes", "duration_minutes")),
	}
	if known, ok := lookup(mapped, "usageKnown", "usage_known", "known"); ok && !boolValue(known) {
		window.UsedPercent = nil
		window.Used = nil
		window.Limit = nil
		window.Quality = domain.QualityUnavailable
		return window
	}
	percentInvalid := false
	percentValue, percentPresent := lookup(mapped, "usedPercent", "used_percentage", "percentUsed", "percent_used", "usagePercent", "used_pct")
	if percentPresent {
		if usedPercent, ok := floatValue(percentValue); ok {
			window.UsedPercent = &usedPercent
		} else {
			window.UsedPercent = nil
			window.Quality = domain.QualityUnavailable
			percentInvalid = true
		}
	} else if remainingValue, present := lookup(mapped, "remainingPercent", "remaining_percentage", "percentRemaining", "percent_remaining"); present {
		if remainingPercent, ok := floatValue(remainingValue); ok {
			used := 100 - remainingPercent
			window.UsedPercent = &used
		} else {
			window.UsedPercent = nil
			window.Quality = domain.QualityUnavailable
			percentInvalid = true
		}
	}
	if used, ok := floatValue(pick(mapped, "used", "usedAmount", "used_amount", "consumed", "usage")); ok {
		window.Used = &used
	}
	if limit, ok := floatValue(pick(mapped, "limit", "total", "allowance", "max", "capacity")); ok {
		window.Limit = &limit
	}
	if !percentInvalid && window.UsedPercent == nil && window.Used != nil && window.Limit != nil && *window.Limit > 0 {
		usedPercent := *window.Used / *window.Limit * 100
		window.UsedPercent = &usedPercent
	}
	return window
}

func normalizeWindowKeys(windows []domain.AllowanceWindow) []domain.AllowanceWindow {
	explicitCount := map[string]int{}
	for _, window := range windows {
		if window.Key != "" {
			explicitCount[window.Key]++
		}
	}
	reserved := map[string]bool{}
	for key, count := range explicitCount {
		if count > 0 {
			reserved[key] = true
		}
	}
	consumed := map[string]bool{}
	assigned := map[string]bool{}
	next := func(base string) string {
		candidate := base
		for suffix := 2; ; suffix++ {
			if !assigned[candidate] && !reserved[candidate] {
				return candidate
			}
			candidate = fmt.Sprintf("%s-%d", base, suffix)
		}
	}
	for index := range windows {
		base := windows[index].Key
		if base != "" && explicitCount[base] > 1 && windows[index].Group != "" {
			candidate := base + "-" + slugKey(windows[index].Group)
			key := candidate
			for suffix := 2; assigned[key] || reserved[key]; suffix++ {
				key = fmt.Sprintf("%s-%d", candidate, suffix)
			}
			windows[index].Key = key
			assigned[key] = true
			consumed[base] = true
			continue
		}
		if base != "" && !consumed[base] {
			windows[index].Key = base
			consumed[base] = true
			assigned[base] = true
			continue
		}
		if base == "" {
			label := windows[index].Label
			if windows[index].Group != "" {
				label = windows[index].Group + " " + label
			}
			base = slugKey(label)
		}
		if base == "" {
			base = fmt.Sprintf("window-%d", index+1)
		}
		key := next(base)
		windows[index].Key = key
		assigned[key] = true
	}
	return windows
}

func slugKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteByte('-')
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func reportedWindowLabels(record map[string]any) map[string]string {
	labels := map[string]string{}
	for _, field := range []string{"rateWindowLabels", "rate_window_labels", "windowLabels", "window_labels"} {
		value := pick(record, field)
		if mapped := asMap(value); mapped != nil {
			for key, label := range mapped {
				if text := stringValue(label); text != "" {
					labels[key] = text
				}
			}
		}
		for _, entry := range asList(value) {
			if mapped := asMap(entry); mapped != nil {
				key := stringValue(pick(mapped, "key", "id", "window", "name"))
				label := stringValue(pick(mapped, "label", "title", "displayName"))
				if key != "" && label != "" {
					labels[key] = label
				}
			}
		}
	}
	return labels
}

func cacheAgeValue(value any) (time.Duration, bool) {
	if text := stringValue(value); text != "" {
		if parsed, err := time.ParseDuration(text); err == nil {
			return parsed, parsed >= 0
		}
	}
	seconds, ok := floatValue(value)
	if !ok || seconds < 0 {
		return 0, false
	}
	return time.Duration(seconds * float64(time.Second)), true
}

func durationValue(value any) time.Duration {
	minutes, ok := floatValue(value)
	if !ok || minutes <= 0 {
		return 0
	}
	return time.Duration(minutes * float64(time.Minute))
}

func balancesFromRecord(record map[string]any) []domain.Balance {
	var balances []domain.Balance
	for _, field := range []string{"balances", "credits", "wallets"} {
		value, present := lookup(record, field)
		if !present || value == nil {
			continue
		}
		if entries := asList(value); entries != nil {
			for _, entry := range entries {
				if mapped := asMap(entry); mapped != nil {
					balances = append(balances, normalizeBalance(mapped))
				} else {
					balances = append(balances, domain.Balance{Label: "Unsupported balance", Kind: "unsupported"})
				}
			}
			continue
		}
		if mapped := asMap(value); mapped != nil {
			balance := normalizeBalance(mapped)
			if balance.Label == "" {
				balance.Label = field
			}
			balances = append(balances, balance)
		} else {
			balances = append(balances, domain.Balance{Label: "Unsupported balance", Kind: "unsupported"})
		}
	}
	for _, field := range []string{"balance", "credit"} {
		if mapped := asMap(pick(record, field)); mapped != nil {
			balance := normalizeBalance(mapped)
			if balance.Label == "" {
				balance.Label = field
			}
			balances = append(balances, balance)
		}
	}
	return balances
}

func normalizeBalance(mapped map[string]any) domain.Balance {
	balance := domain.Balance{
		Label:    stringValue(pick(mapped, "label", "name", "title")),
		Unit:     stringValue(pick(mapped, "unit", "currency", "unitLabel")),
		Kind:     stringValue(pick(mapped, "kind", "type")),
		SampleAt: timeValue(pick(mapped, "sampleAt", "sample_at", "timestamp", "updatedAt")),
	}
	if value, ok := floatValue(pick(mapped, "value", "amount", "remaining", "balance", "credits")); ok {
		balance.Value = &value
	}
	if balance.Label == "" {
		balance.Label = "Credits"
	}
	if balance.Unit == "" {
		balance.Unit = stringValue(pick(mapped, "currencyCode"))
	}
	return balance
}

func detailsFromRecord(record map[string]any) []domain.DetailSection {
	var sections []domain.DetailSection
	for _, field := range []string{"details", "usageDetails", "usage_details", "extraDetails", "extra_details"} {
		value := pick(record, field)
		for _, entry := range asList(value) {
			if mapped := asMap(entry); mapped != nil {
				sections = append(sections, normalizeDetailSection(mapped))
			} else {
				sections = append(sections, domain.DetailSection{Title: "Details", Unsupported: true})
			}
		}
		if mapped := asMap(value); mapped != nil {
			sections = append(sections, normalizeDetailSection(mapped))
		} else if value != nil && asList(value) == nil {
			sections = append(sections, domain.DetailSection{Title: "Details", Unsupported: true})
		}
	}
	return sections
}

func normalizeDetailSection(mapped map[string]any) domain.DetailSection {
	section := domain.DetailSection{
		Title:       stringValue(pick(mapped, "title", "name", "label")),
		Unsupported: boolValue(pick(mapped, "unsupported")),
	}
	for _, entry := range asList(pick(mapped, "rows")) {
		if row := asMap(entry); row != nil {
			label := stringValue(pick(row, "label", "name", "key"))
			section.Rows = append(section.Rows, domain.DetailRow{
				Label:     label,
				Value:     stringValue(pick(row, "value", "text", "amount")),
				Sensitive: boolValue(pick(row, "sensitive", "private", "secret")),
			})
			if progressValue, present := lookup(row, "progress"); present && progressValue != nil {
				if progress := asMap(progressValue); progress != nil {
					bar := domain.DetailBar{Label: label, Unit: stringValue(pick(progress, "unit"))}
					if value, ok := floatValue(pick(progress, "value", "used")); ok {
						bar.Value = &value
					}
					if max, ok := floatValue(pick(progress, "max", "limit", "total")); ok {
						bar.Max = &max
					}
					section.Bars = append(section.Bars, bar)
				} else {
					section.Unsupported = true
				}
			}
		} else {
			section.Unsupported = true
		}
	}
	for _, entry := range asList(pick(mapped, "bars", "progress")) {
		if bar := asMap(entry); bar != nil {
			normalized := domain.DetailBar{
				Label: stringValue(pick(bar, "label", "name", "key")),
				Unit:  stringValue(pick(bar, "unit")),
			}
			if value, ok := floatValue(pick(bar, "value", "used")); ok {
				normalized.Value = &value
			}
			if max, ok := floatValue(pick(bar, "max", "limit", "total")); ok {
				normalized.Max = &max
			}
			section.Bars = append(section.Bars, normalized)
		} else {
			section.Unsupported = true
		}
	}
	chartValue := pick(mapped, "points", "chart", "chartPoints")
	if chartMap := asMap(chartValue); chartMap != nil {
		chartValue = pick(chartMap, "points", "chartPoints")
	}
	for _, entry := range asList(chartValue) {
		if point := asMap(entry); point != nil {
			normalized := domain.ChartPoint{Label: stringValue(pick(point, "label", "name", "key", "day"))}
			if value, ok := floatValue(pick(point, "value", "amount")); ok {
				normalized.Value = &value
			}
			section.Points = append(section.Points, normalized)
		} else {
			section.Unsupported = true
		}
	}
	if section.Title == "" {
		section.Title = "Details"
	}
	for _, field := range []string{"rows", "bars", "progress", "points", "chart", "chartPoints"} {
		value := pick(mapped, field)
		if value == nil || asList(value) != nil {
			continue
		}
		if field == "chart" || field == "chartPoints" {
			if chart := asMap(value); chart != nil && asList(pick(chart, "points", "chartPoints")) != nil {
				continue
			}
		}
		section.Unsupported = true
	}
	known := map[string]bool{"title": true, "name": true, "label": true, "rows": true, "bars": true, "progress": true, "points": true, "chart": true, "chartPoints": true, "unsupported": true}
	for key := range mapped {
		if !known[key] {
			section.Unsupported = true
		}
	}
	if len(section.Rows) == 0 && len(section.Bars) == 0 && len(section.Points) == 0 {
		section.Unsupported = true
	}
	return section
}

func boolValue(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	if text := stringValue(value); text != "" {
		return strings.EqualFold(text, "true") || text == "1"
	}
	return false
}

type historyGroup struct {
	top     map[string]any
	records []map[string]any
}

func normalizeHistory(document any, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
	key := requestKey(backend.UsageRequest{ProviderID: req.ProviderID, Variant: req.Variant, SourceKey: req.SourceKey})
	key.Account = domain.UnknownAccount
	key.AccountUnknown = true
	var groups []historyGroup
	var siblingErr error
	recognizedEmpty := false

	addEnvelope := func(mapped map[string]any) error {
		if provider := stringValue(pick(mapped, "provider", "providerID", "provider_id")); provider != "" && provider != key.Provider {
			if siblingErr == nil {
				siblingErr = scoped(domain.ErrUnsupported, key.String(), "history envelope belongs to another provider")
			}
			return nil
		}
		category := errorCategory(mapped)
		if category != "" && siblingErr == nil {
			siblingErr = scoped(category, key.String(), "backend reported an error")
		}
		for _, field := range []string{"days", "daily", "buckets", "data", "results", "history"} {
			if list := asList(pick(mapped, field)); list != nil {
				records := mapsFromList(list)
				if len(list) == 0 {
					recognizedEmpty = true
				}
				if len(records) != len(list) && siblingErr == nil {
					siblingErr = scoped(domain.ErrUnsupported, key.String(), "history has a non-object daily member")
				}
				groups = append(groups, historyGroup{top: mapped, records: records})
				return nil
			}
		}
		if _, ok := pick(mapped, "day", "date").(string); ok {
			groups = append(groups, historyGroup{top: mapped, records: []map[string]any{mapped}})
			return nil
		}
		if category != "" {
			return nil
		}
		return scoped(domain.ErrUnsupported, key.String(), "history response has no daily records")
	}

	switch typed := document.(type) {
	case []any:
		for _, entry := range typed {
			mapped := asMap(entry)
			if mapped == nil {
				if siblingErr == nil {
					siblingErr = scoped(domain.ErrUnsupported, key.String(), "history array contains a non-object member")
				}
				continue
			}
			if nested := asMap(pick(mapped, "history")); nested != nil {
				mapped = mergeEnvelope(mapped, nested)
			}
			if err := addEnvelope(mapped); err != nil {
				if siblingErr == nil {
					siblingErr = err
				}
				continue
			}
		}
	case map[string]any:
		base := typed
		if history := asMap(pick(typed, "history")); history != nil {
			base = mergeEnvelope(typed, history)
		}
		if err := addEnvelope(base); err != nil {
			return domain.HistorySnapshot{}, err
		}
	default:
		return domain.HistorySnapshot{}, scoped(domain.ErrUnsupported, key.String(), "history response is not an object or array")
	}
	if len(groups) == 0 {
		if siblingErr != nil {
			return domain.HistorySnapshot{}, siblingErr
		}
		return domain.HistorySnapshot{}, scoped(domain.ErrUnsupported, key.String(), "history has no daily records")
	}

	history := domain.HistorySnapshot{Key: key, Days: req.Days}
	if history.Days <= 0 {
		history.Days = 7
	}
	chosenTimezone := ""
	chosenReader := ""
	for _, group := range groups {
		sources := append([]map[string]any{group.top}, group.records...)
		for _, source := range sources {
			if chosenTimezone == "" {
				chosenTimezone = stringValue(pick(source, "timezone", "timeZone"))
			}
			if chosenReader == "" {
				chosenReader = stringValue(pick(source, "reportedSource", "reported_source", "source", "sourceMode", "source_mode"))
			}
			if chosenTimezone != "" && chosenReader != "" {
				break
			}
		}
		if chosenTimezone != "" && chosenReader != "" {
			break
		}
	}
	accounts := map[string]bool{}
	accountUnknown := false
	for _, group := range groups {
		if len(group.top) == 0 {
			accountUnknown = true
			continue
		}
		account := stringValue(pick(group.top, "accountKey", "account_key", "accountID", "account"))
		if account == "" {
			accountUnknown = true
		} else {
			accounts[account] = true
		}
	}
	if len(accounts) == 1 && !accountUnknown {
		for account := range accounts {
			history.Key.Account = account
			history.Key.AccountUnknown = false
		}
	}

	tokenOverflow := false
	modelTotals := map[string]*domain.ModelTotal{}
	modelsMalformed := func(mapped map[string]any) bool {
		for _, field := range []string{"models", "modelBreakdowns"} {
			value, present := lookup(mapped, field)
			if !present || value == nil {
				continue
			}
			list := asList(value)
			if list == nil {
				return true
			}
			for _, entry := range list {
				if asMap(entry) == nil {
					return true
				}
			}
		}
		return false
	}
	modelFieldOverflow := map[string]bool{}
	modelFieldIncomplete := map[string]bool{}
	addModels := func(list []any) {
		for _, entry := range list {
			mapped := asMap(entry)
			if mapped == nil {
				continue
			}
			name := stringValue(pick(mapped, "name", "model", "id"))
			if name == "" {
				continue
			}
			model := modelTotals[name]
			if model == nil {
				model = &domain.ModelTotal{Name: name}
				modelTotals[name] = model
			}
			for _, field := range []struct {
				keys  []string
				label string
				set   func(*int64)
				get   func() *int64
			}{
				{[]string{"input", "inputTokens", "input_tokens"}, "input", func(v *int64) { model.Input = v }, func() *int64 { return model.Input }},
				{[]string{"output", "outputTokens", "output_tokens"}, "output", func(v *int64) { model.Output = v }, func() *int64 { return model.Output }},
			} {
				value, present := lookup(mapped, field.keys...)
				marker := name + "/" + field.label
				if !present {
					modelFieldIncomplete[marker] = true
					continue
				}
				if modelFieldOverflow[marker] {
					continue
				}
				parsed := intPointer(value)
				if parsed == nil {
					modelFieldIncomplete[marker] = true
					continue
				}
				if sum := addInt64(field.get(), *parsed); sum != nil {
					field.set(sum)
				} else {
					tokenOverflow = true
					modelFieldOverflow[marker] = true
					field.set(nil)
				}
			}
		}
	}

	var total int64
	var haveTotal bool
	reportedIncomplete := 0
	var costTotal float64
	var haveCost, priced bool
	costEnvelopes := 0
	currency := ""
	costIncompatible := false
	costUnknownCurrency := false
	costCoverageIncomplete := false
	costOverflow := false
	costProvenanceSet := false
	costReportedOnly := true
	costMixedProvenance := false
	coverage, provenance, timezone := "", "", ""
	for _, group := range groups {
		groupTimezone := stringValue(pick(group.top, "timezone", "timeZone"))
		if chosenTimezone != "" && groupTimezone != "" && groupTimezone != chosenTimezone {
			history.Partial = true
			if siblingErr == nil {
				siblingErr = scoped(domain.ErrUnsupported, key.String(), "history timezones are incompatible")
			}
			continue
		}
		groupReader := stringValue(pick(group.top, "reportedSource", "reported_source", "source", "sourceMode", "source_mode"))
		if chosenReader != "" && groupReader != "" && groupReader != chosenReader {
			history.Partial = true
			if siblingErr == nil {
				siblingErr = scoped(domain.ErrUnsupported, key.String(), "history readers are incompatible")
			}
			continue
		}
		var groupTotal int64
		var groupHaveTotal bool
		groupRejected := false
		var acceptedRecords []map[string]any
		for _, record := range group.records {
			if provider := stringValue(pick(record, "provider", "providerID", "provider_id")); provider != "" && provider != key.Provider {
				history.Partial = true
				groupRejected = true
				if siblingErr == nil {
					siblingErr = scoped(domain.ErrUnsupported, key.String(), "history record belongs to another provider")
				}
				continue
			}
			if reader := stringValue(pick(record, "reportedSource", "reported_source", "source", "sourceMode", "source_mode")); chosenReader != "" && reader != "" && reader != chosenReader {
				history.Partial = true
				groupRejected = true
				if siblingErr == nil {
					siblingErr = scoped(domain.ErrUnsupported, key.String(), "history record belongs to another reader")
				}
				continue
			}
			if recordTimezone := stringValue(pick(record, "timezone", "timeZone")); chosenTimezone != "" && recordTimezone != "" && recordTimezone != chosenTimezone {
				history.Partial = true
				groupRejected = true
				if siblingErr == nil {
					siblingErr = scoped(domain.ErrUnsupported, key.String(), "history record uses an incompatible timezone")
				}
				continue
			}
			if category := errorCategory(record); category != "" {
				groupRejected = true
				if siblingErr == nil {
					siblingErr = scoped(category, key.String(), "backend reported an error")
				}
			}
			acceptedRecords = append(acceptedRecords, record)
			day := stringValue(pick(record, "day", "date", "bucket", "start"))
			if _, dateErr := time.Parse("2006-01-02", day); dateErr != nil {
				if siblingErr == nil {
					siblingErr = scoped(domain.ErrUnsupported, key.String(), "history bucket has an invalid date")
				}
				continue
			}
			bucket := domain.HistoryDay{Day: day}
			tokensValue := pick(record, "tokens", "totalTokens", "total_tokens", "tokenCount", "token_count")
			if tokens := intPointer(tokensValue); tokens != nil {
				bucket.Tokens = tokens
				if sum := addInt64(&groupTotal, *tokens); sum != nil {
					groupTotal = *sum
					groupHaveTotal = true
				} else {
					tokenOverflow = true
				}
			} else {
				history.MissingDays = append(history.MissingDays, day)
			}
			if boolValue(pick(record, "incomplete", "missing", "unavailable")) {
				history.MissingDays = append(history.MissingDays, day)
				history.Partial = true
			}
			history.Buckets = append(history.Buckets, bucket)
		}
		if modelsMalformed(group.top) {
			history.Partial = true
			if siblingErr == nil {
				siblingErr = scoped(domain.ErrUnsupported, key.String(), "history model structure is not supported")
			}
		}
		topModels := append(asList(pick(group.top, "models")), asList(pick(group.top, "modelBreakdowns"))...)
		if len(topModels) > 0 && !groupRejected {
			addModels(topModels)
		} else {
			for _, record := range acceptedRecords {
				if modelsMalformed(record) {
					history.Partial = true
					if siblingErr == nil {
						siblingErr = scoped(domain.ErrUnsupported, key.String(), "history model structure is not supported")
					}
				}
				models := append(asList(pick(record, "models")), asList(pick(record, "modelBreakdowns"))...)
				addModels(models)
			}
		}
		totals := asMap(pick(group.top, "totals"))
		explicit := intPointer(pick(group.top, "totalTokens", "total_tokens", "tokens"))
		if explicit == nil && totals != nil {
			explicit = intPointer(pick(totals, "totalTokens", "total_tokens", "tokens"))
		}
		if explicit != nil && !groupRejected {
			if sum := addInt64(&total, *explicit); sum != nil {
				total = *sum
				haveTotal = true
			} else {
				tokenOverflow = true
			}
		} else if groupHaveTotal {
			if sum := addInt64(&total, groupTotal); sum != nil {
				total = *sum
				haveTotal = true
			} else {
				tokenOverflow = true
			}
		}
		groupCostHandled := false
		cost := asMap(pick(group.top, "cost", "totalCost", "providerCost", "provider_cost"))
		if cost == nil && totals != nil {
			cost = asMap(pick(totals, "cost", "totalCost", "providerCost", "provider_cost"))
		}
		if cost != nil && !groupRejected {
			if amount, ok := floatValue(pick(cost, "amount", "total", "value")); ok {
				reported := boolValue(pick(cost, "reported", "reportedCost")) || boolValue(pick(group.top, "costReported", "reportedCost"))
				if totals != nil {
					reported = reported || boolValue(pick(totals, "costReported", "reportedCost"))
				}
				groupCostHandled = true
				if !costProvenanceSet {
					costProvenanceSet = true
					costReportedOnly = reported
				} else if reported != costReportedOnly {
					costMixedProvenance = true
				}
				costEnvelopes++
				value := stringValue(pick(cost, "currency", "unit"))
				switch {
				case value == "":
					costUnknownCurrency = true
				case currency == "":
					currency = value
					costTotal += amount
					if math.IsInf(costTotal, 0) || math.IsNaN(costTotal) {
						costOverflow = true
					}
					haveCost = true
				case value == currency:
					costTotal += amount
					if math.IsInf(costTotal, 0) || math.IsNaN(costTotal) {
						costOverflow = true
					}
					haveCost = true
				default:
					costIncompatible = true
				}
			}
		} else if !groupRejected {
			if amount, ok := floatValue(pick(group.top, "cost", "totalCost", "amount")); ok {
				groupCostHandled = true
				reported := boolValue(pick(group.top, "costReported", "reportedCost"))
				if !costProvenanceSet {
					costProvenanceSet = true
					costReportedOnly = reported
				} else if reported != costReportedOnly {
					costMixedProvenance = true
				}
				costEnvelopes++
				value := stringValue(pick(group.top, "currency", "unit"))
				switch {
				case value == "":
					costUnknownCurrency = true
				case currency == "":
					currency = value
					costTotal += amount
					haveCost = true
				case value == currency:
					costTotal += amount
					haveCost = true
				default:
					costIncompatible = true
				}
			}
		}
		if !groupCostHandled && totals != nil && !groupRejected {
			if amount, ok := floatValue(pick(totals, "cost", "totalCost", "amount")); ok {
				groupCostHandled = true
				costEnvelopes++
				reported := boolValue(pick(totals, "costReported", "reportedCost")) || boolValue(pick(group.top, "costReported", "reportedCost"))
				if !costProvenanceSet {
					costProvenanceSet = true
					costReportedOnly = reported
				} else if reported != costReportedOnly {
					costMixedProvenance = true
				}
				value := stringValue(pick(totals, "currency", "unit"))
				switch {
				case value == "":
					costUnknownCurrency = true
				case currency == "":
					currency = value
					costTotal += amount
					if math.IsInf(costTotal, 0) || math.IsNaN(costTotal) {
						costOverflow = true
					}
					haveCost = true
				case value == currency:
					costTotal += amount
					if math.IsInf(costTotal, 0) || math.IsNaN(costTotal) {
						costOverflow = true
					}
					haveCost = true
				default:
					costIncompatible = true
				}
			}
		}
		if boolValue(pick(group.top, "priced")) {
			priced = true
		}
		if boolValue(pick(group.top, "partial", "incomplete")) {
			history.Partial = true
		}
		incompleteCount := intPointer(pick(group.top, "incomplete", "incompleteCount", "incomplete_count"))
		if incompleteCount == nil && totals != nil {
			incompleteCount = intPointer(pick(totals, "incomplete", "incompleteCount", "incomplete_count"))
		}
		if incompleteCount != nil {
			reportedIncomplete += int(*incompleteCount)
			if *incompleteCount > 0 {
				history.Partial = true
			}
		}
		if value := stringValue(pick(group.top, "coverage", "scope")); value != "" && !strings.Contains(coverage, value) {
			if coverage != "" {
				coverage += "; "
			}
			coverage += value
		}
		if value := stringValue(pick(group.top, "provenance", "source")); value != "" && !strings.Contains(provenance, value) {
			if provenance != "" {
				provenance += "; "
			}
			provenance += value
		}
		if timezone == "" {
			timezone = stringValue(pick(group.top, "timezone", "timeZone"))
		}
	}
	if math.IsInf(costTotal, 0) || math.IsNaN(costTotal) {
		costOverflow = true
	}
	if haveTotal && !tokenOverflow {
		history.TotalTokens = &total
	}
	for _, model := range modelTotals {
		history.Models = append(history.Models, *model)
	}
	sort.Slice(history.Models, func(i, j int) bool { return history.Models[i].Name < history.Models[j].Name })
	if costEnvelopes > 0 && costEnvelopes < len(groups) {
		costCoverageIncomplete = true
	}
	if haveCost && !costIncompatible && !costMixedProvenance && !costOverflow {
		history.Cost = &domain.Money{Amount: costTotal, Currency: currency}
		history.Currency = currency
	}
	if costOverflow {
		if coverage != "" {
			coverage += "; "
		}
		coverage += "pricing incomplete: monetary overflow"
		history.Partial = true
	}
	if costMixedProvenance {
		if coverage != "" {
			coverage += "; "
		}
		coverage += "costs omitted: mixed reported and estimated provenance"
	}
	if costIncompatible {
		if coverage != "" {
			coverage += "; "
		}
		coverage += "costs omitted: incompatible currencies"
	}
	if costCoverageIncomplete {
		if coverage != "" {
			coverage += "; "
		}
		coverage += "pricing incomplete"
	}
	if costUnknownCurrency {
		if coverage != "" {
			coverage += "; "
		}
		coverage += "costs omitted: unknown currency"
		history.Partial = true
	}
	if tokenOverflow {
		if coverage != "" {
			coverage += "; "
		}
		coverage += "token totals incomplete: overflow"
		history.Partial = true
	}
	for marker := range modelFieldIncomplete {
		if !modelFieldOverflow[marker] {
			history.Partial = true
		}
	}
	history.Priced = haveCost || priced || costEnvelopes > 0
	history.CostReported = costReportedOnly && costProvenanceSet && !costMixedProvenance
	history.Timezone = timezone
	history.Coverage = coverage
	history.Provenance = provenance
	if len(history.Buckets) == 0 && !recognizedEmpty {
		if siblingErr != nil {
			return domain.HistorySnapshot{}, siblingErr
		}
		return domain.HistorySnapshot{}, scoped(domain.ErrUnsupported, key.String(), "history has no valid daily buckets")
	}
	merged := map[string]domain.HistoryDay{}
	for _, bucket := range history.Buckets {
		existing, ok := merged[bucket.Day]
		if !ok {
			merged[bucket.Day] = bucket
			continue
		}
		if existing.Tokens != nil && bucket.Tokens != nil {
			if sum := addInt64(existing.Tokens, *bucket.Tokens); sum != nil {
				existing.Tokens = sum
			} else {
				tokenOverflow = true
				existing.Tokens = nil
			}
		} else if existing.Tokens == nil && bucket.Tokens != nil {
			existing.Tokens = bucket.Tokens
			history.MissingDays = append(history.MissingDays, bucket.Day)
		} else if existing.Tokens != nil && bucket.Tokens == nil {
			history.MissingDays = append(history.MissingDays, bucket.Day)
		} else {
			history.MissingDays = append(history.MissingDays, bucket.Day)
		}
		merged[bucket.Day] = existing
	}
	history.Buckets = history.Buckets[:0]
	for _, bucket := range merged {
		history.Buckets = append(history.Buckets, bucket)
	}
	sort.Slice(history.Buckets, func(i, j int) bool { return history.Buckets[i].Day < history.Buckets[j].Day })
	history.MissingDays = uniqueStrings(history.MissingDays)
	history.Incomplete = len(history.MissingDays)
	if reportedIncomplete > history.Incomplete {
		history.Incomplete = reportedIncomplete
	}
	history.Partial = history.Partial || history.Incomplete > 0 || siblingErr != nil || costCoverageIncomplete
	if len(history.Buckets) > 0 {
		history.Start, history.End = historyRange(history.Buckets, req.Days)
	}
	return history, siblingErr
}

func historyRange(buckets []domain.HistoryDay, days int) (time.Time, time.Time) {
	var first, last time.Time
	for _, bucket := range buckets {
		parsed, err := time.Parse("2006-01-02", bucket.Day)
		if err != nil {
			continue
		}
		if first.IsZero() || parsed.Before(first) {
			first = parsed
		}
		if last.IsZero() || parsed.After(last) {
			last = parsed
		}
	}
	if first.IsZero() {
		now := time.Now().UTC()
		first = now.AddDate(0, 0, -(days - 1))
		last = now
	}
	return first, last
}

func addInt64(current *int64, value int64) *int64 {
	if current == nil {
		result := value
		return &result
	}
	if value > 0 && *current > math.MaxInt64-value {
		return nil
	}
	result := *current + value
	return &result
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
