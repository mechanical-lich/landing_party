package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
)

// contentHazardEvidence is a hazard's evidence as a site build gets it: its
// anchored blocks pointed at the chapter origin.
func contentHazardEvidence(t *testing.T, id string) ([]mapdef.FeatureBlock, *history.EventDef) {
	t.Helper()
	lib, err := history.DefaultLibrary()
	if err != nil {
		t.Fatal(err)
	}
	def := lib.Events[id]
	if def == nil || def.ChapterTier() != history.TierHazard {
		t.Fatalf("%s is not a hazard", id)
	}
	var out []mapdef.FeatureBlock
	for _, e := range def.Evidence {
		if e.Kind != "lore_datapad" {
			out = append(out, e.FeatureBlock)
		}
	}
	return out, def
}

// Each spawning hazard puts its creatures next to its origin on the calm base,
// within its count, and nowhere else.
func TestContentHazardSpawnsAtOrigin(t *testing.T) {
	loreLoadData(t)
	for _, c := range []struct {
		hazard, mapID string
		blueprints    []string
	}{
		{"mutant_band", "alien", []string{"mutant_grunt", "mutant_scout"}},
		{"dormant_nest", "moon", []string{"xeno_egg"}},
		{"hostile_fauna", "frozen_planet", []string{"yeti"}},
		{"restless_graves", "desert_planet", []string{"zombie"}},
		{"autonz_sleeper", "asteroid", []string{"enforcer"}},
		{"troid_clutch", "moon", []string{"troid_egg"}},
	} {
		feats, def := contentHazardEvidence(t, c.hazard)
		lo, hi := 0, 0
		for _, f := range feats {
			if bp, _ := f.Params["blueprint"].(string); bp != "" {
				lo += f.Count
				hi += max(f.Count, f.CountMax)
			}
		}
		md := mapdef.ByID(c.mapID)
		ms, ox, oy, _ := loreBuild(t, loreSite{Map: c.mapID, Scenario: md.BaseScenario, Seed: 83, Chapter: def.ID, Features: feats, RolledX: 0.5, RolledY: 0.5})
		near, far := loreNear(ms.level, c.blueprints, ox, oy, 12)
		if near < lo || near > hi || far != 0 {
			t.Errorf("%s on %s: %d at the origin (want %d-%d), %d elsewhere", c.hazard, c.mapID, near, lo, hi, far)
		}
	}
}

// A relic block (what History.Evidence leaves by a people's structure) puts
// the relic item on the ground at the structure's origin.
func TestContentRelicLiesAtOrigin(t *testing.T) {
	loreLoadData(t)
	for _, relic := range []string{"carved_idol", "old_steel_blade", "machine_parts", "fission_core", "tech_fragment", "alien_alloy"} {
		feats := []mapdef.FeatureBlock{{Kind: "scatter_entity", Count: 1, At: "origin", Params: map[string]any{"blueprint": relic, "kind": "surface"}}}
		ms, ox, oy, _ := loreBuild(t, loreSite{Map: "earth_like", Scenario: "earth_like", Seed: 89, Chapter: "civ_collapses", Features: feats, RolledX: 0.6, RolledY: 0.6})
		if near, far := loreNear(ms.level, []string{relic}, ox, oy, 6); near != 1 || far != 0 {
			t.Errorf("%s: %d at the origin, %d elsewhere; want 1 and 0", relic, near, far)
		}
	}
}
