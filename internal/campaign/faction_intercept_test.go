package campaign

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// interceptCampaign: the player at "p1" (0,0) bound for "p2" (40,0), a jump
// of 8 days; a greyish ship of tier flying across the line, from (20,-20)
// to (20,20) over the same 8 days, so they meet at (20,0) on day 4.
func interceptCampaign(t *testing.T, seed int64, tier int) (*Campaign, *Fleet) {
	t.Helper()
	c := factionTestCampaign(t, map[string]string{"p1": "earth_like", "p2": "earth_like", "n": "earth_like", "s": "earth_like"})
	c.Seed = seed
	set := func(id string, x, y float64) { c.Locations[id].X, c.Locations[id].Y = x, y }
	set("p1", 0, 0)
	set("p2", 40, 0)
	set("n", 20, -20)
	set("s", 20, 20)
	c.CurrentLocationID = "p1"
	d := greyDoctrine(t)
	f := c.newShip("greyish", d, c.Locations["n"], 3, false)
	f.Presence.Seed = seed*977 + 13
	f.ShipTier = tier
	c.liftOff(d, f, c.Locations["s"], 0)
	f.ArriveDay = 8
	return c, &c.Fleets[len(c.Fleets)-1]
}

func TestInterceptionCrossingAndOrbit(t *testing.T) {
	c, f := interceptCampaign(t, 1, 0)
	e := c.DetectInterception("p1", "p2", 0, 8)
	if e == nil || e.FleetID != f.ID || e.At != "crossing" {
		t.Fatalf("the jump across the ship's path met %+v", e)
	}
	if !f.Known {
		t.Error("a ship met face to face is not known")
	}
	if c.DetectInterception("p1", "n", 0, 8) != nil {
		t.Error("a jump away from the ship's path met it")
	}
	// A mothership in orbit of the destination is met on arrival.
	d := greyDoctrine(t)
	ms := parkMothership(c, "greyish", d, map[string]bool{"p1": true, "n": true, "s": true})
	if ms == nil {
		t.Fatal("no mothership")
	}
	e = c.DetectInterception("p1", ms.ToID, 0, 1)
	if e == nil || !e.Mothership || e.At != "orbit" {
		t.Errorf("arriving under the mothership met %+v", e)
	}
}

// Every answer is drawn from the fleet's seed and the day: the same
// campaign answers the same way, and over seeds an evasion at a warship's
// catch chance both succeeds and fails. An evasion costs its tier's fuel; a
// failed one is a boarding.
func TestInterceptionDeterministicPerSeed(t *testing.T) {
	evaded, caught := 0, 0
	r := LoadFactionData().Intercept
	for seed := int64(1); seed <= 40; seed++ {
		var outcomes [2]string
		for k := 0; k < 2; k++ {
			c, _ := interceptCampaign(t, seed, 2)
			c.Encounter = c.DetectInterception("p1", "p2", 0, 8)
			e, err := c.AnswerEncounter(AnswerEvade)
			if err != nil {
				t.Fatal(err)
			}
			if e.FuelCost != r.EvadeFuel[2] {
				t.Errorf("a warship's evasion cost %d fuel, want %d", e.FuelCost, r.EvadeFuel[2])
			}
			if e.Outcome == "caught" && e.BoardingID == "" {
				t.Error("caught, but no boarding")
			}
			outcomes[k] = e.Outcome
		}
		if outcomes[0] != outcomes[1] {
			t.Fatalf("seed %d: %s then %s", seed, outcomes[0], outcomes[1])
		}
		if outcomes[0] == "evaded" {
			evaded++
		} else {
			caught++
		}
	}
	if evaded == 0 || caught == 0 {
		t.Errorf("over 40 seeds: %d evaded, %d caught; want both", evaded, caught)
	}
	t.Logf("warship evasions: %d evaded, %d caught", evaded, caught)
}

// Boarding builds the ship's deck as an encounter location (its class from
// the tier) and holds the ship there while the deck is loaded; a crew of
// none left is the ship taken: the fleet is gone and Legends records it.
// Some left when the player leaves (the deck freezes), it goes on with them.
func TestInterceptionBoardAndTake(t *testing.T) {
	c, f := interceptCampaign(t, 3, 1)
	c.Encounter = c.DetectInterception("p1", "p2", 0, 8)
	e, err := c.AnswerEncounter(AnswerBoard)
	if err != nil || e.Outcome != "boarded" {
		t.Fatalf("board: %+v, %v", e, err)
	}
	b := c.Locations[e.BoardingID]
	if b == nil || b.Kind != EncounterKind || b.Boarding == nil || b.Boarding.Class != "freighter" || b.MapID != "boarding" {
		t.Fatalf("boarding location %+v", b)
	}
	if c.FactionSiteOK(b, false) {
		t.Error("a boarded deck is a site factions may hold")
	}
	pos := f.ArriveDay
	c.StarDate += 3
	c.StepFactions(nil)
	if f = c.fleetByID(e.FleetID); f == nil || f.BoardedAt != b.ID || f.Landed() || f.ArriveDay != pos {
		t.Fatalf("the boarded ship moved on: %+v", f)
	}
	if c.ReconcileBoarding(b, 2) {
		t.Fatal("taken with two crew alive (before its deck was even built)")
	}
	b.Boarding.Built = true
	if c.ReconcileBoarding(b, 2) || c.fleetByID(e.FleetID).BoardedAt != b.ID || c.fleetByID(e.FleetID).Presence.Population != 2 {
		t.Fatal("two alive aboard: the ship should be held with two crew")
	}
	c.ReleaseBoarding(b)
	if c.fleetByID(e.FleetID).BoardedAt != "" {
		t.Fatal("left with two alive: the ship should go on with them")
	}
	c.fleetByID(e.FleetID).BoardedAt = b.ID
	if !c.ReconcileBoarding(b, 0) {
		t.Fatal("not taken with its crew dead")
	}
	if c.fleetByID(e.FleetID) != nil {
		t.Error("a taken ship is still a fleet")
	}
	if !hasEventType(c, "faction_ship_taken") || !hasEventType(c, "faction_boarded") {
		t.Error("no faction_boarded / faction_ship_taken Legends events")
	}
}

// Parley is offered to a neutral faction only.
func TestInterceptionParleyOnlyWhenNeutral(t *testing.T) {
	c, _ := interceptCampaign(t, 5, 0)
	c.Encounter = c.DetectInterception("p1", "p2", 0, 8)
	if c.CanParley(c.Encounter) {
		t.Fatal("the hostile greyish offer parley")
	}
	if _, err := c.AnswerEncounter(AnswerParley); err == nil {
		t.Fatal("parleyed with the hostile greyish")
	}
	c.SetStance("greyish", StanceNeutral, nil)
	e, err := c.AnswerEncounter(AnswerParley)
	if err != nil || e.Outcome != "parleyed" {
		t.Fatalf("parley when neutral: %+v, %v", e, err)
	}
	if !hasEventType(c, "faction_parley") {
		t.Error("no faction_parley event")
	}
	_ = history.StarDate(0)
}

// Taking a mothership is DestroyMothership.
func TestBoardingTheMothership(t *testing.T) {
	c, _ := interceptCampaign(t, 7, 0)
	d := greyDoctrine(t)
	ms := parkMothership(c, "greyish", d, map[string]bool{"p1": true, "n": true, "s": true})
	ms.Presence.Stockpile = map[string]int{"metal_ore": 80}
	c.Encounter = c.DetectInterception("p1", ms.ToID, 0, 1)
	e, err := c.AnswerEncounter(AnswerBoard)
	if err != nil {
		t.Fatal(err)
	}
	b := c.Locations[e.BoardingID]
	if b.Boarding.Class != "mothership" {
		t.Errorf("class %q", b.Boarding.Class)
	}
	b.Boarding.Built = true
	if !c.ReconcileBoarding(b, 0) || c.Mothership("greyish") != nil {
		t.Fatal("the mothership survived its crew")
	}
	if !hasEventType(c, "faction_mothership_lost") {
		t.Error("no faction_mothership_lost event")
	}
}
