package campaign

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

func mirrorDoctrine(t *testing.T) *Doctrine {
	t.Helper()
	d := LoadFactionData().Doctrine("mirror")
	if d == nil || d.Settlement == nil {
		t.Fatal("no mirror doctrine with the colony's settlement")
	}
	return d
}

// shipTestCampaign is a mirror ship landed on "home" with a free site
// "near" 20 units away (5 days at speed 4) and one "far" out of range.
func shipTestCampaign(t *testing.T) (*Campaign, *Doctrine, *Fleet) {
	t.Helper()
	c := factionTestCampaign(t, map[string]string{"home": "earth_like", "near": "earth_like", "far": "earth_like"})
	home := c.Locations["home"]
	c.Locations["near"].X, c.Locations["near"].Y = home.X+20, home.Y
	d := mirrorDoctrine(t)
	f := c.newShip("mirror", d, home, 4, true)
	return c, d, f
}

// Rival factions 1 is still one hive; from 2 the mirror joins the bag, as a
// ship landed on its site with the doctrine's kit, never a planet presence.
func TestSeedingMirrorFromTwoFactions(t *testing.T) {
	useRepoData(t)
	for seed := int64(1); seed <= 6; seed++ {
		one, err := GenerateCampaignOpts("One", seed, GenOptions{Factions: 1})
		if err != nil {
			t.Fatal(err)
		}
		if len(one.Fleets) != 0 {
			t.Errorf("seed %d, 1 faction: fleets %+v", seed, one.Fleets)
		}
		two, err := GenerateCampaignOpts("Two", seed, GenOptions{Factions: 2})
		if err != nil {
			t.Fatal(err)
		}
		ships, hives := 0, 0
		for _, f := range two.Fleets {
			if f.Kind == FleetShip && f.Landed() && f.Presence.Doctrine == RivalColonyDoctrine {
				ships++
				if f.Presence.Stockpile["metal_ore"] == 0 || f.Presence.Population != 4 {
					t.Errorf("seed %d: ship presence %+v lacks the kit or crew", seed, f.Presence)
				}
				if !two.FactionSiteOK(two.Locations[f.LandedID], true) {
					t.Errorf("seed %d: ship landed on %s, not a seeding site", seed, f.LandedID)
				}
			}
		}
		for _, loc := range two.Locations {
			for _, p := range loc.Factions {
				if p.Doctrine == RivalColonyDoctrine {
					t.Errorf("seed %d: a mirror presence on the planet %s", seed, loc.ID)
				}
				if p.Doctrine == "hive" {
					hives++
				}
			}
		}
		if ships != 1 || hives != 1 {
			t.Errorf("seed %d, 2 factions: %d ships, %d hives; want one each", seed, ships, hives)
		}
	}
}

// A landed ship grows and researches off a loaded site, lifts off once its
// stay is over for a free site in range (everyone and the hold go with it),
// waits in orbit while that site is loaded, and lands when it is not, to be
// built fresh there.
func TestShipStaysLiftsOffAndLands(t *testing.T) {
	c, d, _ := shipTestCampaign(t)
	stay := d.Mobility.StayDays
	c.StarDate = 5
	c.StepFactions(nil)
	f := &c.Fleets[0]
	if !f.Landed() || f.LandedID != "home" {
		t.Fatalf("ship left early: %+v", f)
	}
	f.Presence.Materialized = true
	c.StarDate = history.StarDate(stay)
	c.StepFactions(nil)
	f = &c.Fleets[0]
	if f.Landed() || f.ToID != "near" || f.FromID != "home" {
		t.Fatalf("after its stay the ship should fly home -> near: %+v", f)
	}
	if f.Presence.Stockpile["metal_ore"] == 0 {
		t.Error("the hold stayed behind")
	}
	if f.Presence.Population < 4 {
		t.Errorf("crew %d, want at least the 4 it landed with", f.Presence.Population)
	}
	if len(c.LandedFleets("home")) != 0 {
		t.Error("still listed as landed at home")
	}
	// The target is loaded on its arrival day: it waits in orbit.
	arrive := f.ArriveDay
	c.StarDate = arrive
	c.StepFactions(func(id string) bool { return id == "near" })
	if f := &c.Fleets[0]; f.Landed() {
		t.Fatal("landed on a loaded site")
	}
	c.StarDate = arrive + 1
	c.StepFactions(nil)
	f = &c.Fleets[0]
	if !f.Landed() || f.LandedID != "near" || f.Presence.Materialized {
		t.Fatalf("should be landed, unbuilt, at near: %+v", f)
	}
	if x, y := f.Pos(c.StarTime()); x != c.Locations["near"].X || y != c.Locations["near"].Y {
		t.Errorf("landed ship drawn at %v,%v, not at its site", x, y)
	}
	// Legends: landed, departed, landed.
	var types []string
	for _, ev := range c.History.Events {
		if strings.HasPrefix(ev.Type, "faction_") {
			types = append(types, ev.Type)
		}
	}
	if strings.Join(types, ",") != "faction_landed,faction_departed,faction_landed" {
		t.Errorf("events %v", types)
	}
	// The record survives a save.
	b, err := json.Marshal(c.Fleets)
	if err != nil {
		t.Fatal(err)
	}
	var back []Fleet
	if err := json.Unmarshal(b, &back); err != nil || !back[0].Landed() || back[0].LandedID != "near" {
		t.Errorf("round trip: %+v %v", back, err)
	}
}

// A ship with nowhere free in range stays where it is.
func TestShipWithNowhereToGoStays(t *testing.T) {
	c, d, _ := shipTestCampaign(t)
	c.Locations["near"].Factions = []FactionPresence{{ID: "fpx", Faction: "hive", Doctrine: "hive", Population: 3}}
	c.StarDate = history.StarDate(d.Mobility.StayDays + 5)
	c.StepFactions(nil)
	if f := &c.Fleets[0]; !f.Landed() || f.LandedID != "home" {
		t.Fatalf("flew with nowhere to go: %+v", f)
	}
}

// Stance: the mirror starts neutral (its doctrine), the hive hostile (no
// diplomacy); turning hostile is recorded once, as a Legends event.
func TestStance(t *testing.T) {
	c, _, _ := shipTestCampaign(t)
	if c.Stance("mirror") != StanceNeutral || c.Stance("hive") != StanceHostile {
		t.Fatalf("stances %s / %s", c.Stance("mirror"), c.Stance("hive"))
	}
	if got := c.HeldBy(c.Locations["home"]); len(got) != 1 || got[0] != "mirror" {
		t.Errorf("home held by %v", got)
	}
	if !c.SetStance("mirror", StanceHostile, c.Locations["home"]) || c.SetStance("mirror", StanceHostile, c.Locations["home"]) {
		t.Error("SetStance should change once")
	}
	if c.Stance("mirror") != StanceHostile {
		t.Error("not hostile after SetStance")
	}
	found := false
	for _, ev := range c.History.Events {
		found = found || ev.Type == "faction_hostile"
	}
	if !found {
		t.Error("no faction_hostile event")
	}
}

// A mirror ship that knows a tech from its live colony (merged in on
// reconcile, off the walk's order) still researches: Tier 1 takes the first
// entry it does not know, not the entry at its tech count (review finding 8).
func TestShipResearchAfterLiveTechs(t *testing.T) {
	c, d, f := shipTestCampaign(t)
	if len(d.Research) < 3 || d.ResearchPerDay <= 0 {
		t.Fatalf("fixture: mirror research %v at %v a day", d.Research, d.ResearchPerDay)
	}
	// A live tech off the walk's front: the count points past what it knows.
	f.Presence.KnownTechs = []string{d.Research[1].ID}
	f.Presence.ResearchPts = 0
	prev := d.Mobility.StayDays
	d.Mobility.StayDays = 100000 // it stays put (the shared doctrine: put back)
	t.Cleanup(func() { d.Mobility.StayDays = prev })
	cost := 0.0
	for _, r := range d.Research[:3] {
		cost += r.Cost
	}
	days := int(cost/d.ResearchPerDay) + 5
	for day := 0; day < days; day++ {
		c.StarDate++
		c.StepFactions(nil)
	}
	f = c.FleetByPresence(f.Presence.ID)
	if !hasStr(f.Presence.KnownTechs, d.Research[0].ID) || !hasStr(f.Presence.KnownTechs, d.Research[2].ID) {
		t.Errorf("after %d days it knows %v, want %s and %s too", days, f.Presence.KnownTechs, d.Research[0].ID, d.Research[2].ID)
	}
}

// Asking whether a table-flown ship would leave changes nothing: the view
// once filled an empty, never-flown tank as it read it (secondary review
// note: fleetView mutated p.Fuel). The full tank applies only to a real
// move.
func TestFleetViewDoesNotWriteFuel(t *testing.T) {
	c, d, f := shipTestCampaign(t)
	if d.Expedition == nil || d.Expedition.FleetFuel <= 0 {
		t.Fatal("fixture: the mirror flies by the table")
	}
	f.Presence.Fuel, f.LastMove = 0, ""
	c.ShipLeaving(f, c.FactionDay+1000)
	c.fleetView(d, f, c.FactionDay, nil)
	if f.Presence.Fuel != 0 {
		t.Errorf("reading the view put %d fuel in the tank", f.Presence.Fuel)
	}
	if v := c.fleetView(d, f, c.FactionDay, nil); v.Fuel != d.Expedition.FleetFuel {
		t.Errorf("a never-flown ship plans with %d fuel, want a full %d", v.Fuel, d.Expedition.FleetFuel)
	}
}
