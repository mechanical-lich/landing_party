package workerai

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/stretchr/testify/assert"
)

func statEntity(str, intl, dex, con int) *ecs.Entity {
	e := &ecs.Entity{}
	e.AddComponent(&rlcomponents.StatsComponent{Str: str, Int: intl, Dex: dex, Con: con})
	return e
}

func TestWorkRate_BaselineAndScaling(t *testing.T) {
	// Stat 10 is the reference: exactly the pre-scaling 1.0/tick.
	assert.Equal(t, 1.0, workRate(statEntity(10, 10, 10, 10), "Str"))
	// Above baseline scales linearly.
	assert.Equal(t, 1.5, workRate(statEntity(15, 0, 0, 0), "Str"))
	assert.Equal(t, 2.0, workRate(statEntity(0, 20, 0, 0), "Int"))
}

func TestWorkRate_ClampsToPureBonus(t *testing.T) {
	// Below baseline never regresses pacing (floored at 1.0).
	assert.Equal(t, minWorkRate, workRate(statEntity(5, 0, 0, 0), "Str"))
	// Absurdly high stats are capped.
	assert.Equal(t, maxWorkRate, workRate(statEntity(999, 0, 0, 0), "Str"))
}

func TestWorkRate_NoStatsComponentIsBaseline(t *testing.T) {
	assert.Equal(t, minWorkRate, workRate(&ecs.Entity{}, "Str"))
}

func TestWorkStep_FractionalCarryAverages(t *testing.T) {
	wc := &components.WorkerComponent{}
	e := statEntity(13, 0, 0, 0) // rate 1.3/tick

	total := 0
	const ticks = 10
	for i := 0; i < ticks; i++ {
		total += workStep(wc, e, "Str")
	}
	// 1.3 * 10 = 13 progress over 10 ticks — no truncation loss.
	assert.Equal(t, 13, total)
	// Every tick advances at least the baseline.
	assert.GreaterOrEqual(t, workStep(wc, e, "Str"), 1)
}
