package path_test

import (
	"testing"

	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/stretchr/testify/assert"
)

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
