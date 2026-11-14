package campaign

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// Over 300 days of Tier 1 every hive's tally stays in bounds (larvae within
// the stage's cap, biomass within the pools') and its resin under the cap;
// growth is Phase A's (the pace and history-dump tests hold it).
func TestHiveTallyBounded(t *testing.T) {
	useRepoData(t)
	d := hiveDoctrine(t)
	for seed := int64(1); seed <= 4; seed++ {
		c, err := GenerateCampaignOpts("Tally", seed, GenOptions{Factions: 1})
		if err != nil {
			t.Fatal(err)
		}
		seen := false
		for day := 10; day <= 300; day += 10 {
			c.StarDate = history.StarDate(day)
			c.StepFactions(nil)
			for _, l := range c.sortedLocations() {
				for _, p := range l.Factions {
					if p.Doctrine != "hive" {
						continue
					}
					seen = true
					if p.Larvae < 0 || p.Larvae > d.LarvaCap(p.Stage) {
						t.Errorf("seed %d day %d %s: larvae %d, cap %d", seed, day, p.ID, p.Larvae, d.LarvaCap(p.Stage))
					}
					// The cap is the day's before its shot left.
					if b, cap := p.Stockpile["biomass"], d.BiomassCap(p.Population+d.Mobility.ShotPop); b < 0 || b > cap {
						t.Errorf("seed %d day %d %s: biomass %d, cap %d", seed, day, p.ID, b, cap)
					}
					if p.Conversion < 0 || p.Conversion > d.Conversion.Cap+1e-9 {
						t.Errorf("seed %d day %d %s: resin %.2f", seed, day, p.ID, p.Conversion)
					}
				}
			}
		}
		if !seen {
			t.Errorf("seed %d: no hive", seed)
		}
	}
}

// Resin follows the convert formula (per_day x the population's share of
// capacity x the resin research) and does not change capacity: floor 1.
func TestHiveResinFormula(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"site": "earth_like"})
	d := hiveDoctrine(t)
	loc := c.Locations["site"]
	cap := d.CapacityAt(loc)
	p := c.newPresence("hive", "hive", d, loc, cap)
	if d.ConvertCap(loc, p) != cap {
		t.Fatalf("resin changed the capacity: %d, %d", d.ConvertCap(loc, p), cap)
	}
	c.hiveTally(d, loc, p, nil)
	if math.Abs(p.Conversion-d.Conversion.PerDay) > 1e-9 {
		t.Errorf("a day at full population: resin %.4f, want %.4f", p.Conversion, d.Conversion.PerDay)
	}
	p.Conversion = 0
	p.KnownTechs = []string{"hive_warrior_brood", "launcher_range_1", "hive_hunter_brood", "launcher_range_2", "hive_resin_flow_1"}
	c.hiveTally(d, loc, p, nil)
	if math.Abs(p.Conversion-1.5*d.Conversion.PerDay) > 1e-9 {
		t.Errorf("with Resin Flow: %.4f, want 1.5x", p.Conversion)
	}
	if d.ConvertCap(loc, p) != cap {
		t.Error("resin changed the capacity")
	}
}

// A new body spends a larva and its egg's biomass; larvae come back at the
// timer's three a day.
func TestHiveTallyDay(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"site": "earth_like"})
	d := hiveDoctrine(t)
	loc := c.Locations["site"]
	p := c.newPresence("hive", "hive", d, loc, 10)
	p.Stage, p.Larvae = 1, 1
	p.Units = map[string]int{"drone": 8, "scout": 2}
	p.Stockpile = map[string]int{"biomass": 40}
	c.hiveTally(d, loc, p, map[string]int{"drone": 2})
	if p.Larvae != 1+3-2 {
		t.Errorf("larvae %d, want 1 + 3 - 2", p.Larvae)
	}
	tags := 1.0
	for _, tg := range loc.CapacityTags() {
		if tg == "biomass" {
			tags += d.Biomass.Tag
		}
	}
	want := int(math.Round(40 + 8*d.Biomass.PerDrone*tags - d.Biomass.Eat*10 - 2*float64(d.Types["drone"].Cost)))
	if p.Stockpile["biomass"] != want {
		t.Errorf("biomass %d, want %d", p.Stockpile["biomass"], want)
	}
}

// The new fields survive a save, and a version 4 save without them loads and
// steps.
func TestHiveFieldsSave(t *testing.T) {
	useRepoData(t)
	c, err := GenerateCampaignOpts("HS", 3, GenOptions{Factions: 1})
	if err != nil {
		t.Fatal(err)
	}
	c.StarDate = 60
	c.StepFactions(nil)
	b, _ := json.Marshal(c)
	var back Campaign
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if err := MigrateCampaign(&back); err != nil {
		t.Fatal(err)
	}
	for id, l := range c.Locations {
		if !reflect.DeepEqual(l.Factions, back.Locations[id].Factions) {
			t.Errorf("%s differs after the round trip", id)
		}
	}
	// Strip the new fields (an older version 4 save): it loads and steps.
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	for _, lv := range raw["locations"].(map[string]any) {
		l := lv.(map[string]any)
		if fs, ok := l["factions"].([]any); ok {
			for _, f := range fs {
				m := f.(map[string]any)
				delete(m, "units")
				delete(m, "larvae")
				for i, k := range m["known_techs"].([]any) {
					if k == "hive_warrior_brood" {
						m["known_techs"].([]any)[i] = "tier_advanced"
					}
				}
			}
		}
	}
	old, _ := json.Marshal(raw)
	var v4 Campaign
	if err := json.Unmarshal(old, &v4); err != nil {
		t.Fatal(err)
	}
	if err := MigrateCampaign(&v4); err != nil || v4.Version != CampaignVersion {
		t.Fatalf("v4 load: %v version %d", err, v4.Version)
	}
	v4.StarDate = 90
	v4.StepFactions(nil)
	for _, l := range v4.Locations {
		for _, p := range l.Factions {
			if p.Doctrine == "hive" {
				if p.UnitsTotal() != p.Population {
					t.Errorf("an old save's units %v, population %d", p.Units, p.Population)
				}
				for _, k := range p.KnownTechs {
					if k == "tier_advanced" {
						t.Error("the alias was not migrated")
					}
				}
			}
		}
	}
}
