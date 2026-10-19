package tui

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/juanrubio/quotary/internal/backend/demo"
	"github.com/juanrubio/quotary/internal/domain"
)

func TestQuitKeys(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{
		{Code: 'q', Text: "q"},
		{Code: 'c', Mod: tea.ModCtrl},
	} {
		m := &model{}
		_, cmd := m.Update(key)
		if cmd == nil {
			t.Fatalf("%q should return a quit command", key.String())
		}
	}
}

func TestDemoViewIsLabeled(t *testing.T) {
	m := &model{demo: true}
	view := m.View().Content
	if !strings.Contains(view, "DEMO") || !strings.Contains(view, "fictional data") {
		t.Fatalf("demo label missing: %q", view)
	}
}

func TestResize(t *testing.T) {
	m := &model{}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.width != 80 || m.height != 24 {
		t.Fatalf("size = %dx%d", m.width, m.height)
	}
}

func TestTeatestShellPaintsAndQuits(t *testing.T) {
	tm := teatest.NewTestModel(t, &model{demo: true}, teatest.WithInitialTermSize(80, 24))
	t.Cleanup(func() { _ = tm.Quit() })
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte("DEMO"))
	}, teatest.WithCheckInterval(25*time.Millisecond), teatest.WithDuration(5*time.Second))
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
	if _, err := io.ReadAll(tm.FinalOutput(t)); err != nil {
		t.Fatal(err)
	}
}

func TestDemoModelUsesBackendAndSwitchesScenarios(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioSubscription, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	m.width, m.height = 120, 40
	view := m.View().Content
	for _, want := range []string{"DEMO · fictional data", "Scenario: subscription", "Claude Code", "24 sources"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
	m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	view = m.View().Content
	if !strings.Contains(view, "Scenario: attention") || !strings.Contains(view, "Exhausted") {
		t.Fatalf("attention scenario not painted:\n%s", view)
	}
	m.Update(tea.KeyPressMsg{Code: '4', Text: "4"})
	if !strings.Contains(m.View().Content, "No tracked sources") {
		t.Fatalf("empty scenario not painted:\n%s", m.View().Content)
	}
}

func TestDemoRefreshTransitionsVisible(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioSubscription, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if !strings.Contains(m.View().Content, "Queued") {
		t.Fatalf("queued state not visible:\n%s", m.View().Content)
	}
	m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	if !strings.Contains(m.View().Content, "Fetching") {
		t.Fatalf("fetching state not visible:\n%s", m.View().Content)
	}
	m.Update(refreshTickMsg{key: m.refreshKey, generation: m.refreshGen})
	if strings.Contains(m.View().Content, "Queued") || strings.Contains(m.View().Content, "Fetching") {
		t.Fatalf("refresh did not return to current:\n%s", m.View().Content)
	}
}

func TestDemoSetupScenarioShowsBackendMissing(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioSetup, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	m.width, m.height = 100, 32
	view := m.View().Content
	if !strings.Contains(view, "Setup · no backend connection") || !strings.Contains(view, "CodexBar could not be found") {
		t.Fatalf("setup view:\n%s", view)
	}
}

func TestRefreshCompletesOriginalSourceAfterNavigation(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioSubscription, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	m.width, m.height = 120, 40
	original := m.visible[0].Key.String()
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	generation := m.refreshGen
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.Update(refreshTickMsg{key: original, generation: generation})
	m.Update(refreshTickMsg{key: original, generation: generation})
	if stage := runtime.RefreshStage(original); stage != "current" {
		t.Fatalf("original source stranded in %s", stage)
	}
}

func TestScenarioSwitchInvalidatesPendingRefresh(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioSubscription, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	original := m.visible[0].Key.String()
	m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	generation := m.refreshGen
	m.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	m.Update(refreshTickMsg{key: original, generation: generation})
	for _, snapshot := range runtime.Snapshots(context.Background()) {
		if snapshot.State == domain.StatusQueued || snapshot.State == domain.StatusFetching {
			t.Fatalf("stale refresh affected new scenario: %s", snapshot.Key)
		}
	}
}

func TestDemoNoResultsScenarioPreservesQuery(t *testing.T) {
	runtime, err := demo.NewRuntime(demo.ScenarioNoResults, demo.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(Options{Demo: true, Session: runtime})
	view := m.View().Content
	if !strings.Contains(view, "No results for") || !strings.Contains(view, runtime.Query()) {
		t.Fatalf("no-results view:\n%s", view)
	}
}
