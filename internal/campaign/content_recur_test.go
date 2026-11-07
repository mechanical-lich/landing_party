package campaign

import "testing"

// Denser histories keep pool civs recurring: across 100 charted campaigns
// (grown as play grows them), at least 60% of pool civs seen on a charted
// site appear on two or more.
func TestContentCivsRecur(t *testing.T) {
	useRepoData(t)
	seen, twice := 0, 0
	for seed := int64(1); seed <= 100; seed++ {
		c := genCampaign(t, seed*7919)
		c.Expand(4)
		for i := 0; i < 12; i++ {
			c.MaybeTravelQuests()
		}
		for _, id := range c.History.Pool {
			n := 0
			for _, sid := range c.History.Civs[id].Sites {
				if s := c.History.Sites[sid]; s != nil && s.Location != "" {
					n++
				}
			}
			if n > 0 {
				seen++
			}
			if n > 1 {
				twice++
			}
		}
	}
	if seen == 0 || twice*100 < seen*60 {
		t.Errorf("%d of %d pool civs recur on two or more charted sites, want at least 60%%", twice, seen)
	}
}
