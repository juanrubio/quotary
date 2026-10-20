package codexbar

import (
	"math"
	"testing"
	"time"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/domain"
)

func FuzzNormalizeUsageNeverPanics(f *testing.F) {
	for _, seed := range []string{
		`{"provider":"codex","primary":{"usedPercent":20}}`,
		`[{"provider":"muse","accounts":[{"accountKey":"a"}]}]`,
		`{"error":{"code":"auth"}}`,
		`{"details":[42],"windows":[{"usedPercent":"NaN"}]}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body string) {
		document, err := decodeDocument([]byte(body))
		if err != nil {
			if _, ok := domain.AsScopedError(err); !ok {
				t.Fatalf("decode failure is not scoped: %v", err)
			}
			return
		}
		snapshots, _ := normalizeUsage(document, backend.UsageRequest{ProviderID: "codex"}, time.Now(), nil)
		for _, snapshot := range snapshots {
			if snapshot.Key.Provider == "" && len(snapshot.Windows) > 0 {
				t.Fatalf("measurements without provider identity: %+v", snapshot)
			}
			for _, window := range snapshot.Windows {
				if window.UsedPercent != nil && (*window.UsedPercent < 0 || math.IsNaN(*window.UsedPercent) || math.IsInf(*window.UsedPercent, 0)) {
					t.Fatalf("invalid percentage: %v", *window.UsedPercent)
				}
			}
			_ = domain.EffectiveState(snapshot)
		}
	})
}
