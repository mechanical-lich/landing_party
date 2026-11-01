package workerai

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/stretchr/testify/assert"
)

func carrier(str int, bag ...*ecs.Entity) *ecs.Entity {
	e := statEntity(str, 0, 0, 0)
	e.AddComponent(&rlcomponents.InventoryComponent{Bag: bag})
	return e
}

func materialStack(blueprint string, qty int) *ecs.Entity {
	e := &ecs.Entity{Blueprint: blueprint}
	e.AddComponent(&components.MaterialComponent{Quantity: qty, MaxStack: 100})
	return e
}

func TestCarryCapacity_BaselineAtStr10(t *testing.T) {
	assert.Equal(t, baseCarryCapacity, CarryCapacity(statEntity(10, 0, 0, 0)))
}

func TestCarryCapacity_ScalesWithStr(t *testing.T) {
	assert.Equal(t, 24, CarryCapacity(statEntity(12, 0, 0, 0)))
	assert.Equal(t, 30, CarryCapacity(statEntity(15, 0, 0, 0)))
}

func TestCarryCapacity_FloorsAtBaseline(t *testing.T) {
	assert.Equal(t, baseCarryCapacity, CarryCapacity(statEntity(5, 0, 0, 0)))
	assert.Equal(t, baseCarryCapacity, CarryCapacity(statEntity(0, 0, 0, 0)))
	assert.Equal(t, baseCarryCapacity, CarryCapacity(&ecs.Entity{}))
}

func TestCarryUsed_SumsStacksAndCountsNonStackablesAsOne(t *testing.T) {
	e := carrier(10, materialStack("metal_ore", 7), &ecs.Entity{Blueprint: "sleeping_bag"})
	assert.Equal(t, 8, CarryUsed(e))
}

func TestCarryUsed_NoInventoryIsZero(t *testing.T) {
	assert.Equal(t, 0, CarryUsed(&ecs.Entity{}))
}

func TestCarryFree_IsNeverNegative(t *testing.T) {
	assert.Equal(t, 15, carryFree(carrier(10, materialStack("metal_ore", 5))))
	// Haul and crafted output can overfill a bag; the planner just sees no room.
	assert.Equal(t, 0, carryFree(carrier(10, materialStack("metal_ore", 50))))
}
