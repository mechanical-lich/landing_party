package workerai

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
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
