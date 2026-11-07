package game

import "testing"

var loreXenoEggs = []string{"xeno_egg"}

// Xeno eggs cluster around the hive's origin; on a wreck_eggs_hatch site that
// is the old wreck's hull.
func TestLoreXenosGrowFromTheOrigin(t *testing.T) {
	for _, ch := range []string{"meteor_seeding", "wreck_eggs_hatch"} {
		ms, ox, oy, _ := loreBuild(t, loreSite{Map: "moon", Scenario: "exterminate_xenos", Seed: 41, Chapter: ch,
			Flags: map[string]any{"origin_civ": "Vael", "has_wreck": 1.0}})
		near, far := loreNear(ms.level, loreXenoEggs, ox, oy, 21)
		spawned := toSetupFloat(ms.level.Flags["xeno_origin_eggs"])
		if near < 5 || float64(near) != spawned || far != 0 {
			t.Errorf("%s: %d eggs within 21 tiles of the origin, %d elsewhere, script says %v", ch, near, far, spawned)
		}
	}
}

func TestLoreXenosUnchangedWithoutHistory(t *testing.T) {
	loreUnchangedWithoutHistory(t, "moon", "exterminate_xenos", "xeno_purge_setup", 41, loreXenoEggs)
}
