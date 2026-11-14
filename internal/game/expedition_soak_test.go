package game

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
)

// The expedition soak (observer runs; docs/developer/factions.md, "Observer
// mode"): ten observer runs at Rival factions 2 (the hive and a mirror,
// whose ship flies by the same table), nobody at the controls. The world
// steps as the dashboard does until the expedition fleet lands on a site;
// then the viewer opens that planet (View) and it steps live, the
// settlement director running the expedition's colony, for the rest of
// LP_SOAK_TICKS. It runs with the observer soak (make soak).

const (
	// More than expSoakMaxMisses seeds that never land, or that end with
	// fewer than expSoakMinAlive of the expedition alive, fail; a jump that
	// leaves the fuel under the reserve fails in any seed.
	expSoakMaxMisses = 2
	expSoakMinAlive  = 3
)

type expSoakRow struct {
	seed            int64
	landTick        int // -1 never
	site, tier      string
	crew, alive     int
	builds, orders  int
	fuelEnd         int
	broke           bool
	mirror          map[string]int // the mirror ships' lift-offs by why
	day             int
	lastMove, where string
}

func TestExpeditionSoak(t *testing.T) {
	if os.Getenv("LP_SOAK") == "" {
		t.Skip("set LP_SOAK=1 (make soak) to run the expedition soak")
	}
	seeds := envInt("LP_SOAK_SEEDS", 10)
	ticks := envInt("LP_SOAK_TICKS", 20000)
	first := envInt("LP_SOAK_SEED", 1)
	var rows []expSoakRow
	for i := 0; i < seeds; i++ {
		rows = append(rows, expSoakOne(t, int64(first+i), ticks))
	}
	t.Log("\n" + expSoakTable(rows, ticks))
	noLand, dying := 0, 0
	for _, r := range rows {
		if r.landTick < 0 {
			noLand++
		}
		if r.alive < expSoakMinAlive {
			dying++
		}
		if r.broke {
			t.Errorf("seed %d: the expedition's fuel fell under the reserve after a jump", r.seed)
		}
	}
	if noLand > expSoakMaxMisses {
		t.Errorf("%d of %d seeds never landed; at most %d may", noLand, len(rows), expSoakMaxMisses)
	}
	if dying > expSoakMaxMisses {
		t.Errorf("%d of %d seeds ended with fewer than %d of the expedition alive; at most %d may", dying, len(rows), expSoakMinAlive, expSoakMaxMisses)
	}
}

func expSoakOne(t *testing.T, seed int64, ticks int) expSoakRow {
	t.Helper()
	deterministicTaskStops(t) // same seed, same run (seedSimRand seeds the rest)
	c, wm := observerRunWorld(t, seed, 2)
	reserve := campaign.PlayerDoctrine(c).Expedition.FuelReserve
	r := expSoakRow{seed: seed, landTick: -1, mirror: map[string]int{}}
	launched := map[string]bool{}
	watch := func() {
		for i := range c.Fleets {
			f := &c.Fleets[i]
			if f.Landed() || f.LastMove == "" {
				continue
			}
			k := fmt.Sprintf("%s@%d", f.ID, f.LaunchDay)
			if launched[k] {
				continue
			}
			launched[k] = true
			if f.IsExpedition() && f.Presence.Fuel < reserve {
				r.broke = true
			}
			if f.Presence.Doctrine == campaign.RivalColonyDoctrine {
				r.mirror[f.LastMove]++
			}
		}
	}
	progress := soakProgress(fmt.Sprintf("expedition:%d", seed), ticks)
	tick := 0
	for ; tick < ticks; tick++ {
		progress(tick)
		flyDashboard(wm, 1, nil)
		watch()
		if f := c.Expedition(); f != nil && f.Landed() && c.Locations[f.LandedID].HasSite() {
			r.landTick, r.site = tick, f.LandedID
			r.crew = f.Presence.Population
			break
		}
	}
	name := campaign.ExpeditionFaction + ":" + r.site
	if r.site != "" {
		r.tier = c.LocationTier(c.Locations[r.site])
		ms, err := wm.ViewLocation(r.site)
		if err != nil {
			t.Fatal(err)
		}
		done := tick
		stepLevel(ms, ticks-tick, func(int) { watch(); done++; progress(done) })
		r.alive = len(settlementWorkers(ms.level, name))
		if dir := ms.factionDirectors[name]; dir != nil {
			for k, v := range dir.Ordered {
				r.orders += v
				if strings.HasPrefix(k, "build:") {
					r.builds += v
				}
			}
			if os.Getenv("LP_SOAK_DEBUG") != "" {
				t.Logf("seed %d director: %v\n%s", seed, dir.Ordered, strings.Join(dir.Log, "\n"))
			}
		}
		ms.openDashboard()
	}
	if f := c.Expedition(); f != nil {
		r.fuelEnd, r.lastMove = f.Presence.Fuel, f.LastMove
		r.where = f.LandedID
		if !f.Landed() {
			r.where = "> " + f.ToID
		}
	}
	r.day = c.Day()
	progress(ticks) // a run that never lands, or lands on no site
	return r
}

func expSoakTable(rows []expSoakRow, ticks int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "expedition soak, %d ticks (land@: tick of the first landing, -1 never; alive: the expedition's colony at the end; mir t/s/f: mirror lift-offs by table or pack-up, stay over, fallback)\n", ticks)
	fmt.Fprintf(&b, "%5s %6s %8s %7s %5s %5s %6s %6s %5s %4s %9s  %s\n",
		"seed", "land@", "site", "tier", "crew", "alive", "builds", "orders", "fuel", "day", "mir t/s/f", "where")
	for _, r := range rows {
		mir := fmt.Sprintf("%d/%d/%d", r.mirror["table"]+r.mirror["pack_up"], r.mirror["stay"], r.mirror["fallback"])
		fmt.Fprintf(&b, "%5d %6d %8s %7s %5d %5d %6d %6d %5d %4d %9s  %s\n",
			r.seed, r.landTick, r.site, r.tier, r.crew, r.alive, r.builds, r.orders, r.fuelEnd, r.day, mir, r.where)
	}
	return b.String()
}
