package game

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/mechanical-lich/landing_party/internal/mapdef"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/scenario"
)

func heapNow() uint64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// TestMeasureLoadedPlanet prints what one loaded planet costs. It backs the
// loaded-planet cap; run with BG_MEASURE=1 go test ./internal/game -run Measure -v.
func TestMeasureLoadedPlanet(t *testing.T) {
	if os.Getenv("BG_MEASURE") == "" {
		t.Skip("set BG_MEASURE=1 to measure")
	}
	if err := mapdef.Load("data/maps"); err != nil {
		t.Fatal(err)
	}
	if err := scenario.Load("data/scenarios"); err != nil {
		t.Fatal(err)
	}
	b0Campaign(t, "x")
	type run struct{ mapID, scenarioID string }
	var kept []*MainState // all stay loaded, so each delta is one more planet's cost
	for _, r := range []run{
		{"asteroid", "asteroid_field"}, {"moon", ""}, {"earth_like", "earth_like"},
		{"alien", "alien_world"}, {"alien", "crystalline_blight"}, {"alien", "alien_world"},
	} {
		before := heapNow()
		name := r.mapID + "/" + r.scenarioID
		ms, err := NewMainState(SettlementConfig{Name: name, ColonyName: name, MapID: r.mapID, ScenarioID: r.scenarioID, Seed: int64(len(kept)*7 + 42), CampaignMode: true})
		if err != nil {
			t.Fatal(err)
		}
		ms.teardown()
		kept = append(kept, ms)
		after := heapNow()
		for i := 0; i < 5; i++ {
			b0Worker(t, ms, ms.MainSettlement.CenterX+i, ms.MainSettlement.CenterY)
		}
		for i := 0; i < 300; i++ { // hostiles spawn and settle in
			fspath.ResetFrameCounter()
			ms.stepBackground()
		}
		start := time.Now()
		const n = 200
		for i := 0; i < n; i++ {
			fspath.ResetFrameCounter()
			ms.stepBackground()
		}
		perTick := time.Since(start) / n
		settled := heapNow()
		lvl := ms.level
		volume := lvl.GetWidth() * lvl.GetHeight() * lvl.GetDepth()
		t.Logf("%s: %dx%dx%d = %d tiles, %d bytes/tile; %d entities; heap +%.0f MB at load, +%.0f MB after 500 ticks; %v per bg tick",
			name, lvl.GetWidth(), lvl.GetHeight(), lvl.GetDepth(), volume, int(after-before)/volume, len(lvl.Entities),
			float64(after-before)/1e6, float64(settled-before)/1e6, perTick)
	}
	runtime.KeepAlive(kept)
}
