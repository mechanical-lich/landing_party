package systems

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

func stationEntity(lvl *world.Level, x, y int, comps ...ecs.Component) *ecs.Entity {
	e := &ecs.Entity{}
	e.AddComponent(&rlcomponents.PositionComponent{})
	e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent).SetPosition(x, y, 0)
	for _, c := range comps {
		e.AddComponent(c)
	}
	lvl.AddEntity(e)
	return e
}

// A powered light stays dark and a powered door locked until the grid's
// flag is set; then the light comes back and the door unlocks, once.
func TestStationPowerHoldsTheGridDark(t *testing.T) {
	lvl := world.NewLevel(10, 10, 1)
	light := stationEntity(lvl, 2, 2, &rlcomponents.LightComponent{Level: 110, Range: 6}, &components.PoweredComponent{Grid: "main", Level: 110, Range: 6})
	door := stationEntity(lvl, 4, 4, &rlcomponents.DoorComponent{}, &components.PoweredComponent{Grid: "main", Unlock: true})
	sys := &StationPowerSystem{}
	for i := 0; i < 3; i++ {
		_ = sys.UpdateEntity(lvl, light)
		_ = sys.UpdateEntity(lvl, door)
	}
	if lc := light.GetComponent(rlcomponents.Light).(*rlcomponents.LightComponent); lc.Level != 0 {
		t.Errorf("unpowered light at level %d", lc.Level)
	}
	if !door.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent).Locked {
		t.Error("unpowered door is unlocked")
	}
	lvl.Flags[PowerFlag("main")] = 1.0
	_ = sys.UpdateEntity(lvl, light)
	_ = sys.UpdateEntity(lvl, door)
	if lc := light.GetComponent(rlcomponents.Light).(*rlcomponents.LightComponent); lc.Level != 110 || lc.Range != 6 {
		t.Errorf("powered light at %d/%d", lc.Level, lc.Range)
	}
	if door.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent).Locked {
		t.Error("powered door still locked")
	}
	if light.HasComponent(components.Powered) || door.HasComponent(components.Powered) {
		t.Error("Powered not removed once the grid is up")
	}
}

// A key door opens only for a colonist beside it whose colony holds the key;
// it then belongs to the colony and flags its module.
func TestKeyDoorOpensOnlyForTheKey(t *testing.T) {
	lvl := world.NewLevel(20, 20, 1)
	lvl.Modules = []world.Module{{ID: "archive", Kind: "archive", X: 5, Y: 5, W: 7, H: 7}}
	door := stationEntity(lvl, 5, 8, &rlcomponents.DoorComponent{Locked: true, KeyId: "station_keycard"})
	inv := &rlcomponents.InventoryComponent{}
	colonist := stationEntity(lvl, 4, 8, &rlcomponents.DescriptionComponent{Name: "Ada", Faction: "colony"}, inv)
	_ = colonist
	sys := &KeyDoorSystem{Faction: "colony"}
	_ = sys.UpdateEntity(lvl, door)
	dc := door.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent)
	if !dc.Locked {
		t.Fatal("opened without the key")
	}
	inv.AddItem(&ecs.Entity{Blueprint: "tech_fragment"})
	_ = sys.UpdateEntity(lvl, door)
	if !dc.Locked {
		t.Fatal("opened for the wrong item")
	}
	inv.AddItem(&ecs.Entity{Blueprint: "station_keycard"})
	_ = sys.UpdateEntity(lvl, door)
	if dc.Locked || dc.OwnedBy != "colony" || lvl.Flags["door:archive"] != 1.0 {
		t.Errorf("with the key: locked %v owner %q flag %v", dc.Locked, dc.OwnedBy, lvl.Flags["door:archive"])
	}
}

// The key in a far-off colonist's bag does nothing while no colonist is
// beside the door.
func TestKeyDoorNeedsSomeoneBesideIt(t *testing.T) {
	lvl := world.NewLevel(20, 20, 1)
	door := stationEntity(lvl, 5, 8, &rlcomponents.DoorComponent{Locked: true, KeyId: "station_keycard"})
	inv := &rlcomponents.InventoryComponent{}
	inv.AddItem(&ecs.Entity{Blueprint: "station_keycard"})
	stationEntity(lvl, 15, 15, &rlcomponents.DescriptionComponent{Faction: "colony"}, inv)
	_ = (&KeyDoorSystem{Faction: "colony"}).UpdateEntity(lvl, door)
	if !door.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent).Locked {
		t.Error("opened with nobody beside it")
	}
}
