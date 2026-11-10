package game

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// greyOnStart lands a greyish scout of pop on the start system of a fresh
// expedition and enters it with one colonist beamed down far off.
func greyOnStart(t *testing.T, seed int64, pop int) (*WorldManager, *MainState, *campaign.Location, *campaign.Fleet) {
	t.Helper()
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Greys", seed, false)
	if err != nil {
		t.Fatal(err)
	}
	syncFactionHostility(c)
	t.Cleanup(components.ResetFactionHostility)
	loc := c.Locations[campaign.StartLocationID]
	c.FactionSeq++
	c.FleetSeq++
	c.Fleets = append(c.Fleets, campaign.Fleet{
		ID: "flg", Kind: campaign.FleetShip, FromID: loc.ID, ToID: loc.ID,
		FromX: loc.X, FromY: loc.Y, ToX: loc.X, ToY: loc.Y, LandedID: loc.ID,
		Presence: campaign.FactionPresence{ID: "fpg", Faction: "greyish", Doctrine: "greyish", Population: pop,
			Stockpile: map[string]int{"metal_ore": 12, "crystal": 6}, Seed: seed*17 + 3, LastShotDay: campaign.NeverFired},
	})
	if err := wm.Travel(loc.ID); err != nil {
		t.Fatal(err)
	}
	if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
		t.Fatal(err)
	}
	ms, err := wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	return wm, ms, loc, c.FleetByPresence("fpg")
}

// A landed greyish scout is its saucer on the level: the hull stamped, the
// hold in a locker its settlement owns, a bio-printer, and grey crew working
// as the settlement's workers (never the player's, hostile from the start).
// The director prints crew up to the scout's crew cap, the type most under
// its share first, and mines into the hold.
func TestGreyishCampOnTheLevel(t *testing.T) {
	_, ms, loc, f := greyOnStart(t, 1, 2)
	name := "greyish:" + loc.ID
	st := settlement.Settlements[name]
	if st == nil {
		t.Fatal("no greyish settlement")
	}
	ws := settlementWorkers(ms.level, name)
	if len(ws) != 2 {
		t.Fatalf("%d grey crew, want 2", len(ws))
	}
	for _, e := range ws {
		if !strings.HasSuffix(e.Blueprint, "_crew") || e.HasComponent(components.ScriptedAI) {
			t.Errorf("crew %s should be a greyish crew blueprint with no scripted AI", e.Blueprint)
		}
		if components.IsColonyWorker(e, ms.colonyName()) || !components.IsAttackTarget(e) {
			t.Error("a grey is the player's, or not hostile")
		}
	}
	printer, locker := false, false
	for _, e := range append(append(ms.level.Entities[:0:0], ms.level.Entities...), ms.level.StaticEntities...) {
		pc, ok := entityPos(e)
		if !ok || absI(pc.GetX()-st.CenterX) > 12 || absI(pc.GetY()-st.CenterY) > 12 {
			continue
		}
		if e.Blueprint == "bio_printer" {
			printer = true
		}
		if e.HasComponent(components.Storage) && e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy == name {
			locker = true
		}
	}
	if !printer || !locker {
		t.Fatalf("camp: printer %v, owned storage %v", printer, locker)
	}
	hull := 0
	for dy := -8; dy <= 8; dy++ {
		for dx := -8; dx <= 8; dx++ {
			if tileMiddleIs(ms, st.CenterX+dx, st.CenterY+dy, st.CenterZ, "hull_wall") {
				hull++
			}
		}
	}
	if hull < 20 {
		t.Errorf("only %d hull wall tiles: no saucer stamped", hull)
	}

	stepLevel(ms, 3000, nil)
	dir := ms.factionDirectors[name]
	grew := 0
	for k, v := range dir.Ordered {
		if strings.HasPrefix(k, "grow:") {
			grew += v
		}
	}
	d := campaign.LoadFactionData().Doctrine("greyish")
	if grew == 0 {
		t.Fatalf("the scout never printed crew: %v", dir.Log)
	}
	if n := len(settlementWorkers(ms.level, name)); n != d.CrewCap(f.ShipTier) {
		t.Errorf("%d crew after printing, want the scout's cap %d (%v)", n, d.CrewCap(f.ShipTier), dir.Log)
	}
	if dir.Ordered["mine:metal_ore"]+dir.Ordered["mine:crystal"] == 0 {
		t.Errorf("never mined into the hold: %v", dir.Log)
	}
	t.Logf("orders %v", dir.Ordered)
}

func tileMiddleIs(ms *MainState, x, y, z int, typ string) bool {
	ti := ms.level.GetTileAt(x, y, z)
	if ti == nil {
		return false
	}
	tl := ti.(*world.Tile)
	return !tl.Middle.IsEmpty() && world.TileDefinitions[tl.Middle.Type].Name == typ
}
