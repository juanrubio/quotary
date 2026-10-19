// Package config owns Quotary preferences and their persistence.
package config

import (
	"errors"
	"sync"
)

// Appearance is auto, dark, or light.
type Appearance string

const (
	AppearanceAuto  Appearance = "auto"
	AppearanceDark  Appearance = "dark"
	AppearanceLight Appearance = "light"
)

// Layout is the Overview presentation.
type Layout string

const (
	LayoutAuto    Layout = "auto"
	LayoutCards   Layout = "cards"
	LayoutCompact Layout = "compact"
)

// ThemeName is one of the supported named palettes.
type ThemeName string

const (
	ThemeGreen   ThemeName = "green"
	ThemeBlue    ThemeName = "blue"
	ThemeAmber   ThemeName = "amber"
	ThemeMonokai ThemeName = "monokai"
)

// Preferences is the versioned user configuration.
type Preferences struct {
	Version           int        `json:"version"`
	Theme             ThemeName  `json:"theme"`
	Appearance        Appearance `json:"appearance"`
	Layout            Layout     `json:"layout"`
	Tracked           []string   `json:"tracked"`
	Pins              []string   `json:"pins"`
	IntervalSeconds   int        `json:"intervalSeconds"`
	Privacy           bool       `json:"privacy"`
	Monochrome        bool       `json:"monochrome"`
	BackendPath       string     `json:"backendPath,omitempty"`
	BackendConfigPath string     `json:"backendConfigPath,omitempty"`
}

// Defaults returns the documented fallback preferences.
func Defaults() Preferences {
	return Preferences{
		Version:         1,
		Theme:           ThemeGreen,
		Appearance:      AppearanceAuto,
		Layout:          LayoutAuto,
		IntervalSeconds: 300,
		Privacy:         true,
	}
}

// NormalizeTheme keeps absent or unsupported names on Green.
func NormalizeTheme(name string) ThemeName {
	switch ThemeName(name) {
	case ThemeBlue, ThemeAmber, ThemeMonokai:
		return ThemeName(name)
	default:
		return ThemeGreen
	}
}

// Normalize fixes invalid values without discarding valid fields.
func Normalize(prefs Preferences) Preferences {
	if prefs.Version == 0 {
		prefs.Version = 1
	}
	prefs.Theme = NormalizeTheme(string(prefs.Theme))
	if prefs.Appearance != AppearanceDark && prefs.Appearance != AppearanceLight {
		prefs.Appearance = AppearanceAuto
	}
	if prefs.Layout != LayoutCompact && prefs.Layout != LayoutCards {
		prefs.Layout = LayoutAuto
	}
	if prefs.IntervalSeconds < 60 {
		prefs.IntervalSeconds = 300
	}
	return prefs
}

// Store is the preference seam used by demo, TUI, scheduler, and live config.
type Store interface {
	Load() Preferences
	Save(Preferences) error
}

// MemoryStore keeps preferences in memory only. Demo mode must use this.
type MemoryStore struct {
	mu    sync.Mutex
	prefs Preferences
}

// NewMemoryStore returns a store seeded with prefs.
func NewMemoryStore(prefs Preferences) *MemoryStore {
	return &MemoryStore{prefs: clone(Normalize(prefs))}
}

// Load returns the current preferences.
func (s *MemoryStore) Load() Preferences {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.prefs)
}

// Save validates and replaces preferences.
func (s *MemoryStore) Save(prefs Preferences) error {
	if prefs.IntervalSeconds < 60 {
		return errors.New("refresh interval must be at least 60 seconds")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prefs = clone(Normalize(prefs))
	return nil
}

func clone(prefs Preferences) Preferences {
	prefs.Tracked = append([]string(nil), prefs.Tracked...)
	prefs.Pins = append([]string(nil), prefs.Pins...)
	return prefs
}
