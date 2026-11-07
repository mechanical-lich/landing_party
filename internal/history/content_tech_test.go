package history

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

// No event fires outside its tech gates; pool civs are spacefaring and
// natives never are; yeti natives never pass medieval; a native's recorded
// level is the highest any of its events reached; every stamp a site keeps
// names a real script (an "@role" resolved) and each structure's relic
// belongs to its people's level.
func TestContentTechGates(t *testing.T) {
	lib := loreLibrary(t)
	tdOf := map[string]*TechDef{}
	for i := range lib.TechDefs {
		tdOf[lib.TechDefs[i].ID] = &lib.TechDefs[i]
	}
	natives, levels := 0, map[string]int{}
	for m, sites := range contentSites(t, lib, 300, false) {
		for _, s := range sites {
			for _, id := range s.Events {
				ev := s.h.Event(id)
				def := lib.Events[ev.Type]
				lvl, ok := ev.Vars["tech"]
				if len(ev.Civs) > 0 && !ok {
					t.Errorf("%s %s: %s has a civ but no tech", m, s.ID, ev.Type)
				}
				// A new native is not gated; an overlord's gates read the
				// people it comes down on, not its own level.
				if !ok || def.Civ == CivNative || def.Civ == CivTwins || def.Civ == CivRivalNative || def.Civ == CivRivalVisitor {
					continue
				}
				if r := def.Requires; r.TechMin != "" && TechIndex(lvl) < TechIndex(r.TechMin) || r.TechMax != "" && TechIndex(lvl) > TechIndex(r.TechMax) {
					t.Errorf("%s %s: %s fired at %s (gates %q-%q)", m, s.ID, ev.Type, lvl, def.Requires.TechMin, def.Requires.TechMax)
				}
				civ := s.h.Civs[ev.Civs[0]]
				if Has(s.h.Pool, civ.ID) && lvl != TechSpacefaring {
					t.Errorf("%s %s: pool civ %s acted at %s", m, s.ID, civ.Name, lvl)
				}
			}
			log.SetOutput(&bytes.Buffer{})
			evd := s.h.Evidence(lib, s.SiteHistory)
			log.SetOutput(os.Stderr)
			stampAt := map[string]string{}
			for _, f := range evd.Features {
				if f.Kind == "stamp" {
					sc, _ := f.Params["script"].(string)
					if strings.HasPrefix(sc, "@") || sc == "" {
						t.Errorf("%s %s: stamp script %q unresolved", m, s.ID, sc)
					}
					lvl, _ := f.Params["tech"].(string) // a civ-less hazard has none
					stampAt[f.At] = lvl
				}
			}
			allRelics := map[string]bool{}
			for _, td := range lib.TechDefs {
				for _, r := range td.Relics {
					allRelics[r] = true
				}
			}
			for _, f := range evd.Features {
				bp, _ := f.Params["blueprint"].(string)
				if f.Kind != "scatter_entity" || stampAt[f.At] == "" || !allRelics[bp] {
					continue
				}
				if td := tdOf[stampAt[f.At]]; td == nil || !Has(td.Relics, f.Params["blueprint"].(string)) {
					t.Errorf("%s %s: relic %v at a %s structure", m, s.ID, f.Params["blueprint"], stampAt[f.At])
				}
			}
		}
	}
	hs := map[*History]bool{}
	for _, sites := range contentSites(t, lib, 300, false) {
		for _, s := range sites {
			hs[s.h] = true
		}
	}
	for h := range hs {
		for _, c := range h.Civs {
			if Has(h.Pool, c.ID) {
				if c.Tech != TechSpacefaring {
					t.Errorf("pool civ %s is %s", c.Name, c.Tech)
				}
				continue
			}
			natives++
			levels[c.Tech]++
			if TechIndex(c.Tech) < 0 || TechIndex(c.Tech) > TechIndex(TechAtomic) {
				t.Errorf("native %s is %q", c.Name, c.Tech)
			}
			if c.Species == "yeti" && TechIndex(c.Tech) > TechIndex(TechMedieval) {
				t.Errorf("yeti people %s reached %s", c.Name, c.Tech)
			}
		}
	}
	for _, l := range []string{TechPrimitive, TechMedieval, TechIndustrial, TechAtomic} {
		if levels[l] == 0 {
			t.Errorf("no native people reached exactly %s (%d natives: %v)", l, natives, levels)
		}
	}
}

// Every tech role resolves to a real script or drops cleanly at every level,
// every relic is a blueprint, and natives can start where civs.json says.
func TestContentTechData(t *testing.T) {
	lib := loreLibrary(t)
	bps := loreBlueprintText(t)
	if len(lib.TechDefs) != len(TechLevels) {
		t.Fatalf("tech.json has %d levels, want %d", len(lib.TechDefs), len(TechLevels))
	}
	for i, td := range lib.TechDefs {
		if td.ID != TechLevels[i] || td.Label == "" || len(td.Weapons) == 0 || len(td.Relics) == 0 {
			t.Errorf("level %d: %+v", i, td)
		}
		for _, r := range td.Relics {
			if !strings.Contains(bps, `"`+r+`"`) {
				t.Errorf("%s relic %s is no blueprint", td.ID, r)
			}
		}
	}
	for l := range lib.Civs.NativeTech {
		if i := TechIndex(l); i < 0 || i > TechIndex(TechAtomic) {
			t.Errorf("native_tech %q", l)
		}
	}
	if TechIndex(lib.Civs.SpeciesTechMax["yeti"]) != TechIndex(TechMedieval) {
		t.Errorf("yeti cap %q, want medieval", lib.Civs.SpeciesTechMax["yeti"])
	}
}

// An industrial or atomic people can rebuild: over 300 seeds some sites keep a
// city block and some a missile silo, each on a few percent of sites at most,
// and each stands at an industrial-or-later (silo: atomic) people's origin.
func TestContentHigherTechDwellings(t *testing.T) {
	lib := loreLibrary(t)
	sites := 0
	have := map[string]int{}
	for _, list := range contentSites(t, lib, 300, false) {
		for _, s := range list {
			sites++
			log.SetOutput(&bytes.Buffer{})
			evd := s.h.Evidence(lib, s.SiteHistory)
			log.SetOutput(os.Stderr)
			on := map[string]bool{}
			for _, f := range evd.Features {
				sc, _ := f.Params["script"].(string)
				if f.Kind != "stamp" || (sc != "city_block" && sc != "missile_silo") {
					continue
				}
				on[sc] = true
				lvl, _ := f.Params["tech"].(string)
				minLvl := TechIndustrial
				if sc == "missile_silo" {
					minLvl = TechAtomic
				}
				if TechIndex(lvl) < TechIndex(minLvl) || lvl == TechSpacefaring {
					t.Errorf("%s: %s at %q", s.ID, sc, lvl)
				}
			}
			for sc := range on {
				have[sc]++
			}
		}
	}
	for _, sc := range []string{"city_block", "missile_silo"} {
		share := 100 * float64(have[sc]) / float64(sites)
		if have[sc] == 0 || share > 5 {
			t.Errorf("%s on %d of %d sites (%.1f%%), want some and at most 5%%", sc, have[sc], sites, share)
		}
	}
}
