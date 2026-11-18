package game

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/generation"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// The autonz soak (phase E, extended in E3: docs/brainstorm/autonz_plan.md,
// "Balance"): New Expeditions, each with an autonz presence put on the start
// system and one colonist beamed down so the level is live; the autonz'
// director runs the settlement for LP_SOAK_AUTONZ_TICKS (30,000: forty
// days) headless, stepped as the observer soak is. Neutral, so it is
// measured building its economy, not raiding. Two starts: the node (Phase
// E's: four, 20 metal and 4 crystal in its hold; LP_SOAK_SEEDS seeds) and
// the hub (nine, 40 metal, 8 crystal, 10 ferrite, the relay and foundry
// patterns known; LP_SOAK_HUB_SEEDS seeds, default half). Runs with LP_SOAK=1
// (make soak, make soak-autonz). One seed alone: LP_SOAK_ONLY=node:3 (the
// soak-autonz target runs the seeds as separate processes and joins their
// rows; see soakRows).

const (
	autonzSoakPop    = 4
	autonzSoakHubPop = 9
	// Thresholds (the plan's "Balance"): a seed misses when the converted
	// share rose by less than autonzSoakConvert by the end, it never worked
	// toward ore (no mine, stairs or tunnel order), or no cogitator stood
	// by tick 15,000. More than soakMaxMisses seeds missing fails. The
	// other columns are measured, not required.
	autonzSoakConvert = 0.05
	autonzCogitatorBy = 15000
)

type autonzRow struct {
	Seed                      int64
	Start                     string
	Share                     [4]float64
	Converts, Mines           int
	Stairs, Tunnel            int
	Prints                    map[string]int
	Bronze                    int
	Workers0, WorkersEnd      int
	CapEnd, Stage             int
	CogTick                   int
	Techs                     []string
	TechTick                  []int
	Built                     map[string]int
	Ferrite                   int
	Signals                   int
	Units                     map[string]int
	Cancelled                 int
	Raids                     int
	DeployTick, FoundryTick   int
	RelayTick, FoundryPatTick int
}

func TestAutonzSoak(t *testing.T) {
	if os.Getenv("LP_SOAK") == "" {
		t.Skip("set LP_SOAK=1 (make soak) to run the autonz soak")
	}
	ticks := envInt("LP_SOAK_AUTONZ_TICKS", 30000)
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	var rows []autonzRow
	for _, run := range soakRuns(envInt("LP_SOAK_SEEDS", 10), envInt("LP_SOAK_HUB_SEEDS", 0), envInt("LP_SOAK_SEED", 1)) {
		row := autonzSoakOne(t, run.seed, run.start, ticks)
		if soakRowOut(t, "autonz", fmt.Sprintf("%s-%d", run.start, run.seed), row) {
			continue // one process of make soak-autonz: the join applies the thresholds
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return
	}
	autonzSoakCheck(t, rows, ticks)
}

// TestAutonzSoakJoin reads the rows a parallel make soak-autonz wrote
// (LP_SOAK_ROWS) and prints and checks them as one run.
func TestAutonzSoakJoin(t *testing.T) {
	var rows []autonzRow
	if !soakRowsIn(t, "autonz", &rows) {
		t.Skip("set LP_SOAK_ROWS (make soak-autonz) to join a parallel autonz soak")
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Start != rows[j].Start {
			return rows[i].Start > rows[j].Start // node first
		}
		return rows[i].Seed < rows[j].Seed
	})
	autonzSoakCheck(t, rows, envInt("LP_SOAK_AUTONZ_TICKS", 30000))
}

type soakRun struct {
	seed  int64
	start string
}

// soakRuns is the node seeds then the hub seeds (hub 0: half the node
// count, at least one; -1: none, a soak with one start), or the one
// LP_SOAK_ONLY names ("hub:3", "hive:3").
func soakRuns(nodes, hubs, first int) []soakRun {
	if only := os.Getenv("LP_SOAK_ONLY"); only != "" {
		var start string
		var seed int64
		if _, err := fmt.Sscanf(strings.Replace(only, ":", " ", 1), "%s %d", &start, &seed); err == nil {
			return []soakRun{{seed, start}}
		}
	}
	if hubs < 0 {
		hubs = 0 // a soak with one start (the hive's)
	} else if hubs == 0 {
		hubs = max(1, nodes/2)
	}
	var out []soakRun
	for i := 0; i < nodes; i++ {
		out = append(out, soakRun{int64(first + i), "node"})
	}
	for i := 0; i < hubs; i++ {
		out = append(out, soakRun{int64(first + i), "hub"})
	}
	return out
}

func autonzSoakCheck(t *testing.T, rows []autonzRow, ticks int) {
	t.Helper()
	t.Log("\n" + autonzSoakTable(rows, ticks))
	// Misses count per start: the hub seeds reuse the node seeds' maps, so
	// a site with no ore in dig reach (seeds 2 and 7, as in Phase E) would
	// otherwise count twice.
	misses := map[string]int{}
	for _, r := range rows {
		cogLate := r.CogTick < 0 || r.CogTick > autonzCogitatorBy
		if r.Share[3]-r.Share[0] < autonzSoakConvert || r.Mines+r.Stairs+r.Tunnel == 0 || cogLate {
			misses[r.Start]++
		}
		if r.Raids > 0 {
			t.Errorf("seed %d %s raided in a neutral soak", r.Seed, r.Start)
		}
	}
	for start, n := range misses {
		if n > soakMaxMisses {
			t.Errorf("%d %s seeds missed (conversion +%.2f, an order toward ore, a cogitator by %d); at most %d may", n, start, autonzSoakConvert, autonzCogitatorBy, soakMaxMisses)
		}
	}
	t.Logf("misses by start: %v", misses)
}

func autonzSoakOne(t *testing.T, seed int64, start string, ticks int) autonzRow {
	t.Helper()
	deterministicTaskStops(t) // same seed, same run (seedSimRand seeds the rest)
	isolateSettlements(t)
	c, wm, err := newExpedition(fmt.Sprintf("Autonz %d", seed), seed, false)
	if err != nil {
		t.Fatal(err)
	}
	loc := c.Locations[c.StartID]
	fp := campaign.FactionPresence{
		ID: "fpz", Faction: "autonz", Doctrine: "autonz", Population: autonzSoakPop,
		Stockpile: map[string]int{"metal_ore": 20, "crystal": 4}, Seed: seed*131 + 9,
		LastShotDay: campaign.NeverFired,
	}
	if start == "hub" {
		fp.Population, fp.Stage, fp.Conversion = autonzSoakHubPop, 1, 0.45
		// Under its signal conversion and with the cooldown just begun, so
		// Tier 1 does not signal its placers away before the landing; never
		// more than its conversion cap less two, so it has prints to make
		// (the start system carries few: as built).
		fp.LastShotDay = c.StarDate
		// The start system carries five or six autonz: a hub of nine there
		// starves at once. The hub start reads it as an ore and salvage
		// world (+10 capacity), so the hub has room to print (as built).
		loc.Tags = append(append([]string(nil), loc.Tags...), "ore", "salvage")
		d := campaign.LoadFactionData().Doctrine("autonz")
		fp.Population = max(5, min(autonzSoakHubPop, d.ConvertCap(loc, &fp)-2))
		fp.Stockpile = map[string]int{"metal_ore": 40, "crystal": 8, "ferrite": 10}
		fp.KnownTechs = []string{"autonz_relay_pattern", "autonz_foundry_pattern"}
	}
	loc.Factions = append(loc.Factions, fp)
	c.SetStance("autonz", campaign.StanceNeutral, loc)
	if err := wm.Travel(c.StartID); err != nil {
		t.Fatal(err)
	}
	if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
		t.Fatal(err)
	}
	ms, err := wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	c.SetStance("autonz", campaign.StanceNeutral, loc) // the beam-down's contact undone
	doc := campaign.LoadFactionData().Doctrine("autonz")
	p := loc.PresenceByID("fpz")
	name := factionSettlementName(p, loc.ID)
	share := func() float64 {
		cx, cy, cz, ok := generation.Origin(ms.level, factionOriginKey("fpz"))
		if !ok {
			return 0
		}
		return generation.ConversionShare(ms.level, cx, cy, cz, doc.Conversion.Radius, doc.Conversion.Tile)
	}
	r := autonzRow{Seed: seed, Start: start, Workers0: len(settlementWorkers(ms.level, name)), CogTick: -1,
		DeployTick: -1, FoundryTick: -1, RelayTick: -1, FoundryPatTick: -1, Units: map[string]int{}}
	r.Share[0] = share()
	if os.Getenv("LP_SOAK_DEBUG") != "" {
		t.Logf("%s %d start units %v record %v", start, seed, func() map[string]int {
			m := map[string]int{}
			for _, e := range settlementWorkers(ms.level, name) {
				m[autonzTypeOf(doc, e)]++
			}
			return m
		}(), p.Units)
	}
	cx, cy, cz, _ := originOf(ms.level, "fpz")
	known := map[string]bool{}
	for _, k := range p.KnownTechs {
		known[k] = true
		switch k {
		case "autonz_relay_pattern":
			r.RelayTick = 0
		case "autonz_foundry_pattern":
			r.FoundryPatTick = 0
		}
	}
	seen := map[*ecs.Entity]bool{}
	for _, e := range settlementWorkers(ms.level, name) {
		seen[e] = true
	}
	var lastSignal campaign.FactionPresence
	lastSignal.LastShotDay = p.LastShotDay
	progress := soakProgress(fmt.Sprintf("%s:%d", start, seed), ticks)
	for tick := 1; tick <= ticks; tick++ {
		progress(tick)
		fspath.ResetFrameCounter()
		ms.stepWorld()
		wm.TickBackground(ms)
		if ms.tick%30 == 0 {
			ms.purgeCompletedTasks()
		}
		ms.advanceClock()
		switch tick {
		case 10000:
			r.Share[1] = share()
		case 20000:
			r.Share[2] = share()
		}
		if tick%30 != 0 {
			continue
		}
		p = loc.PresenceByID("fpz")
		if p == nil {
			break
		}
		if p.LastShotDay != lastSignal.LastShotDay {
			r.Signals++
			lastSignal.LastShotDay = p.LastShotDay
		}
		if st := settlement.Settlements[name]; st != nil {
			for _, k := range st.KnownTechs {
				if !known[k] {
					known[k] = true
					r.Techs, r.TechTick = append(r.Techs, k), append(r.TechTick, tick)
					switch k {
					case "autonz_relay_pattern":
						r.RelayTick = tick
					case "autonz_foundry_pattern":
						r.FoundryPatTick = tick
					}
				}
			}
		}
		if r.CogTick < 0 && len(hiveStructures(ms.level, "cogitator", cx, cy, cz, 18)) > 0 {
			r.CogTick = tick
		}
		if r.DeployTick < 0 && len(hiveStructures(ms.level, campaign.PylonKey, cx, cy, cz, 18)) > 0 {
			r.DeployTick = tick
		}
		if r.FoundryTick < 0 && len(hiveStructures(ms.level, "foundry", cx, cy, cz, 18)) > 0 {
			r.FoundryTick = tick
		}
		for _, e := range settlementWorkers(ms.level, name) {
			if seen[e] {
				continue
			}
			seen[e] = true
			if k := components.KinOf(e); k != nil && k.Tier == "bronze" {
				r.Bronze++
			}
		}
		if os.Getenv("LP_SOAK_DEBUG") == "research" && tick%300 == 0 {
			if st := settlement.Settlements[name]; st != nil {
				for _, tk := range st.Tasks.GetTasks() {
					if rr, ok := tk.Data.(*task_requests.ResearchRequest); ok {
						t.Logf("tick %d research %s %d/%d done %v inprog %v", tick, rr.TechKey, rr.Progress, rr.Required, tk.Completed, tk.InProgress)
					}
				}
			}
			for _, e := range settlementWorkers(ms.level, name) {
				wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
				if wc.CurrentTask != nil && wc.CurrentTask.Action == "research" {
					mem := e.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
					pc, _ := entityPos(e)
					t.Logf("  researcher state %s at %d,%d task at %d,%d cog %v init %+v", mem.State, pc.GetX(), pc.GetY(), wc.CurrentTask.X, wc.CurrentTask.Y, len(hiveStructures(ms.level, "cogitator", wc.CurrentTask.X, wc.CurrentTask.Y, pc.GetZ(), 0)), e.GetComponent(rlcomponents.Initiative))
				}
			}
		}
		if os.Getenv("LP_SOAK_DEBUG") != "" && tick%3000 == 0 {
			if dir := ms.factionDirectors[name]; dir != nil {
				t.Logf("%s %d tick %d share %.3f cap %d workers %d ordered %v log %v", start, seed, tick, share(),
					crewCap(c, loc, doc, p), len(settlementWorkers(ms.level, name)), dir.Ordered, dir.Log)
			}
		}
	}
	if os.Getenv("LP_SOAK_DEBUG") != "" {
		if st := settlement.Settlements[name]; st != nil {
			for _, tk := range st.Tasks.GetTasks() {
				t.Logf("task %s %+v done %v inprog %v", tk.Action, tk.Data, tk.Completed, tk.InProgress)
			}
		}
		for _, e := range settlementWorkers(ms.level, name) {
			wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
			pc, _ := entityPos(e)
			mem := e.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
			inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
			t.Logf("worker %s at %d,%d,%d state %s bag %d allowed %v task %+v", e.Blueprint, pc.GetX(), pc.GetY(), pc.GetZ(), mem.State, len(inv.Bag), wc.AllowedTasks, wc.CurrentTask)
		}
	}
	r.Share[3] = share()
	r.WorkersEnd = len(settlementWorkers(ms.level, name))
	for _, e := range settlementWorkers(ms.level, name) {
		if k := components.KinOf(e); k != nil {
			key := k.Type
			if k.Tier == "bronze" {
				key += "*"
			}
			r.Units[key]++
		}
	}
	if p := loc.PresenceByID("fpz"); p != nil {
		r.Stage, r.CapEnd = p.Stage, crewCap(c, loc, doc, p)
	}
	r.Built = autonzBuilt(ms.level, doc, cx, cy, cz)
	if dir := ms.factionDirectors[name]; dir != nil {
		r.Prints = map[string]int{}
		for k, v := range dir.Ordered {
			switch {
			case strings.HasPrefix(k, "convert:"):
				r.Converts += v
			case strings.HasPrefix(k, "mine:"):
				r.Mines += v
			case strings.HasPrefix(k, "grow:"):
				r.Prints[strings.TrimPrefix(k, "grow:")] += v
			}
		}
		r.Stairs, r.Tunnel, r.Cancelled = dir.Ordered["dig:stairs"], dir.Ordered["dig:tunnel"], dir.Cancelled
		r.Ferrite, r.Raids = 2*dir.Ordered["craft:refine_ferrite"], dir.Ordered["raids"]
	}
	ms.teardown()
	return r
}

func countLine(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		if m[k] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
		}
	}
	return strings.Join(parts, ", ")
}

func tickOr(t int) string {
	if t < 0 {
		return "-"
	}
	return fmt.Sprint(t)
}

func autonzSoakTable(rows []autonzRow, ticks int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "| Seed | Start | Share 0 / 10k / 20k / %d | Converts | Mines | Prints by type | Bronze prints | Units at the end (* bronze) | Crew start / end (cap) | Stage | Cogitator tick | Techs (first tick) | Core / pylons / foundry / converters / defence | Pylon / foundry tick | Ferrite refined | Signals | Cancelled |\n", ticks)
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		var techs []string
		for i, k := range r.Techs {
			techs = append(techs, fmt.Sprintf("%s %d", strings.TrimPrefix(k, "autonz_"), r.TechTick[i]))
		}
		bl := r.Built
		fmt.Fprintf(&b, "| %d | %s | %.2f / %.2f / %.2f / %.2f | %d | %d | %s | %d | %s | %d / %d (%d) | %d | %s | %s | %d / %d / %d / %d / %d | %s / %s | %d | %d | %d |\n",
			r.Seed, r.Start, r.Share[0], r.Share[1], r.Share[2], r.Share[3], r.Converts, r.Mines, countLine(r.Prints), r.Bronze,
			countLine(r.Units), r.Workers0, r.WorkersEnd, r.CapEnd, r.Stage, tickOr(r.CogTick), strings.Join(techs, ", "),
			bl[autonzCore], bl[campaign.PylonKey], bl["foundry"], bl["converter_node"], bl["defence_pylon"],
			tickOr(r.DeployTick), tickOr(r.FoundryTick), r.Ferrite, r.Signals, r.Cancelled)
	}
	return b.String()
}
