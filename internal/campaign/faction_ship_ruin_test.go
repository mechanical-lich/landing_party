package campaign

import "testing"

// Leaving the mothership's orbit (docked, or just built) is not a lift-off:
// no ruin and no departure there. A ship that lands on a site again and again
// before the level is built leaves one ruin there per presence, not one per
// visit (review finding 2: 61 ruins at the mothership in 300 days).
func TestGreyishRuinsOnlyWhereItLanded(t *testing.T) {
	c, _, _, ms := greyTestCampaign(t)
	ms.Presence.Stockpile = map[string]int{"metal_ore": 400, "crystal": 160}
	mother := c.Locations["mother"]
	for day := 0; day < 300; day++ {
		c.StarDate++
		c.StepFactions(nil)
	}
	if n := len(mother.Ruins); n != 0 {
		t.Errorf("%d ruins at the mothership's orbit site, want 0", n)
	}
	for _, ev := range c.History.Events {
		if ev.Type == "faction_departed" && ev.Site == mother.ID {
			t.Errorf("faction_departed at the mothership's site: %+v", ev)
			break
		}
	}
	landed := 0
	for _, loc := range c.sortedLocations() {
		seen := map[string]bool{}
		for _, r := range loc.Ruins {
			landed++
			if !r.Stamped && seen[r.Presence] {
				t.Errorf("%s: two unstamped ruins of %s", loc.ID, r.Presence)
			}
			seen[r.Presence] = !r.Stamped
		}
	}
	if landed == 0 {
		t.Error("no ruin anywhere: the ships never landed and left (fixture)")
	}
}
