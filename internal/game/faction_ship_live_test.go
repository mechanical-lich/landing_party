package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/settlement"
)

func eventsOf(c *campaign.Campaign, typ string) int {
	n := 0
	if c.History == nil {
		return 0
	}
	for _, ev := range c.History.Events {
		if ev.Type == typ {
			n++
		}
	}
	return n
}

// A ship bound for the planet in orbit comes down while the player watches
// (no more waiting in orbit): its camp and crew appear on the live level,
// with an alert and a Legends line. When its stay is over it gathers its
// crew and hold and lifts off, live, for another site: the crew leave the
// level, the hold leaves with it, the settlement is gone, and the fleet is
// in flight with everyone aboard.
func TestShipLandsAndLiftsOffLive(t *testing.T) {
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Live ship", 6, false)
	if err != nil {
		t.Fatal(err)
	}
	syncFactionHostility(c)
	loc := c.Locations[campaign.StartLocationID]
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
	var from *campaign.Location
	for _, l := range c.Locations {
		if l.ID != loc.ID && c.FactionSiteOK(l, false) && (from == nil || l.ID < from.ID) {
			from = l
		}
	}
	day := c.StarDate
	c.FactionSeq++
	c.FleetSeq++
	c.Fleets = append(c.Fleets, campaign.Fleet{
		ID: "fl900", Kind: campaign.FleetShip, FromID: from.ID, ToID: loc.ID,
		FromX: from.X, FromY: from.Y, ToX: loc.X, ToY: loc.Y, LaunchDay: day - 2, ArriveDay: day + 1,
		Presence: campaign.FactionPresence{ID: "fp900", Faction: "mirror", Doctrine: "mirror", Population: 3,
			Stockpile: map[string]int{"metal_ore": 30, "crystal": 15}, Seed: 4242, LastShotDay: campaign.NeverFired},
	})
	landedEvents := eventsOf(c, "faction_landed")
	name := "mirror:" + loc.ID
	stepLevel(ms, 2*750, nil)
	f := c.FleetByPresence("fp900")
	if f == nil || !f.Landed() || f.LandedID != loc.ID {
		t.Fatalf("the ship did not land on the loaded site: %+v", f)
	}
	if n := len(settlementWorkers(ms.level, name)); n != 3 {
		t.Fatalf("%d crew on the live level after the landing, want 3", n)
	}
	if settlement.Settlements[name] == nil {
		t.Fatal("no settlement for the landed ship")
	}
	if eventsOf(c, "faction_landed") != landedEvents+1 {
		t.Error("no faction_landed Legends event for the live landing")
	}
	if !f.Known {
		t.Error("a ship that came down on the planet in orbit is not seen")
	}

	// Its stay is over: it lifts off live.
	f.LandedDay -= history.StarDate(campaign.LoadFactionData().Doctrine("mirror").Mobility.StayDays + 1)
	departed := eventsOf(c, "faction_departed")
	stepLevel(ms, 2*750, nil)
	f = c.FleetByPresence("fp900")
	if f == nil || f.Landed() {
		t.Fatalf("the ship did not lift off: %+v", f)
	}
	if f.ToID == loc.ID {
		t.Error("it lifted off for the site it left")
	}
	if n := len(settlementWorkers(ms.level, name)); n != 0 {
		t.Errorf("%d crew left behind on the level", n)
	}
	if settlement.Settlements[name] != nil {
		t.Error("the settlement stayed in the global map")
	}
	if f.Presence.Population != 3 || f.Presence.Stockpile["metal_ore"]+f.Presence.Stockpile["crystal"] == 0 {
		t.Errorf("the crew and hold did not go aboard: %+v", f.Presence)
	}
	if eventsOf(c, "faction_departed") != departed+1 {
		t.Error("no faction_departed Legends event for the live lift-off")
	}
}

// A landed ship whose whole crew dies on its loaded level is wiped at once
// (phase E, D's gap): faction_wiped in Legends, the fleet gone, the
// settlement out of the global map; the camp stays as it is.
func TestShipCrewWipedLive(t *testing.T) {
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Live wipe", 6, false)
	if err != nil {
		t.Fatal(err)
	}
	syncFactionHostility(c)
	loc := c.Locations[campaign.StartLocationID]
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
	day := c.StarDate
	c.FactionSeq++
	c.FleetSeq++
	c.Fleets = append(c.Fleets, campaign.Fleet{
		ID: "fl901", Kind: campaign.FleetShip, FromID: loc.ID, ToID: loc.ID,
		FromX: loc.X, FromY: loc.Y, ToX: loc.X, ToY: loc.Y, LaunchDay: day - 2, ArriveDay: day + 1,
		Presence: campaign.FactionPresence{ID: "fp901", Faction: "greyish", Doctrine: "greyish", Population: 3,
			Seed: 4343, LastShotDay: campaign.NeverFired},
	})
	name := "greyish:" + loc.ID
	stepLevel(ms, 2*750, nil)
	if f := c.FleetByPresence("fp901"); f == nil || !f.Landed() {
		t.Fatalf("the ship did not land: %+v", f)
	}
	crew := settlementWorkers(ms.level, name)
	if len(crew) == 0 {
		t.Fatal("no crew landed")
	}
	for _, e := range crew {
		ms.level.RemoveEntity(e)
	}
	wiped := eventsOf(c, "faction_wiped")
	wm.stepLiveShips()
	if c.FleetByPresence("fp901") != nil {
		t.Fatal("a ship with its whole crew dead is still in the campaign")
	}
	if eventsOf(c, "faction_wiped") != wiped+1 {
		t.Error("no faction_wiped Legends line")
	}
	if settlement.Settlements[name] != nil {
		t.Error("the dead ship's settlement stayed in the global map")
	}
}

// A landed ship whose stay is over but that cannot leave (no fuel for any
// site) stays as it is: its crew and camp are not taken off and rebuilt on
// top of themselves every day (review finding 10).
func TestStuckShipStaysLive(t *testing.T) {
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Stuck ship", 6, false)
	if err != nil {
		t.Fatal(err)
	}
	syncFactionHostility(c)
	loc := c.Locations[campaign.StartLocationID]
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
	var from *campaign.Location
	for _, l := range c.Locations {
		if l.ID != loc.ID && c.FactionSiteOK(l, false) && (from == nil || l.ID < from.ID) {
			from = l
		}
	}
	day := c.StarDate
	c.FactionSeq++
	c.FleetSeq++
	c.Fleets = append(c.Fleets, campaign.Fleet{
		ID: "fl901", Kind: campaign.FleetShip, FromID: from.ID, ToID: loc.ID,
		FromX: from.X, FromY: from.Y, ToX: loc.X, ToY: loc.Y, LaunchDay: day - 2, ArriveDay: day + 1,
		Presence: campaign.FactionPresence{ID: "fp901", Faction: "mirror", Doctrine: "mirror", Population: 3,
			Stockpile: map[string]int{"metal_ore": 30, "crystal": 15}, Seed: 4243, LastShotDay: campaign.NeverFired},
	})
	name := "mirror:" + loc.ID
	stepLevel(ms, 2*750, nil)
	f := c.FleetByPresence("fp901")
	if f == nil || !f.Landed() {
		t.Fatalf("the ship did not land: %+v", f)
	}
	crew := settlementWorkers(ms.level, name)
	before := len(ms.level.Entities) + len(ms.level.StaticEntities)
	// Stay over, no fuel to reach anywhere: it cannot leave.
	f.LandedDay -= history.StarDate(campaign.LoadFactionData().Doctrine("mirror").Mobility.StayDays + 1)
	f.Presence.Fuel, f.LastMove = 1, "stay"
	if !c.ShipLeaving(f, c.FactionDay) {
		t.Fatal("fixture: the ship's stay is not over")
	}
	stepLevel(ms, 3*750, nil)
	f = c.FleetByPresence("fp901")
	if f == nil || !f.Landed() || f.LandedID != loc.ID {
		t.Fatalf("the stuck ship left: %+v", f)
	}
	now := settlementWorkers(ms.level, name)
	same := 0
	for _, e := range now {
		for _, o := range crew {
			if e == o {
				same++
			}
		}
	}
	if same != len(crew) || len(now) != len(crew) {
		t.Errorf("crew taken off and respawned: %d of %d bodies the same (%d now)", same, len(crew), len(now))
	}
	if after := len(ms.level.Entities) + len(ms.level.StaticEntities); after > before+len(crew) {
		t.Errorf("the camp was rebuilt on top of itself: %d entities, then %d", before, after)
	}
}
