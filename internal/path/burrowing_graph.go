package path

import "github.com/mechanical-lich/landing_party/internal/world"

// burrowingGraph allows movement through solid underground tiles (rock, dirt,
// ice) and onto surface tiles. Cannot enter open air or space tiles.
type burrowingGraph struct {
	level   *world.Level
	faction string
}

func (g *burrowingGraph) PathNeighborIDs(tileIdx int, buf []int) []int {
	t := g.level.GetTilePtrIndex(tileIdx)
	x, y, z := t.Coords()
	for _, offset := range pathOffsets {
		nx, ny, nz := x+offset[0], y+offset[1], z+offset[2]
		n := g.level.GetTilePtr(nx, ny, nz)
		if n == nil {
			continue
		}
		tk := g.level.GetTerrainKind(nx, ny, nz)
		// Block space, atmosphere, and void. Atmosphere is open air above the
		// surface — allowing it would let burrowers fly through the sky once
		// they broke surface, which contradicts underground-only movement.
		if tk == world.TKSpace || tk == world.TKAtmosphere || tk == world.TKVoid {
			continue
		}
		buf = append(buf, n.Idx)
	}
	return buf
}

func (g *burrowingGraph) PathCost(fromIdx, toIdx int) float64 {
	to := g.level.GetTilePtrIndex(toIdx)
	toX, toY, toZ := to.Coords()
	e := g.level.GetSolidEntityAt(toX, toY, toZ)
	if e != nil {
		if g.faction != "" && world.IsDoorPassableByFaction(e, g.faction) {
			return 11.0
		}
		return 1001.0
	}
	return 1.0
}

func (g *burrowingGraph) PathEstimate(fromIdx, toIdx int) float64 {
	t1 := g.level.GetTilePtrIndex(fromIdx)
	t2 := g.level.GetTilePtrIndex(toIdx)
	x1, y1, z1 := t1.Coords()
	x2, y2, z2 := t2.Coords()
	dx, dy, dz := float64(x2-x1), float64(y2-y1), float64(z2-z1)
	return dx*dx + dy*dy + dz*dz
}
