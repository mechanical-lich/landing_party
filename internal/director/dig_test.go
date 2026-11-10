package director

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/factory"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

// digSite is testSite with two levels of rock below the surface (z 2), ore
// two levels down off to the west beyond the surface's walls, and no ore
// on the surface.
func digSite(t *testing.T) (*Site, *fakeOrders) {
	t.Helper()
	s, o := testSite(t, 2, map[string]int{"metal_ore": 6})
	old := s.Level
	lvl := world.NewLevel(41, 41, 3)
	lvl.AllocTerrain()
	for x := 0; x < 41; x++ {
		for y := 0; y < 41; y++ {
			lvl.SetFloor(x, y, 2, "hull_floor", 0)
			if x < 12 {
				lvl.SetMiddle(x, y, 2, "stone_wall", 0) // the surface ends: tunnel the rest
			}
			for z := 0; z < 2; z++ {
				lvl.SetFloor(x, y, z, "hull_floor", 0)
				lvl.SetMiddle(x, y, z, "stone_wall", 0)
			}
		}
	}
	for y := 19; y <= 21; y++ {
		lvl.SetMiddle(6, y, 0, "ore_deposit", 0)
	}
	for _, e := range old.Entities {
		x, y, _, _ := pos(e)
		e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent).SetPosition(x, y, 2)
		lvl.AddEntity(e)
	}
	s.Level, s.CZ = lvl, 2
	return s, o
}

// complete carries out every open order the way workers would: a stair is
// painted with its pair below, a dig clears the tile, another build stands.
func complete(s *Site) {
	for _, tk := range s.Tasks.GetTasks() {
		switch r := tk.Data.(type) {
		case task_requests.BuildRequest:
			if r.Type != stairsDown {
				e, err := factory.Create(r.Type, r.X, r.Y, r.Z)
				if err != nil {
					panic(err)
				}
				s.Level.AddEntity(e)
				break
			}
			s.Level.PaintTile(r.X, r.Y, r.Z, "stairs_down", 0)
			s.Level.PaintTile(r.X, r.Y, r.Z-1, "stairs_up", 0)
		case task_requests.DigRequest:
			s.Level.ClearMiddle(r.X, r.Y, r.Z)
		default:
			continue
		}
		tk.Complete()
		s.Tasks.RemoveTask(tk)
	}
}

// Short of metal with no face to mine, it digs: a Stairs Down on the
// surface, another a tile across on the level below, then a tunnel to the
// ore, and then mines it, all one order at a time. The stairs still to build
// are held back from the rungs below the dig rung.
func TestDirectorDigsToOreBelow(t *testing.T) {
	s, o := digSite(t)
	d := New(1)
	mined := false
	for pass := 0; pass < 60 && !mined; pass++ {
		d.Plan(s, pass*100)
		open := 0
		for _, tk := range s.Tasks.GetTasks() {
			switch r := tk.Data.(type) {
			case task_requests.MineRequest:
				if r.Z == 0 && yields(s.Level, r.X, r.Y, r.Z) == "metal_ore" {
					mined = true
				}
			case task_requests.DigRequest:
				open++
			case task_requests.BuildRequest:
				if r.Type == stairsDown {
					open++
				}
			}
		}
		if open > 1 {
			t.Fatalf("pass %d: %d dig/stair orders open at once", pass, open)
		}
		complete(s)
	}
	if !mined {
		t.Fatalf("never mined the ore below; orders %v\nlog %v", o.log, d.Log)
	}
	stairs := 0
	for _, l := range o.log {
		if l == "build:stairs_down" {
			stairs++
		}
	}
	if stairs != 2 {
		t.Errorf("%d stairs built for two levels, want 2 (log %v)", stairs, d.Log)
	}
	if d.Ordered["dig:tunnel"] == 0 {
		t.Errorf("no tunnel dug: %v", d.Log)
	}
	// Between the two stairs nothing else was built: the second stair (2
	// metal) was held back from the rungs below the dig rung.
	first := -1
	for i, l := range o.log {
		if l == "build:stairs_down" {
			if first >= 0 {
				return
			}
			first = i
			continue
		}
		if first >= 0 && strings.HasPrefix(l, "build:") {
			t.Errorf("%s between the stairs: %v", l, o.log)
			return
		}
	}
}

// With no ore below within dig_reach and dig_depth, it gives no dig order.
func TestDirectorDigsOnlyWithinRange(t *testing.T) {
	s, o := digSite(t)
	cfg := *s.Doctrine.Director
	cfg.DigReach = 5
	doc := *s.Doctrine
	doc.Director = &cfg
	s.Doctrine = &doc
	d := New(1)
	for pass := 0; pass < 5; pass++ {
		d.Plan(s, pass*100)
		complete(s)
	}
	for _, l := range o.log {
		if l == "build:stairs_down" || l == "dig" {
			t.Fatalf("dug toward ore out of range: %v", o.log)
		}
	}
}

// A shaft stays walkable with a worker idling on a landing: the z step
// beside the shaft (no stair under foot) costs more than any occupied tile,
// so the search does not return it (internal/world/tile.go invalidZStep).
func TestShaftPathPastWorkerOnLanding(t *testing.T) {
	s, _ := digSite(t)
	lvl := s.Level
	for x := 0; x < 41; x++ { // open the surface by the shaft
		lvl.ClearMiddle(x, 10, 2)
		lvl.ClearMiddle(x, 9, 2)
	}
	lvl.PaintTile(10, 10, 2, "stairs_down", 0)
	lvl.PaintTile(10, 10, 1, "stairs_up", 0)
	lvl.PaintTile(11, 10, 1, "stairs_down", 0)
	lvl.PaintTile(11, 10, 0, "stairs_up", 0)
	idler, err := factory.Create("colonist", 10, 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	lvl.AddEntity(idler)
	fspath.ResetFrameCounter()
	from := lvl.GetTileAt(14, 9, 2).(*world.Tile)
	to := lvl.GetTileAt(11, 10, 0).(*world.Tile)
	fspath.ResetFrameCounter()
	if p := fspath.GetPossiblePath(lvl, from, to, nil); len(p) == 0 {
		t.Fatal("no path down the shaft past the worker on its landing")
	}
}
