package campaign

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

const greyishPaceGolden = "testdata/greyish_pace.golden"

// greyishPaceTable is the greyish Tier 1 pace (greyish_plan.md, step 0):
// seeds 1-6 at Rival factions 3, no player, no level, the campaign stepped
// day by day to 300. On days 40, 100, 200 and 300 it lists every greyish
// ship (tier, state, crew, hold, spine techs), the mothership's hoard and
// the landings so far. Crew is the count only and techs only the doctrine's
// spine, so the table is blind to bookkeeping later phases add (types, a
// second walk) and moves only when Tier 1's pace does.
func greyishPaceTable(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	cols := map[int]bool{40: true, 100: true, 200: true, 300: true}
	for seed := int64(1); seed <= 6; seed++ {
		c, err := GenerateCampaignOpts("Pace", seed, GenOptions{Factions: 3})
		if err != nil {
			t.Fatal(err)
		}
		d := LoadFactionData().Doctrine("greyish")
		if d == nil {
			t.Fatal("no greyish doctrine")
		}
		fmt.Fprintf(&b, "seed %d\n", seed)
		landed := map[string]history.StarDate{} // fleet -> the landing day last seen
		landings := 0
		count := func() {
			for _, f := range c.ShipFleets("greyish") {
				if f.Landed() {
					if day, ok := landed[f.ID]; !ok || day != f.LandedDay {
						landed[f.ID] = f.LandedDay
						landings++
					}
				}
			}
		}
		count() // the seeded scout
		for day := 1; day <= 300; day++ {
			c.StarDate = history.StarDate(day)
			c.StepFactions(nil)
			count()
			if !cols[day] {
				continue
			}
			fmt.Fprintf(&b, "  d%d landings %d\n", day, landings)
			for _, f := range c.ShipFleets("greyish") {
				fmt.Fprintf(&b, "    %s\n", c.ShipLine(d, f))
			}
		}
	}
	return b.String()
}

// TestGreyishPaceRivalThree pins the greyish Tier 1 pace. greyish_plan.md's
// hard rule: no step of phase E4 moves it, so the table stays byte-identical.
// go test ./internal/campaign -run GreyishPace -update-golden rewrites it
// (only for a change the owner decided to make to the pace).
func TestGreyishPaceRivalThree(t *testing.T) {
	useRepoData(t)
	got := greyishPaceTable(t)
	if *updateGolden {
		if err := os.WriteFile(greyishPaceGolden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(greyishPaceGolden)
	if err != nil {
		t.Fatalf("%v (go test -run GreyishPace -update-golden writes it)", err)
	}
	if got != string(want) {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := 0; i < max(len(gl), len(wl)); i++ {
			var g, w string
			if i < len(gl) {
				g = gl[i]
			}
			if i < len(wl) {
				w = wl[i]
			}
			if g != w {
				t.Fatalf("the greyish Tier 1 pace moved (line %d):\n got  %q\n want %q\nfull table:\n%s", i+1, g, w, got)
			}
		}
	}
}
