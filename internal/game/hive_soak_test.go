package game

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// The hive soak (phase E2, hive_plan.md "Balance"): ten New Expeditions,
// each with a hive presence of eight at the nest stage (drones and scouts,
// 30 biomass in its pool) put on the start system and one colonist beamed
// down so the level is live, the stance forced neutral so it builds rather
// than raids; its director runs the settlement for LP_SOAK_HIVE_TICKS
// (30,000: 40 days) headless, stepped as the other soaks are. Runs with
// LP_SOAK=1 (make soak) after the autonz soak.

const (
	hiveSoakPop = 8
	// A seed misses when its resin rose by less than hiveSoakResin, its
	// population ended below its start, it laid no egg, or more than one
	// body starved. More than soakMaxMisses seeds missing fails; the other
	// columns are measured, not required.
	hiveSoakResin = 0.05
)

type hiveRow struct {
	Seed                            int64
	Resin                           [4]float64
	Pop0, PopEnd, CapEnd            int
	Tier1                           int
	Units                           map[string]int
	Laid                            map[string]int
	PeakLarvae, Biomass             int
	Chops, Hunts, Hauls             int
	FirstEvolve                     int
	Techs                           []string
	StageTick, SpireTick, ReadyTick int
	LaunchTick                      int
	Launches                        int
	Starved                         int
	Cancelled                       int
	Stage                           int
}

func TestHiveSoak(t *testing.T) {
	if os.Getenv("LP_SOAK") == "" {
		t.Skip("set LP_SOAK=1 (make soak) to run the hive soak")
	}
	seeds := envInt("LP_SOAK_SEEDS", 10)
	ticks := envInt("LP_SOAK_HIVE_TICKS", 30000)
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	var rows []hiveRow
	for _, run := range soakRuns(seeds, -1, envInt("LP_SOAK_SEED", 1)) {
		row := hiveSoakOne(t, run.seed, ticks)
		if soakRowOut(t, "hive", fmt.Sprint(run.seed), row) {
			continue // one process of make soak-hive: the join applies the thresholds
		}
		rows = append(rows, row)
	}
	if len(rows) > 0 {
		hiveSoakCheck(t, rows, ticks)
	}
}

// TestHiveSoakJoin reads the rows a parallel make soak-hive wrote
// (LP_SOAK_ROWS) and prints and checks them as one run.
func TestHiveSoakJoin(t *testing.T) {
	var rows []hiveRow
	if !soakRowsIn(t, "hive", &rows) {
		t.Skip("set LP_SOAK_ROWS (make soak-hive) to join a parallel hive soak")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Seed < rows[j].Seed })
	hiveSoakCheck(t, rows, envInt("LP_SOAK_HIVE_TICKS", 30000))
}

func hiveSoakCheck(t *testing.T, rows []hiveRow, ticks int) {
	t.Helper()
	t.Log("\n" + hiveSoakTable(rows, ticks))
	misses := 0
	for _, r := range rows {
		laid := 0
		for _, n := range r.Laid {
			laid += n
		}
		if r.Resin[3]-r.Resin[0] < hiveSoakResin || r.PopEnd < r.Pop0 || laid == 0 || r.Starved > 1 {
			misses++
		}
	}
	if misses > soakMaxMisses {
		t.Errorf("%d of %d seeds missed (resin +%.2f, population held, an egg laid, at most one starved); at most %d may",
			misses, len(rows), hiveSoakResin, soakMaxMisses)
	}
}

// tier1Pop is the population Tier 1 reaches from pop over days on loc (the
// daily step's growth with no shot), for the comparison column.
func tier1Pop(d *campaign.Doctrine, loc *campaign.Location, pop, days int) int {
	cap := d.CapacityAt(loc)
	rate := d.Growth.PerDay * d.HazardFactor(loc.ScenarioID)
	g := 0.0
	for i := 0; i < days; i++ {
		if pop > cap {
			g -= d.Growth.StarvePerDay * float64(pop)
		} else {
			g += rate * float64(pop)
		}
		for g >= 1 {
			pop++
			g--
		}
		for g <= -1 {
			pop--
			g++
		}
		if pop >= cap {
			pop, g = cap, 0
		}
	}
	return pop
}

func hiveSoakOne(t *testing.T, seed int64, ticks int) hiveRow {
	t.Helper()
	deterministicTaskStops(t) // same seed, same run (seedSimRand seeds the rest)
	isolateSettlements(t)
	c, wm, err := newExpedition(fmt.Sprintf("Hive %d", seed), seed, false)
	if err != nil {
		t.Fatal(err)
	}
	loc := c.Locations[campaign.StartLocationID]
	loc.Factions = append(loc.Factions, campaign.FactionPresence{
		ID: "fph", Faction: "hive", Doctrine: "hive", Population: hiveSoakPop, Stage: 1,
		Units: map[string]int{"drone": 6, "scout": 2}, Larvae: 3,
		Stockpile: map[string]int{"biomass": 30}, Seed: seed*131 + 11,
		LastShotDay: campaign.NeverFired,
	})
	c.SetStance("hive", campaign.StanceNeutral, loc)
	if err := wm.Travel(campaign.StartLocationID); err != nil {
		t.Fatal(err)
	}
	if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
		t.Fatal(err)
	}
	ms, err := wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	c.SetStance("hive", campaign.StanceNeutral, loc) // the beam-down's contact undone
	d := campaign.LoadFactionData().Doctrine("hive")
	p := loc.PresenceByID("fph")
	name := factionSettlementName(p, loc.ID)
	resin := func() float64 {
		q := loc.PresenceByID("fph")
		if q == nil {
			return 0
		}
		cp := *q
		reconcileConversion(ms.level, d, &cp)
		return cp.Conversion
	}
	r := hiveRow{Seed: seed, Pop0: len(hiveBodies(ms.level, "fph", name)), Units: map[string]int{}, Laid: map[string]int{},
		FirstEvolve: -1, StageTick: -1, SpireTick: -1, ReadyTick: -1, LaunchTick: -1}
	r.Resin[0] = resin()
	startTechs := map[string]bool{}
	if st := settlement.Settlements[name]; st != nil {
		for _, k := range st.KnownTechs {
			startTechs[k] = true
		}
	}
	seenEgg := map[*ecs.Entity]bool{}
	for _, e := range hiveBodies(ms.level, "fph", name) {
		if strings.HasSuffix(e.Blueprint, "_egg") {
			seenEgg[e] = true // laid before the run
		}
	}
	hunger := map[*ecs.Entity]int{}
	fleets0 := len(c.Fleets)
	progress := soakProgress(fmt.Sprintf("hive:%d", seed), ticks)
	for tick := 1; tick <= ticks; tick++ {
		progress(tick)
		fspath.ResetFrameCounter()
		ms.stepWorld()
		wm.TickBackground(ms)
		if ms.tick%30 == 0 {
			ms.purgeCompletedTasks()
		}
		ms.advanceClock()
		if tick%30 == 0 {
			live := map[*ecs.Entity]bool{}
			for _, e := range hiveBodies(ms.level, "fph", name) {
				live[e] = true
				if strings.HasSuffix(e.Blueprint, "_egg") && !seenEgg[e] {
					seenEgg[e] = true
					r.Laid[components.KinOf(e).Type]++
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
			r.PeakLarvae = max(r.PeakLarvae, len(hiveLarvae(ms.level, name)))
			if st := settlement.Settlements[name]; st != nil && r.FirstEvolve < 0 {
				for _, k := range st.KnownTechs {
					if !startTechs[k] {
						r.FirstEvolve = tick
					}
				}
			}
			q := loc.PresenceByID("fph")
			if q != nil && r.StageTick < 0 && q.Stage >= 2 {
				r.StageTick = tick
			}
			if q != nil && r.SpireTick < 0 && hiveLauncher(ms.level, d, q) != nil {
				r.SpireTick = tick
			}
			if q != nil && r.ReadyTick < 0 && c.LaunchReady(loc, "fph", len(hiveBodies(ms.level, "fph", name)), wm.IsLoaded) {
				r.ReadyTick = tick
			}
			if r.LaunchTick < 0 && len(c.Fleets) > fleets0 {
				for _, f := range c.Fleets[fleets0:] {
					if f.FromID == loc.ID {
						r.LaunchTick = tick
						if r.ReadyTick < 0 {
							r.ReadyTick = tick
						}
					}
				}
			}
		}
		switch tick {
		case 10000:
			r.Resin[1] = resin()
		case 20000:
			r.Resin[2] = resin()
		}
		if os.Getenv("LP_SOAK_DEBUG") == "research" && tick%500 == 0 {
			if st := settlement.Settlements[name]; st != nil {
				for _, tk := range st.Tasks.GetTasks() {
					if rr, ok := tk.Data.(*task_requests.ResearchRequest); ok && !tk.Completed {
						t.Logf("tick %d research %s progress %d/%d delivered %v inprogress %v stock %d", tick, rr.TechKey, rr.Progress, rr.Required, rr.Delivered, tk.InProgress,
							storage.Summarize(storage.LevelProvider{Level: ms.level}, []string{name})["biomass"])
					}
				}
			}
		}
		if os.Getenv("LP_SOAK_DEBUG") != "" && tick%3000 == 0 {
			if dir := ms.factionDirectors[name]; dir != nil {
				t.Logf("seed %d tick %d resin %.3f bodies %d larvae %d ordered %v log %v", seed, tick, resin(),
					len(hiveBodies(ms.level, "fph", name)), len(hiveLarvae(ms.level, name)), dir.Ordered, dir.Log)
			}
		}
	}
	r.Resin[3] = resin()
	r.PopEnd = len(hiveBodies(ms.level, "fph", name))
	for _, e := range hiveBodies(ms.level, "fph", name) {
		r.Units[bodyType(d, e)]++
	}
	r.Biomass = storage.Summarize(storage.LevelProvider{Level: ms.level}, []string{name})["biomass"]
	r.Tier1 = tier1Pop(d, loc, r.Pop0, int(math.Round(float64(ticks)/750)))
	if q := loc.PresenceByID("fph"); q != nil {
		r.CapEnd, r.Stage = crewCap(c, loc, d, q), q.Stage
	}
	if st := settlement.Settlements[name]; st != nil {
		for _, k := range st.KnownTechs {
			if !startTechs[k] {
				r.Techs = append(r.Techs, k)
			}
		}
	}
	for _, f := range c.Fleets[min(fleets0, len(c.Fleets)):] {
		if f.FromID == loc.ID {
			r.Launches++
		}
	}
	if dir := ms.factionDirectors[name]; dir != nil {
		for k, v := range dir.Ordered {
			switch {
			case strings.HasPrefix(k, "mine:"):
				r.Chops += v
			case strings.HasPrefix(k, "hunt:"):
				r.Hunts += v
			case strings.HasPrefix(k, "haul:"):
				r.Hauls += v
			}
		}
		r.Cancelled = dir.Cancelled
	}
	ms.teardown()
	return r
}

func tickCol(t int) string {
	if t < 0 {
		return "-"
	}
	return fmt.Sprint(t)
}

func hiveSoakTable(rows []hiveRow, ticks int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "| Seed | Resin 0 / 10k / 20k / %d | Pop start / end (cap; Tier 1) | Stage | Units at the end | Eggs laid | Peak larvae | Biomass | Chops / hunts / hauls | First evolve | Techs | Hive stage / spire / launch-ready / first launch tick | Launches | Starved | Cancelled |\n", ticks)
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %d | %.2f / %.2f / %.2f / %.2f | %d / %d (%d; %d) | %d | %s | %s | %d | %d | %d / %d / %d | %s | %s | %s / %s / %s / %s | %d | %d | %d |\n",
			r.Seed, r.Resin[0], r.Resin[1], r.Resin[2], r.Resin[3], r.Pop0, r.PopEnd, r.CapEnd, r.Tier1, r.Stage,
			countsLine(r.Units), countsLine(r.Laid), r.PeakLarvae, r.Biomass, r.Chops, r.Hunts, r.Hauls,
			tickCol(r.FirstEvolve), strings.Join(r.Techs, ", "), tickCol(r.StageTick), tickCol(r.SpireTick), tickCol(r.ReadyTick), tickCol(r.LaunchTick),
			r.Launches, r.Starved, r.Cancelled)
	}
	return b.String()
}
