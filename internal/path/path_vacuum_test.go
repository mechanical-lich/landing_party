package path_test

import (
	"testing"

	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/stretchr/testify/assert"
)

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
