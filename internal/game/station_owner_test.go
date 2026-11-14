package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/settlement"
)

// A rival's station opens nothing of the player's: a click on the mirror's
// workbench (inside its camp's radius) or on a hive larva (its Kin names the
// laying settlement) gives no crafting modal, while the player's own bench
// still does (phase F's station owner check; before it, any station opened
// the player's modal).
func TestRivalStationOpensNoPlayerModal(t *testing.T) {
	_, ms, loc := mirrorOnStart(t, 1, 4)
	neutralMirror(t, ms, loc)
	st := settlement.Settlements["mirror:"+loc.ID]
	if st == nil {
		t.Fatal("no mirror settlement")
	}
	rival, err := factory.Create("basic_workbench", st.CenterX+1, st.CenterY, st.CenterZ)
	if err != nil {
		t.Fatal(err)
	}
	ms.level.AddEntity(rival)
	if ms.stationOwner(rival) != st.Name || !ms.foreignStation(rival) {
		t.Fatalf("rival bench owner %q, want %q", ms.stationOwner(rival), st.Name)
	}
	if err := ms.HandleEvent(gui.StationClickedEvent{Station: rival}); err != nil {
		t.Fatal(err)
	}
	if ids := ms.guiManager.CraftingRecipeIDs(); len(ids) != 0 {
		t.Fatalf("rival bench opened the player's modal: %v", ids)
	}

	larva, err := factory.Create("hive_larva", 2, 2, ms.MainSettlement.CenterZ)
	if err != nil {
		t.Fatal(err)
	}
	if k := components.KinOf(larva); k != nil {
		k.Settlement = "hive:" + loc.ID
	} else {
		larva.AddComponent(&components.KinComponent{Settlement: "hive:" + loc.ID})
	}
	if !ms.foreignStation(larva) {
		t.Fatal("a rival hive's larva reads as the player's")
	}

	ms0 := ms.MainSettlement
	own, err := factory.Create("basic_workbench", ms0.CenterX, ms0.CenterY, ms0.CenterZ)
	if err != nil {
		t.Fatal(err)
	}
	ms.level.AddEntity(own)
	if ms.foreignStation(own) {
		t.Fatalf("the player's bench reads as %q's", ms.stationOwner(own))
	}
	if err := ms.HandleEvent(gui.StationClickedEvent{Station: own}); err != nil {
		t.Fatal(err)
	}
	if ids := ms.guiManager.CraftingRecipeIDs(); len(ids) == 0 {
		t.Fatal("the player's own bench opened no modal")
	}
}
