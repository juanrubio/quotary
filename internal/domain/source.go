package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	providerPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	variantPattern  = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

// Variant is a supported backend source mode. Configured means Quotary leaves
// source selection to the backend.
type Variant string

const (
	VariantConfigured Variant = "configured"
	VariantWeb        Variant = "web"
	VariantAPI        Variant = "api"
)

// UnknownAccount is the explicit sentinel for a reading without backend-provided
// account identity. It never stands in for an observed account.
const UnknownAccount = "unknown"

// SourceKey identifies one trackable source and, when known, one account.
type SourceKey struct {
	Provider       string
	Variant        Variant
	Account        string
	AccountUnknown bool
}

// ParseSourceKey parses provider[:variant][@account]. Accounts are
// percent-encoded for reserved characters. The literal @unknown is the
// missing-identity sentinel; a backend account literally named "unknown" is
// canonicalized as @%75nknown so the two never collide.
func ParseSourceKey(raw string) (SourceKey, error) {
	if raw == "" {
		return SourceKey{}, errors.New("empty source key")
	}
	base, encodedAccount := raw, ""
	hasAccount := false
	if idx := strings.LastIndex(raw, "@"); idx >= 0 {
		base, encodedAccount = raw[:idx], raw[idx+1:]
		hasAccount = true
		if encodedAccount == "" {
			return SourceKey{}, fmt.Errorf("empty account in source key %q", raw)
		}
		if err := validateEncodedAccount(encodedAccount); err != nil {
			return SourceKey{}, fmt.Errorf("source key account: %w", err)
		}
	}
	provider, variant, err := parseBase(base)
	if err != nil {
		return SourceKey{}, err
	}
	key := SourceKey{Provider: provider, Variant: variant}
	if !hasAccount {
		return key, nil
	}
	if encodedAccount == UnknownAccount {
		key.Account = UnknownAccount
		key.AccountUnknown = true
		return key, nil
	}
	decoded, err := unescape(encodedAccount)
	if err != nil {
		return SourceKey{}, fmt.Errorf("source key account: %w", err)
	}
	if !utf8.ValidString(decoded) {
		return SourceKey{}, fmt.Errorf("source key account is not valid UTF-8")
	}
	key.Account = decoded
	return key, nil
}

func parseBase(base string) (string, Variant, error) {
	provider, variant := base, ""
	if idx := strings.Index(base, ":"); idx >= 0 {
		provider, variant = base[:idx], base[idx+1:]
		if variant == "" {
			return "", "", fmt.Errorf("empty variant in source key %q", base)
		}
	}
	if !providerPattern.MatchString(provider) {
		return "", "", fmt.Errorf("invalid provider %q", provider)
	}
	if variant != "" && !variantPattern.MatchString(variant) {
		return "", "", fmt.Errorf("invalid variant %q", variant)
	}
	return provider, Variant(variant), nil
}

// CoalescesWith reports whether two source keys identify the same refresh scope.
// Known and unknown account identities never coalesce.
func (k SourceKey) CoalescesWith(other SourceKey) bool {
	return k.String() == other.String()
}

func validateEncodedAccount(value string) error {
	for i := 0; i < len(value); {
		if value[i] == '%' {
			if i+2 >= len(value) || !isHex(value[i+1]) || !isHex(value[i+2]) {
				return fmt.Errorf("invalid escape in %q", value)
			}
			i += 3
			continue
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && size == 1 {
			return fmt.Errorf("invalid UTF-8 in %q", value)
		}
		if unicode.IsControl(r) || r == ' ' || r == ':' || r == '@' || r == '/' {
			return fmt.Errorf("reserved or control character must be escaped in %q", value)
		}
		i += size
	}
	return nil
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// String formats the canonical key. An empty account is omitted; the explicit
// UnknownAccount sentinel is rendered as @unknown.
func (k SourceKey) String() string {
	base := k.Provider
	if k.Variant != "" {
		base += ":" + string(k.Variant)
	}
	if k.Account != "" {
		base += "@" + encodeAccount(k.Account, k.AccountUnknown)
	}
	return base
}

func encodeAccount(account string, unknown bool) string {
	if unknown {
		return UnknownAccount
	}
	if account == UnknownAccount {
		return "%75" + escape(account[1:])
	}
	return escape(account)
}

// ValidateVariant rejects variants the catalog manifest does not support.
func (k SourceKey) ValidateVariant(allowed map[Variant]bool) error {
	if k.Variant == "" {
		return nil
	}
	if !allowed[k.Variant] {
		return fmt.Errorf("unsupported source variant %q for %s", k.Variant, k.Provider)
	}
	return nil
}

func escape(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r == '%' || r == ':' || r == '@' || r == '/' || r == ' ' || unicode.IsControl(r) {
			for _, raw := range []byte(string(r)) {
				fmt.Fprintf(&b, "%%%02X", raw)
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func unescape(value string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '%' {
			b.WriteByte(value[i])
			continue
		}
		if i+2 >= len(value) || !isHex(value[i+1]) || !isHex(value[i+2]) {
			return "", fmt.Errorf("invalid escape in %q", value)
		}
		b.WriteByte(hexValue(value[i+1])<<4 | hexValue(value[i+2]))
		i += 2
	}
	return b.String(), nil
}

func hexValue(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

// ServiceKind is a catalog classification, never inferred from a reading.
type ServiceKind string

const (
	KindSubscription ServiceKind = "subscription"
	KindCredits      ServiceKind = "credits"
	KindMeteredAPI   ServiceKind = "metered_api"
	KindMixed        ServiceKind = "mixed"
	KindNonUsage     ServiceKind = "non_usage"
	KindUnknown      ServiceKind = "unknown"
)

// CapabilityState preserves undiscovered capability information instead of
// collapsing unknown into a confirmed false.
type CapabilityState string

const (
	CapabilityUnknown     CapabilityState = "unknown"
	CapabilitySupported   CapabilityState = "supported"
	CapabilityUnsupported CapabilityState = "unsupported"
)

// Capabilities describes what the catalog says a provider can expose.
type Capabilities struct {
	Usage          CapabilityState
	History        CapabilityState
	Balances       CapabilityState
	Details        CapabilityState
	SourceVariants []Variant
	ActiveProbe    CapabilityState
}

// Supported reports confirmed support only.
func (s CapabilityState) Supported() bool {
	return s == CapabilitySupported
}

// BulkTrackable is conservative: known subscription/mixed kinds, confirmed usage
// support, and a non-probe source.
func (c Capabilities) BulkTrackable(kind ServiceKind) bool {
	if kind != KindSubscription && kind != KindMixed {
		return false
	}
	return c.Usage.Supported() && c.ActiveProbe == CapabilityUnsupported
}

// SourceDescriptor is backend-supplied catalog metadata. Tracked and Pinned are
// presentation inputs merged from preferences at the application layer.
type SourceDescriptor struct {
	Key            SourceKey
	ProviderID     string
	DisplayName    string
	Variant        Variant
	Kind           ServiceKind
	Capabilities   Capabilities
	Available      bool
	Enabled        bool
	DefaultEnabled bool
	Tracked        bool
	Pinned         bool
	PinOrder       int
	LastRead       SourceStatus
}

// SourceStatus is the presentation state of one source.
type SourceStatus string

const (
	StatusUnknown             SourceStatus = "unknown"
	StatusQueued              SourceStatus = "queued"
	StatusFetching            SourceStatus = "fetching"
	StatusCurrent             SourceStatus = "current"
	StatusLow                 SourceStatus = "low"
	StatusExhausted           SourceStatus = "exhausted"
	StatusSignIn              SourceStatus = "sign_in"
	StatusAuthFailed          SourceStatus = "auth_failed"
	StatusStale               SourceStatus = "stale"
	StatusMissingWindow       SourceStatus = "missing_window"
	StatusUnavailable         SourceStatus = "unavailable"
	StatusPartial             SourceStatus = "partial"
	StatusBackendIncompatible SourceStatus = "backend_incompatible"
	StatusError               SourceStatus = "error"
)

// NeedsAttention reports whether the source belongs in the attention filter.
func (s SourceStatus) NeedsAttention() bool {
	switch s {
	case StatusLow, StatusExhausted, StatusSignIn, StatusAuthFailed, StatusStale,
		StatusMissingWindow, StatusUnavailable, StatusPartial, StatusBackendIncompatible, StatusError:
		return true
	default:
		return false
	}
}

// Counts summarizes source states. Current includes low and exhausted readings;
// attention intentionally overlaps current.
type Counts struct {
	Total       int
	Current     int
	Attention   int
	Low         int
	SignIn      int
	Unavailable int
	Stale       int
	Fetching    int
	Queued      int
}

// Ordering is the default source ordering.
type Ordering struct {
	PinnedFirst bool
	ByState     bool
}

// SortSources applies pinned-then-name ordering with a source-key tie-break.
func SortSources(sources []SourceDescriptor) {
	sortSources(sources)
}

func sortSources(sources []SourceDescriptor) {
	for i := 1; i < len(sources); i++ {
		for j := i; j > 0 && lessSource(sources[j], sources[j-1]); j-- {
			sources[j], sources[j-1] = sources[j-1], sources[j]
		}
	}
}

func lessSource(a, b SourceDescriptor) bool {
	if a.Pinned != b.Pinned {
		return a.Pinned
	}
	if a.Pinned && a.PinOrder != b.PinOrder {
		return a.PinOrder < b.PinOrder
	}
	la, lb := strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName)
	if la != lb {
		return la < lb
	}
	if a.DisplayName != b.DisplayName {
		return a.DisplayName < b.DisplayName
	}
	return a.Key.String() < b.Key.String()
}
