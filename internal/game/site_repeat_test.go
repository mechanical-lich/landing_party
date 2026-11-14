package game

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

// siteRun builds lore_ufo's site (seed 1372: a warship shot down on sys_46),
// beams one colonist down and steps it ticks ticks, returning every living
// body by blueprint and tile after each tick.
func siteRun(t *testing.T, ticks int) []string {
	t.Helper()
	useContentTables(t)
	loadTutorialData(t)
	isolateSettlements(t)
	_, wm, err := newExpeditionOpts("Lore lore_ufo", 1372, false, campaign.GenOptions{Factions: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := wm.Travel("sys_46"); err != nil {
		t.Fatal(err)
	}
	ms := wm.Current()
	defer ms.teardown()
	if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
		t.Fatal(err)
	}
	var out []string
	for i := 0; i < ticks; i++ {
		stepRun(wm, ms, 1) // the game's frame: the pathfinding budget reset first
		var bodies []string
		for _, e := range ms.level.Entities {
			if e == nil || !e.HasComponent(rlcomponents.Position) || e.HasComponent(rlcomponents.Inanimate) {
				continue
			}
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			bodies = append(bodies, fmt.Sprintf("%s@%d,%d,%d", e.Blueprint, pc.GetX(), pc.GetY(), pc.GetZ()))
		}
		sort.Strings(bodies)
		out = append(out, strings.Join(bodies, " "))
	}
	return out
}

// The same site built twice plays out the same: the vmtest lore_ufo's
// origin frame differed between runs of one commit, because the GameMaster's
// spawn pick walked its rules in map order (a worker one run, a heavy the
// next, at the same tile) and the asteroid primer's regions came out in map
// order on ties (so ore veins landed on a different rock, and a wanderer
// walked into ore in one run and open ground in the other).
func TestSameSiteBuiltTwicePlaysTheSame(t *testing.T) {
	a := siteRun(t, 60)
	b := siteRun(t, 60)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("the two runs differ at tick %d:\n%s\n%s", i+1, a[i], b[i])
		}
	}
}
