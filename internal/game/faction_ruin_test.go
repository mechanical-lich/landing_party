package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/mlge/ecs"
)

// A ship lifting off live leaves its camp as a ruin at once: one ruin
// record, stamped, its footprint among the level's stamps, the departure's
// Legends event with an origin at the camp and a datapad there that holds
// it, and nothing left in the camp's storage.
func TestRuinLeftOnLiveDeparture(t *testing.T) {
	_, ms, loc, f := greyOnStart(t, 2, 3)
	c := ms.campaign
	name := "greyish:" + loc.ID
	stepLevel(ms, 300, nil)
	stamps := len(ms.level.StampFootprints)
	f.LandedDay -= history.StarDate(campaign.LoadFactionData().Doctrine("greyish").Mobility.StayDays + 1)
	stepLevel(ms, 2*750, nil)
	f = c.FleetByPresence("fpg")
	if f == nil || f.Landed() {
		t.Fatalf("the ship did not lift off: %+v", f)
	}
	if len(loc.Ruins) != 1 || !loc.Ruins[0].Stamped || loc.Ruins[0].EventID == "" {
		t.Fatalf("ruins %+v", loc.Ruins)
	}
	r := loc.Ruins[0]
	if len(ms.level.StampFootprints) <= stamps {
		t.Error("the ruin is not among the level's stamps")
	}
	if _, _, _, ok := generation.Origin(ms.level, r.EventID); !ok {
		t.Error("the departure event has no origin on the level")
	}
	if pads := ruinDatapads(ms.level, r.EventID); len(pads) != 1 {
		t.Errorf("%d datapads hold the departure, want 1", len(pads))
	}
	for _, e := range ms.level.Entities {
		if e != nil && e.HasComponent(components.Storage) {
			if sc := e.GetComponent(components.Storage).(*components.StorageComponent); sc.OwnedBy == name && len(sc.Items) > 0 {
				t.Errorf("the camp's storage still holds %d items", len(sc.Items))
			}
		}
	}
	if len(settlementWorkers(ms.level, name)) != 0 {
		t.Error("crew left in the ruin")
	}
}

// A ship that landed and left while its site was frozen (never built
// there) leaves a ruin the level builds on its next build: the landing
// scar (phase E4: no hull, no modules), with the datapad.
func TestRuinBuiltAfterAFrozenDeparture(t *testing.T) {
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Ruin", 4, false)
	if err != nil {
		t.Fatal(err)
	}
	var site *campaign.Location
	for _, l := range c.Locations {
		if l.ID != campaign.StartLocationID && c.FactionSiteOK(l, false) && l.SaveFile == "" && (site == nil || l.ID < site.ID) {
			site = l
		}
	}
	c.FactionSeq++
	c.FleetSeq++
	c.Fleets = append(c.Fleets, campaign.Fleet{
		ID: "flr", Kind: campaign.FleetShip, FromID: site.ID, ToID: site.ID,
		FromX: site.X, FromY: site.Y, ToX: site.X, ToY: site.Y, LandedID: site.ID, LandedDay: c.StarDate,
		Presence: campaign.FactionPresence{ID: "fpr", Faction: "greyish", Doctrine: "greyish", Population: 3,
			Seed: 5150, LastShotDay: campaign.NeverFired},
	})
	stay := campaign.LoadFactionData().Doctrine("greyish").Mobility.StayDays
	c.StarDate += history.StarDate(stay + 2)
	c.StepFactions(nil)
	if f := c.FleetByPresence("fpr"); f == nil || f.Landed() {
		t.Fatalf("the ship did not leave: %+v", f)
	}
	if len(site.Ruins) != 1 || site.Ruins[0].Stamped || site.Ruins[0].CampBuilt {
		t.Fatalf("ruins before the build %+v", site.Ruins)
	}
	ms, err := wm.buildParked(site.ID)
	if err != nil {
		t.Fatal(err)
	}
	r := site.Ruins[0]
	if !r.Stamped {
		t.Fatal("the ruin was not stamped on the build")
	}
	if pads := ruinDatapads(ms.level, r.EventID); len(pads) != 1 {
		t.Errorf("%d datapads hold the departure, want 1", len(pads))
	}
	x, y, z, ok := generation.Origin(ms.level, r.EventID)
	if !ok {
		t.Fatal("no origin for the departure")
	}
	// Phase E4: the hull went with the ship; the ruin is a landing scar.
	hull, scar := 0, 0
	for dy := -8; dy <= 8; dy++ {
		for dx := -8; dx <= 8; dx++ {
			if tileMiddleIs(ms, x+dx, y+dy, z, "hull_wall") {
				hull++
			}
			if floorNameAt(ms.level, x+dx, y+dy, z) == greyScarFloor {
				scar++
			}
		}
	}
	if hull != 0 || scar < 100 {
		t.Errorf("the ruin: %d hull wall tiles, %d scorched: want the landing scar, no hull", hull, scar)
	}
	for _, list := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range list {
			if e != nil && (e.Blueprint == "grey_vat" || e.Blueprint == "grey_console") {
				t.Errorf("a %s was left at the scar: modules go aboard", e.Blueprint)
			}
		}
	}
	again, err := wm.buildParked(site.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = again
	if n := len(site.Ruins); n != 1 || !site.Ruins[0].Stamped {
		t.Errorf("a rebuild changed the ruins: %+v", site.Ruins)
	}
}
