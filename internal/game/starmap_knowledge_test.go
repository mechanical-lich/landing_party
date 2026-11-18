package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
)

// The Star Map lists only what the player has seen: an unidentified seen
// system as a nameless row whose Travel reads Jump Blind; an unseen one not at
// all. Jumping there blind identifies it on arrival and sees exactly what lies
// within sight of it; a scan from the Star Map identifies what lies in range
// and charges its fuel.
func TestStarMapBlindJumpAndScan(t *testing.T) {
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Blind", 17, false)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDashboardState(c, wm)
	p := d.starMapPanel()
	if len(p.locIDs) != len(c.MapLocations()) || len(p.locIDs) == len(c.Locations) {
		t.Fatalf("listed %d of %d locations, %d seen", len(p.locIDs), len(c.Locations), len(c.MapLocations()))
	}
	for _, id := range p.locIDs {
		if !c.IsSeen(c.Locations[id]) {
			t.Fatalf("unseen %s listed", id)
		}
	}
	// Nothing unseen can be selected or jumped to.
	for _, l := range c.Systems() {
		if c.IsSeen(l) {
			continue
		}
		p.selectByID(l.ID)
		if sel := p.selectedLocation(); sel == l {
			t.Fatalf("unseen %s selectable", l.ID)
		}
		break
	}
	seenBefore := map[string]bool{}
	for _, l := range c.Locations {
		if c.IsSeen(l) {
			seenBefore[l.ID] = true
		}
	}
	// The nearest unidentified system.
	var blind *campaign.Location
	for _, id := range p.locIDs {
		if l := c.Locations[id]; l.HasSite() && !l.Identified {
			blind = l
			break
		}
	}
	if blind == nil {
		t.Fatal("no unidentified system listed")
	}
	p.selectByID(blind.ID)
	p.refreshTravelLock()
	if p.travelBtn.Text != "Jump Blind" || !p.travelBtn.IsEnabled() {
		t.Fatalf("travel button %q enabled %v", p.travelBtn.Text, p.travelBtn.IsEnabled())
	}
	p.travel()
	if c.CurrentLocationID != blind.ID || !blind.Identified {
		t.Fatalf("blind jump: at %s, identified %v (%s)", c.CurrentLocationID, blind.Identified, p.status)
	}
	// (and any travel quest the jump posted put its system on the map)
	named := map[string]bool{}
	for _, q := range c.AvailableQuests() {
		named[q.Location] = true
	}
	// Arriving saw exactly what lies within sight of it.
	for _, l := range c.Locations {
		if !l.GalaxyPoint() {
			continue
		}
		in := (l.X-blind.X)*(l.X-blind.X)+(l.Y-blind.Y)*(l.Y-blind.Y) <= c.SightRange()*c.SightRange()
		if c.IsSeen(l) != (seenBefore[l.ID] || in || named[l.ID]) {
			t.Errorf("%s: seen %v, before %v, in sight %v", l.ID, c.IsSeen(l), seenBefore[l.ID], in)
		}
	}
	// A scan identifies the systems in range for fuel.
	c.UnlockTech("ship_scanner_1")
	p.refreshLocations()
	known, _ := c.IdentifiedCount()
	fuel := holdCount(wm, "fuel")
	p.scan()
	after, _ := c.IdentifiedCount()
	if got := holdCount(wm, "fuel"); got != fuel-c.ScanFuelCost() {
		t.Errorf("fuel %d after a scan, want %d", got, fuel-c.ScanFuelCost())
	}
	cur := c.CurrentLocation()
	inRange := 0
	for _, l := range c.Systems() {
		if dx, dy := l.X-cur.X, l.Y-cur.Y; dx*dx+dy*dy <= campaign.ScanRange(1)*campaign.ScanRange(1) && !l.Identified {
			t.Errorf("%s in range is still unidentified", l.ID)
		}
		if dx, dy := l.X-cur.X, l.Y-cur.Y; dx*dx+dy*dy <= campaign.ScanRange(1)*campaign.ScanRange(1) {
			inRange++
		}
	}
	if after <= known && inRange > 2 {
		t.Errorf("scan identified nothing with %d systems in range", inRange)
	}
	t.Logf("scan identified %d (%q / %q)", after-known, p.scanLabel.Text, p.scanLabel2.Text)
}
