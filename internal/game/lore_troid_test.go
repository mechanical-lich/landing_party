package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

// A troid_outbreak site is a hive of fliers (troids): their eggs fester in
// the caves under the impact and the first adults stand around it.
func TestLoreFliersGrowFromTheImpact(t *testing.T) {
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
	checkHiveMorph(t, ms.level, "flier", "troid_egg", "troid_stage2")
}

func TestLoreFliersUnchangedWithoutHistory(t *testing.T) {
	loreUnchangedWithoutHistory(t, "moon", "troid_outbreak", "troid_outbreak_setup", 53, []string{"troid_egg", "troid_stage2"})
}

// checkHiveMorph checks that every creature of the given blueprints on level
// is of the hive faction and of morph, with no type and no presence (a
// scenario's own hive, not a campaign presence's).
func checkHiveMorph(t *testing.T, level *world.Level, morph string, blueprints ...string) {
	t.Helper()
	want := map[string]bool{}
	for _, b := range blueprints {
		want[b] = true
	}
	n := 0
	for _, e := range level.Entities {
		if !want[e.Blueprint] {
			continue
		}
		n++
		f := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction
		k := components.KinOf(e)
		if f != "hive" || k == nil || k.Morph != morph || k.Type != "" || k.Presence != "" {
			t.Fatalf("%s: faction %q kin %+v, want hive %s", e.Blueprint, f, k, morph)
		}
	}
	if n == 0 {
		t.Errorf("no %v on the level", blueprints)
	}
}
