package game

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

// The grey run soak (phase F step 6e, playable_races_plan.md "Step 6"): ten
// grey runs started as the title screen starts one (Rival factions 1), the
// ship flown to the start system and the whole crew of three beamed down,
// then left alone for LP_SOAK_GREY_RUN_TICKS (30,000: forty days) with the
// player doing nothing: no director runs a ship race's crew, so they idle,
// defend themselves and use the doors. A seed fails when the crew is wiped
// out (the owner's rule for a race: a player who does nothing must not
// lose in forty days); losses short of that are measured. make
// soak-grey-run.

type greyRunRow struct {
	Seed      int64
	Pop       [4]int // the crew alive on the site at ticks 0, 10k, 20k, end
	FirstLoss int    // tick, -1 never
	WipedTick int    // tick, -1 never
	Hostiles  int    // most hostile workers and scripted creatures (any stance) seen at once
	Site      string // the start system's map and scenario
}

func TestGreyRunSoak(t *testing.T) {
	if os.Getenv("LP_SOAK") == "" {
		t.Skip("set LP_SOAK=1 (make soak-grey-run) to run the grey run soak")
	}
	ticks := envInt("LP_SOAK_GREY_RUN_TICKS", 30000)
	var rows []greyRunRow
	for _, run := range soakRuns(envInt("LP_SOAK_SEEDS", 10), -1, envInt("LP_SOAK_SEED", 1)) {
		row := greyRunSoakOne(t, run.seed, ticks)
		if soakRowOut(t, "greyrun", fmt.Sprint(run.seed), row) {
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) > 0 {
		greyRunSoakCheck(t, rows, ticks)
	}
}

// TestGreyRunSoakJoin reads a parallel make soak-grey-run's rows.
func TestGreyRunSoakJoin(t *testing.T) {
	var rows []greyRunRow
	if !soakRowsIn(t, "greyrun", &rows) {
		t.Skip("set LP_SOAK_ROWS (make soak-grey-run) to join a parallel grey run soak")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Seed < rows[j].Seed })
	greyRunSoakCheck(t, rows, envInt("LP_SOAK_GREY_RUN_TICKS", 30000))
}

func greyRunSoakCheck(t *testing.T, rows []greyRunRow, ticks int) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "grey run soak: the crew of three beamed down on the start system, the player doing nothing, Rival factions %d, %d live ticks\n", envInt("LP_SOAK_GREY_RUN_FACTIONS", 1), ticks)
	b.WriteString("| Seed | Site | Crew 0 / 10k / 20k / end | First loss tick | Wiped tick | Most creatures at once |\n|---|---|---|---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %d | %s | %d / %d / %d / %d | %s | %s | %d |\n", r.Seed, r.Site, r.Pop[0], r.Pop[1], r.Pop[2], r.Pop[3],
			tickCol(r.FirstLoss), tickCol(r.WipedTick), r.Hostiles)
	}
	t.Log("\n" + b.String())
	for _, r := range rows {
		if r.WipedTick >= 0 || r.Pop[0] == 0 {
			t.Errorf("seed %d: the grey crew was wiped out (tick %d) inside the live %d", r.Seed, r.WipedTick, ticks)
		}
	}
}

func greyRunSoakOne(t *testing.T, seed int64, ticks int) greyRunRow {
	t.Helper()
	deterministicTaskStops(t)
	c, wm := greyRun(t, seed, envInt("LP_SOAK_GREY_RUN_FACTIONS", 1))
	r := greyRunRow{Seed: seed, FirstLoss: -1, WipedTick: -1}
	if err := wm.Travel(c.StartID); err != nil {
		t.Fatalf("seed %d: travel: %v", seed, err)
	}
	ms := wm.Current()
	t.Cleanup(ms.teardown)
	for _, e := range wm.ShipColonists() {
		if err := wm.BeamDown(e); err != nil {
			t.Fatalf("seed %d: beam down: %v", seed, err)
		}
	}
	loc := c.Locations[c.StartID]
	r.Site = loc.MapID + "/" + loc.ScenarioID
	colony := wm.currentColonyName()
	crew := func() int { return countLevelColonists(ms.level, colony) }
	hostiles := func() int {
		n := 0
		for _, e := range ms.level.Entities {
			if e == nil || e.HasComponent(rlcomponents.Dead) {
				continue
			}
			if components.IsHostileWorker(e) || e.HasComponent(components.ScriptedAI) && !e.HasComponent(rlcomponents.Inanimate) && e.HasComponent(rlcomponents.Health) {
				n++
			}
		}
		return n
	}
	r.Pop[0] = crew()
	progress := soakProgress(fmt.Sprintf("greyrun:%d", seed), ticks)
	for tick := 1; tick <= ticks; tick++ {
		progress(tick)
		stepRun(wm, ms, 1)
		switch tick {
		case 10000:
			r.Pop[1] = crew()
		case 20000:
			r.Pop[2] = crew()
		}
		if tick%30 != 0 {
			continue
		}
		n := crew()
		if n < r.Pop[0] && r.FirstLoss < 0 {
			r.FirstLoss = tick
		}
		if n == 0 {
			r.WipedTick = tick
			break
		}
		r.Hostiles = max(r.Hostiles, hostiles())
	}
	r.Pop[3] = crew()
	return r
}
