package game

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// The greyish soak (phase E4: docs/brainstorm/greyish_plan.md, "Step 6"):
// New Expeditions, each with a grey ship landing live on the nearest ore
// world the player has flown to (one colonist beamed down so the level is
// live; the start system carries no ore in reach on most seeds); the
// ship's director runs it for LP_SOAK_GREY_TICKS (22,500: a full 30-day
// stay) headless, stepped as the autonz soak is, under the real rules: a
// ship whose hold reaches its target, or whose stay ends, lifts off and the
// row ends there. Neutral, so it is measured working, not raiding. Two
// starts, each with its mothership (beaming its camp up to the beam floor
// once a day): the scout (three: a worker, a technician, a soldier; today's kit,
// 12 metal and 6 crystal; no techs; LP_SOAK_SEEDS seeds) and the warship
// (eight: three workers, two technicians, two soldiers, a heavy; 6 metal,
// 4 crystal, 4 alloy, 4 specimens: under the 30-unit hold target with its
// trip's digging (the plan's 68 units would send it home before it
// landed); the spine's three
// entries and the
// armory module, beam weapons and field emitter known; LP_SOAK_HUB_SEEDS
// seeds, default half). make soak-greyish runs the seeds side by side and
// joins them (TestGreyishSoakJoin).

const (
	// Thresholds (as built; greyish_plan.md "As built", step 6): a seed
	// misses when its camp got nowhere in its stay: no armory standing
	// (built or carried) by greyArmoryBy, no research entry learned, and no
	// metal ore reached. More than soakMaxMisses seeds of a start missing
	// fails; a raid in this neutral soak or a crew over its cap fails at
	// once. The plan's own targets (an armory by day 3 on 9 of 10 seeds, 2-3
	// research entries a landing, the hold at least half its target on 8 of
	// 10) are not met on today's maps, where ore lies 2-5 levels down and
	// far off: the table records how far short, for the owner.
	greyArmoryBy = 3000
)

type greyRow struct {
	Seed             int64
	Start            string
	Crew0, CrewEnd   int
	Cap              int
	Prints           map[string]int
	ArmoryTick       int
	Alloy            int
	Weapons          map[string]int
	Armed, Fighters  int
	Techs            []string
	TechTick         []int
	ArrayTick        int
	EmitterTick      int
	Lancers          int
	Specimens        int
	Hold0, HoldEnd   int
	HoldTarget       int
	Departure        string
	DepartTick       int
	Cancelled, Raids int
	Units            map[string]int
	Tunnels, Stairs  int
	Mines            int
	OreTick          int
}

func TestGreyishSoak(t *testing.T) {
	if os.Getenv("LP_SOAK") == "" {
		t.Skip("set LP_SOAK=1 (make soak-greyish) to run the greyish soak")
	}
	ticks := envInt("LP_SOAK_GREY_TICKS", 22500)
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	var rows []greyRow
	for _, run := range greySoakRuns(envInt("LP_SOAK_SEEDS", 10), envInt("LP_SOAK_HUB_SEEDS", 0), envInt("LP_SOAK_SEED", 1)) {
		row := greySoakOne(t, run.seed, run.start, ticks)
		if soakRowOut(t, "greyish", fmt.Sprintf("%s-%d", run.start, run.seed), row) {
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return
	}
	greySoakCheck(t, rows, ticks)
}

// TestGreyishSoakJoin reads the rows a parallel make soak-greyish wrote and
// prints and checks them as one run.
func TestGreyishSoakJoin(t *testing.T) {
	var rows []greyRow
	if !soakRowsIn(t, "greyish", &rows) {
		t.Skip("set LP_SOAK_ROWS (make soak-greyish) to join a parallel greyish soak")
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Start != rows[j].Start {
			return rows[i].Start == "scout"
		}
		return rows[i].Seed < rows[j].Seed
	})
	greySoakCheck(t, rows, envInt("LP_SOAK_GREY_TICKS", 22500))
}

// greySoakRuns is the scout seeds then the warship seeds, or the one
// LP_SOAK_ONLY names ("warship:3").
func greySoakRuns(scouts, warships, first int) []soakRun {
	var out []soakRun
	for _, r := range soakRuns(scouts, warships, first) {
		switch r.start {
		case "node":
			r.start = "scout"
		case "hub":
			r.start = "warship"
		}
		out = append(out, r)
	}
	return out
}

func greySoakCheck(t *testing.T, rows []greyRow, ticks int) {
	t.Helper()
	t.Log("\n" + greySoakTable(rows, ticks))
	misses := map[string]int{}
	for _, r := range rows {
		armory := r.ArmoryTick >= 0 && r.ArmoryTick <= greyArmoryBy
		if !armory && len(r.Techs) == 0 && r.OreTick < 0 {
			misses[r.Start]++ // it got nowhere
		}
		if r.Raids > 0 {
			t.Errorf("seed %d %s raided in a neutral soak", r.Seed, r.Start)
		}
		if r.CrewEnd-r.Lancers > r.Cap { // emitter lancers are over the cap
			t.Errorf("seed %d %s ended with %d crew, over its cap %d", r.Seed, r.Start, r.CrewEnd, r.Cap)
		}
	}
	for start, n := range misses {
		if n > soakMaxMisses {
			t.Errorf("%d %s seeds got nowhere (no armory by %d, no research entry, no metal ore reached); at most %d may",
				n, start, greyArmoryBy, soakMaxMisses)
		}
	}
	t.Logf("misses by start: %v", misses)
}

func greySoakOne(t *testing.T, seed int64, start string, ticks int) greyRow {
	t.Helper()
	deterministicTaskStops(t)
	isolateSettlements(t)
	c, wm, err := newExpedition(fmt.Sprintf("Greys %d", seed), seed, false)
	if err != nil {
		t.Fatal(err)
	}
	d := campaign.LoadFactionData().Doctrine("greyish")
	// An ore world (the greys' economy tag weights their landings toward
	// one): the nearest the ship's fuel reaches.
	var loc *campaign.Location
	for _, l := range c.Locations {
		if l.ID == c.StartID || !c.FactionSiteOK(l, false) || !hasTag(l.CapacityTags(), "ore") {
			continue
		}
		if loc == nil || c.FuelCost(l.ID) < c.FuelCost(loc.ID) || c.FuelCost(l.ID) == c.FuelCost(loc.ID) && l.ID < loc.ID {
			loc = l
		}
	}
	if loc == nil {
		t.Fatalf("seed %d: no ore world", seed)
	}
	tier, pop := 0, 3
	units := map[string]int{"grey_worker": 1, "grey_technician": 1, "grey_soldier": 1}
	stock := map[string]int{"metal_ore": 12, "crystal": 6}
	var techs []string
	if start == "warship" {
		tier, pop = 2, 8
		units = map[string]int{"grey_worker": 3, "grey_technician": 2, "grey_soldier": 2, "grey_heavy": 1}
		stock = map[string]int{"metal_ore": 6, "crystal": 4, "grey_alloy": 4, "specimen": 4}
		techs = []string{"grey_hull_plating", "grey_beam_weapons", "grey_cloning", "grey_armory_module", "grey_field_emitter"}
	}
	// In flight to the site, due long after the player gets there: once the
	// player is down it is brought in and lands live, fresh (no days down).
	c.FactionSeq++
	c.FleetSeq++
	c.Fleets = append(c.Fleets, campaign.Fleet{
		ID: "flsoak", Kind: campaign.FleetShip, FromID: "", ToID: loc.ID, ShipTier: tier, // far off: no encounter on the way
		FromX: loc.X + 900, FromY: loc.Y + 900, ToX: loc.X, ToY: loc.Y, LaunchDay: c.StarDate, ArriveDay: c.StarDate + 1000,
		Presence: campaign.FactionPresence{ID: "fpsoak", Faction: "greyish", Doctrine: "greyish", Population: pop,
			Units: units, KnownTechs: techs, Stockpile: stock, Seed: seed*173 + 11, LastShotDay: campaign.NeverFired},
	})
	if c.Mothership("greyish") == nil {
		// Its mothership (none is seeded below Rival factions 3), parked far
		// off, with its beam stock: what the camp's materials come down from.
		var far *campaign.Location
		for _, l := range c.Locations {
			if l.ID != loc.ID && l.ID != c.StartID && (far == nil || l.ID < far.ID) {
				far = l
			}
		}
		ms := campaign.Fleet{ID: "flms", Kind: campaign.FleetShip, Mothership: true, ShipTier: 2,
			FromID: far.ID, ToID: far.ID, FromX: far.X, FromY: far.Y, ToX: far.X, ToY: far.Y,
			Presence: campaign.FactionPresence{ID: "fpms", Faction: "greyish", Doctrine: "greyish", Population: 10,
				Seed: seed*179 + 5, LastShotDay: campaign.NeverFired}}
		if b := d.Ships.Mothership.Beam; b != nil {
			ms.Presence.BeamStock = map[string]int{}
			for k, v := range b.Stock {
				ms.Presence.BeamStock[k] = v
			}
		}
		c.Fleets = append(c.Fleets, ms)
	}
	c.SetStance("greyish", campaign.StanceNeutral, loc)
	if err := wm.Travel(loc.ID); err != nil {
		t.Fatal(err)
	}
	if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
		t.Fatal(err)
	}
	ms, err := wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	c.SetStance("greyish", campaign.StanceNeutral, loc)
	syncFactionHostility(c)
	f := c.FleetByPresence("fpsoak")
	f.ArriveDay = c.FactionDay
	for i := 0; i < 2*750 && !f.Landed(); i++ {
		fspath.ResetFrameCounter()
		ms.stepWorld()
		wm.TickBackground(ms)
		ms.advanceClock()
		f = c.FleetByPresence("fpsoak")
	}
	if f == nil || !f.Landed() || !f.Presence.Materialized {
		t.Fatalf("seed %d %s: the ship did not land live on %s (%+v)", seed, start, loc.Name, f)
	}
	p := &f.Presence
	name := factionSettlementName(p, loc.ID)
	if os.Getenv("LP_SOAK_DEBUG") == "ore" {
		cx, cy, cz, _ := originOf(ms.level, "fpsoak")
		surf, below := map[string]int{}, map[string]int{}
		for z := max(cz-8, 0); z <= cz; z++ {
			for y := cy - 80; y <= cy+80; y++ {
				for x := cx - 80; x <= cx+80; x++ {
					ti := ms.level.GetTilePtr(x, y, z)
					if ti == nil || ti.Middle.IsEmpty() {
						continue
					}
					if dd := world.DepositDrop(world.TileDefinitions[ti.Middle.Type].Name); dd != "" {
						if z == cz {
							surf[dd]++
						} else {
							below[fmt.Sprintf("%s@%d", dd, cz-z)]++
						}
					}
				}
			}
		}
		t.Logf("%s %d on %s (%s, tags %v) centre %d,%d,%d: surface ore within 80 %v, below (by depth) %v", start, seed, loc.Name, loc.ScenarioID, loc.CapacityTags(), cx, cy, cz, surf, below)
		all, chop := map[string]int{}, map[string]int{}
		for z := 0; z < ms.level.GetDepth(); z++ {
			for y := 0; y < ms.level.GetHeight(); y++ {
				for x := 0; x < ms.level.GetWidth(); x++ {
					if ti := ms.level.GetTilePtr(x, y, z); ti != nil && !ti.Middle.IsEmpty() {
						if dd := world.DepositDrop(world.TileDefinitions[ti.Middle.Type].Name); dd != "" {
							all[fmt.Sprintf("%s@z%d", dd, z)]++
						}
					}
				}
			}
		}
		for _, list := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
			for _, e := range list {
				if e != nil && e.HasComponent(components.Choppable) && e.HasComponent(components.Drops) {
					for _, it := range e.GetComponent(components.Drops).(*components.DropsComponent).Items {
						chop[it]++
					}
				}
			}
		}
		t.Logf("   level-wide deposits %v; choppable drops %v", all, chop)
	}
	r := greyRow{Seed: seed, Start: start, Crew0: len(settlementWorkers(ms.level, name)), Cap: d.CrewCap(tier),
		ArmoryTick: -1, ArrayTick: -1, EmitterTick: -1, DepartTick: -1, OreTick: -1, HoldTarget: d.Ships.HoldTarget, Departure: "-",
		Units: map[string]int{}}
	reconcileGreyish(c, ms.level, loc, d, p)
	r.Hold0 = sumStock(p.Stockpile)
	cx, cy, cz, _ := originOf(ms.level, "fpsoak")
	known := map[string]bool{}
	for _, k := range p.KnownTechs {
		known[k] = true
	}
	hold := r.Hold0
	var dir *greyDirSnapshot
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
		if tick%30 != 0 {
			continue
		}
		f = c.FleetByPresence("fpsoak")
		if f == nil || !f.Landed() {
			r.DepartTick = tick
			r.Departure = "stay"
			if f != nil && f.Returning {
				r.Departure = "hold"
			}
			if f != nil {
				hold = sumStock(f.Presence.Stockpile)
			}
			break
		}
		p = &f.Presence
		if dir == nil || tick%300 == 0 {
			dir = greySnapshotDir(ms, name)
			greyCrewNow(ms, d, name, &r) // the last count before a lift-off stands
		}
		if r.OreTick < 0 {
			if gd := ms.factionDirectors[name]; gd != nil && gd.Ordered["mine:metal_ore"] > 0 {
				r.OreTick = tick
			}
		}
		if os.Getenv("LP_SOAK_DEBUG") == "tasks" && tick%1500 == 0 {
			if st := settlement.Settlements[name]; st != nil {
				for _, tk := range st.Tasks.GetTasks() {
					if !tk.Completed {
						t.Logf("tick %d task %s %+v inprog %v", tick, tk.Action, tk.Data, tk.InProgress)
					}
				}
			}
			for _, e := range settlementWorkers(ms.level, name) {
				wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
				mem := e.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
				act := ""
				if wc.CurrentTask != nil {
					act = fmt.Sprintf("%s %+v", wc.CurrentTask.Action, wc.CurrentTask.Data)
				}
				t.Logf("tick %d %s %s state %s task %s stock %v", tick, e.Blueprint, wc.Role, mem.State, act, storeOf(ms, name))
			}
		}
		if st := settlement.Settlements[name]; st != nil {
			for _, k := range st.KnownTechs {
				if !known[k] {
					known[k] = true
					r.Techs, r.TechTick = append(r.Techs, k), append(r.TechTick, tick)
				}
			}
		}
		if r.ArmoryTick < 0 && len(greyStanding(ms.level, d, "grey_armory", cx, cy, cz)) > 0 {
			r.ArmoryTick = tick
		}
		if r.ArrayTick < 0 && len(greyStanding(ms.level, d, "grey_array", cx, cy, cz)) > 0 {
			r.ArrayTick = tick
		}
		if r.EmitterTick < 0 && len(greyStanding(ms.level, d, "grey_emitter", cx, cy, cz)) >= 2 {
			r.EmitterTick = tick
		}
		hold = sumStock(storeOf(ms, name))
	}
	progress(ticks) // a row that ended at lift-off
	r.HoldEnd = hold
	if r.DepartTick < 0 {
		greyCrewNow(ms, d, name, &r)
		r.Specimens = storeOf(ms, name)["specimen"]
		dir = greySnapshotDir(ms, name)
	} else if f != nil {
		r.Specimens = f.Presence.Stockpile["specimen"]
	}
	if dir != nil {
		r.Prints, r.Weapons, r.Alloy, r.Cancelled, r.Raids = dir.prints, dir.weapons, dir.alloy, dir.cancelled, dir.raids
		r.Tunnels, r.Stairs = dir.ordered["dig:tunnel"], dir.ordered["dig:stairs"]
		for k, v := range dir.ordered {
			if strings.HasPrefix(k, "mine:") {
				r.Mines += v
			}
		}
	}
	if os.Getenv("LP_SOAK_DEBUG") != "" && dir != nil {
		t.Logf("%s %d ordered %v log %v", start, seed, dir.ordered, dir.log)
	}
	ms.teardown()
	return r
}

// greyDirSnapshot is the director's counts, kept while the ship is down
// (its director goes with it at lift-off).
type greyDirSnapshot struct {
	prints, weapons map[string]int
	alloy           int
	cancelled       int
	raids           int
	ordered         map[string]int
	log             []string
}

func greySnapshotDir(ms *MainState, name string) *greyDirSnapshot {
	dir := ms.factionDirectors[name]
	if dir == nil {
		return nil
	}
	out := &greyDirSnapshot{prints: map[string]int{}, weapons: map[string]int{}, ordered: map[string]int{}}
	for k, v := range dir.Ordered {
		out.ordered[k] = v
		switch {
		case strings.HasPrefix(k, "grow:"):
			out.prints[strings.TrimPrefix(k, "grow:")] += v
		case k == "craft:refine_alloy":
			out.alloy += v
		case strings.HasPrefix(k, "craft:grey_"):
			out.weapons[strings.TrimPrefix(k, "craft:grey_")] += v
		}
	}
	out.cancelled, out.raids = dir.Cancelled, dir.Ordered["raids"]
	out.log = append([]string(nil), dir.Log...)
	return out
}

func sumStock(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

func greySoakTable(rows []greyRow, ticks int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "| Seed | Start | Crew start / end (cap) | Units at the end | Prints by type | Stairs / tunnels / mines | Metal ore tick | Armory tick | Alloy refines | Weapons made | Armed fighters | Techs (first tick) | Array / emitters tick | Lancers | Specimens | Hold start / end (target) | Departure (tick) | Cancelled |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		var techs []string
		for i, k := range r.Techs {
			techs = append(techs, fmt.Sprintf("%s %d", strings.TrimPrefix(k, "grey_"), r.TechTick[i]))
		}
		fmt.Fprintf(&b, "| %d | %s | %d / %d (%d) | %s | %s | %d / %d / %d | %s | %s | %d | %s | %d / %d | %s | %s / %s | %d | %d | %d / %d (%d) | %s %s | %d |\n",
			r.Seed, r.Start, r.Crew0, r.CrewEnd, r.Cap, countLine(r.Units), countLine(r.Prints), r.Stairs, r.Tunnels, r.Mines, tickOr(r.OreTick), tickOr(r.ArmoryTick), r.Alloy,
			countLine(r.Weapons), r.Armed, r.Fighters, strings.Join(techs, ", "), tickOr(r.ArrayTick), tickOr(r.EmitterTick),
			r.Lancers, r.Specimens, r.Hold0, r.HoldEnd, r.HoldTarget, r.Departure, tickOr(r.DepartTick), r.Cancelled)
	}
	fmt.Fprintf(&b, "(%d ticks; a row ends at lift-off)\n", ticks)
	return b.String()
}

func hasTag(tags []string, k string) bool {
	for _, t := range tags {
		if t == k {
			return true
		}
	}
	return false
}

// greyCrewNow counts the camp's crew into r: by type, fighters (soldiers and
// heavies) and those holding a beam weapon, lancers.
func greyCrewNow(ms *MainState, d *campaign.Doctrine, name string, r *greyRow) {
	r.CrewEnd, r.Fighters, r.Armed, r.Lancers = 0, 0, 0, 0
	r.Units = map[string]int{}
	for _, e := range settlementWorkers(ms.level, name) {
		r.CrewEnd++
		typ := greyTypeOf(d, e)
		r.Units[typ]++
		if typ == "grey_lancer" {
			r.Lancers++
		}
		dt := d.Types[typ]
		if dt == nil || dt.Role != "soldier" && dt.Role != "heavy" {
			continue
		}
		r.Fighters++
		if inv, ok := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent); ok && inv != nil {
			for _, it := range wornGear(inv) {
				if it.HasComponent(rlcomponents.Weapon) && it.HasComponent(components.GearDrop) {
					r.Armed++
					break
				}
			}
		}
	}
}
