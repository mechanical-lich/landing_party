package game

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
)

func countShown(c *campaign.Campaign) (presences, fleets int) {
	for _, loc := range c.Locations {
		presences += len(shownPresences(loc))
	}
	return presences, len(shownFleets(c))
}

func countKnown(c *campaign.Campaign) (presences int) {
	for _, loc := range c.Locations {
		for _, p := range loc.Factions {
			if p.Known {
				presences++
			}
		}
	}
	return presences
}

// factionsText is the Factions tab's rows as text.
func factionsText(p *factionsPanel) string {
	var b strings.Builder
	for _, r := range p.rows {
		fmt.Fprintln(&b, r.text)
	}
	return b.String()
}

// The reveal is a rule of the view keyed on the observer run: the same seed
// as a normal campaign shows only what the player has seen (nothing at day
// 0, the morph's name), as an observer run every presence and fleet (the
// faction's name too, with populations) and every faction event in
// Legends; the records' Known flags are the same either way.
func TestRevealOnlyInObserverRun(t *testing.T) {
	cn, wmn := expeditionWorld(t, 777)
	np, nf := countShown(cn)
	nKnown := countKnown(cn)
	if np != nKnown || nf != len(cn.KnownFleets()) {
		t.Fatalf("a normal campaign shows %d presences and %d fleets, knows %d and %d", np, nf, nKnown, len(cn.KnownFleets()))
	}
	dn := NewDashboardState(cn, wmn)
	fpn := &factionsPanel{c: cn, wm: wmn, d: dn}
	fpn.rebuild()
	if txt := factionsText(fpn); strings.Contains(txt, "HIVE") || !strings.Contains(txt, "No faction seen yet.") {
		t.Errorf("a normal campaign's Factions tab shows the unseen:\n%s", txt)
	}

	co, wmo := observerRunWorld(t, 777, 1)
	op, of := countShown(co)
	total := 0
	for _, loc := range co.Locations {
		total += len(loc.Factions)
	}
	if op != total || of != len(co.Fleets) || op == 0 {
		t.Fatalf("an observer run shows %d of %d presences and %d of %d fleets", op, total, of, len(co.Fleets))
	}
	if k := countKnown(co); k != nKnown {
		t.Errorf("the observer run marked presences known in the record: %d, a normal campaign %d", k, nKnown)
	}
	do := NewDashboardState(co, wmo)
	fpo := &factionsPanel{c: co, wm: wmo, d: do}
	fpo.rebuild()
	txt := factionsText(fpo)
	if !strings.Contains(txt, "HIVE") || !strings.Contains(txt, "EXPEDITION") {
		t.Fatalf("the observer run's Factions tab lacks the hive or the expedition:\n%s", txt)
	}
	// Legends: every faction event in the run.
	h := co.History
	hidden := 0
	for i := range h.Events {
		ev := &h.Events[i]
		if strings.HasPrefix(ev.Type, "faction_") && !h.Known[ev.ID] {
			hidden++
			if !eventShown(h, ev) {
				t.Fatalf("faction event %s hidden in an observer run", ev.ID)
			}
		}
	}
	observerRunActive = false
	for i := range h.Events {
		ev := &h.Events[i]
		if !h.Known[ev.ID] && eventShown(h, ev) {
			t.Fatalf("event %s shown outside an observer run", ev.ID)
		}
	}
	observerRunActive = true
	t.Logf("observer run: %d presences, %d fleets, %d unknown faction events shown\n%s", op, of, hidden, txt)
}

// A Factions tab site row selects that site on the Star Map tab.
func TestFactionsTabSelectsSite(t *testing.T) {
	c, wm := observerRunWorld(t, 777, 1)
	d := NewDashboardState(c, wm)
	d.selectTabByID("factions")
	fp, ok := d.tabs[d.active].panel.(*factionsPanel)
	if !ok {
		t.Fatal("no Factions tab")
	}
	fp.Enter()
	var site string
	for _, r := range fp.rows {
		if r.locID != "" && c.Locations[r.locID].HasSite() {
			site = r.locID
			break
		}
	}
	if site == "" {
		t.Fatalf("no site row:\n%s", factionsText(fp))
	}
	fp.selectSite(site)
	if d.tabs[d.active].id != "starmap" {
		t.Fatalf("the click left the tab on %s", d.tabs[d.active].id)
	}
	if loc := d.starMapPanel().selectedLocation(); loc == nil || loc.ID != site {
		t.Fatalf("the Star Map selected %v, want %s", loc, site)
	}
}
