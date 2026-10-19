package tui

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
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
