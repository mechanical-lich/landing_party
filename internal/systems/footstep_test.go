package systems

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mechanical-basic/pkg/basic"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/stretchr/testify/require"
)

// TestScriptedMoveEmitsFootstep guards the return-value inversion bug: a scripted
// mob (zombies use these primitives) must emit a footstep when it actually
// steps to a new tile. rlentity.Move returns false on a successful move, so the
// emit keys off a real position change, not that return.
func TestScriptedMoveEmitsFootstep(t *testing.T) {
	if err := world.LoadTileDefinitionsDir("../../data/tiledefinitions"); err != nil {
		t.Fatalf("load tile defs: %v", err)
	}
	lvl := world.NewLevel(6, 3, 1)
	lvl.AllocTerrain()
	// A walkable floor row so rlentity.Move succeeds.
	for x := 0; x < 6; x++ {
		lvl.SetFloor(x, 1, 0, "hull_floor", 0)
	}

	worm := mob(lvl, "monsters", 1, 1, 0) // reuse the tremorsense test helper
	ai := &components.ScriptedAIComponent{Vars: map[string]any{}}
	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, worm, lvl, ai)
	require.NoError(t, interp.Load(`
function step()
    return move_by(1, 0)
endfunction
`))

	_, err := interp.Call("step")
	require.NoError(t, err)

	// It actually moved east.
	pc := worm.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if pc.GetX() != 2 {
		t.Fatalf("mob at x=%d, want 2 (should have stepped)", pc.GetX())
	}

	// ...and a footstep fired at the destination.
	var footsteps int
	for _, s := range lvl.Sounds {
		if s.Tag == world.SoundTagFootstep {
			footsteps++
			if s.X != 2 || s.Y != 1 {
				t.Fatalf("footstep at (%d,%d), want (2,1)", s.X, s.Y)
			}
		}
	}
	if footsteps != 1 {
		t.Fatalf("emitted %d footsteps, want 1", footsteps)
	}
}
