package game

import (
	"sort"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/mlge/ecs"
)

// onLevel reports whether e is on ms's level.
func onLevel(ms *MainState, e *ecs.Entity) bool {
	for _, x := range ms.level.Entities {
		if x == e {
			return true
		}
	}
	return false
}

// Beam Down lands on the planet in orbit, never on the Star Map's selected
// destination nor on a planet the viewer has open: through a jump away from
// a staffed colony, a View of a third planet, and a jump back. (The owner
// saw three colonists end up on a planet the ship had left, in observer
// mode before v2's redesign; this pins the beam's target.)
func TestBeamDownLandsWhereTheShipIs(t *testing.T) {
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Beam", 9, false)
	if err != nil {
		t.Fatal(err)
	}
	var sites []string
	for id, loc := range c.Locations {
		if loc.Discovered && loc.HasSite() && loc.Kind != campaign.EncounterKind {
			sites = append(sites, id)
		}
	}
	sort.Strings(sites)
	if len(sites) < 3 {
		t.Skip("fewer than three sites")
	}
	a, b, view := sites[0], sites[1], sites[2]
	if err := wm.TravelPaying(a); err != nil {
		t.Fatal(err)
	}
	crew := wm.ShipColonists()
	for _, e := range crew[:3] {
		if err := wm.BeamDown(e); err != nil {
			t.Fatal(err)
		}
	}
	if err := wm.TravelPaying(b); err != nil {
		t.Fatal(err)
	}
	if !wm.IsLoaded(a) || wm.Current() != wm.loaded[b] {
		t.Fatalf("after the jump: a loaded %v, current %q", wm.IsLoaded(a), wm.currentID)
	}
	// The Star Map has the first planet selected; the viewer opens a third.
	d := NewDashboardState(c, wm)
	d.starMapPanel().selectByID(a)
	setObserverMode(true)
	t.Cleanup(func() { observerLock = nil })
	ms, err := wm.ViewLocation(view)
	if err != nil {
		t.Fatal(err)
	}
	ms.openDashboard()
	setObserverMode(false)
	p := d.starMapPanel()
	p.refreshCrew()
	p.shipList.SelectedIndex = 0
	e := p.shipEnts[0]
	p.beamDown()
	if !onLevel(wm.loaded[b], e) {
		t.Fatalf("the Star Map's Beam Down did not land on the planet in orbit (%s): %q", b, p.status)
	}
	if onLevel(wm.loaded[a], e) {
		t.Fatal("beamed onto the selected destination the ship has left")
	}
	if n := countLevelColonists(wm.loaded[a].level, wm.loaded[a].colonyName()); n != 3 {
		t.Errorf("the first colony has %d, want its 3", n)
	}
	// Back to the first: a beam lands there.
	if err := wm.TravelPaying(a); err != nil {
		t.Fatal(err)
	}
	last := wm.ShipColonists()[0]
	if err := wm.BeamDown(last); err != nil {
		t.Fatal(err)
	}
	if !onLevel(wm.loaded[a], last) || !components.IsColonyWorker(last, wm.loaded[a].colonyName()) {
		t.Fatal("after the jump back the beam did not land on the first colony")
	}
	if c.CurrentLocationID != wm.currentID {
		t.Errorf("campaign location %q, world manager's %q", c.CurrentLocationID, wm.currentID)
	}
}
