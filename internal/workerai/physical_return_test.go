package workerai

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
	"github.com/stretchr/testify/assert"
)

func TestReturn_IdleWorkerStoresMaterialsAndKeepsGear(t *testing.T) {
	colony(t)
	lvl := physicalLevel(t, 12, 3)
	chest := physicalChest(lvl, 8, 1, materialStack("metal_ore", 1))
	gear := &ecs.Entity{Blueprint: "sleeping_bag"}
	e, wc, mem := physicalWorker(lvl, 1, 1, materialStack("metal_ore", 2), gear, materialStack("crystal", 1))
	mem.State = "idle"

	runUntil(t, lvl, e, mem, 60, func() bool {
		return chest.CountResource("metal_ore") == 3 && chest.CountResource("crystal") == 1 && mem.State == "idle"
	})
	bag := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag
	assert.Equal(t, []*ecs.Entity{gear}, bag)
	assert.Nil(t, wc.DropOffItem)
	assert.Nil(t, wc.DropOffDestEntity)
	assert.Equal(t, -1, mem.TargetX, "find-food and find-bedroll read -1 as no target")
}

func TestReturn_TaskComesFirst(t *testing.T) {
	s := colony(t)
	lvl := physicalLevel(t, 12, 3)
	physicalChest(lvl, 8, 1)
	e, wc, mem := physicalWorker(lvl, 1, 1, materialStack("metal_ore", 2))
	mem.State = "idle"
	dig := &task.Task{X: 10, Y: 1, Action: task_requests.DigAction}
	s.Tasks.AddTask(dig)

	HandleWorkerIdleState(lvl, e)
	assert.Same(t, dig, wc.CurrentTask)
	assert.Equal(t, "task", mem.State)
}

func TestReturn_NoRoomLeavesWorkerIdleAndCarrying(t *testing.T) {
	colony(t)
	lvl := physicalLevel(t, 6, 3)
	chest := physicalChest(lvl, 4, 1, materialStack("crystal", 1))
	chest.Capacity = 1
	e, wc, mem := physicalWorker(lvl, 1, 1, materialStack("metal_ore", 2))
	mem.State = "idle"

	HandleWorkerIdleState(lvl, e)
	assert.Equal(t, "idle", mem.State)
	assert.Equal(t, 2, bagCount(e, "metal_ore"))
	assert.Equal(t, returnRetryTicks, wc.ReturnRetryTicks)
}

func TestReturn_UnreachableStorageDoesNotPinWorker(t *testing.T) {
	s := colony(t)
	lvl := physicalLevel(t, 10, 3)
	for y := 0; y < 3; y++ {
		setWall(lvl, 5, y, 0)
	}
	physicalChest(lvl, 8, 1)
	e, wc, mem := physicalWorker(lvl, 1, 1, materialStack("metal_ore", 2))
	mem.State = "idle"

	HandleWorkerIdleState(lvl, e)
	assert.Equal(t, "dropoff", mem.State)
	runUntil(t, lvl, e, mem, 60, func() bool { return wc.ReturnRetryTicks > 0 })
	assert.Equal(t, "idle", mem.State)
	assert.Nil(t, wc.DropOffItem)
	assert.Equal(t, 2, bagCount(e, "metal_ore"))

	// Now free to take work, and not sent straight back to the wall.
	HandleWorkerIdleState(lvl, e)
	assert.Equal(t, "idle", mem.State)
	dig := &task.Task{X: 2, Y: 1, Action: task_requests.DigAction}
	s.Tasks.AddTask(dig)
	HandleWorkerIdleState(lvl, e)
	assert.Same(t, dig, wc.CurrentTask)
}

func TestIdleLog_PrintsOncePerStretchWithoutAssignableTask(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	count := func() int { return strings.Count(buf.String(), "none assignable") }

	fakeBuildable(t, map[string]int{"metal_ore": 3})
	s := colony(t)
	lvl := physicalLevel(t, 6, 3)
	e, wc, mem := physicalWorker(lvl, 1, 1)
	mem.State = "idle"
	queuedBuild(s, 3, 1) // unaffordable

	for i := 0; i < 5; i++ {
		HandleWorkerIdleState(lvl, e)
	}
	assert.Equal(t, 1, count())

	// Getting work ends the stretch; the next one logs again.
	dig := &task.Task{X: 2, Y: 1, Action: task_requests.DigAction}
	s.Tasks.AddTask(dig)
	HandleWorkerIdleState(lvl, e)
	assert.Same(t, dig, wc.CurrentTask)
	dig.Complete()
	wc.CurrentTask = nil
	mem.State = "idle"
	HandleWorkerIdleState(lvl, e)
	HandleWorkerIdleState(lvl, e)
	assert.Equal(t, 2, count())
}

func foodItem(name string) *ecs.Entity {
	e := &ecs.Entity{Blueprint: name}
	e.AddComponent(&rlcomponents.FoodComponent{Amount: 10})
	return e
}

// A worker keeps one food item; the rest goes to storage with its next
// drop-off, where the others can eat it.
func TestReturn_SurplusFoodIsStoredOneKept(t *testing.T) {
	colony(t)
	lvl := physicalLevel(t, 12, 3)
	chest := physicalChest(lvl, 8, 1)
	gear := &ecs.Entity{Blueprint: "sleeping_bag"}
	e, _, mem := physicalWorker(lvl, 1, 1, foodItem("ration_pack"), gear, materialStack("metal_ore", 1), foodItem("protein_bar"), foodItem("protein_bar"))
	mem.State = "idle"
	runUntil(t, lvl, e, mem, 80, func() bool {
		return chest.CountResource("protein_bar") == 2 && chest.CountResource("metal_ore") == 1 && mem.State == "idle"
	})
	assert.Equal(t, 1, bagCount(e, "ration_pack"), "its own ration stays with it")
	assert.Equal(t, 0, bagCount(e, "protein_bar"))
	assert.Equal(t, 1, bagCount(e, "sleeping_bag"))
}

// A drop-off that never delivers (here: pinned out of reach of its
// container) is given up after maxDropOffTicks, the item kept in the bag.
func TestDropOff_GivesUpAfterTheCap(t *testing.T) {
	colony(t)
	lvl := physicalLevel(t, 12, 3)
	e, wc, mem := physicalWorker(lvl, 1, 1, materialStack("metal_ore", 1))
	mem.State = "dropoff"
	mem.TargetX, mem.TargetY = 10, 1
	wc.DropOffTicks = maxDropOffTicks - 1
	HandleDropOffState(lvl, e)
	assert.Equal(t, "idle", mem.State)
	assert.Equal(t, 0, wc.DropOffTicks)
	assert.Equal(t, 1, bagCount(e, "metal_ore"))
	assert.Equal(t, returnRetryTicks, wc.ReturnRetryTicks)
}
