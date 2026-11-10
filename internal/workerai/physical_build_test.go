package workerai

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/task"
	"github.com/stretchr/testify/assert"
)

func buildReq(x, y, required int) task_requests.BuildRequest {
	return task_requests.BuildRequest{Type: "hull_wall", X: x, Y: y, Required: required}
}

func TestBuild_NoProgressUntilDeliveredCoversCost(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 5})
	lvl := physicalLevel(t, 6, 6)
	e, wc, mem := physicalWorker(lvl, 2, 2, materialStack("metal_ore", 3))
	wc.CurrentTask = buildTask(3, 2, 100)

	// First load delivers what the bag holds, without advancing progress.
	HandleTaskState(lvl, e)
	br := wc.CurrentTask.Data.(task_requests.BuildRequest)
	assert.Equal(t, 3, br.Delivered["metal_ore"])
	assert.Equal(t, 0, br.Progress)
	assert.Equal(t, 0, bagCount(e, "metal_ore"))

	// Nothing left to deliver and the cost isn't met: back out for more.
	HandleTaskState(lvl, e)
	assert.Equal(t, "gather_materials", mem.State)
	assert.Equal(t, 0, wc.CurrentTask.Data.(task_requests.BuildRequest).Progress)

	// Second trip tops the tally up, taking only what is still owed.
	mem.State = "task"
	e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).AddItem(materialStack("metal_ore", 4))
	HandleTaskState(lvl, e)
	br = wc.CurrentTask.Data.(task_requests.BuildRequest)
	assert.Equal(t, 5, br.Delivered["metal_ore"])
	assert.Equal(t, 2, bagCount(e, "metal_ore"), "surplus stays in the bag")
	assert.Equal(t, 0, br.Progress)

	// Paid: work starts.
	HandleTaskState(lvl, e)
	assert.Greater(t, wc.CurrentTask.Data.(task_requests.BuildRequest).Progress, 0)
	assert.Equal(t, 2, bagCount(e, "metal_ore"), "working consumes nothing further")
}

func TestBuild_WalksToSiteBeforeDelivering(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 2})
	lvl := physicalLevel(t, 12, 3)
	e, wc, _ := physicalWorker(lvl, 0, 1, materialStack("metal_ore", 2))
	wc.CurrentTask = buildTask(10, 1, 100)

	HandleTaskState(lvl, e)
	assert.Nil(t, wc.CurrentTask.Data.(task_requests.BuildRequest).Delivered)
	assert.Equal(t, 2, bagCount(e, "metal_ore"))
}

func TestBuild_CompletesWithoutTouchingBagAgain(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 2})
	lvl := physicalLevel(t, 6, 6)
	e, wc, mem := physicalWorker(lvl, 2, 2, materialStack("metal_ore", 3))
	tk := buildTask(3, 2, 1)
	wc.CurrentTask = tk

	HandleTaskState(lvl, e) // deliver
	HandleTaskState(lvl, e) // work, completes
	assert.True(t, tk.Completed)
	assert.Equal(t, "idle", mem.State)
	assert.Equal(t, 1, bagCount(e, "metal_ore"))
}

// A sized build whose footprint someone stands in waits for them to move
// instead of finishing with nothing placed: found by the observer soak,
// where colonists crossing a bed's second tile ate the bed's metal.
func TestBuild_SizedWaitsForAWorkerInTheFootprint(t *testing.T) {
	prev := getBuildable
	getBuildable = func(string) construction.Buildable {
		return construction.Buildable{Name: "Bed", IsEntity: true, Cost: map[string]int{"metal_ore": 2}}
	}
	t.Cleanup(func() { getBuildable = prev })
	lvl := physicalLevel(t, 8, 6)
	e, wc, mem := physicalWorker(lvl, 2, 3, materialStack("metal_ore", 2))
	// A bed at (3, 3) covers (3, 2) and (3, 3); a worker stands on (3, 2).
	stander := workerEntity(lvl, 3, 2, 0)
	stander.AddComponent(&components.WorkerComponent{})
	stander.AddComponent(&rlcomponents.SolidComponent{})
	tk := &task.Task{X: 3, Y: 3, Action: task_requests.BuildAction, InProgress: true,
		Data: task_requests.BuildRequest{Type: "bed", X: 3, Y: 3, Required: 1}}
	wc.CurrentTask = tk
	for i := 0; i < 6; i++ {
		HandleTaskState(lvl, e)
	}
	beds := func() int {
		n := 0
		for _, x := range lvl.Entities {
			if x.Blueprint == "bed" {
				n++
			}
		}
		return n
	}
	br := tk.Data.(task_requests.BuildRequest)
	if tk.Completed || beds() != 0 || br.Delivered["metal_ore"] != 2 || br.Progress != br.Required {
		t.Fatalf("blocked build: completed=%v beds=%d %+v", tk.Completed, beds(), br)
	}
	// The stander leaves; the builder, still on the task, places the bed.
	lvl.RemoveEntity(stander)
	HandleTaskState(lvl, e)
	if !tk.Completed || beds() != 1 {
		t.Fatalf("after the stander left: completed=%v beds=%d", tk.Completed, beds())
	}
	_ = mem
}

// A builder standing in its own build's footprint steps out of it, then
// places the structure.
func TestBuild_BuilderStepsOutOfItsFootprint(t *testing.T) {
	prev := getBuildable
	getBuildable = func(string) construction.Buildable {
		return construction.Buildable{Name: "Bed", IsEntity: true, Cost: map[string]int{"metal_ore": 2}}
	}
	t.Cleanup(func() { getBuildable = prev })
	lvl := physicalLevel(t, 8, 6)
	// A bed at (3, 3) covers (3, 2) and (3, 3); the builder stands on (3, 2).
	e, wc, _ := physicalWorker(lvl, 3, 2, materialStack("metal_ore", 2))
	e.AddComponent(&rlcomponents.SolidComponent{})
	tk := &task.Task{X: 3, Y: 3, Action: task_requests.BuildAction, InProgress: true,
		Data: task_requests.BuildRequest{Type: "bed", X: 3, Y: 3, Required: 1}}
	wc.CurrentTask = tk
	for i := 0; i < 10 && !tk.Completed; i++ {
		HandleTaskState(lvl, e)
	}
	pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if !tk.Completed || pc.GetX() == 3 && (pc.GetY() == 2 || pc.GetY() == 3) {
		t.Fatalf("completed=%v builder at %d,%d", tk.Completed, pc.GetX(), pc.GetY())
	}
	n := 0
	for _, x := range lvl.Entities {
		if x.Blueprint == "bed" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d beds", n)
	}
}

// Someone who never leaves the footprint (a rival's sentry) does not pin the
// builder forever: after maxFootprintWait turns the task goes back to the
// queue with its materials kept (secondary finding).
func TestBuild_SizedGivesUpOnAWorkerWhoNeverMoves(t *testing.T) {
	prev := getBuildable
	getBuildable = func(string) construction.Buildable {
		return construction.Buildable{Name: "Bed", IsEntity: true, Cost: map[string]int{"metal_ore": 2}}
	}
	t.Cleanup(func() { getBuildable = prev })
	lvl := physicalLevel(t, 8, 6)
	e, wc, _ := physicalWorker(lvl, 2, 3, materialStack("metal_ore", 2))
	stander := workerEntity(lvl, 3, 2, 0)
	stander.AddComponent(&components.WorkerComponent{})
	stander.AddComponent(&rlcomponents.SolidComponent{})
	tk := &task.Task{X: 3, Y: 3, Action: task_requests.BuildAction, InProgress: true,
		Data: task_requests.BuildRequest{Type: "bed", X: 3, Y: 3, Required: 1}}
	wc.CurrentTask = tk
	for i := 0; i < maxFootprintWait+10 && wc.CurrentTask == tk; i++ {
		HandleTaskState(lvl, e)
	}
	if wc.CurrentTask == tk || tk.InProgress || tk.Completed {
		t.Fatalf("the builder is still pinned: current=%v inprogress=%v completed=%v", wc.CurrentTask == tk, tk.InProgress, tk.Completed)
	}
	if br := tk.Data.(task_requests.BuildRequest); br.Delivered["metal_ore"] != 2 {
		t.Errorf("the materials were lost: %+v", br)
	}
}
