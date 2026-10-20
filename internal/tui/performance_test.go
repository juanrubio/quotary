package tui

import (
	"testing"
	"time"

	"github.com/juanrubio/quotary/internal/backend/demo"
)

func TestFirstPaintBudget(t *testing.T) {
	if raceEnabled {
		t.Skip("timing budget is not meaningful under the race detector")
	}
	const iterations = 20
	start := time.Now()
	for i := 0; i < iterations; i++ {
		m := newModel(Options{Demo: true, Session: mustDemoSession(t)})
		m.width, m.height = 160, 48
		_ = m.View().Content
	}
	average := time.Since(start) / iterations
	t.Logf("demo first paint average: %v over %d iterations", average, iterations)
	if average > 100*time.Millisecond {
		t.Fatalf("first paint average %v exceeds 100ms", average)
	}
}

func BenchmarkFirstPaint(b *testing.B) {
	session := mustDemoSession(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := newModel(Options{Demo: true, Session: session})
		m.width, m.height = 160, 48
		_ = m.View().Content
	}
}

func mustDemoSession(tb testing.TB) *demo.Runtime {
	tb.Helper()
	runtime, err := demo.NewRuntime(demo.ScenarioSubscription, demo.Deps{Fixtures: demo.FixtureFS()})
	if err != nil {
		tb.Fatal(err)
	}
	return runtime
}
