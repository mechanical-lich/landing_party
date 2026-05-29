package path_test

import (
	"os"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
)

func TestMain(m *testing.M) {
	if err := world.LoadTileDefinitionsDir("../../data/tiledefinitions"); err != nil {
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

// buildTwoLevelStairLayout creates the canonical two-floor stair scenario used
// by several tests:
//
//	z=0  walkable corridor x=1..7, stair_up at x=4
//	z=1  stair_down at x=4 (landing), walkable x=5..7
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
