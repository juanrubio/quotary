package theme

import "testing"

func TestAllPalettesMeetAA(t *testing.T) {
	for _, name := range Names {
		for _, light := range []bool{false, true} {
			if err := ValidateContrast(name, light); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestNormalize(t *testing.T) {
	if Normalize("monokai") != "monokai" {
		t.Fatal("monokai lost")
	}
	if Normalize("") != "green" || Normalize("neon") != "green" {
		t.Fatal("unknown theme did not normalize")
	}
}

func TestMonochromeStyles(t *testing.T) {
	for _, name := range Names {
		styles := New(name, false, true)
		if !styles.Mono {
			t.Fatalf("%s is not monochrome", name)
		}
		if styles.Text.GetForeground() != styles.Muted.GetForeground() {
			t.Fatalf("%s monochrome styles differ", name)
		}
	}
}

func TestCanvasAndSurfaceCarrySemanticForeground(t *testing.T) {
	styles := New("green", true, false)
	if styles.Canvas.GetForeground() != styles.Text.GetForeground() {
		t.Fatal("canvas does not apply the semantic text foreground")
	}
	if styles.Surface.GetForeground() != styles.Text.GetForeground() {
		t.Fatal("surface does not apply the semantic text foreground")
	}
}
