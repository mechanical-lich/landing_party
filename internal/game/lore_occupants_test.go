package game

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
)

// loreStructureScripts are the structures with default guards.
var loreStructureScripts = []string{"abandoned_reactor", "bunker", "castle", "circle_ruined_tower",
	"circle_tower", "underground_bunker", "underground_dungeon", "underground_hideout"}

// loreStampAll builds mapID quiet (no chapter: the base scenario plays as
// without a history) with every guarded structure stamped at its own origin,
// passing occupants as given ("" leaves the param out, as scenarios and quest
// fixtures do). It returns the level's brutes, named guards and lockers, and
// how many stamps were placed (a small map cannot space them all).
func loreStampAll(t *testing.T, mapID, occupants string) (brutes, guards, lockers, placed int) {
	t.Helper()
	loreLoadData(t)
	md := mapdef.ByID(mapID)
	w, h, _ := md.Size.Roll(91)
	origins := map[string][2]int{}
	var feats []mapdef.FeatureBlock
	for i, script := range loreStructureScripts {
		key := fmt.Sprintf("s%d", i)
		origins[key] = [2]int{w * (2 + 2*(i%4)) / 10, h * (3 + 4*(i/4)) / 10}
		p := map[string]any{"script": script, "w": 25.0, "h": 21.0}
		if occupants != "" {
			p["occupants"] = occupants
		}
		feats = append(feats, mapdef.FeatureBlock{Kind: "stamp", Count: 1, At: "origin:" + key, Params: p})
	}
	const colony = "Occupants Colony"
	ms, err := NewMainState(SettlementConfig{Name: colony, ColonyName: colony, MapID: mapID,
		ScenarioID: md.BaseScenario, Seed: 91, CampaignMode: true,
		SiteOrigins: origins, SiteFeatures: feats})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ms.teardown)
	placed = len(ms.level.StampFootprints)
	for _, e := range ms.level.Entities {
		switch {
		case e == nil:
		case e.Blueprint == "mutant_brute":
			brutes++
		case e.Blueprint == "storage_locker":
			lockers++
		case strings.HasSuffix(rlentity.GetName(e), "Guard") || strings.HasSuffix(rlentity.GetName(e), "Sentry"):
			guards++
		}
	}
	return
}

// A quiet site of every map, its history's structures stamped with
// occupants: "none", has no structure guards: no brute, no named guard; the
// structures themselves are there (their lockers).
func TestLoreQuietSiteStructuresAreEmpty(t *testing.T) {
	loreLoadData(t)
	var maps []string
	for _, m := range mapdef.All() {
		if m.Present != nil {
			maps = append(maps, m.ID)
		}
	}
	for _, m := range maps {
		brutes, guards, lockers, placed := loreStampAll(t, m, "none")
		if brutes != 0 || guards != 0 {
			t.Errorf("%s: %d brutes and %d guards in empty structures", m, brutes, guards)
		}
		// Stamps need open surface; a station has none, so none are placed
		// there (as before this change): only the no-guards check applies.
		// Elsewhere every placed stamp is there (its locker); a small map
		// drops the ones its stamp spacing cannot fit.
		if m != "abandoned_station" && (lockers < placed || placed < len(loreStructureScripts)/2) {
			t.Errorf("%s: %d lockers for %d of %d structures placed", m, lockers, placed, len(loreStructureScripts))
		}
	}
}

// Without the param (a scenario or quest stamp) the structures keep their
// guards and boss, as before.
func TestLoreStructuresKeepGuardsByDefault(t *testing.T) {
	brutes, guards, _, placed := loreStampAll(t, "moon", "")
	if brutes < placed || placed == 0 || guards == 0 {
		t.Errorf("default occupants: %d brutes, %d guards; want a brute per structure and its guards", brutes, guards)
	}
}

// An anchored feature's count is absolute: two caves' worth of spawns at an
// origin on a large map is two, not two scaled by the map's area.
func TestLoreAnchoredCountIsAbsolute(t *testing.T) {
	loreLoadData(t)
	md := mapdef.ByID("earth_like")
	seed := int64(1)
	for ; seed < 500; seed++ {
		if w, h, _ := md.Size.Roll(seed); float64(w*h) > 1.3*float64(md.Size.MidArea()) {
			break
		}
	}
	ms, ox, oy, _ := loreBuild(t, loreSite{Map: "earth_like", Scenario: "earth_like", Seed: seed, Chapter: "lore_test",
		Features: []mapdef.FeatureBlock{{Kind: "cave_spawner", Count: 2, At: "origin:ev1", Jitter: 80,
			Params: map[string]any{"blueprint": "xeno_egg"}}}})
	if m := ms.level.FeatureAreaMultiplier(); m <= 1.2 {
		t.Fatalf("map area multiplier %.2f: not a large map", m)
	}
	near, far := loreNear(ms.level, []string{"xeno_egg"}, ox, oy, 81)
	if near != 2 || far != 0 {
		t.Errorf("anchored cave_spawner count 2 placed %d near the origin and %d elsewhere", near, far)
	}
	if _, _, _, ok := generation.Origin(ms.level, "ev1"); !ok {
		t.Error("no origin")
	}
}
