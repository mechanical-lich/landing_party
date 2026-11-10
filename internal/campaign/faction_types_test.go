package campaign

import (
	"math"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// The hive's types are data: six of them, two morphs, each with its colour
// block; the stage mixes name types; the fallbacks and aliases resolve.
func TestHiveTypesLoad(t *testing.T) {
	useRepoData(t)
	d := hiveDoctrine(t)
	if len(d.Types) != 6 {
		t.Fatalf("%d types, want 6", len(d.Types))
	}
	for k, ty := range d.Types {
		if d.Morphs[ty.Morph] == nil || ty.SpriteDX%144 != 0 || ty.SeenName == "" || ty.Name == "" || ty.Health <= 0 || ty.HatchTurns <= 0 {
			t.Errorf("type %s: %+v", k, ty)
		}
	}
	for m, fb := range d.MorphFallback {
		if ty := d.Types[fb]; ty == nil || ty.Morph != m || ty.Unlock != "" {
			t.Errorf("fallback of %s is %s, want a starting type of that morph", m, fb)
		}
	}
	for i, st := range d.Stages {
		for k := range st.Mix {
			if d.Types[k] == nil {
				t.Errorf("stage %d mix names %q, not a type", i, k)
			}
		}
	}
	open := d.TypesUnlocked(nil)
	if !open["drone"] || !open["scout"] || len(open) != 2 {
		t.Errorf("unlocked at the start: %v, want drone and scout", open)
	}
	open = d.TypesUnlocked([]string{"tier_advanced", "tier_elite"})
	if !open["warrior"] || !open["hunter"] || open["burrower"] {
		t.Errorf("an old save's tier techs unlock %v, want warrior and hunter", open)
	}
}

// Locked types give their share to their morph's fallback, so the
// crawler/flier ratio per stage is Phase B's (nest 0.8/0.2, hive 0.6/0.4)
// whatever is known.
func TestTypeMixFallsBackByMorph(t *testing.T) {
	useRepoData(t)
	d := hiveDoctrine(t)
	loc := &Location{ScenarioID: "earth_like"}
	for _, techs := range [][]string{nil, {"tier_advanced"}, {"tier_advanced", "tier_elite"}} {
		for stage, want := range map[int]float64{1: 0.8, 2: 0.6} {
			mix := d.TypeMix(stage, loc, techs)
			open := d.TypesUnlocked(techs)
			crawl := 0.0
			for k, v := range mix {
				if !open[k] {
					t.Errorf("techs %v stage %d: locked %s in the mix", techs, stage, k)
				}
				if d.Types[k].Morph == "crawler" {
					crawl += v
				}
			}
			if math.Abs(crawl-want) > 1e-9 {
				t.Errorf("techs %v stage %d: crawlers %.2f, want %.2f", techs, stage, crawl, want)
			}
		}
	}
	if mix := d.TypeMix(1, loc, nil); math.Abs(mix["drone"]-0.8) > 1e-9 || math.Abs(mix["scout"]-0.2) > 1e-9 {
		t.Errorf("a fresh nest's mix %v, want drone 0.8 scout 0.2", mix)
	}
}

// Growth adds to the type most under the mix, starvation takes the type most
// over it with the drones last, a shot takes drones first, a landed brood is
// drones.
func TestUnitsCompositionRules(t *testing.T) {
	useRepoData(t)
	d := hiveDoctrine(t)
	loc := &Location{ScenarioID: "earth_like"}
	p := &FactionPresence{Stage: 1, KnownTechs: []string{"tier_advanced"}}
	d.AddUnits(p, loc, 10)
	// nest with warriors: drone .6, warrior .2, scout .2
	if p.Units["drone"] != 6 || p.Units["warrior"] != 2 || p.Units["scout"] != 2 {
		t.Errorf("10 grown: %v, want 6 drones 2 warriors 2 scouts", p.Units)
	}
	d.StarveUnits(p, loc, 3)
	if p.Units["drone"] != 6 || p.UnitsTotal() != 7 {
		t.Errorf("3 starved: %v, want the drones kept", p.Units)
	}
	p.Units = map[string]int{"drone": 3, "scout": 2, "warrior": 4}
	d.ShotUnits(p, loc, 4)
	if p.Units["drone"] != 0 || p.Units["scout"] != 1 || p.Units["warrior"] != 4 {
		t.Errorf("shot of 4: %v, want the 3 drones then a scout", p.Units)
	}
	q := &FactionPresence{}
	d.AddEconomyUnits(q, 2)
	if q.Units["drone"] != 2 || len(q.Units) != 1 {
		t.Errorf("landed brood: %v", q.Units)
	}
}

// Over 300 days of Tier 1 every presence's units sum to its population, and
// its types are among the unlocked ones.
func TestUnitsTrackPopulation(t *testing.T) {
	useRepoData(t)
	for seed := int64(1); seed <= 4; seed++ {
		c, err := GenerateCampaignOpts("Units", seed, GenOptions{Factions: 2})
		if err != nil {
			t.Fatal(err)
		}
		fd := LoadFactionData()
		for day := 10; day <= 300; day += 10 {
			c.StarDate = history.StarDate(day)
			c.StepFactions(nil)
			for _, l := range c.sortedLocations() {
				for i := range l.Factions {
					p := &l.Factions[i]
					d := fd.Doctrine(p.Doctrine)
					if !d.HasTypes() {
						if p.Units != nil {
							t.Fatalf("seed %d day %d: untyped %s has units %v", seed, day, p.Doctrine, p.Units)
						}
						continue
					}
					if p.UnitsTotal() != p.Population {
						t.Fatalf("seed %d day %d %s: units %v sum %d, population %d", seed, day, p.ID, p.Units, p.UnitsTotal(), p.Population)
					}
					open := d.TypesUnlocked(p.KnownTechs)
					for k := range p.Units {
						if !open[k] {
							t.Errorf("seed %d day %d %s: locked %s", seed, day, p.ID, k)
						}
					}
				}
			}
		}
	}
}
