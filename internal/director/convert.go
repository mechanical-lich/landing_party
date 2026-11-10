package director

import (
	"fmt"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
)

// convert is the conversion economy's rung (phase C2, ConvertAction): while
// fewer than the rung wants of the ground within Radius is its tile, it
// orders ConvertActions on the reachable tiles nearest the centre, at most
// MaxMines open at once. The autonz will run it (phase E); no doctrine does
// yet.
func (d *Director) convert(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, p campaign.DirectorPriority, tick int) {
	want := p.Want(len(snap.colonists))
	have := 0
	type cand struct{ x, y, dist int }
	var cands []cand
	origins := [][2]int{{s.CX, s.CY}}
	if len(p.ConvertOrigins) > 0 {
		// The autonz' converter nodes: the wire grows out from each too.
		for _, list := range [][]*ecs.Entity{s.Level.Entities, s.Level.StaticEntities} {
			for _, e := range list {
				if e == nil || !has(p.ConvertOrigins, e.Blueprint) {
					continue
				}
				if x, y, z, ok := pos(e); ok && z == s.CZ && cheb(x-s.CX, y-s.CY) <= cfg.Radius+2 {
					origins = append(origins, [2]int{x, y})
				}
			}
		}
	}
	for k := range snap.reach {
		if cheb(k[0]-s.CX, k[1]-s.CY) > cfg.Radius {
			continue
		}
		dist := 1 << 30
		for _, o := range origins {
			dist = min(dist, cheb(k[0]-o[0], k[1]-o[1]))
		}
		switch floorName(s.Level, k[0], k[1], s.CZ) {
		case p.Convert:
			have++
		case "":
		default:
			if !snap.taskAt[[3]int{k[0], k[1], s.CZ}] && !d.avoid[[3]int{k[0], k[1], s.CZ}] {
				cands = append(cands, cand{k[0], k[1], dist})
			}
		}
	}
	if p.AtCap && s.CrewCap > 0 && len(snap.colonists) >= s.CrewCap {
		// At its conversion-capped crew (the autonz) a settlement lays wire
		// to raise the cap: the want never holds it.
		want = max(want, have+snap.converts+1)
	}
	if have+snap.converts >= want {
		return
	}
	if len(cands) == 0 && snap.converts == 0 {
		snap.noConvert = true // nothing left to lay it on within Radius
	}
	sort.Slice(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.dist != b.dist {
			return a.dist < b.dist
		}
		return a.y < b.y || a.y == b.y && a.x < b.x
	})
	for _, c := range cands {
		if have+snap.converts >= want || snap.converts >= max(cfg.MaxMines, 1) {
			return
		}
		before := len(s.Tasks.GetTasks())
		if !s.Orders.Convert(c.x, c.y, s.CZ, p.Convert) {
			d.avoid[[3]int{c.x, c.y, s.CZ}] = true
			continue
		}
		snap.converts++
		snap.taskAt[[3]int{c.x, c.y, s.CZ}] = true
		d.watchNew(s, before, tick)
		d.note("convert:"+p.Convert, fmt.Sprintf("convert %d,%d to %s", c.x, c.y, p.Convert), tick)
	}
}

// floorName is the floor tile's name at (x, y, z), "" for none.
func floorName(lvl *world.Level, x, y, z int) string {
	ti := lvl.GetTileAt(x, y, z)
	if ti == nil {
		return ""
	}
	t := ti.(*world.Tile)
	if t.Floor.IsEmpty() {
		return ""
	}
	return world.TileDefinitions[t.Floor.Type].Name
}
