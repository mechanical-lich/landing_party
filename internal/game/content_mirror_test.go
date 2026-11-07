package game

import (
	"fmt"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// contentTileMap is what a build left, keyed by tile: the floor/middle tile
// types, each entity covering it (by its Size) and the innermost room name.
func contentTileMap(level *world.Level, x0, y0, w, h, z int) map[[2]int]string {
	out := map[[2]int]string{}
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			tl := level.GetTilePtr(x, y, z)
			out[[2]int{x - x0, y - y0}] = fmt.Sprintf("%d/%d %s", tl.Floor.Type, tl.Middle.Type, level.RoomAt(x, y, z))
		}
	}
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			pc, ok := entityPos(e)
			if !ok || e.Blueprint == "" {
				continue
			}
			sw, sh := 1, 1
			if e.HasComponent(rlcomponents.Size) {
				sc := e.GetComponent(rlcomponents.Size).(*rlcomponents.SizeComponent)
				sw, sh = max(sc.Width, 1), max(sc.Height, 1)
			}
			for yy := pc.GetY() - sh/2; yy < pc.GetY()-sh/2+sh; yy++ {
				for xx := pc.GetX() - sw/2; xx < pc.GetX()-sw/2+sw; xx++ {
					out[[2]int{xx - x0, yy - y0}] += " " + e.Blueprint
				}
			}
		}
	}
	return out
}

// mirror(x, y, w, h, fx, fy) reflects everything a script then builds, a
// nested structure and a two-tile bed included, exactly.
func TestContentMirrorReflects(t *testing.T) {
	loreLoadData(t)
	contentScript(t, "content_mirror_inner", `
function generate(x, y, w, h)
    sz = get_surface_z(x, y)
    set_tile(x, y, sz, "rubble_pile")
    spawn_entity("bed", x + 1, y + 1, sz)
endfunction
`)
	contentScript(t, "content_mirror", `
function generate(x, y, w, h)
    mirror(x, y, w, h, get_param("fx") + 0, get_param("fy") + 0)
    sz = get_surface_z(x + 1, y + 1)
    carve_room(x, y, sz, 5, 4, "stone_wall", "stone_floor")
    name_room(x, y, sz, 5, 4, "Corner")
    clear_middle(x + 2, y + 3, sz)
    spawn_entity("storage_locker", x + 1, y + 1, sz)
    spawn_entity("bed", x + 3, y + 2, sz)
    spawn_entity("bed", x + 8, y + 7, sz)
    gen_structure("content_mirror_inner", x + 6, y + 1, 3, 3)
endfunction
`)
	const w, h, m = 11, 9, 3
	build := func(fx, fy float64) map[[2]int]string {
		level := loreFlatLevel(t, w+2*m, h+2*m)
		z := level.GetSurfaceZ(m+1, m+1)
		if err := RunStructureScriptWithParams(level, "content_mirror", m, m, w, h, map[string]any{"fx": fx, "fy": fy}, 1); err != nil {
			t.Fatal(err)
		}
		return contentTileMap(level, m, m, w, h, z)
	}
	plain := build(0, 0)
	for _, f := range [][2]float64{{1, 0}, {0, 1}, {1, 1}} {
		got := build(f[0], f[1])
		for p, want := range plain {
			q := p
			if f[0] != 0 {
				q[0] = w - 1 - p[0]
			}
			if f[1] != 0 {
				q[1] = h - 1 - p[1]
			}
			if got[q] != want {
				t.Errorf("flip %v: (%d,%d) is %q, want the reflection of (%d,%d) %q", f, q[0], q[1], got[q], p[0], p[1], want)
			}
		}
	}
}

// The mirrored scripts without a preset build cleanly in every orientation.
func TestContentMirroredScriptsBuild(t *testing.T) {
	loreLoadData(t)
	for _, c := range []struct {
		script string
		w, h   int
		params map[string]any
	}{
		{"nest", 13, 13, map[string]any{"occupants": "dormant"}},
		{"cave_in", 13, 9, map[string]any{"tech": "industrial"}},
		{"defence_post", 13, 13, map[string]any{"occupants": "dormant", "turrets": 3.0}},
		{"hull_breach", 13, 9, map[string]any{}},
		{"circle_ruined_tower", 25, 25, map[string]any{}},
	} {
		for seed := int64(1); seed <= 8; seed++ {
			contentBuild(t, c.script, c.w, c.h, c.params, seed, "*")
		}
	}
}
