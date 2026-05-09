package world

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlworld"
	"github.com/mechanical-lich/mlge/ecs"
)

// Tile is a type alias for the base rlworld.Tile.
type Tile = rlworld.Tile

// pathCostFunction returns the game-specific A* cost function for the level.
func getPathCostFunction(level *Level) func(from, to *rlworld.Tile) float64 {
	return PathCostFunctionForFaction(level, "")
}

// pathCostFunctionForFaction returns an A* cost function that treats doors
// owned by faction as low-cost passable rather than high-cost blockers.
func PathCostFunctionForFaction(level *Level, faction string) func(from, to *rlworld.Tile) float64 {
	return PathCostFunctionForEntity(level, faction, false)
}

// PathCostFunctionForEntity returns a cost function that allows space tiles
// when vacuumResist is true (entity has the vacuum_resist skill, e.g. enviro
// suit equipped). Solid and water remain blocked.
func PathCostFunctionForEntity(level *Level, faction string, vacuumResist bool) func(from, to *rlworld.Tile) float64 {
	return func(from, to *rlworld.Tile) float64 {
		tileDef := TileDefinitions[to.Type]

		if tileDef.Solid || tileDef.Water {
			return 5000.0
		}
		if tileDef.Space && !vacuumResist {
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
			if faction != "" && IsDoorPassableByFaction(e, faction) {
				cost += 10 // small cost to prefer open paths but still route through
			} else {
				cost += 1000
			}
		}

		return cost
	}
}

// IsDoorPassableByFaction returns true if the entity is a door that allows the given faction.
func IsDoorPassableByFaction(e *ecs.Entity, faction string) bool {
	if !e.HasComponent(rlcomponents.Door) {
		return false
	}
	door := e.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent)
	if door.Locked {
		return false
	}
	return door.OwnedBy != "" && door.OwnedBy == faction
}
