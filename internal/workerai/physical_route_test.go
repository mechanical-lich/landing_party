package workerai

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func looseItem(t *testing.T, lvl *world.Level, blueprint string, qty, x, y int) *ecs.Entity {
	t.Helper()
	e, err := factory.Create(blueprint, x, y, 0)
	require.NoError(t, err)
	require.True(t, e.HasComponent(rlcomponents.Item), "%s must be an Item", blueprint)
	e.GetComponent(components.Material).(*components.MaterialComponent).Quantity = qty
	lvl.AddEntity(e)
	return e
}

// routeSetup starts a worker gathering 3 ore from a chest far to the east.
func routeSetup(t *testing.T) (*world.Level, *ecs.Entity, *components.WorkerComponent) {
	t.Helper()
	fakeBuildable(t, map[string]int{"metal_ore": 3})
	lvl := physicalLevel(t, 20, 5)
	physicalChest(lvl, 18, 2, materialStack("metal_ore", 10))
	e, wc, mem := physicalWorker(lvl, 2, 2)
	mem.State = "gather_materials"
	wc.CurrentTask = buildTask(0, 0, 100)
	return lvl, e, wc
}

func onLevel(lvl *world.Level, e *ecs.Entity) bool {
	for _, c := range lvl.Entities {
		if c == e {
			return true
		}
	}
	for _, c := range lvl.StaticEntities {
		if c == e {
			return true
		}
	}
	return false
}

func TestRoute_ListedItemWithinATileIsGrabbed(t *testing.T) {
	lvl, e, wc := routeSetup(t)
	item := looseItem(t, lvl, "metal_ore", 2, 3, 3) // diagonal: still one tile

	HandleGatherMaterialsState(lvl, e)
	assert.Equal(t, 2, bagCount(e, "metal_ore"))
	assert.False(t, onLevel(lvl, item))
	assert.Equal(t, 1, wc.GatherList["metal_ore"], "the list shrinks by what was grabbed")
}

func TestRoute_UnlistedItemIsLeft(t *testing.T) {
	lvl, e, _ := routeSetup(t)
	item := looseItem(t, lvl, "crystal", 2, 3, 2)

	HandleGatherMaterialsState(lvl, e)
	assert.Equal(t, 0, bagCount(e, "crystal"))
	assert.True(t, onLevel(lvl, item))
}

func TestRoute_ItemTwoTilesAwayIsLeft(t *testing.T) {
	lvl, e, _ := routeSetup(t)
	item := looseItem(t, lvl, "metal_ore", 2, 2, 4)

	HandleGatherMaterialsState(lvl, e)
	assert.Equal(t, 0, bagCount(e, "metal_ore"))
	assert.True(t, onLevel(lvl, item))
}

func TestRoute_TakesOnlyWhatTheListNeeds(t *testing.T) {
	lvl, e, wc := routeSetup(t)
	item := looseItem(t, lvl, "metal_ore", 5, 3, 2)

	HandleGatherMaterialsState(lvl, e)
	assert.Equal(t, 3, bagCount(e, "metal_ore"))
	assert.Equal(t, 2, carryUnits(item), "the rest stays on the ground")
	assert.True(t, onLevel(lvl, item))
	assert.Empty(t, wc.GatherList)
}

func TestRoute_RespectsFreeCapacity(t *testing.T) {
	fakeBuildable(t, map[string]int{"metal_ore": 10})
	lvl := physicalLevel(t, 20, 5)
	physicalChest(lvl, 18, 2, materialStack("metal_ore", 10))
	e, wc, mem := physicalWorker(lvl, 2, 2, materialStack("crystal", 18))
	mem.State = "gather_materials"
	wc.CurrentTask = buildTask(0, 0, 100)
	item := looseItem(t, lvl, "metal_ore", 5, 3, 2)

	HandleGatherMaterialsState(lvl, e)
	assert.Equal(t, 2, bagCount(e, "metal_ore"))
	assert.Equal(t, 3, carryUnits(item))
}

func TestRoute_ContainersAreNotOpenedOpportunistically(t *testing.T) {
	lvl, e, _ := routeSetup(t)
	// Adjacent, but owned by nobody the worker draws from, so never a hop.
	other := physicalChest(lvl, 3, 2, materialStack("metal_ore", 5))
	other.OwnedBy = "someone_else"

	HandleGatherMaterialsState(lvl, e)
	assert.Equal(t, 5, other.CountResource("metal_ore"))
	assert.Equal(t, 0, bagCount(e, "metal_ore"))
}
