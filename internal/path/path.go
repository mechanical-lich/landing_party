package path

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/path"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/scifi_settlements/internal/skills"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

// VacuumResistSkill grants the ability to traverse space tiles. Granted by
// equipping a full enviro outfit.
const VacuumResistSkill = "vacuum_resist"

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
	vacuum := skills.Has(entity, VacuumResistSkill)
	return getPossiblePath(level, faction, vacuum, fromTile, toTile, reuse)
}

func GetPossiblePath(level *world.Level, fromTile, toTile *world.Tile, reuse []int) []int {
	return getPossiblePath(level, "", false, fromTile, toTile, reuse)
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
			def := world.TileDefinitions[st.Type]
			if def.StairsUp || def.StairsDown {
				stairCount++
			}
			if (def.Air || def.Space) && !vacuumResist {
				sx, sy, sz := st.Coords()
				below := level.GetTileAt(sx, sy, sz-1)
				if below == nil || !below.IsSolid() {
					floating = true
				}
			}
			if i != 0 && i != len(steps)-1 {
				sx, sy, sz := st.Coords()
				e := level.GetSolidEntityAt(sx, sy, sz)
				if e != nil && !world.IsDoorPassableByFaction(e, faction) {
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
