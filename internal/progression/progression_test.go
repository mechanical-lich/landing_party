package progression

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/stretchr/testify/assert"
)

// colonistEntity builds a minimal entity with StatProgression and Stats.
func colonistEntity() *ecs.Entity {
	e := &ecs.Entity{}
	e.AddComponent(&components.StatProgressionComponent{})
	e.AddComponent(&rlcomponents.StatsComponent{Str: 10, Int: 10, Dex: 10, Con: 10})
	e.AddComponent(&rlcomponents.DescriptionComponent{Name: "Test Colonist"})
	return e
}

func TestAwardXP_NoProgressionComponent_IsNoop(t *testing.T) {
	e := &ecs.Entity{}
	e.AddComponent(&rlcomponents.StatsComponent{Str: 10})
	AwardXP(e, "Str", 9999)
	sc := e.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
	assert.Equal(t, 10, sc.Str, "stat should be unchanged without StatProgressionComponent")
}

func TestAwardXP_AccumulatesXP(t *testing.T) {
	e := colonistEntity()
	AwardXP(e, "Str", 50)
	prog := e.GetComponent(components.StatProgression).(*components.StatProgressionComponent)
	assert.Equal(t, 50, prog.Str.XP)
	assert.Equal(t, 0, prog.Str.Level)
}

func TestAwardXP_LevelUp_IncrementsStat(t *testing.T) {
	e := colonistEntity()
	// threshold for level 0→1 is (0+1)^2 * 100 = 100
	AwardXP(e, "Str", 100)
	sc := e.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
	prog := e.GetComponent(components.StatProgression).(*components.StatProgressionComponent)
	assert.Equal(t, 1, prog.Str.Level)
	assert.Equal(t, 11, sc.Str, "Str should increment on level-up")
}

func TestAwardXP_LevelUp_ResetsXP(t *testing.T) {
	e := colonistEntity()
	// Award exactly threshold + 10 overflow
	AwardXP(e, "Str", 110)
	prog := e.GetComponent(components.StatProgression).(*components.StatProgressionComponent)
	assert.Equal(t, 1, prog.Str.Level)
	assert.Equal(t, 10, prog.Str.XP, "overflow XP should carry over after level-up")
}

func TestAwardXP_MultipleStats_Independent(t *testing.T) {
	e := colonistEntity()
	AwardXP(e, "Int", 100) // level up Int
	sc := e.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
	prog := e.GetComponent(components.StatProgression).(*components.StatProgressionComponent)
	assert.Equal(t, 1, prog.Int.Level)
	assert.Equal(t, 11, sc.Int)
	// Other stats untouched
	assert.Equal(t, 0, prog.Str.Level)
	assert.Equal(t, 10, sc.Str)
	assert.Equal(t, 0, prog.Dex.Level)
	assert.Equal(t, 10, sc.Dex)
}

func TestAwardXP_AllStats_LevelUp(t *testing.T) {
	e := colonistEntity()
	sc := e.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)

	for _, tc := range []struct {
		stat    string
		getBase func() int
	}{
		{"Str", func() int { return sc.Str }},
		{"Dex", func() int { return sc.Dex }},
		{"Int", func() int { return sc.Int }},
		{"Con", func() int { return sc.Con }},
	} {
		before := tc.getBase()
		AwardXP(e, tc.stat, xpThreshold(0))
		assert.Equal(t, before+1, tc.getBase(), "%s should increase on level-up", tc.stat)
	}
}

func TestAwardXP_Cap_StopsAtMaxLevel(t *testing.T) {
	e := colonistEntity()
	prog := e.GetComponent(components.StatProgression).(*components.StatProgressionComponent)
	sc := e.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)

	// Level up to cap by awarding enough XP for each level
	for prog.Str.Level < maxLevel {
		AwardXP(e, "Str", xpThreshold(prog.Str.Level))
	}
	assert.Equal(t, maxLevel, prog.Str.Level)
	assert.Equal(t, 20, sc.Str)

	statBefore := sc.Str
	AwardXP(e, "Str", 99999)
	assert.Equal(t, maxLevel, prog.Str.Level, "level should not exceed cap")
	assert.Equal(t, statBefore, sc.Str, "stat should not increase past cap")
}

func TestXPThreshold_Formula(t *testing.T) {
	assert.Equal(t, 100, xpThreshold(0))
	assert.Equal(t, 400, xpThreshold(1))
	assert.Equal(t, 900, xpThreshold(2))
	assert.Equal(t, 10000, xpThreshold(9))
}
