package game

import "testing"

var loreWirePlacers = []string{"wire_placer"}

// Autonz wire placers start at the origin: where the swarm landed, or the old
// wreck's hull.
func TestLoreAutonzGrowsFromTheOrigin(t *testing.T) {
	for _, ch := range []string{"swarm_landing", "wreck_nanites_wake"} {
		ms, ox, oy, _ := loreBuild(t, loreSite{Map: "moon", Scenario: "autonz_incursion", Seed: 43, Chapter: ch,
			Flags: map[string]any{"origin_civ": "Vael", "has_wreck": 1.0}})
		near, far := loreNear(ms.level, loreWirePlacers, ox, oy, 13)
		spawned := toSetupFloat(ms.level.Flags["autonz_origin_placers"])
		if near < 4 || float64(near) != spawned || far != 0 {
			t.Errorf("%s: %d placers within 13 tiles of the origin, %d elsewhere, script says %v", ch, near, far, spawned)
		}
	}
}

func TestLoreAutonzUnchangedWithoutHistory(t *testing.T) {
	loreUnchangedWithoutHistory(t, "moon", "autonz_incursion", "autonz_incursion_setup", 43, loreWirePlacers)
}
