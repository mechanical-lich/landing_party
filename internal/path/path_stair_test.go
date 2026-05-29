package path_test

import (
	"testing"

	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── stair Z-transitions (ground walker) ─────────────────────────────────────

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
