package game

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/world"
)

func b0SavedSettlements(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sf, err := unmarshalSaveFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	return b0Names(sf.WorldData)
}

func b0Names(d world.SaveData) []string {
	names := []string{}
	for name := range d.Settlements {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Freezing one of two loaded planets touches only its own settlements, and its
// save carries only those; thawing brings them back with their queue.
func TestFreezeAndThawLeaveOtherSettlementsAlone(t *testing.T) {
	c := b0Campaign(t, "a", "b")
	wm := NewWorldManager(c)
	a := b0Planet(t, locationColonyName(c, c.Locations["a"]))
	b := b0Planet(t, locationColonyName(c, c.Locations["b"]))
	nameA, nameB := a.MainSettlement.Name, b.MainSettlement.Name
	settlementB := b.MainSettlement
	settlementShip := wm.shipLevel.MainSettlement
	settlement.Settlements["raiders:a"] = &settlement.Settlement{Name: "raiders:a"}
	raidersB := &settlement.Settlement{Name: "raiders:b"}
	settlement.Settlements["raiders:b"] = raidersB

	orderA := b0QueueBuild(a, 3, 3)
	orderB := b0QueueBuild(b, 4, 4)
	orderShip := queueShipBuildOrder(settlementShip)

	if err := wm.Freeze(c.Locations["a"], a); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	for _, name := range []string{nameA, "raiders:a"} {
		if _, ok := settlement.Settlements[name]; ok {
			t.Errorf("%q still in the global map after freeze", name)
		}
	}
	assertOthers := func() {
		t.Helper()
		if settlement.Settlements[nameB] != settlementB || !hasTask(settlementB, orderB) {
			t.Error("the other planet's settlement or queue changed")
		}
		if settlement.Settlements["raiders:b"] != raidersB {
			t.Error("the other planet's faction settlement changed")
		}
		if settlement.Settlements[wm.shipSettlementName()] != settlementShip || !hasTask(settlementShip, orderShip) {
			t.Error("the ship's settlement or queue changed")
		}
	}
	assertOthers()

	saved := b0SavedSettlements(t, filepath.Join(campaignRoot(c.Name), c.Locations["a"].SaveFile))
	want := []string{nameA, "raiders:a"}
	sort.Strings(want)
	if len(saved) != 2 || saved[0] != want[0] || saved[1] != want[1] {
		t.Errorf("location save carries %v, want %v", saved, want)
	}

	thawed, err := wm.buildParked("a")
	if err != nil {
		t.Fatalf("thaw: %v", err)
	}
	if thawed.MainSettlement == nil || thawed.MainSettlement != settlement.Settlements[nameA] {
		t.Fatal("thawed planet did not adopt its settlement")
	}
	got := thawed.MainSettlement.Tasks.GetNextTask()
	if got == nil || got.X != orderA.X || got.Y != orderA.Y {
		t.Errorf("thawed queue's next task = %+v, want the one queued before the freeze", got)
	}
	assertOthers()
}

// Saving the campaign keeps playing, so unlike a freeze the live settlements stay
// registered. The ship's file holds only the ship's settlement.
func TestSaveCampaignKeepsSettlementsLive(t *testing.T) {
	wm := shipTestWorld(t)
	if err := wm.EnterStartingOrbit(); err != nil {
		t.Fatalf("thaw: %v", err)
	}
	colony := wm.Current().MainSettlement
	if err := wm.SaveCampaign(wm.Current()); err != nil {
		t.Fatalf("save: %v", err)
	}
	if settlement.Settlements[colony.Name] != colony {
		t.Error("saving dropped the live colony settlement")
	}
	shipSaved := b0SavedSettlements(t, wm.shipSaveFile())
	if len(shipSaved) != 1 || shipSaved[0] != wm.shipSettlementName() {
		t.Errorf("ship save carries %v", shipSaved)
	}
	locSaved := b0SavedSettlements(t, filepath.Join(campaignRoot(wm.Campaign.Name), wm.Campaign.CurrentLocation().SaveFile))
	if len(locSaved) != 1 || locSaved[0] != colony.Name {
		t.Errorf("location save carries %v", locSaved)
	}
}

// Saving while aboard must write the planet to the planet's file, not the ship.
func TestSavingFromTheShipKeepsThePlanetSave(t *testing.T) {
	wm := shipTestWorld(t)
	if err := wm.EnterStartingOrbit(); err != nil {
		t.Fatalf("thaw: %v", err)
	}
	if err := wm.SaveCampaign(wm.shipLevel); err != nil {
		t.Fatalf("save: %v", err)
	}
	locSaved := b0SavedSettlements(t, filepath.Join(campaignRoot(wm.Campaign.Name), wm.Campaign.CurrentLocation().SaveFile))
	if len(locSaved) != 1 || locSaved[0] != wm.Current().MainSettlement.Name {
		t.Errorf("location save carries %v, want only the planet's colony", locSaved)
	}
}
