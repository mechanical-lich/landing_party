package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/mapdef"
)

// contentHostileBlueprints lists the blueprints that act on their own and are
// not passive wildlife (critters wander) or the colony's: anything with an AI
// that is not FactionAI "wander".
func contentHostileBlueprints(t *testing.T) map[string]bool {
	t.Helper()
	paths, _ := filepath.Glob("data/blueprints/entities/*.json")
	out := map[string]bool{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]map[string]json.RawMessage
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		for id, bp := range m {
			_, scripted := bp["ScriptedAI"]
			raw, faction := bp["FactionAI"]
			var ai struct{ BehaviorKey, Faction string }
			if faction {
				_ = json.Unmarshal(raw, &ai)
			}
			if (scripted || faction) && ai.BehaviorKey != "wander" && ai.Faction != "colony" {
				out[id] = true
			}
		}
	}
	if !out["mutant_grunt"] || !out["xeno_egg"] || !out["yeti"] || out["critter"] {
		t.Fatalf("hostile classification is off: grunt %v egg %v yeti %v critter %v", out["mutant_grunt"], out["xeno_egg"], out["yeti"], out["critter"])
	}
	return out
}

// A quiet site is quiet: every history map's base scenario, built with no
// history, starts with no hostile creature anywhere on the map.
func TestContentQuietBasesSpawnNothingHostile(t *testing.T) {
	loreLoadData(t)
	hostile := contentHostileBlueprints(t)
	for _, m := range contentHistoryMaps() {
		md := mapdef.ByID(m)
		ms, _, _, _ := loreBuild(t, loreSite{Map: m, Scenario: md.BaseScenario, Seed: 101})
		n := map[string]int{}
		for _, e := range ms.level.Entities {
			if e != nil && hostile[e.Blueprint] {
				n[e.Blueprint]++
			}
		}
		if len(n) > 0 {
			t.Errorf("%s (%s): quiet start has hostiles %v", m, md.BaseScenario, n)
		}
	}
}
