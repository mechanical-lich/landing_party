package workerai

import (
	"testing"
	"time"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/lore"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeRecipe(t *testing.T, cost map[string]int) {
	t.Helper()
	prev := getRecipe
	getRecipe = func(string) (crafting.Recipe, bool) {
		return crafting.Recipe{Name: "Test Recipe", Cost: cost, Output: "metal_ore"}, true
	}
	t.Cleanup(func() { getRecipe = prev })
}

func fakeTech(t *testing.T, cost map[string]int) {
	t.Helper()
	prev := getTech
	getTech = func(string) (research.Tech, bool) {
		return research.Tech{Name: "Test Tech", Cost: cost}, true
	}
	t.Cleanup(func() { getTech = prev })
}

func craftTask(x, y, required int) *task.Task {
	return &task.Task{X: x, Y: y, Action: task_requests.CraftAction, InProgress: true,
		Data: task_requests.CraftRequest{RecipeID: "r", WorkbenchX: x, WorkbenchY: y, Required: required}}
}

func colony(t *testing.T) *settlement.Settlement {
	t.Helper()
	s := &settlement.Settlement{Name: testColony}
	settlement.Settlements[testColony] = s
	t.Cleanup(func() { delete(settlement.Settlements, testColony) })
	return s
}

func TestCraft_FullLoopGathersDeliversThenWorks(t *testing.T) {
	fakeRecipe(t, map[string]int{"metal_ore": 2, "crystal": 1})
	lvl := physicalLevel(t, 12, 3)
	chest := physicalChest(lvl, 1, 1, materialStack("metal_ore", 5), materialStack("crystal", 5))
	e, wc, mem := physicalWorker(lvl, 2, 1)
	tk := craftTask(8, 1, 3)
	wc.CurrentTask = tk
	data := func() task_requests.CraftRequest { return tk.Data.(task_requests.CraftRequest) }

	// Standing at the chest with nothing in hand, the worker gathers rather than
	// having the cost pulled from the pool.
	HandleTaskState(lvl, e)
	assert.Equal(t, "gather_materials", mem.State)
	assert.Equal(t, 5, chest.CountResource("metal_ore"))

	for i := 0; i < 40 && !tk.Completed; i++ {
		if data().Progress > 0 {
			assert.Equal(t, 2, data().Delivered["metal_ore"], "no progress before paid")
			assert.Equal(t, 1, data().Delivered["crystal"], "no progress before paid")
		}
		ticks := 0
		runUntil(t, lvl, e, mem, 1, func() bool { ticks++; return ticks > 1 })
	}
	assert.True(t, tk.Completed)
	assert.Equal(t, 3, chest.CountResource("metal_ore"), "completion deducts nothing further")
	assert.Equal(t, 4, chest.CountResource("crystal"))
}

func TestCraft_NoProgressOnPartialDelivery(t *testing.T) {
	fakeRecipe(t, map[string]int{"metal_ore": 5})
	lvl := physicalLevel(t, 6, 6)
	e, wc, mem := physicalWorker(lvl, 2, 2, materialStack("metal_ore", 3))
	tk := craftTask(3, 2, 100)
	wc.CurrentTask = tk

	HandleTaskState(lvl, e)
	HandleTaskState(lvl, e)
	cr := tk.Data.(task_requests.CraftRequest)
	assert.Equal(t, 3, cr.Delivered["metal_ore"])
	assert.Equal(t, 0, cr.Progress)
	assert.Equal(t, "gather_materials", mem.State)
}

func TestCraft_TwoWorkersShareTallyAndNeitherDoublePays(t *testing.T) {
	fakeRecipe(t, map[string]int{"metal_ore": 5})
	lvl := physicalLevel(t, 6, 6)
	a, wcA, _ := physicalWorker(lvl, 2, 2, materialStack("metal_ore", 3))
	b, wcB, _ := physicalWorker(lvl, 4, 2, materialStack("metal_ore", 4))
	tk := craftTask(3, 2, 100)
	wcA.CurrentTask, wcB.CurrentTask = tk, tk

	HandleTaskState(lvl, a)
	HandleTaskState(lvl, b)
	cr := tk.Data.(task_requests.CraftRequest)
	assert.Equal(t, 5, cr.Delivered["metal_ore"])
	assert.Equal(t, 0, bagCount(a, "metal_ore"))
	assert.Equal(t, 2, bagCount(b, "metal_ore"), "only the shortfall is taken")

	// Both now work the paid task and nothing more leaves either bag.
	HandleTaskState(lvl, a)
	HandleTaskState(lvl, b)
	assert.Equal(t, 2, tk.Data.(task_requests.CraftRequest).Progress)
	assert.Equal(t, 2, bagCount(b, "metal_ore"))
}

func researchTask(x, y int, progress int) (*task.Task, *task_requests.ResearchRequest) {
	rr := &task_requests.ResearchRequest{TechKey: "k", Required: 100, Progress: progress}
	return &task.Task{X: x, Y: y, Action: task_requests.ResearchAction, InProgress: true, Data: rr}, rr
}

func TestResearch_NoProgressUntilDeliveredCoversCost(t *testing.T) {
	fakeTech(t, map[string]int{"crystal": 3})
	lvl := physicalLevel(t, 6, 6)
	e, wc, mem := physicalWorker(lvl, 2, 2, materialStack("crystal", 1))
	tk, rr := researchTask(3, 2, 0)
	wc.CurrentTask = tk

	HandleTaskState(lvl, e) // deliver 1
	HandleTaskState(lvl, e) // out of crystal, leaves to gather
	assert.Equal(t, 1, rr.Delivered["crystal"])
	assert.Equal(t, 0, rr.Progress)
	assert.Equal(t, "gather_materials", mem.State)

	mem.State = "task"
	rr.Delivered["crystal"] = 3
	HandleTaskState(lvl, e)
	assert.Greater(t, rr.Progress, 0)
}

func TestResearch_InProgressFromOldSaveIsNotChargedAgain(t *testing.T) {
	fakeTech(t, map[string]int{"crystal": 3})
	lvl := physicalLevel(t, 6, 6)
	e, wc, mem := physicalWorker(lvl, 2, 2)
	tk, rr := researchTask(3, 2, 10) // paid from the pool under the old rules
	wc.CurrentTask = tk

	HandleTaskState(lvl, e)
	assert.Equal(t, "task", mem.State)
	assert.Equal(t, 11, rr.Progress)
}

func queuedBuild(s *settlement.Settlement, x, y int) *task.Task {
	tk := buildTask(x, y, 100)
	tk.InProgress = false
	tk.Created = time.Now()
	s.Tasks.AddTask(tk)
	return tk
}

func TestPicker_SkipsUnaffordableTaskWithoutCancellingIt(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 3})
	s := colony(t)
	lvl := physicalLevel(t, 6, 6)
	physicalChest(lvl, 5, 5, materialStack("metal_ore", 2))
	e, wc, mem := physicalWorker(lvl, 2, 2)
	mem.State = "idle"
	tk := queuedBuild(s, 3, 3)

	HandleWorkerIdleState(lvl, e)
	assert.Nil(t, wc.CurrentTask)
	assert.Equal(t, "idle", mem.State)
	assert.False(t, tk.InProgress, "left free for when stock arrives")
	assert.False(t, tk.ManuallyStopped)
	assert.False(t, tk.Completed)
	assert.Contains(t, s.Tasks.GetTasks(), tk)

	// Stock arrives: the same task is now picked.
	physicalChest(lvl, 0, 0, materialStack("metal_ore", 1))
	HandleWorkerIdleState(lvl, e)
	assert.Same(t, tk, wc.CurrentTask)
	assert.True(t, tk.InProgress)
}

func TestPicker_UnaffordableNearbyTaskDoesNotBlockOtherWork(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 3})
	s := colony(t)
	lvl := physicalLevel(t, 12, 3)
	e, wc, mem := physicalWorker(lvl, 1, 1)
	mem.State = "idle"
	near := queuedBuild(s, 2, 1)
	dig := &task.Task{X: 10, Y: 1, Action: task_requests.DigAction, Created: time.Now()}
	s.Tasks.AddTask(dig)

	HandleWorkerIdleState(lvl, e)
	assert.Same(t, dig, wc.CurrentTask)
	assert.False(t, near.InProgress, "skip is only held during the search")
}

func TestPicker_CountsDeliveredAndBagTowardsCost(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 5})
	s := colony(t)
	lvl := physicalLevel(t, 6, 6)
	physicalChest(lvl, 5, 5, materialStack("metal_ore", 1))
	e, wc, mem := physicalWorker(lvl, 2, 2, materialStack("metal_ore", 2))
	mem.State = "idle"
	tk := queuedBuild(s, 3, 3)
	br := tk.Data.(task_requests.BuildRequest)
	br.Delivered = map[string]int{"metal_ore": 2}
	tk.Data = br

	HandleWorkerIdleState(lvl, e) // 2 delivered + 2 carried + 1 stored = 5
	assert.Same(t, tk, wc.CurrentTask)
}

func TestTaskNeeds_HalfDeliveredBuildListsEachMaterial(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 5, "crystal": 1})
	tk := buildTask(0, 0, 100)
	br := tk.Data.(task_requests.BuildRequest)
	br.Delivered = map[string]int{"metal_ore": 3}
	tk.Data = br

	assert.Equal(t, []MaterialNeed{
		{Blueprint: "crystal", Delivered: 0, Required: 1},
		{Blueprint: "metal_ore", Delivered: 3, Required: 5},
	}, TaskNeeds(tk))
	delivered, required := TaskDeliveryTotals(tk)
	assert.Equal(t, 3, delivered)
	assert.Equal(t, 6, required)
}

func TestTaskNeeds_PaidTaskListsNone(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 5})
	tk := buildTask(0, 0, 100)
	br := tk.Data.(task_requests.BuildRequest)
	br.Delivered = map[string]int{"metal_ore": 5}
	tk.Data = br

	assert.Nil(t, TaskNeeds(tk))
	delivered, required := TaskDeliveryTotals(tk)
	assert.Equal(t, 5, delivered)
	assert.Equal(t, 5, required)
}

func TestTaskTotals_StartedResearchReadsAsPaid(t *testing.T) {
	fakeTech(t, map[string]int{"crystal": 3})
	tk, _ := researchTask(0, 0, 10)
	assert.Nil(t, TaskNeeds(tk))
	delivered, required := TaskDeliveryTotals(tk)
	assert.Equal(t, 3, delivered)
	assert.Equal(t, 3, required)
}

func TestTaskTotals_NoCostTaskRequiresNothing(t *testing.T) {
	_, required := TaskDeliveryTotals(&task.Task{Action: task_requests.DigAction})
	assert.Equal(t, 0, required)
}

// craftOutput runs a free, one-step craft of recipe at the worker's side and
// returns the worker.
func craftOutput(t *testing.T, recipe crafting.Recipe) (*world.Level, *ecs.Entity) {
	t.Helper()
	prev := getRecipe
	getRecipe = func(string) (crafting.Recipe, bool) { return recipe, true }
	t.Cleanup(func() { getRecipe = prev })
	lore.NamesPath = "../../data/names.json" // a colonist is named at creation
	lvl := physicalLevel(t, 6, 3)
	e, wc, _ := physicalWorker(lvl, 2, 1)
	tk := craftTask(3, 1, 1)
	wc.CurrentTask = tk
	for i := 0; i < 5 && !tk.Completed; i++ {
		HandleTaskState(lvl, e)
	}
	require.True(t, tk.Completed)
	return lvl, e
}

// deploy_on_craft (finding 4): the output joins the crafter's settlement on
// the level at the station; without it the output goes into the bag, as a
// Robot Workbench robot does (it deploys when put down).
func TestCraft_DeployOnCraft(t *testing.T) {
	lvl, e := craftOutput(t, crafting.Recipe{Name: "Print", Output: "colonist", DeployOnCraft: true})
	assert.Equal(t, 0, bagCount(e, "colonist"))
	var printed *ecs.Entity
	for _, x := range lvl.Entities {
		if x.Blueprint == "colonist" {
			printed = x
		}
	}
	require.NotNil(t, printed, "the printed colonist is on the level")
	assert.True(t, components.IsColonyWorker(printed, testColony))
	pc := printed.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	assert.Equal(t, [2]int{3, 1}, [2]int{pc.GetX(), pc.GetY()}, "at the station")

	lvl, e = craftOutput(t, crafting.Recipe{Name: "Hauler", Output: "hauler_robot"})
	assert.Equal(t, 1, bagCount(e, "hauler_robot"), "no flag: into the crafter's bag")
	for _, x := range lvl.Entities {
		assert.NotEqual(t, "hauler_robot", x.Blueprint)
	}
}

// The data: the bio-printer's colonist deploys, the workbench robots go into
// the bag as before.
func TestCraft_DeployOnCraftData(t *testing.T) {
	require.NoError(t, crafting.Load("../../data/crafting_recipes.json"))
	t.Cleanup(func() { _ = crafting.Load("") }) // as init left it here: no data/ under the package
	// The units that walk off the printer: the colony's colonist and (phase
	// D) the greyish bio-printer's crew, (phase E) the autonz fabricator's.
	deploys := map[string]bool{"print_colonist": true, "print_greenish": true, "print_greyish": true,
		"print_pinkish": true, "print_whitish": true, "print_wire_placer": true, "print_enforcer": true,
		// (phase E2) the hive's eggs, laid on a grey egg in its place.
		"hive_egg_drone": true, "hive_egg_scout": true, "hive_egg_warrior": true, "hive_egg_hunter": true,
		"hive_egg_burrower": true, "hive_egg_spitter": true,
		// (phase E3) the autonz' other prints, at the fabricator and foundry.
		"print_relay_drone": true, "print_seeker": true, "print_harvester": true, "print_sentinel": true, "print_warden": true,
		// (phase E4) the greys' fliers, at the grey vat.
		"print_skimmer": true, "print_lancer": true}
	for _, r := range crafting.AllRecipes() {
		assert.Equal(t, deploys[r.Key], r.DeployOnCraft, r.Key)
	}
}
