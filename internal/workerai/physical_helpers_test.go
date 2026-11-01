package workerai

import (
	"sync"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/factory"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
	"github.com/stretchr/testify/require"
)

const testColony = "physical_test_colony"

var loadBlueprints sync.Once

// physicalLevel returns an open single-floor level with blueprints loaded.
func physicalLevel(t *testing.T, w, h int) *world.Level {
	t.Helper()
	loadBlueprints.Do(func() {
		require.NoError(t, factory.FactoryLoadDir("../../data/blueprints"))
	})
	lvl := newLevel(w, h, 1)
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			setWalkable(lvl, x, y, 0)
		}
	}
	return lvl
}

// physicalWorker places a colony worker with an empty bag at (x,y).
func physicalWorker(lvl *world.Level, x, y int, bag ...*ecs.Entity) (*ecs.Entity, *components.WorkerComponent, *rlcomponents.AIMemoryComponent) {
	e := workerEntity(lvl, x, y, 0)
	wc := &components.WorkerComponent{}
	e.AddComponent(wc)
	e.AddComponent(&components.SettlementComponent{Name: testColony})
	e.AddComponent(&rlcomponents.InventoryComponent{Bag: bag})
	mem := e.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	mem.State = "task"
	return e, wc, mem
}

// bagCount totals blueprint across the entity's bag.
func bagCount(e *ecs.Entity, blueprint string) int {
	n := 0
	for _, item := range e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag {
		if item.Blueprint == blueprint {
			n += carryUnits(item)
		}
	}
	return n
}

// fakeBuildable overrides the build registry for the duration of a test.
func fakeBuildable(t *testing.T, cost map[string]int) {
	t.Helper()
	prev := getBuildable
	getBuildable = func(string) construction.Buildable {
		return construction.Buildable{Name: "test", Cost: cost}
	}
	t.Cleanup(func() { getBuildable = prev })
}

func buildTask(x, y, required int) *task.Task {
	return &task.Task{X: x, Y: y, Action: "build", InProgress: true,
		Data: buildReq(x, y, required)}
}

// physicalChest places a colony container holding the given stacks at (x,y).
func physicalChest(lvl *world.Level, x, y int, items ...*ecs.Entity) *components.StorageComponent {
	sc := &components.StorageComponent{OwnedBy: testColony, Items: items}
	e := &ecs.Entity{Blueprint: "test_chest"}
	e.AddComponent(&rlcomponents.PositionComponent{X: x, Y: y})
	e.AddComponent(sc)
	lvl.AddEntity(e)
	return sc
}

// runUntilState ticks the worker's gather and task states until pred holds.
func runUntil(t *testing.T, lvl *world.Level, e *ecs.Entity, mem *rlcomponents.AIMemoryComponent, maxTicks int, pred func() bool) {
	t.Helper()
	for i := 0; i < maxTicks; i++ {
		if pred() {
			return
		}
		fspath.ResetFrameCounter()
		switch mem.State {
		case "task":
			HandleTaskState(lvl, e)
		case "gather_materials":
			HandleGatherMaterialsState(lvl, e)
		case "idle":
			HandleWorkerIdleState(lvl, e)
		case "dropoff":
			HandleDropOffState(lvl, e)
		}
	}
	require.True(t, pred(), "condition not reached within %d ticks", maxTicks)
}
