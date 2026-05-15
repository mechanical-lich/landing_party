package world

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rllayered"
	"github.com/mechanical-lich/mlge/ecs"
)

// Tile is a layered cell with Floor/Middle/Ceiling slots.
type Tile = rllayered.Tile

// getPathCostFunction returns the game-specific A* cost function for the level.
func getPathCostFunction(level *Level) func(from, to *Tile) float64 {
	return PathCostFunctionForFaction(level, "")
}

// PathCostFunctionForFaction returns an A* cost function that treats doors
// owned by faction as low-cost passable rather than high-cost blockers.
func PathCostFunctionForFaction(level *Level, faction string) func(from, to *Tile) float64 {
	return PathCostFunctionForEntity(level, faction, false)
}

// PathCostFunctionForEntity returns a cost function that allows space tiles
// when vacuumResist is true. Layer-aware: rejects cells with no Floor (you
// can't stand there) and blocks based on the Middle slot.
func PathCostFunctionForEntity(level *Level, faction string, vacuumResist bool) func(from, to *Tile) float64 {
	return func(from, to *Tile) float64 {
		// Middle slot decides blocking. Empty Middle = walkable through.
		isStairTile := false
		if !to.Middle.IsEmpty() {
			midDef := TileDefinitions[to.Middle.Type]
			if midDef.Solid || midDef.Water {
				return 5000.0
			}
			if midDef.Space && !vacuumResist {
				return 5000.0
			}
			isStairTile = midDef.StairsUp || midDef.StairsDown
		}

		// Need ground to stand on — but stairs are self-supporting so skip
		// the floor check for stair tiles painted into previously-solid ground.
		if to.Floor.IsEmpty() && !isStairTile {
			return 5000.0
		}

		cost := 0.0
		_, _, fromZ := from.Coords()
		toX, toY, toZ := to.Coords()

		// Stair traversal still reads the Middle slot of `from`.
		if fromZ != toZ {
			if from.Middle.IsEmpty() {
				return 1000.0
			}
			fromDef := TileDefinitions[from.Middle.Type]
			if fromZ < toZ && !fromDef.StairsUp {
				return 1000.0
			}
			if fromZ > toZ && !fromDef.StairsDown {
				return 1000.0
			}
		}

		e := level.GetSolidEntityAt(toX, toY, toZ)
		if e != nil {
			if faction != "" && IsDoorPassableByFaction(e, faction) {
				cost += 10
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
