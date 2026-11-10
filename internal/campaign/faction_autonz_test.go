package campaign

import (
	"reflect"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

func autonzDoctrine(t *testing.T) *Doctrine {
	t.Helper()
	d := LoadFactionData().Doctrine("autonz")
	if d == nil {
		t.Fatal("no autonz doctrine in data/factions.json")
	}
	return d
}

// The doctrine: planet-carried, signal mobility, convert economy, a node
// then a signalling hub, the stages' stamps, hostile by default and seeded
// only from the fourth rival faction.
func TestAutonzDoctrineLoads(t *testing.T) {
	d := func() *Doctrine { useRepoData(t); return autonzDoctrine(t) }()
	if d.Mobility.Kind != MobilitySignal || !d.Converts() || d.Settlement == nil || d.SeedFrom != 4 {
		t.Fatalf("autonz doctrine: %+v", d)
	}
	if len(d.Stages) != 2 || d.Stages[0].Stamp != "autonz_node" || d.Stages[1].Stamp != "autonz_hub" || !d.Stages[1].Signal || d.Stages[0].Signal {
		t.Errorf("stages %+v", d.Stages)
	}
	if d.Conversion.Tile != "blue_wires" || d.ConversionCap() >= 1 || d.Conversion.Floor <= 0 {
		t.Errorf("conversion %+v", d.Conversion)
	}
	c := &Campaign{}
	if c.Stance("autonz") != StanceHostile {
		t.Errorf("autonz stance %q, want hostile by default", c.Stance("autonz"))
	}
	if d.MorphAt(nil).Tag != "AUTONZ" {
		t.Errorf("tag %q", d.MorphAt(nil).Tag)
	}
}

// Rival factions 1 to 3 never seed an autonz presence (so every earlier
// seed's map and vmtest sites stay as they were); 4 seeds exactly one,
// awake, at its node stage, on a site a presence may be on.
func TestAutonzSeededFromTheFourthFaction(t *testing.T) {
	useRepoData(t)
	for seed := int64(1); seed <= 6; seed++ {
		for n := 1; n <= 4; n++ {
			c, err := GenerateCampaignOpts("A", seed, GenOptions{Factions: n})
			if err != nil {
				t.Fatal(err)
			}
			var found []*Location
			for _, loc := range c.sortedLocations() {
				if p := loc.Presence("autonz"); p != nil {
					found = append(found, loc)
					if p.Dormant || p.Stage != 0 || p.Population != autonzDoctrine(t).SeedPopulation || !c.FactionSiteOK(loc, true) {
						t.Errorf("seed %d n %d: autonz presence %+v on %s", seed, n, p, loc.ID)
					}
				}
			}
			want := 0
			if n >= 4 {
				want = 1
			}
			if len(found) != want {
				t.Errorf("seed %d, Rival factions %d: %d autonz presences, want %d", seed, n, len(found), want)
			}
			if n == 4 && !hasEventType(c, "faction_swarm_landed") {
				t.Errorf("seed %d: no faction_swarm_landed on seeding", seed)
			}
		}
	}
}

// The conversion share is the economy: the capacity grows with it from the
// floor to the site's full capacity at the cap, conversion rises faster on
// ore and with the rate techs, and growth stops at the conversion-capped
// capacity.
func TestAutonzConversionDrivesGrowthAndCapacity(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"a": "earth_like", "b": "earth_like"})
	d := autonzDoctrine(t)
	a, b := c.Locations["a"], c.Locations["b"]
	b.Tags = []string{"biomass"} // no ore
	full := d.CapacityAt(a)
	p := c.newPresence("autonz", d, a, 3)
	if got := d.ConvertCap(a, p); got >= full || got < d.Capacity.Min {
		t.Fatalf("unconverted capacity %d of %d", got, full)
	}
	low := d.ConvertCap(a, p)
	p.Conversion = d.ConversionCap() / 2
	mid := d.ConvertCap(a, p)
	p.Conversion = d.ConversionCap()
	if top := d.ConvertCap(a, p); !(low < mid && mid < top && top == full) {
		t.Errorf("capacity by conversion: %d, %d, %d (full %d)", low, mid, top, full)
	}
	p.Conversion = 0

	// Thirty days on an ore site against one without ore.
	q := c.newPresence("autonz", d, b, 3)
	c.StarDate = 30
	c.StepFactions(nil)
	p, q = a.Presence("autonz"), b.Presence("autonz")
	if p.Conversion <= 0 || q.Conversion <= 0 || p.Conversion <= q.Conversion {
		t.Errorf("conversion after 30 days: ore %.4f, no ore %.4f", p.Conversion, q.Conversion)
	}
	if p.Population > d.ConvertCap(a, p) {
		t.Errorf("population %d above its conversion cap %d", p.Population, d.ConvertCap(a, p))
	}
	// Techs raise the rate.
	if r := d.ConversionRate([]string{"conversion_rate_1"}); r <= 1 {
		t.Errorf("conversion_rate_1 gives rate %.2f", r)
	}

	// Long run: conversion reaches the cap and never passes it, the
	// population fills the capacity it allows.
	c.StarDate = 2000
	c.StepFactions(nil)
	p = a.Presence("autonz")
	if p == nil || p.Conversion > d.ConversionCap()+1e-9 || p.Conversion < d.ConversionCap()-1e-9 {
		t.Fatalf("after 2000 days: %+v", p)
	}
}

// A fully converted site stops growing and spends its population on
// signals: no growth day to day, and every signal costs shot_pop.
func TestAutonzFullyConvertedSpendsPopulation(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"a": "earth_like"})
	d := autonzDoctrine(t)
	a := c.Locations["a"]
	p := c.newPresence("autonz", d, a, 9)
	p.Stage, p.Conversion = 1, d.ConversionCap()
	// Nowhere to signal (every other site is far out of range): it holds.
	c.StarDate = 10
	c.StepFactions(nil)
	if got := a.Presence("autonz").Population; got != 9 {
		t.Fatalf("a fully converted presence grew or shrank to %d with nothing to signal", got)
	}
	// A site in range: one signal, its cost paid, no growth after.
	c.Locations["n"] = &Location{ID: "n", Name: "n", Kind: "planet", ScenarioID: "earth_like", Tags: []string{"ore"}, X: a.X + 5, Y: a.Y, Discovered: true}
	c.StarDate = 20
	c.StepFactions(nil)
	if got := a.Presence("autonz").Population; got != 9-d.Mobility.ShotPop {
		t.Errorf("after a signal at full conversion: population %d, want %d", got, 9-d.Mobility.ShotPop)
	}
}

// A hub signals on its cooldown to a site within range only, founding a
// dormant presence there that wakes after wake_days; the same campaign
// signals the same way every time.
func TestAutonzSignalSpread(t *testing.T) {
	run := func() *Campaign {
		c := factionTestCampaign(t, map[string]string{"a": "earth_like", "near": "earth_like", "near2": "earth_like", "far": "earth_like"})
		d := autonzDoctrine(t)
		a := c.Locations["a"]
		c.Locations["near"].X, c.Locations["near"].Y = a.X+8, a.Y
		c.Locations["near2"].X, c.Locations["near2"].Y = a.X, a.Y+12
		c.Locations["far"].X, c.Locations["far"].Y = a.X+d.Mobility.BaseRange+5, a.Y
		p := c.newPresence("autonz", d, a, 8)
		p.Stage = 1
		p.Conversion = d.Mobility.SignalConversion
		return c
	}
	c := run()
	d := autonzDoctrine(t)
	c.StarDate = 1
	c.StepFactions(nil)
	dormant := func(c *Campaign) []string {
		var ids []string
		for _, loc := range c.sortedLocations() {
			if p := loc.Presence("autonz"); p != nil && p.Dormant {
				ids = append(ids, loc.ID)
			}
		}
		return ids
	}
	got := dormant(c)
	if len(got) != 1 || got[0] == "far" {
		t.Fatalf("after day 1 the dormant presences are %v, want one in range", got)
	}
	q := c.Locations[got[0]].Presence("autonz")
	if q.WakeDay != 1+history.StarDate(d.Mobility.WakeDays) || q.Population != d.Mobility.EggPop || q.Stage != 0 {
		t.Errorf("dormant presence %+v", q)
	}
	if !hasEventType(c, "faction_signalled") {
		t.Error("no faction_signalled event")
	}
	// Inside the cooldown nothing more (the pressure divides it).
	cool := int(float64(d.Mobility.Cooldown) / c.pressure(LoadFactionData()))
	c.Locations["a"].Presence("autonz").Population = 8
	c.StarDate = history.StarDate(cool)
	c.StepFactions(nil)
	if n := len(dormant(c)) + countAwake(c) - 1; n != 1 {
		t.Fatalf("%d signals inside the %d-day cooldown", n, cool)
	}
	c.Locations["a"].Presence("autonz").Population = 8
	c.StarDate = history.StarDate(cool + 1)
	c.StepFactions(nil)
	if c.Locations["far"].HasFaction("autonz") {
		t.Error("a signal reached a site out of range")
	}
	if !c.Locations["near"].HasFaction("autonz") || !c.Locations["near2"].HasFaction("autonz") {
		t.Errorf("after the cooldown both near sites should hold the autonz: %v", dormant(c))
	}
	// Deterministic.
	c2 := run()
	c2.StarDate = 1
	c2.StepFactions(nil)
	if !reflect.DeepEqual(dormant(c2), got) {
		t.Errorf("a second run signalled %v, the first %v", dormant(c2), got)
	}
}

func countAwake(c *Campaign) int {
	n := 0
	for _, loc := range c.Locations {
		if p := loc.Presence("autonz"); p != nil && !p.Dormant {
			n++
		}
	}
	return n
}

// A dormant presence neither grows, converts nor researches until its wake
// day; then it wakes (faction_woke) and steps as a node.
func TestAutonzWakeDay(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"a": "earth_like"})
	d := autonzDoctrine(t)
	a := c.Locations["a"]
	p := c.newPresence("autonz", d, a, 1)
	p.Dormant, p.WakeDay = true, 15
	c.StarDate = 14
	c.StepFactions(nil)
	p = a.Presence("autonz")
	if !p.Dormant || p.Population != 1 || p.Conversion != 0 || p.Growth != 0 {
		t.Fatalf("asleep on day 14: %+v", p)
	}
	c.StarDate = 15
	c.StepFactions(nil)
	p = a.Presence("autonz")
	if p.Dormant || p.ArrivedDay != 15 || !hasEventType(c, "faction_woke") {
		t.Fatalf("woke on day 15: %+v", p)
	}
	c.StarDate = 60
	c.StepFactions(nil)
	if p = a.Presence("autonz"); p.Conversion <= 0 {
		t.Errorf("awake 45 days and converted nothing: %+v", p)
	}
	// Loaded: WakePresence wakes it on the level's day.
	q := c.newPresence("autonz", d, c.Locations["a"], 1)
	q.Dormant, q.WakeDay = true, 70
	if c.WakePresence(a, q.ID) {
		t.Error("woke before its day")
	}
	c.FactionDay = 70
	if !c.WakePresence(a, q.ID) || a.PresenceByID(q.ID).Dormant {
		t.Error("did not wake on its day")
	}
	// A sleeper destroyed before it wakes is a wipe.
	r := c.newPresence("autonz", d, a, 1)
	r.Dormant, r.WakeDay = true, 200
	if !c.WipeDormant(a, r.ID) || a.PresenceByID(r.ID) != nil {
		t.Error("WipeDormant left the presence")
	}
}

// A scan sweep reveals dormant presences in its range like any other, and
// the Star Map's signal note counts down to the wake day.
func TestAutonzScanRevealsDormant(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"a": "earth_like", "b": "earth_like"})
	d := autonzDoctrine(t)
	a, b := c.Locations["a"], c.Locations["b"]
	p := c.newPresence("autonz", d, a, 1)
	p.Dormant, p.WakeDay = true, 9
	q := c.newPresence("autonz", d, b, 1)
	q.Dormant, q.WakeDay = true, 9
	c.RevealPresencesWithin(a.X, a.Y, 50)
	if !a.Presence("autonz").Known || b.Presence("autonz").Known {
		t.Fatalf("scan reveal: a %v, b %v", a.Presence("autonz").Known, b.Presence("autonz").Known)
	}
	if got := len(a.KnownDormant()); got != 1 {
		t.Errorf("KnownDormant = %d", got)
	}
	c.StarDate = 5
	if got := c.SignalNote(d, a, a.Presence("autonz")); got != "AUTONZ SIGNAL · WAKES IN 4 DAYS" {
		t.Errorf("signal note %q", got)
	}
}
