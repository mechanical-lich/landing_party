package campaign

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// fleetTestCampaign is a full hive at "hive" with one free site 20 units
// away (an egg takes 40 days at speed 0.5).
func fleetTestCampaign(t *testing.T) (*Campaign, *Doctrine) {
	t.Helper()
	c := factionTestCampaign(t, map[string]string{"hive": "earth_like", "far": "earth_like"})
	c.Locations["far"].X, c.Locations["far"].Y = c.Locations["hive"].X+20, c.Locations["hive"].Y
	d := hiveDoctrine(t)
	c.newPresence("hive", d, c.Locations["hive"], d.CapacityAt(c.Locations["hive"]))
	return c, d
}

// A fired egg is a projectile fleet that moves along its line by the day
// and the tick, and lands as a brood presence carrying its parent's techs.
func TestFleetMotionAndArrival(t *testing.T) {
	c, d := fleetTestCampaign(t)
	c.Locations["hive"].Factions[0].KnownTechs = []string{"tier_advanced"}
	c.StarDate = 1
	c.StepFactions(nil)
	if len(c.Fleets) != 1 {
		t.Fatalf("fleets = %+v", c.Fleets)
	}
	f := &c.Fleets[0]
	if f.ID != "fl1" || f.Kind != FleetProjectile || f.FromID != "hive" || f.ToID != "far" || f.ArriveDay != 41 {
		t.Fatalf("fleet = %+v", f)
	}
	hive, far := c.Locations["hive"], c.Locations["far"]
	for _, tc := range []struct{ t, want float64 }{{0, hive.X}, {1, hive.X}, {21, hive.X + 10}, {31.5, hive.X + 15.25}, {41, far.X}, {60, far.X}} {
		if x, y := f.Pos(tc.t); math.Abs(x-tc.want) > 1e-9 || y != hive.Y {
			t.Errorf("Pos(%v) = %v,%v, want %v", tc.t, x, y, tc.want)
		}
	}
	c.StarDate, c.StarTick = 21, history.TicksPerDay/2
	if x, _ := f.Pos(c.StarTime()); math.Abs(x-(hive.X+10.25)) > 1e-9 {
		t.Errorf("mid-day position %v", x)
	}
	c.StarTick = 0
	c.StarDate = 41
	c.StepFactions(nil)
	if len(c.Fleets) != 0 {
		t.Fatalf("fleet still flying after arrival: %+v", c.Fleets)
	}
	p := far.Presence("hive")
	if p == nil || p.Population != d.Mobility.EggPop || p.Stage != 0 || p.Materialized || !reflect.DeepEqual(p.KnownTechs, []string{"tier_advanced"}) {
		t.Errorf("brood = %+v", p)
	}
}

// The player sees an egg leave a hive they know, or catches it in scanner
// range of the ship; otherwise it flies unseen. Fleets survive a save.
func TestFleetVisibility(t *testing.T) {
	c, _ := fleetTestCampaign(t)
	c.StarDate = 1
	c.StepFactions(nil)
	if c.Fleets[0].Known || len(c.KnownFleets()) != 0 {
		t.Fatal("an egg from an unseen hive is known")
	}
	// A scanner at a site 22 units off the egg's line sees it once it is
	// within range (tier 1: 25).
	c.Locations["watch"] = &Location{ID: "watch", Name: "watch", Kind: "planet", X: c.Locations["hive"].X + 15, Y: c.Locations["hive"].Y + 22, Discovered: true}
	c.CurrentLocationID = "watch"
	c.UnlockTech("ship_scanner_1")
	c.StarDate = 2
	c.StepFactions(nil)
	if c.Fleets[0].Known {
		t.Fatal("seen out of range")
	}
	c.StarDate = 25
	c.StepFactions(nil)
	if !c.Fleets[0].Known || len(c.KnownFleets()) != 1 {
		t.Fatalf("not seen in range: %+v", c.Fleets[0])
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back Campaign
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.Fleets, c.Fleets) || back.FleetSeq != 1 {
		t.Errorf("fleets after a save: %+v", back.Fleets)
	}

	// Seeing the hive later reveals the eggs it fired still in flight.
	c3, _ := fleetTestCampaign(t)
	c3.StarDate = 1
	c3.StepFactions(nil)
	c3.RevealPresences(c3.Locations["hive"])
	if !c3.Fleets[0].Known {
		t.Error("seeing the hive did not reveal its egg in flight")
	}

	// From a known hive, the egg is known from launch.
	c2, _ := fleetTestCampaign(t)
	c2.Locations["hive"].Factions[0].Known = true
	c2.StarDate = 1
	c2.StepFactions(nil)
	if len(c2.Fleets) != 1 || !c2.Fleets[0].Known {
		t.Fatalf("egg from a known hive: %+v", c2.Fleets)
	}
}
