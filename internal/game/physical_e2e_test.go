package game

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/factory"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/workerai"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// The cost registries read data/ at package init, before TestMain moves to the repo
// root, so they are empty here. These tests load the same files into the worker
// AI's lookups, so they run on real costs.
func useRealCosts(t *testing.T) (map[string]construction.Buildable, map[string]crafting.Recipe) {
	t.Helper()
	load := func(path string, into any) {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if err := json.Unmarshal(raw, into); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
	}
	builds := map[string]construction.Buildable{}
	recipes := map[string]crafting.Recipe{}
	techs := map[string]research.Tech{}
	load("data/build.json", &builds)
	load("data/crafting_recipes.json", &recipes)
	load("data/research.json", &techs)
	t.Cleanup(workerai.SetCostLookups(
		func(k string) construction.Buildable { return builds[k] },
		func(k string) (crafting.Recipe, bool) { r, ok := recipes[k]; return r, ok },
		func(k string) (research.Tech, bool) { r, ok := techs[k]; return r, ok },
	))
	return builds, recipes
}

// e2eColony is a small hand-built level running under a real MainState: a floor,
// one colonist with an empty bag at the west end, and whatever containers the test
// places.
type e2eColony struct {
	wm       *WorldManager
	ms       *MainState
	colonist *ecs.Entity
	name     string
}

const (
	e2eWidth  = 30
	e2eHeight = 5
	e2eTicks  = 5000
)

func newE2EColony(t *testing.T) *e2eColony {
	t.Helper()
	wm := shipTestWorld(t)
	name := campaignColonyName(wm.Campaign)

	lvl := world.NewLevel(e2eWidth, e2eHeight, 2)
	lvl.AllocTerrain()
	for x := 0; x < e2eWidth; x++ {
		for y := 0; y < e2eHeight; y++ {
			lvl.SetFloor(x, y, 0, "hull_floor", 0)
		}
	}
	ms, err := newMainStateFromLevel(lvl, SettlementConfig{Name: name, ColonyName: name, CampaignMode: true})
	if err != nil {
		t.Fatalf("main state: %v", err)
	}
	ms.gm.suppressSpawns = true
	t.Cleanup(func() { ms.teardown() })
	c := &e2eColony{wm: wm, ms: ms, name: name}
	c.colonist = c.addColonist(t, 1, 2)
	return c
}

func (c *e2eColony) addColonist(t *testing.T, x, y int) *ecs.Entity {
	t.Helper()
	e, err := factory.Create("colonist", x, y, 0)
	if err != nil {
		t.Fatalf("create colonist: %v", err)
	}
	if e.HasComponent(components.Settlement) {
		e.GetComponent(components.Settlement).(*components.SettlementComponent).Name = c.name
	} else {
		e.AddComponent(&components.SettlementComponent{Name: c.name})
	}
	e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag = nil
	c.ms.level.AddEntity(e)
	return e
}

// addChest places a real storage locker holding the given material stacks.
func (c *e2eColony) addChest(t *testing.T, x, y int, stock map[string]int) *components.StorageComponent {
	t.Helper()
	e, err := factory.Create("storage_locker", x, y, 0)
	if err != nil {
		t.Fatalf("create locker: %v", err)
	}
	sc := e.GetComponent(components.Storage).(*components.StorageComponent)
	sc.OwnedBy = c.name
	for name, qty := range stock {
		item, err := factory.Create(name, 0, 0, 0)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		item.GetComponent(components.Material).(*components.MaterialComponent).Quantity = qty
		if !sc.AddItem(item) {
			t.Fatalf("locker refused %s", name)
		}
	}
	c.ms.level.AddEntity(e)
	return sc
}

// queueCraft queues a task shaped like addCraftTask's, at a real bench entity.
func (c *e2eColony) queueCraft(t *testing.T, recipeID string, recipe crafting.Recipe, x, y int) *task.Task {
	t.Helper()
	bench, err := factory.Create("basic_workbench", x, y, 0)
	if err != nil {
		t.Fatalf("create bench: %v", err)
	}
	c.ms.level.AddEntity(bench)
	tk := &task.Task{
		Action: task_requests.CraftAction,
		X:      x, Y: y, Z: 0,
		Data: task_requests.CraftRequest{
			RecipeID: recipeID, WorkbenchX: x, WorkbenchY: y, WorkbenchZ: 0, Required: recipe.BuildTime,
		},
	}
	c.ms.MainSettlement.Tasks.AddTask(tk)
	return tk
}

func (c *e2eColony) worker() *components.WorkerComponent {
	return c.colonist.GetComponent(components.Worker).(*components.WorkerComponent)
}

func (c *e2eColony) bag() []*ecs.Entity {
	return c.colonist.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag
}

func (c *e2eColony) bagUnits(blueprint string) int {
	n := 0
	for _, item := range c.bag() {
		if item.Blueprint != blueprint {
			continue
		}
		if item.HasComponent(components.Material) {
			n += item.GetComponent(components.Material).(*components.MaterialComponent).Quantity
		} else {
			n++
		}
	}
	return n
}

func (c *e2eColony) stock() map[string]int {
	return storage.Summarize(storage.LevelProvider{Level: c.ms.level}, []string{c.name})
}

// tickUntil steps the real world until done holds, failing with the worker's state
// at the cap. It returns the ticks taken.
func (c *e2eColony) tickUntil(t *testing.T, what string, done func() bool) int {
	t.Helper()
	for i := 0; i < e2eTicks; i++ {
		if done() {
			return i
		}
		fspath.ResetFrameCounter() // the game's Update does this before each step
		c.ms.stepWorld()
	}
	mem := c.colonist.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	t.Fatalf("%s: not reached in %d ticks; state=%q task=%+v bag=%d stock=%v",
		what, e2eTicks, mem.State, c.worker().CurrentTask, len(c.bag()), c.stock())
	return 0
}

func craftDelivered(tk *task.Task) map[string]int {
	return tk.Data.(task_requests.CraftRequest).Delivered
}

func total(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

// One chest, one colonist, the recipe's exact inputs. A load this size fits in one
// trip and is delivered in one tick, so the hover goes unpaid 0/n straight to paid.
func TestE2E_SingleChestCraft(t *testing.T) {
	_, recipes := useRealCosts(t)
	c := newE2EColony(t)
	const id = "pistol_shield"
	recipe := recipes[id]
	chest := c.addChest(t, 2, 0, recipe.Cost)
	benchX, benchY := e2eWidth-2, 2
	tk := c.queueCraft(t, id, recipe, benchX, benchY)

	sawUnpaid, paidBeforeUnpaid, sawPaid := false, false, false
	ticks := c.tickUntil(t, "craft completes", func() bool {
		needs, paid := c.ms.taskNeedsAt(benchX, benchY, 0)
		if !tk.Completed {
			if needs != nil {
				sawUnpaid = true
				if sawPaid {
					paidBeforeUnpaid = true
				}
				if len(needs) != len(recipe.Cost) {
					t.Fatalf("hover lists %d inputs, want %d", len(needs), len(recipe.Cost))
				}
			}
			sawPaid = sawPaid || paid
		}
		return tk.Completed
	})
	t.Logf("completed in %d ticks", ticks)

	if !sawUnpaid || !sawPaid || paidBeforeUnpaid {
		t.Errorf("hover sequence: unpaid=%v paid=%v paidBeforeUnpaid=%v, want unpaid then paid", sawUnpaid, sawPaid, paidBeforeUnpaid)
	}
	for name := range recipe.Cost {
		if got := chest.CountResource(name); got != 0 {
			t.Errorf("chest still holds %d %s, want exactly the cost removed", got, name)
		}
		if got := c.bagUnits(name); got != 0 {
			t.Errorf("bag still holds %d %s", got, name)
		}
	}
	if c.bagUnits(recipe.Output) != 1 || len(c.bag()) != 1 {
		t.Errorf("bag = %d items, want only the crafted %s", len(c.bag()), recipe.Output)
	}
}

// print_colonist costs 25 biomass against a carry limit of 20, split 15 + 10.
func TestE2E_TwoChestTwoTripCraft(t *testing.T) {
	_, recipes := useRealCosts(t)
	c := newE2EColony(t)
	const id = "print_colonist"
	recipe := recipes[id]
	if recipe.Cost["biomass"] <= workerai.CarryCapacity(c.colonist) {
		t.Fatalf("recipe no longer exceeds the carry limit: %v", recipe.Cost)
	}
	first := c.addChest(t, 2, 0, map[string]int{"biomass": 15})
	second := c.addChest(t, 12, 0, map[string]int{"biomass": 10})
	benchX, benchY := e2eWidth-2, 2
	tk := c.queueCraft(t, id, recipe, benchX, benchY)
	colonists := countLevelColonists(c.ms.level)

	var tallies []int
	ticks := c.tickUntil(t, "craft completes", func() bool {
		d := total(craftDelivered(tk))
		if len(tallies) == 0 || tallies[len(tallies)-1] != d {
			tallies = append(tallies, d)
		}
		return tk.Completed
	})
	t.Logf("completed in %d ticks; tally went %v", ticks, tallies)

	if first.CountResource("biomass") != 0 || second.CountResource("biomass") != 0 {
		t.Errorf("chests hold %d and %d, want both drained", first.CountResource("biomass"), second.CountResource("biomass"))
	}
	if len(tallies) != 3 || tallies[1] != 20 || tallies[2] != 25 {
		t.Errorf("tally went %v, want 0, 20 (full bag), 25", tallies)
	}
	if got := craftDelivered(tk)["biomass"]; got != 25 {
		t.Errorf("delivered %d, want 25", got)
	}
	if got := countLevelColonists(c.ms.level); got != colonists+1 {
		t.Errorf("%d colonists, want the printed one added to %d", got, colonists)
	}
	if c.bagUnits("biomass") != 0 {
		t.Errorf("bag still holds %d biomass", c.bagUnits("biomass"))
	}
}

// startPartialBuild runs a robot_workbench build (8 ore + 4 crystal) with a bag
// nearly full of gear, so loads are small, until part is delivered and the colonist
// is carrying another load.
func startPartialBuild(t *testing.T, builds map[string]construction.Buildable) (*e2eColony, *task.Task, map[string]int) {
	t.Helper()
	c := newE2EColony(t)
	const kind = "robot_workbench"
	inv := c.colonist.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for i := 0; i < 15; i++ {
		knife, err := factory.Create("knife", 0, 0, 0)
		if err != nil {
			t.Fatalf("create knife: %v", err)
		}
		inv.AddItem(knife)
	}
	c.addChest(t, 2, 0, map[string]int{"metal_ore": 20, "crystal": 10})
	before := c.stock()

	x, y := e2eWidth-3, 2
	tk := &task.Task{Action: task_requests.BuildAction, X: x, Y: y, Z: 0,
		Data: task_requests.BuildRequest{Type: kind, X: x, Y: y, Z: 0, Required: builds[kind].BuildTime}}
	c.ms.MainSettlement.Tasks.AddTask(tk)

	ticks := c.tickUntil(t, "partly delivered with a load in the bag", func() bool {
		d := total(tk.Data.(task_requests.BuildRequest).Delivered)
		return d > 0 && d < total(builds[kind].Cost) && c.bagUnits("metal_ore")+c.bagUnits("crystal") > 0
	})
	t.Logf("partial after %d ticks: delivered %v", ticks, tk.Data.(task_requests.BuildRequest).Delivered)
	return c, tk, before
}

func assertMaterialsReturned(t *testing.T, c *e2eColony, tk *task.Task, before map[string]int) {
	t.Helper()
	delivered := tk.Data.(task_requests.BuildRequest).Delivered
	ticks := c.tickUntil(t, "carried materials back in storage", func() bool {
		return c.bagUnits("metal_ore")+c.bagUnits("crystal") == 0
	})
	t.Logf("returned in %d ticks", ticks)
	stock := c.stock()
	for name, qty := range before {
		if want := qty - delivered[name]; stock[name] != want {
			t.Errorf("%s stock = %d, want %d (%d before, %d lost to the cancelled task)", name, stock[name], want, qty, delivered[name])
		}
	}
	if c.bagUnits("knife") != 15 {
		t.Errorf("gear went with the materials: %d knives left", c.bagUnits("knife"))
	}
}

// Stop as the game's own call sites use it: Stop alone leaves a worker holding the
// task, so each one also clears CurrentTask. Removing the task keeps it from being
// picked up again once its short cooldown ends.
func TestE2E_CancelMidDelivery_Stop(t *testing.T) {
	builds, _ := useRealCosts(t)
	c, tk, before := startPartialBuild(t, builds)

	tk.Stop()
	c.worker().CurrentTask = nil
	c.ms.MainSettlement.Tasks.RemoveTask(tk)

	assertMaterialsReturned(t, c, tk, before)
}

// The player's cancel action: Complete and remove, leaving the worker's pointer.
func TestE2E_CancelMidDelivery_PlayerAction(t *testing.T) {
	builds, _ := useRealCosts(t)
	c, tk, before := startPartialBuild(t, builds)

	tk.Complete()
	c.ms.MainSettlement.Tasks.RemoveTask(tk)

	assertMaterialsReturned(t, c, tk, before)
}

// Freeze with 20 of 25 biomass delivered and the last 5 in the bag, thaw from disk,
// and finish on the restored queue and restored bag.
func TestE2E_SaveAndLoadMidGather(t *testing.T) {
	_, recipes := useRealCosts(t)
	c := newE2EColony(t)
	const id = "print_colonist"
	chest := c.addChest(t, 2, 0, map[string]int{"biomass": 30})
	benchX, benchY := e2eWidth-2, 2
	tk := c.queueCraft(t, id, recipes[id], benchX, benchY)

	ticks := c.tickUntil(t, "tally partly filled with a load in the bag", func() bool {
		return craftDelivered(tk)["biomass"] == 20 && c.bagUnits("biomass") > 0
	})
	t.Logf("froze after %d ticks", ticks)
	if chest.CountResource("biomass") != 5 {
		t.Fatalf("chest holds %d at freeze, want 5", chest.CountResource("biomass"))
	}

	if err := c.wm.Freeze(c.ms); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	c.ms.teardown()
	c.wm.current = nil
	if err := c.wm.EnterStartingOrbit(); err != nil {
		t.Fatalf("thaw: %v", err)
	}
	ms, err := c.wm.EnterCurrent()
	if err != nil {
		t.Fatalf("enter: %v", err)
	}
	c.ms = ms
	c.ms.gm.suppressSpawns = true
	c.colonist = nil
	for _, e := range ms.level.Entities {
		if e.Blueprint == "colonist" {
			c.colonist = e
		}
	}
	if c.colonist == nil {
		t.Fatal("colonist not restored")
	}
	if c.bagUnits("biomass") != 5 {
		t.Errorf("bag holds %d biomass after thaw, want the 5 carried at freeze", c.bagUnits("biomass"))
	}

	var restored *task.Task
	for _, q := range ms.MainSettlement.Tasks.GetTasks() {
		if q.Action == task_requests.CraftAction {
			restored = q
		}
	}
	if restored == nil {
		t.Fatal("craft task not restored")
	}
	if got := craftDelivered(restored)["biomass"]; got != 20 {
		t.Errorf("tally after thaw = %d, want 20", got)
	}

	ticks = c.tickUntil(t, "restored craft completes", func() bool { return restored.Completed })
	t.Logf("completed %d ticks after thaw", ticks)

	if got := craftDelivered(restored)["biomass"]; got != 25 {
		t.Errorf("delivered %d, want 25", got)
	}
	if got := c.stock()["biomass"]; got != 5 {
		t.Errorf("stock = %d, want 5: 30 less exactly the 25 cost", got)
	}
	if c.bagUnits("biomass") != 0 {
		t.Errorf("bag still holds %d biomass", c.bagUnits("biomass"))
	}
}
