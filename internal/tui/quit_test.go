package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/juanrubio/quotary/internal/backend/demo"
)

func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func pressKey(m *model, key tea.KeyPressMsg) tea.Cmd {
	_, cmd := m.Update(key)
	return cmd
}

func TestQuitRequiresConfirmation(t *testing.T) {
	for _, cancel := range []tea.KeyPressMsg{
		{Code: tea.KeyEnter},
		{Code: tea.KeyEscape},
		{Code: 'n', Text: "n"},
	} {
		m := demoModel(t, demo.ScenarioSubscription)
		m.selected = 3
		if cmd := pressKey(m, tea.KeyPressMsg{Code: 'q', Text: "q"}); isQuitCmd(cmd) || !m.quitPrompt || m.quitSelected {
			t.Fatal("q must open the prompt with Cancel selected")
		}
		if cmd := pressKey(m, tea.KeyPressMsg{Code: 'q', Text: "q"}); isQuitCmd(cmd) || !m.quitPrompt {
			t.Fatal("repeated q bypassed the prompt")
		}
		if cmd := pressKey(m, cancel); isQuitCmd(cmd) || m.quitPrompt {
			t.Fatalf("%q did not cancel the prompt", cancel.String())
		}
		if m.selected != 3 {
			t.Fatal("cancelling the prompt changed overview state")
		}
	}
	for _, confirm := range [][]tea.KeyPressMsg{
		{{Code: 'y', Text: "y"}},
		{{Code: tea.KeyRight}, {Code: tea.KeyEnter}},
		{{Code: tea.KeyTab}, {Code: tea.KeyEnter}},
		{{Code: tea.KeyLeft}, {Code: tea.KeyEnter}},
	} {
		m := demoModel(t, demo.ScenarioSubscription)
		pressKey(m, tea.KeyPressMsg{Code: 'q', Text: "q"})
		var cmd tea.Cmd
		for _, key := range confirm {
			cmd = pressKey(m, key)
		}
		if !isQuitCmd(cmd) || m.quitPrompt {
			t.Fatalf("%v did not confirm quit", confirm)
		}
	}
}

func TestQuitPromptCtrlCStillQuits(t *testing.T) {
	m := demoModel(t, demo.ScenarioSubscription)
	pressKey(m, tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd := pressKey(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); !isQuitCmd(cmd) {
		t.Fatal("ctrl+c must quit even while the prompt is open")
	}
}

func TestQuitPromptRendering(t *testing.T) {
	for _, size := range [][2]int{{40, 12}, {80, 24}, {120, 40}} {
		m := demoModel(t, demo.ScenarioSubscription)
		pressKey(m, tea.KeyPressMsg{Code: 'q', Text: "q"})
		view := checkFrame(t, m, size[0], size[1])
		text := cleanText(view)
		for _, want := range []string{"Quit Quotary?", "Cancel", "Quit", "> Cancel", "Tab Choose", "Enter Confirm", "Esc Cancel"} {
			if !strings.Contains(text, want) {
				t.Fatalf("quit prompt at %v missing %q:\n%s", size, want, text)
			}
		}
		pressKey(m, tea.KeyPressMsg{Code: tea.KeyRight})
		if view := checkFrame(t, m, size[0], size[1]); !strings.Contains(cleanText(view), "> Quit") {
			t.Fatalf("quit prompt at %v did not move focus to Quit:\n%s", size, cleanText(view))
		}
	}
}
