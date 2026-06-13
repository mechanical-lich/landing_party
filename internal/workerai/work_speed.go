package workerai

import (
	"github.com/mechanical-lich/landing_party/internal/components"
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
	wc.WorkCarry += workRate(entity, stat)
	steps := int(wc.WorkCarry)
	wc.WorkCarry -= float64(steps)
	return steps
}
