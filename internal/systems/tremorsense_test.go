package systems

import (
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

// mob builds a positioned, living entity of the given faction and adds it to
// the level so the spatial index (entityPos) can find it.
func mob(lvl *world.Level, faction string, x, y, z int, skills ...string) *ecs.Entity {
	e := &ecs.Entity{}
	e.AddComponent(&rlcomponents.PositionComponent{X: x, Y: y, Z: z})
	e.AddComponent(&rlcomponents.HealthComponent{MaxHealth: 10, Health: 10})
	e.AddComponent(&rlcomponents.DescriptionComponent{Faction: faction})
	if len(skills) > 0 {
		e.AddComponent(&components.SkillsComponent{Skills: skills})
	}
	lvl.AddEntity(e)
	return e
}

func callNum(t *testing.T, interp *basic.MechBasic, fn string) float64 {
	t.Helper()
	v, err := interp.Call(fn)
	require.NoError(t, err)
	return toAIFloat(v)
}

// TestTremorsenseSeesThroughRockAcrossZ is the regression test for the worm
// never surfacing: find_nearest_enemy only scans the caller's own z-plane (and
// requires line-of-sight), so a buried worm cannot detect a colonist on the
// surface. sense_nearest_enemy sweeps every z-level without LOS and finds it.
func TestTremorsenseSeesThroughRockAcrossZ(t *testing.T) {
	lvl := world.NewLevel(16, 16, 12)
	worm := mob(lvl, "monsters", 4, 4, 2)          // buried deep
	_ = mob(lvl, "colony", 4, 4, 9)                // colonist on the surface, 7 z above
	ai := &components.ScriptedAIComponent{Vars: map[string]any{}}

	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, worm, lvl, ai)
	require.NoError(t, interp.Load(`
function probe_find()
    return find_nearest_enemy(10)
endfunction
function probe_sense()
    return sense_nearest_enemy(10)
endfunction
function sensed_z()
    sense_nearest_enemy(10)
    return get_nearest_z()
endfunction
`))

	// Old sense is blind: the enemy is on a different z-plane.
	assert.Equal(t, 0.0, callNum(t, interp, "probe_find"),
		"find_nearest_enemy should NOT see an enemy on another z-plane")

	// Tremorsense feels it through the rock and reports its true z.
	assert.Equal(t, 1.0, callNum(t, interp, "probe_sense"),
		"sense_nearest_enemy should detect the surface colonist from underground")
	assert.Equal(t, 9.0, callNum(t, interp, "sensed_z"),
		"tremorsense should report the colonist's z so the worm knows to rise")
}

// TestTremorsenseRespectsFactionAndRange confirms the sense ignores allies and
// anything outside its radius.
func TestTremorsenseRespectsFactionAndRange(t *testing.T) {
	lvl := world.NewLevel(64, 16, 12)
	worm := mob(lvl, "monsters", 2, 2, 2)
	_ = mob(lvl, "monsters", 2, 2, 9) // ally directly above — must be ignored
	_ = mob(lvl, "colony", 40, 2, 9)  // enemy far outside radius 10
	ai := &components.ScriptedAIComponent{Vars: map[string]any{}}

	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, worm, lvl, ai)
	require.NoError(t, interp.Load(`
function probe()
    return sense_nearest_enemy(10)
endfunction
`))
	assert.Equal(t, 0.0, callNum(t, interp, "probe"),
		"tremorsense should ignore same-faction and out-of-range entities")
}

// TestPlaySoundEmitsEntityClip: the play_sound primitive resolves the entity's
// SoundComponent and emits a positional sound carrying the resolved clip.
func TestPlaySoundEmitsEntityClip(t *testing.T) {
	lvl := world.NewLevel(8, 8, 4)
	e := mob(lvl, "monsters", 3, 3, 1)
	e.AddComponent(&components.SoundComponent{Sounds: map[string]string{"alert": "sfx_worm_alert"}})
	ai := &components.ScriptedAIComponent{Vars: map[string]any{}}

	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, e, lvl, ai)
	require.NoError(t, interp.Load(`
function probe()
    play_sound("alert")
    return 1
endfunction
`))
	_, err := interp.Call("probe")
	require.NoError(t, err)

	if len(lvl.Sounds) != 1 {
		t.Fatalf("expected 1 emitted sound, got %d", len(lvl.Sounds))
	}
	if lvl.Sounds[0].Clip != "sfx_worm_alert" {
		t.Fatalf("clip = %q, want sfx_worm_alert", lvl.Sounds[0].Clip)
	}
	if lvl.Sounds[0].Tag != world.SoundTag("alert") {
		t.Fatalf("tag = %q, want alert", lvl.Sounds[0].Tag)
	}
}

// TestPlaySoundNoopWithoutClip: play_sound does nothing when the entity has no
// sound for the event, so scripts can call it unconditionally.
func TestPlaySoundNoopWithoutClip(t *testing.T) {
	lvl := world.NewLevel(8, 8, 4)
	e := mob(lvl, "monsters", 3, 3, 1) // no SoundComponent
	ai := &components.ScriptedAIComponent{Vars: map[string]any{}}

	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, e, lvl, ai)
	require.NoError(t, interp.Load(`
function probe()
    play_sound("alert")
    return 1
endfunction
`))
	_, err := interp.Call("probe")
	require.NoError(t, err)

	if len(lvl.Sounds) != 0 {
		t.Fatalf("expected no sound without a clip, got %d", len(lvl.Sounds))
	}
}

// TestWormSurfacesTowardPrey drives the real worm.basic on_turn against a buried
// worm and a surface colonist, and asserts the worm climbs out of the ground to
// the surface z instead of burrowing in circles at a fixed depth.
func TestWormSurfacesTowardPrey(t *testing.T) {
	const surfaceZ = 7
	lvl := world.NewLevel(20, 8, 10)
	lvl.AllocTerrain()
	for z := 0; z < lvl.GetDepth(); z++ {
		for y := 0; y < lvl.GetHeight(); y++ {
			for x := 0; x < lvl.GetWidth(); x++ {
				switch {
				case z < surfaceZ:
					lvl.SetTerrainKind(x, y, z, world.TKUnderground)
				case z == surfaceZ:
					lvl.SetTerrainKind(x, y, z, world.TKSurface)
					lvl.SetSurfaceZ(x, y, surfaceZ)
				default:
					lvl.SetTerrainKind(x, y, z, world.TKAtmosphere)
				}
			}
		}
	}

	worm := mob(lvl, "monsters", 3, 4, 2, fspath.BurrowingSkill)
	// Colonist within tremorsense range (|dx|=5 <= 10) so the worm senses it,
	// but far enough that it surfaces and chases without reaching melee range in
	// the turns we run (no combat side effects to set up).
	_ = mob(lvl, "colony", 8, 4, surfaceZ)

	script, err := os.ReadFile("../../data/scripts/ai/worm.basic")
	require.NoError(t, err)
	ai := &components.ScriptedAIComponent{Vars: map[string]any{}}
	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, worm, lvl, ai)
	require.NoError(t, interp.Load(string(script)))

	wpos := worm.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	startX := wpos.GetX()
	// 5 turns to rise (z 2→7) plus a couple of surface steps — the colonist at
	// x=8 stays out of melee range (nearest the worm gets is x=7, |dx|=1 only on
	// a later turn), so no attack fires.
	for turn := 0; turn < 8; turn++ {
		_, err := interp.Call("on_turn")
		require.NoError(t, err)
		// The worm must never rise above the surface (it can't fly).
		require.LessOrEqual(t, wpos.GetZ(), surfaceZ,
			"worm burrowed into the sky at turn %d", turn)
	}

	assert.Equal(t, surfaceZ, wpos.GetZ(),
		"worm should climb from z=2 to the surface to reach the colonist")
	assert.Greater(t, wpos.GetX(), startX,
		"once surfaced the worm should move horizontally toward the colonist")
}
