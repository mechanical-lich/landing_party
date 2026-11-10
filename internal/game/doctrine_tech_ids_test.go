package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/research"
)

// Every id a doctrine's Tier 1 walks (research, assimilation) and its live
// settlement researches is a research.json entry (secondary finding: the
// greyish' three were nowhere).
func TestDoctrineTechIDsResolve(t *testing.T) {
	useContentTables(t)
	fd := campaign.LoadFactionData()
	for key, d := range fd.Doctrines {
		for _, walk := range [][]campaign.DoctrineTech{d.Research, d.Assimilation} {
			for _, r := range walk {
				if _, ok := research.GetTech(r.ID); !ok {
					t.Errorf("%s: Tier 1 tech %q is not in research.json", key, r.ID)
				}
			}
		}
		if d.Settlement != nil {
			for _, id := range d.Settlement.Research {
				if _, ok := research.GetTech(id); !ok {
					t.Errorf("%s: settlement research %q is not in research.json", key, id)
				}
			}
		}
	}
}
