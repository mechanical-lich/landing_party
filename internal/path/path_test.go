package path_test

import (
	"os"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
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

// newLevel creates a blank level. Tiles start with empty slots.
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

// setWalkable gives a cell a floor so a normal entity can stand on it.
// Middle stays empty (open air above floor).
func setWalkable(lvl *world.Level, x, y, z int) {
	lvl.SetFloor(x, y, z, "hull_floor", 0)
}

// setWall places a solid hull_wall in the middle slot (+ floor so the cell
// itself existed before being walled off, matching typical level layouts).
func setWall(lvl *world.Level, x, y, z int) {
	lvl.SetFloor(x, y, z, "hull_floor", 0)
	lvl.SetMiddle(x, y, z, "hull_wall", 0)
}

// setStairsUp paints floor + stairs_up middle. The entity stands here to
// begin a Z-upward transition.
func setStairsUp(lvl *world.Level, x, y, z int) {
	lvl.SetFloor(x, y, z, "hull_floor", 0)
	lvl.SetMiddle(x, y, z, "stairs_up", 0)
}

// setStairsDown paints stairs_down middle. Stair tiles are self-supporting so
// no floor is needed; the cost function skips the floor check for them.
func setStairsDown(lvl *world.Level, x, y, z int) {
	lvl.SetMiddle(x, y, z, "stairs_down", 0)
}

// setSpaceTile paints a space (vacuum) tile in the middle slot.
func setSpaceTile(lvl *world.Level, x, y, z int) {
	lvl.SetMiddle(x, y, z, "space", 0)
}

func entityWithSkills(skills ...string) *ecs.Entity {
	e := &ecs.Entity{}
	e.AddComponent(&components.SkillsComponent{Skills: skills})
	return e
}

// buildCorridor paints a walkable floor along y=1 from x=x0 to x=x1 at the
// given z-level, and solid walls at y=0 and y=2 to prevent detour paths.
func buildCorridor(lvl *world.Level, x0, x1, z int) {
	for x := x0; x <= x1; x++ {
		setWalkable(lvl, x, 1, z)
		setWall(lvl, x, 0, z)
		setWall(lvl, x, 2, z)
	}
}

// ─── flat (same Z) pathfinding ────────────────────────────────────────────────

func TestFlatPath(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 2)
	buildCorridor(lvl, 1, 7, 0)

	path := fspath.GetPossiblePath(lvl, tileAt(lvl, 1, 1, 0), tileAt(lvl, 7, 1, 0), nil)
	assert.NotNil(t, path, "should find path along flat corridor")
}

// TestFlatPath_FlyingBlockedBySolidWall uses a flying entity (whose graph truly
// excludes solid-middle tiles from the neighbor list) in a single-row, single-z
// level so there is no y or z detour available. A solid wall at the midpoint
// must produce nil.
func TestFlatPath_FlyingBlockedBySolidWall(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(9, 1, 1)
	for x := 0; x < 9; x++ {
		setWalkable(lvl, x, 0, 0)
	}
	setWall(lvl, 4, 0, 0)

	entity := entityWithSkills(fspath.FlyingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 0, 0, 0), tileAt(lvl, 8, 0, 0), nil)
	assert.Nil(t, path, "solid wall in single-row level should produce no path for flying entity")
}

// ─── stair Z-transitions (ground walker) ─────────────────────────────────────

// buildTwoLevelStairLayout creates the canonical two-floor stair scenario used
// by several tests:
//
//	z=0  walkable corridor x=1..7, stair_up at x=4
//	z=1  stair_down at x=4 (landing), walkable x=5..7
//
// The stairAt argument receives the x-coordinate of the stair column (4).
func buildTwoLevelStairLayout(lvl *world.Level) {
	buildCorridor(lvl, 1, 7, 0) // ground floor corridor
	setStairsUp(lvl, 4, 1, 0)   // stair foot

	setStairsDown(lvl, 4, 1, 1) // stair landing (self-supporting, no floor needed)
	// Upper-floor walkable cells with their containment walls.
	for x := 5; x <= 7; x++ {
		setWalkable(lvl, x, 1, 1)
		setWall(lvl, x, 0, 1)
		setWall(lvl, x, 2, 1)
	}
}

func TestStairZTransitionUp(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 3)
	buildTwoLevelStairLayout(lvl)

	path := fspath.GetPossiblePath(lvl, tileAt(lvl, 1, 1, 0), tileAt(lvl, 7, 1, 1), nil)
	require.NotNil(t, path, "should find path from z=0 to z=1 via stairs")
}

// TestStairZTransitionBlocked_SolidAtLanding is a regression test for a class
// of bug where a solid tile overwrites the stair landing at z=1 (e.g. a wall
// tile accidentally painted over the stairs_down position). The base-level
// PathNeighborIDs only admits z-transition neighbours with a stair tile in
// their Middle slot, so hull_wall there makes z=1 completely unreachable.
func TestStairZTransitionBlocked_SolidAtLanding(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 3)
	buildTwoLevelStairLayout(lvl)
	// Overwrite the stairs_down landing with a solid wall — this is the
	// regression scenario where the stair column's z=1 tile gets repainted.
	lvl.SetMiddle(4, 1, 1, "hull_wall", 0)

	path := fspath.GetPossiblePath(lvl, tileAt(lvl, 1, 1, 0), tileAt(lvl, 7, 1, 1), nil)
	assert.Nil(t, path, "solid wall painted over stair landing should prevent Z-transition")
}

// TestStairZTransitionBlocked_NoStairDown verifies that a stair_up at z=0
// without a matching stair_down at z=1 never produces a valid path.
// This protects against mis-painted stair columns (e.g. a floor tile replacing
// the stairs_down, which was the root cause of the historical stair regression).
func TestStairZTransitionBlocked_NoStairDown(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 3)
	buildCorridor(lvl, 1, 7, 0)
	setStairsUp(lvl, 4, 1, 0)
	// Intentionally omit setStairsDown so (4,1,1) has no stair tile — this
	// mirrors the scenario where a floor tile was placed instead.
	for x := 4; x <= 7; x++ {
		setWalkable(lvl, x, 1, 1)
		setWall(lvl, x, 0, 1)
		setWall(lvl, x, 2, 1)
	}

	path := fspath.GetPossiblePath(lvl, tileAt(lvl, 1, 1, 0), tileAt(lvl, 7, 1, 1), nil)
	assert.Nil(t, path, "missing stairs_down at landing should prevent Z-transition")
}

// ─── flying ───────────────────────────────────────────────────────────────────

func TestFlying_FlatPath(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 2)
	// Flying entities still navigate flat corridors normally.
	buildCorridor(lvl, 1, 7, 0)

	entity := entityWithSkills(fspath.FlyingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 1, 1, 0), tileAt(lvl, 7, 1, 0), nil)
	assert.NotNil(t, path, "flying entity should find flat path")
}

// TestFlying_ZUpWithoutStairs confirms that flying entities can transition
// between z-levels without stair tiles — they fly through open air cells.
func TestFlying_ZUpWithoutStairs(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 3)
	// z=0: walkable floor corridor.
	buildCorridor(lvl, 1, 7, 0)
	// z=1: open air cells (no floor, no middle) — flying entities can occupy
	// these. Containment walls prevent y-axis detours.
	for x := 1; x <= 7; x++ {
		setWall(lvl, x, 0, 1)
		setWall(lvl, x, 2, 1)
	}
	// No stairs anywhere.

	entity := entityWithSkills(fspath.FlyingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 1, 1, 0), tileAt(lvl, 5, 1, 1), nil)
	assert.NotNil(t, path, "flying entity should reach z=1 through open air without stairs")
}

// TestFlying_ZUpViaStairs confirms that flying entities can still use stairs
// for Z-transitions (they are not forced to avoid them).
func TestFlying_ZUpViaStairs(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 3)
	buildTwoLevelStairLayout(lvl)

	entity := entityWithSkills(fspath.FlyingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 1, 1, 0), tileAt(lvl, 7, 1, 1), nil)
	assert.NotNil(t, path, "flying entity should be able to use stairs")
}

// TestFlying_ZUpBlockedByFloor verifies that a floor placed at z=1 prevents a
// flying entity from entering that z-level from below — the floor acts as a
// physical ceiling blocking upward flight.
func TestFlying_ZUpBlockedByFloor(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 3)
	buildCorridor(lvl, 1, 7, 0)
	// Seal the z=1 level with floor tiles on every cell so flying up is blocked.
	for x := 0; x < 10; x++ {
		for y := 0; y < 5; y++ {
			setWalkable(lvl, x, y, 1)
		}
	}

	entity := entityWithSkills(fspath.FlyingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 1, 1, 0), tileAt(lvl, 5, 1, 1), nil)
	assert.Nil(t, path, "floor at z=1 should block upward flight entry")
}

// TestFlying_BlockedBySpaceTile confirms that the flying skill does NOT grant
// traversal of space/vacuum tiles. Only spacefaring does.
//
// Uses a single-row, single-z level (no y or z detour) so the flying graph's
// exclusion of space-middle tiles produces a true nil result.
func TestFlying_BlockedBySpaceTile(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(9, 1, 1)
	for x := 0; x < 9; x++ {
		setWalkable(lvl, x, 0, 0)
	}
	setSpaceTile(lvl, 4, 0, 0)

	entity := entityWithSkills(fspath.FlyingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 0, 0, 0), tileAt(lvl, 8, 0, 0), nil)
	assert.Nil(t, path, "flying skill should not allow traversal of space tiles")
}

// ─── spacefaring ─────────────────────────────────────────────────────────────

// TestSpacefaring_TraversesSpaceTiles confirms that spacefaring entities can
// move through space/vacuum tiles that block all other movement modes.
func TestSpacefaring_TraversesSpaceTiles(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 2)
	buildCorridor(lvl, 1, 7, 0)
	// Space tile blocks the middle of the corridor for non-spacefaring entities.
	setSpaceTile(lvl, 4, 1, 0)

	entity := entityWithSkills(fspath.SpacefaringSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 1, 1, 0), tileAt(lvl, 7, 1, 0), nil)
	assert.NotNil(t, path, "spacefaring entity should traverse space tiles")
}

// TestSpacefaring_ZUpThroughSpace verifies spacefaring Z-transitions through
// a space column — no stairs needed, space tiles are passable.
func TestSpacefaring_ZUpThroughSpace(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 3)
	buildCorridor(lvl, 1, 7, 0)
	// Entire z=1 level is vacuum space.
	for x := 0; x < 10; x++ {
		for y := 0; y < 5; y++ {
			setSpaceTile(lvl, x, y, 1)
		}
	}

	entity := entityWithSkills(fspath.SpacefaringSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 1, 1, 0), tileAt(lvl, 5, 1, 1), nil)
	assert.NotNil(t, path, "spacefaring entity should navigate through a space z-level")
}

// ─── climbing ────────────────────────────────────────────────────────────────

// TestClimbing_ZUpWithAdjacentWall verifies that a climbing entity can
// transition from z=0 to z=1 when an adjacent wall exists at the source.
//
// Layout: open floor corridor on z=0 (no containment walls so they cannot
// accidentally satisfy hasAdjacentWall), one solid wall placed at (1,2,0)
// immediately beside the start tile (2,2,0). z=1 is open air.
func TestClimbing_ZUpWithAdjacentWall(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 3)
	// Open floor — no walls anywhere except the one placed explicitly below.
	for x := 1; x <= 7; x++ {
		setWalkable(lvl, x, 2, 0)
	}
	setWall(lvl, 1, 2, 0) // the wall the entity grabs to climb

	entity := entityWithSkills(fspath.ClimbingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 2, 2, 0), tileAt(lvl, 6, 2, 1), nil)
	assert.NotNil(t, path, "climbing entity with adjacent wall should reach z=1")
}

// TestClimbing_ZUpBlockedWithoutWall confirms that a climbing entity cannot
// change z-levels when no adjacent solid wall is present anywhere in the level.
// Every tile in the path has an empty middle, so hasAdjacentWall is always
// false and z-transitions are never allowed.
func TestClimbing_ZUpBlockedWithoutWall(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 5, 3)
	// Plain open floor — no walls of any kind in the level.
	for x := 1; x <= 7; x++ {
		setWalkable(lvl, x, 2, 0)
	}

	entity := entityWithSkills(fspath.ClimbingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 2, 2, 0), tileAt(lvl, 6, 2, 1), nil)
	assert.Nil(t, path, "climbing entity without any adjacent wall should not reach z=1")
}

// TestClimbing_FlatPathNoWallNeeded verifies that horizontal movement for a
// climbing entity works normally and does not require an adjacent wall.
func TestClimbing_FlatPathNoWallNeeded(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(10, 6, 3)
	// Plain flat corridor, no walls adjacent to the path.
	for x := 1; x <= 7; x++ {
		setWalkable(lvl, x, 2, 0)
		setWall(lvl, x, 1, 0)
		setWall(lvl, x, 3, 0)
	}

	entity := entityWithSkills(fspath.ClimbingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 1, 2, 0), tileAt(lvl, 7, 2, 0), nil)
	assert.NotNil(t, path, "climbing entity should move horizontally without needing a wall")
}

// ─── vacuum_resist (ground-level space traversal) ────────────────────────────

// TestVacuumResist_TraversesSpaceTile verifies that an entity wearing enviro
// gear (vacuum_resist skill) can move through pure-vacuum space tiles (space
// middle, no floor) on the ground level using the standard pathfinder.
//
// Vacuum-resist bypasses the floating check, so the path is returned even
// though the space tile has no floor to stand on.
func TestVacuumResist_TraversesSpaceTile(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(9, 1, 1)
	for x := 0; x < 9; x++ {
		setWalkable(lvl, x, 0, 0)
	}
	// Pure vacuum cell: space in middle, no floor layer.
	lvl.SetMiddle(4, 0, 0, "space", 0)
	lvl.ClearFloor(4, 0, 0)

	entity := entityWithSkills(fspath.VacuumResistSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 0, 0, 0), tileAt(lvl, 8, 0, 0), nil)
	assert.NotNil(t, path, "vacuum_resist entity should traverse pure-vacuum space tiles")
}

// TestNoSkill_BlockedBySpaceTile confirms a baseline: a plain entity (no
// skills) cannot pass through a pure-vacuum space tile.
//
// Without vacuum_resist the post-path floating check fires (space tile +
// no floor + no vacuumResist), causing getPossiblePath to discard the
// path candidate and ultimately return nil.
func TestNoSkill_BlockedBySpaceTile(t *testing.T) {
	fspath.ResetFrameCounter()
	lvl := newLevel(9, 1, 1)
	for x := 0; x < 9; x++ {
		setWalkable(lvl, x, 0, 0)
	}
	// Same pure-vacuum cell as the vacuum_resist test above.
	lvl.SetMiddle(4, 0, 0, "space", 0)
	lvl.ClearFloor(4, 0, 0)

	entity := &ecs.Entity{}
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 0, 0, 0), tileAt(lvl, 8, 0, 0), nil)
	assert.Nil(t, path, "entity without vacuum_resist should be blocked by pure-vacuum space tile")
}
