package campaign

import (
	"math"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

func greyDoctrine(t *testing.T) *Doctrine {
	t.Helper()
	d := LoadFactionData().Doctrine("greyish")
	if d == nil || d.Settlement == nil || d.Ships == nil || d.Ships.Mothership == nil {
		t.Fatal("no greyish doctrine with a settlement, ships and a mothership")
	}
	return d
}

// greyTestCampaign: a greyish scout landed on "ore" (an ore site), its
// mothership in orbit of "mother", free sites "a" and "b" in range.
func greyTestCampaign(t *testing.T) (*Campaign, *Doctrine, *Fleet, *Fleet) {
	t.Helper()
	c := factionTestCampaign(t, map[string]string{"ore": "earth_like", "mother": "earth_like", "a": "earth_like", "b": "earth_like"})
	d := greyDoctrine(t)
	base := c.Locations["ore"]
	for i, id := range []string{"mother", "a", "b"} {
		c.Locations[id].X, c.Locations[id].Y = base.X+float64(10*(i+1)), base.Y
	}
	ms := c.newShip("greyish", d, c.Locations["mother"], d.Ships.Mothership.Crew, false)
	ms.LandedID, ms.Mothership, ms.ShipTier = "", true, len(d.ShipTiers())-1
	ms.Presence.Stockpile = map[string]int{}
	f := c.newShip("greyish", d, base, 3, true)
	f.Presence.Stockpile = nil
	return c, d, &c.Fleets[1], &c.Fleets[0]
}

// Seeding three rival factions puts a greyish scout on a site and its
// mothership in orbit far from Home, at the top tier, never landed.
func TestGreyishSeededWithAMothership(t *testing.T) {
	useRepoData(t)
	for seed := int64(1); seed <= 6; seed++ {
		c, err := GenerateCampaignOpts("Three", seed, GenOptions{Factions: 3})
		if err != nil {
			t.Fatal(err)
		}
		d := greyDoctrine(t)
		ms := c.Mothership("greyish")
		if ms == nil {
			t.Fatalf("seed %d: no greyish mothership", seed)
		}
		if ms.Landed() || ms.ShipTier != len(d.ShipTiers())-1 || ms.Presence.Population != d.Ships.Mothership.Crew {
			t.Errorf("seed %d: mothership %+v", seed, ms)
		}
		var home *Location
		for _, l := range c.Locations {
			if l.Kind == HomeKind {
				home = l
			}
		}
		at := c.Locations[ms.ToID]
		if at == nil || !c.FactionSiteOK(at, true) {
			t.Fatalf("seed %d: mothership over %q", seed, ms.ToID)
		}
		far := math.Hypot(at.X-home.X, at.Y-home.Y)
		for _, l := range c.Locations {
			if c.FactionSiteOK(l, true) && far < d.Ships.Mothership.MinHomeDistance && math.Hypot(l.X-home.X, l.Y-home.Y) > far+1e-9 {
				t.Errorf("seed %d: mothership %.0f from Home, but %s is farther", seed, far, l.ID)
				break
			}
		}
		scouts := 0
		for _, f := range c.Fleets {
			if f.Presence.Doctrine == "greyish" && !f.Mothership {
				scouts++
				if !f.Landed() || f.ShipTier != 0 || f.Presence.Population > d.CrewCap(0) {
					t.Errorf("seed %d: scout %+v", seed, f)
				}
			}
		}
		if scouts != 1 {
			t.Errorf("seed %d: %d greyish ships, want 1 scout", seed, scouts)
		}
	}
}

// A landed scout digs its hold full on an ore site, its crew capped by its
// tier; with the hold at the target it lifts off for the mothership, docks
// (hold and techs delivered), and its tier rises only there, paid from the
// hoard; then it sets out again, its crew cap higher.
func TestGreyishStockpileAndTierRiseAtTheMothership(t *testing.T) {
	c, d, f, ms := greyTestCampaign(t)
	f.Presence.Population = 3
	f.Presence.KnownTechs = []string{"grey_hull_plating"}
	ms.Presence.Stockpile = map[string]int{"metal_ore": 50, "crystal": 20} // a freighter's cost
	start := c.FactionDay
	for day := 0; day < 60 && !f.Returning; day++ {
		c.StarDate++
		c.StepFactions(nil)
		f = c.FleetByPresence(f.Presence.ID)
		if f.Presence.Population > d.CrewCap(0) {
			t.Fatalf("a scout grew past its crew cap: %d", f.Presence.Population)
		}
	}
	if !f.Returning || f.ToID != "mother" {
		t.Fatalf("the full scout did not head home: %+v", f)
	}
	hold := holdUnits(f.Presence.Stockpile)
	if hold < d.Ships.HoldTarget {
		t.Fatalf("it left with %d in the hold, target %d", hold, d.Ships.HoldTarget)
	}
	t.Logf("hold %v after %d days", f.Presence.Stockpile, c.FactionDay-start)
	if f.ShipTier != 0 {
		t.Fatal("the tier rose away from the mothership")
	}
	before := holdUnits(ms.Presence.Stockpile)
	for day := 0; day < 20 && (f.Returning || f.ShipTier == 0); day++ {
		c.StarDate++
		c.StepFactions(nil)
		f, ms = c.FleetByPresence(f.Presence.ID), c.Mothership("greyish")
	}
	if f.ShipTier != 1 {
		t.Fatalf("docked with %d units and 50/20 hoarded: tier %d, want 1 (freighter)", hold, f.ShipTier)
	}
	if !hasStr(ms.Presence.KnownTechs, "grey_hull_plating") {
		t.Error("the scout's tech did not reach the mothership")
	}
	cost := d.ShipTiers()[1].Cost
	if got, want := holdUnits(ms.Presence.Stockpile), before+hold-cost["metal_ore"]-cost["crystal"]; got != want {
		t.Errorf("mothership hoard %d, want %d (before %d + hold %d - the tier's cost)", got, want, before, hold)
	}
	if len(f.Presence.Stockpile) != 0 || f.Returning || f.ToID == "mother" || f.Landed() {
		t.Errorf("after docking it should be bound for a new site, hold empty: %+v", f)
	}
	if !hasEventType(c, "faction_upgraded") {
		t.Error("no faction_upgraded Legends event")
	}
	// Landed again, it grows to the freighter's crew.
	for day := 0; day < 60; day++ {
		c.StarDate++
		c.StepFactions(nil)
		f = c.FleetByPresence(f.Presence.ID)
	}
	if f.Presence.Population <= d.CrewCap(0) || f.Presence.Population > d.CrewCap(1) {
		t.Errorf("a freighter's crew %d, want above %d and at most %d", f.Presence.Population, d.CrewCap(0), d.CrewCap(1))
	}
}

// A mothership builds a ship from its hoard when it can and has room.
func TestMothershipBuildsShips(t *testing.T) {
	c, d, _, ms := greyTestCampaign(t)
	ms.Presence.Stockpile = map[string]int{"metal_ore": 200, "crystal": 80}
	c.StarDate++
	c.StepFactions(nil)
	n := 0
	for _, f := range c.Fleets {
		if f.Presence.Faction == "greyish" && !f.Mothership {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d greyish ships after a rich day, want 2", n)
	}
	for day := 0; day < 5; day++ {
		c.StarDate++
		c.StepFactions(nil)
	}
	n = 0
	for _, f := range c.Fleets {
		if f.Presence.Faction == "greyish" && !f.Mothership {
			n++
		}
	}
	if n > d.Ships.Mothership.MaxShips {
		t.Errorf("%d ships, max %d", n, d.Ships.Mothership.MaxShips)
	}
}

// Destroying the mothership (the plan's open question): its hoard goes to
// the victor, no tier rises again and no ship is built, and a ship bound
// home scatters to the nearest free site and lands there.
func TestDestroyingTheMothershipFreezesTiersAndScatters(t *testing.T) {
	c, d, f, ms := greyTestCampaign(t)
	ms.Presence.Stockpile = map[string]int{"metal_ore": 500, "crystal": 500}
	f.Presence.Stockpile = map[string]int{"metal_ore": d.Ships.HoldTarget}
	c.StarDate++
	c.StepFactions(nil)
	f = c.FleetByPresence(f.Presence.ID)
	if !f.Returning {
		t.Fatalf("a full ship should head home: %+v", f)
	}
	want := map[string]int{}
	for k, v := range c.Mothership("greyish").Presence.Stockpile {
		want[k] = v
	}
	hoard := c.DestroyMothership("greyish", c.Locations["mother"])
	if hoard["metal_ore"] != want["metal_ore"] || hoard["crystal"] != want["crystal"] {
		t.Errorf("the hoard returned %v, the mothership held %v", hoard, want)
	}
	if c.Mothership("greyish") != nil {
		t.Fatal("the mothership is still there")
	}
	f = c.FleetByPresence(f.Presence.ID)
	if f.Returning || f.ToID == "mother" || f.ToID == "" {
		t.Fatalf("the homebound ship did not scatter: %+v", f)
	}
	if !hasEventType(c, "faction_mothership_lost") {
		t.Error("no faction_mothership_lost Legends event")
	}
	for day := 0; day < 90; day++ {
		c.StarDate++
		c.StepFactions(nil)
	}
	for _, g := range c.Fleets {
		if g.Presence.Faction == "greyish" && g.ShipTier != 0 {
			t.Errorf("a tier rose with no mothership: %+v", g)
		}
	}
	n := 0
	for _, g := range c.Fleets {
		if g.Presence.Faction == "greyish" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("%d greyish ships after 90 days without a mothership, want the two it had built", n)
	}
	_ = history.StarDate(0)
}

func hasEventType(c *Campaign, typ string) bool {
	for _, ev := range c.History.Events {
		if ev.Type == typ {
			return true
		}
	}
	return false
}

// A scan sweep reveals the ships within its range, landed, flying or in
// orbit, and no farther.
func TestScanRevealsShipsInRange(t *testing.T) {
	c, _, f, ms := greyTestCampaign(t)
	f.Known, ms.Known = false, false
	ore := c.Locations["ore"]
	c.RevealPresencesWithin(ore.X, ore.Y, 5)
	if !f.Known {
		t.Error("the landed scout within range was not revealed")
	}
	if ms.Known {
		t.Error("the mothership 10 away was revealed by a 5-unit sweep")
	}
	c.RevealPresencesWithin(ore.X, ore.Y, 15)
	if !c.Mothership("greyish").Known {
		t.Error("the mothership within a 15-unit sweep was not revealed")
	}
}
