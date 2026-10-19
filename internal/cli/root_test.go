package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "quotary version") {
		t.Fatalf("version output = %q", out.String())
	}
}

func TestDemoFlagRegistered(t *testing.T) {
	cmd := NewRootCmd()
	if cmd.Flags().Lookup("demo") == nil {
		t.Fatal("--demo flag is missing")
	}
	if cmd.Flags().Lookup("scenario") == nil {
		t.Fatal("--scenario flag is missing")
	}
}

func TestDemoDepsUseInMemoryFixtureFS(t *testing.T) {
	deps := demoDeps()
	if deps.Store == nil || deps.Fixtures == nil || deps.Runner == nil {
		t.Fatal("demo deps are incomplete")
	}
	if file, err := deps.Fixtures.Open("fixtures.json"); err != nil {
		t.Fatal(err)
	} else if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
