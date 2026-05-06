package world

import "github.com/mechanical-lich/ml-rogue-lib/pkg/rlworld"

// Tile is a type alias for the base rlworld.Tile.
type Tile = rlworld.Tile

// pathCostFunction returns the game-specific A* cost function for the level.
func getPathCostFunction(level *Level) func(from, to *rlworld.Tile) float64 {
	return func(from, to *rlworld.Tile) float64 {
		tileDef := TileDefinitions[to.Type]

		if tileDef.Solid || tileDef.Water || tileDef.Space {
			return 5000.0
		}

		cost := 0.0
		_, _, fromZ := from.Coords()
		_, _, toZ := to.Coords()

		if fromZ < toZ {
			if !TileDefinitions[from.Type].StairsUp {
				return 1000.0
			}
		} else if fromZ > toZ {
			if !TileDefinitions[from.Type].StairsDown {
				return 1000.0
			}
		}

		toX, toY, _ := to.Coords()
		e := level.GetSolidEntityAt(toX, toY, toZ)
		if e != nil {
			cost += 1000
		}

		return cost
	}
}
