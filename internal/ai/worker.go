package ai

import (
	"log"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlai"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcombat"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/task"
	"github.com/mechanical-lich/mlge/utility"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/construction"
	"github.com/mechanical-lich/scifi_settlements/internal/eventsystem"
	"github.com/mechanical-lich/scifi_settlements/internal/factory"
	"github.com/mechanical-lich/scifi_settlements/internal/research"
	"github.com/mechanical-lich/scifi_settlements/internal/settlement"
	"github.com/mechanical-lich/scifi_settlements/internal/task_requests"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

func HandleWorkerIdleState(level *world.Level, entity *ecs.Entity) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)

	if !entity.HasComponents(components.Settlement, components.Worker) {
		return
	}

	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
	wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)

	if mySettlement, ok := settlement.Settlements[sc.Name]; ok {
		t := mySettlement.Tasks.GetClosestNextTask(pc.GetX(), pc.GetY(), pc.GetZ())
		if t != nil {
			aiMemory.State = "task"
			wc.CurrentTask = t
			return
		}

		// Drop off inventory if we're holding anything
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		if len(inv.Bag) > 0 {
			storage := FindAvailableStorage(level, sc.Name)
			if storage != nil {
				storagePC := storage.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
				aiMemory.TargetX = storagePC.GetX()
				aiMemory.TargetY = storagePC.GetY()
				aiMemory.TargetZ = storagePC.GetZ()
				aiMemory.State = "dropoff"
			}
		}
	}
}

func HandleTaskState(level *world.Level, entity *ecs.Entity) {
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)

	if !entity.HasComponent(components.Settlement) {
		aiMemory.State = "idle"
		return
	}

	if wc.CurrentTask == nil || wc.CurrentTask.Completed {
		aiMemory.State = "idle"
		wc.CurrentTask = nil
		return
	}

	switch wc.CurrentTask.Action {
	case task_requests.BuildAction:
		handleBuildTask(level, entity, wc, aiMemory)
	case task_requests.DigAction:
		handleDigTask(level, entity, wc, aiMemory)
	case task_requests.MineAction:
		handleMineTask(level, entity, wc, aiMemory)
	case task_requests.PickupAction:
		handlePickupTask(level, entity, wc, aiMemory)
	case task_requests.AttackAction:
		handleAttackTask(level, entity, wc, aiMemory)
	case task_requests.ResearchAction:
		handleResearchTask(level, entity, wc, aiMemory)
	default:
		handleMoveTask(level, entity, wc, aiMemory)
	}
}

func HandleDropOffState(level *world.Level, entity *ecs.Entity) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)

	if MoveTowardsTarget(level, entity, aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ) {
		return
	}

	storageEntity := level.GetEntityAt(aiMemory.TargetX, aiMemory.TargetY, pc.GetZ())
	if storageEntity != nil && storageEntity.HasComponent(components.Storage) {
		storageC := storageEntity.GetComponent(components.Storage).(*components.StorageComponent)
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for _, item := range inv.Bag {
			if item.HasComponent(rlcomponents.Item) {
				storageC.AddItem(item)
				inv.RemoveItem(item)
				dc := item.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
				event.GetQueuedInstance().QueueEvent(eventsystem.ItemStoredEvent{
					ItemName:   dc.Name,
					Blueprint:  item.Blueprint,
					Settlement: storageC.OwnedBy,
				})
			}
		}
	}
	aiMemory.State = "idle"
}

func HandleGatherMaterialsState(level *world.Level, entity *ecs.Entity) {
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)

	if wc.CurrentTask == nil {
		aiMemory.State = "idle"
		return
	}

	buildRequest, ok := wc.CurrentTask.Data.(task_requests.BuildRequest)
	if !ok {
		aiMemory.State = "idle"
		return
	}

	buildable := construction.GetBuildable(buildRequest.Type)
	if buildable.Name == "" {
		aiMemory.State = "idle"
		return
	}

	// Find the first missing material
	searching := ""
	for name, cost := range buildable.Cost {
		count := 0
		for _, item := range inv.Bag {
			if item.Blueprint == name {
				count++
			}
		}
		if count < cost {
			searching = name
			break
		}
	}

	if searching == "" {
		aiMemory.State = "task"
		return
	}

	storageEntity := FindClosestStorageWith(level, sc.Name, searching, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z)
	if storageEntity == nil {
		wc.CurrentTask.Stop()
		wc.CurrentTask = nil
		message.PostMessage(rlentity.GetName(entity), "Insufficient materials to build "+buildRequest.Type)
		aiMemory.State = "idle"
		return
	}

	storagePC := storageEntity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)

	if !MoveTowardsTarget(level, entity, storagePC.GetX(), storagePC.GetY(), storagePC.GetZ()) {
		if pc.GetX() == storagePC.GetX() && pc.GetY() == storagePC.GetY() {
			storageC := storageEntity.GetComponent(components.Storage).(*components.StorageComponent)
			material := storageC.TakeOne(searching)
			if material != nil {
				inv.AddItem(material)
			}
		} else {
			wc.CurrentTask.Stop()
			wc.CurrentTask = nil
			aiMemory.State = "idle"
		}
	}
}

func handleBuildTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	buildRequest, ok := wc.CurrentTask.Data.(task_requests.BuildRequest)
	if !ok {
		aiMemory.State = "idle"
		return
	}

	buildable := construction.GetBuildable(buildRequest.Type)
	if !checkInventoryForMaterials(entity, buildable) {
		aiMemory.State = "gather_materials"
		return
	}

	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z, 1, 1, 0) {
		// Progress lives in the request data so multiple workers share the same counter
		buildRequest.Progress++
		wc.CurrentTask.Data = buildRequest
		if buildRequest.Progress >= buildRequest.Required {
			sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
			tile := level.GetTileAt(buildRequest.X, buildRequest.Y, buildRequest.Z).(*world.Tile)
			if buildable.IsEntity {
				newEntity, err := factory.Create(buildRequest.Type, buildRequest.X, buildRequest.Y, buildRequest.Z)
				if err == nil {
					if newEntity.HasComponent(components.Storage) {
						storageC := newEntity.GetComponent(components.Storage).(*components.StorageComponent)
						storageC.OwnedBy = sc.Name
					}
					level.AddEntity(newEntity)
				}
			} else {
				tile.Type = world.TileNameToIndex[buildRequest.Type]
				tile.Variant = utility.GetRandom(0, len(world.TileDefinitions[tile.Type].Variants))
				level.InvalidateSunColumn(buildRequest.X, buildRequest.Y)
			}
			removeMaterialsFromInventory(entity, buildable)
			event.GetQueuedInstance().QueueEvent(eventsystem.StructureBuiltEvent{
				Type: buildRequest.Type, Settlement: sc.Name,
				X: buildRequest.X, Y: buildRequest.Y, Z: buildRequest.Z,
			})
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Built "+buildRequest.Type)
			aiMemory.State = "idle"
		}
	} else {
		if !MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z) {
			wc.CurrentTask.Stop()
			wc.CurrentTask = nil
		}
	}
}

func handleDigTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	req, ok := wc.CurrentTask.Data.(task_requests.DigRequest)
	if !ok {
		aiMemory.State = "idle"
		return
	}

	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z, 1, 1, 0) {
		req.Progress++
		wc.CurrentTask.Data = req
		if req.Progress >= req.Required {
			tile := level.GetTileAt(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z).(*world.Tile)
			tile.Type = world.TileNameToIndex["regolith"]
			tile.Variant = world.RandomTileVariant("regolith")
			level.InvalidateSunColumn(wc.CurrentTask.X, wc.CurrentTask.Y)
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Dug out tile")
			aiMemory.State = "idle"
		}
	} else {
		if !MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z) {
			wc.CurrentTask.Stop()
			wc.CurrentTask = nil
		}
	}
}

func handleMineTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	req, ok := wc.CurrentTask.Data.(task_requests.MineRequest)
	if !ok {
		aiMemory.State = "idle"
		return
	}

	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z, 1, 1, 0) {
		req.Progress++
		wc.CurrentTask.Data = req

		// Yield ore every 10 progress ticks so partial work isn't wasted
		if req.Progress%10 == 0 {
			tile := level.GetTileAt(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z).(*world.Tile)
			tileName := world.TileDefinitions[tile.Type].Name
			var dropBlueprint string
			switch tileName {
			case "ore_deposit":
				dropBlueprint = "metal_ore"
			case "crystal_vein":
				dropBlueprint = "crystal"
			}
			if dropBlueprint != "" {
				ore, err := factory.Create(dropBlueprint, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z)
				if err == nil {
					inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
					inv.AddItem(ore)
				}
			}
		}

		if req.Progress >= req.Required {
			tile := level.GetTileAt(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z).(*world.Tile)
			tile.Type = world.TileNameToIndex["rock"]
			tile.Variant = world.RandomTileVariant("rock")
			level.InvalidateSunColumn(wc.CurrentTask.X, wc.CurrentTask.Y)
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Mined out deposit")
			aiMemory.State = "idle"
		}
	} else {
		if !MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z) {
			wc.CurrentTask.Stop()
			wc.CurrentTask = nil
		}
	}
}

func handlePickupTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if !MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z) {
		if pc.GetX() == wc.CurrentTask.X && pc.GetY() == wc.CurrentTask.Y {
			PickupItemFromTile(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, pc.GetZ())
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Fetched item")
			aiMemory.State = "idle"
		} else {
			wc.CurrentTask.Stop()
			wc.CurrentTask = nil
		}
	}
}

func handleAttackTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	target, ok := wc.CurrentTask.Data.(*ecs.Entity)
	if !ok || target == nil || target.HasComponent(rlcomponents.Dead) {
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
		return
	}

	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	targetPC := target.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)

	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), targetPC.GetX(), targetPC.GetY(), targetPC.GetZ(), 1, 1, 0) {
		rlcombat.Hit(level, entity, target, true)
		rlentity.Face(entity, targetPC.GetX()-pc.GetX(), targetPC.GetY()-pc.GetY())
		if target.HasComponent(rlcomponents.Dead) {
			event.GetQueuedInstance().QueueEvent(eventsystem.EntityKilledEvent{
				KillerName: rlentity.GetName(entity),
				TargetName: rlentity.GetName(target),
				Blueprint:  target.Blueprint,
				X:          targetPC.GetX(), Y: targetPC.GetY(), Z: targetPC.GetZ(),
			})
			wc.CurrentTask.Complete()
			wc.CurrentTask = nil
			aiMemory.State = "idle"
		}
	} else {
		wc.CurrentTask.X = targetPC.GetX()
		wc.CurrentTask.Y = targetPC.GetY()
		wc.CurrentTask.Z = targetPC.GetZ()
		MoveTowardsTarget(level, entity, targetPC.GetX(), targetPC.GetY(), targetPC.GetZ())
		rlentity.Face(entity, targetPC.GetX()-pc.GetX(), targetPC.GetY()-pc.GetY())
	}
}

func handleMoveTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	if !MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z) {
		CompleteTaskWithMessage(entity, wc.CurrentTask, "Reached destination")
		aiMemory.State = "idle"
	}
}

func handleResearchTask(_ *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	rr, ok := wc.CurrentTask.Data.(*task_requests.ResearchRequest)
	if !ok {
		aiMemory.State = "idle"
		wc.CurrentTask = nil
		return
	}

	rr.Progress++
	if rr.Progress >= rr.Required {
		event.GetQueuedInstance().QueueEvent(eventsystem.ResearchDoneEvent{TechKey: rr.TechKey})
		if tech, ok := research.GetTech(rr.TechKey); ok {
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Researched "+tech.Name)
		} else {
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Research complete")
		}
		aiMemory.State = "idle"
	}
}

func checkInventoryForMaterials(entity *ecs.Entity, buildable construction.Buildable) bool {
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for name, cost := range buildable.Cost {
		count := 0
		for _, item := range inv.Bag {
			if item.Blueprint == name {
				count++
			}
		}
		if count < cost {
			return false
		}
	}
	return true
}

func removeMaterialsFromInventory(entity *ecs.Entity, buildable construction.Buildable) {
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for name, cost := range buildable.Cost {
		removed := 0
		for removed < cost {
			inv.RemoveItemByName(name)
			removed++
		}
	}
}

// Satisfy unused import of task package via type reference
var _ = task.Task{}
var _ = log.Printf
