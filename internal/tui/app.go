// Package tui implements Quotary's terminal interface.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/juanrubio/quotary/internal/domain"
)

// DemoSession is the stateful demo seam. The TUI never imports the demo backend
// directly; the CLI wires a runtime that satisfies this interface.
type DemoSession interface {
	ScenarioName() string
	Clock() string
	SetScenarioName(string) error
	Snapshots(context.Context) []domain.UsageSnapshot
	BackendError() string
	Query() string
	StartRefresh(string)
	AdvanceRefresh(string) bool
}

// Options configures a TUI session.
type Options struct {
	Demo    bool
	Session DemoSession
}

type refreshTickMsg struct {
	key        string
	generation uint64
}

type model struct {
	width, height int
	demo          bool
	session       DemoSession
	snapshots     []domain.UsageSnapshot
	selected      int
	status        string
	refreshing    bool
	refreshKey    string
	refreshGen    uint64
	scenarioGen   uint64
}

// Run starts the interactive application.
func Run(ctx context.Context, opts Options) error {
	m := newModel(opts)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

func newModel(opts Options) *model {
	m := &model{demo: opts.Demo, session: opts.Session}
	if m.session != nil {
		m.reload()
	}
	return m
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) reload() {
	if m.session == nil {
		return
	}
	m.snapshots = m.session.Snapshots(context.Background())
	if m.selected >= len(m.snapshots) {
		m.selected = len(m.snapshots) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case refreshTickMsg:
		if m.session == nil || msg.key != m.refreshKey || msg.generation != m.refreshGen {
			return m, nil
		}
		done := m.session.AdvanceRefresh(msg.key)
		m.reload()
		if !done {
			m.refreshing = false
			m.status = ""
			m.refreshKey = ""
			return m, nil
		}
		return m, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
			return refreshTickMsg{key: msg.key, generation: msg.generation}
		})
	case tea.KeyPressMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m *model) onKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.selected+1 < len(m.snapshots) {
			m.selected++
		}
	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}
	case "r":
		if m.session != nil && len(m.snapshots) > 0 && !m.refreshing {
			key := m.snapshots[m.selected].Key.String()
			m.refreshGen++
			m.refreshKey = key
			m.session.StartRefresh(key)
			m.refreshing = true
			m.status = "Queueing refresh…"
			m.reload()
			generation := m.refreshGen
			return m, tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
				return refreshTickMsg{key: key, generation: generation}
			})
		}
	case "1", "2", "3", "4", "5", "6":
		scenario := map[string]string{
			"1": "subscription",
			"2": "attention",
			"3": "catalog",
			"4": "empty",
			"5": "setup",
			"6": "noresults",
		}[msg.String()]
		if m.session != nil {
			if err := m.session.SetScenarioName(scenario); err != nil {
				m.status = err.Error()
				return m, nil
			}
			m.selected = 0
			m.refreshing = false
			m.refreshKey = ""
			m.scenarioGen++
			m.status = "Scenario: " + scenario
			m.reload()
		}
	}
	return m, nil
}

func (m *model) View() tea.View {
	width := m.width
	if width <= 0 {
		width = 80
	}
	var lines []string
	lines = append(lines, "  QUOTARY")
	if m.demo {
		lines = append(lines, "  DEMO · fictional data · "+m.clock())
	}
	if m.session != nil {
		counts := domain.CountStates(m.snapshots)
		lines = append(lines, fmt.Sprintf("  Scenario: %s · %d sources · %d current · %d attention",
			m.session.ScenarioName(), counts.Total, counts.Current, counts.Attention))
	} else {
		lines = append(lines, "  Starting…")
	}
	lines = append(lines, strings.Repeat("─", max(1, width-4)))
	if len(m.snapshots) == 0 {
		if m.session != nil && m.session.BackendError() != "" {
			lines = append(lines, "", "  Setup · no backend connection", "  "+m.session.BackendError(), "", "  Press 1–6 to switch deterministic demo scenarios.")
		} else if m.session != nil && m.session.Query() != "" {
			lines = append(lines, "", fmt.Sprintf("  No results for %q.", m.session.Query()), "  The tracked collection is preserved.", "", "  Press 1–6 to switch deterministic demo scenarios.")
		} else {
			lines = append(lines, "", "  No tracked sources.", "  Press 1–6 to switch deterministic demo scenarios.")
		}
	} else {
		lines = append(lines, "    Source                         State       Left   Reset")
		visible := m.height - 9
		if visible < 1 {
			visible = 1
		}
		start := 0
		if m.selected >= visible {
			start = m.selected - visible + 1
		}
		end := start + visible
		if end > len(m.snapshots) {
			end = len(m.snapshots)
		}
		for index := start; index < end; index++ {
			snapshot := m.snapshots[index]
			prefix := "  "
			if index == m.selected {
				prefix = "> "
			}
			left, reset := "—", "—"
			if len(snapshot.Windows) > 0 {
				if remaining := snapshot.Windows[0].RemainingPercent(); remaining != nil {
					left = fmt.Sprintf("%.0f%%", *remaining)
				}
				reset = domain.ResetLabel(snapshot.Windows[0].ResetAt, m.now())
			}
			lines = append(lines, fmt.Sprintf("%s%-30s %-11s %-6s %s",
				prefix, truncate(snapshot.DisplayName, 30), snapshot.State, left, reset))
		}
	}
	if m.status != "" {
		lines = append(lines, "  "+m.status)
	}
	lines = append(lines, strings.Repeat("─", max(1, width-4)))
	lines = append(lines, "  1–6 Scenarios   ↑↓ Move   r Refresh   q Quit")
	if m.height > 0 && len(lines) > m.height {
		lines = lines[:m.height]
	}
	view := tea.NewView(strings.Join(lines, "\n"))
	view.AltScreen = true
	return view
}

func (m *model) now() time.Time {
	if m.session != nil {
		if parsed, err := time.Parse(time.RFC3339, m.session.Clock()); err == nil {
			return parsed
		}
	}
	return time.Now().UTC()
}

func (m *model) clock() string {
	if m.session != nil {
		return m.session.Clock()
	}
	return ""
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
