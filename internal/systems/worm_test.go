package systems

import (
	"fmt"
	"os"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mechanical-basic/pkg/basic"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// impacts counts the melee hits attack_at has landed on the level so far (each
// one emits an impact sound), so a test can tell whether a bite happened
// without depending on combat's hit rolls.
func impacts(lvl *world.Level) int {
	n := 0
	for _, s := range lvl.Sounds {
		if s.Tag == world.SoundTagImpact {
			n++
		}
	}
	return n
}

// wormLevel builds a w×h level whose ground surface in column (x, y) is at
// surf(x, y): rock below, surface at that z, open air above.
func wormLevel(w, h, depth int, surf func(x, y int) int) *world.Level {
	lvl := world.NewLevel(w, h, depth)
	lvl.AllocTerrain()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s := surf(x, y)
			lvl.SetSurfaceZ(x, y, s)
			for z := 0; z < depth; z++ {
				switch {
				case z < s:
					lvl.SetTerrainKind(x, y, z, world.TKUnderground)
				case z == s:
					lvl.SetTerrainKind(x, y, z, world.TKSurface)
				default:
					lvl.SetTerrainKind(x, y, z, world.TKAtmosphere)
				}
			}
		}
	}
	return lvl
}

// surfacedWorm puts a worm on the surface at (x, y, z), already in its surfaced
// state, and loads the real worm.basic for it.
func surfacedWorm(t *testing.T, lvl *world.Level, x, y, z int) (*ecs.Entity, *basic.MechBasic) {
	t.Helper()
	worm := mob(lvl, "monsters", x, y, z, fspath.BurrowingSkill)
	script, err := os.ReadFile("../../data/scripts/ai/worm.basic")
	require.NoError(t, err)
	ai := &components.ScriptedAIComponent{Vars: map[string]any{"surfaced": 1.0, "surface_ttl": 4.0}}
	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, worm, lvl, ai)
	require.NoError(t, interp.Load(string(script)))
	return worm, interp
}

func turn(t *testing.T, interp *basic.MechBasic) {
	t.Helper()
	_, err := interp.Call("on_turn")
	require.NoError(t, err)
}

func posOf(e *ecs.Entity) *rlcomponents.PositionComponent {
	return e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
}

// TestAttackAtOnlyReachesAdjacent: attack_at is melee — it lands on the
// caller's neighbours (one z up or down allowed) and on nothing farther, no
// matter which tile the script names.
func TestAttackAtOnlyReachesAdjacent(t *testing.T) {
	for _, tc := range []struct {
		dx, dy, dz int
		hit        bool
	}{
		{1, 0, 0, true}, {-1, -1, 0, true}, {0, 1, 1, true}, {1, 0, -1, true},
		{2, 0, 0, false}, {0, -3, 0, false}, {2, 2, 0, false}, {0, 1, 2, false}, {6, 0, 0, false},
	} {
		t.Run(fmt.Sprintf("%+d%+d%+d", tc.dx, tc.dy, tc.dz), func(t *testing.T) {
			lvl := world.NewLevel(16, 16, 6)
			self := mob(lvl, "monsters", 6, 6, 2)
			_ = mob(lvl, "colony", 6+tc.dx, 6+tc.dy, 2+tc.dz)
			interp := basic.NewMechanicalBasic()
			registerScriptedAIFuncs(interp, self, lvl, &components.ScriptedAIComponent{Vars: map[string]any{}})
			require.NoError(t, interp.Load(fmt.Sprintf(`
function probe()
    return attack_at(%d, %d, %d)
endfunction
`, 6+tc.dx, 6+tc.dy, 2+tc.dz)))
			want, wantImpacts := 0.0, 0
			if tc.hit {
				want, wantImpacts = 1.0, 1
			}
			assert.Equal(t, want, callNum(t, interp, "probe"))
			assert.Equal(t, wantImpacts, impacts(lvl))
		})
	}
}

// TestSurfacedWormChasesThenBites is the regression test for the surfaced worm
// biting colonists up to three tiles away and never moving: it must close the
// distance on the surface first and only bite once it is beside its prey.
func TestSurfacedWormChasesThenBites(t *testing.T) {
	const surfZ = 4
	lvl := wormLevel(16, 8, 8, func(x, y int) int { return surfZ })
	worm, interp := surfacedWorm(t, lvl, 3, 4, surfZ)
	_ = mob(lvl, "colony", 6, 4, surfZ) // three tiles east, inside find_nearest_enemy(3)
	wp := posOf(worm)

	turn(t, interp)
	assert.Equal(t, 0, impacts(lvl), "no bite from three tiles away")
	assert.Equal(t, 4, wp.GetX(), "worm slithers one tile toward the colonist")

	turn(t, interp)
	assert.Equal(t, 0, impacts(lvl), "no bite from two tiles away")
	assert.Equal(t, 5, wp.GetX())

	turn(t, interp)
	assert.Equal(t, 1, impacts(lvl), "bites once it is beside the colonist")
	assert.Equal(t, 5, wp.GetX(), "does not move on the turn it bites")
	assert.Equal(t, surfZ, wp.GetZ(), "stays on the surface the whole chase")
}

// TestSurfacedWormFollowsGround: chasing over the surface, the worm climbs or
// drops one level with the ground, but will not scale a cliff two levels high
// (and never floats off into the air).
func TestSurfacedWormFollowsGround(t *testing.T) {
	t.Run("one step up", func(t *testing.T) {
		// A one-level bump at x=4 between the worm and a colonist on its level.
		lvl := wormLevel(16, 8, 8, func(x, y int) int {
			if x == 4 {
				return 4
			}
			return 3
		})
		worm, interp := surfacedWorm(t, lvl, 3, 4, 3)
		_ = mob(lvl, "colony", 6, 4, 3)
		turn(t, interp)
		assert.Equal(t, 4, posOf(worm).GetX())
		assert.Equal(t, 4, posOf(worm).GetZ(), "worm climbs onto the bump")
		turn(t, interp)
		assert.Equal(t, 5, posOf(worm).GetX())
		assert.Equal(t, 3, posOf(worm).GetZ(), "and back down the far side")
	})
	t.Run("one step down", func(t *testing.T) {
		// A one-level dip at x=4 between the worm and a colonist on its level.
		lvl := wormLevel(16, 8, 8, func(x, y int) int {
			if x == 4 {
				return 2
			}
			return 3
		})
		worm, interp := surfacedWorm(t, lvl, 3, 4, 3)
		_ = mob(lvl, "colony", 6, 4, 3)
		turn(t, interp)
		assert.Equal(t, 4, posOf(worm).GetX())
		assert.Equal(t, 2, posOf(worm).GetZ(), "worm drops into the dip, not across the air above it")
	})
	t.Run("cliff blocks", func(t *testing.T) {
		// A two-level spike at x=4 between the worm and a colonist on its level.
		lvl := wormLevel(16, 8, 8, func(x, y int) int {
			if x == 4 {
				return 5
			}
			return 3
		})
		worm, interp := surfacedWorm(t, lvl, 3, 4, 3)
		_ = mob(lvl, "colony", 6, 4, 3)
		turn(t, interp)
		assert.Equal(t, 3, posOf(worm).GetX())
		assert.Equal(t, 3, posOf(worm).GetZ())
	})
}

// TestSurfacedWormRetreatsWhenPreyLeaves: a colonist it had locked onto who
// walks out of its three-tile reach is dropped, so the worm's surface timer
// runs out and it digs back down instead of staying up forever.
func TestSurfacedWormRetreatsWhenPreyLeaves(t *testing.T) {
	const surfZ = 4
	lvl := wormLevel(24, 8, 8, func(x, y int) int { return surfZ })
	worm, interp := surfacedWorm(t, lvl, 3, 4, surfZ)
	colonist := mob(lvl, "colony", 6, 4, surfZ)

	turn(t, interp) // locks on and steps to x=4
	require.Equal(t, 4, posOf(worm).GetX())

	lvl.PlaceEntity(20, 4, surfZ, colonist) // well out of reach (and of tremorsense)
	for i := 0; i < 6; i++ {
		turn(t, interp)
	}
	assert.Less(t, posOf(worm).GetZ(), surfZ, "worm should have dug back underground")
	assert.Equal(t, 0, impacts(lvl), "it never bit the colonist from afar")
}

// ramp is a surface that climbs one level every two columns east of x=4:
// z 3 up to x=4, 4 at x=5-6, 5 at x=7-8, 6 from x=9.
func ramp(x, y int) int {
	switch {
	case x <= 4:
		return 3
	case x <= 6:
		return 4
	case x <= 8:
		return 5
	default:
		return 6
	}
}

// runWorm plays turns of the worm's AI, failing if it ever ends a turn in the
// open air above its column's ground. Stops early once it has bitten.
func runWorm(t *testing.T, lvl *world.Level, worm *ecs.Entity, interp *basic.MechBasic, turns int) {
	t.Helper()
	wp := posOf(worm)
	for i := 0; i < turns && impacts(lvl) == 0; i++ {
		turn(t, interp)
		require.LessOrEqual(t, wp.GetZ(), lvl.GetSurfaceZ(wp.GetX(), wp.GetY()),
			"worm is floating in the air at (%d,%d,%d) after turn %d", wp.GetX(), wp.GetY(), wp.GetZ(), i)
	}
}

// TestSurfacedWormChasesUphill: a surfaced worm notices a colonist higher up
// the slope (not on its own z-level) and follows the ground up to bite it.
func TestSurfacedWormChasesUphill(t *testing.T) {
	lvl := wormLevel(16, 8, 10, ramp)
	worm, interp := surfacedWorm(t, lvl, 4, 4, 3)
	_ = mob(lvl, "colony", 7, 4, 5) // two levels up, three tiles east
	runWorm(t, lvl, worm, interp, 6)
	assert.Equal(t, 1, impacts(lvl), "worm should climb the slope and bite")
	assert.Equal(t, 6, posOf(worm).GetX())
	assert.Equal(t, 4, posOf(worm).GetZ(), "bites from the ground beside the colonist")
}

// TestSurfacedWormChasesDownhill: the same going down the slope.
func TestSurfacedWormChasesDownhill(t *testing.T) {
	lvl := wormLevel(16, 8, 10, ramp)
	worm, interp := surfacedWorm(t, lvl, 8, 4, 5)
	_ = mob(lvl, "colony", 5, 4, 4)
	runWorm(t, lvl, worm, interp, 6)
	assert.Equal(t, 1, impacts(lvl), "worm should come down the slope and bite")
	assert.Equal(t, 6, posOf(worm).GetX())
}

// TestBuriedWormHuntsUpHill: a worm buried under low ground senses a colonist
// up on a hill. It must rise only to its own column's surface (the old script
// kept rising straight up into the air to match the colonist's z) and then
// climb the hill along the ground.
func TestBuriedWormHuntsUpHill(t *testing.T) {
	lvl := wormLevel(16, 8, 10, ramp)
	worm := mob(lvl, "monsters", 3, 4, 1, fspath.BurrowingSkill)
	_ = mob(lvl, "colony", 9, 4, 6)
	script, err := os.ReadFile("../../data/scripts/ai/worm.basic")
	require.NoError(t, err)
	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, worm, lvl, &components.ScriptedAIComponent{Vars: map[string]any{}})
	require.NoError(t, interp.Load(string(script)))

	runWorm(t, lvl, worm, interp, 12)
	assert.Equal(t, 1, impacts(lvl), "worm should reach the colonist on the hill and bite")
}

// TestBuriedWormNeverFliesAtACliff: with the colonist on a plateau behind a
// sheer three-level cliff the worm cannot climb, it must still never leave
// the ground, whatever it does instead.
func TestBuriedWormNeverFliesAtACliff(t *testing.T) {
	lvl := wormLevel(16, 8, 10, func(x, y int) int {
		if x >= 6 {
			return 6
		}
		return 3
	})
	worm := mob(lvl, "monsters", 3, 4, 1, fspath.BurrowingSkill)
	_ = mob(lvl, "colony", 9, 4, 6)
	script, err := os.ReadFile("../../data/scripts/ai/worm.basic")
	require.NoError(t, err)
	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, worm, lvl, &components.ScriptedAIComponent{Vars: map[string]any{}})
	require.NoError(t, interp.Load(string(script)))

	runWorm(t, lvl, worm, interp, 40)
}
