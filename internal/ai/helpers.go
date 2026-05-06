package ai

import (
	"fmt"

	fspath "github.com/mechanical-lich/scifi_settlements/internal/path"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlai"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/task"
	"github.com/mechanical-lich/mlge/utility"
)

func CompleteTaskWithMessage(entity *ecs.Entity, t *task.Task, msg string) {
	t.Complete()
	wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)
	wc.CurrentTask = nil
	message.PostMessage(rlentity.GetName(entity), msg)
}

func FindAvailableStorage(level *world.Level, settlementName string) *ecs.Entity {
	for _, e := range level.Entities {
		if e.HasComponent(components.Storage) {
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			if sc.OwnedBy == settlementName {
				return e
			}
		}
	}
	return nil
}

func FindClosestStorageWith(level *world.Level, settlementName, itemName string, x, y, z int) *ecs.Entity {
	var closest *ecs.Entity
	minDist := 99999
	for _, e := range level.Entities {
		if !e.HasComponent(components.Storage) {
			continue
		}
		sc := e.GetComponent(components.Storage).(*components.StorageComponent)
		if sc.OwnedBy == settlementName && sc.HasItem(itemName) {
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			d := utility.Distance(pc.GetX(), pc.GetY(), x, y)
			if closest == nil || d < minDist {
				closest = e
				minDist = d
			}
		}
	}
	return closest
}

func MoveTowardsTarget(level *world.Level, entity *ecs.Entity, targetX, targetY, targetZ int) bool {
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
			return false
		}
		aiMemory.CurrentSteps = fspath.GetPossiblePath(level, from.(*world.Tile), to.(*world.Tile), aiMemory.CurrentSteps)
		aiMemory.TargetX = targetX
		aiMemory.TargetY = targetY
		aiMemory.TargetZ = targetZ
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
			rlentity.Move(entity, level, dx, dy, dz)
			rlentity.Face(entity, dx, dy)
			return true
		}
		aiMemory.CurrentSteps = nil
		break
	}
	return false
}

func canMoveTo(level *world.Level, entity *ecs.Entity, tile *world.Tile) bool {
	if tile == nil {
		return false
	}
	def := world.TileDefinitions[tile.Type]
	if def.Solid || def.Water || def.Space {
		return false
	}
	tX, tY, tZ := tile.Coords()
	solid := level.GetSolidEntityAt(tX, tY, tZ)
	return solid == nil || solid == entity
}

func PickupItemFromTile(level *world.Level, entity *ecs.Entity, x, y, z int) {
	tileEntity := level.GetEntityAt(x, y, z)
	if tileEntity != nil && tileEntity.HasComponent(rlcomponents.Item) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		inv.AddItem(tileEntity)
		level.RemoveEntity(tileEntity)
		message.PostMessage(rlentity.GetName(entity), fmt.Sprintf("Picked up %s", tileEntity.Blueprint))
	}
}
