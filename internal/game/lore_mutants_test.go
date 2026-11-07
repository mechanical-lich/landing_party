package game

import "testing"

// The blight's crystal epicenters and mutated critters radiate from its origin.
func TestLoreBlightRadiatesFromTheOrigin(t *testing.T) {
	ms, ox, oy, _ := loreBuild(t, loreSite{Map: "alien", Scenario: "crystalline_blight", Seed: 61, Chapter: "blight_spread"})
	n := int(toSetupFloat(ms.level.Flags["blight_origin_crystals"]))
	crystals, _ := loreNear(ms.level, []string{"alien_crystal"}, ox, oy, 20)
	critters, crittersFar := loreNear(ms.level, []string{"critter"}, ox, oy, 21)
	if n < 4 || crystals < 3*n {
		t.Errorf("%d crystal clusters by the script, %d crystals within 20 tiles of the origin", n, crystals)
	}
	if critters == 0 || float64(critters) != toSetupFloat(ms.level.Flags["blight_origin_critters"]) || crittersFar != 0 {
		t.Errorf("%d critters at the origin, %d elsewhere", critters, crittersFar)
	}
}

func TestLoreBlightUnchangedWithoutHistory(t *testing.T) {
	loreUnchangedWithoutHistory(t, "alien", "crystalline_blight", "crystalline_blight_setup", 61, []string{"critter", "mutant_scout"})
}

// Mutants stand around the leak on an alien world, and around the experiment
// on a station.
func TestLoreMutantsGrowFromTheOrigin(t *testing.T) {
	for _, c := range []loreSite{
		{Map: "alien", Scenario: "alien_world", Seed: 67, Chapter: "reactor_mutation", Flags: map[string]any{"has_reactor": 1.0}},
		{Map: "abandoned_station", Scenario: "abandoned_station", Seed: 67, Chapter: "station_experiment", RolledX: 0.5, RolledY: 0.5},
	} {
		ms, ox, oy, _ := loreBuild(t, c)
		near, far := loreNear(ms.level, []string{"mutant_grunt"}, ox, oy, 16)
		if near == 0 || float64(near) != toSetupFloat(ms.level.Flags["mutant_origin_spawned"]) || far != 0 {
			t.Errorf("%s: %d grunts at the origin, %d elsewhere, script says %v", c.Scenario, near, far, ms.level.Flags["mutant_origin_spawned"])
		}
	}
}

func TestLoreMutantsUnchangedWithoutHistory(t *testing.T) {
	loreUnchangedWithoutHistory(t, "alien", "alien_world", "alien_setup", 67, []string{"mutant_grunt"})
	ms, _, _, _ := loreBuild(t, loreSite{Map: "abandoned_station", Scenario: "abandoned_station", Seed: 67})
	if n, _ := loreNear(ms.level, []string{"mutant_grunt"}, 0, 0, 1<<20); n != 0 {
		t.Errorf("a station with no history starts with %d grunts, want none as before", n)
	}
}
