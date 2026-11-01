package game

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
)

// beamWorld is a ship in orbit of a loaded planet with one colony locker of the
// given capacity (0 = unlimited) already holding `preload` rations.
func beamWorld(t *testing.T, capacity, preload int) (*WorldManager, *components.StorageComponent) {
	t.Helper()
	wm := shipTestWorld(t)
	if err := wm.EnterStartingOrbit(); err != nil {
		t.Fatalf("enter orbit: %v", err)
	}
	t.Cleanup(wm.Current().teardown)
	locker, err := factory.Create("storage_locker", 2, 2, 0)
	if err != nil {
		t.Fatalf("create locker: %v", err)
	}
	sc := locker.GetComponent(components.Storage).(*components.StorageComponent)
	sc.OwnedBy = wm.currentColonyName()
	sc.Capacity = capacity
	for i := 0; i < preload; i++ {
		r, err := factory.Create("ration_pack", 0, 0, 0)
		if err != nil {
			t.Fatalf("create ration: %v", err)
		}
		if !sc.AddItem(r) {
			t.Fatal("preload rejected")
		}
	}
	wm.Current().level.AddEntity(locker)
	return wm, sc
}

func TestShipHold_NonStackablesAddOnePerUnit(t *testing.T) {
	wm := shipTestWorld(t)
	wm.addToShipHold("ration_pack", 5)
	if got := wm.shipLocker().CountResource("ration_pack"); got != 5 {
		t.Fatalf("hold rations = %d, want 5", got)
	}
}

func TestBeamDown_NonStackablesAllArrive(t *testing.T) {
	wm, sc := beamWorld(t, 0, 0)
	wm.addToShipHold("ration_pack", 5)
	if err := wm.BeamResourceDown("ration_pack", 5); err != nil {
		t.Fatalf("beam: %v", err)
	}
	if got := sc.CountResource("ration_pack"); got != 5 {
		t.Errorf("site rations = %d, want 5", got)
	}
	if got := wm.shipLocker().CountResource("ration_pack"); got != 0 {
		t.Errorf("hold rations = %d, want 0", got)
	}
}

func TestBeamDown_PartialRoomDebitsOnlyWhatLanded(t *testing.T) {
	wm, sc := beamWorld(t, 3, 1) // room for two more
	wm.addToShipHold("ration_pack", 5)
	err := wm.BeamResourceDown("ration_pack", 5)
	if err == nil || !strings.Contains(err.Error(), "2 of 5") {
		t.Fatalf("want a '2 of 5' shortfall error, got %v", err)
	}
	if got := sc.CountResource("ration_pack"); got != 3 {
		t.Errorf("site rations = %d, want 3", got)
	}
	if got := wm.shipLocker().CountResource("ration_pack"); got != 3 {
		t.Errorf("hold rations = %d, want 3 (5 minus the 2 that landed)", got)
	}
}

func TestBeamDown_StackPartialFitDebitsOnlyWhatLanded(t *testing.T) {
	wm, sc := beamWorld(t, 1, 0)
	ore, err := factory.Create("metal_ore", 0, 0, 0)
	if err != nil {
		t.Fatalf("create ore: %v", err)
	}
	// Fill the only slot to ten short of a full stack, so a beam of 30 can top
	// it up by 10 and has nowhere to put the other 20.
	mc := ore.GetComponent(components.Material).(*components.MaterialComponent)
	full := mc.MaxStack
	mc.Quantity = full - 10
	if !sc.AddItem(ore) {
		t.Fatal("ore preload rejected")
	}
	wm.addToShipHold("metal_ore", 30)

	err = wm.BeamResourceDown("metal_ore", 30)
	if err == nil || !strings.Contains(err.Error(), "10 of 30") {
		t.Fatalf("want a '10 of 30' shortfall error, got %v", err)
	}
	if got := sc.CountResource("metal_ore"); got != full {
		t.Errorf("site ore = %d, want %d (stack topped up)", got, full)
	}
	if got := wm.shipLocker().CountResource("metal_ore"); got != 20 {
		t.Errorf("hold ore = %d, want 20", got)
	}
}
