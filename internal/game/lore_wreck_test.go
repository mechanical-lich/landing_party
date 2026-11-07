package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/mapdef"
)

var loreUFOCrew = []string{"greyish", "whitish", "greenish"}

func loreHull(crew string) mapdef.FeatureBlock {
	p := map[string]any{"script": "crashed_ufo", "w": 17.0, "h": 17.0}
	if crew != "" {
		p["crew"] = crew
	}
	return mapdef.FeatureBlock{Kind: "stamp", Count: 1, At: "origin:ev1", Params: p}
}

// A history wreck is an old hull: the stamp at its origin spawns no crew. The
// same stamp without crew = "dead" (a live crash) still does.
func TestLoreWreckEvidenceHasNoLiveCrew(t *testing.T) {
	ms, ox, oy, _ := loreBuild(t, loreSite{Map: "moon", Scenario: "moon", Seed: 31, Chapter: "ancient_wreck",
		Features: []mapdef.FeatureBlock{loreHull("dead")}})
	if near, _ := loreNear(ms.level, loreUFOCrew, ox, oy, 14); near != 0 {
		t.Errorf("%d live crew at the old wreck", near)
	}
	if lockers, _ := loreNear(ms.level, []string{"storage_locker"}, ox, oy, 14); lockers == 0 {
		t.Error("no hull (its locker) at the wreck's origin")
	}
	live, ox2, oy2, _ := loreBuild(t, loreSite{Map: "moon", Scenario: "moon", Seed: 31, Chapter: "scout_accident",
		Features: []mapdef.FeatureBlock{loreHull("")}})
	if near, _ := loreNear(live.level, loreUFOCrew, ox2, oy2, 14); near < 4 {
		t.Errorf("a live crash has %d crew at its origin, want 5", near)
	}
}
