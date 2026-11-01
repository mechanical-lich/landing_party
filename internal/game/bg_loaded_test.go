package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// bgWorld is a campaign whose locations are all frozen on disk, so Travel thaws
// hand-built levels instead of generating maps. Each ID in staffed gets a worker.
func bgWorld(t *testing.T, staffed []string, unstaffed ...string) *WorldManager {
	t.Helper()
	c := b0Campaign(t, append(append([]string{}, staffed...), unstaffed...)...)
	wm := NewWorldManager(c)
	freeze := func(id string, worker bool) {
		ms := b0Planet(t, locationColonyName(c, c.Locations[id]))
		if worker {
			b0Worker(t, ms, 1, 1)
		}
		if err := wm.Freeze(c.Locations[id], ms); err != nil {
			t.Fatalf("freeze %s: %v", id, err)
		}
	}
	for _, id := range staffed {
		freeze(id, true)
	}
	for _, id := range unstaffed {
		freeze(id, false)
	}
	c.CurrentLocationID = ""
	return wm
}

func bgTravel(t *testing.T, wm *WorldManager, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if err := wm.Travel(id); err != nil {
			t.Fatalf("travel %s: %v", id, err)
		}
	}
}

func assertShipAlive(t *testing.T, wm *WorldManager) {
	t.Helper()
	if settlement.Settlements[wm.shipSettlementName()] != wm.shipLevel.MainSettlement {
		t.Error("ship settlement diverged from the global map")
	}
}

func TestStaffedPlanetsStayLoadedAcrossTravel(t *testing.T) {
	wm := bgWorld(t, []string{"a", "b"})
	bgTravel(t, wm, "a")
	a := wm.Current()
	settlementA := a.MainSettlement
	bgTravel(t, wm, "b")
	if !wm.IsLoaded("a") || wm.loaded["a"] != a {
		t.Fatal("staffed planet a left memory")
	}
	if settlement.Settlements[settlementA.Name] != settlementA {
		t.Error("staffed planet's settlement left the global map")
	}
	if wm.Current() == a || wm.currentID != "b" {
		t.Errorf("current = %q, want b", wm.currentID)
	}
	bgTravel(t, wm, "a")
	if wm.Current() != a {
		t.Error("returning to a loaded planet rebuilt it")
	}
	if !wm.IsLoaded("b") {
		t.Error("staffed planet b left memory")
	}
	assertShipAlive(t, wm)
}

func TestUnstaffedPlanetFreezesOnDeparture(t *testing.T) {
	wm := bgWorld(t, []string{"b"}, "a")
	bgTravel(t, wm, "a")
	name := wm.Current().colonyName()
	bgTravel(t, wm, "b")
	if wm.IsLoaded("a") {
		t.Error("unstaffed planet stayed in memory")
	}
	if _, ok := settlement.Settlements[name]; ok {
		t.Error("unstaffed planet's settlement stayed in the global map")
	}
	assertShipAlive(t, wm)
}

// A hostile carrying Worker doesn't keep a planet loaded; a lone robot does.
func TestStaffedCountsRobotsNotHostiles(t *testing.T) {
	wm := bgWorld(t, []string{"z"}, "hostile", "robot")
	bgTravel(t, wm, "hostile")
	b0Add(t, wm.Current(), "mutant_grunt", "raiders:hostile", 3, 3)
	bgTravel(t, wm, "robot")
	if wm.IsLoaded("hostile") {
		t.Error("a hostile worker kept the planet loaded")
	}
	b0Add(t, wm.Current(), "hauler_robot", wm.Current().colonyName(), 3, 3)
	bgTravel(t, wm, "z")
	if !wm.IsLoaded("robot") {
		t.Error("a robot-only planet froze")
	}
}

func TestLoadedCapFreezesLeastRecentlyVisited(t *testing.T) {
	wm := bgWorld(t, []string{"a", "b", "c", "d"})
	bgTravel(t, wm, "a", "b", "c", "a", "d") // b is now the stalest
	if wm.IsLoaded("b") {
		t.Error("least recently visited planet stayed loaded")
	}
	for _, id := range []string{"a", "c", "d"} {
		if !wm.IsLoaded(id) {
			t.Errorf("%s evicted", id)
		}
	}
	if loc := wm.Campaign.Locations["b"]; loc.Colonists != 1 {
		t.Errorf("evicted planet recorded %d colonists, want 1", loc.Colonists)
	}
	if wm.TotalColonists() != 4 {
		t.Errorf("total colonists = %d, want 4", wm.TotalColonists())
	}
}

// Save and load brings every loaded planet back, with settlement and queue.
func TestSaveAndLoadRestoresAllLoadedPlanets(t *testing.T) {
	useRealCosts(t)
	wm := bgWorld(t, []string{"a", "b"}, "empty")
	bgTravel(t, wm, "empty", "a", "b")
	if wm.IsLoaded("empty") {
		t.Fatal("empty planet should have frozen")
	}
	orders := map[string]*task.Task{}
	for _, id := range []string{"a", "b"} {
		orders[id] = b0QueueBuild(wm.loaded[id], 5, 5)
	}
	queueShipBuildOrder(wm.shipLevel.MainSettlement)
	if err := wm.SaveCampaign(wm.Current()); err != nil {
		t.Fatalf("save: %v", err)
	}
	settlement.Settlements = make(map[string]*settlement.Settlement) // a new process
	c, err := LoadCampaign(wm.Campaign.Name)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	loaded := NewWorldManager(c)
	if err := loaded.EnterStartingOrbit(); err != nil {
		t.Fatalf("orbit: %v", err)
	}
	if loaded.currentID != "b" {
		t.Errorf("orbit = %q, want b", loaded.currentID)
	}
	if loaded.IsLoaded("empty") {
		t.Error("frozen planet came back loaded")
	}
	for _, id := range []string{"a", "b"} {
		ms := loaded.loaded[id]
		if ms == nil {
			t.Fatalf("%s not restored", id)
		}
		if ms.MainSettlement == nil || settlement.Settlements[c.Locations[id].SettlementName] != ms.MainSettlement {
			t.Errorf("%s settlement not adopted", id)
		}
		if ms.MainSettlement != nil && ms.MainSettlement.Tasks.GetNextTask() == nil {
			t.Errorf("%s lost its queue", id)
		}
	}
	assertShipAlive(t, loaded)
	if len(loaded.shipLevel.MainSettlement.Tasks.GetTasks()) != 1 {
		t.Error("ship lost its queue")
	}
}

// Only entering orbit moves the orbited scenario.
func TestRestoringAwayPlanetDoesNotMoveOrbitedScenario(t *testing.T) {
	if err := scenario.Load("data/scenarios"); err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	t.Cleanup(func() { _ = scenario.Load(empty) })
	wm := bgWorld(t, []string{"a", "b"})
	wm.Campaign.Locations["a"].ScenarioID = "earth_like"
	wm.Campaign.Locations["b"].ScenarioID = "alien_world"

	bgTravel(t, wm, "a")
	if got := scenario.Active().ID; got != "earth_like" {
		t.Errorf("orbited = %s", got)
	}
	if ms, err := wm.buildParked("b"); err != nil {
		t.Fatal(err)
	} else if ms.scenario.ID != "alien_world" {
		t.Errorf("parked scenario = %s", ms.scenario.ID)
	}
	if got := scenario.Active().ID; got != "earth_like" {
		t.Errorf("loading without orbiting moved the orbit to %s", got)
	}
	bgTravel(t, wm, "b")
	if got := scenario.Active().ID; got != "alien_world" {
		t.Errorf("orbited = %s after travel", got)
	}
}

// A fixture posted while a planet stayed loaded appears when the player arrives.
func TestFixtureSpawnsOnReturnToLoadedPlanet(t *testing.T) {
	wm := bgWorld(t, []string{"a", "b"})
	bgTravel(t, wm, "a", "b")
	wm.Campaign.Locations["a"].AddFixture(campaign.QuestFixture{QuestID: "q", Blueprint: "mutant_grunt", Kind: "boss"})
	bgTravel(t, wm, "a")
	var found *ecs.Entity
	for _, e := range wm.Current().level.Entities {
		if e.Blueprint == "mutant_grunt" {
			found = e
		}
	}
	if found == nil {
		t.Error("fixture did not spawn")
	}
}

// Planetside crew on the orbited planet counts while the player is aboard.
func TestTotalColonistsCountsEveryLoadedPlanet(t *testing.T) {
	wm := bgWorld(t, []string{"a", "b"})
	bgTravel(t, wm, "a", "b")
	if got := wm.TotalColonists(); got != 2 {
		t.Errorf("total = %d, want 2", got)
	}
}
