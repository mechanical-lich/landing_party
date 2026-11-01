package game

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
)

// A new expedition starts adrift: no level in orbit, crew aboard, the ship
// ticking. Beam Down and To Site are refused with a reason; Travel to the
// nearest system works and lands normally; the total-wipe check does not fire.
func TestAdriftStart(t *testing.T) {
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Adrift", 17, false)
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsAdrift() || wm.Current() != nil || len(wm.loaded) != 0 {
		t.Fatalf("start: adrift=%v current=%v loaded=%d", c.IsAdrift(), wm.Current(), len(wm.loaded))
	}
	crew := campaign.GenerationConfig().StartColonists
	if len(wm.ShipColonists()) != crew || wm.TotalColonists() != crew {
		t.Fatalf("crew: aboard %d total %d, want %d", len(wm.ShipColonists()), wm.TotalColonists(), crew)
	}

	// The ship still ticks while adrift; nothing else is loaded to tick.
	before := wm.shipLevel.tick
	for i := 0; i < 10; i++ {
		wm.TickBackground(nil)
	}
	if wm.shipLevel.tick != before+10 {
		t.Errorf("ship ticked %d times, want 10", wm.shipLevel.tick-before)
	}

	d := NewDashboardState(c, wm)
	p := d.starMapPanel()
	p.beamDown()
	if !strings.Contains(p.status, "Adrift") {
		t.Errorf("Beam Down while adrift: %q", p.status)
	}
	if err := wm.BeamDown(wm.ShipColonists()[0]); err == nil {
		t.Error("BeamDown succeeded with no level")
	}
	d.returnToSite()
	if !strings.Contains(d.status, "Adrift") || d.done {
		t.Errorf("To Site while adrift: status=%q done=%v", d.status, d.done)
	}
	if _, err := wm.EnterCurrent(); err == nil || !strings.Contains(err.Error(), "adrift") {
		t.Errorf("EnterCurrent while adrift: %v", err)
	}
	if err := wm.BeamResourceDown("metal_ore", 1); err == nil {
		t.Error("Beam Resources Down succeeded with no level")
	}
	d.returnSiteBtn.SetEnabled(wm.Current() != nil)
	if d.returnSiteBtn.IsEnabled() {
		t.Error("To Site enabled while adrift")
	}
	// The wreck itself is a listed row that cannot be travelled to.
	p.selectByID(campaign.AdriftLocationID)
	p.refreshTravelLock()
	if p.travelLock != "" {
		t.Errorf("the current wreck row reports %q", p.travelLock)
	}

	// Saving while adrift writes a campaign with no site level.
	t.Chdir(t.TempDir())
	if err := wm.SaveCampaign(wm.Current()); err != nil {
		t.Fatalf("save while adrift: %v", err)
	}

	// Travel to the start system: fuel is charged, a level is generated and
	// landed on normally.
	fuel := holdCount(wm, "fuel")
	cost := c.FuelCost(campaign.StartLocationID)
	if cost <= 0 {
		t.Fatalf("start costs %d", cost)
	}
	p.selectByID(campaign.StartLocationID)
	p.refreshTravelLock()
	if !p.travelBtn.IsEnabled() {
		t.Fatalf("Travel to the start locked: %s", p.travelLock)
	}
	p.travel()
	if c.CurrentLocationID != campaign.StartLocationID || wm.Current() == nil || c.IsAdrift() {
		t.Fatalf("travel: current=%s level=%v status=%q", c.CurrentLocationID, wm.Current(), p.status)
	}
	if got := holdCount(wm, "fuel"); got != fuel-cost {
		t.Errorf("fuel %d after the jump, want %d", got, fuel-cost)
	}
	if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
		t.Fatalf("beam down after arriving: %v", err)
	}
	ms, err := wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ms.teardown)
	ms.checkTotalWipe()
	if c.Lost {
		t.Error("total wipe fired with the crew alive")
	}

	// Jumping back to the wreck is refused on the Star Map.
	p.selectByID(campaign.AdriftLocationID)
	p.refreshTravelLock()
	if p.travelLock == "" || p.travelBtn.IsEnabled() {
		t.Errorf("Travel back to the wreck allowed: lock=%q", p.travelLock)
	}
	p.travel()
	if c.CurrentLocationID != campaign.StartLocationID {
		t.Errorf("travel() moved the ship to %s", c.CurrentLocationID)
	}
}

// An old-format campaign whose current location is a planet still loads and
// parks that planet, exactly as before the adrift start existed.
func TestOldCampaignWithPlanetCurrentStillLoads(t *testing.T) {
	loadTutorialData(t)
	isolateSettlements(t)
	c := &campaign.Campaign{Name: "Old", Seed: 5, CurrentLocationID: "sys_1", Locations: map[string]*campaign.Location{
		"home":  {ID: "home", Name: "Home", Kind: campaign.HomeKind, Discovered: true, X: 300},
		"sys_1": {ID: "sys_1", Name: "Old Start", Kind: "planet", MapID: "tutorial", ScenarioID: "tutorial", Seed: 9, Discovered: true},
	}}
	c.BindQuests()
	wm := NewWorldManager(c)
	wm.SeedShipCrew(2)
	if err := wm.EnterStartingOrbit(); err != nil {
		t.Fatal(err)
	}
	if c.IsAdrift() || wm.Current() == nil || wm.Current().locationID != "sys_1" {
		t.Fatalf("old campaign: adrift=%v current=%v", c.IsAdrift(), wm.Current())
	}
	if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
		t.Fatalf("beam down on the old start: %v", err)
	}
	if c.Locations[campaign.AdriftLocationID] != nil {
		t.Error("loading an old campaign invented a Wreck Site")
	}
}
