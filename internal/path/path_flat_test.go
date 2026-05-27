package path_test

import (
	"testing"

	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/stretchr/testify/assert"
)

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
