package workerai

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// baseWorkStat is the stat value at which a worker performs a task at the
// reference rate of 1.0 progress/tick — the speed every handler used before
// stat scaling existed. Fresh colonists start with all stats at 10, so their
// pacing is unchanged; progression and high-stat chassis (e.g. the Str-12
// excavator robot) only ever speed work up, never below this baseline.
const baseWorkStat = 10.0

const (
	// minWorkRate keeps stat scaling a pure bonus: the original balance is the
	// slowest a task can ever go, so low/unset stats never regress pacing.
	minWorkRate = 1.0
	// maxWorkRate guards against absurd progress jumps if stats ever climb far
	// past the colonist cap.
	maxWorkRate = 3.0
)

// baseCarryCapacity is how many stack units a worker with baseline Str carries
// on a gather trip. Only the gather planner enforces it; haul, pickup orders and
// crafted output ignore it so no existing flow can get stuck on a full bag.
const baseCarryCapacity = 20

// CarryCapacity returns the stack units the entity may carry on a gather trip:
// baseCarryCapacity scaled by Str, floored at the baseline so low or unset
// stats are never a penalty.
func CarryCapacity(entity *ecs.Entity) int {
	if !entity.HasComponent(rlcomponents.Stats) {
		return baseCarryCapacity
	}
	sc := entity.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
	capacity := int(float64(baseCarryCapacity) * float64(sc.Str) / baseWorkStat)
	if capacity < baseCarryCapacity {
		return baseCarryCapacity
	}
	return capacity
}

// carryUnits is what one bag item counts for: a stack's quantity, or 1 for a
// non-stackable item.
func carryUnits(item *ecs.Entity) int {
	if item.HasComponent(components.Material) {
		return item.GetComponent(components.Material).(*components.MaterialComponent).Quantity
	}
	return 1
}

// CarryUsed sums the stack units in the entity's bag.
func CarryUsed(entity *ecs.Entity) int {
	if !entity.HasComponent(rlcomponents.Inventory) {
		return 0
	}
	used := 0
	for _, item := range entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag {
		used += carryUnits(item)
	}
	return used
}

// carryFree returns the room left in the bag, 0 when it is already overfull.
func carryFree(entity *ecs.Entity) int {
	free := CarryCapacity(entity) - CarryUsed(entity)
	if free < 0 {
		return 0
	}
	return free
}

// workRate returns the per-tick progress multiplier for the named governing
// stat ("Str", "Int", "Dex", "Con"). Entities without a StatsComponent (or an
// unrecognised stat) fall back to the baseline rate.
func workRate(entity *ecs.Entity, stat string) float64 {
	if !entity.HasComponent(rlcomponents.Stats) {
		return minWorkRate
	}
	sc := entity.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
	v := 0
	switch stat {
	case "Str":
		v = sc.Str
	case "Int":
		v = sc.Int
	case "Dex":
		v = sc.Dex
	case "Con":
		v = sc.Con
	}
	rate := float64(v) / baseWorkStat
	if rate < minWorkRate {
		return minWorkRate
	}
	if rate > maxWorkRate {
		return maxWorkRate
	}
	return rate
}

// workStep advances the worker's fractional work carry by the governing stat's
// rate and returns the whole number of progress steps to apply this tick
// (always >= 1, since workRate is floored at 1.0). Sub-unit fractions bank on
// the WorkerComponent so a rate of e.g. 1.3 averages out across ticks instead
// of truncating to 1 every tick.
func workStep(wc *components.WorkerComponent, entity *ecs.Entity, stat string) int {
	rate := workRate(entity, stat)
	if wc.WorkMult > 0 {
		rate *= wc.WorkMult
	}
	wc.WorkCarry += rate
	steps := int(wc.WorkCarry)
	wc.WorkCarry -= float64(steps)
	return steps
}

// mineStep is workStep for mining: the worker's MineMult on top (the greys'
// salvage rigs speed their mine work, live only).
func mineStep(wc *components.WorkerComponent, entity *ecs.Entity) int {
	return multStep(wc, entity, wc.MineMult)
}

// digStep is workStep for digging: the worker's DigMult on top (the greys'
// tunnelling, from their type).
func digStep(wc *components.WorkerComponent, entity *ecs.Entity) int {
	return multStep(wc, entity, wc.DigMult)
}

// multStep is a Str workStep with an extra multiplier m (0 or 1: none).
func multStep(wc *components.WorkerComponent, entity *ecs.Entity, m float64) int {
	if m <= 0 || m == 1 {
		return workStep(wc, entity, "Str")
	}
	rate := workRate(entity, "Str") * m
	if wc.WorkMult > 0 {
		rate *= wc.WorkMult
	}
	wc.WorkCarry += rate
	steps := int(wc.WorkCarry)
	wc.WorkCarry -= float64(steps)
	return steps
}

// boreOn is a boring worker's dig carried on (the greys' tunnelling bore: a
// type's bore): after (tx, ty, tz) is dug from (wx, wy), up to Bore-1 more
// tiles straight on in the same direction are cleared, while each is solid
// rock: never ore (the mine work takes that), a structure or open ground.
func boreOn(level *world.Level, wc *components.WorkerComponent, wx, wy, tx, ty, tz int) {
	if wc.Bore <= 1 {
		return
	}
	dx, dy := tx-wx, ty-wy
	if dx < -1 || dx > 1 || dy < -1 || dy > 1 || dx == 0 && dy == 0 || dx != 0 && dy != 0 {
		return // only straight on from an orthogonal neighbour
	}
	x, y := tx, ty
	for i := 1; i < wc.Bore; i++ {
		x, y = x+dx, y+dy
		t := level.GetTilePtr(x, y, tz)
		if t == nil || t.Middle.IsEmpty() || world.DepositDrop(world.TileDefinitions[t.Middle.Type].Name) != "" {
			return
		}
		if level.GetEntityAt(x, y, tz) != nil {
			return
		}
		clearTileRadiation(t)
		level.ClearMiddle(x, y, tz)
		level.InvalidateSunColumn(x, y)
	}
}
