package demo

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/juanrubio/quotary/internal/domain"
)

//go:embed fixtures.json
var fixtureFS embed.FS

// FixtureFS exposes the embedded fictional fixtures as an in-memory filesystem
// seam. Demo startup uses this FS and never the operating-system filesystem.
func FixtureFS() fs.FS { return fixtureFS }

type fixtures struct {
	Fictional bool             `json:"fictional"`
	Clock     string           `json:"clock"`
	Timezone  string           `json:"timezone"`
	Notice    string           `json:"notice"`
	Sources   []fixtureSource  `json:"sources"`
	Catalog   []fixtureCatalog `json:"catalog"`
	History   fixtureHistory   `json:"history"`
}

type fixtureSource struct {
	ID       string          `json:"id"`
	Provider string          `json:"provider"`
	Name     string          `json:"name"`
	Plan     string          `json:"plan"`
	Windows  []fixtureWindow `json:"windows"`
	State    string          `json:"state"`
	Age      string          `json:"age"`
	Kind     string          `json:"kind"`
	History  bool            `json:"history"`
	Pinned   bool            `json:"pinned"`
	PinOrder int             `json:"pinOrder"`
}

type fixtureWindow struct {
	Label        string   `json:"label"`
	Left         *float64 `json:"left"`
	Reset        string   `json:"reset"`
	ResetMinutes *int     `json:"resetMinutes"`
	Group        *string  `json:"group"`
}

type fixtureCatalog struct {
	Provider       string `json:"provider"`
	Enabled        bool   `json:"enabled"`
	DefaultEnabled bool   `json:"defaultEnabled"`
	DisplayName    string `json:"displayName"`
}

type fixtureHistory struct {
	Source      string              `json:"source"`
	Days        []fixtureHistoryDay `json:"days"`
	TotalTokens int64               `json:"totalTokens"`
	Models      []fixtureModel      `json:"models"`
}

type fixtureHistoryDay struct {
	Day    string `json:"day"`
	Tokens *int64 `json:"tokens"`
}

type fixtureModel struct {
	Name   string `json:"name"`
	Input  int64  `json:"input"`
	Output int64  `json:"output"`
}

func loadFixtures() (fixtures, error) {
	return loadFixturesFrom(fixtureFS)
}

func loadFixturesFrom(fsys fs.FS) (fixtures, error) {
	raw, err := fs.ReadFile(fsys, "fixtures.json")
	if err != nil {
		return fixtures{}, fmt.Errorf("demo fixtures: %w", err)
	}
	var data fixtures
	if err := json.Unmarshal(raw, &data); err != nil {
		return fixtures{}, fmt.Errorf("demo fixtures: %w", err)
	}
	if !data.Fictional {
		return fixtures{}, fmt.Errorf("demo fixtures must be marked fictional")
	}
	if _, err := data.clock(); err != nil {
		return fixtures{}, err
	}
	return data, nil
}

func (f fixtures) clock() (time.Time, error) {
	value, err := time.Parse(time.RFC3339, f.Clock)
	if err != nil {
		return time.Time{}, fmt.Errorf("demo clock: %w", err)
	}
	return value, nil
}

func (f fixtures) snapshot(source fixtureSource, clock time.Time) domain.UsageSnapshot {
	key, err := domain.ParseSourceKey(source.ID)
	if err != nil {
		key = domain.SourceKey{Provider: source.Provider, Variant: domain.VariantConfigured}
	}
	key.Account = domain.UnknownAccount
	key.AccountUnknown = true
	sample := clock.Add(-parseAge(source.Age))
	var sampleAt *time.Time
	if source.Age != "" {
		sampleAt = &sample
	}
	snapshot := domain.UsageSnapshot{
		Key:          key,
		ProviderID:   source.Provider,
		DisplayName:  source.Name,
		AccountKey:   domain.UnknownAccount,
		AccountLabel: "Account unknown",
		Plan:         source.Plan,
		SampleAt:     sampleAt,
		ReceivedAt:   clock,
		Quality:      domain.QualityMeasured,
	}
	for index, window := range source.Windows {
		converted := domain.AllowanceWindow{
			Key:     windowKey(window, index),
			Label:   window.Label,
			Quality: domain.QualityMeasured,
		}
		if window.Group != nil {
			converted.Group = *window.Group
		}
		if window.Left != nil {
			used := 100 - *window.Left
			converted.UsedPercent = &used
		}
		if window.ResetMinutes != nil {
			reset := clock.Add(time.Duration(*window.ResetMinutes) * time.Minute)
			converted.ResetAt = &reset
		}
		snapshot.Windows = append(snapshot.Windows, converted)
	}
	applyExtras(&snapshot, source.Provider)
	snapshot.State = domain.DeriveState(snapshot, false)
	return snapshot
}

func windowKey(window fixtureWindow, index int) string {
	key := strings.ToLower(strings.ReplaceAll(window.Label, " ", "-"))
	if window.Group != nil && *window.Group != "" {
		key = strings.ToLower(strings.ReplaceAll(*window.Group, " ", "-")) + "-" + key
	}
	return fmt.Sprintf("%s-%d", key, index)
}

func applyExtras(snapshot *domain.UsageSnapshot, provider string) {
	switch provider {
	case "codex":
		credits := 250.0
		snapshot.Balances = []domain.Balance{{Label: "Credits", Value: &credits, Unit: "credits", Kind: "credits"}}
	case "antigravity":
		snapshot.Details = []domain.DetailSection{{
			Title: "Gemini models",
			Rows:  []domain.DetailRow{{Label: "Model group", Value: "Gemini · Claude"}},
		}}
	}
}

func parseAge(value string) time.Duration {
	if value == "" {
		return 0
	}
	unit := value[len(value)-1]
	number, err := strconv.Atoi(value[:len(value)-1])
	if err != nil {
		return 0
	}
	switch unit {
	case 's':
		return time.Duration(number) * time.Second
	case 'm':
		return time.Duration(number) * time.Minute
	case 'h':
		return time.Duration(number) * time.Hour
	case 'd':
		return time.Duration(number) * 24 * time.Hour
	default:
		return 0
	}
}

func (f fixtures) historySnapshot(clock time.Time) domain.HistorySnapshot {
	key, err := domain.ParseSourceKey(f.History.Source)
	if err != nil {
		key = domain.SourceKey{Provider: "muse", Variant: domain.VariantConfigured}
	}
	key.Account = domain.UnknownAccount
	key.AccountUnknown = true
	total := f.History.TotalTokens
	result := domain.HistorySnapshot{
		Key:         key,
		Days:        7,
		Start:       clock.AddDate(0, 0, -6),
		End:         clock,
		Timezone:    f.Timezone,
		TotalTokens: &total,
		Currency:    "",
		Priced:      false,
		Provenance:  "Muse local token history",
		Coverage:    "Partial coverage · this machine only",
		Partial:     true,
	}
	for _, day := range f.History.Days {
		var tokens *int64
		if day.Tokens != nil {
			value := *day.Tokens
			tokens = &value
		}
		result.Buckets = append(result.Buckets, domain.HistoryDay{Day: day.Day, Tokens: tokens})
		if tokens == nil {
			result.MissingDays = append(result.MissingDays, day.Day)
		}
	}
	for _, model := range f.History.Models {
		input, output := model.Input, model.Output
		result.Models = append(result.Models, domain.ModelTotal{Name: model.Name, Input: &input, Output: &output})
	}
	return result
}
