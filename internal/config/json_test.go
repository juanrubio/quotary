package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quotary", "config.json")
	store, err := NewJSONStore(path)
	if err != nil {
		t.Fatal(err)
	}
	prefs := Defaults()
	prefs.Theme = ThemeMonokai
	prefs.Appearance = AppearanceLight
	prefs.Layout = LayoutCompact
	prefs.Tracked = []string{"codex:configured@unknown", "gemini:web@unknown"}
	prefs.Pins = []string{"codex:configured@unknown"}
	prefs.IntervalSeconds = 120
	prefs.Privacy = false
	prefs.Monochrome = true
	prefs.BackendPath = "/opt/codexbar"
	prefs.BackendConfigPath = "/tmp/codexbar.json"
	if err := store.Save(prefs); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewJSONStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.Load()
	if got.Theme != ThemeMonokai || got.Layout != LayoutCompact || got.IntervalSeconds != 120 || len(got.Tracked) != 2 || len(got.Pins) != 1 || got.Privacy || !got.Monochrome || got.BackendPath != "/opt/codexbar" || got.BackendConfigPath != "/tmp/codexbar.json" {
		t.Fatalf("round trip=%+v", got)
	}
}

func TestJSONStoreCorruptBacksUpAndRestoresDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := NewJSONStore(path)
	prefs := store.Load()
	if prefs.Theme != ThemeGreen || store.Warning() == "" {
		t.Fatalf("prefs=%+v warning=%q", prefs, store.Warning())
	}
	matches, _ := filepath.Glob(path + ".corrupt-*.bak")
	if len(matches) != 1 {
		t.Fatalf("backup files=%v", matches)
	}
	var disk Preferences
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &disk) != nil || disk.Version != CurrentVersion {
		t.Fatal("defaults were not written atomically")
	}
}

func TestJSONStoreOlderVersionMigrates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	prefs := Defaults()
	prefs.Version = 0
	prefs.Theme = ThemeBlue
	data, _ := json.Marshal(prefs)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := NewJSONStore(path)
	got := store.Load()
	if got.Version != CurrentVersion || got.Theme != ThemeBlue || store.Warning() == "" {
		t.Fatalf("migrated=%+v warning=%q", got, store.Warning())
	}
	var disk Preferences
	read, _ := os.ReadFile(path)
	if json.Unmarshal(read, &disk) != nil || disk.Version != CurrentVersion {
		t.Fatal("migration not persisted")
	}
}

func TestJSONStoreNewerVersionRefusesWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	prefs := Defaults()
	prefs.Version = CurrentVersion + 1
	prefs.Theme = ThemeAmber
	data, _ := json.Marshal(prefs)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := NewJSONStore(path)
	got := store.Load()
	if got.Theme != ThemeGreen || store.Warning() == "" {
		t.Fatalf("newer schema prefs=%+v", got)
	}
	if err := store.Save(Defaults()); err == nil {
		t.Fatal("newer schema accepted writes")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(data) {
		t.Fatal("newer schema file was modified")
	}
}

func TestJSONStoreUnknownThemeNormalizesGreen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"theme":"unknown","intervalSeconds":300}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := NewJSONStore(path)
	if got := store.Load(); got.Theme != ThemeGreen {
		t.Fatalf("theme=%s", got.Theme)
	}
}

func TestDefaultJSONStorePathHonorsEnv(t *testing.T) {
	t.Setenv("QUOTARY_CONFIG_PATH", "/tmp/custom-quotary.json")
	path, err := DefaultJSONStorePath()
	if err != nil || path != "/tmp/custom-quotary.json" {
		t.Fatalf("path=%q err=%v", path, err)
	}
}

func TestJSONStoreSaveBeforeLoadRefusesNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	prefs := Defaults()
	prefs.Version = CurrentVersion + 1
	data, _ := json.Marshal(prefs)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := NewJSONStore(path)
	if err := store.Save(Defaults()); err == nil {
		t.Fatal("Save before Load overwrote newer schema")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(data) {
		t.Fatal("newer schema file changed")
	}
}

func TestDefaultJSONStorePathUsesXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("QUOTARY_CONFIG_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", dir)
	path, err := DefaultJSONStorePath()
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, "quotary", "config.json") {
		t.Fatalf("path=%q", path)
	}
}

func TestJSONStoreNewerVersionWithIncompatibleFieldTypesRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := []byte(`{"version":2,"tracked":"not-an-array","theme":"blue"}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := NewJSONStore(path)
	got := store.Load()
	if got.Version != CurrentVersion || store.Warning() == "" {
		t.Fatalf("prefs=%+v warning=%q", got, store.Warning())
	}
	if err := store.Save(Defaults()); err == nil {
		t.Fatal("newer incompatible schema accepted Save")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(original) {
		t.Fatal("newer incompatible file was modified")
	}
}
