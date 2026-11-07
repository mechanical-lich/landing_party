package game

import "testing"

// Troid eggs fester in the caves under the impact and the first adults stand
// around it.
func TestLoreTroidsGrowFromTheImpact(t *testing.T) {
	ms, ox, oy, _ := loreBuild(t, loreSite{Map: "moon", Scenario: "troid_outbreak", Seed: 53, Chapter: "impact_colony"})
	eggs, eggsFar := loreNear(ms.level, []string{"troid_egg"}, ox, oy, 21)
	adults, adultsFar := loreNear(ms.level, []string{"troid_stage2"}, ox, oy, 16)
	t.Logf("%d eggs, %d adults at the impact", eggs, adults)
	if eggs < 6 || float64(eggs) != toSetupFloat(ms.level.Flags["troid_origin_eggs"]) || eggsFar != 0 {
		t.Errorf("%d eggs under the impact, %d elsewhere, script says %v", eggs, eggsFar, ms.level.Flags["troid_origin_eggs"])
	}
	if adults == 0 || adultsFar != 0 {
		t.Errorf("%d adults around the impact, %d elsewhere", adults, adultsFar)
	}
}

func TestLoreTroidsUnchangedWithoutHistory(t *testing.T) {
	loreUnchangedWithoutHistory(t, "moon", "troid_outbreak", "troid_outbreak_setup", 53, []string{"troid_egg", "troid_stage2"})
}
