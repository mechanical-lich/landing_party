package history

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
)

// stationSites binds n abandoned_station sites, one per campaign seed, as
// -stats does.
func stationSites(t *testing.T, lib *Library, n int) []struct {
	h    *History
	site *SiteHistory
} {
	t.Helper()
	var out []struct {
		h    *History
		site *SiteHistory
	}
	for s := 1; s <= n; s++ {
		h := New(int64(s) * 7919)
		site, err := h.BindSite(lib, "abandoned_station", h.Seed+131)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, struct {
			h    *History
			site *SiteHistory
		}{h, site})
	}
	return out
}

// No event kept to planets (requires.site: planet) happens on a station, and
// no stamp that needs ground is kept there: it is dropped as no_surface.
func TestStationSitesHaveNoPlanetEvents(t *testing.T) {
	lib := loreLibrary(t)
	if lib.Maps["abandoned_station"].Present.SiteKind() != mapdef.SiteStation {
		t.Fatal("abandoned_station is not a station site")
	}
	planetOnly := 0
	for _, def := range lib.Events {
		if len(def.Requires.Site) > 0 && !Has(def.Requires.Site, mapdef.SiteStation) {
			planetOnly++
		}
	}
	if planetOnly < 5 {
		t.Fatalf("only %d planet-only events", planetOnly)
	}
	for _, s := range stationSites(t, lib, 500) {
		for _, id := range s.site.Events {
			ev := s.h.Event(id)
			if def := lib.Events[ev.Type]; len(def.Requires.Site) > 0 && !Has(def.Requires.Site, mapdef.SiteStation) {
				t.Errorf("%s: planet-only event %s on a station", s.site.ID, ev.Type)
			}
		}
		for _, f := range s.h.Evidence(lib, s.site).Features {
			if script, _ := f.Params["script"].(string); f.Kind == "stamp" && !buildsOffSurface(script) {
				t.Errorf("%s: kept a %s stamp, which needs ground", s.site.ID, script)
			}
		}
	}
}

// Every station site has a plan: the same for the same seed, with its
// purpose's modules; a module two events ask for is built once; a drone
// relay is always a relay with no quarters; every event whose origin names a
// module (module evidence, origin_at) is bound to one that is in the plan.
func TestStationPlans(t *testing.T) {
	lib := loreLibrary(t)
	for _, s := range stationSites(t, lib, 300) {
		ev := s.h.Evidence(lib, s.site)
		plan := ev.Station
		if plan == nil {
			t.Fatalf("%s: no station plan", s.site.ID)
		}
		again := s.h.Evidence(lib, s.site).Station
		if StationPlanText(plan, nil) != StationPlanText(again, nil) {
			t.Errorf("%s: plan differs between two renders", s.site.ID)
		}
		sp, ok := lib.StationPurposes[plan.Purpose]
		if !ok {
			t.Errorf("%s: unknown purpose %q", s.site.ID, plan.Purpose)
			continue
		}
		for _, k := range sp.Modules {
			if plan.Uncrewed && crewOnlyModules[k] {
				continue
			}
			if !plan.HasKind(k) {
				t.Errorf("%s: %s station lacks its %s", s.site.ID, plan.Purpose, k)
			}
		}
		ids := map[string]bool{}
		for _, m := range plan.Modules {
			if ids[m.ID] {
				t.Errorf("%s: module %s twice", s.site.ID, m.ID)
			}
			ids[m.ID] = true
			if _, ok := generation.ModuleKinds[m.Kind]; !ok {
				t.Errorf("%s: unknown module kind %s", s.site.ID, m.Kind)
			}
		}
		if s.site.Arc == "drone_station" {
			if plan.Purpose != "relay" || !plan.Uncrewed || plan.HasKind("quarters") {
				t.Errorf("%s: drone station plan %s", s.site.ID, StationPlanText(plan, nil))
			}
		}
		// A module asked for twice (lab_founded on a research station) is
		// one module: no second of a kind unless the purpose or an expansion
		// added it.
		for _, id := range s.site.Events {
			e := s.h.Event(id)
			def := lib.Events[e.Type]
			if e.Origin == nil {
				continue
			}
			wants := len(def.ModuleEvidence()) > 0
			if wants && !(plan.Uncrewed && crewOnlyModules[def.ModuleEvidence()[0].Params["module"].(string)]) {
				m := ev.OriginModules[id]
				if m == "" || plan.Module(m) == nil {
					t.Errorf("%s: %s asks for a module but its origin is bound to %q", s.site.ID, e.Type, m)
				}
			}
		}
	}
}

// Purposes are rolled by the map's weights among those the arc can found:
// a lost station's shares are within 5 points of the weights.
func TestStationPurposeShares(t *testing.T) {
	lib := loreLibrary(t)
	w := lib.Maps["abandoned_station"].Present.StationPurposes
	total := 0
	for _, v := range w {
		total += v
	}
	got, n := map[string]int{}, 0
	for _, s := range stationSites(t, lib, 500) {
		if s.site.Arc != "lost_station" {
			continue
		}
		got[s.h.Evidence(lib, s.site).Station.Purpose]++
		n++
	}
	for p, v := range w {
		want := 100 * float64(v) / float64(total)
		have := 100 * float64(got[p]) / float64(n)
		if have < want-5 || have > want+5 {
			t.Errorf("purpose %s: %.0f%% of lost stations, want %.0f%% ± 5", p, have, want)
		}
	}
}
