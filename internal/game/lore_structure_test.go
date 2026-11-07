package game

import (
	"fmt"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// loreFlatLevel is an open planet surface for running one structure script.
func loreFlatLevel(t *testing.T, w, h int) *world.Level {
	t.Helper()
	level := world.NewLevel(w, h, 10)
	if err := (generation.PlanetPrimer{}).Prime(level, map[string]any{"mountain_frequency": 0.0}, 1); err != nil {
		t.Fatal(err)
	}
	return level
}

// lorePosts counts the two wall faults the autotiler draws as rows of posts:
// a 2x2 block of wall (two walls laid side by side a tile apart, as when a
// chamber misses the keep wall by one) and a wall tile with no wall on any
// side.
func lorePosts(level *world.Level, wall string, x0, y0, w, h, z int) (posts int, where []string) {
	return lorePostsWhere(level, wall, x0, y0, w, h, z, nil)
}

// lorePostsWhere is lorePosts limited to the tiles keep accepts (nil: all).
func lorePostsWhere(level *world.Level, wall string, x0, y0, w, h, z int, keep func(dx, dy int) bool) (posts int, where []string) {
	isWall := func(x, y int) bool {
		t := level.GetTilePtr(x, y, z)
		if t == nil || t.Middle.IsEmpty() {
			return false
		}
		if wall == "" {
			return t.IsSolid()
		}
		return world.TileDefinitions[t.Middle.Type].Name == wall
	}
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			if !isWall(x, y) || (keep != nil && !keep(x-x0, y-y0)) {
				continue
			}
			lone := !isWall(x-1, y) && !isWall(x+1, y) && !isWall(x, y-1) && !isWall(x, y+1)
			block := isWall(x+1, y) && isWall(x, y+1) && isWall(x+1, y+1)
			if lone || block {
				posts++
				where = append(where, fmt.Sprintf("%d,%d", x-x0, y-y0))
			}
		}
	}
	return posts, where
}

// The castle's chambers share the keep's walls exactly: across footprints
// (odd and even, down to its minimum) and seeds, no wall stands alone.
func TestLoreCastleHasNoPosts(t *testing.T) {
	loreLoadData(t)
	for _, size := range [][2]int{{28, 22}, {29, 23}, {18, 16}, {19, 17}, {21, 19}, {25, 25}} {
		for seed := int64(1); seed <= 20; seed++ {
			level := loreFlatLevel(t, 40, 40)
			x0, y0 := 5, 5
			if err := RunStructureScriptWithParams(level, "castle", x0, y0, size[0], size[1], map[string]any{"occupants": "none"}, seed); err != nil {
				t.Fatal(err)
			}
			if n, where := lorePosts(level, "stone_wall", x0, y0, size[0], size[1], level.GetSurfaceZ(x0+size[0]/2, y0+size[1]/2)); n > 0 {
				t.Errorf("castle %dx%d seed %d: %d doubled or lone wall tiles at %v", size[0], size[1], seed, n, where)
			}
		}
	}
}

// The structures history stamps, at the sizes it stamps them, lay no two walls
// side by side and leave no wall standing alone: the faults the autotiler
// draws as rows of posts. Round towers are checked inside their outline (a
// circle's outline steps diagonally by design); the reactor's solid core and
// the crashed ship's round hull are left out.
func TestLoreHistoryStructuresHaveNoPosts(t *testing.T) {
	loreLoadData(t)
	for _, c := range []struct {
		script string
		w, h   int
		round  bool
	}{{"castle", 28, 22, false}, {"underground_hideout", 21, 17, false}, {"underground_bunker", 15, 13, false},
		{"bunker", 11, 9, false}, {"underground_dungeon", 25, 21, false},
		{"circle_tower", 25, 25, true}, {"circle_ruined_tower", 25, 25, true}} {
		for seed := int64(1); seed <= 20; seed++ {
			level := loreFlatLevel(t, 40, 40)
			x0, y0 := 5, 5
			sz := level.GetSurfaceZ(x0+c.w/2, y0+c.h/2)
			if err := RunStructureScriptWithParams(level, c.script, x0, y0, c.w, c.h, map[string]any{"occupants": "none"}, seed); err != nil {
				t.Fatal(err)
			}
			var keep func(dx, dy int) bool
			if c.round {
				r := c.w/2 - 1
				keep = func(dx, dy int) bool {
					ex, ey := dx-c.w/2, dy-c.h/2
					return ex*ex+ey*ey < (r-1)*(r-1)
				}
			}
			if n, where := lorePostsWhere(level, "", x0, y0, c.w, c.h, sz, keep); n > 0 {
				t.Errorf("%s %dx%d seed %d: %d doubled or lone wall tiles at %v", c.script, c.w, c.h, seed, n, where)
			}
		}
	}
}
