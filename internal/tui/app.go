// Package tui implements Quotary's terminal interface.
package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// Options configures a TUI session.
type Options struct {
	Demo bool
}

type model struct {
	width, height int
	demo          bool
}

// Run starts the interactive application.
func Run(ctx context.Context, opts Options) error {
	m := &model{demo: opts.Demo}
	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *model) View() tea.View {
	label := "QUOTARY"
	if m.demo {
		label = "QUOTARY · DEMO · fictional data"
	}
	view := tea.NewView(fmt.Sprintf("\n  %s\n\n  Starting…\n\n  q Quit\n", label))
	view.AltScreen = true
	return view
}
