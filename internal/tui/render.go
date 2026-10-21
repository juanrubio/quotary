package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/domain"
	"github.com/juanrubio/quotary/internal/theme"
)

func (m *model) render(width, height int) string {
	if width < 20 || height < 8 {
		lines := []string{"", "  Resize to continue · q quit"}
		for len(lines) < height {
			lines = append(lines, "")
		}
		for index := range lines {
			lines[index] = padRight(truncateWidth(lines[index], width), width)
		}
		return strings.Join(lines[:height], "\n")
	}
	lines := m.shellHeader(width)
	footer := m.renderFooter(width)
	contentHeight := height - len(lines) - len(footer)
	if contentHeight < 1 {
		contentHeight = 1
	}
	var content []string
	switch m.screen {
	case screenDetails:
		content = m.renderDetails(width, contentHeight)
	case screenHistory:
		content = m.renderHistory(width, contentHeight)
	case screenPicker:
		content = m.renderPicker(width, contentHeight)
	case screenSettings:
		content = m.renderSettings(width, contentHeight)
	case screenHelp:
		content = m.renderHelp(width, contentHeight)
	default:
		content = m.renderOverview(width, contentHeight)
	}
	for len(content) < contentHeight {
		content = append(content, "")
	}
	if len(content) > contentHeight {
		content = content[:contentHeight]
	}
	lines = append(lines, content...)
	lines = append(lines, footer...)
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for index := range lines {
		lines[index] = m.paintLine(lines[index], width)
	}
	return strings.Join(lines, "\n")
}

// paintLine pads line to width and ensures every cell carries the canvas
// style — Text foreground on the Canvas background. Lipgloss fragments reset
// styling with ESC[m between fragments, and that reset also returns the
// terminal to its *default* foreground and background. On terminals whose
// default background is not the canvas color — kitty defaults to black — the
// cells after each reset (separators, trailing padding, and line-ends the
// renderer skips) paint as dark bands beside the content, and unstyled text
// after a reset would take the terminal's default foreground instead of the
// palette Text colour. Re-asserting only the background regressed the
// foreground that Canvas.Render used to apply, so both are re-asserted.
// See docs/evidence/kitty-rep-compatibility.md.
//
// This mirrors surface(), which keeps panel backgrounds alive the same way.
func (m *model) paintLine(line string, width int) string {
	line = padRight(line, width)
	if m.theme.Mono {
		return line
	}
	sequence := theme.ForegroundSequence(m.theme.Palette.Text) +
		theme.BackgroundSequence(m.theme.Palette.Canvas)
	return sequence + strings.ReplaceAll(line, "\x1b[m", "\x1b[m"+sequence)
}

// backendLabel returns the version to show in the header. Some backends
// report a version line that already carries the program name ("CodexBar
// 0.78.2"), so prepending "CodexBar " again would duplicate the brand.
func (m *model) backendLabel(version string) string {
	if strings.Contains(strings.ToLower(version), "codexbar") {
		return version
	}
	return "CodexBar " + version
}

func (m *model) shellHeader(width int) []string {
	brand := "  " + m.theme.Accent.Bold(true).Render("QUOTARY")
	tagline := ""
	if width >= 120 {
		tagline = "    " + m.theme.Muted.Render("Your AI subscriptions, at a glance")
	}
	demo := ""
	if m.demo {
		demo = "DEMO · fictional data  "
	}
	first := padRight(brand+tagline, width-ansiWidth(demo)) + m.theme.Muted.Render(demo)
	counts := domain.CountStates(m.all)
	second := "  " + m.theme.Muted.Render(fmt.Sprintf("%d sources · %d current · %d attention", counts.Total, counts.Current, counts.Attention))
	if m.searching || m.query != "" {
		second = "  " + m.theme.Accent.Render("/ "+m.query+"▌") + "    " + m.theme.Muted.Render(fmt.Sprintf("%d sources · %d attention", counts.Total, counts.Attention))
	} else if m.session != nil {
		second += "    " + m.theme.Muted.Render(fmt.Sprintf("Scenario: %s · %s", m.scenarioName(), m.clock()))
		if m.backendVersion != "" {
			version := truncateWidth(cleanText(m.backendVersion), max(8, width/3))
			second += "    " + m.theme.Muted.Render(m.backendLabel(version))
		}
	}
	if m.session == nil || m.screen == screenHelp {
		second = "  " + m.theme.Muted.Render("Terminal interface")
	}
	if m.errorText != "" {
		second = "  " + m.theme.Error.Render(cleanText(m.errorText))
	}
	return []string{first, second, m.rule(width), m.navLine(width)}
}

func (m *model) scenarioName() string {
	if m.session == nil {
		return "setup"
	}
	return m.session.ScenarioName()
}

func (m *model) navLine(width int) string {
	if width < 60 {
		return "  " + m.theme.Accent.Render("Overview")
	}
	tabs := []string{"Overview", "Details", "History"}
	active := 0
	switch m.screen {
	case screenDetails:
		active = 1
	case screenHistory:
		active = 2
	}
	var parts []string
	for index, tab := range tabs {
		if index == active {
			parts = append(parts, m.theme.Accent.Bold(true).Render("["+tab+"]"))
		} else {
			parts = append(parts, m.theme.Muted.Render(tab))
		}
	}
	left := "  " + strings.Join(parts, "   ")
	filterText := filterLabel(m.filter)
	if width < 88 {
		filterText = map[string]string{"all": "[All]", "attention": "[Attention]", "pinned": "[Pinned]"}[m.filter]
	}
	left += "   " + m.theme.Muted.Render(filterText)
	if !m.searching && m.query == "" {
		if width >= 120 {
			left += "   " + m.theme.Muted.Render("Sort: "+sortDescription(m.sortMode))
		} else if width >= 80 {
			left += "   " + m.theme.Muted.Render(sortShort(m.sortMode))
		}
	}
	if m.searching || m.query != "" {
		left += "   " + m.theme.Accent.Render("/ "+m.query+"▌")
	}
	right := ""
	if width >= 88 && !m.searching && m.query == "" && m.sortMode == 0 {
		right = m.theme.Muted.Render(" / Search   f Filter   p Sources") + "  "
	}
	return padRight(left, max(0, width-ansiWidth(right))) + right
}

func (m *model) renderFooter(width int) []string {
	hints := "  ↑↓ Move  Enter Details  l Cards/List  r Refresh  / Search  p Sources  ? Help  q Quit"
	if width < 120 {
		// The full hint set no longer fits beside the position indicator;
		// keep the layout and help hints, which are the ones users look for.
		hints = "  ↑↓ Move  Enter Details  l Layout  ? Help  q Quit"
	}
	switch m.screen {
	case screenPicker:
		if width < 100 {
			hints = "  ↑↓ Space a / Esc ? q"
		} else {
			hints = "  ↑↓ Move  Space Track  a Track subscriptions  / Search  Esc Back  ? Help  q Quit"
		}
	case screenSettings:
		if width < 100 {
			hints = "  Tab Field  ←→ Adjust  E Edit  Enter Save  ? q"
		} else {
			hints = "  ↑↓ Theme  Tab Field  ←→ Adjust  E Edit  Enter Save  Esc Cancel  q Quit"
		}
	case screenHelp:
		hints = "  Esc Return  ↑↓ Scroll  q Quit"
	case screenHistory:
		hints = "  Esc Overview  Tab Details  r Refresh history  ? Help  q Quit"
	case screenDetails:
		hints = "  Esc Overview  Tab Allowances/History  r Refresh  ? Help  q Quit"
	}
	position := ""
	if m.screen == screenPicker {
		entries := m.pickerEntries()
		if len(entries) > 0 && m.pickerIndex < len(entries) {
			position = fmt.Sprintf("  %d–%d / %d", m.pickerIndex+1, min(m.pickerIndex+1, len(entries)), len(entries))
		}
	} else if len(m.visible) > 0 {
		position = fmt.Sprintf("  %d–%d / %d", pageStart(m)+1, min(pageEnd(m), len(m.visible)), len(m.visible))
	}
	if m.refreshing && len(m.visible) > 0 {
		position = "  Refreshing…" + position
	}
	if m.status != "" {
		position = "  " + cleanText(m.status)
		if width < 100 {
			position = "  " + truncateWidth(cleanText(m.status), max(6, width-ansiWidth(hints)-2))
		}
	}
	if width < 80 {
		switch m.screen {
		case screenSettings:
			hints = "  Tab Field  ←→  E Edit  Enter Save  ? q"
		case screenPicker:
			hints = "  ↑↓  Space  a Track all  ? q"
		default:
			hints = "  ↑↓  Enter  l  r  p  ?  q"
		}
	}
	return []string{m.rule(width), padRight(hints, max(0, width-ansiWidth(position))) + m.theme.Muted.Render(position)}
}

func (m *model) rule(width int) string {
	if width < 6 {
		return ""
	}
	char := "─"
	if m.theme.Mono {
		char = "-"
	}
	return "  " + m.theme.Border.Render(strings.Repeat(char, width-4)) + "  "
}

func sortShort(mode int) string {
	switch mode {
	case 1:
		return "Name"
	case 2:
		return "Attention"
	case 3:
		return "Reset↑"
	default:
		return "Pinned"
	}
}

func sortDescription(mode int) string {
	switch mode {
	case 1:
		return "Name"
	case 2:
		return "Attention first"
	case 3:
		return "Soonest reset · earliest reported window"
	default:
		return "Pinned first · name"
	}
}

func filterLabel(filter string) string {
	switch filter {
	case "attention":
		return "[ Attention ]"
	case "pinned":
		return "[ Pinned ]"
	default:
		return "[ All tracked ]"
	}
}

func (m *model) renderOverview(width, height int) []string {
	if len(m.visible) == 0 {
		return m.renderEmpty(width, height)
	}
	compact := m.prefs.Layout == config.LayoutCompact || (m.prefs.Layout != config.LayoutCards && (width < 88 || m.height < 32))
	if compact {
		return m.renderCompact(width, height)
	}
	return m.renderCards(width, height)
}

func (m *model) cardColumns(width int) int {
	columns := 1
	switch {
	case width >= 160:
		columns = 4
	case width >= 120:
		columns = 3
	case width >= 88:
		columns = 2
	}
	for columns > 1 && (width-4-(columns-1)*2)/columns < 36 {
		columns--
	}
	return columns
}

func (m *model) renderCards(width, height int) []string {
	columns := m.cardColumns(width)
	cardWidth := (width - 4 - (columns-1)*2) / columns
	cardHeight := 9
	if height >= 40 {
		cardHeight = 10
	}
	rows := (height + 1) / (cardHeight + 1)
	if rows < 1 {
		rows = 1
	}
	pageSize := columns * rows
	start := m.pageStart(pageSize)
	end := min(start+pageSize, len(m.visible))
	var lines []string
	for rowStart := start; rowStart < end; rowStart += columns {
		rowEnd := min(rowStart+columns, end)
		cards := make([][]string, 0, columns)
		for index := rowStart; index < rowEnd; index++ {
			cards = append(cards, m.card(index, cardWidth, cardHeight))
		}
		for i := 0; i < cardHeight; i++ {
			var row []string
			for _, card := range cards {
				row = append(row, m.theme.Text.Render(card[i]))
			}
			lines = append(lines, "  "+strings.Join(row, "  "))
		}
		lines = append(lines, "")
	}
	return lines
}

func (m *model) card(index, width, height int) []string {
	snapshot := m.visible[index]
	selected := index == m.selected
	title := cleanText(snapshot.DisplayName)
	if selected {
		title = "> " + title
	}
	inner := width - 4
	top := m.cardTop(title, width)
	if selected {
		top = m.theme.Accent.Render(top)
	} else {
		top = m.theme.Text.Render(top)
	}
	body := make([]string, height-2)
	plan := snapshot.Plan
	if plan == "" {
		plan = "Plan not reported"
	}
	pinLabel := ""
	if contains(m.prefs.Pins, snapshot.Key.String()) {
		pinLabel = " · pinned"
	}
	windows := snapshot.Windows
	extra := ""
	if len(windows) > 2 {
		extra = fmt.Sprintf("+%d more", len(windows)-2)
	}
	planLine := plan + pinLabel
	if extra != "" {
		planLine = padRight(planLine, max(0, inner-ansiWidth(extra))) + extra
	}
	body[0] = m.theme.Muted.Render(padRight(planLine, inner))
	shown := windows
	if len(shown) > 2 {
		shown = shown[:2]
	}
	line := 1
	for _, window := range shown {
		if line+1 >= len(body) {
			break
		}
		remaining := window.RemainingPercent()
		gauge := domain.GaugeText(remaining, 10, m.theme.Mono)
		percent := "—"
		if remaining != nil {
			percent = fmt.Sprintf("%.0f%% left", *remaining)
		} else if domain.ValidQuantity(window.Used) {
			percent = fmt.Sprintf("%g", *window.Used)
			if domain.ValidQuantity(window.Limit) {
				percent += fmt.Sprintf("/%g", *window.Limit)
			}
			if window.Unit != "" {
				percent += " " + window.Unit
			}
		}
		if window.OverLimit() {
			percent = "Over · 0% left"
		}
		label := window.Label
		if label == "" {
			label = "Allowance"
		}
		labelPart := padRight(label, 7) + gauge
		dataWidth := max(8, inner-ansiWidth(labelPart)-2)
		body[line] = m.styleWindow(window, padRight(labelPart+"  "+padLeft(percent, dataWidth), inner), inner)
		body[line+1] = m.theme.Muted.Render("       " + padRight(domain.ResetLabel(window.ResetAt, m.now()), inner-7))
		line += 2
	}
	if len(shown) == 0 && line < len(body) && len(snapshot.Balances) > 0 {
		balance := snapshot.Balances[0]
		value := "Not reported"
		if balance.Value != nil && domain.ValidNumber(*balance.Value) {
			value = fmt.Sprintf("%.2f %s", *balance.Value, balance.Unit)
		}
		body[line] = m.theme.Muted.Render(padRight(truncateWidth(balance.Label, inner), inner))
		line++
		if line < len(body) {
			body[line] = m.theme.Text.Render(padRight(value, inner))
			line++
		}
		if line < len(body) {
			body[line] = m.theme.Muted.Render(padRight("sampled "+balanceAge(balance, snapshot, m.now()), inner))
			line++
		}
	}
	for line < len(body) {
		body[line] = ""
		line++
	}
	bottomIndex := len(body) - 1
	if bottomIndex > 0 {
		state := m.effectiveState(snapshot)
		age := ageLabel(snapshot, m.now())
		if age == "time unknown" {
			age = "age unknown"
		}
		updated := "Updated " + age
		if state == domain.StatusQueued || state == domain.StatusFetching {
			updated = stateLabel(state)
			if m.retainedPrior(snapshot) {
				updated += " · prior"
			}
			if snapshot.SampleAt != nil {
				updated += " · sample " + age
			}
		} else if state.NeedsAttention() {
			label := m.stateDisplay(snapshot)
			label = truncateWidth(label, max(1, inner-ansiWidth(age)-3))
			updated = label + " · " + age
		}
		body[bottomIndex] = m.styleState(state, padRight(updated, inner))
	}
	border := m.theme.Border
	if selected {
		border = m.theme.Accent
	}
	lines := []string{top}
	side := "│"
	if m.theme.Mono {
		side = "|"
	}
	for _, row := range body {
		lines = append(lines, border.Render(side)+" "+m.theme.Text.Render(padRight(row, inner))+" "+border.Render(side))
	}
	bottom := "╰" + strings.Repeat("─", width-2) + "╯"
	if m.theme.Mono {
		bottom = "+" + strings.Repeat("-", width-2) + "+"
	}
	lines = append(lines, border.Render(bottom))
	for index := range lines {
		lines[index] = m.surface(lines[index])
	}
	return lines
}

func (m *model) surface(line string) string {
	if m.theme.Mono {
		return line
	}
	sequence := theme.BackgroundSequence(m.theme.Palette.Surface)
	return sequence + strings.ReplaceAll(line, "\x1b[m", "\x1b[m"+sequence)
}

func (m *model) renderCompact(width, height int) []string {
	reduced := width < 70
	var nameWidth, allowanceWidth, leftWidth, resetWidth int
	if reduced {
		nameWidth, allowanceWidth, leftWidth, resetWidth = 10, 8, 14, 8
	} else {
		nameWidth, allowanceWidth, leftWidth, resetWidth = 22, 10, 16, 12
	}
	header := padRight("Source", nameWidth) + padRight("Allowance", allowanceWidth) + padRight("Left", leftWidth) + padRight("Resets", resetWidth) + "State"
	lines := []string{"  " + m.theme.Muted.Render(header)}
	start, end := m.compactViewport(height)
	index := start
	for index < end {
		snapshot := m.visible[index]
		selected := index == m.selected
		prefix := "  "
		if selected {
			prefix = "> "
		}
		window := domain.AllowanceWindow{}
		left, reset := "—", "—"
		label := "—"
		if len(snapshot.Windows) > 0 {
			window = snapshot.Windows[0]
			label, left = compactWindowValue(window)
			reset = compactReset(window.ResetAt, m.now())
		} else if len(snapshot.Balances) > 0 {
			balance := snapshot.Balances[0]
			label = balance.Label
			if balance.Value != nil && domain.ValidNumber(*balance.Value) {
				left = fmt.Sprintf("%.2f %s", *balance.Value, balance.Unit)
			}
			reset = truncateWidth(balanceAge(balance, snapshot, m.now()), resetWidth)
		}
		line := prefix + m.theme.Text.Render(padRight(snapshot.DisplayName, nameWidth)) +
			m.theme.Muted.Render(padRight(label, allowanceWidth)+padRight(left, leftWidth))
		line += m.theme.Muted.Render(padRight(reset, resetWidth))
		state := m.effectiveState(snapshot)
		line += m.styleState(state, stateLabel(state))
		if len(lines) < height {
			lines = append(lines, line)
		}
		if m.retainedPrior(snapshot) && hasMeasurements(snapshot) && len(lines) < height {
			lines = append(lines, "    "+m.theme.Low.Render(fmt.Sprintf("prior data · %s", ageLabel(snapshot, m.now()))))
		}
		if selected && len(snapshot.Windows) > 1 {
			second := snapshot.Windows[1]
			secondLabel, left2 := compactWindowValue(second)
			second.Label = secondLabel
			secondLine := "  " + m.theme.Muted.Render(padRight("", nameWidth)+padRight(second.Label, allowanceWidth)+padRight(left2, leftWidth)+padRight(compactReset(second.ResetAt, m.now()), resetWidth))
			lines = append(lines, secondLine)
		}
		index++
	}
	if m.selected >= start && m.selected < end {
		if selected, ok := m.selectedSnapshot(); ok {
			lines = append(lines, "")
			lines = append(lines, "  "+m.theme.Muted.Render(truncateWidth(fmt.Sprintf("%s · %s · %s · %s", selected.DisplayName, emptyFallback(selected.Plan, "Plan not reported"), m.stateDisplay(selected), ageLabel(selected, m.now())), width-4)))
		}
	}
	return lines
}

func (m *model) renderEmpty(width, _ int) []string {
	title := "Your subscriptions, together."
	body := []string{
		"No tracked sources.",
		"Make room for your whole toolkit.",
		"",
		"Choose sources from the library to build your desk.",
		"Claude, Codex, Muse, Google, and many more.",
		"",
		"p Choose sources     1–6 Explore demo scenarios",
	}
	if len(m.all) > 0 && m.query == "" {
		switch m.filter {
		case "pinned":
			title = "No pinned sources"
			body = []string{"Nothing is pinned yet.", "", "Press f to clear the filter, or Space on a source to pin it."}
		case "attention":
			title = "Nothing needs attention"
			body = []string{"All tracked sources look healthy.", "", "Press f to clear the filter."}
		default:
			title = "No matching sources"
			body = []string{"The current filter has no matches.", "", "Press f to clear the filter."}
		}
	}
	backendError := m.setupError
	if m.session != nil && m.session.BackendError() != "" {
		backendError = m.session.BackendError()
	}
	if backendError != "" {
		if m.backendVersion != "" && m.session != nil {
			// The backend binary was found and reported a version, but its
			// data could not be read: the guidance is about the data, not
			// about locating the executable.
			title = "CodexBar data unreadable"
			body = []string{
				"The backend runs, but its data could not be read.",
				cleanText(backendError),
				"",
				"The CodexBar build may be newer or older than this app expects.",
				"Keep its resource bundle beside the executable.",
				"Linux credentials can require a keyring or backend cookie setup.",
				"Demo mode never contacts CodexBar or your accounts.",
				"",
				"r Refresh     , Open settings     d Explore the demo",
			}
		} else {
			title = "Connect CodexBar"
			body = []string{
				"Setup · no backend connection",
				cleanText(backendError),
				"",
				"Set the executable in Settings, or add codexbar to PATH.",
				"Keep its resource bundle beside the executable.",
				"Linux credentials can require a keyring or backend cookie setup.",
				"Demo mode never contacts CodexBar or your accounts.",
				"",
				", Open settings     d Explore the demo",
			}
		}
	} else if m.query != "" {
		title = "No results"
		body = []string{
			fmt.Sprintf("No results for %q.", m.query),
			"",
			"Clear the search or change the filter.",
			"",
			"/ Clear search     f Change filter",
		}
	}
	var lines []string
	lines = append(lines, "", "")
	lines = append(lines, "  "+m.theme.Accent.Bold(true).Render(title), "")
	boxWidth := min(width-8, 78)
	for _, line := range body {
		lines = append(lines, "  "+m.theme.Text.Render(padRight("  "+line, boxWidth)))
	}
	return lines
}

func (m *model) styleWindow(window domain.AllowanceWindow, text string, _ int) string {
	remaining := window.RemainingPercent()
	switch {
	case window.Quality == domain.QualityStale:
		return m.theme.Stale.Render(text)
	case remaining == nil:
		return m.theme.Muted.Render(text)
	case *remaining <= 0:
		return m.theme.Error.Render(text)
	case *remaining <= 20:
		return m.theme.Low.Render(text)
	default:
		return m.theme.Text.Render(text)
	}
}

func (m *model) styleState(state domain.SourceStatus, text string) string {
	switch state {
	case domain.StatusLow:
		return m.theme.Low.Render(text)
	case domain.StatusExhausted, domain.StatusError, domain.StatusAuthFailed, domain.StatusUnavailable, domain.StatusBackendIncompatible:
		return m.theme.Error.Render(text)
	case domain.StatusStale:
		return m.theme.Stale.Render(text)
	case domain.StatusSignIn:
		return m.theme.Low.Render(text)
	case domain.StatusPartial:
		return m.theme.Low.Render(text)
	default:
		return m.theme.Muted.Render(text)
	}
}

func (m *model) pageStart(pageSize int) int {
	if pageSize <= 0 || len(m.visible) == 0 {
		return 0
	}
	page := m.selected / pageSize
	start := page * pageSize
	if start >= len(m.visible) {
		start = 0
	}
	return start
}

func pageStart(m *model) int {
	if m.prefs.Layout == config.LayoutCompact || (m.prefs.Layout != config.LayoutCards && (m.width < 88 || m.height < 32)) {
		start, _ := m.compactViewport(max(1, m.height-6))
		return start
	}
	return m.pageStart(m.pageSize())
}

func pageEnd(m *model) int {
	if m.prefs.Layout == config.LayoutCompact || (m.prefs.Layout != config.LayoutCards && (m.width < 88 || m.height < 32)) {
		_, end := m.compactViewport(max(1, m.height-6))
		return end
	}
	return pageStart(m) + m.pageSize()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func stateLabel(state domain.SourceStatus) string {
	switch state {
	case domain.StatusQueued:
		return "Queued"
	case domain.StatusFetching:
		return "Fetching"
	case domain.StatusCurrent:
		return "Current"
	case domain.StatusLow:
		return "Low"
	case domain.StatusExhausted:
		return "Exhausted"
	case domain.StatusSignIn:
		return "Sign-in"
	case domain.StatusAuthFailed:
		return "Auth failed"
	case domain.StatusStale:
		return "Stale"
	case domain.StatusMissingWindow:
		return "Allowance not reported"
	case domain.StatusUnavailable:
		return "Unavailable"
	case domain.StatusPartial:
		return "Partial"
	case domain.StatusBackendIncompatible:
		return "Backend incompatible"
	case domain.StatusError:
		return "Error"
	default:
		return "Unknown"
	}
}

func ansiWidth(value string) int {
	return lipgloss.Width(value)
}

func compactWindowValue(window domain.AllowanceWindow) (label, value string) {
	label = window.Label
	if label == "" {
		label = "Allowance"
	}
	if window.OverLimit() {
		return label, "Over 0%"
	}
	if remaining := window.RemainingPercent(); remaining != nil {
		return label, fmt.Sprintf("%.0f%%", *remaining)
	}
	if domain.ValidQuantity(window.Used) {
		if domain.ValidQuantity(window.Limit) {
			remaining := *window.Limit - *window.Used
			if remaining < 0 {
				remaining = 0
			}
			value = compactQuantity(remaining) + "/" + compactQuantity(*window.Limit)
			if window.Unit != "" {
				value += " " + window.Unit
			}
		} else {
			return label, "—"
		}
		return label, value
	}
	return label, "—"
}

func compactQuantity(value float64) string {
	format := func(divisor float64, suffix string) string {
		return strings.TrimSuffix(fmt.Sprintf("%.1f", value/divisor), ".0") + suffix
	}
	switch {
	case value >= 1_000_000_000:
		return format(1_000_000_000, "G")
	case value >= 1_000_000:
		return format(1_000_000, "M")
	case value >= 1_000:
		return format(1_000, "k")
	default:
		return fmt.Sprintf("%g", value)
	}
}

func compactReset(reset *time.Time, now time.Time) string {
	if reset == nil {
		return "—"
	}
	if !reset.After(now) {
		return "Due"
	}
	return domain.HumanDuration(reset.Sub(now))
}

func (m *model) cardTop(title string, width int) string {
	left, right, fill := "╭─ ", "╮", "─"
	if m.theme.Mono {
		left, right, fill = "+- ", "+", "-"
	}
	inner := width - ansiWidth(left) - ansiWidth(right)
	if inner < 1 {
		return truncateWidth(left, width)
	}
	title = truncateWidth(title, max(0, inner-1))
	padding := max(0, inner-ansiWidth(title)-1)
	return left + title + " " + strings.Repeat(fill, padding) + right
}
