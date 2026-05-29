package path

import "github.com/mechanical-lich/landing_party/internal/world"

// flyingGraph wraps world.Level and allows z-transitions through non-solid,
// non-space tiles (i.e. air/empty middles), no stairs required.
type flyingGraph struct {
	level      *world.Level
	faction    string
	allowSpace bool
}

func (g *flyingGraph) PathNeighborIDs(tileIdx int, buf []int) []int {
	t := g.level.GetTilePtrIndex(tileIdx)
	x, y, z := t.Coords()
	for _, offset := range pathOffsets {
		nx, ny, nz := x+offset[0], y+offset[1], z+offset[2]
		n := g.level.GetTilePtr(nx, ny, nz)
		if n == nil {
			continue
		}
		if offset[2] != 0 {
			// Allow z-transitions at explicit stair tiles (same as base pathfinding).
			tIsStair := !t.Middle.IsEmpty() && (world.TileDefinitions[t.Middle.Type].StairsUp || world.TileDefinitions[t.Middle.Type].StairsDown)
			nIsStair := !n.Middle.IsEmpty() && (world.TileDefinitions[n.Middle.Type].StairsUp || world.TileDefinitions[n.Middle.Type].StairsDown)
			isStairTransition := tIsStair || nIsStair
			// Flying up: destination's floor blocks entry from below.
			// Flying down: source's floor blocks exit downward.
			if offset[2] > 0 && !n.Floor.IsEmpty() && !isStairTransition {
				continue
			}
			if offset[2] < 0 && !t.Floor.IsEmpty() && !isStairTransition {
				continue
			}
		}
		if n.Middle.IsEmpty() {
			buf = append(buf, n.Idx)
			continue
		}
		def := world.TileDefinitions[n.Middle.Type]
		if def.Solid {
			continue
		}
		if def.Space && !g.allowSpace {
			continue
		}
		buf = append(buf, n.Idx)
	}
	return buf
}

func (g *flyingGraph) PathCost(fromIdx, toIdx int) float64 {
	from := g.level.GetTilePtrIndex(fromIdx)
	to := g.level.GetTilePtrIndex(toIdx)
	if to.Middle.IsEmpty() {
		return 1.0
	}
	def := world.TileDefinitions[to.Middle.Type]
	if def.Solid {
		return 5000.0
	}
	if def.Space && !g.allowSpace {
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

func (g *flyingGraph) PathEstimate(fromIdx, toIdx int) float64 {
	t1 := g.level.GetTilePtrIndex(fromIdx)
	t2 := g.level.GetTilePtrIndex(toIdx)
	x1, y1, z1 := t1.Coords()
	x2, y2, z2 := t2.Coords()
	dx, dy, dz := float64(x2-x1), float64(y2-y1), float64(z2-z1)
	return dx*dx + dy*dy + dz*dz
}
