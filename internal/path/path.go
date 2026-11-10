package path

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/skills"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/path"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// VacuumResistSkill grants the ability to traverse space tiles. Granted by
// equipping a full enviro outfit.
const VacuumResistSkill = "vacuum_resist"

const (
	FlyingSkill      = "flying"      // 3D movement through air tiles, no stair requirement
	SpacefaringSkill = "spacefaring" // 3D movement through air or space tiles
	ClimbingSkill    = "climbing"    // 3D movement only when adjacent solid wall exists
	BurrowingSkill   = "burrowing"   // 3D movement through solid underground tiles
)

// pathOffsets mirrors the six cardinal directions used in rllayered.
var pathOffsets = [6][3]int{
	{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1},
}

var (
	pathRequestsThisFrame   = 0
	maxPathRequestsPerFrame = 10

	pathfinder = path.NewAStar(2048)
)

func ResetFrameCounter() {
	pathRequestsThisFrame = 0
}

func GetPossiblePathForEntity(level *world.Level, entity *ecs.Entity, fromTile, toTile *world.Tile, reuse []int) []int {
	faction := ""
	if entity.HasComponent(rlcomponents.Description) {
		dc := entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
		faction = dc.Faction
	}

	if skills.Has(entity, SpacefaringSkill) {
		return getCustomPath(level, &flyingGraph{level: level, faction: faction, allowSpace: true}, fromTile, toTile, reuse)
	}
	if skills.Has(entity, FlyingSkill) {
		return getCustomPath(level, &flyingGraph{level: level, faction: faction, allowSpace: false}, fromTile, toTile, reuse)
	}
	if skills.Has(entity, ClimbingSkill) {
		return getCustomPath(level, &climbingGraph{level: level, faction: faction}, fromTile, toTile, reuse)
	}
	if skills.Has(entity, BurrowingSkill) {
		return getBurrowingPath(level, &burrowingGraph{level: level, faction: faction}, fromTile, toTile, reuse)
	}

	vacuum := skills.Has(entity, VacuumResistSkill)
	return getPossiblePath(level, faction, vacuum, fromTile, toTile, reuse)
}

func GetPossiblePath(level *world.Level, fromTile, toTile *world.Tile, reuse []int) []int {
	return getPossiblePath(level, "", false, fromTile, toTile, reuse)
}

// getCustomPath runs A* with a caller-supplied graph (flying, spacefaring, climbing).
// Skips the stair/floating validation used by getPossiblePath.
func getCustomPath(level *world.Level, graph path.Graph, fromTile, toTile *world.Tile, reuse []int) []int {
	if pathRequestsThisFrame >= maxPathRequestsPerFrame {
		return nil
	}
	pathRequestsThisFrame++

	toX, toY, toZ := toTile.Coords()
	fromX, fromY, _ := fromTile.Coords()

	for _, offset := range [][]int{{0, 0, 0}, {-1, 0, 0}, {1, 0, 0}, {0, -1, 0}, {0, 1, 0}} {
		nx, ny, nz := toX+offset[0], toY+offset[1], toZ+offset[2]
		fx, fy := fromX+offset[0], fromY+offset[1]
		if fx < 0 || fx >= level.Width || fy < 0 || fy >= level.Height {
			continue
		}
		if nx < 0 || nx >= level.Width || ny < 0 || ny >= level.Height || nz < 0 || nz >= level.Depth {
			continue
		}
		targetTile := level.GetTileAt(nx, ny, nz)
		if targetTile == nil || targetTile.IsSolid() {
			continue
		}

		steps, _, found := pathfinder.Path(graph, fromTile.Idx, targetTile.(*world.Tile).Idx)
		if !found {
			continue
		}

		var result []int
		if cap(reuse) >= len(steps) {
			result = reuse[:len(steps)]
		} else {
			result = make([]int, len(steps))
		}
		copy(result, steps)
		return result
	}
	return nil
}

// getBurrowingPath is like getCustomPath but also searches z-adjacent tiles
// for the target, allowing a burrowing entity underground to path toward a
// surface entity.
func getBurrowingPath(level *world.Level, graph path.Graph, fromTile, toTile *world.Tile, reuse []int) []int {
	if pathRequestsThisFrame >= maxPathRequestsPerFrame {
		return nil
	}
	pathRequestsThisFrame++

	toX, toY, toZ := toTile.Coords()
	fromX, fromY, _ := fromTile.Coords()

	offsets := [][]int{{0, 0, 0}, {-1, 0, 0}, {1, 0, 0}, {0, -1, 0}, {0, 1, 0}, {0, 0, 1}, {0, 0, -1}}
	for _, offset := range offsets {
		nx, ny, nz := toX+offset[0], toY+offset[1], toZ+offset[2]
		fx, fy := fromX+offset[0], fromY+offset[1]
		if fx < 0 || fx >= level.Width || fy < 0 || fy >= level.Height {
			continue
		}
		if nx < 0 || nx >= level.Width || ny < 0 || ny >= level.Height || nz < 0 || nz >= level.Depth {
			continue
		}
		targetTile := level.GetTileAt(nx, ny, nz)
		if targetTile == nil {
			continue
		}

		steps, _, found := pathfinder.Path(graph, fromTile.Idx, targetTile.(*world.Tile).Idx)
		if !found {
			continue
		}

		var result []int
		if cap(reuse) >= len(steps) {
			result = reuse[:len(steps)]
		} else {
			result = make([]int, len(steps))
		}
		copy(result, steps)
		return result
	}
	return nil
}

func getPossiblePath(level *world.Level, faction string, vacuumResist bool, fromTile, toTile *world.Tile, reuse []int) []int {
	if pathRequestsThisFrame >= maxPathRequestsPerFrame {
		return nil
	}
	pathRequestsThisFrame++

	// Use entity-aware cost function: faction-aware doors + optional space
	// traversal for vacuum-resistant entities (enviro suit equipped).
	level.PathCostFunc = world.PathCostFunctionForEntity(level, faction, vacuumResist)

	toX, toY, toZ := toTile.Coords()
	fromX, fromY, _ := fromTile.Coords()

	for _, offset := range [][]int{{0, 0, 0}, {-1, 0, 0}, {1, 0, 0}, {0, -1, 0}, {0, 1, 0}} {
		nx, ny, nz := toX+offset[0], toY+offset[1], toZ+offset[2]
		fx, fy := fromX+offset[0], fromY+offset[1]
		if fx < 0 || fx >= level.Width || fy < 0 || fy >= level.Height {
			continue
		}
		if nx < 0 || nx >= level.Width || ny < 0 || ny >= level.Height || nz < 0 || nz >= level.Depth {
			continue
		}
		targetTile := level.GetTileAt(nx, ny, nz)
		if targetTile == nil || targetTile.IsSolid() {
			continue
		}

		steps, _, found := pathfinder.Path(level.Level, fromTile.Idx, targetTile.(*world.Tile).Idx)
		if !found {
			continue
		}
		if crossesSolid(level, steps) {
			// The level's heuristic (squared distance) outweighs a wall's cost
			// over a long walk, so the search went through one: a worker
			// cannot follow that. Search again with a step cost and a
			// heuristic that agree (phase D: a raid party walking out of its
			// camp to a colony 300 tiles away stood at its wall).
			if again, _, ok := pathfinder.Path(steadyGraph{level}, fromTile.Idx, targetTile.(*world.Tile).Idx); ok {
				steps = again
			}
		}

		// Validate: correct stair use, no floating, no solid steps
		_, _, fromZ := fromTile.Coords()
		zDiff := toZ - fromZ
		if zDiff < 0 {
			zDiff = -zDiff
		}
		zDiff *= 2 // each z transition needs an up and a down stair tile

		stairCount, floating, blocked := 0, false, false
		for i, stepID := range steps {
			st := level.Level.GetTilePtrIndex(stepID)
			// Stair semantics live on the Middle slot. Missing Middle = the
			// cell is air/empty and counts as a non-stair.
			var midDef world.TileDefinition
			if !st.Middle.IsEmpty() {
				midDef = world.TileDefinitions[st.Middle.Type]
			}
			if midDef.StairsUp || midDef.StairsDown {
				stairCount++
			}
			if (st.Middle.IsEmpty() || midDef.Air || midDef.Space) && !vacuumResist {
				// Layered model: a cell is "floating" if it has no Floor
				// (nothing to stand on).
				if st.Floor.IsEmpty() {
					floating = true
				}
			}
			if i != 0 && i != len(steps)-1 {
				sx, sy, sz := st.Coords()
				e := level.GetSolidEntityAt(sx, sy, sz)
				if e != nil && !world.IsDoorPassableByFaction(e, faction) && !e.HasComponent(components.Worker) {
					blocked = true
				}
			}
		}
		if zDiff > stairCount || floating || blocked {
			continue
		}

		var result []int
		if cap(reuse) >= len(steps) {
			result = reuse[:len(steps)]
		} else {
			result = make([]int, len(steps))
		}
		copy(result, steps)
		return result
	}
	return nil
}

// crossesSolid reports whether a path steps through a solid tile on the way
// (the ends may be solid: a mining target, a worker's own wall tile).
func crossesSolid(level *world.Level, steps []int) bool {
	for i := 1; i < len(steps)-1; i++ {
		st := level.Level.GetTilePtrIndex(steps[i])
		if !st.Middle.IsEmpty() && world.TileDefinitions[st.Middle.Type].Solid {
			return true
		}
	}
	return false
}

// steadyGraph is a level's graph with a step cost of 1 on top of the
// level's costs and a Manhattan heuristic, which never overestimates: a
// true shortest path, found by exploring more of the map than the level's
// own greedy search. Used only when that search crossed a wall.
type steadyGraph struct{ level *world.Level }

func (g steadyGraph) PathNeighborIDs(id int, buf []int) []int {
	return g.level.Level.PathNeighborIDs(id, buf)
}
func (g steadyGraph) PathCost(from, to int) float64 { return 1 + g.level.Level.PathCost(from, to) }
func (g steadyGraph) PathEstimate(from, to int) float64 {
	x1, y1, z1 := g.level.Level.GetTilePtrIndex(from).Coords()
	x2, y2, z2 := g.level.Level.GetTilePtrIndex(to).Coords()
	return float64(absInt(x2-x1) + absInt(y2-y1) + absInt(z2-z1))
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
