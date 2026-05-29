package path_test

import (
	"testing"

	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/stretchr/testify/assert"
)

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
