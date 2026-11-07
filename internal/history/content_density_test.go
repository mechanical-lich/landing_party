package history

import (
	"sort"
	"strings"
	"testing"
)

type contentBand struct{ lo, hi float64 }

// The plan's density targets (lore_content_plan.md §2), events per site.
var (
	contentPopBands = map[string]contentBand{
		PopNever: {4, 6}, PopOutpost: {6, 8}, PopColony: {8, 10}, PopCivilization: {10, 14},
	}
	contentLayeredMax = 16.0
	contentMapBands   = map[string]contentBand{
		"asteroid": {5, 7}, "ocean_planet": {7, 10}, "abandoned_station": {6, 8}, "moon": {6, 8},
		"alien": {8, 11}, "desert_planet": {7, 10}, "earth_like": {9, 12}, "frozen_planet": {8, 11},
	}
)

func contentMean(v []int) float64 {
	t := 0
	for _, x := range v {
		t += x
	}
	return float64(t) / float64(max(len(v), 1))
}

func contentP90(v []int) int {
	s := append([]int(nil), v...)
	sort.Ints(s)
	return s[len(s)*9/10]
}

// Events per site (happenings: a per-side pair counts once) hit the plan's
// bands per population (sites without an older layer) and per map; a layered civilization stays under 16; the 90th
// percentile stays within three of each band's top.
func TestContentDensityTargets(t *testing.T) {
	lib := loreLibrary(t)
	byPop, byMap := map[string][]int{}, map[string][]int{}
	var layered []int
	for m, sites := range contentSites(t, lib, 500, false) {
		for _, s := range sites {
			n := s.h.Happenings(lib, s.SiteHistory)
			byMap[m] = append(byMap[m], n)
			if s.Layer != "" {
				if s.State.Population == PopCivilization {
					layered = append(layered, n)
				}
				continue
			}
			byPop[s.State.Population] = append(byPop[s.State.Population], n)
		}
	}
	for p, b := range contentPopBands {
		got := contentMean(byPop[p])
		if got < b.lo || got > b.hi {
			t.Errorf("%s: %.2f events per site, want %.0f-%.0f", p, got, b.lo, b.hi)
		}
		if p90 := contentP90(byPop[p]); float64(p90) > b.hi+3 {
			t.Errorf("%s: p90 %d events, want at most %.0f", p, p90, b.hi+3)
		}
	}
	if got := contentMean(layered); len(layered) == 0 || got > contentLayeredMax {
		t.Errorf("layered civilizations: %.2f events per site over %d sites, want at most %.0f", got, len(layered), contentLayeredMax)
	}
	for m, b := range contentMapBands {
		if got := contentMean(byMap[m]); got < b.lo || got > b.hi {
			t.Errorf("%s: %.2f events per site, want %.0f-%.0f", m, got, b.lo, b.hi)
		}
	}
}

// A chronicle reads: no event follows one of its own type unless repeatable
// (or the other side's half of a per-side event),
// an interlude type happens once per site,
// it spans under 600 years (2,500 with an older layer), interludes never
// stamp, and at most a third of interlude types take an origin.
func TestContentChronicleReadable(t *testing.T) {
	lib := loreLibrary(t)
	for m, sites := range contentSites(t, lib, 300, false) {
		for _, s := range sites {
			prev := ""
			oldest := StarDate(0)
			interludes := map[string]bool{}
			for _, id := range s.Events {
				ev := s.h.Event(id)
				if def := lib.Events[ev.Type]; def.Interlude && !def.Repeatable {
					if interludes[ev.Type] {
						t.Errorf("%s %s: interlude %s twice", m, s.ID, ev.Type)
					}
					interludes[ev.Type] = true
				}
				if ev.Type == prev && !lib.Events[ev.Type].Repeatable && !lib.Events[ev.Type].PerSide {
					t.Errorf("%s %s: %s twice in a row", m, s.ID, ev.Type)
				}
				prev = ev.Type
				if ev.Date < oldest {
					oldest = ev.Date
				}
			}
			limit := 600
			if s.Layer != "" {
				limit = 2500
			}
			if years := int(-oldest) / 365; years >= limit {
				t.Errorf("%s %s (layer %q): spans %d years, want under %d", m, s.ID, s.Layer, years, limit)
			}
		}
	}
	interludes, withOrigin := 0, 0
	for _, e := range lib.Interludes() {
		interludes++
		if e.Origin || len(e.OriginFrom) > 0 {
			withOrigin++
		}
		for _, ev := range e.Evidence {
			if ev.Kind != "lore_datapad" {
				t.Errorf("interlude %s leaves a %s", e.ID, ev.Kind)
			}
		}
		if e.IsChapter() {
			t.Errorf("interlude %s is a chapter", e.ID)
		}
	}
	if interludes == 0 || withOrigin*3 > interludes {
		t.Errorf("%d of %d interlude types take an origin; want at most a third", withOrigin, interludes)
	}
}

// A site with an older layer names it: an event built on its ruins fills
// {former} with a civ of that site, and an ancestor link resolves.
func TestContentLayersConnect(t *testing.T) {
	lib := loreLibrary(t)
	layered, named, ancestors := 0, 0, 0
	for _, sites := range contentSites(t, lib, 300, false) {
		for _, s := range sites {
			if s.Layer == "" {
				continue
			}
			layered++
			names := map[string]bool{}
			for _, id := range s.Events {
				for _, c := range s.h.Event(id).Civs {
					names[s.h.Civs[c].Name] = true
					if a := s.h.Civs[c].Ancestor; a != "" {
						if s.h.Civs[a] == nil {
							t.Errorf("%s: %s descends from unknown civ %s", s.ID, c, a)
						}
						ancestors++
					}
				}
			}
			for _, id := range s.Events {
				ev := s.h.Event(id)
				if f, ok := ev.Vars["former"]; ok {
					if !names[f] {
						t.Errorf("%s: %s names former %q, not a civ of the site", s.ID, ev.Type, f)
					}
					named++
					if !strings.Contains(s.h.Render(lib, ev, "X"), f) {
						t.Errorf("%s: %s does not render its former civ", s.ID, ev.Type)
					}
				}
			}
		}
	}
	if layered == 0 || named == 0 || ancestors == 0 {
		t.Errorf("layered sites %d, {former} fills %d, ancestor links %d; want each above zero", layered, named, ancestors)
	}
}
