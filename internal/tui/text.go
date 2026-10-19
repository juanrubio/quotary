package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// cleanText removes control sequences and control characters from external text.
func cleanText(value string) string {
	value = ansi.Strip(value)
	var b strings.Builder
	for _, r := range value {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(' ')
		case unicode.IsControl(r):
			continue
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func truncateWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(value) <= width {
		return value
	}
	return ansi.Truncate(value, width, "…")
}

func padRight(value string, width int) string {
	value = truncateWidth(value, width)
	gap := width - ansi.StringWidth(value)
	if gap <= 0 {
		return value
	}
	return value + strings.Repeat(" ", gap)
}

func padLeft(value string, width int) string {
	value = truncateWidth(value, width)
	gap := width - ansi.StringWidth(value)
	if gap <= 0 {
		return value
	}
	return strings.Repeat(" ", gap) + value
}

func cleanInput(value string) string {
	value = ansi.Strip(value)
	var b strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) && r != ' ' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func tailWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	start := len(runes)
	for start > 0 && ansi.StringWidth(string(runes[start-1:])) <= width {
		start--
	}
	if start == len(runes) {
		return ""
	}
	return string(runes[start:])
}

func wrapText(value string, width int) []string {
	value = cleanText(value)
	if width <= 0 {
		return []string{value}
	}
	var lines []string
	current := ""
	flush := func() {
		if current != "" {
			lines = append(lines, current)
			current = ""
		}
	}
	for _, word := range strings.Fields(value) {
		if ansi.StringWidth(word) > width {
			flush()
			chunks := splitWidth(word, width)
			for i, chunk := range chunks {
				if i == len(chunks)-1 {
					current = chunk
				} else {
					lines = append(lines, chunk)
				}
			}
			continue
		}
		if current == "" {
			current = word
			continue
		}
		if ansi.StringWidth(current)+1+ansi.StringWidth(word) > width {
			flush()
			current = word
		} else {
			current += " " + word
		}
	}
	flush()
	return lines
}

func splitWidth(value string, width int) []string {
	var chunks []string
	current := ""
	for _, r := range value {
		cell := string(r)
		if ansi.StringWidth(current)+ansi.StringWidth(cell) > width {
			chunks = append(chunks, current)
			current = ""
		}
		current += cell
	}
	if current != "" {
		chunks = append(chunks, current)
	}
	return chunks
}
