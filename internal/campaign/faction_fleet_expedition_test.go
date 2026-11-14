package campaign

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// mirrorFlight steps a New Expedition with two rival factions (the hive and
// a mirror ship) for days of Tier 1 and returns one line per mirror ship
// per day, the table-driven lift-offs, and the fuel bounds broken.
func mirrorFlight(t *testing.T, seed int64, days int) (lines []string, moves map[string]int, broke []string) {
	t.Helper()
	c, err := GenerateCampaignOpts("Fleet", seed, GenOptions{Factions: 2})
	if err != nil {
		t.Fatal(err)
	}
	tbl := LoadFactionData().Doctrine("mirror").Expedition
	moves = map[string]int{}
	launched := map[string]bool{}
	for day := 1; day <= days; day++ {
		c.StarDate = history.StarDate(day)
		c.StepFactions(nil)
		for i := range c.Fleets {
			f := &c.Fleets[i]
			if f.Presence.Doctrine != RivalColonyDoctrine {
				continue
			}
			p := &f.Presence
			lines = append(lines, fmt.Sprintf("d%d %s landed %s to %s fuel %d pop %d move %s", day, f.ID, f.LandedID, f.ToID, p.Fuel, p.Population, f.LastMove))
			if p.Fuel < 0 || p.Fuel > tbl.FleetFuel {
				broke = append(broke, lines[len(lines)-1])
			}
			key := fmt.Sprintf("%s@%d", f.ID, f.LaunchDay)
			if !f.Landed() && !launched[key] && f.LaunchDay == history.StarDate(day) {
				launched[key] = true
				moves[f.LastMove]++
				if p.Fuel < tbl.FuelReserve {
					broke = append(broke, "under the reserve after a jump: "+lines[len(lines)-1])
				}
			}
		}
	}
	return lines, moves, broke
}

// A mirror ship flies by the colony's expedition table: over 300 days the
// same campaign seed moves it the same way, its fuel never goes below the
// reserve after a jump nor past its tanks, and at least one lift-off in the
// seeds is the table's own move rather than the stay_days fallback.
func TestMirrorFleetFliesByTheTable(t *testing.T) {
	useRepoData(t)
	all := map[string]int{}
	for seed := int64(1); seed <= 4; seed++ {
		a, moves, broke := mirrorFlight(t, seed, 300)
		b, _, _ := mirrorFlight(t, seed, 300)
		if strings.Join(a, "\n") != strings.Join(b, "\n") {
			t.Fatalf("seed %d: two flights differ", seed)
		}
		for _, l := range broke {
			t.Errorf("seed %d: %s", seed, l)
		}
		for k, v := range moves {
			all[k] += v
		}
		t.Logf("seed %d lift-offs %v", seed, moves)
	}
	if all["table"]+all["pack_up"] == 0 {
		t.Errorf("no table-driven lift-off in four seeds: %v", all)
	}
}

// An observer run's expedition is a colony-doctrine fleet on the same
// table: from the Wreck Site it lifts off on its first day ("launch"),
// lands on sites (visiting them), never spends past the reserve, and the
// same seed moves it the same way over 300 days.
func TestExpeditionFleetFliesByTheTable(t *testing.T) {
	useRepoData(t)
	fly := func(seed int64) (string, int, []string) {
		c, err := GenerateCampaignOpts("Obs", seed, GenOptions{Factions: 2, Observer: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.AddExpeditionFleet(6, map[string]int{"metal_ore": 100, "crystal": 100}, 400); err != nil {
			t.Fatal(err)
		}
		tbl := PlayerDoctrine(c).Expedition
		var lines, broke []string
		landings, prev := 0, ""
		for day := 1; day <= 300; day++ {
			c.StarDate = history.StarDate(day)
			c.StepFactions(nil)
			f := c.Expedition()
			if f == nil {
				lines = append(lines, fmt.Sprintf("d%d gone", day))
				break
			}
			if day == 1 && (f.Landed() || f.LastMove != "launch") {
				broke = append(broke, fmt.Sprintf("did not launch off the Wreck Site on day 1: %+v", f))
			}
			state := fmt.Sprintf("%s>%s", f.LandedID, f.ToID)
			if state != prev {
				prev = state
				lines = append(lines, fmt.Sprintf("d%d %s fuel %d pop %d move %s", day, state, f.Presence.Fuel, f.Presence.Population, f.LastMove))
				if f.Landed() {
					landings++
					if !c.Locations[f.LandedID].Visited {
						broke = append(broke, "landed without visiting "+f.LandedID)
					}
				} else if f.Presence.Fuel < tbl.FuelReserve {
					broke = append(broke, fmt.Sprintf("d%d under the reserve after a jump: %d", day, f.Presence.Fuel))
				}
			}
		}
		return strings.Join(lines, "\n"), landings, broke
	}
	for seed := int64(1); seed <= 4; seed++ {
		a, landings, broke := fly(seed)
		b, _, _ := fly(seed)
		if a != b {
			t.Fatalf("seed %d: two flights differ", seed)
		}
		for _, l := range broke {
			t.Errorf("seed %d: %s", seed, l)
		}
		if landings == 0 {
			t.Errorf("seed %d: the expedition never landed: %s", seed, a)
		}
		t.Logf("seed %d: %d landings\n%s", seed, landings, a)
	}
}
