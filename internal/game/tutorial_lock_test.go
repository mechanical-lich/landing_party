package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
)

// A locked destination disables the Travel button, and travel() neither moves
// the ship nor spends fuel; an unlocked one travels as before.
func TestStarMapTravelLock(t *testing.T) {
	wm := bgWorld(t, []string{"a", "b", "c"})
	bgTravel(t, wm, "a")
	c := wm.Campaign
	for _, loc := range c.Locations {
		loc.Discovered = true // the Star Map lists discovered locations only
	}
	c.Tutorial = &campaign.TutorialProgress{NextLocationID: "c", Unlocked: []string{"b"}}
	wm.StockShipLocker(50, map[string]int{})
	fuel := holdCount(wm, "fuel")

	p := newStarMapPanel(c, wm)
	p.selectByID("c")
	p.refreshTravelLock()
	if p.travelLock == "" || p.travelBtn.IsEnabled() {
		t.Fatalf("locked destination: lock=%q enabled=%v", p.travelLock, p.travelBtn.IsEnabled())
	}
	p.travel()
	if c.CurrentLocationID != "a" || wm.Current() != wm.loaded["a"] {
		t.Errorf("travel() moved the ship to %s", c.CurrentLocationID)
	}
	if got := holdCount(wm, "fuel"); got != fuel {
		t.Errorf("fuel %d after a refused jump, want %d", got, fuel)
	}
	if p.status != p.travelLock {
		t.Errorf("status = %q, want the lock reason %q", p.status, p.travelLock)
	}

	p.selectByID("b")
	p.refreshTravelLock()
	if p.travelLock != "" || !p.travelBtn.IsEnabled() {
		t.Fatalf("unlocked destination: lock=%q enabled=%v", p.travelLock, p.travelBtn.IsEnabled())
	}
	p.travel()
	if c.CurrentLocationID != "b" {
		t.Errorf("travel() to the unlocked planet left the ship at %s", c.CurrentLocationID)
	}

	// The current location is never locked, and Skip lifts the lock at once.
	p.selectByID("b")
	p.refreshTravelLock()
	if p.travelLock != "" {
		t.Errorf("current location locked: %q", p.travelLock)
	}
	c.Tutorial.Skipped = true
	p.selectByID("c")
	p.refreshTravelLock()
	if p.travelLock != "" || !p.travelBtn.IsEnabled() {
		t.Errorf("after Skip: lock=%q enabled=%v", p.travelLock, p.travelBtn.IsEnabled())
	}
}

// An incident on a staffed planet can always be answered: TravelToIncident
// bypasses the lock.
func TestTravelToIncidentBypassesLock(t *testing.T) {
	wm := bgWorld(t, []string{"a", "b"})
	bgTravel(t, wm, "a")
	c := wm.Campaign
	c.Tutorial = &campaign.TutorialProgress{NextLocationID: "a", Unlocked: []string{"a"}}
	if c.TutorialTravelLock("b") == "" {
		t.Fatal("b should be locked")
	}
	wm.incidents = []Incident{{LocationID: "b", Where: "b", Victim: "v"}}
	ms, err := wm.TravelToIncident()
	if err != nil {
		t.Fatalf("TravelToIncident: %v", err)
	}
	t.Cleanup(ms.teardown)
	if c.CurrentLocationID != "b" || ms != wm.loaded["b"] {
		t.Errorf("incident travel ended at %s", c.CurrentLocationID)
	}
}
