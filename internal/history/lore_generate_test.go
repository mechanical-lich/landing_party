package history

import (
	"encoding/json"
	"math/rand"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/scenario"
)

// loreBindMany binds n sites cycling through the history maps, with site seeds
// derived the way charting derives them (campaign seed + GenSeq*131).
func loreBindMany(t testing.TB, lib *Library, h *History, from, n int) []*SiteHistory {
	t.Helper()
	maps := historyMaps(lib)
	var out []*SiteHistory
	for i := from; i < from+n; i++ {
		site, err := h.BindSite(lib, maps[i%len(maps)], h.Seed+int64(i+1)*131)
		if err != nil {
			t.Fatalf("seed %d site %d: %v", h.Seed, i, err)
		}
		out = append(out, site)
	}
	return out
}

func loreJSON(t testing.TB, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The same seed and binding order give the same history, and a saved and
// reloaded history goes on exactly as the original would have.
func TestLoreSameSeedSameHistoryAcrossSaveLoad(t *testing.T) {
	lib := loreLibrary(t)
	for _, seed := range []int64{1, 42, 987654321} {
		a, b := New(seed), New(seed)
		loreBindMany(t, lib, a, 0, 6)
		loreBindMany(t, lib, b, 0, 6)
		if loreJSON(t, a) != loreJSON(t, b) {
			t.Fatalf("seed %d: two runs differ", seed)
		}
		var loaded History
		if err := json.Unmarshal([]byte(loreJSON(t, a)), &loaded); err != nil {
			t.Fatal(err)
		}
		loreBindMany(t, lib, a, 6, 6)
		loreBindMany(t, lib, &loaded, 6, 6)
		if loreJSON(t, a) != loreJSON(t, &loaded) {
			t.Errorf("seed %d: history diverged after save and load", seed)
		}
	}
}

// Every arc a map lists can end in that map's present.
func TestLoreEveryListedArcReachesPresent(t *testing.T) {
	lib := loreLibrary(t)
	for _, mapID := range historyMaps(lib) {
		md := lib.Maps[mapID]
		for _, arcID := range md.Present.Arcs {
			arc := lib.Arcs[arcID]
			ok := false
			for seed := int64(1); seed <= 300 && !ok; seed++ {
				h := New(seed)
				h.RollPool(lib)
				w, hh, _ := md.Size.Roll(seed)
				s := &siteSim{h: h, lib: lib, md: md, rng: rand.New(rand.NewSource(seed)), seed: seed, arc: arc,
					state: InitialState(), outcomes: map[string]string{}, claimed: map[int]bool{}, mapW: w, mapH: hh}
				ok = s.walk() && arc.Ends.Holds(s.state) && presentHolds(md.Present, s.state)
			}
			if !ok {
				t.Errorf("map %s: arc %s never ends in its present", mapID, arcID)
			}
		}
	}
}

// loreSweep binds sites for many campaign seeds.
func loreSweep(t testing.TB, lib *Library, seeds, perSeed int) ([]*History, []*SiteHistory) {
	var hs []*History
	var sites []*SiteHistory
	for seed := int64(1); seed <= int64(seeds); seed++ {
		h := New(seed * 7919)
		sites = append(sites, loreBindMany(t, lib, h, 0, perSeed)...)
		hs = append(hs, h)
	}
	return hs, sites
}

// Every scenario turns up on at least one map that supports it, and no site
// plays a scenario its map doesn't support or ends outside its present.
func TestLoreEveryScenarioReachable(t *testing.T) {
	lib := loreLibrary(t)
	_, sites := loreSweep(t, lib, 120, 16)
	seen := map[string]bool{}
	for _, s := range sites {
		sc := scenario.ByID(s.Scenario)
		if sc == nil || !sc.SupportsMap(s.Map) {
			t.Errorf("site on %s plays %q, which doesn't support it", s.Map, s.Scenario)
			continue
		}
		if !presentHolds(lib.Maps[s.Map].Present, s.State) {
			t.Errorf("site on %s ended as %+v, outside its present", s.Map, s.State)
		}
		seen[s.Scenario] = true
	}
	for _, sc := range scenario.AllEnabled() {
		if sc.ID != "tutorial" && !seen[sc.ID] {
			t.Errorf("scenario %s never came up", sc.ID)
		}
	}
}

// Uninhabited sites have no residents, figures, invented civs or population
// events; the lifeless arc's planets never had life.
func TestLoreLifelessArcsStayEmpty(t *testing.T) {
	lib := loreLibrary(t)
	hs, _ := loreSweep(t, lib, 80, 16)
	checked := 0
	for _, h := range hs {
		for _, s := range h.Sites {
			if !lib.Arcs[s.Arc].Uninhabited {
				continue
			}
			checked++
			if s.State.Population != PopNever || len(s.Outcomes) > 0 {
				t.Errorf("%s (%s): population %s, outcomes %v", s.ID, s.Arc, s.State.Population, s.Outcomes)
			}
			if s.Arc == "lifeless" && s.State.Life != LifeNone {
				t.Errorf("%s: lifeless arc ended with life %s", s.ID, s.State.Life)
			}
			for _, id := range s.Events {
				ev := h.Event(id)
				def := lib.Events[ev.Type]
				if len(ev.Figures) > 0 || populates(def) {
					t.Errorf("%s (%s): event %s puts people here", s.ID, s.Arc, ev.Type)
				}
				for _, c := range ev.Civs {
					if !Has(h.Pool, c) {
						t.Errorf("%s (%s): event %s invents civ %s", s.ID, s.Arc, ev.Type, c)
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no uninhabited sites generated")
	}
}

// About a quarter of sites are uninhabited, and every pool size 1-10 occurs.
func TestLoreLifelessShareAndPoolSizes(t *testing.T) {
	lib := loreLibrary(t)
	hs, sites := loreSweep(t, lib, 300, 12)
	un := 0
	for _, s := range sites {
		if lib.Arcs[s.Arc].Uninhabited {
			un++
		}
	}
	share := float64(un) / float64(len(sites))
	if share < 0.22 || share > 0.28 {
		t.Errorf("uninhabited share %.3f over %d sites, want near 0.25", share, len(sites))
	}
	sizes := map[int]bool{}
	fates := map[string]int{}
	for _, h := range hs {
		sizes[len(h.Pool)] = true
		for _, c := range h.Civs {
			fates[c.Fate]++
		}
	}
	for n := 1; n <= 10; n++ {
		if !sizes[n] {
			t.Errorf("no campaign rolled a pool of %d civs", n)
		}
	}
	for _, f := range []string{FateExtant, FateExtinct, FateFled} {
		if fates[f] == 0 {
			t.Errorf("no civ ever ended %s", f)
		}
	}
}

// A chapter is the last event and carries the site's scenario; the quiet
// outcome plays the map's base scenario; wreck threats grow from the wreck.
func TestLoreChapterEndsTheHistory(t *testing.T) {
	lib := loreLibrary(t)
	hs, _ := loreSweep(t, lib, 60, 16)
	wrecks := 0
	for _, h := range hs {
		for _, s := range h.Sites {
			if s.Chapter == "" {
				if s.Scenario != lib.Maps[s.Map].BaseScenario {
					t.Errorf("%s: quiet site plays %s", s.ID, s.Scenario)
				}
				continue
			}
			if s.Events[len(s.Events)-1] != s.Chapter {
				t.Errorf("%s: chapter is not the last event", s.ID)
			}
			ch := h.Event(s.Chapter)
			def := lib.Events[ch.Type]
			if def.Scenario != s.Scenario {
				t.Errorf("%s: chapter %s plays %s, site says %s", s.ID, ch.Type, def.Scenario, s.Scenario)
			}
			if len(def.OriginFrom) > 0 {
				src := h.Event(ch.Causes[0])
				if !Has(def.OriginFrom, src.Type) || src.Origin == nil || ch.Origin == nil || *src.Origin != *ch.Origin {
					t.Errorf("%s: %s does not grow from its source's origin (%s)", s.ID, ch.Type, src.Type)
				}
				if src.Type == "ancient_wreck" {
					wrecks++
				}
			}
			if ch.Date >= 0 {
				t.Errorf("%s: chapter dated %d, want the recent past", s.ID, ch.Date)
			}
		}
	}
	if wrecks == 0 {
		t.Error("no wreck_* chapter came up")
	}
}
