package game

import "testing"

// The reactor complex is clean and walkable: no wall posts or doubled walls
// (the wings used to overrun the core room and leave stubs), and from the core
// tile, where the overseer stands and a history datapad lands, a colonist can
// walk out of the complex.
func TestContentReactorCoreReachable(t *testing.T) {
	loreLoadData(t)
	const x0, y0, w, h = 4, 4, 21, 19
	for seed := int64(1); seed <= 8; seed++ {
		level := loreFlatLevel(t, 30, 28)
		if err := RunStructureScriptWithParams(level, "abandoned_reactor", x0, y0, w, h, map[string]any{"occupants": "none"}, seed); err != nil {
			t.Fatal(err)
		}
		cx, cy := x0+w/2, y0+h/2
		z := level.GetSurfaceZ(cx, cy)
		if posts, where := lorePosts(level, "hull_wall", x0, y0, w, h, z); posts > 0 {
			t.Errorf("seed %d: %d wall faults at %v", seed, posts, where)
		}
		open := func(x, y int) bool {
			tl := level.GetTilePtr(x, y, z)
			return tl != nil && !tl.IsSolid()
		}
		seen := map[[2]int]bool{{cx, cy}: true}
		queue := [][2]int{{cx, cy}}
		out := false
		for len(queue) > 0 && !out {
			p := queue[0]
			queue = queue[1:]
			for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				q := [2]int{p[0] + d[0], p[1] + d[1]}
				if seen[q] || !open(q[0], q[1]) {
					continue
				}
				if q[0] < x0 || q[0] >= x0+w || q[1] < y0 || q[1] >= y0+h {
					out = true
				}
				seen[q] = true
				queue = append(queue, q)
			}
		}
		if !out && seed == 1 {
			for y := y0 - 1; y <= y0+h; y++ {
				row := ""
				for x := x0 - 1; x <= x0+w; x++ {
					switch {
					case seen[[2]int{x, y}]:
						row += "*"
					case open(x, y):
						row += "."
					default:
						row += "#"
					}
				}
				t.Log(row)
			}
		}
		if !open(cx, cy) || !out {
			t.Errorf("seed %d: the core at (%d,%d) is sealed in (reached %d tiles)", seed, cx, cy, len(seen))
		}
	}
}
