package game

import (
	"os"
	"reflect"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/lore"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/task"
)

func TestMain(m *testing.M) {
	// MainState construction reads config and data relative to the repo root.
	if err := os.Chdir("../.."); err != nil {
		panic("chdir to repo root: " + err.Error())
	}
	if err := world.LoadTileDefinitionsDir("data/tiledefinitions"); err != nil {
		panic("load tile definitions: " + err.Error())
	}
	if err := factory.FactoryLoadDir("data/blueprints"); err != nil {
		panic("load blueprints: " + err.Error())
	}
	if err := loadContentTables(); err != nil {
		panic("load content tables: " + err.Error())
	}
	// Both load lazily once and are cached, so prime them before tests chdir to
	// their temp save dirs.
	config.Global()
	lore.RandomSettlementName()
	campaign.LoadFactionData() // the player's doctrine, read by every MainState
	if _, err := history.DefaultLibrary(); err != nil {
		panic("load history library: " + err.Error())
	}
	os.Exit(m.Run())
}

// shipTestWorld builds a campaign with one location that has a frozen save on
// disk, so EnterStartingOrbit thaws it through LoadSaveData.
func shipTestWorld(t *testing.T) *WorldManager {
	t.Helper()
	t.Chdir(t.TempDir()) // campaign saves are written relative to the cwd

	prev := settlement.Settlements
	settlement.Settlements = make(map[string]*settlement.Settlement)
	t.Cleanup(func() { settlement.Settlements = prev })

	c := &campaign.Campaign{
		Version:           campaign.CampaignVersion,
		Name:              "Test",
		CurrentLocationID: "planet",
		Locations:         map[string]*campaign.Location{"planet": {ID: "planet", Name: "Planet"}},
	}
	wm := NewWorldManager(c)
	if wm.shipLevel == nil {
		t.Fatal("ship was not built")
	}

	colony := wm.currentColonyName()
	s := settlement.NewSettlement(1, 1, 0)
	delete(settlement.Settlements, s.Name)
	s.Name = colony
	settlement.Settlements[colony] = s

	lvl := world.NewLevel(8, 8, 2)
	lvl.AllocTerrain()
	ms, err := newMainStateFromLevel(lvl, SettlementConfig{Name: colony, ColonyName: colony, CampaignMode: true})
	if err != nil {
		t.Fatalf("planet state: %v", err)
	}
	ms.teardown()
	if err := wm.Freeze(c.Locations["planet"], ms); err != nil {
		t.Fatalf("freeze planet: %v", err)
	}
	return wm
}

func queueShipBuildOrder(s *settlement.Settlement) *task.Task {
	order := &task.Task{X: 3, Y: 3, Z: 0, Action: task_requests.BuildAction}
	s.Tasks.AddTask(order)
	return order
}

// assertShipSettlementIntact checks what workers actually read: the map entry
// must be the object the ship's MainState queues orders on, and must own the
// ship's storage.
func assertShipSettlementIntact(t *testing.T, wm *WorldManager) *settlement.Settlement {
	t.Helper()
	name := wm.shipSettlementName()
	got, ok := settlement.Settlements[name]
	if !ok {
		t.Fatalf("ship settlement %q missing from global map", name)
	}
	if got != wm.shipLevel.MainSettlement {
		t.Errorf("map entry %p and ship MainSettlement %p have diverged: player orders go to one, workers read the other",
			got, wm.shipLevel.MainSettlement)
	}
	sc := wm.shipLocker()
	if sc == nil {
		t.Fatal("ship has no storage container")
	}
	if sc.OwnedBy != got.Name {
		t.Errorf("ship storage owned by %q, want %q", sc.OwnedBy, got.Name)
	}
	if wm.ShipHoldEntity().GetComponent(components.Storage) == nil {
		t.Error("ship hold lost its storage component")
	}
	return got
}

func hasTask(s *settlement.Settlement, want *task.Task) bool {
	for _, tk := range s.Tasks.GetTasks() {
		if tk == want {
			return true
		}
	}
	return false
}

// A build order queued on the ship must still be visible to its workers after a
// planet save, whose settlement snapshot predates the order, is thawed.
func TestShipBuildOrderSurvivesLocationThaw(t *testing.T) {
	wm := shipTestWorld(t)
	order := queueShipBuildOrder(wm.shipLevel.MainSettlement)

	if err := wm.EnterStartingOrbit(); err != nil {
		t.Fatalf("thaw planet: %v", err)
	}
	if _, err := wm.EnterShip(); err != nil {
		t.Fatalf("enter ship: %v", err)
	}
	t.Cleanup(wm.shipLevel.teardown)

	got := assertShipSettlementIntact(t, wm)
	if !hasTask(got, order) {
		t.Errorf("build order lost from ship queue after thaw; queue has %d tasks", len(got.Tasks.GetTasks()))
	}
	colony := wm.currentColonyName()
	if s := settlement.Settlements[colony]; s == nil || s != wm.Current().MainSettlement {
		t.Errorf("thawed planet's MainSettlement %p is not the map's colony entry %p", wm.Current().MainSettlement, s)
	}
}

// reloadCampaign saves, drops all process state and loads the campaign back.
func reloadCampaign(t *testing.T, wm *WorldManager) *WorldManager {
	t.Helper()
	if err := wm.SaveCampaign(wm.Current()); err != nil {
		t.Fatalf("save campaign: %v", err)
	}
	settlement.Settlements = make(map[string]*settlement.Settlement) // a new process
	c, err := LoadCampaign(wm.Campaign.Name)
	if err != nil {
		t.Fatalf("load campaign: %v", err)
	}
	loaded := NewWorldManager(c)
	if err := loaded.EnterStartingOrbit(); err != nil {
		t.Fatalf("enter orbit: %v", err)
	}
	return loaded
}

// Orders queued before a save must come back on both the ship and the planet, in
// the form workers need: not in progress, with Data of the type handlers assert.
func TestQueuedOrdersSurviveCampaignSaveAndLoad(t *testing.T) {
	wm := shipTestWorld(t)
	if err := wm.EnterStartingOrbit(); err != nil {
		t.Fatalf("thaw planet: %v", err)
	}
	shipOrder := task_requests.BuildRequest{Type: "hull_wall", X: 3, Y: 3, Required: 10}
	wm.shipLevel.MainSettlement.Tasks.AddTask(&task.Task{X: 3, Y: 3, Action: task_requests.BuildAction, Data: shipOrder})
	planetOrder := task_requests.DigRequest{X: 2, Y: 2, Required: 5}
	wm.Current().MainSettlement.Tasks.AddTask(&task.Task{X: 2, Y: 2, Action: task_requests.DigAction, Data: planetOrder, InProgress: true})

	loaded := reloadCampaign(t, wm)

	got := assertShipSettlementIntact(t, loaded).Tasks.GetNextTask()
	if got == nil {
		t.Fatal("ship build order not restored, or not available to workers")
	}
	if data, ok := got.Data.(task_requests.BuildRequest); !ok || !reflect.DeepEqual(data, shipOrder) {
		t.Errorf("ship order Data = %#v, want %#v", got.Data, shipOrder)
	}
	colony := settlement.Settlements[loaded.currentColonyName()]
	if colony == nil || colony != loaded.Current().MainSettlement {
		t.Fatal("planet MainSettlement is not the map's colony entry")
	}
	got = colony.Tasks.GetNextTask()
	if got == nil {
		t.Fatal("planet dig order not restored, or not available to workers")
	}
	if data, ok := got.Data.(task_requests.DigRequest); !ok || data != planetOrder {
		t.Errorf("planet order Data = %#v, want %#v", got.Data, planetOrder)
	}
}

// Full campaign cycle: save, drop all process state, load, enter the ship. Orders
// queued afterwards must reach the settlement workers consult, including across
// a further thaw of the planet.
func TestShipSettlementSurvivesCampaignLoad(t *testing.T) {
	wm := shipTestWorld(t)
	if err := wm.EnterStartingOrbit(); err != nil {
		t.Fatalf("thaw planet: %v", err)
	}
	if err := wm.SaveCampaign(wm.Current()); err != nil {
		t.Fatalf("save campaign: %v", err)
	}

	settlement.Settlements = make(map[string]*settlement.Settlement) // a new process
	c, err := LoadCampaign(wm.Campaign.Name)
	if err != nil {
		t.Fatalf("load campaign: %v", err)
	}
	loaded := NewWorldManager(c)
	if err := loaded.EnterStartingOrbit(); err != nil {
		t.Fatalf("enter orbit: %v", err)
	}
	if _, err := loaded.EnterShip(); err != nil {
		t.Fatalf("enter ship: %v", err)
	}
	t.Cleanup(loaded.shipLevel.teardown)
	got := assertShipSettlementIntact(t, loaded)

	order := queueShipBuildOrder(loaded.shipLevel.MainSettlement)
	if !hasTask(got, order) {
		t.Error("order queued on the ship is not in the settlement workers read")
	}

	delete(loaded.loaded, "planet")
	loaded.currentID = ""
	if err := loaded.EnterStartingOrbit(); err != nil {
		t.Fatalf("re-thaw planet: %v", err)
	}
	got = assertShipSettlementIntact(t, loaded)
	if !hasTask(got, order) {
		t.Error("build order lost from ship queue after re-thawing the planet")
	}
}
