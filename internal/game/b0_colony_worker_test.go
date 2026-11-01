package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// b0Add creates a blueprint on the level, a Worker under the given settlement.
func b0Add(t *testing.T, ms *MainState, blueprint, settlementName string, x, y int) *ecs.Entity {
	t.Helper()
	e, err := factory.Create(blueprint, x, y, 0)
	if err != nil {
		t.Fatalf("create %s: %v", blueprint, err)
	}
	tagColonistSettlement(e, settlementName)
	ms.level.AddEntity(e)
	return e
}

func TestIsColonyWorker(t *testing.T) {
	worker := func(settlementName string) *ecs.Entity {
		e := &ecs.Entity{}
		e.AddComponent(&components.WorkerComponent{})
		e.AddComponent(&components.SettlementComponent{Name: settlementName})
		return e
	}
	dead := worker("Colony")
	dead.AddComponent(&rlcomponents.DeadComponent{})
	unsettled := &ecs.Entity{}
	unsettled.AddComponent(&components.WorkerComponent{})
	notWorker := &ecs.Entity{}
	notWorker.AddComponent(&components.SettlementComponent{Name: "Colony"})

	cases := []struct {
		name string
		e    *ecs.Entity
		want bool
	}{
		{"own worker", worker("Colony"), true},
		{"other settlement", worker("raiders:a"), false},
		{"dead", dead, false},
		{"no settlement", unsettled, false},
		{"not a worker", notWorker, false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		if got := components.IsColonyWorker(tc.e, "Colony"); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A hostile carrying Worker is not part of the colony, wherever colonists are
// counted or listed. A robot is.
func TestHostileWorkerIsExcludedFromColonyLists(t *testing.T) {
	c := b0Campaign(t, "a")
	wm := NewWorldManager(c)
	ms := b0Planet(t, locationColonyName(c, c.Locations["a"]))
	wm.setCurrent("a", ms)

	colonist := b0Worker(t, ms, 1, 1)
	robot := b0Add(t, ms, "hauler_robot", ms.colonyName(), 2, 1)
	b0Add(t, ms, "mutant_grunt", "raiders:a", 3, 1)
	want := map[*ecs.Entity]bool{colonist: true, robot: true}

	if got := countLevelColonists(ms.level, ms.colonyName()); got != 2 {
		t.Errorf("colonist count = %d, want 2", got)
	}
	planet := wm.PlanetColonists()
	if len(planet) != 2 {
		t.Errorf("beam list has %d entries, want 2", len(planet))
	}
	for _, e := range planet {
		if !want[e] {
			t.Errorf("beam list includes %v", e.Blueprint)
		}
	}
	entries := ms.populationEntries()
	if len(entries) != 2 {
		t.Errorf("population has %d entries, want 2", len(entries))
	}
	for _, entry := range entries {
		if !want[entry.Entity] {
			t.Errorf("population includes %v", entry.Entity.Blueprint)
		}
	}
	if got := ms.buildEvalContext().SettlementPop["colony"]; got != 2 {
		t.Errorf("objective population = %d, want 2", got)
	}

	// A wiped colony with a hostile worker still standing counts as empty.
	ms.level.RemoveEntity(colonist)
	ms.level.RemoveEntity(robot)
	if got := countLevelColonists(ms.level, ms.colonyName()); got != 0 {
		t.Errorf("after wipe colonist count = %d, want 0", got)
	}
	if len(wm.PlanetColonists()) != 0 {
		t.Error("hostile worker kept the planet's beam list non-empty")
	}

	// The same holds aboard the ship.
	crewBefore := len(wm.ShipColonists())
	b0Add(t, wm.shipLevel, "mutant_grunt", "raiders:ship", 2, 2)
	b0Add(t, wm.shipLevel, "hauler_robot", wm.shipSettlementName(), 3, 2)
	if got := len(wm.ShipColonists()); got != crewBefore+1 {
		t.Errorf("ship roster = %d, want the robot added to %d", got, crewBefore)
	}
}

// Cursor tools don't accept a hostile worker as a colonist to order or a thing to
// pick up, and do accept a robot.
func TestCursorToolsRejectHostileWorkers(t *testing.T) {
	hostile := colonistEntity()
	hostile.GetComponent(components.Settlement).(*components.SettlementComponent).Name = "raiders:a"

	h := &fakeToolHost{}
	pickup := &pickupTool{host: h}
	pickup.Request(hostile)
	if len(h.modes) != 0 {
		t.Errorf("pickup tool took orders for a hostile: %v", h.modes)
	}
	drop := &dropTool{host: h}
	drop.Request(hostile, &ecs.Entity{})
	if len(h.modes) != 0 {
		t.Errorf("drop tool took orders for a hostile: %v", h.modes)
	}
	pickup.Request(colonistEntity())
	if len(h.modes) != 1 || h.modes[0] != gui.CursorModePickup {
		t.Errorf("pickup tool refused a colonist: %v", h.modes)
	}
}
