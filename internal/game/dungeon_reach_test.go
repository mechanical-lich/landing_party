package game

import (
	"math/rand"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/generation"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

// A quest lair built by underground_dungeon can be walked into: from its
// surface stair the game's own path search reaches a tile beside its boss,
// down a shaft whose every stairs_down stands over a stairs_up, into a far
// room joined to the stair room, on every seed and shaft depth tried.
func TestUndergroundDungeonWalkable(t *testing.T) {
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Lair", 3, false)
	if err != nil {
		t.Fatal(err)
	}
	ms, err := wm.buildParked(c.StartID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ms.teardown)
	lvl := ms.level
	ax, ay, _ := findStartingPlaza(lvl, lvl.SurfaceZ, 5)
	const w, h = 25, 21
	for i := 0; i < 8; i++ {
		// A fresh spot for each lair, away from the landing and the last.
		x0, y0 := ax-60+(i%4)*30, ay+30+(i/4)*30
		ctx := &setupContext{Level: lvl, rng: rand.New(rand.NewSource(int64(1000 + i)))}
		p := ctx.topParams()
		p["depth"] = float64(1 + i%3)
		generation.ClearFootprint(lvl, x0, y0, w, h)
		if err := runGenStructure(ctx, "underground_dungeon", x0, y0, w, h); err != nil {
			t.Fatal(err)
		}
		// The boss: the brute inside the footprint, at the bottom.
		bx, by, bz := -1, -1, -1
		for _, e := range lvl.Entities {
			if e == nil || e.Blueprint != "mutant_brute" || !e.HasComponent(rlcomponents.Position) {
				continue
			}
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			if pc.GetX() >= x0 && pc.GetX() < x0+w && pc.GetY() >= y0 && pc.GetY() < y0+h {
				bx, by, bz = pc.GetX(), pc.GetY(), pc.GetZ()
			}
		}
		// The surface stair: the highest stairs_down in the footprint.
		sx, sy, sz := -1, -1, -1
		for z := lvl.Depth - 1; z > bz && sx < 0; z-- {
			for y := y0; y < y0+h && sx < 0; y++ {
				for x := x0; x < x0+w; x++ {
					ti := lvl.GetTileAt(x, y, z)
					if ti == nil {
						continue
					}
					if tt := ti.(*world.Tile); !tt.Middle.IsEmpty() && world.TileDefinitions[tt.Middle.Type].StairsDown {
						sx, sy, sz = x, y, z
						break
					}
				}
			}
		}
		if bx < 0 || sx < 0 {
			t.Fatalf("lair %d: boss at %d,%d,%d, surface stair at %d,%d,%d", i, bx, by, bz, sx, sy, sz)
		}
		reached := false
		from := lvl.GetTileAt(sx, sy, sz).(*world.Tile)
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			ti := lvl.GetTileAt(bx+d[0], by+d[1], bz)
			if ti == nil || lvl.GetSolidEntityAt(bx+d[0], by+d[1], bz) != nil {
				continue
			}
			to := ti.(*world.Tile)
			if to.IsSolid() {
				continue
			}
			fspath.ResetFrameCounter()
			if len(fspath.GetPossiblePath(lvl, from, to, nil)) > 0 {
				reached = true
				break
			}
		}
		if !reached {
			t.Errorf("lair %d (depth %d): no path from the surface stair %d,%d,%d to beside the boss %d,%d,%d", i, 1+i%3, sx, sy, sz, bx, by, bz)
		}
	}
}
