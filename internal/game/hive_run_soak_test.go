package game

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// The hive run soak (phase F step 1, playable_races_plan.md "Balance"): ten
// hive runs started as the title screen starts one (Rival factions 1), the
// player doing nothing: the brood's director runs every rung (assist), the
// first world viewed and live for LP_SOAK_RACE_TICKS (30,000: forty days).
// Then the world is frozen and Tier 1 alone steps the campaign to day
// LP_SOAK_RACE_DAYS (150). A seed fails when the run is lost inside the
// live forty days (the owner's rule: a player who does nothing must not lose
// in forty days); everything else is measured. make soak-hive-run.

type hiveRunRow struct {
	Seed                  int64
	Pop                   [4]int // live bodies at ticks 0, 10k, 20k, end
	Units                 map[string]int
	Laid                  int
	Biomass               int
	Starved               int
	Stage                 int
	SpireTick, LaunchTick int // first (live), -1 never
	LostTick              int // inside the live run, -1 never
	SitesLive             int // worlds held at the end of the live run
	MirrorHostile         bool
	Sites100, SitesEnd    int // worlds held at day 100 and at the end
	FirstLandDay          int // first second world, -1 never
	LostDay, WonDay       int // -1 never
}

func TestHiveRunSoak(t *testing.T) {
	if os.Getenv("LP_SOAK") == "" {
		t.Skip("set LP_SOAK=1 (make soak-hive-run) to run the hive run soak")
	}
	seeds := envInt("LP_SOAK_SEEDS", 10)
	ticks := envInt("LP_SOAK_RACE_TICKS", 30000)
	days := envInt("LP_SOAK_RACE_DAYS", 150)
	raceRunSoak(t, "hive", "hiverun", seeds, ticks, days)
}

// TestAutonzRunSoak is the same soak for an autonz run (phase F step 5):
// the node start, the player doing nothing. make soak-autonz-run.
func TestAutonzRunSoak(t *testing.T) {
	if os.Getenv("LP_SOAK") == "" {
		t.Skip("set LP_SOAK=1 (make soak-autonz-run) to run the autonz run soak")
	}
	raceRunSoak(t, "autonz", "autonzrun", envInt("LP_SOAK_SEEDS", 10), envInt("LP_SOAK_RACE_TICKS", 30000), envInt("LP_SOAK_RACE_DAYS", 150))
}

// TestAutonzRunSoakJoin reads a parallel make soak-autonz-run's rows.
func TestAutonzRunSoakJoin(t *testing.T) {
	var rows []hiveRunRow
	if !soakRowsIn(t, "autonzrun", &rows) {
		t.Skip("set LP_SOAK_ROWS (make soak-autonz-run) to join a parallel autonz run soak")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Seed < rows[j].Seed })
	raceRunSoakCheck(t, "autonz", rows, envInt("LP_SOAK_RACE_TICKS", 30000), envInt("LP_SOAK_RACE_DAYS", 150))
}

func raceRunSoak(t *testing.T, race, kind string, seeds, ticks, days int) {
	t.Helper()
	var rows []hiveRunRow
	for _, run := range soakRuns(seeds, -1, envInt("LP_SOAK_SEED", 1)) {
		row := raceRunSoakOne(t, race, run.seed, ticks, days)
		if soakRowOut(t, kind, fmt.Sprint(run.seed), row) {
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) > 0 {
		raceRunSoakCheck(t, race, rows, ticks, days)
	}
}

// TestHiveRunSoakJoin reads a parallel make soak-hive-run's rows.
func TestHiveRunSoakJoin(t *testing.T) {
	var rows []hiveRunRow
	if !soakRowsIn(t, "hiverun", &rows) {
		t.Skip("set LP_SOAK_ROWS (make soak-hive-run) to join a parallel hive run soak")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Seed < rows[j].Seed })
	raceRunSoakCheck(t, "hive", rows, envInt("LP_SOAK_RACE_TICKS", 30000), envInt("LP_SOAK_RACE_DAYS", 150))
}

func raceRunSoakCheck(t *testing.T, race string, rows []hiveRunRow, ticks, days int) {
	t.Helper()
	t.Log("\n" + raceRunSoakTable(race, rows, ticks, days))
	for _, r := range rows {
		if r.LostTick >= 0 || r.Pop[3] == 0 {
			t.Errorf("seed %d: the %s run was lost (tick %d, bodies at the end %d), inside the live %d", r.Seed, race, r.LostTick, r.Pop[3], ticks)
		}
	}
}

func raceRunSoakOne(t *testing.T, race string, seed int64, ticks, days int) hiveRunRow {
	t.Helper()
	deterministicTaskStops(t)
	c, wm, ms := raceRun(t, race, seed, envInt("LP_SOAK_RACE_FACTIONS", 1))
	loc := c.Locations[c.StartID]
	name := playerSettlementName(loc.ID)
	r := hiveRunRow{Seed: seed, Units: map[string]int{}, SpireTick: -1, LaunchTick: -1, LostTick: -1,
		FirstLandDay: -1, LostDay: -1, WonDay: -1}
	pid := loc.Presence(campaign.PlayerFaction).ID
	bodies := func() []*ecs.Entity { return hiveBodies(ms.level, pid, name) }
	spireBP := "egg_launcher"
	if race != "hive" {
		bodies = func() []*ecs.Entity { return settlementWorkers(ms.level, name) }
		spireBP = "signal_core"
	}
	held0 := func() int { // presences of the player's, dormant ones too
		n := 0
		for _, l := range c.Locations {
			if c.HoldsSite(l) {
				n++
			}
		}
		return n
	}
	r.Pop[0] = len(bodies())
	seenEgg := map[*ecs.Entity]bool{}
	for _, e := range bodies() {
		if strings.HasSuffix(e.Blueprint, "_egg") {
			seenEgg[e] = true
		}
	}
	hunger := map[*ecs.Entity]int{}
	playerFleets := func() int {
		n := 0
		for _, f := range c.Fleets {
			if f.Presence.Faction == campaign.PlayerFaction {
				n++
			}
		}
		return n
	}
	firstLand := func() {
		if r.FirstLandDay < 0 && len(c.PlayerSites()) > 1 {
			r.FirstLandDay = c.Day()
		}
	}
	// The soak's player does only what the director no longer does: once a
	// day it aims every world it holds at the nearest world in reach (the
	// owner, 2026-11-12: the director never spreads by itself).
	aimAll := func() {
		for _, l := range c.Locations {
			if p := l.Presence(campaign.PlayerFaction); p != nil && !p.Dormant && p.LaunchTarget == "" {
				aimNearest(c, l, wm.IsLoaded)
			}
		}
	}
	aimAll()
	progress := soakProgress(fmt.Sprintf("%s:%d", race, seed), ticks)
	for tick := 1; tick <= ticks; tick++ {
		progress(tick)
		if tick%750 == 0 {
			aimAll()
		}
		stepRun(wm, ms, 1)
		if c.Lost {
			r.LostTick = tick
			break
		}
		switch tick {
		case 10000:
			r.Pop[1] = len(bodies())
		case 20000:
			r.Pop[2] = len(bodies())
		}
		if tick%30 != 0 {
			continue
		}
		live := map[*ecs.Entity]bool{}
		for _, e := range bodies() {
			live[e] = true
			if strings.HasSuffix(e.Blueprint, "_egg") && !seenEgg[e] {
				seenEgg[e] = true
				r.Laid++
			}
			if e.HasComponent(components.Needs) {
				hunger[e] = e.GetComponent(components.Needs).(*components.NeedsComponent).Hunger
			}
		}
		for e, h := range hunger {
			if !live[e] {
				if h > 90 && (e.HasComponent(rlcomponents.Dead) || !hasEntity(ms.level, e)) {
					r.Starved++
				}
				delete(hunger, e)
			}
		}
		if r.SpireTick < 0 {
			for _, list := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
				for _, e := range list {
					if e != nil && e.Blueprint == spireBP {
						r.SpireTick = tick
					}
				}
			}
		}
		if r.LaunchTick < 0 && (playerFleets() > 0 || held0() > 1) {
			r.LaunchTick = tick
		}
		firstLand()
	}
	r.Pop[3] = len(bodies())
	for _, e := range bodies() {
		if k := components.KinOf(e); k != nil && k.Type != "" && !strings.HasSuffix(e.Blueprint, "_egg") {
			r.Units[k.Type]++
		}
	}
	if p := loc.Presence(campaign.PlayerFaction); p != nil {
		r.Stage = p.Stage
	}
	r.Biomass = storage.Summarize(storage.LevelProvider{Level: ms.level}, []string{name})["biomass"]
	r.SitesLive = len(c.PlayerSites())
	r.MirrorHostile = c.Stance("mirror") == campaign.StanceHostile
	if c.Lost {
		r.LostDay = c.Day()
		return r
	}
	// Tier 1 alone from here: the viewer leaves and the world freezes.
	wm.EndView()
	if wm.IsLoaded(loc.ID) {
		ms.teardown()
		if err := wm.Freeze(loc, ms); err != nil {
			t.Fatal(err)
		}
	}
	for c.Day() < days && !c.Lost && !c.Won {
		aimAll()
		c.AdvanceDays(1)
		c.StepFactions(wm.IsLoaded)
		wm.checkSpreadEnd()
		firstLand()
		if c.Day() == 100 {
			r.Sites100 = len(c.PlayerSites())
		}
	}
	r.SitesEnd = len(c.PlayerSites())
	if c.Lost {
		r.LostDay = c.Day()
	}
	if c.Won {
		r.WonDay = c.Day()
	}
	return r
}

func raceRunSoakTable(race string, rows []hiveRunRow, ticks, days int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s run soak:", race)
	fmt.Fprintf(&b, " assist (the player only aims), Rival factions %d, %d live ticks then Tier 1 to day %d\n", envInt("LP_SOAK_RACE_FACTIONS", 1), ticks, days)
	b.WriteString("| Seed | Bodies 0 / 10k / 20k / end | Units at the end | Eggs laid | Biomass | Starved | Stage | Spire / first launch tick | Lost tick | Worlds live end / day 100 / end | Second world day | Lost / won day | Mirror hostile |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %d | %d / %d / %d / %d | %s | %d | %d | %d | %d | %s / %s | %s | %d / %d / %d | %s | %s / %s | %v |\n",
			r.Seed, r.Pop[0], r.Pop[1], r.Pop[2], r.Pop[3], countsLine(r.Units), r.Laid, r.Biomass, r.Starved, r.Stage,
			tickCol(r.SpireTick), tickCol(r.LaunchTick), tickCol(r.LostTick), r.SitesLive, r.Sites100, r.SitesEnd,
			tickCol(r.FirstLandDay), tickCol(r.LostDay), tickCol(r.WonDay), r.MirrorHostile)
	}
	return b.String()
}
