package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/storage"
)

// holdCount reads one resource from the ship hold the way the Star Map does.
func holdCount(wm *WorldManager, bp string) int {
	return storage.CountResource(storage.ShipProvider{Level: wm.ShipLevel()}, []string{wm.shipSettlementName()}, bp)
}

// With the toggle off a new expedition is exactly today's game: no tutorial
// progress, no Training Ground, the full hold. With it on, the planet is on the
// map, the ship still parks at the generated start, and the hold's metal is the
// tutorial's ten.
func TestNewExpeditionTutorialToggle(t *testing.T) {
	loadTutorialData(t)
	isolateSettlements(t)

	c, wm, err := newExpedition("Off", 31, false)
	if err != nil {
		t.Fatal(err)
	}
	if c.Tutorial != nil || c.Locations[campaign.TutorialLocationID] != nil {
		t.Fatalf("toggle off still added the tutorial: %+v", c.Tutorial)
	}
	if got := holdCount(wm, "metal_ore"); got != startingResourceQty {
		t.Errorf("hold metal with the tutorial off = %d, want %d", got, startingResourceQty)
	}
	if c.CurrentLocationID != campaign.AdriftLocationID || wm.Current() != nil {
		t.Errorf("a plain expedition starts at %s with level %v, want adrift with none", c.CurrentLocationID, wm.Current())
	}

	c, wm, err = newExpedition("On", 31, true)
	if err != nil {
		t.Fatal(err)
	}
	if c.Tutorial == nil || c.Locations[campaign.TutorialLocationID] == nil {
		t.Fatal("toggle on did not add the tutorial")
	}
	if c.CurrentLocationID != campaign.AdriftLocationID || wm.Current() != nil {
		t.Errorf("tutorial expedition starts at %s with level %v, want adrift with none", c.CurrentLocationID, wm.Current())
	}
	if c.Tutorial.NextLocationID != c.StartID {
		t.Errorf("second destination = %s, want the generated start", c.Tutorial.NextLocationID)
	}
	if c.FuelCost(campaign.TutorialLocationID) >= c.FuelCost(c.StartID) {
		t.Errorf("Training Ground (fuel %d) is not nearer than the start (fuel %d)", c.FuelCost(campaign.TutorialLocationID), c.FuelCost(c.StartID))
	}
	if got := holdCount(wm, "metal_ore"); got != tutorialHoldMetal {
		t.Errorf("hold metal with the tutorial on = %d, want %d", got, tutorialHoldMetal)
	}
	for _, bp := range []string{"crystal", "biomass", "stone", "radioactive_material"} {
		if got := holdCount(wm, bp); got != startingResourceQty {
			t.Errorf("hold %s = %d, want the normal %d", bp, got, startingResourceQty)
		}
	}
	if got := holdCount(wm, "fuel"); got != campaign.GenerationConfig().StartFuel {
		t.Errorf("hold fuel = %d, want %d", got, campaign.GenerationConfig().StartFuel)
	}
	if got := len(wm.ShipColonists()); got != campaign.GenerationConfig().StartColonists {
		t.Errorf("%d crew aboard, want %d", got, campaign.GenerationConfig().StartColonists)
	}
}
