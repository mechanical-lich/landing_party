package campaign

import (
	"math"
	"testing"
)

// Every campaign starts adrift at the Wreck Site: an origin point with no map
// or scenario and nothing to land on. The generated start system is the nearest
// and cheapest destination, well within the starting fuel.
func TestGenerateStartsAdrift(t *testing.T) {
	useRepoData(t)
	cfg := genConfig()
	for seed := int64(1); seed <= 40; seed++ {
		c := genCampaign(t, seed)
		cur := c.CurrentLocation()
		if cur == nil || cur.Kind != AdriftKind || cur.ID != AdriftLocationID || !c.IsAdrift() {
			t.Fatalf("seed %d: current = %+v, want the Wreck Site", seed, cur)
		}
		if cur.X != 0 || cur.Y != 0 || cur.MapID != "" || cur.ScenarioID != "" || !cur.Discovered || cur.HasSite() {
			t.Errorf("seed %d: wreck site = %+v", seed, cur)
		}
		start := c.Locations[StartLocationID]
		if start == nil || !start.HasSite() || !start.Discovered {
			t.Fatalf("seed %d: start = %+v", seed, start)
		}
		cost := c.FuelCost(StartLocationID)
		if cost != int(math.Round(cfg.StartRadius)) {
			t.Errorf("seed %d: first jump costs %d, want %v", seed, cost, cfg.StartRadius)
		}
		if cost > cfg.StartFuel/10 {
			t.Errorf("seed %d: first jump %d is more than a tenth of the %d fuel", seed, cost, cfg.StartFuel)
		}
		for id, loc := range c.Locations {
			if id == AdriftLocationID || id == StartLocationID {
				continue
			}
			if loc.Discovered && c.FuelCost(id) <= cost {
				t.Errorf("seed %d: %s (fuel %d) is as near as the start (fuel %d)", seed, id, c.FuelCost(id), cost)
			}
		}
		// Quests belong to the start system, not to the wreck.
		startQuests := 0
		for _, q := range c.QuestDefs {
			if q.Location == AdriftLocationID {
				t.Errorf("seed %d: quest %s bound to the Wreck Site", seed, q.ID)
			}
			if q.Location == StartLocationID {
				startQuests++
			}
		}
		if startQuests == 0 {
			t.Errorf("seed %d: the start system has no quests", seed)
		}
	}
}

// Rolls that pick an existing system never pick the Wreck Site, and scanning
// from it works: it has coordinates, so distance and placement are defined.
func TestAdriftIsNeverATarget(t *testing.T) {
	useRepoData(t)
	for seed := int64(1); seed <= 15; seed++ {
		c := genCampaign(t, seed)
		for i := 0; i < 60; i++ {
			c.MaybeTravelQuests()
		}
		c.Expand(10)
		for _, q := range c.QuestDefs {
			if q.Location == AdriftLocationID {
				t.Fatalf("seed %d: quest %s posted to the Wreck Site", seed, q.ID)
			}
		}
		if len(c.Locations[AdriftLocationID].Fixtures) != 0 {
			t.Fatalf("seed %d: fixtures on the Wreck Site", seed)
		}
		if _, ran := c.Scan(3); !ran {
			t.Fatalf("seed %d: scan from the Wreck Site did not run", seed)
		}
		if c.TutorialTravelLock(StartLocationID) != "" {
			t.Fatalf("seed %d: lock reported with no tutorial", seed)
		}
	}
}
