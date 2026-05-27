package path

import "github.com/mechanical-lich/landing_party/internal/world"

// climbingGraph allows z-transitions only when the destination tile has an
// adjacent solid Middle tile on the same z-level (i.e. there's a wall to grab).
type climbingGraph struct {
	level   *world.Level
	faction string
}

func (g *climbingGraph) PathNeighborIDs(tileIdx int, buf []int) []int {
	t := g.level.GetTilePtrIndex(tileIdx)
	x, y, z := t.Coords()
	// Z-transitions are only allowed when the source tile has an adjacent
	// solid wall — the entity grabs/climbs that wall to change elevation.
	sourceHasWall := g.hasAdjacentWall(x, y, z)
	for _, offset := range pathOffsets {
		nx, ny, nz := x+offset[0], y+offset[1], z+offset[2]
		n := g.level.GetTilePtr(nx, ny, nz)
		if n == nil {
			continue
		}
		if offset[2] != 0 && !sourceHasWall {
			continue
		}
		if n.Middle.IsEmpty() {
			buf = append(buf, n.Idx)
			continue
		}
		def := world.TileDefinitions[n.Middle.Type]
		if def.Solid || def.Space {
			continue
		}
		buf = append(buf, n.Idx)
	}
	return buf
}

func (g *climbingGraph) hasAdjacentWall(x, y, z int) bool {
	for _, off := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		adj := g.level.GetTilePtr(x+off[0], y+off[1], z)
		if adj == nil {
			continue
		}
		if !adj.Middle.IsEmpty() && world.TileDefinitions[adj.Middle.Type].Solid {
			return true
		}
	}
	return false
}

func (g *climbingGraph) PathCost(fromIdx, toIdx int) float64 {
	from := g.level.GetTilePtrIndex(fromIdx)
	to := g.level.GetTilePtrIndex(toIdx)
	if to.Middle.IsEmpty() {
		return 1.0
	}
	def := world.TileDefinitions[to.Middle.Type]
	if def.Solid || def.Space {
		return 5000.0
	}
	toX, toY, toZ := to.Coords()
	e := g.level.GetSolidEntityAt(toX, toY, toZ)
	if e != nil {
		if g.faction != "" && world.IsDoorPassableByFaction(e, g.faction) {
			return 11.0
		}
		return 1001.0
	}
	_ = from
	return 1.0
}

func (g *climbingGraph) PathEstimate(fromIdx, toIdx int) float64 {
	t1 := g.level.GetTilePtrIndex(fromIdx)
	t2 := g.level.GetTilePtrIndex(toIdx)
	x1, y1, z1 := t1.Coords()
	x2, y2, z2 := t2.Coords()
	dx, dy, dz := float64(x2-x1), float64(y2-y1), float64(z2-z1)
	return dx*dx + dy*dy + dz*dz
}
