package history

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/generation"
)

// A site over its lore budget keeps the chapter's evidence and the newest
// events' and drops the oldest, logging each drop; the drops keep their
// origins and datapads.
func TestContentBudgetDropsOldestAndLogs(t *testing.T) {
	lib := loreLibrary(t)
	h := New(1)
	site := &SiteHistory{ID: "site1", Map: "earth_like", Seed: 5}
	const reactors = 10
	for i := 1; i <= reactors+1; i++ {
		typ := "reactor_built"
		if i == reactors+1 {
			typ = "old_reactor_leak" // the chapter: a hazard
		}
		ev := Event{ID: fmt.Sprintf("ev%d", i), Type: typ, Site: site.ID, Origin: &SitePoint{X: 20 * i, Y: 40}}
		h.Events = append(h.Events, ev)
		site.Events = append(site.Events, ev.ID)
	}
	site.Chapter = site.Events[reactors]
	w, _, _ := lib.Maps[site.Map].Size.Roll(site.Seed)
	class := generation.Budget().For(w)
	if class.Stamps >= reactors {
		t.Fatalf("budget allows %d stamps; the test needs fewer than %d", class.Stamps, reactors)
	}

	var buf bytes.Buffer
	log.SetOutput(&buf)
	ev := h.Evidence(lib, site)
	log.SetOutput(os.Stderr)

	kept := map[string]bool{}
	for _, f := range ev.Features {
		kept[strings.TrimPrefix(f.At, "origin:")] = true
	}
	if !kept[site.Chapter] {
		t.Errorf("the chapter's evidence was trimmed")
	}
	for i := 1; i <= reactors; i++ {
		id := fmt.Sprintf("ev%d", i)
		wantKept := i > reactors-class.Stamps
		if kept[id] != wantKept {
			t.Errorf("%s (reactor %d of %d): kept %v, want %v", id, i, reactors, kept[id], wantKept)
		}
		if _, ok := ev.Origins[id]; !ok {
			t.Errorf("%s lost its origin", id)
		}
	}
	if len(ev.Datapads) != reactors+1 {
		t.Errorf("%d datapads, want one per event (%d)", len(ev.Datapads), reactors+1)
	}
	if got := len(ev.Dropped); got != reactors-class.Stamps {
		t.Errorf("%d dropped, want %d", got, reactors-class.Stamps)
	}
	for _, d := range ev.Dropped {
		if !strings.Contains(buf.String(), "dropped the stamp abandoned_reactor of reactor_built "+d.Event+": over the stamps limit") {
			t.Errorf("no log line for dropping %s; log:\n%s", d.Event, buf.String())
		}
	}
	if ev.KeepOut[site.Chapter] != generation.Budget().HazardKeepOut || ev.KeepOut["ev1"] != generation.Budget().KeepOut {
		t.Errorf("keep-outs %v: want the hazard's %d, a ruin's %d", ev.KeepOut, generation.Budget().HazardKeepOut, generation.Budget().KeepOut)
	}
}

// Across the 500-seed sweep no site's kept evidence breaks its budget:
// radiation and creatures never, structures only through the chapter's own.
func TestContentBudgetHolds(t *testing.T) {
	lib := loreLibrary(t)
	b := generation.Budget()
	for m, sites := range contentSites(t, lib, 500, false) {
		for _, s := range sites {
			log.SetOutput(&bytes.Buffer{})
			ev := s.h.Evidence(lib, s.SiteHistory)
			log.SetOutput(os.Stderr)
			w, hh, _ := lib.Maps[m].Size.Roll(s.Seed)
			class := b.For(w)
			add := func(e *evidenceCost, c evidenceCost) {
				e.stamps += c.stamps
				e.footprint += c.footprint
				e.underground += c.underground
				e.hostiles += c.hostiles
				e.radiation += c.radiation
			}
			var spent, chapter evidenceCost
			for _, f := range ev.Features {
				add(&spent, costOf(f))
			}
			if s.Chapter != "" {
				for _, e := range lib.Events[s.h.Event(s.Chapter).Type].Evidence {
					if e.Kind != "lore_datapad" {
						add(&chapter, costOf(e.FeatureBlock))
					}
				}
			}
			// A chapter's structure always stays (the smallest maps can run
			// over on footprint through it); counts never run over.
			over := func(got, chap, limit int) bool { return got > limit && got > chap }
			if over(spent.stamps, chapter.stamps, class.Stamps) || over(spent.underground, chapter.underground, class.Underground) ||
				spent.hostiles > class.Hostiles || spent.radiation > class.Radiation ||
				over(spent.footprint, chapter.footprint, int(class.FootprintPercent/100*float64(w*hh))) {
				t.Errorf("%s %s: evidence %+v over the budget %+v", m, s.ID, spent, class)
			}
		}
	}
}

// The data's lore_budget parses and has a class for every size.
func TestContentBudgetData(t *testing.T) {
	b, err := generation.LoadLoreBudget(loreDataRoot + "/generation.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Classes) != 3 || b.Classes[len(b.Classes)-1].MaxWidth != 0 || b.LandingHalf <= 0 || b.KeepOut <= 0 || b.HazardKeepOut < b.KeepOut {
		t.Errorf("lore_budget %+v", b)
	}
	if b.For(512).Stamps != 7 || b.For(100).Spacing != 20 {
		t.Errorf("size classes: large %+v, small %+v", b.For(512), b.For(100))
	}
}
