package domain

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Quality describes how complete a reading is.
type Quality string

const (
	QualityMeasured    Quality = "measured"
	QualityPartial     Quality = "partial"
	QualityUnavailable Quality = "unavailable"
	QualityUnsupported Quality = "unsupported"
	QualityStale       Quality = "stale"
)

// ErrorCategory is the closed adapter failure taxonomy.
type ErrorCategory string

const (
	ErrMalformedJSON  ErrorCategory = "malformed_json"
	ErrEmptyStdout    ErrorCategory = "empty_stdout"
	ErrProcessFailure ErrorCategory = "process_failure"
	ErrTimeout        ErrorCategory = "timeout"
	ErrAuth           ErrorCategory = "auth"
	ErrUnavailable    ErrorCategory = "unavailable_quota"
	ErrUnsupported    ErrorCategory = "unsupported_shape"
	ErrOutputCap      ErrorCategory = "output_cap"
	ErrConfig         ErrorCategory = "config"
	ErrUnknown        ErrorCategory = "unknown"
)

// UXState maps an error category to the state the interface renders.
func (c ErrorCategory) UXState() SourceStatus {
	switch c {
	case ErrAuth:
		return StatusSignIn
	case ErrTimeout, ErrProcessFailure:
		return StatusStale
	case ErrUnavailable:
		return StatusUnavailable
	case ErrUnsupported:
		return StatusBackendIncompatible
	case ErrMalformedJSON, ErrEmptyStdout, ErrOutputCap:
		return StatusError
	default:
		return StatusError
	}
}

// UserMessage is safe product copy. It never carries raw stderr or secrets.
func (c ErrorCategory) UserMessage() string {
	switch c {
	case ErrMalformedJSON:
		return "The backend returned unreadable data."
	case ErrEmptyStdout:
		return "The backend returned no data."
	case ErrProcessFailure:
		return "The backend could not read this source."
	case ErrTimeout:
		return "The backend took too long to respond."
	case ErrAuth:
		return "Sign-in needed for this source."
	case ErrUnavailable:
		return "This source has no current allowance."
	case ErrUnsupported:
		return "This backend response is not supported yet."
	case ErrOutputCap:
		return "The backend response was too large to read safely."
	case ErrConfig:
		return "The backend configuration needs attention."
	default:
		return "This source could not be refreshed."
	}
}

// ScopedError attaches a safe failure to one source or operation.
type ScopedError struct {
	Category  ErrorCategory
	SourceKey string
	Detail    string
}

// Error implements error without exposing diagnostics or sensitive detail.
func (e *ScopedError) Error() string {
	if e == nil {
		return ""
	}
	return e.Category.UserMessage()
}

// NewScopedError builds a typed backend-agnostic failure.
func NewScopedError(category ErrorCategory, sourceKey, detail string) *ScopedError {
	return &ScopedError{Category: category, SourceKey: sourceKey, Detail: detail}
}

// AsScopedError recovers a typed failure through the Backend error seam.
func AsScopedError(err error) (*ScopedError, bool) {
	var scoped *ScopedError
	if errors.As(err, &scoped) {
		return scoped, true
	}
	return nil, false
}

// AllowanceWindow is one reported quota window.
type AllowanceWindow struct {
	Key         string
	Label       string
	Group       string
	UsedPercent *float64
	Used        *float64
	Limit       *float64
	Unit        string
	Duration    time.Duration
	ResetAt     *time.Time
	Quality     Quality
}

// RemainingPercent derives percentage left for a valid used percentage.
func (w AllowanceWindow) RemainingPercent() *float64 {
	if w.UsedPercent == nil || math.IsNaN(*w.UsedPercent) || math.IsInf(*w.UsedPercent, 0) || *w.UsedPercent < 0 {
		return nil
	}
	left := 100 - *w.UsedPercent
	if left < 0 {
		left = 0
	}
	return &left
}

// OverLimit reports a valid used percentage above 100.
func (w AllowanceWindow) OverLimit() bool {
	return w.UsedPercent != nil && !math.IsNaN(*w.UsedPercent) && !math.IsInf(*w.UsedPercent, 0) && *w.UsedPercent > 100
}

// Balance is credits or money, never an implicit quota. A nil Value means
// unavailable, which is distinct from a measured zero.
type Balance struct {
	Label    string
	Value    *float64
	Unit     string
	Kind     string
	SampleAt *time.Time
}

// DetailRow is a label/value pair from supported detail payloads.
type DetailRow struct {
	Label string
	Value string
}

// DetailBar is one supported proportional bar. Nil means unavailable; zero is
// a measured zero.
type DetailBar struct {
	Label string
	Value *float64
	Max   *float64
	Unit  string
}

// ChartPoint is one supported detail chart sample. Nil means unavailable.
type ChartPoint struct {
	Label string
	Value *float64
}

// DetailSection is a provider-specific detail family. Unsupported marks a shape
// the application deliberately does not render yet.
type DetailSection struct {
	Title       string
	Rows        []DetailRow
	Bars        []DetailBar
	Points      []ChartPoint
	Unsupported bool
}

// UsageSnapshot is one normalized reading for one source and account.
type UsageSnapshot struct {
	Key            SourceKey
	ProviderID     string
	DisplayName    string
	AccountKey     string
	AccountLabel   string
	Plan           string
	ReportedSource string
	SampleAt       *time.Time
	ReceivedAt     time.Time
	Windows        []AllowanceWindow
	Balances       []Balance
	Details        []DetailSection
	Quality        Quality
	Error          *ScopedError
	State          SourceStatus
}

// HistoryDay is one daily bucket. Tokens nil means unavailable, not zero.
type HistoryDay struct {
	Day    string
	Tokens *int64
}

// ModelTotal is one model's aggregate token mix. Nil values are unavailable;
// zero is a measured zero.
type ModelTotal struct {
	Name   string
	Input  *int64
	Output *int64
}

// Money is a reported or estimated monetary amount with its own currency.
type Money struct {
	Amount   float64
	Currency string
}

// HistorySnapshot is normalized local history. TotalTokens, model values, and
// Cost are optional. Timezone identifies the bucket boundary.
type HistorySnapshot struct {
	Key         SourceKey
	Days        int
	Start       time.Time
	End         time.Time
	Timezone    string
	Buckets     []HistoryDay
	TotalTokens *int64
	Models      []ModelTotal
	Cost        *Money
	Currency    string
	Priced      bool
	Provenance  string
	Coverage    string
	Partial     bool
	Incomplete  int
	MissingDays []string
}

// RefreshState tracks scheduling without conflating attempt and success.
type RefreshState struct {
	LastAttempt *time.Time
	LastSuccess *time.Time
	InFlight    bool
	Failure     *ScopedError
	RetryAt     *time.Time
	Generation  uint64
}

// GaugeCells clamps a remaining percentage to a gauge width.
func GaugeCells(remaining *float64, width int) int {
	if remaining == nil || width <= 0 || !finite(*remaining) {
		return 0
	}
	value := math.Max(0, math.Min(100, *remaining))
	return int(math.Round(value / 100 * float64(width)))
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// GaugeText renders a ten-cell gauge, using ASCII when requested.
func GaugeText(remaining *float64, width int, ascii bool) string {
	if remaining == nil || !finite(*remaining) {
		return ""
	}
	filled := GaugeCells(remaining, width)
	full, empty := "█", "░"
	if ascii {
		full, empty = "#", "-"
	}
	return repeat(full, filled) + repeat(empty, width-filled)
}

func repeat(value string, count int) string {
	if count <= 0 {
		return ""
	}
	out := ""
	for range count {
		out += value
	}
	return out
}

// FreshnessInfo separates sample age from receipt age and records whether the
// sample time is known. A missing sample time is "time unknown", not "now".
type FreshnessInfo struct {
	SampleAge    time.Duration
	SampleKnown  bool
	ReceiptAge   time.Duration
	ReceiptKnown bool
	Stale        bool
}

// CheckFreshness applies the SPEC freshness rules.
func CheckFreshness(sample, received *time.Time, now time.Time, interval time.Duration, failed bool) FreshnessInfo {
	var info FreshnessInfo
	if sample != nil {
		info.SampleKnown = true
		info.SampleAge = nonNegative(now.Sub(*sample))
	}
	if received != nil {
		info.ReceiptKnown = true
		info.ReceiptAge = nonNegative(now.Sub(*received))
	}
	info.Stale = failed
	age, known := info.SampleAge, info.SampleKnown
	if !known {
		age, known = info.ReceiptAge, info.ReceiptKnown
	}
	if known && interval > 0 && age > 2*interval {
		info.Stale = true
	}
	return info
}

// AgeLabel is the card freshness copy.
func (f FreshnessInfo) AgeLabel() string {
	if !f.SampleKnown {
		return "time unknown"
	}
	return HumanDuration(f.SampleAge) + " ago"
}

func nonNegative(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	return d
}

// HumanDuration is compact age/countdown copy.
func HumanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := int(d.Seconds())
	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dm", seconds/60)
	case seconds < 86400:
		h, m := seconds/3600, (seconds%3600)/60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %02dm", h, m)
	default:
		days, hours := seconds/86400, (seconds%86400)/3600
		if hours == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd %02dh", days, hours)
	}
}

// ResetLabel renders the reset countdown and due state.
func ResetLabel(reset *time.Time, now time.Time) string {
	if reset == nil {
		return "Reset not reported"
	}
	if !reset.After(now) {
		return "Reset due · awaiting reading"
	}
	return "Resets in " + HumanDuration(reset.Sub(now))
}

// ResetAbsolute renders the full reset instant in local time with zone.
func ResetAbsolute(reset *time.Time, location *time.Location) string {
	if reset == nil {
		return "Reset not reported"
	}
	if location == nil {
		location = time.UTC
	}
	return reset.In(location).Format("2006-01-02 15:04 MST")
}

// DeriveState chooses the presentation state. Errors and quality markers take
// precedence. Availability is aggregated across windows, balances, and details
// so a partial reading never becomes current or wholly unavailable.
func DeriveState(snapshot UsageSnapshot, stale bool) SourceStatus {
	if snapshot.Error != nil {
		switch snapshot.Error.Category {
		case ErrAuth:
			if hasAvailableData(snapshot) {
				return StatusAuthFailed
			}
			return StatusSignIn
		case ErrTimeout, ErrProcessFailure:
			if hasAvailableData(snapshot) {
				return StatusStale
			}
			return StatusError
		default:
			return snapshot.Error.Category.UXState()
		}
	}
	switch snapshot.Quality {
	case QualityUnsupported:
		return StatusBackendIncompatible
	case QualityUnavailable:
		return StatusUnavailable
	case QualityStale:
		return StatusStale
	}
	if stale {
		return StatusStale
	}
	if snapshot.Quality == QualityPartial {
		return StatusPartial
	}
	switch snapshot.State {
	case StatusStale, StatusSignIn, StatusUnavailable, StatusAuthFailed:
		return snapshot.State
	}

	windows := analyzeWindows(snapshot.Windows)
	if windows.stale {
		return StatusStale
	}
	if windows.unsupported {
		return StatusBackendIncompatible
	}
	balancesAvailable, balancesUnavailable := balanceAvailability(snapshot.Balances)
	detailsAvailable, detailsPartial, detailsInvalid, detailsUnsupported := detailAvailability(snapshot.Details)
	hasAvailable := windows.measured > 0 || balancesAvailable || detailsAvailable
	if windows.partial {
		return StatusPartial
	}
	incomplete := windows.unavailable > 0 || windows.unmeasured > 0 || balancesUnavailable || detailsPartial || detailsInvalid
	if incomplete && hasAvailable {
		return StatusPartial
	}
	if windows.measured > 0 {
		if windows.exhausted {
			return StatusExhausted
		}
		if windows.low {
			return StatusLow
		}
		return StatusCurrent
	}
	if balancesAvailable || detailsAvailable {
		return StatusCurrent
	}
	if windows.unavailable > 0 || balancesUnavailable || detailsInvalid {
		return StatusUnavailable
	}
	if detailsUnsupported {
		return StatusBackendIncompatible
	}
	if snapshot.State == StatusSignIn || snapshot.State == StatusUnavailable || snapshot.State == StatusAuthFailed {
		return snapshot.State
	}
	return StatusMissingWindow
}

type windowAnalysis struct {
	measured    int
	unavailable int
	unmeasured  int
	low         bool
	exhausted   bool
	partial     bool
	stale       bool
	unsupported bool
}

func analyzeWindows(windows []AllowanceWindow) windowAnalysis {
	var out windowAnalysis
	for _, window := range windows {
		switch window.Quality {
		case QualityUnsupported:
			out.unsupported = true
			continue
		case QualityStale:
			out.stale = true
			continue
		case QualityUnavailable:
			out.unavailable++
			continue
		case QualityPartial:
			out.partial = true
		}
		if !windowMeasured(window) {
			out.unmeasured++
			continue
		}
		out.measured++
		remaining := window.RemainingPercent()
		if remaining == nil {
			continue
		}
		if *remaining <= 0 {
			out.exhausted = true
		} else if *remaining <= 20 {
			out.low = true
		}
	}
	return out
}

// EffectiveState returns the explicit state when present, otherwise derives it.
func EffectiveState(snapshot UsageSnapshot) SourceStatus {
	if snapshot.Error == nil && (snapshot.State == StatusQueued || snapshot.State == StatusFetching) {
		return snapshot.State
	}
	if snapshot.Error != nil || !measuredQuality(snapshot.Quality) {
		return DeriveState(snapshot, false)
	}
	switch snapshot.State {
	case StatusStale, StatusSignIn, StatusUnavailable, StatusAuthFailed:
		return snapshot.State
	}
	return DeriveState(snapshot, false)
}

func measuredQuality(quality Quality) bool {
	return quality == "" || quality == QualityMeasured
}

func windowMeasured(window AllowanceWindow) bool {
	if window.RemainingPercent() != nil {
		return true
	}
	if window.Used == nil || !finite(*window.Used) || *window.Used < 0 {
		return false
	}
	if window.Limit != nil && finite(*window.Limit) && *window.Limit > 0 {
		return true
	}
	return window.Unit != ""
}

func balanceAvailability(balances []Balance) (available, unavailable bool) {
	for _, balance := range balances {
		if balance.Value != nil && finite(*balance.Value) {
			available = true
		} else {
			unavailable = true
		}
	}
	return available, unavailable
}

func detailAvailability(details []DetailSection) (available, partial, invalid, unsupported bool) {
	for _, detail := range details {
		if detail.Unsupported {
			unsupported = true
			continue
		}
		content, bad := len(detail.Rows) > 0, false
		for _, bar := range detail.Bars {
			if barMeasured(bar) {
				content = true
			} else {
				bad = true
			}
		}
		for _, point := range detail.Points {
			if point.Value != nil && finite(*point.Value) {
				content = true
			} else {
				bad = true
			}
		}
		if content {
			available = true
			if bad {
				partial = true
			}
		} else if bad {
			invalid = true
		}
	}
	return available, partial, invalid, unsupported
}

func barMeasured(bar DetailBar) bool {
	if bar.Value == nil || !finite(*bar.Value) {
		return false
	}
	if bar.Max != nil && (!finite(*bar.Max) || *bar.Max <= 0) {
		return false
	}
	return true
}

func hasAvailableData(snapshot UsageSnapshot) bool {
	for _, window := range snapshot.Windows {
		if windowMeasured(window) {
			return true
		}
	}
	if available, _ := balanceAvailability(snapshot.Balances); available {
		return true
	}
	available, _, _, _ := detailAvailability(snapshot.Details)
	return available
}

// CountStates summarizes a collection.
func CountStates(snapshots []UsageSnapshot) Counts {
	var counts Counts
	counts.Total = len(snapshots)
	for _, snapshot := range snapshots {
		state := EffectiveState(snapshot)
		switch state {
		case StatusCurrent, StatusLow, StatusExhausted:
			counts.Current++
		case StatusSignIn, StatusAuthFailed:
			counts.SignIn++
		case StatusUnavailable:
			counts.Unavailable++
		case StatusStale:
			counts.Stale++
		case StatusFetching:
			counts.Fetching++
		case StatusQueued:
			counts.Queued++
		}
		if state.NeedsAttention() {
			counts.Attention++
		}
		if state == StatusLow {
			counts.Low++
		}
	}
	return counts
}

// SortSnapshots orders snapshots by pinned descriptors when provided.
func SortSnapshots(snapshots []UsageSnapshot, pinned map[string]int) {
	sort.SliceStable(snapshots, func(i, j int) bool {
		pi, iok := pinned[snapshots[i].Key.String()]
		pj, jok := pinned[snapshots[j].Key.String()]
		if iok != jok {
			return iok
		}
		if iok && pi != pj {
			return pi < pj
		}
		li, lj := lowerTrim(snapshots[i].DisplayName), lowerTrim(snapshots[j].DisplayName)
		if li != lj {
			return li < lj
		}
		if snapshots[i].DisplayName != snapshots[j].DisplayName {
			return snapshots[i].DisplayName < snapshots[j].DisplayName
		}
		return snapshots[i].Key.String() < snapshots[j].Key.String()
	})
}

// Filter matches a snapshot against the Overview filters.
func Filter(snapshot UsageSnapshot, filter string, query string, pinned bool) bool {
	switch filter {
	case "attention":
		if !EffectiveState(snapshot).NeedsAttention() {
			return false
		}
	case "pinned":
		if !pinned {
			return false
		}
	}
	query = lowerTrim(query)
	if query == "" {
		return true
	}
	return containsFold(snapshot.DisplayName, query) || containsFold(snapshot.Key.String(), query)
}

func lowerTrim(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func containsFold(value, lowerQuery string) bool {
	lower := lowerTrim(value)
	for i := 0; i+len(lowerQuery) <= len(lower); i++ {
		if lower[i:i+len(lowerQuery)] == lowerQuery {
			return true
		}
	}
	return false
}
