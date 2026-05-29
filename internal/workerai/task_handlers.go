package workerai

import (
	"log"
	"time"

	"github.com/mechanical-lich/landing_party/internal/combat"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/emotes"
	"github.com/mechanical-lich/landing_party/internal/eventsystem"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/progression"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlai"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcombat"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/task"
	"github.com/mechanical-lich/mlge/utility"
)

const sleepRecoveryPerCycle = 10

func CompleteTaskWithMessage(entity *ecs.Entity, t *task.Task, msg string) {
	t.Complete()
	wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)
	wc.CurrentTask = nil
	message.PostMessage(rlentity.GetName(entity), msg)
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

func markSeenAroundStairs(level *world.Level, x, y, z int) {
	const radius = 6
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if !level.InBounds(x+dx, y+dy, z) {
				continue
			}
			if !level.GetSeen(x+dx, y+dy, z) {
				level.SetSeen(x+dx, y+dy, z, true)
			}
		}
	}
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
		// Mining targets the Middle slot (ore veins, walls).
		if tile.Middle.IsEmpty() {
			return false
		}
		tileDef := world.TileDefinitions[tile.Middle.Type]
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

func queueRetrieveTask(entity *ecs.Entity, item *ecs.Entity, x, y, z int) {
	if !entity.HasComponent(components.Settlement) {
		return
	}
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
	mySettlement, ok := settlement.Settlements[sc.Name]
	if !ok {
		return
	}
	mySettlement.Tasks.AddTask(&task.Task{
		X: x, Y: y, Z: z,
		Action:  task_requests.RetrieveAction,
		Data:    task_requests.RetrieveRequest{Item: item},
		Created: time.Now(),
	})
}

// dismountSleeper moves the worker off the bed back to their pre-mount tile if
// it's still walkable; otherwise picks any walkable tile adjacent to the bed.
// If nothing is walkable, the worker stays on the bed (caller should still
// complete the task).
func dismountSleeper(level *world.Level, entity *ecs.Entity, sr *task_requests.SleepRequest) {
	if canStand(level, sr.PrevX, sr.PrevY, sr.PrevZ) {
		level.PlaceEntity(sr.PrevX, sr.PrevY, sr.PrevZ, entity)
		return
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := sr.X+dx, sr.Y+dy
			if canStand(level, nx, ny, sr.Z) {
				level.PlaceEntity(nx, ny, sr.Z, entity)
				return
			}
		}
	}
}

// canStand reports whether (x,y,z) has walkable terrain and no blocking entity.
func canStand(level *world.Level, x, y, z int) bool {
	if !level.IsWalkable(x, y, z) {
		return false
	}
	return level.GetSolidEntityAt(x, y, z) == nil
}

func handleRetrieveTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)

	req, ok := wc.CurrentTask.Data.(task_requests.RetrieveRequest)
	if !ok {
		aiMemory.State = "idle"
		return
	}

	tx, ty, tz := wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z

	if !MoveTowardsTarget(level, entity, tx, ty, tz) {
		if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), tx, ty, tz, 1, 1, 0) {
			wc.CurrentTask.ReQueue()
			wc.CurrentTask = nil
			aiMemory.State = "idle"
			return
		}

		// At or adjacent to the drop tile — look for our specific item
		var tileEntities []*ecs.Entity
		level.GetEntitiesAt(tx, ty, tz, &tileEntities)
		for _, e := range tileEntities {
			if e == req.Item {
				inv.AddItem(e)
				level.RemoveEntity(e)
				progression.AwardXP(entity, "Dex", progression.XPPerTask)
				CompleteTaskWithMessage(entity, wc.CurrentTask, "Retrieved "+e.Blueprint)
				storage := FindAvailableStorage(level, sc.Name)
				if storage != nil {
					spc := storage.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
					aiMemory.TargetX = spc.GetX()
					aiMemory.TargetY = spc.GetY()
					aiMemory.TargetZ = spc.GetZ()
					wc.DropOffItem = e
					aiMemory.State = "dropoff"
				} else {
					aiMemory.State = "idle"
				}
				return
			}
		}

		// Item already picked up by someone else — complete silently
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

	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ, 1, 1, 0) {
		if !MoveTowardsTarget(level, entity, cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ) {
			log.Printf("[CRAFT] %s can't reach workbench at (%d,%d,%d) from (%d,%d,%d)",
				rlentity.GetName(entity), cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ,
				pc.GetX(), pc.GetY(), pc.GetZ())
			wc.CurrentTask.ReQueue()
			wc.CurrentTask = nil
			aiMemory.State = "idle"
		}
		return
	}

	// At workbench: check storage has ingredients on first tick
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
	if cr.Progress == 0 && !checkSettlementStorageForCraft(level, sc.Name, recipe.Cost) {
		wc.CurrentTask.Stop()
		wc.CurrentTask = nil
		message.PostMessage(rlentity.GetName(entity), "Insufficient materials to craft "+recipe.Name)
		aiMemory.State = "idle"
		return
	}

	cr.Progress++
	wc.CurrentTask.Data = cr
	if cr.Progress >= cr.Required {
		deductFromSettlementStorage(level, sc.Name, recipe.Cost)
		output, err := factory.Create(recipe.Output, cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ)
		if err == nil {
			if recipe.Output == "colonist" {
				// Bio-printed colonists join the colony at the printer rather
				// than going into the crafter's inventory.
				if output.HasComponent(components.Settlement) {
					output.GetComponent(components.Settlement).(*components.SettlementComponent).Name = sc.Name
				} else {
					output.AddComponent(&components.SettlementComponent{Name: sc.Name})
				}
				if !output.HasComponent(components.Worker) {
					output.AddComponent(&components.WorkerComponent{SelfDefend: true})
				}
				level.AddEntity(output)
			} else {
				inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
				inv.AddItem(output)
			}
		}
		progression.AwardXP(entity, "Int", progression.XPPerTask)
		CompleteTaskWithMessage(entity, wc.CurrentTask, "Crafted "+recipe.Name)
		aiMemory.State = "idle"
	}
}

func handleResearchTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	rr, ok := wc.CurrentTask.Data.(*task_requests.ResearchRequest)
	if !ok {
		aiMemory.State = "idle"
		wc.CurrentTask = nil
		return
	}

	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z, 1, 1, 0) {
		if !MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z) {
			wc.CurrentTask.ReQueue()
			wc.CurrentTask = nil
			aiMemory.State = "idle"
		}
		return
	}

	tech, techFound := research.GetTech(rr.TechKey)
	if rr.Progress == 0 && techFound && len(tech.Cost) > 0 {
		sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
		if !checkSettlementStorageForCraft(level, sc.Name, tech.Cost) {
			wc.CurrentTask.Stop()
			wc.CurrentTask = nil
			message.PostMessage(rlentity.GetName(entity), "Insufficient materials to research "+tech.Name)
			aiMemory.State = "idle"
			return
		}
		deductFromSettlementStorage(level, sc.Name, tech.Cost)
	}
	rr.Progress++
	if rr.Progress >= rr.Required {
		event.GetQueuedInstance().QueueEvent(eventsystem.ResearchDoneEvent{TechKey: rr.TechKey})
		progression.AwardXP(entity, "Int", progression.XPPerTask)
		if techFound {
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Researched "+tech.Name)
		} else {
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Research complete")
		}
		aiMemory.State = "idle"
	}
}

func handleSleepTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	sr, ok := wc.CurrentTask.Data.(*task_requests.SleepRequest)
	if !ok {
		aiMemory.State = "idle"
		wc.CurrentTask = nil
		return
	}

	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)

	if !sr.OnBed {
		if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), sr.X, sr.Y, sr.Z, 1, 1, 0) {
			if !MoveTowardsTarget(level, entity, sr.X, sr.Y, sr.Z) {
				wc.CurrentTask.ReQueue()
				wc.CurrentTask = nil
				aiMemory.State = "idle"
			}
			return
		}
		// Adjacent to bed: climb on. Save dismount tile so we can return there
		// when fully rested.
		sr.PrevX, sr.PrevY, sr.PrevZ = pc.GetX(), pc.GetY(), pc.GetZ()
		level.PlaceEntity(sr.X, sr.Y, sr.Z, entity)
		sr.OnBed = true
	}

	sr.Progress++
	if sr.OnBed {
		emotes.Set(entity, emotes.Sleeps, 2, 2)
	}
	if sr.Progress < sr.Required {
		return
	}
	// Cycle complete: heal 1 HP and drain exhaustion.
	sr.Progress = 0
	hc := entity.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
	if hc.Health < hc.MaxHealth {
		hc.Health++
	}
	exhausted := false
	if entity.HasComponent(components.Needs) {
		nc := entity.GetComponent(components.Needs).(*components.NeedsComponent)
		nc.Exhaustion -= sleepRecoveryPerCycle
		if nc.Exhaustion < 0 {
			nc.Exhaustion = 0
		}
		// Record this bed so the colonist returns here next time.
		nc.LastBedX, nc.LastBedY, nc.LastBedZ = sr.X, sr.Y, sr.Z
		nc.HasKnownBed = true
		exhausted = nc.Exhaustion > 0
	}
	if !exhausted && hc.Health >= hc.MaxHealth {
		dismountSleeper(level, entity, sr)
		CompleteTaskWithMessage(entity, wc.CurrentTask, "Fully rested")
		aiMemory.State = "idle"
	}
}

func handlePassoutTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	pr, ok := wc.CurrentTask.Data.(*task_requests.PassoutRequest)
	if !ok {
		aiMemory.State = "idle"
		wc.CurrentTask = nil
		return
	}

	pr.Progress++
	emotes.Set(entity, emotes.Sleeps, 2, 2)
	if pr.Progress < pr.Required {
		return
	}
	pr.Progress = 0

	if entity.HasComponent(components.Needs) {
		nc := entity.GetComponent(components.Needs).(*components.NeedsComponent)
		nc.Exhaustion -= sleepRecoveryPerCycle
		if nc.Exhaustion < 0 {
			nc.Exhaustion = 0
		}
		if nc.Exhaustion > 0 {
			return // still recovering
		}
	}

	CompleteTaskWithMessage(entity, wc.CurrentTask, "Woke up after passing out")
	aiMemory.State = "idle"
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

		// Tile branch — yield ore every 10 ticks so partial work isn't wasted.
		// Drops land on the miner's tile so haulers can reach them without
		// needing to stand on the (solid) ore deposit.
		if req.Progress%10 == 0 {
			tile := level.GetTileAt(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z).(*world.Tile)
			// Ore lives in the Middle slot.
			tileName := ""
			if !tile.Middle.IsEmpty() {
				tileName = world.TileDefinitions[tile.Middle.Type].Name
			}
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
				ore, err := factory.Create(dropBlueprint, pc.GetX(), pc.GetY(), pc.GetZ())
				if err == nil {
					level.AddEntity(ore)
					queueRetrieveTask(entity, ore, pc.GetX(), pc.GetY(), pc.GetZ())
				}
			}
		}

		if req.Progress >= req.Required {
			tile := level.GetTileAt(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z).(*world.Tile)
			clearTileRadiation(tile)
			// Layered mine: just remove the Middle. Floor stays.
			level.ClearMiddle(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z)
			level.InvalidateSunColumn(wc.CurrentTask.X, wc.CurrentTask.Y)
			progression.AwardXP(entity, "Str", progression.XPPerTask)
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Mined out deposit")
			aiMemory.State = "idle"
		}
	} else {
		if !MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z) {
			wc.CurrentTask.ReQueue()
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
		wc.CurrentTask.ReQueue()
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
	progression.AwardXP(entity, "Dex", progression.XPPerTask)
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
			if buildable.IsEntity {
				newEntity, err := factory.Create(buildRequest.Type, buildRequest.X, buildRequest.Y, buildRequest.Z)
				if err == nil {
					canPlace := true
					if newEntity.HasComponent(rlcomponents.Size) {
						sizec := newEntity.GetComponent(rlcomponents.Size).(*rlcomponents.SizeComponent)
						if sizec.Width > 0 && sizec.Height > 0 {
							startX := buildRequest.X - sizec.Width/2
							startY := buildRequest.Y - sizec.Height/2
							for dx := 0; dx < sizec.Width && canPlace; dx++ {
								for dy := 0; dy < sizec.Height && canPlace; dy++ {
									if level.GetSolidEntityAt(startX+dx, startY+dy, buildRequest.Z) != nil {
										canPlace = false
									}
								}
							}
						}
					}
					if canPlace {
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
				}
			} else {
				typeIdx := world.TileNameToIndex[buildRequest.Type]
				tileDef := world.TileDefinitions[typeIdx]
				variant := 0
				if tileDef.AutoTile == 0 {
					variant = utility.GetRandom(0, len(tileDef.Variants))
				}
				// PaintTile dispatches into the slot declared by the tile def.
				level.PaintTile(buildRequest.X, buildRequest.Y, buildRequest.Z, buildRequest.Type, variant)
				if tileDef.StairsUp {
					level.PaintTile(buildRequest.X, buildRequest.Y, buildRequest.Z+1, "stairs_down", 0)
					markSeenAroundStairs(level, buildRequest.X, buildRequest.Y, buildRequest.Z+1)
				}
				if tileDef.StairsDown {
					level.PaintTile(buildRequest.X, buildRequest.Y, buildRequest.Z-1, "stairs_up", 0)
					markSeenAroundStairs(level, buildRequest.X, buildRequest.Y, buildRequest.Z-1)
				}
				markSeenAroundStairs(level, buildRequest.X, buildRequest.Y, buildRequest.Z)
				level.InvalidateSunColumn(buildRequest.X, buildRequest.Y)
			}
			removeMaterialsFromInventory(entity, buildable)
			event.GetQueuedInstance().QueueEvent(eventsystem.StructureBuiltEvent{
				Type: buildRequest.Type, Settlement: sc.Name,
				X: buildRequest.X, Y: buildRequest.Y, Z: buildRequest.Z,
			})
			progression.AwardXP(entity, "Str", progression.XPPerTask)
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Built "+buildRequest.Type)
			aiMemory.State = "idle"
		}
	} else {
		if !MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z) {
			wc.CurrentTask.ReQueue()
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
		tile := level.GetTilePtr(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z)
		// Bail out cleanly if the target became invalid (out of bounds) or no
		// longer has anything to dig (Middle already empty — e.g. someone
		// dug it first). Without this the task sits in-progress until a
		// worker hits Required ticks for nothing.
		if tile == nil || tile.Middle.IsEmpty() {
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Nothing to dig")
			aiMemory.State = "idle"
			return
		}
		req.Progress++
		wc.CurrentTask.Data = req
		if req.Progress >= req.Required {
			clearTileRadiation(tile)
			// Layered dig: just remove the Middle. Floor stays as whatever
			// was there.
			level.ClearMiddle(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z)
			level.InvalidateSunColumn(wc.CurrentTask.X, wc.CurrentTask.Y)
			progression.AwardXP(entity, "Str", progression.XPPerTask)
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Dug out tile")
			aiMemory.State = "idle"
		}
	} else {
		if !MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z) {
			wc.CurrentTask.ReQueue()
			wc.CurrentTask = nil
		}
	}
}
