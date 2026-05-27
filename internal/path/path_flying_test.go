package path_test

import (
	"testing"

	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/stretchr/testify/assert"
)

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
