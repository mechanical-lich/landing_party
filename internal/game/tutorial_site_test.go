package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// loadTutorialData loads the real maps, biomes and scenarios so newGame can
// resolve the Training Ground. Scenarios are unloaded again afterwards, as the
// other level tests do, so no test leaks an active scenario.
func loadTutorialData(t *testing.T) {
	t.Helper()
	if err := mapdef.Load("data/maps"); err != nil {
		t.Fatalf("load maps: %v", err)
	}
	if err := generation.LoadBiomes("data/biomes"); err != nil {
		t.Fatalf("load biomes: %v", err)
	}
	if err := scenario.Load("data/scenarios"); err != nil {
		t.Fatalf("load scenarios: %v", err)
	}
	empty := t.TempDir()
	t.Cleanup(func() { _ = scenario.Load(empty) })
}

// isolateSettlements gives the test its own settlement map without leaving the
// repo root (the setup script is read by relative path).
func isolateSettlements(t *testing.T) {
	t.Helper()
	prev := settlement.Settlements
	settlement.Settlements = make(map[string]*settlement.Settlement)
	t.Cleanup(func() { settlement.Settlements = prev })
}

// tutorialSite generates the Training Ground the way buildParked does for an
// unvisited location: through NewMainState with the map and scenario fixed.
func tutorialSite(t *testing.T, seed int64) *MainState {
	t.Helper()
	const colony = "Tutorial Colony @ Training Ground"
	ms, err := NewMainState(SettlementConfig{
		Name: colony, ColonyName: colony, MapID: "tutorial", ScenarioID: "tutorial",
		Seed: seed, CampaignMode: true,
	})
	if err != nil {
		t.Fatalf("tutorial site: %v", err)
	}
	t.Cleanup(ms.teardown)
	return ms
}

func tutorialPad(ms *MainState) *ecs.Entity {
	for _, ents := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range ents {
			if e != nil && e.Blueprint == "teleporter" {
				return e
			}
		}
	}
	return nil
}

func countBlueprint(ms *MainState, bp string) int {
	n := 0
	for _, e := range ms.level.Entities {
		if e != nil && e.Blueprint == bp && !e.HasComponent(rlcomponents.Dead) {
			n++
		}
	}
	return n
}

func tileName(ms *MainState, x, y, z int) string {
	tI := ms.level.GetTileAt(x, y, z)
	if tI == nil {
		return ""
	}
	t := tI.(*world.Tile)
	if t.Middle.IsEmpty() {
		return ""
	}
	return world.TileDefinitions[t.Middle.Type].Name
}

func tickSite(t *testing.T, ms *MainState, max int, what string, done func() bool) int {
	t.Helper()
	for i := 0; i < max; i++ {
		if done() {
			return i
		}
		fspath.ResetFrameCounter()
		ms.stepWorld()
	}
	t.Fatalf("%s: not reached in %d ticks", what, max)
	return 0
}

// The Training Ground lands at the map centre on every seed, so everything the
// script places relative to the plaza is where the prompts say it is.
func TestTutorialSiteLandsAtCentre(t *testing.T) {
	loadTutorialData(t)
	isolateSettlements(t)
	for seed := int64(1); seed <= 5; seed++ {
		ms := tutorialSite(t, seed)
		if ms.MainSettlement == nil {
			t.Fatalf("seed %d: no settlement", seed)
		}
		if ms.MainSettlement.CenterX != 32 || ms.MainSettlement.CenterY != 32 {
			t.Errorf("seed %d: plaza at (%d,%d), want (32,32)", seed, ms.MainSettlement.CenterX, ms.MainSettlement.CenterY)
		}
		if ms.MainSettlement.CenterZ != ms.level.SurfaceZ {
			t.Errorf("seed %d: plaza z %d, want surface %d", seed, ms.MainSettlement.CenterZ, ms.level.SurfaceZ)
		}
		if ms.level.GetWidth() != 64 || ms.level.GetHeight() != 64 || ms.level.GetDepth() != 10 {
			t.Errorf("seed %d: map %dx%dx%d, want 64x64x10", seed, ms.level.GetWidth(), ms.level.GetHeight(), ms.level.GetDepth())
		}
	}
}

// The site script does everything the tutorial relies on: a colony-owned pad,
// deposits a Mine order accepts, rations on the ground, tagged regions, and no
// hostile ever appears.
func TestTutorialSiteScript(t *testing.T) {
	loadTutorialData(t)
	isolateSettlements(t)
	ms := tutorialSite(t, 3)
	sx, sy, sz := ms.MainSettlement.CenterX, ms.MainSettlement.CenterY, ms.MainSettlement.CenterZ

	pad := tutorialPad(ms)
	if pad == nil {
		t.Fatal("no teleporter pad on site")
	}
	sc := pad.GetComponent(components.Storage).(*components.StorageComponent)
	if sc.OwnedBy != ms.colonyName() {
		t.Fatalf("pad owned by %q, want the colony %q", sc.OwnedBy, ms.colonyName())
	}
	if pad != siteHold(ms) {
		t.Fatal("the pad is not the site's hold container")
	}

	ore, crystal := 0, 0
	for x := sx + 5; x <= sx+10; x++ {
		for y := sy - 3; y <= sy+2; y++ {
			switch tileName(ms, x, y, sz) {
			case "ore_deposit":
				ore++
			case "crystal_vein":
				crystal++
			case "rock":
			default:
				t.Errorf("outcrop tile (%d,%d) is %q", x, y, tileName(ms, x, y, sz))
			}
		}
	}
	if ore < 4 || crystal < 4 {
		t.Errorf("outcrop has %d ore and %d crystal tiles, want at least 4 each", ore, crystal)
	}
	// The uniform biome paints a walkable grass surface around the plaza.
	for _, p := range [][2]int{{sx - 2, sy}, {sx + 2, sy}, {sx, sy - 6}, {sx, sy + 8}} {
		tI := ms.level.GetTileAt(p[0], p[1], sz)
		if tI == nil {
			t.Fatalf("no tile at %v", p)
		}
		tile := tI.(*world.Tile)
		if tile.Floor.IsEmpty() || world.TileDefinitions[tile.Floor.Type].Name != "grass" || !tile.Middle.IsEmpty() {
			t.Errorf("surface at %v is not open grass", p)
		}
	}

	ms.camera.Z = sz
	if !ms.addMineTask(sx+5, sy) {
		t.Error("Mine order refused on the ore tile")
	}
	if !ms.addMineTask(sx+5, sy+1) {
		t.Error("Mine order refused on the crystal tile")
	}
	if ms.addMineTask(sx+10, sy-3) {
		t.Error("Mine order accepted on plain rock")
	}

	if n := countBlueprint(ms, "ration_pack"); n != 12 {
		t.Errorf("%d rations on the ground, want 12", n)
	}
	for _, tag := range []string{"tut_pad", "tut_outcrop", "tut_rations"} {
		if len(ms.level.Regions[tag]) == 0 {
			t.Errorf("region %q not tagged", tag)
		}
	}
	if ms.scenario == nil || ms.scenario.HostileMax != 0 {
		t.Fatalf("scenario = %+v, want hostile_max 0", ms.scenario)
	}

	for i := 0; i < 10000; i++ {
		fspath.ResetFrameCounter()
		ms.stepWorld()
	}
	for _, e := range ms.level.Entities {
		if components.IsAttackTarget(e) {
			t.Fatalf("a hostile %q spawned on the Training Ground", e.Blueprint)
		}
	}
}

// siteHold mirrors WorldManager.SiteHoldEntity for a bare MainState.
func siteHold(ms *MainState) *ecs.Entity {
	for _, ents := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range ents {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			if e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy == ms.colonyName() {
				return e
			}
		}
	}
	return nil
}

// The bootstrap the first planet depends on: two colonists build a locker from
// metal in the pad, mine the west face, and the ore and crystal reach storage.
// A clicked ration is retrieved into the pad too.
func TestTutorialSiteBootstrap(t *testing.T) {
	builds, _ := useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	ms := tutorialSite(t, 4)
	ms.gm.suppressSpawns = true
	sx, sy, sz := ms.MainSettlement.CenterX, ms.MainSettlement.CenterY, ms.MainSettlement.CenterZ
	colony := ms.colonyName()

	for i := 0; i < 2; i++ {
		e, err := factory.Create("colonist", sx-1+i*2, sy, sz)
		if err != nil {
			t.Fatal(err)
		}
		tagColonistSettlement(e, colony)
		e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag = nil
		ms.level.AddEntity(e)
	}
	pad := tutorialPad(ms)
	padSc := pad.GetComponent(components.Storage).(*components.StorageComponent)
	ore, _ := factory.Create("metal_ore", 0, 0, 0)
	ore.GetComponent(components.Material).(*components.MaterialComponent).Quantity = 20
	if !padSc.AddItem(ore) {
		t.Fatal("pad refused metal")
	}
	stock := func() map[string]int {
		return storage.Summarize(storage.LevelProvider{Level: ms.level}, []string{colony})
	}

	// Locker two tiles north of the plaza.
	lockerCost := builds["storage_locker"].Cost["metal_ore"]
	if lockerCost <= 0 || lockerCost > 20 {
		t.Fatalf("storage_locker metal cost %d, test assumes 1..20", lockerCost)
	}
	lx, ly := sx, sy-2
	ms.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.BuildAction, X: lx, Y: ly, Z: sz,
		Data: task_requests.BuildRequest{Type: "storage_locker", X: lx, Y: ly, Z: sz, Required: builds["storage_locker"].BuildTime},
	})
	ticks := tickSite(t, ms, 6000, "locker built", func() bool { return countStatic(ms, "storage_locker") == 1 })
	t.Logf("locker built at tick %d", ticks)
	if got := stock()["metal_ore"]; got != 20-lockerCost {
		t.Errorf("pad holds %d metal after the build, want %d", got, 20-lockerCost)
	}

	// Click a ration while the crew is idle: the worker retrieves it and drops
	// it in storage. (A worker mid-way through a deposit does not break off for
	// it, so the tutorial teaches hauling before mining.)
	var ration *ecs.Entity
	for _, e := range ms.level.Entities {
		if e.Blueprint == "ration_pack" {
			ration = e
			break
		}
	}
	rpc := ration.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	ms.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.RetrieveAction,
		Data:   task_requests.RetrieveRequest{Item: ration},
		X:      rpc.GetX(), Y: rpc.GetY(), Z: rpc.GetZ(), Escalated: true,
	})
	ticks = tickSite(t, ms, 4000, "ration stored", func() bool { return stock()["ration_pack"] >= 1 })
	t.Logf("ration stored at tick %d", ticks)
	if n := countBlueprint(ms, "ration_pack"); n != 11 {
		t.Errorf("%d rations left on the ground, want 11", n)
	}
	if ms.buildEvalContext().ResourceCounts["food"] < 1 {
		t.Error("eval context does not count the stored ration as food")
	}

	// Mine one ore and one crystal tile from the west face. Deposits hold many
	// units (25-100 ore), so the first of each reaching storage is the check.
	ms.camera.Z = sz
	ms.addMineTask(sx+5, sy)
	ms.addMineTask(sx+5, sy+1)
	before := stock()
	ticks = tickSite(t, ms, 8000, "ore and crystal stored", func() bool {
		s := stock()
		return s["metal_ore"] > before["metal_ore"] && s["crystal"] > 0
	})
	t.Logf("cargo stored at tick %d: %v", ticks, stock())
}

func countStatic(ms *MainState, bp string) int {
	n := 0
	for _, ents := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range ents {
			if e != nil && e.Blueprint == bp {
				n++
			}
		}
	}
	return n
}
