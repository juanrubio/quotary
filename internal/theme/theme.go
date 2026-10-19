// Package theme defines Quotary's semantic color tokens and named palettes.
package theme

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// Palette is the complete semantic palette for one theme variant.
type Palette struct {
	Canvas    string
	Surface   string
	Border    string
	Text      string
	Muted     string
	Accent    string
	Available string
	Low       string
	Error     string
	Stale     string
}

// Names lists supported theme names.
var Names = []string{"green", "blue", "amber", "monokai"}

// Normalize maps absent or unknown names to Green.
func Normalize(name string) string {
	for _, candidate := range Names {
		if name == candidate {
			return name
		}
	}
	return "green"
}

// For returns the named palette in light or dark.
func For(name string, light bool) Palette {
	switch Normalize(name) {
	case "blue":
		if light {
			return Palette{"#f6f9fe", "#eaf0f9", "#bccbdc", "#20334b", "#52677b", "#215fa0", "#166534", "#92400e", "#be123c", "#6d28d9"}
		}
		return Palette{"#171e29", "#202b3a", "#35465c", "#e5edf5", "#a4b3c5", "#91c7ff", "#86efac", "#fcd34d", "#fda4af", "#c4b5fd"}
	case "amber":
		if light {
			return Palette{"#fdf9f1", "#f3ecdf", "#d6c8b1", "#3d3021", "#73634f", "#845000", "#166534", "#92400e", "#be123c", "#6d28d9"}
		}
		return Palette{"#242019", "#30291f", "#514536", "#f0e8db", "#bdb09a", "#edc17a", "#86efac", "#fcd34d", "#fda4af", "#c4b5fd"}
	case "monokai":
		if light {
			return Palette{"#ffffff", "#f5f5f2", "#c4c4b8", "#272822", "#66665e", "#486b00", "#2e7d32", "#8a6d00", "#b00020", "#006b80"}
		}
		return Palette{"#000000", "#161616", "#49483e", "#f8f8f2", "#a6a69c", "#a6e22e", "#a6e22e", "#e6db74", "#f92672", "#66d9ef"}
	default:
		if light {
			return Palette{"#f8fafc", "#ffffff", "#cbd5e1", "#0f172a", "#475569", "#0369a1", "#166534", "#92400e", "#be123c", "#6d28d9"}
		}
		return Palette{"#0b1020", "#151d2c", "#344155", "#e6edf3", "#a6b2c3", "#7dd3fc", "#86efac", "#fcd34d", "#fda4af", "#c4b5fd"}
	}
}

// Styles is the resolved style set for one rendering context.
type Styles struct {
	Name    string
	Light   bool
	Mono    bool
	Palette Palette
	Canvas  lipgloss.Style
	Surface lipgloss.Style
	Border  lipgloss.Style
	Focus   lipgloss.Style
	Text    lipgloss.Style
	Muted   lipgloss.Style
	Accent  lipgloss.Style
	Good    lipgloss.Style
	Low     lipgloss.Style
	Error   lipgloss.Style
	Stale   lipgloss.Style
}

// New resolves styles for name, light/dark, and monochrome.
func New(name string, light, mono bool) Styles {
	name = Normalize(name)
	palette := For(name, light)
	style := func(color string) lipgloss.Style {
		if mono {
			return lipgloss.NewStyle()
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	}
	styles := Styles{
		Name: name, Light: light, Mono: mono, Palette: palette,
		Canvas:  lipgloss.NewStyle(),
		Surface: lipgloss.NewStyle(),
		Border:  style(palette.Border),
		Focus:   style(palette.Accent).Bold(true),
		Text:    style(palette.Text),
		Muted:   style(palette.Muted),
		Accent:  style(palette.Accent),
		Good:    style(palette.Available),
		Low:     style(palette.Low),
		Error:   style(palette.Error),
		Stale:   style(palette.Stale),
	}
	if !mono {
		styles.Canvas = styles.Canvas.Foreground(lipgloss.Color(palette.Text)).Background(lipgloss.Color(palette.Canvas))
		styles.Surface = styles.Surface.Foreground(lipgloss.Color(palette.Text)).Background(lipgloss.Color(palette.Surface))
	}
	return styles
}

// ContrastRatio computes WCAG contrast between two #RRGGBB colors.
func ContrastRatio(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(hex string) float64 {
	r, g, b := rgb(hex)
	channel := func(value float64) float64 {
		value /= 255
		if value <= 0.04045 {
			return value / 12.92
		}
		return math.Pow((value+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}

func rgb(hex string) (float64, float64, float64) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0
	}
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, 0
	}
	return float64(value >> 16 & 0xff), float64(value >> 8 & 0xff), float64(value & 0xff)
}

// ValidateContrast returns an error for any token below AA on Canvas or Surface.
func ValidateContrast(name string, light bool) error {
	palette := For(name, light)
	pairs := [][3]string{
		{"text", palette.Text, palette.Canvas},
		{"text/surface", palette.Text, palette.Surface},
		{"muted", palette.Muted, palette.Canvas},
		{"muted/surface", palette.Muted, palette.Surface},
		{"accent", palette.Accent, palette.Canvas},
		{"accent/surface", palette.Accent, palette.Surface},
		{"available", palette.Available, palette.Canvas},
		{"available/surface", palette.Available, palette.Surface},
		{"low", palette.Low, palette.Canvas},
		{"low/surface", palette.Low, palette.Surface},
		{"error", palette.Error, palette.Canvas},
		{"error/surface", palette.Error, palette.Surface},
		{"stale", palette.Stale, palette.Canvas},
		{"stale/surface", palette.Stale, palette.Surface},
	}
	for _, pair := range pairs {
		if ratio := ContrastRatio(pair[1], pair[2]); ratio < 4.5 {
			return fmt.Errorf("%s/%v %s contrast %.2f", name, light, pair[0], ratio)
		}
	}
	return nil
}

// BackgroundSequence returns the truecolor background escape for a #RRGGBB
// color, used when preserving a panel surface across nested style resets.
func BackgroundSequence(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return ""
	}
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", value>>16&0xff, value>>8&0xff, value&0xff)
}
