package path_test

import (
	"testing"

	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/stretchr/testify/assert"
)

// ─── burrowing ────────────────────────────────────────────────────────────────
//
// The burrowing graph keys on TerrainKind, not tile layers, so these tests
// call AllocTerrain and SetTerrainKind directly rather than using the
// setWalkable/setWall helpers.

// newBurrowLevel creates a level with terrain allocated and every cell set to
// defaultKind. Individual cells can be overridden with SetTerrainKind.
func newBurrowLevel(w, h, d int, defaultKind world.TerrainKind) *world.Level {
	lvl := world.NewLevel(w, h, d)
	lvl.AllocTerrain()
	for z := 0; z < d; z++ {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				lvl.SetTerrainKind(x, y, z, defaultKind)
			}
		}
	}
	return lvl
}

// TestBurrowing_ThroughSolidUnderground verifies the core burrowing property:
// a burrowing entity can path through solid underground tiles (rock, dirt, etc.)
// that would block every other movement mode.
func TestBurrowing_ThroughSolidUnderground(t *testing.T) {
	fspath.ResetFrameCounter()
	// Single-row level, all cells tagged TKUnderground (solid rock).
	// No floor or middle tiles — the burrowing graph does not require them.
	lvl := newBurrowLevel(9, 1, 1, world.TKUnderground)

	entity := entityWithSkills(fspath.BurrowingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 0, 0, 0), tileAt(lvl, 8, 0, 0), nil)
	assert.NotNil(t, path, "burrowing entity should path through solid underground tiles")
}

// TestBurrowing_CanReachSurface verifies that a burrowing entity starting in
// underground rock can path up to a surface tile on the z-level above.
func TestBurrowing_CanReachSurface(t *testing.T) {
	fspath.ResetFrameCounter()
	// z=0: solid underground, z=1: surface layer.
	lvl := newBurrowLevel(5, 1, 2, world.TKUnderground)
	for x := 0; x < 5; x++ {
		lvl.SetTerrainKind(x, 0, 1, world.TKSurface)
	}

	entity := entityWithSkills(fspath.BurrowingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 0, 0, 0), tileAt(lvl, 4, 0, 1), nil)
	assert.NotNil(t, path, "burrowing entity should be able to surface from underground")
}

// TestBurrowing_BlockedBySpace verifies that space/vacuum tiles stop burrowers
// just as they stop all other non-spacefaring movement modes.
func TestBurrowing_BlockedBySpace(t *testing.T) {
	fspath.ResetFrameCounter()
	// Single-row underground level with one space tile blocking the middle.
	lvl := newBurrowLevel(9, 1, 1, world.TKUnderground)
	lvl.SetTerrainKind(4, 0, 0, world.TKSpace)

	entity := entityWithSkills(fspath.BurrowingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 0, 0, 0), tileAt(lvl, 8, 0, 0), nil)
	assert.Nil(t, path, "burrowing entity should be blocked by a space tile")
}

// TestBurrowing_BlockedByAtmosphere verifies that atmosphere tiles (open air
// above the surface) are treated as impassable for burrowers. Without this
// exclusion a burrower that breaks the surface would be able to route through
// the sky like a flying entity.
func TestBurrowing_BlockedByAtmosphere(t *testing.T) {
	fspath.ResetFrameCounter()
	// Single-row level with one atmosphere tile interrupting an underground path.
	lvl := newBurrowLevel(9, 1, 1, world.TKUnderground)
	lvl.SetTerrainKind(4, 0, 0, world.TKAtmosphere)

	entity := entityWithSkills(fspath.BurrowingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 0, 0, 0), tileAt(lvl, 8, 0, 0), nil)
	assert.Nil(t, path, "burrowing entity should be blocked by an atmosphere tile")
}

// TestBurrowing_BlockedByVoid confirms that void/out-of-region tiles (the zero
// value of TerrainKind) also stop burrowers — this is the default for any cell
// whose terrain was never explicitly set.
func TestBurrowing_BlockedByVoid(t *testing.T) {
	fspath.ResetFrameCounter()
	// Single-row level; the middle cell is left as TKVoid (zero value after
	// AllocTerrain, never overridden).
	lvl := world.NewLevel(9, 1, 1)
	lvl.AllocTerrain()
	for x := 0; x < 9; x++ {
		if x != 4 {
			lvl.SetTerrainKind(x, 0, 0, world.TKUnderground)
		}
		// x==4 stays TKVoid
	}

	entity := entityWithSkills(fspath.BurrowingSkill)
	path := fspath.GetPossiblePathForEntity(lvl, entity, tileAt(lvl, 0, 0, 0), tileAt(lvl, 8, 0, 0), nil)
	assert.Nil(t, path, "burrowing entity should be blocked by a void tile")
}
