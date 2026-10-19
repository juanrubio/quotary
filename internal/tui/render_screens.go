package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/juanrubio/quotary/internal/config"
	"github.com/juanrubio/quotary/internal/domain"
)

func (m *model) renderDetails(width, height int) []string {
	snapshot, ok := m.selectedSnapshot()
	if !ok {
		return m.renderEmpty(width, height)
	}
	title := truncateWidth(snapshot.DisplayName+" · "+snapshot.Plan, max(10, width-4))
	lines := []string{
		"  " + m.theme.Accent.Bold(true).Render(title),
		m.rule(width),
		"  " + m.tab("Allowances", m.detailTab == 0) + "   " + m.tab("History", m.detailTab == 1) + "   " + m.theme.Muted.Render("Esc Overview"),
		"",
	}
	if m.detailTab == 1 {
		lines = append(lines, "  "+m.rangeControl(width), "")
		return append(lines, m.historyLines(width, height-len(lines))...)
	}
	contentHeight := max(1, height-len(lines))
	rightWidth := 0
	leftWidth := width - 4
	rightAlloc := width - 4
	if width >= 100 {
		rightWidth = 36
		rightAlloc = rightWidth
		leftWidth = width - rightWidth - 6
	}
	left := m.detailLeft(snapshot, leftWidth)
	right := m.detailRight(snapshot, rightAlloc)
	if rightWidth > 0 {
		maxRows := max(len(left), len(right))
		var combined []string
		for row := 0; row < maxRows; row++ {
			leftRow, rightRow := "", ""
			if row < len(left) {
				leftRow = left[row]
			}
			if row < len(right) {
				rightRow = right[row]
			}
			combined = append(combined, padRight(leftRow, leftWidth)+"  "+rightRow)
		}
		left = combined
	} else if len(right) > 0 {
		left = append(left, "")
		left = append(left, right...)
	}
	offset := min(m.detailScroll, max(0, len(left)-contentHeight))
	if offset < 0 {
		offset = 0
	}
	if offset < len(left) {
		left = left[offset:]
	}
	for len(left) < contentHeight {
		left = append(left, "")
	}
	if len(left) > contentHeight {
		left = left[:contentHeight]
	}
	return append(lines, left...)
}

func (m *model) detailLeft(snapshot domain.UsageSnapshot, width int) []string {
	var lines []string
	state := m.effectiveState(snapshot)
	if m.retainedPrior(snapshot) && hasMeasurements(snapshot) {
		lines = append(lines, "  "+m.styleState(state, m.stateDisplay(snapshot)+" · "+ageLabel(snapshot, m.now())), "")
	}
	for _, window := range snapshot.Windows {
		label := window.Label
		if window.Group != "" {
			label = window.Group + " · " + label
		}
		if label == "" {
			label = "Allowance"
		}
		value := "Not reported"
		gauge := ""
		if remaining := window.RemainingPercent(); remaining != nil {
			value = fmt.Sprintf("%.0f%% left", *remaining)
			if window.UsedPercent != nil && window.OverLimit() {
				value = fmt.Sprintf("%g%% used · %s", *window.UsedPercent, value)
			}
			if domain.ValidQuantity(window.Used) {
				raw := fmt.Sprintf("%g", *window.Used)
				if domain.ValidQuantity(window.Limit) {
					raw += fmt.Sprintf("/%g", *window.Limit)
				}
				if window.Unit != "" {
					raw += " " + window.Unit
				}
				value += " · " + raw
			}
			gauge = domain.GaugeText(remaining, max(10, width-16), m.theme.Mono)
		} else if domain.ValidQuantity(window.Used) {
			value = fmt.Sprintf("Used %g", *window.Used)
			if domain.ValidQuantity(window.Limit) {
				value += fmt.Sprintf(" / %g", *window.Limit)
			}
			if window.Unit != "" {
				value += " " + window.Unit
			}
		}
		for _, wrapped := range wrapText(label+"  "+value, max(10, width-4)) {
			lines = append(lines, "  "+m.styleState(domain.DeriveState(domain.UsageSnapshot{Windows: []domain.AllowanceWindow{window}}, false), wrapped))
		}
		if gauge != "" {
			lines = append(lines, "  "+m.styleWindow(window, gauge, width))
		}
		if window.OverLimit() {
			lines = append(lines, "  "+m.theme.Error.Render("Over limit · reported used percentage exceeds 100"))
		}
		absolute := domain.ResetAbsolute(window.ResetAt, m.location)
		lines = append(lines, "  "+m.theme.Muted.Render(domain.ResetLabel(window.ResetAt, m.now())+" · "+absolute))
		lines = append(lines, "")
	}
	if len(snapshot.Windows) == 0 {
		lines = append(lines, "  "+m.theme.Muted.Render("Allowance not reported"))
	}
	for _, balance := range snapshot.Balances {
		value := "Not reported"
		if balance.Value != nil && domain.ValidNumber(*balance.Value) {
			value = fmt.Sprintf("%.2f %s", *balance.Value, balance.Unit)
		}
		for _, wrapped := range wrapText(balance.Label+"  "+value+" · sampled "+balanceAge(balance, snapshot, m.now()), max(10, width-4)) {
			lines = append(lines, "  "+m.theme.Text.Render(wrapped))
		}
	}
	for _, detail := range snapshot.Details {
		for _, wrapped := range wrapText(detail.Title, max(10, width-4)) {
			lines = append(lines, "  "+m.theme.Text.Render(wrapped))
		}
		for _, row := range detail.Rows {
			value := row.Value
			if m.prefs.Privacy && (row.Sensitive || domain.LooksSensitive(row.Value)) {
				value = "Hidden · privacy mode"
			}
			for _, wrapped := range wrapText(row.Label+": "+value, max(10, width-4)) {
				lines = append(lines, "    "+m.theme.Text.Render(wrapped))
			}
		}
		for _, bar := range detail.Bars {
			value := "Not reported"
			if bar.Value != nil && domain.ValidNumber(*bar.Value) {
				value = fmt.Sprintf("%g", *bar.Value)
				if bar.Max != nil && domain.ValidNumber(*bar.Max) {
					value += fmt.Sprintf(" / %g", *bar.Max)
				}
				if bar.Unit != "" {
					value += " " + bar.Unit
				}
			}
			for _, wrapped := range wrapText(bar.Label+": "+value, max(10, width-4)) {
				lines = append(lines, "    "+m.theme.Text.Render(wrapped))
			}
		}
		for _, point := range detail.Points {
			value := "Not reported"
			if point.Value != nil && domain.ValidNumber(*point.Value) {
				value = fmt.Sprintf("%g", *point.Value)
			}
			for _, wrapped := range wrapText(point.Label+": "+value, max(10, width-4)) {
				lines = append(lines, "    "+m.theme.Text.Render(wrapped))
			}
		}
		if detail.Unsupported {
			lines = append(lines, "    "+m.theme.Low.Render("Additional data not yet displayed"))
		}
	}
	return lines
}

func (m *model) detailRight(snapshot domain.UsageSnapshot, width int) []string {
	if width <= 0 {
		return nil
	}
	status := m.stateDisplay(snapshot)
	refresh := "No refresh recorded"
	if m.session != nil {
		refresh = m.session.RefreshOutcome(snapshot.Key.String())
	}
	if snapshot.Error != nil {
		refresh = snapshot.Error.Error()
	}
	return m.box("Reading", m.readingRows(width, []string{
		"SOURCE", emptyFallback(snapshot.DisplayName, "Unknown"),
		"STATUS", status,
		"LAST SUCCESS", ageLabel(snapshot, m.now()),
		"ACCOUNT", m.accountLabel(snapshot),
		"PLAN", emptyFallback(snapshot.Plan, "Plan not reported"),
		"REPORTED SOURCE", emptyFallback(snapshot.ReportedSource, "Unknown"),
		"SOURCE KEY", m.displayKey(snapshot),
		"REFRESH", refresh,
	}), width)
}

func (m *model) readingRows(width int, fields []string) []string {
	inner := max(4, width-4)
	var rows []string
	for i := 0; i+1 < len(fields); i += 2 {
		rows = append(rows, fields[i])
		rows = append(rows, wrapText(fields[i+1], inner)...)
		rows = append(rows, "")
	}
	return rows
}

func (m *model) displayKey(snapshot domain.UsageSnapshot) string {
	key := snapshot.Key
	if m.prefs.Privacy && key.Account != "" {
		key.Account = "hidden"
		key.AccountUnknown = false
	}
	return key.String()
}

func (m *model) tab(label string, active bool) string {
	if active {
		return m.theme.Accent.Bold(true).Render("[" + label + "]")
	}
	return m.theme.Muted.Render(label)
}

func (m *model) historyLines(width, height int) []string {
	if m.historyErr != "" {
		return []string{"", "  " + m.theme.Low.Render(m.historyErr)}
	}
	history := m.history
	selected, ok := m.selectedSnapshot()
	if ok && history.Key.Provider != "" && history.Key.String() != selected.Key.String() {
		return []string{"", "  " + m.theme.Low.Render("History does not match the selected source. Press r to reload.")}
	}
	if history.Key.Provider == "" && history.TotalTokens == nil && len(history.Buckets) == 0 {
		return []string{"", "  " + m.theme.Muted.Render("History unavailable for this source.")}
	}
	if width < 100 || height < 22 {
		return m.scrollHistory(m.compactHistory(history, width), height)
	}
	days := m.historyDays
	if days != 30 {
		days = 7
	}
	rangeText := fmt.Sprintf("%s – %s", history.Start.Format("2 Jan"), history.End.Format("2 Jan 2006"))
	var lines []string
	for _, line := range wrapText(fmt.Sprintf("Last %d days · %s · %s", days, history.Timezone, history.Provenance), max(10, width-4)) {
		lines = append(lines, "  "+m.theme.Muted.Render(line))
	}
	lines = append(lines, "  "+m.theme.Muted.Render(rangeText), "")
	for _, line := range wrapText(history.Coverage, max(10, width-4)) {
		lines = append(lines, "  "+m.theme.Muted.Render(line))
	}
	if history.Partial {
		lines = append(lines, "  "+m.theme.Low.Render("Partial coverage · not a complete account total"))
	}
	if history.Incomplete > 0 {
		lines = append(lines, "  "+m.theme.Low.Render(fmt.Sprintf("%d incomplete records", history.Incomplete)))
	}
	if history.TotalTokens != nil {
		lines = append(lines, "  "+m.theme.Accent.Bold(true).Render(formatTokens(*history.TotalTokens)+" known tokens"))
	}
	lines = append(lines, "  "+m.theme.Muted.Render(fmt.Sprintf("%d of %d days with recorded data", recordedDays(history), len(history.Buckets))))
	lines = append(lines, "", "  "+m.theme.Muted.Render("DAILY TOTALS"))
	lines = append(lines, dailyBreakdown(history, width)...)
	lines = append(lines, "  "+m.theme.Muted.Render("Days: "+bucketDates(history)))
	if len(history.MissingDays) > 0 {
		lines = append(lines, "  "+m.theme.Muted.Render("?"+strings.Join(history.MissingDays, ", ")+" unavailable · known total excludes gaps"))
	}
	if len(history.Models) > 0 {
		lines = append(lines, "", "  "+m.theme.Muted.Render("TOKEN BREAKDOWN"))
		lines = append(lines, modelBreakdown(history.Models, width)...)
	}
	if history.Cost != nil {
		label := "Estimated API-price cost"
		if history.CostReported {
			label = "Reported cost"
		}
		lines = append(lines, "", "  "+m.theme.Text.Render(fmt.Sprintf("%s: %.2f %s", label, history.Cost.Amount, history.Cost.Currency)))
	} else if history.Priced {
		lines = append(lines, "", "  "+m.theme.Muted.Render("Pricing unavailable for some records"))
	} else {
		lines = append(lines, "", "  "+m.theme.Muted.Render("Token history only; no dollar total is available."))
	}
	lines = append(lines, "", "  "+m.theme.Muted.Render("TOKENS / DAY"), "")
	lines = append(lines, m.chartLines(history, width)...)
	return m.scrollHistory(lines, height)
}

func (m *model) scrollHistory(lines []string, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(lines) <= height {
		return lines
	}
	offset := min(m.historyScroll, max(0, len(lines)-height))
	return lines[offset : offset+height]
}

func dailyBreakdown(history domain.HistorySnapshot, width int) []string {
	var lines []string
	current := "  "
	for _, bucket := range history.Buckets {
		day := bucket.Day
		if len(day) >= 10 {
			day = day[5:10]
		}
		value := "?"
		if bucket.Tokens != nil {
			value = formatTokens(*bucket.Tokens)
		}
		piece := day + " " + value + "   "
		if ansiWidth(current)+ansiWidth(piece) > max(10, width-4) {
			lines = append(lines, current)
			current = "  "
		}
		current += piece
	}
	if strings.TrimSpace(current) != "" {
		lines = append(lines, current)
	}
	return lines
}

func (m *model) chartLines(history domain.HistorySnapshot, width int) []string {
	buckets := history.Buckets
	barWidth := 2
	if len(buckets) > max(1, (width-12)/2) {
		barWidth = 1
	}
	maxBuckets := max(1, (width-12)/barWidth)
	if len(buckets) > maxBuckets {
		buckets = buckets[len(buckets)-maxBuckets:]
	}
	var maxTokens int64
	for _, bucket := range buckets {
		if bucket.Tokens != nil && *bucket.Tokens > maxTokens {
			maxTokens = *bucket.Tokens
		}
	}
	const chartHeight = 5
	var lines []string
	for row := chartHeight - 1; row >= 0; row-- {
		var b strings.Builder
		b.WriteString("  ")
		for _, bucket := range buckets {
			if bucket.Tokens == nil {
				if row == 2 {
					b.WriteString("?")
					b.WriteString(strings.Repeat(" ", barWidth-1))
				} else {
					b.WriteString(strings.Repeat(" ", barWidth))
				}
				continue
			}
			level := int(float64(*bucket.Tokens) / float64(max(1, int(maxTokens))) * chartHeight)
			if level > row {
				fill := "█"
				if m.theme.Mono {
					fill = "#"
				}
				b.WriteString(strings.Repeat(fill, barWidth))
			} else {
				b.WriteString(strings.Repeat(" ", barWidth))
			}
		}
		lines = append(lines, m.theme.Text.Render(b.String()))
	}
	return lines
}

func (m *model) renderHistory(width, height int) []string {
	snapshot, ok := m.selectedSnapshot()
	if !ok {
		return m.renderEmpty(width, height)
	}
	lines := []string{
		"  " + m.theme.Accent.Bold(true).Render(snapshot.DisplayName),
		m.rule(width),
		"  " + m.tab("Overview", false) + "   " + m.tab("Allowances", false) + "   " + m.tab("History", true) + "   " + m.rangeControl(width),
		"",
	}
	lines = append(lines, m.historyLines(width, height-len(lines))...)
	return lines
}

func (m *model) renderPicker(width, height int) []string {
	entries := m.pickerEntries()
	tracked := 0
	for _, descriptor := range m.catalog.Providers {
		if descriptor.Tracked {
			tracked++
		}
	}
	uniqueProviders := map[string]bool{}
	for _, descriptor := range m.catalog.Providers {
		uniqueProviders[descriptor.ProviderID] = true
	}
	lines := []string{
		"  " + m.theme.Accent.Bold(true).Render("Source library") + "  " + m.theme.Muted.Render(fmt.Sprintf("%d providers · %d selectable sources", len(uniqueProviders), len(m.catalog.Providers))),
		"",
		"  " + m.theme.Text.Render("Track the subscriptions you use.") + padLeft(m.theme.Muted.Render(fmt.Sprintf("%d tracked", tracked)), max(0, width-40)),
		"",
		"  " + m.theme.Accent.Render("[ Track all subscriptions ]") + "    " + m.theme.Muted.Render("Tracking does not sign you in."),
		"",
	}
	search := m.pickerQuery
	if m.searching {
		search += "▌"
	}
	lines = append(lines, "  "+m.theme.Muted.Render("/ Search the full catalog: ")+m.theme.Text.Render(search), "")
	headerName, headerKind := 40, 20
	if width < 110 {
		headerName, headerKind = 30, 16
	}
	if width < 90 {
		headerName, headerKind = 22, 0
	}
	header := "       " + padRight("Source", headerName)
	if headerKind > 0 {
		header += padRight("Kind", headerKind)
	}
	if width >= 110 {
		header += padRight("Catalog", 12)
	}
	header += "Last reading"
	lines = append(lines, "  "+m.theme.Muted.Render(header))
	start, end := m.pickerViewport(height)
	nameWidth, kindWidth := 40, 20
	if width < 110 {
		nameWidth, kindWidth = 30, 16
	}
	if width < 90 {
		nameWidth, kindWidth = 22, 0
	}
	for index := start; index < end && len(lines) < height-2; index++ {
		descriptor := entries[index]
		prefix := "  "
		if index == m.pickerIndex {
			prefix = "> "
		}
		mark := "[ ]"
		if descriptor.Tracked {
			mark = "[x]"
		}
		last := stateLabel(descriptor.LastRead)
		if descriptor.LastRead == "" {
			last = "No reading"
		}
		if descriptor.Capabilities.ActiveProbe == domain.CapabilitySupported {
			last = "Manual setup · active probe"
		}
		line := prefix + mark + "  " + padRight(descriptor.DisplayName, nameWidth)
		if kindWidth > 0 {
			line += padRight(string(descriptor.Kind), kindWidth)
		}
		if width >= 110 {
			availability := "available"
			if !descriptor.Available {
				availability = "unavailable"
			}
			line += padRight(availability, 12)
		}
		line += last
		if index == m.pickerIndex {
			line = m.theme.Accent.Render(line)
		} else {
			line = m.theme.Text.Render(line)
		}
		lines = append(lines, line)
		if index == m.pickerIndex && width < 110 {
			availability := "available"
			if !descriptor.Available {
				availability = "unavailable"
			}
			lines = append(lines, "    "+m.theme.Muted.Render(fmt.Sprintf("%s · %s · %s", descriptor.Kind, availability, last)))
		}
	}
	return lines
}

func (m *model) renderSettings(width, height int) []string {
	prefs := m.settingsDraft
	if prefs.Theme == "" {
		prefs = m.prefs
	}
	themeLine := ""
	for _, name := range []config.ThemeName{config.ThemeGreen, config.ThemeBlue, config.ThemeAmber, config.ThemeMonokai} {
		label := map[config.ThemeName]string{config.ThemeGreen: "Green", config.ThemeBlue: "Blue", config.ThemeAmber: "Amber", config.ThemeMonokai: "Monokai"}[name]
		if name == prefs.Theme {
			themeLine += m.theme.Accent.Bold(true).Render("["+label+"]") + "  "
		} else {
			themeLine += m.theme.Muted.Render(label) + "  "
		}
	}
	backend := cleanText(prefs.BackendPath)
	configPath := cleanText(prefs.BackendConfigPath)
	masked := "Hidden · privacy mode"
	if backend == "" {
		backend = "auto-detect on PATH"
	}
	if configPath == "" {
		configPath = "auto-detect"
	}
	if m.settingsEditing {
		field := tailWidth(cleanInput(m.settingsInput), max(8, width-20)) + "▌"
		if m.settingsIdx == 6 {
			backend = field
			if prefs.Privacy {
				configPath = masked
			}
		} else {
			configPath = field
			if prefs.Privacy {
				backend = masked
			}
		}
	} else if prefs.Privacy {
		backend, configPath = masked, masked
	}
	rows := []string{
		"  Theme        " + themeLine,
		"  Appearance   " + m.choiceRow([]string{"auto", "dark", "light"}, string(prefs.Appearance)),
		"  Layout       " + m.choiceRow([]string{"auto", "cards", "compact"}, string(prefs.Layout)),
		fmt.Sprintf("  Interval     %d s  (←→ adjust, 60 s minimum)", prefs.IntervalSeconds),
		"  Privacy      " + m.choiceRow([]string{"on", "off"}, map[bool]string{true: "on", false: "off"}[prefs.Privacy]),
		"  Monochrome   " + m.choiceRow([]string{"on", "off"}, map[bool]string{true: "on", false: "off"}[prefs.Monochrome]),
		"  Backend path " + backend,
		"  Config path  " + configPath,
	}
	lines := []string{
		"  " + m.theme.Accent.Bold(true).Render("Settings"),
		m.rule(width),
		"",
	}
	for index, row := range rows {
		if index == m.settingsIdx {
			lines = append(lines, "> "+m.theme.Text.Render(row))
		} else {
			lines = append(lines, "  "+m.theme.Muted.Render(row))
		}
	}
	lines = append(lines,
		"",
		"  "+m.theme.Muted.Render("Theme changes preview immediately. Enter saves; Esc cancels."),
		"  "+m.theme.Muted.Render("Enter a path row to edit; type, then Enter to commit."),
		"",
	)
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines[:min(len(lines), height)]
}

func (m *model) choiceRow(values []string, current string) string {
	var parts []string
	for _, value := range values {
		if value == current {
			parts = append(parts, m.theme.Accent.Bold(true).Render("["+value+"]"))
		} else {
			parts = append(parts, m.theme.Muted.Render(value))
		}
	}
	return strings.Join(parts, "  ")
}

var helpRows = [][2]string{
	{"↑↓←→ / j k", "Move between sources"},
	{"Enter / Esc", "Open details / return"},
	{"Tab / Shift+Tab", "Move fields or switch details tabs"},
	{"↑↓ / j k", "Choose a theme or move between fields"},
	{"← / →", "Adjust the focused setting"},
	{"e", "Edit a backend path field"},
	{"a", "Track all subscriptions in the picker"},
	{"[ / ]", "Change history range"},
	{"/", "Search the current collection"},
	{"f / s", "Cycle filter / sort"},
	{"Space", "Pin source / track in library"},
	{"p / l", "Source library / layout"},
	{"r / R", "Refresh selected / all"},
	{"PgUp / PgDn", "Page through the collection"},
	{",", "Open settings"},
	{"1–6", "Switch demo scenario"},
	{"? / q / Ctrl+C", "Help / quit"},
}

func (m *model) renderHelp(width, height int) []string {
	lines := []string{
		"  " + m.theme.Accent.Bold(true).Render("Keyboard guide"),
		m.rule(width),
		"",
		"  A keyboard-first desk.",
		"",
	}
	visibleRows := max(1, height-5)
	start := min(m.helpScroll, max(0, len(helpRows)-visibleRows))
	for _, row := range helpRows[start:] {
		if len(lines) >= height {
			break
		}
		lines = append(lines, fmt.Sprintf("  %-20s %s", row[0], m.theme.Muted.Render(row[1])))
	}
	return lines
}

func (m *model) retainedPrior(snapshot domain.UsageSnapshot) bool {
	if snapshot.Error != nil || snapshot.Quality == domain.QualityStale ||
		snapshot.State == domain.StatusStale || snapshot.State == domain.StatusAuthFailed {
		return true
	}
	if base, ok := m.baseStates[snapshot.Key.String()]; ok &&
		(base == domain.StatusStale || base == domain.StatusAuthFailed) {
		return true
	}
	for _, window := range snapshot.Windows {
		if window.Quality == domain.QualityStale {
			return true
		}
	}
	interval := time.Duration(m.prefs.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	var received *time.Time
	if !snapshot.ReceivedAt.IsZero() {
		value := snapshot.ReceivedAt
		received = &value
	}
	if domain.CheckFreshness(snapshot.SampleAt, received, m.now(), interval, false).Stale {
		return true
	}
	for _, balance := range snapshot.Balances {
		if balance.SampleAt != nil && domain.CheckFreshness(balance.SampleAt, nil, m.now(), interval, false).Stale {
			return true
		}
	}
	return false
}

func (m *model) effectiveState(snapshot domain.UsageSnapshot) domain.SourceStatus {
	if snapshot.State == domain.StatusQueued || snapshot.State == domain.StatusFetching {
		return snapshot.State
	}
	var received *time.Time
	if !snapshot.ReceivedAt.IsZero() {
		value := snapshot.ReceivedAt
		received = &value
	}
	interval := time.Duration(m.prefs.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	freshness := domain.CheckFreshness(snapshot.SampleAt, received, m.now(), interval, snapshot.Error != nil)
	stale := freshness.Stale
	for _, balance := range snapshot.Balances {
		if balance.SampleAt != nil && domain.CheckFreshness(balance.SampleAt, nil, m.now(), interval, false).Stale {
			stale = true
		}
	}
	return domain.DeriveState(snapshot, stale)
}

func (m *model) accountLabel(snapshot domain.UsageSnapshot) string {
	if m.prefs.Privacy {
		if snapshot.Key.AccountUnknown || snapshot.Key.Account == "" {
			return "Account unknown · masked"
		}
		return "Hidden · privacy mode"
	}
	return emptyFallback(snapshot.AccountLabel, "Account unknown")
}

func (m *model) box(title string, body []string, width int) []string {
	inner := max(1, width-4)
	if m.theme.Mono {
		lines := []string{m.theme.Border.Render("+- " + truncateWidth(title, inner-2) + " " + strings.Repeat("-", max(0, inner-2-len(title))) + " +")}
		for _, text := range body {
			lines = append(lines, m.theme.Border.Render("|")+" "+m.theme.Text.Render(padRight(text, inner))+" "+m.theme.Border.Render("|"))
		}
		lines = append(lines, m.theme.Border.Render("+"+strings.Repeat("-", width-2)+"+"))
		return lines
	}
	lines := []string{m.theme.Border.Render("╭─ " + truncateWidth(title, inner-2) + " " + strings.Repeat("─", max(0, inner-2-len(title))) + " ╮")}
	for _, text := range body {
		if text == "" {
			lines = append(lines, m.theme.Border.Render("│")+" "+strings.Repeat(" ", inner)+" "+m.theme.Border.Render("│"))
			continue
		}
		lines = append(lines, m.theme.Border.Render("│")+" "+m.theme.Text.Render(padRight(text, inner))+" "+m.theme.Border.Render("│"))
	}
	lines = append(lines, m.theme.Border.Render("╰"+strings.Repeat("─", width-2)+"╯"))
	for index := range lines {
		lines[index] = m.surface(lines[index])
	}
	return lines
}

func emptyFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func formatTokens(value int64) string {
	switch {
	case value >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(value)/1_000_000)
	case value >= 1_000:
		return fmt.Sprintf("%.1fk", float64(value)/1_000)
	default:
		return fmt.Sprintf("%d", value)
	}
}

func modelBreakdown(models []domain.ModelTotal, width int) []string {
	var lines []string
	limit := max(10, width-4)
	for _, model := range models {
		total := modelTotal(model)
		nameWidth := max(8, limit-ansiWidth(total)-2)
		parts := wrapText(model.Name, nameWidth)
		for i, part := range parts {
			if i == len(parts)-1 {
				lines = append(lines, "  "+part+"  "+total)
			} else {
				lines = append(lines, "  "+part)
			}
		}
	}
	return lines
}

func modelTotal(model domain.ModelTotal) string {
	if model.Input == nil && model.Output == nil {
		return "Unavailable"
	}
	var total int64
	if model.Input != nil {
		total += *model.Input
	}
	if model.Output != nil {
		total += *model.Output
	}
	if model.Input == nil || model.Output == nil {
		return formatTokens(total) + " known"
	}
	return formatTokens(total)
}

func balanceAge(balance domain.Balance, snapshot domain.UsageSnapshot, now time.Time) string {
	sample := balance.SampleAt
	if sample == nil {
		sample = snapshot.SampleAt
	}
	info := domain.CheckFreshness(sample, nil, now, 5*time.Minute, false)
	return info.AgeLabel()
}

func ageLabel(snapshot domain.UsageSnapshot, now time.Time) string {
	var received *time.Time
	if !snapshot.ReceivedAt.IsZero() {
		value := snapshot.ReceivedAt
		received = &value
	}
	info := domain.CheckFreshness(snapshot.SampleAt, received, now, 5*time.Minute, snapshot.Error != nil)
	return info.AgeLabel()
}

func (m *model) rangeControl(width int) string {
	if width < 70 {
		if m.historyDays == 30 {
			return m.theme.Muted.Render("7d") + " " + m.theme.Accent.Bold(true).Render("[30d]") + " " + m.theme.Muted.Render("[ ]")
		}
		return m.theme.Accent.Bold(true).Render("[7d]") + " " + m.theme.Muted.Render("30d") + " " + m.theme.Muted.Render("[ ]")
	}
	if m.historyDays == 30 {
		return m.theme.Muted.Render("7 days") + "  " + m.theme.Accent.Bold(true).Render("[30 days]") + "  " + m.theme.Muted.Render("[ / ] change")
	}
	return m.theme.Accent.Bold(true).Render("[7 days]") + "  " + m.theme.Muted.Render("30 days") + "  " + m.theme.Muted.Render("[ / ] change")
}

func (m *model) compactHistory(history domain.HistorySnapshot, width int) []string {
	var lines []string
	for _, line := range wrapText(fmt.Sprintf("Last %d days · %s", history.Days, history.Timezone), max(10, width-4)) {
		lines = append(lines, "  "+m.theme.Muted.Render(line))
	}
	for _, line := range wrapText(history.Provenance, max(10, width-4)) {
		lines = append(lines, "  "+m.theme.Muted.Render(line))
	}
	for _, line := range wrapText(history.Coverage, max(10, width-4)) {
		lines = append(lines, "  "+m.theme.Muted.Render(line))
	}
	if history.Partial {
		lines = append(lines, "  "+m.theme.Low.Render("Partial coverage · not a complete account total"))
	}
	if history.Incomplete > 0 {
		lines = append(lines, "  "+m.theme.Low.Render(fmt.Sprintf("%d incomplete records", history.Incomplete)))
	}
	if history.TotalTokens != nil {
		lines = append(lines, "  "+m.theme.Accent.Bold(true).Render(formatTokens(*history.TotalTokens)+" known tokens"))
	}
	if len(history.MissingDays) > 0 {
		lines = append(lines, "  "+m.theme.Low.Render(strings.Join(history.MissingDays, ", ")+" unavailable"))
	}
	switch {
	case history.Cost != nil:
		label := "Estimated API-price cost"
		if history.CostReported {
			label = "Reported cost"
		}
		lines = append(lines, "  "+m.theme.Text.Render(fmt.Sprintf("%s: %.2f %s", label, history.Cost.Amount, history.Cost.Currency)))
	case history.Priced:
		lines = append(lines, "  "+m.theme.Low.Render("Pricing unavailable for some records"))
	default:
		lines = append(lines, "  "+m.theme.Muted.Render("Token history only; no dollar total."))
	}
	lines = append(lines, "", "  "+m.theme.Muted.Render("DAILY TOTALS"))
	lines = append(lines, dailyBreakdown(history, width)...)
	if len(history.Models) > 0 {
		lines = append(lines, "", "  "+m.theme.Muted.Render("TOKEN BREAKDOWN"))
		lines = append(lines, modelBreakdown(history.Models, width)...)
	}
	lines = append(lines, "", "  "+m.theme.Muted.Render("TOKENS / DAY"), "")
	lines = append(lines, m.chartLines(history, width)...)
	return lines
}

func recordedDays(history domain.HistorySnapshot) int {
	count := 0
	for _, bucket := range history.Buckets {
		if bucket.Tokens != nil {
			count++
		}
	}
	return count
}

func (m *model) stateDisplay(snapshot domain.UsageSnapshot) string {
	state := m.effectiveState(snapshot)
	label := stateLabel(state)
	if m.retainedPrior(snapshot) && hasMeasurements(snapshot) {
		label += " · prior"
	}
	return label
}

func bucketDates(history domain.HistorySnapshot) string {
	var parts []string
	for _, bucket := range history.Buckets {
		if len(bucket.Day) >= 10 {
			parts = append(parts, bucket.Day[8:10])
		} else {
			parts = append(parts, bucket.Day)
		}
	}
	return strings.Join(parts, " ")
}
