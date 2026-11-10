package game

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// The observer soak (phase C1; the plan's "What a soak failure looks like"):
// ten New Expeditions, each landing a party of four on its start system with
// an opening kit (soakKit), the director running the settlement and
// the world stepped headless as the game's Update does. It measures what the
// director builds and how long the colony lasts, and fails on the thresholds
// below. It takes minutes, so it only runs with LP_SOAK=1 (make soak);
// LP_SOAK_SEEDS (count), LP_SOAK_SEED (the first) and LP_SOAK_TICKS shorten
// it.

const (
	soakParty = 4
	// Thresholds, recorded with the numbers in the plan's "Phase C1 as
	// built". A seed fails a milestone when it is not reached by its tick.
	soakBedBy      = 5000  // a bed the director ordered, built
	soakLockerBy   = 8000  // a storage container it ordered, built
	soakResearchBy = 15000 // a research order given
	// More than soakMaxMisses seeds missing any milestone fails the soak.
	soakMaxMisses = 2
	// Survival: fewer than soakMinAlive of the party alive at the end, in
	// more than soakMaxMisses seeds, fails.
	soakMinAlive = 3
)

// soakKit is beamed down with the party.
var soakKit = map[string]int{"metal_ore": 30, "crystal": 15}

type soakRow struct {
	seed                         int64
	bed, locker, bench, lab, res int // first tick reached, -1 never
	alive5k, alive10k, aliveEnd  int
	firstDeath                   int
	techs                        int
	built                        map[string]int
	cancelled                    int
	stock                        map[string]int
	// The director digging (phase C2): stairs and tunnel orders, mine
	// orders, and structures built by 10k ticks and at the end.
	stairs, tunnel, mines        int
	built10k, builtEnd           int
}

func TestObserverSoak(t *testing.T) {
	if os.Getenv("LP_SOAK") == "" {
		t.Skip("set LP_SOAK=1 (make soak) to run the observer soak")
	}
	seeds := envInt("LP_SOAK_SEEDS", 10)
	ticks := envInt("LP_SOAK_TICKS", 20000)
	useContentTables(t)
	useRealCosts(t)
	useObserver(t)
	loadTutorialData(t) // maps, biomes, scenarios for generated sites

	var rows []soakRow
	first := envInt("LP_SOAK_SEED", 1) // LP_SOAK_SEED=3 LP_SOAK_SEEDS=1: seed 3 alone
	for i := 0; i < seeds; i++ {
		rows = append(rows, soakOne(t, int64(first+i), ticks))
	}
	t.Log("\n" + soakTable(rows, ticks))
	misses := 0
	for _, r := range rows {
		if miss(r.bed, soakBedBy) || miss(r.locker, soakLockerBy) || (ticks >= soakResearchBy && miss(r.res, soakResearchBy)) {
			misses++
		}
	}
	dying := 0
	for _, r := range rows {
		if r.aliveEnd < soakMinAlive {
			dying++
		}
	}
	if dying > soakMaxMisses {
		t.Errorf("%d of %d seeds ended with fewer than %d alive; at most %d may", dying, len(rows), soakMinAlive, soakMaxMisses)
	}
	if misses > soakMaxMisses {
		t.Errorf("%d of %d seeds missed a milestone (bed by %d, storage by %d, research by %d); at most %d may",
			misses, len(rows), soakBedBy, soakLockerBy, soakResearchBy, soakMaxMisses)
	}
}

func miss(at, by int) bool { return at < 0 || at > by }

func soakOne(t *testing.T, seed int64, ticks int) soakRow {
	t.Helper()
	deterministicTaskStops(t) // same seed, same run (seedSimRand seeds the rest)
	isolateSettlements(t)
	c, wm, err := newExpedition(fmt.Sprintf("Soak %d", seed), seed, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := wm.Travel(campaign.StartLocationID); err != nil {
		t.Fatal(err)
	}
	crew := wm.ShipColonists()
	for i := 0; i < soakParty && i < len(crew); i++ {
		if err := wm.BeamDown(crew[i]); err != nil {
			t.Fatal(err)
		}
	}
	ms, err := wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	// The opening kit a player beams down with the party (the tutorial
	// beams 10 metal): the director never beams, it builds from what is on
	// the site and what it mines.
	for bp, q := range soakKit {
		if err := wm.BeamResourceDown(bp, q); err != nil {
			t.Logf("seed %d: kit %s: %v", seed, bp, err)
		}
	}
	r := soakRow{seed: seed, bed: -1, locker: -1, bench: -1, lab: -1, res: -1, firstDeath: -1}
	_ = c
	for tick := 1; tick <= ticks; tick++ {
		fspath.ResetFrameCounter() // the game's Update does this before each step
		ms.stepWorld()
		wm.TickBackground(ms)
		if ms.tick%30 == 0 {
			ms.purgeCompletedTasks()
		}
		ms.advanceClock()
		if tick%50 != 0 && tick != ticks {
			continue
		}
		st := ms.settlementStats()
		if os.Getenv("LP_SOAK_DEBUG") != "" && tick%1000 == 0 {
			t.Logf("seed %d tick %d stats %+v stock %v", seed, tick, st, ms.buildEvalContext().ResourceCounts)
			if ms.director != nil {
				t.Logf("  log %v", ms.director.Log)
			}
			for _, e := range append(append([]*ecs.Entity(nil), ms.level.Entities...), ms.level.StaticEntities...) {
				if e != nil && (e.Blueprint == "bed" || e.Blueprint == "storage_locker") {
					if pc, ok := entityPos(e); ok {
						t.Logf("  %s at %d,%d,%d", e.Blueprint, pc.GetX(), pc.GetY(), pc.GetZ())
					}
				}
			}
			if os.Getenv("LP_SOAK_MAP") != "" {
				st := ms.MainSettlement
				for y := st.CenterY - 8; y <= st.CenterY + 8; y++ {
					row := ""
					for x := st.CenterX - 12; x <= st.CenterX + 12; x++ {
						ch := "."
						ti := ms.level.GetTileAt(x, y, st.CenterZ)
						if ti == nil {
							ch = " "
						} else if tl := ti.(*world.Tile); tl.Floor.IsEmpty() {
							ch = "_"
						} else if !tl.Middle.IsEmpty() && world.TileDefinitions[tl.Middle.Type].Solid {
							ch = "#"
						}
						if e := ms.level.GetSolidEntityAt(x, y, st.CenterZ); e != nil {
							ch = string(e.Blueprint[0])
						}
						for _, tk := range st.Tasks.GetTasks() {
							if tk.X == x && tk.Y == y {
								ch = "T"
							}
						}
						row += ch
					}
					t.Logf("  %d %s", y, row)
				}
			}
			for _, e := range colonyWorkers(ms) {
				pc, _ := entityPos(e)
				mem := e.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
				wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
				task := "none"
				if wc.CurrentTask != nil {
					task = fmt.Sprintf("%s@%d,%d", wc.CurrentTask.Action, wc.CurrentTask.X, wc.CurrentTask.Y)
				}
				t.Logf("  crew %s at %d,%d,%d state %s task %s bag %d", e.Blueprint, pc.GetX(), pc.GetY(), pc.GetZ(), mem.State, task,
					len(e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag))
			}
			for _, tk := range ms.MainSettlement.Tasks.GetTasks() {
				t.Logf("  task %s @%d,%d,%d inprog=%v stopped=%v %+v", tk.Action, tk.X, tk.Y, tk.Z, tk.InProgress, tk.ManuallyStopped, tk.Data)
			}
		}
		first := func(at *int, ok bool) {
			if *at < 0 && ok {
				*at = tick
			}
		}
		first(&r.bed, st.Built["bed"] > 0)
		first(&r.locker, st.Built["storage_locker"] > 0)
		first(&r.bench, st.Built["basic_workbench"] > 0)
		first(&r.lab, st.Built["research_lab"] > 0)
		first(&r.res, ms.director != nil && researchOrdered(ms.director.Ordered))
		first(&r.firstDeath, st.Dead > 0)
		switch tick {
		case 5000:
			r.alive5k = st.Alive
		case 10000:
			r.alive10k = st.Alive
			r.built10k = sumBuilt(st.Built)
		}
		if tick == ticks {
			r.aliveEnd, r.techs, r.built, r.cancelled = st.Alive, st.Techs, st.Built, st.Cancelled
			r.stock = ms.buildEvalContext().ResourceCounts
			r.builtEnd = sumBuilt(st.Built)
			if ms.director != nil {
				r.stairs, r.tunnel = ms.director.Ordered["dig:stairs"], ms.director.Ordered["dig:tunnel"]
				for k, v := range ms.director.Ordered {
					if strings.HasPrefix(k, "mine:") {
						r.mines += v
					}
				}
			}
		}
	}
	ms.teardown()
	return r
}

func sumBuilt(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

func researchOrdered(m map[string]int) bool {
	for k := range m {
		if strings.HasPrefix(k, "research:") {
			return true
		}
	}
	return false
}

func soakTable(rows []soakRow, ticks int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "| Seed | Bed | Locker | Bench | Lab | Research | First death | Alive 5k | Alive 10k | Alive %d | Techs | Stairs | Tunnel | Mines | Built 10k / end | Built | Cancelled |\n", ticks)
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	at := func(n int) string {
		if n < 0 {
			return "—"
		}
		return strconv.Itoa(n)
	}
	for _, r := range rows {
		var built []string
		for k, v := range r.built {
			built = append(built, fmt.Sprintf("%s %d", k, v))
		}
		sort.Strings(built)
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %s | %s | %s | %d | %d | %d | %d | %d | %d | %d | %d / %d | %s | %d |\n",
			r.seed, at(r.bed), at(r.locker), at(r.bench), at(r.lab), at(r.res), at(r.firstDeath),
			r.alive5k, r.alive10k, r.aliveEnd, r.techs, r.stairs, r.tunnel, r.mines, r.built10k, r.builtEnd,
			strings.Join(built, ", "), r.cancelled)
	}
	return b.String()
}

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v > 0 {
		return v
	}
	return def
}
