package config

import "testing"

func TestDefaults(t *testing.T) {
	prefs := Defaults()
	if prefs.Theme != ThemeGreen || prefs.IntervalSeconds != 300 || !prefs.Privacy {
		t.Fatalf("defaults = %+v", prefs)
	}
}

func TestNormalizeTheme(t *testing.T) {
	if NormalizeTheme("monokai") != ThemeMonokai {
		t.Fatal("monokai should persist")
	}
	if NormalizeTheme("") != ThemeGreen || NormalizeTheme("neon") != ThemeGreen {
		t.Fatal("unknown themes should normalize to green")
	}
}

func TestMemoryStoreRoundTrip(t *testing.T) {
	store := NewMemoryStore(Defaults())
	prefs := store.Load()
	prefs.Theme = ThemeBlue
	prefs.Tracked = []string{"codex:configured"}
	prefs.Pins = []string{"codex:configured"}
	prefs.IntervalSeconds = 120
	if err := store.Save(prefs); err != nil {
		t.Fatal(err)
	}
	loaded := store.Load()
	if loaded.Theme != ThemeBlue || loaded.IntervalSeconds != 120 || len(loaded.Tracked) != 1 {
		t.Fatalf("loaded = %+v", loaded)
	}
	loaded.Tracked[0] = "mutated"
	if store.Load().Tracked[0] != "codex:configured" {
		t.Fatal("store must not expose its slice")
	}
}

func TestMemoryStoreRejectsShortInterval(t *testing.T) {
	store := NewMemoryStore(Defaults())
	prefs := store.Load()
	prefs.IntervalSeconds = 30
	if err := store.Save(prefs); err == nil {
		t.Fatal("30 seconds should be rejected")
	}
}

func TestNormalizePreservesValidFields(t *testing.T) {
	prefs := Normalize(Preferences{Theme: ThemeAmber, Appearance: AppearanceLight, Layout: LayoutCompact, IntervalSeconds: 600})
	if prefs.Theme != ThemeAmber || prefs.Appearance != AppearanceLight || prefs.Layout != LayoutCompact || prefs.IntervalSeconds != 600 {
		t.Fatalf("normalized = %+v", prefs)
	}
}

func TestMemoryStoreOwnsSlices(t *testing.T) {
	prefs := Defaults()
	prefs.Tracked = []string{"codex:configured"}
	store := NewMemoryStore(prefs)
	prefs.Tracked[0] = "mutated"
	if store.Load().Tracked[0] != "codex:configured" {
		t.Fatal("constructor retained caller slice")
	}
	saved := store.Load()
	saved.Pins = []string{"codex:configured"}
	if err := store.Save(saved); err != nil {
		t.Fatal(err)
	}
	saved.Pins[0] = "mutated"
	if store.Load().Pins[0] != "codex:configured" {
		t.Fatal("save retained caller slice")
	}
}

func TestRejectedSavePreservesPreviousState(t *testing.T) {
	store := NewMemoryStore(Defaults())
	good := store.Load()
	good.Theme = ThemeAmber
	good.IntervalSeconds = 120
	if err := store.Save(good); err != nil {
		t.Fatal(err)
	}
	bad := store.Load()
	bad.Theme = ThemeMonokai
	bad.IntervalSeconds = 10
	if err := store.Save(bad); err == nil {
		t.Fatal("expected rejection")
	}
	after := store.Load()
	if after.Theme != ThemeAmber || after.IntervalSeconds != 120 {
		t.Fatalf("rejected save mutated state: %+v", after)
	}
}
