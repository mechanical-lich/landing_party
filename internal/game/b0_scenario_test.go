package game

import (
	"testing"

	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/scenario"
)

func b0SpawnScenario(id, blueprint string) *scenario.Scenario {
	return &scenario.Scenario{
		ID:         id,
		HostileMax: 3,
		SpawnRules: map[string]scenario.SpawnRule{blueprint: {SpawnRate: 1, Tiles: []string{"hull_floor"}}},
	}
}

func b0Spawned(ms *MainState) map[string]int {
	out := map[string]int{}
	for _, e := range ms.level.Entities {
		out[e.Blueprint]++
	}
	return out
}

// Two levels stepped side by side each spawn from their own scenario, whichever
// one the global says is in orbit.
func TestLevelsSpawnFromTheirOwnScenario(t *testing.T) {
	b0Campaign(t, "a")
	a, b := b0Planet(t, "A"), b0Planet(t, "B")
	a.scenario = b0SpawnScenario("a", "mutant_grunt")
	b.scenario = b0SpawnScenario("b", "critter")
	for i := 0; i < 20; i++ {
		fspath.ResetFrameCounter()
		a.stepBackground()
		b.stepWorld()
	}
	gotA, gotB := b0Spawned(a), b0Spawned(b)
	if gotA["mutant_grunt"] == 0 || gotA["critter"] != 0 {
		t.Errorf("level A spawned %v, want only mutant_grunt", gotA)
	}
	if gotB["critter"] == 0 || gotB["mutant_grunt"] != 0 {
		t.Errorf("level B spawned %v, want only critter", gotB)
	}
}

// Building a location takes its scenario from its own config and moves the orbit;
// building another level afterwards must not disturb the first.
func TestLoadedLevelKeepsItsScenario(t *testing.T) {
	if err := scenario.Load("data/scenarios"); err != nil {
		t.Fatalf("load scenarios: %v", err)
	}
	empty := t.TempDir()
	t.Cleanup(func() { _ = scenario.Load(empty) })
	if err := scenario.SelectByID("alien_world"); err != nil {
		t.Fatal(err)
	}
	orbited := scenario.Active()

	b0Campaign(t, "a")
	first, err := newMainStateFromLevel(b0FloorLevel(), SettlementConfig{Name: "A", ScenarioID: "earth_like"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first.teardown)
	second, err := newMainStateFromLevel(b0FloorLevel(), SettlementConfig{Name: "B", ScenarioID: "crashed_ufo"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(second.teardown)

	if first.scenario == nil || first.scenario.ID != "earth_like" {
		t.Errorf("first level scenario = %+v, want earth_like", first.scenario)
	}
	if second.scenario == nil || second.scenario.ID != "crashed_ufo" {
		t.Errorf("second level scenario = %+v, want crashed_ufo", second.scenario)
	}
	if scenario.Active() != orbited {
		t.Errorf("constructing a level moved the orbited scenario to %s", scenario.Active().ID)
	}
}

// The ship has no scenario, so it never spawns, even with one in orbit.
func TestShipIgnoresOrbitedScenario(t *testing.T) {
	wm := NewWorldManager(b0Campaign(t, "a"))
	before := len(wm.shipLevel.level.Entities)
	wm.shipLevel.gm.Update(wm.shipLevel.scenario)
	wm.shipLevel.gm.suppressSpawns = false
	wm.shipLevel.gm.Update(wm.shipLevel.scenario)
	if got := len(wm.shipLevel.level.Entities); got != before {
		t.Errorf("ship spawned %d entities", got-before)
	}
}
