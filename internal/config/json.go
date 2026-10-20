package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CurrentVersion is the on-disk schema version.
const CurrentVersion = 1

// JSONStore persists versioned preferences atomically at the XDG config path.
type JSONStore struct {
	mu       sync.Mutex
	path     string
	loaded   bool
	firstRun bool
	prefs    Preferences
	warning  string
	readOnly bool
}

// DefaultJSONStorePath resolves the XDG config path with a HOME fallback.
func DefaultJSONStorePath() (string, error) {
	if explicit := os.Getenv("QUOTARY_CONFIG_PATH"); explicit != "" {
		return explicit, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", fmt.Errorf("resolve user config directory: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "quotary", "config.json"), nil
}

// NewJSONStore resolves the path; an empty path uses the default location.
func NewJSONStore(path string) (*JSONStore, error) {
	if path == "" {
		resolved, err := DefaultJSONStorePath()
		if err != nil {
			return nil, err
		}
		path = resolved
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return &JSONStore{path: absolute}, nil
}

// Path reports the resolved configuration file path.
func (s *JSONStore) Path() string { return s.path }

// Warning reports a migration, corruption, or newer-version setup message.
func (s *JSONStore) Warning() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.warning
}

// Load reads the file once and applies the migration policy.
func (s *JSONStore) Load() Preferences {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// FirstRun reports whether the config file was absent at load time.
func (s *JSONStore) FirstRun() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		s.loadLocked()
	}
	return s.firstRun
}

func (s *JSONStore) loadLocked() Preferences {
	if s.loaded {
		return clone(s.prefs)
	}
	s.loaded = true
	s.firstRun = true
	s.prefs = defaultsForLoad()
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return clone(s.prefs)
	}
	s.firstRun = false
	if err != nil {
		s.warning = "Could not read Quotary preferences; using defaults."
		return clone(s.prefs)
	}
	var probe struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &probe); err == nil && probe.Version > CurrentVersion {
		s.readOnly = true
		s.warning = fmt.Sprintf("Quotary preferences use newer schema version %d; running on defaults without writing.", probe.Version)
		return clone(s.prefs)
	}
	var prefs Preferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		backup := s.path + ".corrupt-" + time.Now().UTC().Format("20060102T150405Z") + ".bak"
		renameErr := os.Rename(s.path, backup)
		writeErr := s.writeLocked(s.prefs)
		if renameErr != nil || writeErr != nil {
			s.warning = "Quotary preferences were corrupt; backup or default rewrite failed."
		} else {
			s.warning = "Quotary preferences were corrupt; defaults were restored."
		}
		return clone(s.prefs)
	}
	if prefs.Version > CurrentVersion {
		s.readOnly = true
		s.warning = fmt.Sprintf("Quotary preferences use newer schema version %d; running on defaults without writing.", prefs.Version)
		return clone(s.prefs)
	}
	if prefs.Version < CurrentVersion {
		prefs.Version = CurrentVersion
		prefs = Normalize(prefs)
		s.prefs = prefs
		if err := s.writeLocked(prefs); err != nil {
			s.warning = "Quotary preferences could not be migrated; running on defaults."
			s.prefs = defaultsForLoad()
		} else {
			s.warning = "Quotary preferences were migrated to the current schema."
		}
		return clone(s.prefs)
	}
	s.prefs = Normalize(prefs)
	return clone(s.prefs)
}

// Save validates and atomically replaces the preference file.
func (s *JSONStore) Save(prefs Preferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		s.loadLocked()
	}
	if s.readOnly {
		return errors.New("preferences use a newer schema; refusing to overwrite")
	}
	prefs = Normalize(prefs)
	prefs.Version = CurrentVersion
	if err := s.writeLocked(prefs); err != nil {
		return err
	}
	s.prefs = clone(prefs)
	s.loaded = true
	s.firstRun = false
	return nil
}

func defaultsForLoad() Preferences {
	prefs := Defaults()
	prefs.Version = CurrentVersion
	return prefs
}

func (s *JSONStore) writeLocked(prefs Preferences) error {
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer func() { _ = os.Remove(name) }()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.path)
}

var _ Store = (*JSONStore)(nil)
