package game

import (
	"fmt"
	"sort"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// hiveTestSite is factionTestSite with the hive's settlement isolated: a hive
// presence of pop at stage with stock biomass, on the moon map.
func hiveTestSite(t *testing.T, seed int64, pop, stage, stock int, techs ...string) (*MainState, *campaign.Location) {
	t.Helper()
	isolateSettlements(t)
	useContentTables(t)
	ms, loc := factionTestSite(t, "moon", seed, pop, stage, techs...)
	if stock > 0 {
		loc.Factions[0].Stockpile = map[string]int{"biomass": stock}
	}
	return ms, loc
}

// hiveThings lists everything of presence fp1's hive on level (bodies,
// larvae, structures, pools), sorted, with type and place.
func hiveThings(level *world.Level, name string) []string {
	var out []string
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e == nil || e.HasComponent(rlcomponents.Dead) {
				continue
			}
			k := components.KinOf(e)
			mine := k != nil && (k.Presence == "fp1" || k.Settlement == name)
			if e.HasComponent(components.Storage) && e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy == name {
				mine = true
			}
			if !mine {
				continue
			}
			pc, _ := entityPos(e)
			typ := ""
			if k != nil {
				typ = k.Type
			}
			out = append(out, fmt.Sprintf("%s %s @%d,%d,%d", e.Blueprint, typ, pc.GetX(), pc.GetY(), pc.GetZ()))
		}
	}
	sort.Strings(out)
	return out
}

// A hive presence builds as a settlement the same way every time: the
// heart, a pool with the stock, the stage's structures each on resin (laid
// before them), bodies by type matching the record's units, larvae to its
// record within the cap; a second build adds nothing.
func TestHiveMaterialise(t *testing.T) {
	var first []string
	for run := 0; run < 2; run++ {
		ms, loc := hiveTestSite(t, 7, 16, 2, 150, "hive_warrior_brood", "launcher_range_1", "hive_hunter_brood")
		p := &loc.Factions[0]
		p.Conversion, p.Larvae = 0.3, 20
		materialiseFactions(nil, loc, ms.level, ms.scenario)
		name := factionSettlementName(p, loc.ID)
		snap := hiveThings(ms.level, name)
		if run == 0 {
			first = snap
		} else if fmt.Sprint(snap) != fmt.Sprint(first) {
			t.Fatalf("second build differs:\n%v\n%v", first, snap)
		}
		d := campaign.LoadFactionData().Doctrine("hive")
		cx, cy, cz, _ := presenceCentre(ms.level, loc, d, p)
		for bp, n := range map[string]int{"hive_heart": 1, "egg_cluster": 2, "evolution_chamber": 1, "egg_launcher": 1} {
			got := hiveStructures(ms.level, bp, cx, cy, cz, hiveRadius(d)+2)
			if len(got) != n {
				t.Errorf("%d %s, want %d", len(got), bp, n)
			}
			for _, e := range got {
				if pc, _ := entityPos(e); bp != "hive_heart" && floorNameAt(ms.level, pc.GetX(), pc.GetY(), pc.GetZ()) != resinTile {
					t.Errorf("%s off resin at %d,%d", bp, pc.GetX(), pc.GetY())
				}
			}
		}
		units := map[string]int{}
		for _, e := range hiveBodies(ms.level, p.ID, name) {
			units[bodyType(d, e)]++
		}
		if fmt.Sprint(units) != fmt.Sprint(p.Units) || p.UnitsTotal() != 16 {
			t.Errorf("bodies by type %v, record %v", units, p.Units)
		}
		if units["warrior"] == 0 || units["hunter"] == 0 || units["drone"] == 0 {
			t.Errorf("types %v: want drones, warriors and hunters", units)
		}
		if n, cap := len(hiveLarvae(ms.level, name)), larvaCap(ms.level, d, cx, cy, cz); n != cap || cap != 3+2*2 {
			t.Errorf("%d larvae, cap %d (want 3 + 2 per cluster)", n, cap)
		}
		stock := storage.Summarize(storage.LevelProvider{Level: ms.level}, []string{name})
		if stock["biomass"] != 150 || len(hivePools(ms.level, name)) < 2 {
			t.Errorf("stock %v in %d pools, want 150 in at least 2", stock, len(hivePools(ms.level, name)))
		}
		if share := campaignShare(ms.level, d, p); share < 0.25 {
			t.Errorf("resin share %.2f, record 0.3", share)
		}
		materialiseFactions(nil, loc, ms.level, ms.scenario)
		if again := hiveThings(ms.level, name); fmt.Sprint(again) != fmt.Sprint(snap) {
			t.Errorf("a second materialise changed the level: %d -> %d", len(snap), len(again))
		}
	}
}

// campaignShare is the resin share round p's centre.
func campaignShare(level *world.Level, d *campaign.Doctrine, p *campaign.FactionPresence) float64 {
	q := *p
	reconcileConversion(level, d, &q)
	return q.Conversion
}

// Phase B's scripted bodies of a presence are adopted once: an adult is
// replaced by its type's crew body on its tile; an egg is marked the
// settlement's, so it grows into one.
func TestHiveAdoptsScriptedBodies(t *testing.T) {
	ms, loc := hiveTestSite(t, 9, 3, 0, 10)
	p := &loc.Factions[0]
	d := campaign.LoadFactionData().Doctrine("hive")
	cx, cy, cz, _ := presenceCentre(ms.level, loc, d, p)
	adult, _ := factory.CreateTyped("xeno_stage3", cx+3, cy, cz, "warrior", p.ID)
	egg, _ := factory.CreateTyped("troid_egg", cx-3, cy, cz, "", p.ID)
	ms.level.AddEntity(adult)
	ms.level.AddEntity(egg)
	p.Units = map[string]int{"warrior": 1, "scout": 1, "drone": 1}
	p.KnownTechs = []string{"hive_warrior_brood"}
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	name := factionSettlementName(p, loc.ID)
	if !adult.HasComponent(rlcomponents.Dead) && hasEntity(ms.level, adult) {
		t.Error("the scripted adult is still there")
	}
	var crew *ecs.Entity
	for _, e := range settlementWorkers(ms.level, name) {
		if components.KinOf(e).Type == "warrior" {
			crew = e
		}
	}
	if crew == nil || crew.Blueprint != "xeno_crew" {
		t.Fatalf("no warrior crew body (workers %d)", len(settlementWorkers(ms.level, name)))
	}
	if k := components.KinOf(egg); k.Settlement != name || k.Type != "scout" {
		t.Errorf("adopted egg kin %+v", k)
	}
	before := hiveThings(ms.level, name)
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	if after := hiveThings(ms.level, name); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Error("adoption ran twice")
	}
}

func hasEntity(level *world.Level, e *ecs.Entity) bool {
	for _, x := range level.Entities {
		if x == e {
			return true
		}
	}
	return false
}

// Reconcile writes the level back: units, population, larvae, stock, resin
// and techs round-trip; a dead warrior and a used larva come back as fewer.
func TestHiveReconcileRoundTrips(t *testing.T) {
	ms, loc := hiveTestSite(t, 5, 12, 1, 40, "hive_warrior_brood")
	p := &loc.Factions[0]
	p.Conversion, p.Larvae = 0.2, 3
	c := factionTestCampaign(loc)
	materialiseFactions(c, loc, ms.level, ms.scenario)
	want := *p
	wantUnits := fmt.Sprint(p.Units)
	settlement.Settlements[factionSettlementName(p, loc.ID)].UnlockTech("hive_drone_vigour")
	reconcileFactions(c, loc, ms.level)
	got := loc.PresenceByID("fp1")
	if got == nil || fmt.Sprint(got.Units) != wantUnits || got.Population != want.Population || got.Larvae != 3 ||
		got.Stockpile["biomass"] != 40 || got.Conversion < want.Conversion-0.05 || !containsStr(got.KnownTechs, "hive_drone_vigour") {
		t.Fatalf("round trip: %+v, want units %s", got, wantUnits)
	}
	name := factionSettlementName(p, loc.ID)
	for _, e := range hiveBodies(ms.level, p.ID, name) {
		if components.KinOf(e).Type == "warrior" {
			e.AddComponent(&rlcomponents.DeadComponent{})
			break
		}
	}
	hiveLarvae(ms.level, name)[0].AddComponent(&rlcomponents.DeadComponent{})
	reconcileFactions(c, loc, ms.level)
	got = loc.PresenceByID("fp1")
	if got.Population != want.Population-1 || got.Units["warrior"] != want.Units["warrior"]-1 || got.Larvae != 2 {
		t.Errorf("after a kill: pop %d units %v larvae %d", got.Population, got.Units, got.Larvae)
	}
}

// A live brood with stock: the heart lays larvae on its timer, the director
// orders a drone egg on one, a drone carries the biomass and lays it, the egg
// hatches, and its hatchling grows into a working drone of the settlement.
func TestHiveLarvaBecomesADrone(t *testing.T) {
	if testing.Short() {
		t.Skip("long")
	}
	isolateSettlements(t)
	useContentTables(t)
	ms, loc := factionTestSite(t, "moon", 11, 3, 0)
	loc.Factions[0].Stockpile = map[string]int{"biomass": 20}
	c := factionTestCampaign(loc)
	c.CurrentLocationID = loc.ID
	ms.campaign, ms.locationID = c, loc.ID
	materialiseFactions(c, loc, ms.level, ms.scenario)
	p := loc.PresenceByID("fp1")
	name := factionSettlementName(p, loc.ID)
	start := len(settlementWorkers(ms.level, name))
	sawLarva, sawEgg := false, false
	for tick := 1; tick <= 4000; tick++ {
		fspath.ResetFrameCounter()
		ms.stepWorld()
		if ms.tick%30 == 0 {
			ms.purgeCompletedTasks()
		}
		sawLarva = sawLarva || len(hiveLarvae(ms.level, name)) > 0
		for _, e := range hiveBodies(ms.level, p.ID, name) {
			if e.Blueprint == "xeno_egg" {
				sawEgg = true
			}
		}
		if len(settlementWorkers(ms.level, name)) > start {
			break
		}
	}
	dir := ms.factionDirectors[name]
	if !sawLarva || !sawEgg || dir == nil || dir.Ordered["grow:drone"] == 0 {
		t.Fatalf("larva %v egg %v director %v", sawLarva, sawEgg, dir)
	}
	if n := len(settlementWorkers(ms.level, name)); n <= start {
		t.Fatalf("workers %d, started %d: no egg grew into a drone", n, start)
	}
}

// exterminate_xenos is not won while a hive's drones or grey eggs live.
func TestExterminateCountsTheHive(t *testing.T) {
	ms, _, _, _ := loreBuild(t, loreSite{Map: "moon", Scenario: "exterminate_xenos", Seed: 3, Chapter: "impact_colony"})
	for _, e := range append([]*ecs.Entity(nil), ms.level.Entities...) {
		if components.KinOf(e) != nil {
			ms.level.RemoveEntity(e)
		}
	}
	drone, _ := factory.Create("xeno_crew", 5, 5, ms.level.SurfaceZ)
	ms.level.AddEntity(drone)
	ms.level.Flags[loreThreatSeenFlag] = 1.0
	ms.checkThreatCleared()
	if ms.level.Flags[loreThreatClearedFlag] != nil {
		t.Error("cleared while a drone lives")
	}
	ms.level.RemoveEntity(drone)
	larva, _ := factory.Create("hive_larva", 5, 5, ms.level.SurfaceZ)
	ms.level.AddEntity(larva)
	ms.checkThreatCleared()
	if ms.level.Flags[loreThreatClearedFlag] != nil {
		t.Error("cleared while a grey egg lies there")
	}
}

// A hive on the site in orbit: built when the level is entered, frozen and
// thawed without losing or doubling anything (bodies by type, larvae, stock).
func TestHiveFreezeThaw(t *testing.T) {
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Hive", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	loc := c.Locations[c.StartID]
	loc.Factions = append(loc.Factions, campaign.FactionPresence{
		ID: "fph", Faction: "hive", Doctrine: "hive", Population: 10, Stage: 1, Larvae: 2,
		KnownTechs: []string{"hive_warrior_brood"}, Stockpile: map[string]int{"biomass": 30},
		Seed: 5*31 + 7, LastShotDay: campaign.NeverFired,
	})
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
	p := loc.PresenceByID("fph")
	name := factionSettlementName(p, loc.ID)
	before := hiveThings(ms.level, name)
	units, pop, larvae := fmt.Sprint(p.Units), p.Population, p.Larvae // Tier 1 stepped it while the ship flew
	if len(hiveBodies(ms.level, "fph", name)) != pop || len(hiveLarvae(ms.level, name)) != larvae || larvae == 0 {
		t.Fatalf("built %d bodies, %d larvae: %v units %v", len(hiveBodies(ms.level, "fph", name)), len(hiveLarvae(ms.level, name)), before, p.Units)
	}
	if err := wm.Freeze(loc, ms); err != nil {
		t.Fatal(err)
	}
	p = loc.PresenceByID("fph")
	if p == nil || p.Population != pop || fmt.Sprint(p.Units) != units || p.Larvae != larvae || p.Stockpile["biomass"] == 0 {
		t.Fatalf("after freeze %+v (units %s)", p, units)
	}
	thawed, err := wm.buildParked(loc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after := hiveThings(thawed.level, name); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("thaw changed the hive:\n%v\n%v", before, after)
	}
}

// The player's own hive bodies are never the scenario's threat: a drone and
// a larva of the player's settlement neither keep exterminate_xenos from
// clearing nor count toward the GameMaster's hostile_max, while a rival
// hive's drone does both (phase F; before it, every settled body counted).
func TestPlayersHiveIsNoThreat(t *testing.T) {
	ms, _, _, _ := loreBuild(t, loreSite{Map: "moon", Scenario: "exterminate_xenos", Seed: 3, Chapter: "impact_colony"})
	for _, e := range append([]*ecs.Entity(nil), ms.level.Entities...) {
		if components.KinOf(e) != nil || e.HasComponent(components.ScriptedAI) || e.HasComponent(components.FactionAI) {
			ms.level.RemoveEntity(e)
		}
	}
	for _, e := range append([]*ecs.Entity(nil), ms.level.StaticEntities...) {
		if components.KinOf(e) != nil {
			ms.level.RemoveEntity(e)
		}
	}
	own := ms.colonyName()
	if own == "" {
		t.Fatal("no player settlement name")
	}
	base := ms.gm.hostiles()
	drone, _ := factory.Create("xeno_crew", 5, 5, ms.level.SurfaceZ)
	enrolHiveWorker(drone, own)
	larva, _ := factory.Create("hive_larva", 6, 5, ms.level.SurfaceZ)
	if k := components.KinOf(larva); k != nil {
		k.Settlement = own
	}
	ms.level.AddEntity(drone)
	ms.level.AddEntity(larva)
	if n := ms.gm.hostiles(); n != base {
		t.Errorf("hostiles %d with the player's drone, want %d", n, base)
	}
	ms.level.Flags[loreThreatSeenFlag] = 1.0
	ms.checkThreatCleared()
	if ms.level.Flags[loreThreatClearedFlag] == nil {
		t.Error("the player's own drone and larva kept the threat from clearing")
	}

	delete(ms.level.Flags, loreThreatClearedFlag)
	rival, _ := factory.Create("xeno_crew", 8, 5, ms.level.SurfaceZ)
	enrolHiveWorker(rival, "hive:rival")
	ms.level.AddEntity(rival)
	if n := ms.gm.hostiles(); n != base+1 {
		t.Errorf("hostiles %d with a rival drone, want %d", n, base+1)
	}
	ms.checkThreatCleared()
	if ms.level.Flags[loreThreatClearedFlag] != nil {
		t.Error("cleared while a rival hive's drone lives")
	}
}
