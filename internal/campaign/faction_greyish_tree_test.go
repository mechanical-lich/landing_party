package campaign

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/research"
)

func loadRepoResearch(t *testing.T) {
	t.Helper()
	if err := research.Load("../../data/research.json"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = research.Load("") })
}

// greyShipDays steps a grey ship landed on an ore site of tier for days
// days (its stay long and its hold target out of reach, so it stays), with
// or without the second walk, and reports the day each tech was learned.
func greyShipDays(t *testing.T, withSecond bool, tier, days int) (map[string]int, *Fleet) {
	t.Helper()
	c, d, f, _ := greyTestCampaign(t)
	saved, savedPer := d.Assimilation, d.AssimilationPerDay
	stay, hold := d.Mobility.StayDays, d.Ships.HoldTarget
	if !withSecond {
		d.Assimilation, d.AssimilationPerDay = nil, nil
	}
	d.Mobility.StayDays, d.Ships.HoldTarget = 10000, 100000
	defer func() {
		d.Assimilation, d.AssimilationPerDay = saved, savedPer
		d.Mobility.StayDays, d.Ships.HoldTarget = stay, hold
	}()
	f.ShipTier = tier
	id := f.Presence.ID
	got := map[string]int{}
	for day := 1; day <= days; day++ {
		c.StarDate = history.StarDate(day)
		c.StepFactions(nil)
		f = c.FleetByPresence(id)
		if f == nil || !f.Landed() {
			t.Fatalf("the ship left on day %d", day)
		}
		for _, k := range f.Presence.KnownTechs {
			if _, ok := got[k]; !ok {
				got[k] = day
			}
		}
	}
	return got, f
}

// A landed grey ship walks both lists: the spine's entries land on the same
// days with or without the second walk, and the second walk's pace is by
// hull tier (a warship's 1.5 a day is three times a scout's 0.5).
func TestGreyishTreeTwoWalksOnShips(t *testing.T) {
	useRepoData(t)
	d := greyDoctrine(t)
	if len(d.Assimilation) == 0 || len(d.AssimilationPerDay) != len(d.ShipTiers()) {
		t.Fatalf("the greys' second walk %d entries, per day %v", len(d.Assimilation), d.AssimilationPerDay)
	}
	with, _ := greyShipDays(t, true, 0, 160)
	without, _ := greyShipDays(t, false, 0, 160)
	for _, e := range d.Research {
		if with[e.ID] == 0 || with[e.ID] != without[e.ID] {
			t.Errorf("spine %s on day %d with the second walk, %d without", e.ID, with[e.ID], without[e.ID])
		}
	}
	first := d.Assimilation[0].ID
	if with[first] == 0 {
		t.Fatalf("a scout learned no second-walk entry in 160 days: %v", with)
	}
	war, _ := greyShipDays(t, true, 2, 160)
	if war[first] == 0 || war[first] >= with[first] {
		t.Errorf("%s: scout day %d, warship day %d: the warship should be faster", first, with[first], war[first])
	}
	n := 0
	for _, e := range d.Assimilation {
		if war[e.ID] > 0 {
			n++
		}
	}
	t.Logf("scout: %s on day %d; warship: %d of %d second-walk entries by day 160", first, with[first], n, len(d.Assimilation))
}

// The second walk's techs go to the mothership at dock and back out with
// every ship, as the spine's do.
func TestGreyishSecondWalkMergesAtDock(t *testing.T) {
	useRepoData(t)
	c, _, f, ms := greyTestCampaign(t)
	f.Presence.KnownTechs = []string{"grey_salvage_rigs"}
	f.Presence.Stockpile = map[string]int{"metal_ore": 40}
	for day := 0; day < 30 && !f.Returning; day++ {
		c.StarDate++
		c.StepFactions(nil)
		f = c.FleetByPresence(f.Presence.ID)
	}
	for day := 0; day < 30 && f.Returning; day++ {
		c.StarDate++
		c.StepFactions(nil)
		f, ms = c.FleetByPresence(f.Presence.ID), c.Mothership("greyish")
	}
	if !hasStr(ms.Presence.KnownTechs, "grey_salvage_rigs") {
		t.Errorf("the mothership did not take the second walk's tech: %v", ms.Presence.KnownTechs)
	}
}

// No grey entry has an effect Tier 1 reads (greyish_plan.md, "The tree"):
// only a develop block, whose keys are the live and Tier 3 effects; no
// evolve or assimilate (they carry range and rate). Every grey entry the
// doctrine walks, researches, unlocks or gates with is a greyish entry.
func TestGreyishTreeHasNoTier1Effect(t *testing.T) {
	useRepoData(t)
	loadRepoResearch(t)
	raw, err := os.ReadFile("../../data/research.json")
	if err != nil {
		t.Fatal(err)
	}
	var all map[string]map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"unlock": true, "upgrade": true, "craft": true, "build": true, "module": true,
		"print_time": true, "salvage": true, "intercept": true, "boarding": true}
	n := 0
	for key, e := range all {
		var tree string
		_ = json.Unmarshal(e["tree"], &tree)
		if tree != "greyish" {
			continue
		}
		n++
		if _, ok := e["evolve"]; ok {
			t.Errorf("%s has an evolve block", key)
		}
		if _, ok := e["assimilate"]; ok {
			t.Errorf("%s has an assimilate block", key)
		}
		var dev map[string]json.RawMessage
		if err := json.Unmarshal(e["develop"], &dev); err != nil || len(dev) == 0 {
			t.Errorf("%s has no develop block", key)
		}
		for k := range dev {
			if !allowed[k] {
				t.Errorf("%s: develop key %q (Tier 1 reads none of range, conversion_rate, yield, crew or tier)", key, k)
			}
		}
	}
	if n != 16 {
		t.Errorf("%d greyish entries, want 16", n)
	}
	d := greyDoctrine(t)
	isGrey := func(id string) bool {
		tech, ok := research.GetTech(id)
		return ok && tech.Tree == "greyish"
	}
	for _, e := range append(append([]DoctrineTech(nil), d.Research...), d.Assimilation...) {
		if !isGrey(e.ID) {
			t.Errorf("walk entry %s is not a greyish entry", e.ID)
		}
	}
	for _, k := range d.Settlement.Research {
		if !isGrey(k) {
			t.Errorf("settlement research %s is not a greyish entry", k)
		}
	}
	for k, ty := range d.Types {
		if ty.Unlock != "" && !isGrey(ty.Unlock) {
			t.Errorf("type %s unlocks by %s", k, ty.Unlock)
		}
	}
	for _, tier := range d.ShipTiers() {
		for _, m := range tier.Modules {
			if m.RequiresTech != "" && !isGrey(m.RequiresTech) {
				t.Errorf("module %s needs %s", m.Key, m.RequiresTech)
			}
		}
	}
	// Both walks together cover the settlement's live list.
	walked := map[string]bool{}
	for _, e := range append(append([]DoctrineTech(nil), d.Research...), d.Assimilation...) {
		walked[e.ID] = true
	}
	for _, k := range d.Settlement.Research {
		if !walked[k] {
			t.Errorf("%s is researched live but on neither Tier 1 walk", k)
		}
	}
}

// The tractor lock raises the player's chance of being caught evading a
// grey ship; the cloak shrinks the range a jump meets one in flight.
func TestGreyishInterceptTechs(t *testing.T) {
	useRepoData(t)
	loadRepoResearch(t)
	caught := func(techs []string) int {
		n := 0
		for seed := int64(1); seed <= 200; seed++ {
			c, f := interceptCampaign(t, seed, 0)
			f.Presence.KnownTechs = techs
			e := c.DetectInterception("p1", "p2", 0, 8)
			if e == nil {
				t.Fatal("no encounter")
			}
			c.Encounter = e
			if out, err := c.AnswerEncounter(AnswerEvade); err == nil && out.Outcome == "caught" {
				n++
			}
		}
		return n
	}
	plain, locked := caught(nil), caught([]string{"grey_tractor_lock"})
	if locked <= plain {
		t.Errorf("caught %d/200 with the tractor lock, %d without", locked, plain)
	}
	t.Logf("evade caught %d/200 plain, %d/200 with the tractor lock", plain, locked)
	r := LoadFactionData().Intercept
	// A ship flying beside the jump, just inside the plain range the whole
	// way: met plain, missed cloaked.
	c, f := interceptCampaign(t, 1, 0)
	off := r.Range - 1
	f.FromX, f.FromY, f.ToX, f.ToY = 0, off, 40, off
	if c.DetectInterception("p1", "p2", 0, 8) == nil {
		t.Fatalf("the jump %v off the ship's path was not met (range %v)", off, r.Range)
	}
	f.Presence.KnownTechs = []string{"grey_tractor_lock", "grey_cloak"}
	if e := c.DetectInterception("p1", "p2", 0, 8); e != nil {
		t.Errorf("a cloaked ship %v off the line was met (range %v - 2)", off, r.Range)
	}
}

// Beamed stock does not send a ship home (the owner's call, 2026-11-12):
// its hold target is measured without what its mothership beamed down and
// the camp has not spent; what the camp spends comes off the beamed units
// first; a new landing starts with nothing beamed.
func TestGreyishBeamedStockNotInHold(t *testing.T) {
	useRepoData(t)
	c, d, f, _ := greyTestCampaign(t)
	target := d.Ships.HoldTarget
	f.Presence.Stockpile = map[string]int{"metal_ore": target - 5}
	f.NoteBeamed(target-25, 20) // 20 of its 25 came down by beam (5 mined)
	f.Presence.Stockpile["metal_ore"] = target + 5
	if f.HoldForHome() != target-15 || c.ShipLeaving(f, c.FactionDay) {
		t.Errorf("hold for home %d with %d beamed: the ship leaves on beamed stock", f.HoldForHome(), f.Presence.Beamed)
	}
	f.NoteBeamed(target-18, 0) // its 25 down to 12: 13 spent, off the beamed first
	if f.Presence.Beamed != 7 {
		t.Errorf("beamed %d after spending 13 of 20, want 7", f.Presence.Beamed)
	}
	f.Presence.Stockpile["metal_ore"] = target + 7
	if !c.ShipLeaving(f, c.FactionDay) {
		t.Errorf("a ship with %d mined (hold %d, %d beamed) does not go home", f.HoldForHome(), target+7, f.Presence.Beamed)
	}
	f.ToID = f.LandedID
	c.LandShip(f, c.FactionDay)
	if f.Presence.Beamed != 0 {
		t.Error("a new landing kept the last camp's beamed units")
	}
}
