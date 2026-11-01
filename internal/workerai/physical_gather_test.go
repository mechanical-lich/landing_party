package workerai

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/stretchr/testify/assert"
)

func TestPlanner_TakesTwoListedMaterialsInOneStop(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 2, "crystal": 1})
	lvl := physicalLevel(t, 6, 6)
	chest := physicalChest(lvl, 3, 2, materialStack("metal_ore", 5), materialStack("crystal", 5))
	e, wc, mem := physicalWorker(lvl, 2, 2)
	mem.State = "gather_materials"
	wc.CurrentTask = buildTask(0, 0, 100)

	HandleGatherMaterialsState(lvl, e) // one load, adjacent to the chest
	assert.Equal(t, 2, bagCount(e, "metal_ore"))
	assert.Equal(t, 1, bagCount(e, "crystal"))
	assert.Equal(t, 3, chest.CountResource("metal_ore"), "only what's listed leaves the chest")
	assert.Empty(t, wc.GatherList)

	HandleGatherMaterialsState(lvl, e)
	assert.Equal(t, "task", mem.State)
	assert.Nil(t, wc.GatherList)
	assert.Nil(t, wc.GatherTarget)
}

func TestPlanner_VisitsNearestContainerToWorkerNotSite(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 2})
	lvl := physicalLevel(t, 20, 3)
	nearWorker := physicalChest(lvl, 2, 1, materialStack("metal_ore", 5))
	nearSite := physicalChest(lvl, 17, 1, materialStack("metal_ore", 5))
	e, wc, mem := physicalWorker(lvl, 1, 1)
	mem.State = "gather_materials"
	wc.CurrentTask = buildTask(18, 1, 100)

	HandleGatherMaterialsState(lvl, e)
	assert.Equal(t, 3, nearWorker.CountResource("metal_ore"))
	assert.Equal(t, 5, nearSite.CountResource("metal_ore"))
}

func TestPlanner_ContinuesToSecondContainerForTheRest(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 4, "crystal": 1})
	lvl := physicalLevel(t, 12, 3)
	physicalChest(lvl, 2, 1, materialStack("metal_ore", 4))
	physicalChest(lvl, 8, 1, materialStack("crystal", 1))
	e, wc, mem := physicalWorker(lvl, 1, 1)
	mem.State = "gather_materials"
	wc.CurrentTask = buildTask(1, 2, 100)

	runUntil(t, lvl, e, mem, 40, func() bool { return mem.State == "task" })
	assert.Equal(t, 4, bagCount(e, "metal_ore"))
	assert.Equal(t, 1, bagCount(e, "crystal"))
}

func TestPlanner_CarryLimitSplitsCostAcrossTrips(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 25})
	lvl := physicalLevel(t, 10, 3)
	chest := physicalChest(lvl, 1, 1, materialStack("metal_ore", 30))
	e, wc, mem := physicalWorker(lvl, 2, 1)
	tk := buildTask(7, 1, 1000)
	wc.CurrentTask = tk
	delivered := func() int { return tk.Data.(task_requests.BuildRequest).Delivered["metal_ore"] }

	// First trip: a Str-10 worker carries 20 of the 25.
	runUntil(t, lvl, e, mem, 40, func() bool { return delivered() > 0 })
	assert.Equal(t, 20, delivered())
	assert.Equal(t, 10, chest.CountResource("metal_ore"))

	// Second trip fetches only the remaining 5, and the tally accumulates.
	runUntil(t, lvl, e, mem, 40, func() bool { return delivered() > 20 })
	assert.Equal(t, 25, delivered())
	assert.Equal(t, 5, chest.CountResource("metal_ore"))
	assert.Equal(t, 0, bagCount(e, "metal_ore"))
}

func TestPlanner_NoStockStopsTaskWithoutLosingIt(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 1})
	lvl := physicalLevel(t, 6, 6)
	e, wc, mem := physicalWorker(lvl, 2, 2)
	mem.State = "gather_materials"
	tk := buildTask(0, 0, 100)
	wc.CurrentTask = tk

	HandleGatherMaterialsState(lvl, e)
	assert.Nil(t, wc.CurrentTask)
	assert.Equal(t, "idle", mem.State)
	assert.False(t, tk.Completed)
	assert.True(t, tk.ManuallyStopped)
}

func TestPlanner_StockVanishingMidTripDeliversWhatWasFetched(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 2, "crystal": 1})
	lvl := physicalLevel(t, 6, 6)
	physicalChest(lvl, 3, 2, materialStack("metal_ore", 2))
	e, wc, mem := physicalWorker(lvl, 2, 2)
	mem.State = "gather_materials"
	tk := buildTask(0, 2, 100)
	wc.CurrentTask = tk

	HandleGatherMaterialsState(lvl, e) // loads ore; crystal exists nowhere
	HandleGatherMaterialsState(lvl, e)
	assert.Equal(t, "task", mem.State)
	assert.Equal(t, tk, wc.CurrentTask)
	assert.Equal(t, 2, bagCount(e, "metal_ore"))
}

func TestPlanner_BagFullOfUnrelatedItemsReleasesTask(t *testing.T) {
	fakeBuildable(t, map[string]int{"crystal": 1})
	lvl := physicalLevel(t, 6, 6)
	physicalChest(lvl, 3, 2, materialStack("crystal", 2))
	e, wc, mem := physicalWorker(lvl, 2, 2, materialStack("metal_ore", 20))
	mem.State = "gather_materials"
	tk := buildTask(0, 0, 100)
	wc.CurrentTask = tk

	HandleGatherMaterialsState(lvl, e)
	assert.Nil(t, wc.CurrentTask)
	assert.False(t, tk.InProgress)
	assert.Equal(t, "haul", mem.State)
}

func TestPlanner_NonStackablesTakenPerItem(t *testing.T) {
	fakeBuildable(t, map[string]int{"gizmo": 2})
	lvl := physicalLevel(t, 6, 6)
	chest := physicalChest(lvl, 3, 2, &ecs.Entity{Blueprint: "gizmo"}, &ecs.Entity{Blueprint: "gizmo"}, &ecs.Entity{Blueprint: "gizmo"})
	e, wc, mem := physicalWorker(lvl, 2, 2)
	mem.State = "gather_materials"
	wc.CurrentTask = buildTask(0, 0, 100)

	HandleGatherMaterialsState(lvl, e)
	assert.Equal(t, 2, len(e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag))
	assert.Equal(t, 1, chest.CountResource("gizmo"))
}

func TestPlanner_EmptiedTargetIsReplanned(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 1})
	lvl := physicalLevel(t, 20, 3)
	first := physicalChest(lvl, 8, 1, materialStack("metal_ore", 1))
	physicalChest(lvl, 16, 1, materialStack("metal_ore", 1))
	e, wc, mem := physicalWorker(lvl, 1, 1)
	mem.State = "gather_materials"
	wc.CurrentTask = buildTask(1, 2, 100)

	HandleGatherMaterialsState(lvl, e)
	assert.NotNil(t, wc.GatherTarget)
	first.Items = nil // someone else got there first
	runUntil(t, lvl, e, mem, 40, func() bool { return mem.State == "task" })
	assert.Equal(t, 1, bagCount(e, "metal_ore"))
}
