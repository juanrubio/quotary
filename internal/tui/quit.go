package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Quit confirmation, following the sibling apps (gh-boardwalk, trailmark):
// q asks first instead of quitting, Cancel is the default choice, arrows or
// Tab choose, Enter confirms the choice, y quits outright, Esc or n stays,
// and Ctrl+C still quits immediately (handled before this in onKey).

func (m *model) requestQuit() {
	m.quitPrompt = true
	m.quitSelected = false
}

func (m *model) onQuitKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "n":
		m.quitPrompt = false
	case "left", "right", "up", "down", "tab", "shift+tab", "h", "l":
		m.quitSelected = !m.quitSelected
	case "y":
		m.quitPrompt = false
		return m, tea.Quit
	case "enter":
		m.quitPrompt = false
		if m.quitSelected {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *model) quitButton(label string, selected bool, destructive bool) string {
	marker := "  "
	if selected {
		marker = "> "
	}
	style := lipgloss.NewStyle().Padding(0, 1)
	if selected {
		style = style.Bold(true)
	}
	if !m.theme.Mono {
		style = style.Foreground(lipgloss.Color(m.theme.Palette.Text)).
			Background(lipgloss.Color(m.theme.Palette.Surface))
		if selected {
			fill := m.theme.Palette.Accent
			if destructive {
				fill = m.theme.Palette.Error
			}
			style = style.Foreground(lipgloss.Color(m.theme.Palette.Canvas)).
				Background(lipgloss.Color(fill))
		}
	}
	return style.Render("[" + marker + label + "]")
}

func (m *model) quitButtons() string {
	return m.quitButton("Cancel", !m.quitSelected, false) + "   " +
		m.quitButton("Quit", m.quitSelected, true)
}

// renderQuitModal builds the centred confirmation box. Each content line is
// surface-treated so the raised Surface background survives the style resets
// inside it, the same trick card() uses for panels.
func (m *model) renderQuitModal(width int) []string {
	boxWidth := min(46, max(20, width-4))
	content := []string{
		m.theme.Accent.Bold(true).Render("Quit Quotary?"),
		"",
		m.quitButtons(),
		"",
		m.theme.Muted.Render("←→/Tab Choose"),
		m.theme.Muted.Render("Enter Confirm · Esc Cancel"),
	}
	for index := range content {
		content[index] = m.surface(content[index])
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.Palette.Accent)).
		Padding(0, 1).
		Width(boxWidth)
	if !m.theme.Mono {
		style = style.Background(lipgloss.Color(m.theme.Palette.Surface))
	}
	return strings.Split(style.Render(strings.Join(content, "\n")), "\n")
}

// overlayQuitPrompt splices the modal into the centre of the painted frame.
// The underlying lines keep their canvas painting; only the cells the box
// covers are replaced.
func (m *model) overlayQuitPrompt(lines []string, width, height int) []string {
	modal := m.renderQuitModal(width)
	boxWidth := 0
	for _, line := range modal {
		boxWidth = max(boxWidth, ansi.StringWidth(line))
	}
	if boxWidth > width || len(modal) > height {
		return lines
	}
	startX := (width - boxWidth) / 2
	startY := (height - len(modal)) / 2
	for index, modalLine := range modal {
		row := startY + index
		lines[row] = ansi.Cut(lines[row], 0, startX) + modalLine +
			ansi.Cut(lines[row], startX+boxWidth, width)
	}
	return lines
}
