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
	"github.com/mechanical-lich/scifi_settlements/internal/combat"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/construction"
	"github.com/mechanical-lich/scifi_settlements/internal/crafting"
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

	// Task was assigned directly (e.g. equip/unequip from UI)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		aiMemory.State = "task"
		return
	}

	if mySettlement, ok := settlement.Settlements[sc.Name]; ok {
		t := mySettlement.Tasks.GetClosestNextTask(pc.GetX(), pc.GetY(), pc.GetZ())
		if t != nil {
			aiMemory.State = "task"
			wc.CurrentTask = t
			return
		}
		if mySettlement.Tasks.Count() > 0 {
			inProgress, completed, stopped := 0, 0, 0
			for _, t := range mySettlement.Tasks.GetTasks() {
				if t.Completed {
					completed++
				} else if t.InProgress {
					inProgress++
				} else if t.ManuallyStopped {
					stopped++
				}
			}
			log.Printf("[AI] %s idle — %d tasks in queue but none assignable (inProgress=%d completed=%d recentlyStopped=%d)",
				rlentity.GetName(entity), mySettlement.Tasks.Count(), inProgress, completed, stopped)
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
			return
		}

		// Auto-haul: pick up loose items on the ground
		item := FindLooseItemOnGround(level, pc.GetX(), pc.GetY(), pc.GetZ())
		if item != nil {
			aiMemory.TargetX = -1
			aiMemory.TargetY = -1
			aiMemory.State = "haul"
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
	case task_requests.CraftAction:
		handleCraftTask(level, entity, wc, aiMemory)
	case task_requests.EquipAction:
		handleEquipTask(level, entity, wc, aiMemory)
	case task_requests.UnequipAction:
		handleUnequipTask(level, entity, wc, aiMemory)
	default:
		handleMoveTask(level, entity, wc, aiMemory)
	}
}

func HandleDropOffState(level *world.Level, entity *ecs.Entity) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	var wc *components.WorkerComponent
	if entity.HasComponent(components.Worker) {
		wc = entity.GetComponent(components.Worker).(*components.WorkerComponent)
	}

	if MoveTowardsTarget(level, entity, aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ) {
		if wc != nil {
			wc.InteractTicks = 0
		}
		return
	}

	if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ, 1, 1, 0) {
		return
	}

	const dropoffHoldTicks = 6
	if wc != nil {
		wc.InteractTicks++
		if wc.InteractTicks < dropoffHoldTicks {
			return
		}
		wc.InteractTicks = 0
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

	MoveTowardsTarget(level, entity, storagePC.GetX(), storagePC.GetY(), storagePC.GetZ())
	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), storagePC.GetX(), storagePC.GetY(), storagePC.GetZ(), 1, 1, 0) {
		storageC := storageEntity.GetComponent(components.Storage).(*components.StorageComponent)
		material := storageC.TakeOne(searching)
		if material != nil {
			inv.AddItem(material)
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
					if newEntity.HasComponent(rlcomponents.Door) {
						door := newEntity.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent)
						faction := sc.Name
						if entity.HasComponent(rlcomponents.Description) {
							dc := entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
							if dc.Faction != "" {
								faction = dc.Faction
							}
						}
						door.OwnedBy = faction
					}
					level.AddEntity(newEntity)
				}
			} else {
				tile.Type = world.TileNameToIndex[buildRequest.Type]
				tileDef := world.TileDefinitions[tile.Type]
				if tileDef.AutoTile != 0 {
					tile.Variant = 0
				} else {
					tile.Variant = utility.GetRandom(0, len(tileDef.Variants))
				}
				if tileDef.StairsUp {
					if t2i := level.GetTileAt(buildRequest.X, buildRequest.Y, buildRequest.Z+1); t2i != nil {
						t2 := t2i.(*world.Tile)
						t2.Type = world.TileNameToIndex["stairs_down"]
						t2.Variant = 0
					}
				}
				if tileDef.StairsDown {
					if t2i := level.GetTileAt(buildRequest.X, buildRequest.Y, buildRequest.Z-1); t2i != nil {
						t2 := t2i.(*world.Tile)
						t2.Type = world.TileNameToIndex["stairs_up"]
						t2.Variant = 0
					}
				}
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
			clearTileRadiation(tile)
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

// clearTileRadiation zeros a tile's runtime radiation level. Called after a
// tile is dug or mined since the contamination leaves with the displaced
// material. Radioactive yield is now driven by the tile type (e.g.
// "radioactive_ore"), not the runtime radiation field.
func clearTileRadiation(tile *world.Tile) {
	if tile == nil {
		return
	}
	tile.Radiation = 0
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

		// Choppable entity branch — take it down and let CleanUpSystem fire
		// drops. The Target may have been removed already if a previous
		// worker finished it; cancel the task in that case.
		if req.Target != nil {
			if req.Target.HasComponent(rlcomponents.Dead) {
				CompleteTaskWithMessage(entity, wc.CurrentTask, "Already harvested")
				aiMemory.State = "idle"
				return
			}
			if req.Progress >= req.Required {
				req.Target.AddComponent(&rlcomponents.DeadComponent{})
				CompleteTaskWithMessage(entity, wc.CurrentTask, "Harvested")
				aiMemory.State = "idle"
			}
			return
		}

		// Tile branch — yield ore every 10 ticks so partial work isn't wasted
		if req.Progress%10 == 0 {
			tile := level.GetTileAt(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z).(*world.Tile)
			tileName := world.TileDefinitions[tile.Type].Name
			var dropBlueprint string
			switch tileName {
			case "ore_deposit":
				dropBlueprint = "metal_ore"
			case "crystal_vein":
				dropBlueprint = "crystal"
			case "radioactive_ore":
				dropBlueprint = "radioactive_material"
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
			clearTileRadiation(tile)
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
	if MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z) {
		wc.InteractTicks = 0
		return
	}
	if pc.GetX() != wc.CurrentTask.X || pc.GetY() != wc.CurrentTask.Y {
		wc.CurrentTask.Stop()
		wc.CurrentTask = nil
		wc.InteractTicks = 0
		return
	}
	const pickupHoldTicks = 6
	wc.InteractTicks++
	if wc.InteractTicks < pickupHoldTicks {
		return
	}
	PickupItemFromTile(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, pc.GetZ())
	CompleteTaskWithMessage(entity, wc.CurrentTask, "Fetched item")
	wc.InteractTicks = 0
	aiMemory.State = "idle"
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

	// Check for equipped ranged weapon
	var rangedWeaponEntity *ecs.Entity
	if entity.HasComponent(rlcomponents.Inventory) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for _, item := range []*ecs.Entity{inv.LeftHand, inv.RightHand} {
			if item != nil && item.HasComponent(rlcomponents.Weapon) {
				w := item.GetComponent(rlcomponents.Weapon).(*rlcomponents.WeaponComponent)
				if w.Ranged {
					rangedWeaponEntity = item
					break
				}
			}
		}
	}

	if rangedWeaponEntity != nil && combat.Shoot(level, entity, targetPC.GetX(), targetPC.GetY(), targetPC.GetZ(), rangedWeaponEntity) {
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
		return
	}

	// Melee fallback
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
	tx, ty, tz := wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z
	if MoveTowardsTarget(level, entity, tx, ty, tz) {
		return
	}
	// Arrived — inspect target tile and auto-detect action.
	if interactWithTile(level, entity, wc, aiMemory, tx, ty, tz) {
		return
	}
	CompleteTaskWithMessage(entity, wc.CurrentTask, "Reached destination")
	aiMemory.State = "idle"
}

// interactWithTile inspects the tile at (tx,ty,tz) and morphs wc.CurrentTask into
// the appropriate action. Returns true if an action was started, false to fall through to scout.
func interactWithTile(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent, tx, ty, tz int) bool {
	// 1. Attack: hostile entity with health on the tile
	var candidates []*ecs.Entity
	level.GetEntitiesAt(tx, ty, tz, &candidates)
	for _, candidate := range candidates {
		if candidate == entity {
			continue
		}
		if candidate.HasComponent(rlcomponents.Health) && candidate.HasComponent(rlcomponents.HostileAI) {
			wc.CurrentTask.Action = task_requests.AttackAction
			wc.CurrentTask.Data = candidate
			return true
		}
	}

	// 2. Item on tile
	for _, candidate := range candidates {
		if candidate == entity {
			continue
		}
		if candidate.HasComponent(rlcomponents.Item) {
			wc.CurrentTask.Action = task_requests.PickupAction
			return true
		}
	}

	// 3. Minable or diggable tile
	tileI := level.GetTileAt(tx, ty, tz)
	if tileI != nil {
		tile := tileI.(*world.Tile)
		tileDef := world.TileDefinitions[tile.Type]
		tileName := tileDef.Name
		if tileName == "ore_deposit" || tileName == "crystal_vein" {
			wc.CurrentTask.Action = task_requests.MineAction
			wc.CurrentTask.Data = task_requests.MineRequest{X: tx, Y: ty, Z: tz, Required: 50}
			return true
		}
		if tileDef.Solid {
			wc.CurrentTask.Action = task_requests.DigAction
			wc.CurrentTask.Data = task_requests.DigRequest{X: tx, Y: ty, Z: tz, Required: 5}
			return true
		}
	}

	return false
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

func handleCraftTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	cr, ok := wc.CurrentTask.Data.(task_requests.CraftRequest)
	if !ok {
		log.Printf("[CRAFT] bad task data type for %s", rlentity.GetName(entity))
		aiMemory.State = "idle"
		return
	}

	recipe, found := crafting.GetRecipe(cr.RecipeID)
	if !found {
		log.Printf("[CRAFT] recipe not found: %s", cr.RecipeID)
		wc.CurrentTask.Stop()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
		return
	}

	if !checkInventoryForCraft(entity, recipe.Cost) {
		aiMemory.State = "gather_materials_craft"
		return
	}

	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(),
		cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ, 1, 1, 0) {

		cr.Progress++
		wc.CurrentTask.Data = cr
		if cr.Progress >= cr.Required {
			removeMaterialsByCost(entity, recipe.Cost)
			output, err := factory.Create(recipe.Output, cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ)
			if err == nil {
				inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
				inv.AddItem(output)
			}
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Crafted "+recipe.Name)
			aiMemory.State = "idle"
		}
	} else {
		if !MoveTowardsTarget(level, entity, cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ) {
			log.Printf("[CRAFT] %s can't reach workbench at (%d,%d,%d) from (%d,%d,%d)",
				rlentity.GetName(entity), cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ,
				pc.GetX(), pc.GetY(), pc.GetZ())
			wc.CurrentTask.Stop()
			wc.CurrentTask = nil
			aiMemory.State = "idle"
		}
	}
}

func HandleGatherMaterialsCraftState(level *world.Level, entity *ecs.Entity) {
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)

	if wc.CurrentTask == nil {
		aiMemory.State = "idle"
		return
	}

	cr, ok := wc.CurrentTask.Data.(task_requests.CraftRequest)
	if !ok {
		aiMemory.State = "idle"
		return
	}

	recipe, found := crafting.GetRecipe(cr.RecipeID)
	if !found {
		aiMemory.State = "idle"
		return
	}

	// Find the first missing material
	searching := ""
	for name, cost := range recipe.Cost {
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

	storageEntity := FindClosestStorageWith(level, sc.Name, searching, cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ)
	if storageEntity == nil {
		wc.CurrentTask.Stop()
		wc.CurrentTask = nil
		message.PostMessage(rlentity.GetName(entity), "Insufficient materials to craft "+recipe.Name)
		aiMemory.State = "idle"
		return
	}

	storagePC := storageEntity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)

	MoveTowardsTarget(level, entity, storagePC.GetX(), storagePC.GetY(), storagePC.GetZ())
	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), storagePC.GetX(), storagePC.GetY(), storagePC.GetZ(), 1, 1, 0) {
		storageC := storageEntity.GetComponent(components.Storage).(*components.StorageComponent)
		material := storageC.TakeOne(searching)
		if material != nil {
			inv.AddItem(material)
		}
	}
}

func handleEquipTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)

	req, ok := wc.CurrentTask.Data.(task_requests.EquipRequest)
	if !ok {
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
		return
	}

	// Check if already in bag
	for _, item := range inv.Bag {
		if item.Blueprint == req.ItemBlueprint {
			inv.Equip(item)
			wc.CurrentTask.Complete()
			wc.CurrentTask = nil
			aiMemory.State = "idle"
			return
		}
	}

	// Go to storage to pick it up
	storageEntity := FindClosestStorageWith(level, sc.Name, req.ItemBlueprint, pc.GetX(), pc.GetY(), pc.GetZ())
	if storageEntity == nil {
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
		return
	}

	storagePC := storageEntity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	MoveTowardsTarget(level, entity, storagePC.GetX(), storagePC.GetY(), storagePC.GetZ())
	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), storagePC.GetX(), storagePC.GetY(), storagePC.GetZ(), 1, 1, 0) {
		storageC := storageEntity.GetComponent(components.Storage).(*components.StorageComponent)
		item := storageC.TakeOne(req.ItemBlueprint)
		if item != nil {
			inv.Equip(item)
		}
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
	}
}

func handleUnequipTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)

	req, ok := wc.CurrentTask.Data.(task_requests.UnequipRequest)
	if !ok {
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
		return
	}

	slot := rlcomponents.ItemSlot(req.Slot)
	item := inv.Unequip(slot) // moves item to bag
	if item == nil {
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
		return
	}

	// Walk to storage and deposit
	storage := FindAvailableStorage(level, sc.Name)
	if storage == nil {
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "dropoff"
		return
	}

	storagePC := storage.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	MoveTowardsTarget(level, entity, storagePC.GetX(), storagePC.GetY(), storagePC.GetZ())
	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), storagePC.GetX(), storagePC.GetY(), storagePC.GetZ(), 1, 1, 0) {
		storageC := storage.GetComponent(components.Storage).(*components.StorageComponent)
		for _, bagItem := range append([]*ecs.Entity{}, inv.Bag...) {
			storageC.AddItem(bagItem)
			inv.RemoveItem(bagItem)
		}
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
	}
}

func checkInventoryForCraft(entity *ecs.Entity, cost map[string]int) bool {
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for name, required := range cost {
		count := 0
		for _, item := range inv.Bag {
			if item.Blueprint == name {
				count++
			}
		}
		if count < required {
			return false
		}
	}
	return true
}

func removeMaterialsByCost(entity *ecs.Entity, cost map[string]int) {
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for name, required := range cost {
		for i := 0; i < required; i++ {
			inv.RemoveItemByName(name)
		}
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
