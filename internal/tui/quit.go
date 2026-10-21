package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Confirmation modals, following the sibling apps (gh-boardwalk,
// trailmark): a destructive or discarding action asks first, the safe
// choice is selected by default, arrows or Tab choose, Enter confirms the
// choice, y takes the destructive action outright, Esc or n cancels, and
// Ctrl+C still quits immediately (handled before this in onKey).

type confirmationKind int

const (
	confirmationNone confirmationKind = iota
	confirmationQuit
	confirmationDiscardSettings
)

type confirmationSpec struct {
	title       string
	body        string
	safe        string
	destructive string
}

func (m *model) confirmationSpec() confirmationSpec {
	if m.confirmation == confirmationDiscardSettings {
		return confirmationSpec{
			title:       "Discard settings changes?",
			body:        "Unsaved settings will be lost.",
			safe:        "Keep editing",
			destructive: "Discard",
		}
	}
	return confirmationSpec{
		title:       "Quit Quotary?",
		safe:        "Cancel",
		destructive: "Quit",
	}
}

func (m *model) requestQuit() {
	m.confirmation = confirmationQuit
	m.confirmationSelected = false
}

func (m *model) requestDiscardSettings() {
	m.confirmation = confirmationDiscardSettings
	m.confirmationSelected = false
}

func (m *model) onConfirmationKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "n":
		m.confirmation = confirmationNone
	case "left", "right", "up", "down", "tab", "shift+tab", "h", "l":
		m.confirmationSelected = !m.confirmationSelected
	case "y":
		return m.confirmConfirmation()
	case "enter":
		if m.confirmationSelected {
			return m.confirmConfirmation()
		}
		m.confirmation = confirmationNone
	}
	return m, nil
}

func (m *model) confirmConfirmation() (tea.Model, tea.Cmd) {
	kind := m.confirmation
	m.confirmation = confirmationNone
	switch kind {
	case confirmationQuit:
		return m, tea.Quit
	case confirmationDiscardSettings:
		m.discardSettings()
	}
	return m, nil
}

func (m *model) confirmationButton(label string, selected bool, destructive bool) string {
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

func (m *model) confirmationButtons() string {
	spec := m.confirmationSpec()
	return m.confirmationButton(spec.safe, !m.confirmationSelected, false) + "   " +
		m.confirmationButton(spec.destructive, m.confirmationSelected, true)
}

// renderConfirmationModal builds the centred confirmation box. Each content
// line is surface-treated so the raised Surface background survives the
// style resets inside it, the same trick card() uses for panels.
func (m *model) renderConfirmationModal(width int) []string {
	spec := m.confirmationSpec()
	boxWidth := min(46, max(20, width-4))
	content := []string{m.theme.Accent.Bold(true).Render(spec.title)}
	if spec.body != "" {
		content = append(content, m.theme.Text.Render(spec.body))
	}
	content = append(content,
		"",
		m.confirmationButtons(),
		"",
		m.theme.Muted.Render("←→/Tab Choose"),
		m.theme.Muted.Render("Enter Confirm · Esc Cancel"),
	)
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

// overlayConfirmationPrompt splices the modal into the centre of the
// painted frame. The underlying lines keep their canvas painting; only the
// cells the box covers are replaced.
func (m *model) overlayConfirmationPrompt(lines []string, width, height int) []string {
	modal := m.renderConfirmationModal(width)
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
