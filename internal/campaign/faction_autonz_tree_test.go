package campaign

import (
	"encoding/json"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/research"
)

// spineDays is the day each spine entry lands for a presence stepped
// days days on a, with or without the doctrine's assimilation walk.
func spineDays(t *testing.T, withAssim bool, days int) (map[string]int, *FactionPresence) {
	t.Helper()
	c := factionTestCampaign(t, map[string]string{"a": "earth_like"})
	d := autonzDoctrine(t)
	saved, savedPer := d.Assimilation, d.AssimilationPerDay
	if !withAssim {
		d.Assimilation, d.AssimilationPerDay = nil, nil
	}
	defer func() { d.Assimilation, d.AssimilationPerDay = saved, savedPer }()
	a := c.Locations["a"]
	c.newPresence("autonz", "autonz", d, a, 3)
	got := map[string]int{}
	for day := 1; day <= days; day++ {
		c.StarDate = history.StarDate(day)
		c.StepFactions(nil)
		p := a.Presence("autonz")
		if p == nil {
			t.Fatalf("presence gone on day %d", day)
		}
		for _, k := range p.KnownTechs {
			if _, ok := got[k]; !ok {
				got[k] = day
			}
		}
	}
	return got, a.Presence("autonz")
}

// Both walks advance; the spine's entries land on the days they land on
// without the second walk; no assimilation entry has a range or a rate.
func TestAutonzTreeTwoWalks(t *testing.T) {
	useRepoData(t)
	if err := research.Load("../../data/research.json"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = research.Load("") })
	with, p := spineDays(t, true, 160)
	without, _ := spineDays(t, false, 160)
	d := autonzDoctrine(t)
	for _, e := range d.Research {
		if with[e.ID] != without[e.ID] {
			t.Errorf("spine %s on day %d with the second walk, %d without", e.ID, with[e.ID], without[e.ID])
		}
	}
	for _, k := range []string{"autonz_relay_pattern", "autonz_seeker_pattern", "autonz_foundry_pattern"} {
		if with[k] == 0 {
			t.Errorf("%s not known by day 160 (%v)", k, with)
		}
	}
	if with["autonz_relay_pattern"] > with["autonz_seeker_pattern"] {
		t.Errorf("walk out of order: %v", with)
	}
	if p.AssimPoints < 0 {
		t.Errorf("assim points %v", p.AssimPoints)
	}
	r0 := d.Range(nil)
	var all []string
	for _, e := range d.Assimilation {
		all = append(all, e.ID)
		if e.Range != 0 || e.ConversionRate != 0 {
			t.Errorf("assimilation %s carries range or rate", e.ID)
		}
		tech, ok := research.GetTech(e.ID)
		if !ok || tech.Tree != "autonz" || tech.Assimilate == nil || tech.Assimilate.Range != 0 || tech.Assimilate.ConversionRate != 0 {
			t.Errorf("research %s: %+v", e.ID, tech)
		}
	}
	if d.Range(all) != r0 || d.ConversionRate(all) != 1 {
		t.Errorf("the assimilation walk moved range %.1f or rate %.2f", d.Range(all), d.ConversionRate(all))
	}
	// The four spine entries are tree entries with their effects.
	for _, e := range d.Research {
		tech, ok := research.GetTech(e.ID)
		if !ok || tech.Tree != "autonz" || tech.Assimilate == nil || tech.Assimilate.Range != e.Range || tech.Assimilate.ConversionRate != e.ConversionRate {
			t.Errorf("spine %s: %+v", e.ID, tech)
		}
	}
	// Every unlock and bronze entry names a type the doctrine has, and the
	// settlement's list is both walks.
	for _, e := range append(append([]DoctrineTech{}, d.Research...), d.Assimilation...) {
		a := research.AllTechs()[e.ID].Assimilate
		if a.Unlock != "" && d.Types[a.Unlock] == nil || a.Tier != nil && (d.Types[a.Tier.Type] == nil || d.Tier(a.Tier.Tier) == nil) {
			t.Errorf("%s: %+v", e.ID, a)
		}
		if !d.Settlement.CanResearch(e.ID) {
			t.Errorf("%s not in the settlement's research", e.ID)
		}
	}
	if len(d.Settlement.Research) != len(d.Research)+len(d.Assimilation) {
		t.Errorf("settlement research %d, walks %d", len(d.Settlement.Research), len(d.Research)+len(d.Assimilation))
	}
	if d.TierOfType("enforcer", nil) != "steel" || d.TierOfType("enforcer", []string{"autonz_bronze_enforcer"}) != "bronze" ||
		d.TierOfType("wire_placer", []string{"autonz_bronze_enforcer"}) != "steel" {
		t.Error("TierOfType")
	}
	if u := d.TypesUnlocked([]string{"autonz_relay_pattern"}); !u["relay_drone"] || u["seeker"] || !u["wire_placer"] {
		t.Errorf("unlocked %v", u)
	}
}

// A save without assim_points loads, and the walk carries on from it.
func TestAutonzTreeOldSave(t *testing.T) {
	var p FactionPresence
	if err := json.Unmarshal([]byte(`{"id":"fp1","faction":"autonz","doctrine":"autonz","population":4,"known_techs":["signal_range_1"],"research_pts":3}`), &p); err != nil {
		t.Fatal(err)
	}
	if p.AssimPoints != 0 || len(p.KnownTechs) != 1 {
		t.Errorf("%+v", p)
	}
}
