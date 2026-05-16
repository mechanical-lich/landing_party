package ai

import (
	"os"
	"testing"

	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	fspath "github.com/mechanical-lich/scifi_settlements/internal/path"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := world.LoadTileDefinitions("../../data/tile_definitions.json"); err != nil {
		panic("failed to load tile definitions: " + err.Error())
	}
	os.Exit(m.Run())
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func newLevel(w, h, d int) *world.Level {
	return world.NewLevel(w, h, d)
}

func tileAt(lvl *world.Level, x, y, z int) *world.Tile {
	t := lvl.GetTilePtr(x, y, z)
	if t == nil {
		panic("tileAt: out of bounds")
	}
	return t
}

func setWalkable(lvl *world.Level, x, y, z int) {
	lvl.SetFloor(x, y, z, "hull_floor", 0)
}

func setWall(lvl *world.Level, x, y, z int) {
	lvl.SetFloor(x, y, z, "hull_floor", 0)
	lvl.SetMiddle(x, y, z, "hull_wall", 0)
}

func setStairsUp(lvl *world.Level, x, y, z int) {
	lvl.SetFloor(x, y, z, "hull_floor", 0)
	lvl.SetMiddle(x, y, z, "stairs_up", 0)
}

// setStairsDown paints stairs_down with no floor — stair tiles are
// self-supporting. This is the exact layout that triggered the canMoveTo bug.
func setStairsDown(lvl *world.Level, x, y, z int) {
	lvl.SetMiddle(x, y, z, "stairs_down", 0)
}

// workerEntity builds a minimal entity with the components MoveTowardsTarget
// requires: Position (placed at x,y,z on the level) and AIMemory.
func workerEntity(lvl *world.Level, x, y, z int, skills ...string) *ecs.Entity {
	e := &ecs.Entity{}
	pc := &rlcomponents.PositionComponent{X: x, Y: y, Z: z}
	e.AddComponent(pc)
	e.AddComponent(&rlcomponents.AIMemoryComponent{TargetX: -1, TargetY: -1})
	if len(skills) > 0 {
		e.AddComponent(&components.SkillsComponent{Skills: skills})
	}
	lvl.AddEntity(e)
	return e
}

// buildStairLayout creates the canonical two-floor layout used by movement tests:
//
//	z=0: walkable corridor x=1..7, stair_up at x=4
//	z=1: stair_down at x=4 (no floor — self-supporting), walkable x=5..7
func buildStairLayout(lvl *world.Level) {
	for x := 1; x <= 7; x++ {
		setWalkable(lvl, x, 1, 0)
	}
	setStairsUp(lvl, 4, 1, 0)
	setStairsDown(lvl, 4, 1, 1) // no floor — this is the tile canMoveTo failed on
	for x := 5; x <= 7; x++ {
		setWalkable(lvl, x, 1, 1)
	}
}

// ─── canMoveTo unit tests ─────────────────────────────────────────────────────

// TestCanMoveTo_WalkableFloor confirms that a normal tile with a floor is
// always passable.
func TestCanMoveTo_WalkableFloor(t *testing.T) {
	lvl := newLevel(5, 5, 2)
	setWalkable(lvl, 2, 2, 0)
	entity := &ecs.Entity{}

	assert.True(t, canMoveTo(lvl, entity, tileAt(lvl, 2, 2, 0)), "tile with floor should be passable")
}

// TestCanMoveTo_NoFloor confirms that a tile with no floor blocks a ground entity.
func TestCanMoveTo_NoFloor(t *testing.T) {
	lvl := newLevel(5, 5, 2)
	// Leave tile empty (no floor, no middle).
	entity := &ecs.Entity{}

	assert.False(t, canMoveTo(lvl, entity, tileAt(lvl, 2, 2, 0)), "tile with no floor should block ground entity")
}

// TestCanMoveTo_SolidWall confirms that a solid wall tile is never passable.
func TestCanMoveTo_SolidWall(t *testing.T) {
	lvl := newLevel(5, 5, 2)
	setWall(lvl, 2, 2, 0)
	entity := &ecs.Entity{}

	assert.False(t, canMoveTo(lvl, entity, tileAt(lvl, 2, 2, 0)), "solid wall should not be passable")
}

// TestCanMoveTo_StairsUpWithFloor confirms that a stairs_up tile (which always
// has a floor) is passable for a ground entity.
func TestCanMoveTo_StairsUpWithFloor(t *testing.T) {
	lvl := newLevel(5, 5, 2)
	setStairsUp(lvl, 2, 2, 0) // floor + stairs_up middle
	entity := &ecs.Entity{}

	assert.True(t, canMoveTo(lvl, entity, tileAt(lvl, 2, 2, 0)), "stairs_up tile with floor should be passable")
}

// TestCanMoveTo_StairsDownNoFloor is the direct regression test for the bug
// where canMoveTo blocked workers at the stair landing on z=1.
//
// stairs_down tiles are self-supporting (no floor slot needed), so they must
// be passable even when the Floor slot is empty. Before the fix, this returned
// false and workers would reach the stair foot but never climb to z=1.
func TestCanMoveTo_StairsDownNoFloor(t *testing.T) {
	lvl := newLevel(5, 5, 2)
	setStairsDown(lvl, 2, 2, 1) // no floor — exactly what z=1 stair landings look like
	entity := &ecs.Entity{}

	assert.True(t, canMoveTo(lvl, entity, tileAt(lvl, 2, 2, 1)),
		"stairs_down tile without floor should be passable (stair tiles are self-supporting)")
}

// TestCanMoveTo_FlyingNoFloor confirms that a flying entity can enter a
// tile with no floor and no middle.
func TestCanMoveTo_FlyingNoFloor(t *testing.T) {
	lvl := newLevel(5, 5, 2)
	// Empty cell at z=1 — no floor, no middle.
	entity := &ecs.Entity{}
	entity.AddComponent(&components.SkillsComponent{Skills: []string{fspath.FlyingSkill}})

	assert.True(t, canMoveTo(lvl, entity, tileAt(lvl, 2, 2, 1)), "flying entity should enter tile with no floor")
}

// ─── MoveTowardsTarget integration tests ─────────────────────────────────────

// TestMoveTowardsTarget_StairLanding is the end-to-end regression test.
// It simulates a worker that must climb from z=0 to z=1 via stairs and
// verifies it actually advances past the stair landing — the exact failure
// mode of the canMoveTo bug.
func TestMoveTowardsTarget_StairLanding(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 3)
	buildStairLayout(lvl)

	// Worker starts at (1,1,0); target is (7,1,1) — requires climbing stairs.
	worker := workerEntity(lvl, 1, 1, 0)

	// Simulate up to 30 movement ticks. The worker must eventually reach z=1.
	reachedUpperFloor := false
	for tick := 0; tick < 30; tick++ {
		fspath.ResetFrameCounter()
		MoveTowardsTarget(lvl, worker, 7, 1, 1)
		pc := worker.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if pc.GetZ() == 1 {
			reachedUpperFloor = true
			break
		}
	}

	require.True(t, reachedUpperFloor, "worker should reach z=1 by climbing stairs within 30 ticks")
}

// TestMoveTowardsTarget_StairLanding_ReachesDestination extends the above to
// confirm the worker fully reaches the upper-floor destination, not just the
// stair landing.
func TestMoveTowardsTarget_StairLanding_ReachesDestination(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 3)
	buildStairLayout(lvl)

	worker := workerEntity(lvl, 1, 1, 0)

	reachedTarget := false
	for tick := 0; tick < 40; tick++ {
		fspath.ResetFrameCounter()
		moved := MoveTowardsTarget(lvl, worker, 7, 1, 1)
		pc := worker.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if pc.GetX() == 7 && pc.GetY() == 1 && pc.GetZ() == 1 {
			reachedTarget = true
			break
		}
		if !moved {
			break
		}
	}

	assert.True(t, reachedTarget, "worker should fully reach destination (7,1,1) on the upper floor")
}

// TestMoveTowardsTarget_FlatPath confirms baseline movement on a single z-level
// still works after the stair fix.
func TestMoveTowardsTarget_FlatPath(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 2)
	for x := 1; x <= 7; x++ {
		setWalkable(lvl, x, 1, 0)
	}

	worker := workerEntity(lvl, 1, 1, 0)

	reached := false
	for tick := 0; tick < 20; tick++ {
		fspath.ResetFrameCounter()
		MoveTowardsTarget(lvl, worker, 7, 1, 0)
		pc := worker.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if pc.GetX() == 7 && pc.GetY() == 1 && pc.GetZ() == 0 {
			reached = true
			break
		}
	}

	assert.True(t, reached, "worker should reach flat destination within 20 ticks")
}
