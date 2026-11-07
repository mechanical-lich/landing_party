package campaign

import "testing"

// Every charted site carries its history's tier for the Star Map, and a save
// from before tiers (no Tier on the location) derives the same one.
func TestContentLocationsCarryTier(t *testing.T) {
	useRepoData(t)
	seen := map[string]int{}
	for seed := int64(1); seed <= 20; seed++ {
		c := genCampaign(t, seed)
		for _, loc := range c.Locations {
			site := c.SiteHistory(loc)
			if site == nil {
				if loc.Tier != "" {
					t.Errorf("seed %d %s: tier %q without a history", seed, loc.ID, loc.Tier)
				}
				continue
			}
			if loc.Tier == "" || loc.Tier != site.Tier {
				t.Errorf("seed %d %s: location tier %q, site tier %q", seed, loc.ID, loc.Tier, site.Tier)
			}
			seen[loc.Tier]++
			old := *loc
			old.Tier = ""
			oldSite := *site
			oldSite.Tier = ""
			c.History.Sites[site.ID] = &oldSite
			if got := c.LocationTier(&old); got != loc.Tier {
				t.Errorf("seed %d %s: an old save derives tier %q, want %q", seed, loc.ID, got, loc.Tier)
			}
			c.History.Sites[site.ID] = site
		}
	}
	for _, tier := range []string{"threat", "hazard", "quiet"} {
		if seen[tier] == 0 {
			t.Errorf("no charted site is %s", tier)
		}
	}
}
