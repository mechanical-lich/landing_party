package workerai

import (
	"log"

	"github.com/mechanical-lich/landing_party/internal/components"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/skills"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlai"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
)

// MoveTowardsTarget advances entity one step along a path to (targetX,targetY,targetZ).
// Returns true if the entity moved, false if it has arrived or is blocked.
// MoveTowardsTarget advances the worker one step toward the target.
// moved reports whether it moved/swapped this tick. pathFound reports whether a
// route to the target exists; it is false ONLY when the pathfinder found no
// path (or the target tile is out of bounds) — i.e. the target is genuinely
// unreachable right now, as opposed to merely blocked by another colonist this
// tick. This lets task handlers cool an unreachable task down (Stop) instead of
// re-queueing it, so a lone worker doesn't spin on it forever. Callers that
// don't care why a move failed can ignore pathFound with `moved, _ := …`.
// footstepLoudness keeps footsteps quiet — audible near the camera but only a
// tile or two out for AI hearing.
const footstepLoudness float32 = 2

// EmitFootstep plays a footstep sound for a ground step onto (x,y,z), chosen by
// the destination floor's material. Shared by colonist, scripted-mob (zombies,
// critters) and faction-mob movement. No-op for flying/spacefaring entities —
// they don't touch the ground. The positional bridge gates it to the on-screen
// view and coalescing caps a crowd of walkers.
func EmitFootstep(level *world.Level, entity *ecs.Entity, x, y, z int) {
	if isFlying(entity) {
		return
	}
	key := components.FootstepClipKey(level.FloorMaterialAt(x, y, z))
	level.EmitSoundClip(x, y, z, footstepLoudness, world.SoundTagFootstep, key, entity)
}

func MoveTowardsTarget(level *world.Level, entity *ecs.Entity, targetX, targetY, targetZ int) (moved bool, pathFound bool) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)

	needNewPath := len(aiMemory.CurrentSteps) < 2 ||
		aiMemory.TargetX != targetX || aiMemory.TargetY != targetY || aiMemory.TargetZ != targetZ

	if !needNewPath {
		firstStep := level.Level.GetTilePtrIndex(aiMemory.CurrentSteps[1])
		fsX, fsY, fsZ := firstStep.Coords()
		if pc.GetX() != fsX || pc.GetY() != fsY || pc.GetZ() != fsZ {
			needNewPath = true
		}
	}

	if needNewPath {
		from := level.GetTileAt(pc.GetX(), pc.GetY(), pc.GetZ())
		to := level.GetTileAt(targetX, targetY, targetZ)
		if from == nil || to == nil {
			return false, false
		}
		aiMemory.CurrentSteps = fspath.GetPossiblePathForEntity(level, entity, from.(*world.Tile), to.(*world.Tile), aiMemory.CurrentSteps)
		aiMemory.TargetX = targetX
		aiMemory.TargetY = targetY
		aiMemory.TargetZ = targetZ
		if len(aiMemory.CurrentSteps) == 0 {
			log.Printf("[PATH] %s no path from (%d,%d,%d) to (%d,%d,%d)", rlentity.GetName(entity), pc.GetX(), pc.GetY(), pc.GetZ(), targetX, targetY, targetZ)
			return false, false
		}
	}

	for len(aiMemory.CurrentSteps) > 1 {
		next := level.Level.GetTilePtrIndex(aiMemory.CurrentSteps[1])
		ntX, ntY, ntZ := next.Coords()
		if pc.GetX() == ntX && pc.GetY() == ntY && pc.GetZ() == ntZ {
			aiMemory.CurrentSteps = aiMemory.CurrentSteps[1:]
			continue
		}
		if canMoveTo(level, entity, next) {
			dx, dy, dz := rlai.TrackTarget(pc.GetX(), pc.GetY(), pc.GetZ(), ntX, ntY, ntZ)
			if isFlying(entity) {
				flyMove(entity, level, dx, dy, dz)
			} else {
				rlentity.Move(entity, level, dx, dy, dz)
				EmitFootstep(level, entity, ntX, ntY, ntZ)
			}
			rlentity.Face(entity, dx, dy)
			return true, true
		}
		if tryColonistSwap(level, entity, pc, ntX, ntY, ntZ) {
			aiMemory.CurrentSteps = aiMemory.CurrentSteps[1:]
			return true, true
		}
		aiMemory.CurrentSteps = nil
		break
	}
	// A path exists (or existed) but the worker couldn't advance this tick —
	// a transient block, not a dead end.
	return false, true
}

// releaseTaskAfterFailedMove hands the worker's current task back to the queue
// after it couldn't advance toward the target. When the target was genuinely
// unreachable (pathFound == false), the task is Stop()ped so the scheduler skips
// it for a short cooldown and the worker picks other work instead of re-selecting
// the same closest-but-unreachable task every tick; a transient block
// (pathFound == true) is ReQueue()d for immediate retry.
func releaseTaskAfterFailedMove(wc *components.WorkerComponent, pathFound bool) {
	if wc.CurrentTask == nil {
		return
	}
	if pathFound {
		wc.CurrentTask.ReQueue()
	} else {
		wc.CurrentTask.Stop()
	}
	wc.CurrentTask = nil
}

func tryColonistSwap(level *world.Level, entity *ecs.Entity, pc *rlcomponents.PositionComponent, ntX, ntY, ntZ int) bool {
	blocker := level.GetSolidEntityAt(ntX, ntY, ntZ)
	if blocker == nil || blocker == entity {
		return false
	}
	if !blocker.HasComponent(components.Worker) || !blocker.HasComponent(components.Settlement) || !entity.HasComponent(components.Settlement) {
		return false
	}
	entitySC := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
	blockerSC := blocker.GetComponent(components.Settlement).(*components.SettlementComponent)
	if entitySC.Name != blockerSC.Name {
		return false
	}
	blockerWC := blocker.GetComponent(components.Worker).(*components.WorkerComponent)
	if blockerWC.SwapCooldown > 0 {
		return false
	}
	myX, myY, myZ := pc.GetX(), pc.GetY(), pc.GetZ()
	level.PlaceEntity(ntX, ntY, ntZ, entity)
	level.PlaceEntity(myX, myY, myZ, blocker)
	blockerWC.SwapCooldown = 3
	if blocker.HasComponent(rlcomponents.AIMemory) {
		blockerAI := blocker.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
		blockerAI.CurrentSteps = nil
	}
	rlentity.Face(entity, ntX-myX, ntY-myY)
	return true
}

func isFlying(entity *ecs.Entity) bool {
	return skills.Has(entity, fspath.FlyingSkill) || skills.Has(entity, fspath.SpacefaringSkill) || skills.Has(entity, fspath.ClimbingSkill)
}

func canMoveTo(level *world.Level, entity *ecs.Entity, tile *world.Tile) bool {
	if tile == nil {
		return false
	}
	flying := isFlying(entity)
	// Layered walkability: need ground (Floor non-empty) unless flying.
	// Stair tiles are self-supporting so they don't require a floor slot,
	// mirroring the exemption in PathCostFunctionForEntity.
	if tile.Floor.IsEmpty() && !flying {
		isStair := !tile.Middle.IsEmpty() &&
			(world.TileDefinitions[tile.Middle.Type].StairsUp || world.TileDefinitions[tile.Middle.Type].StairsDown)
		if !isStair {
			return false
		}
	}
	if !tile.Middle.IsEmpty() {
		def := world.TileDefinitions[tile.Middle.Type]
		if def.Solid || def.Water {
			return false
		}
		if def.Space && !skills.Has(entity, fspath.VacuumResistSkill) && !skills.Has(entity, fspath.SpacefaringSkill) {
			return false
		}
	}
	tX, tY, tZ := tile.Coords()
	solid := level.GetSolidEntityAt(tX, tY, tZ)
	return solid == nil || solid == entity
}

// flyMove moves a flying entity directly to a destination tile, bypassing
// the rlentity.Move floor/air checks which block movement through empty space.
func flyMove(entity *ecs.Entity, level *world.Level, dx, dy, dz int) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	destX := pc.GetX() + dx
	destY := pc.GetY() + dy
	destZ := pc.GetZ() + dz
	tile, _ := level.GetTileAt(destX, destY, destZ).(*world.Tile)
	if tile == nil {
		return
	}
	if !tile.Middle.IsEmpty() && world.TileDefinitions[tile.Middle.Type].Solid {
		return
	}
	solid := level.GetSolidEntityAt(destX, destY, destZ)
	if solid != nil && solid != entity {
		return
	}
	level.PlaceEntity(destX, destY, destZ, entity)
}
