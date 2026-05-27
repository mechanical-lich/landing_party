package workerai

import (
	"fmt"
	"log"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/eventsystem"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlai"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/task"
	"github.com/mechanical-lich/mlge/utility"
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
		const momentumRadius = 15

		// Momentum: if last task was extraction and that type is still allowed,
		// prefer same type nearby before falling back.
		if isExtractionAction(wc.LastTaskAction) && isActionAllowed(wc, wc.LastTaskAction) {
			t := mySettlement.Tasks.PeekClosestNextTask(pc.GetX(), pc.GetY(), pc.GetZ(), wc.LastTaskAction)
			if t != nil && utility.Abs(t.X-pc.GetX())+utility.Abs(t.Y-pc.GetY()) <= momentumRadius {
				t.Start()
				t.Interruptible = task_requests.IsInterruptibleAction(t.Action)
				wc.CurrentTask = t
				wc.LastTaskAction = t.Action
				aiMemory.State = "task"
				return
			}
			// No nearby same-type work — clear momentum and fall through
			wc.LastTaskAction = ""
		}

		var t *task.Task
		if wc.AllowedTasks == nil {
			t = mySettlement.Tasks.GetClosestNextTask(pc.GetX(), pc.GetY(), pc.GetZ())
		} else if len(wc.AllowedTasks) > 0 {
			t = mySettlement.Tasks.GetClosestNextTask(pc.GetX(), pc.GetY(), pc.GetZ(), wc.AllowedTasks...)
		}
		// else: AllowedTasks is non-nil empty — all tasks blocked, t stays nil
		if t != nil {
			if !workerQualifiesForTask(entity, t) {
				t.ReQueue()
			} else {
				t.Interruptible = task_requests.IsInterruptibleAction(t.Action)
				aiMemory.State = "task"
				wc.CurrentTask = t
				wc.LastTaskAction = t.Action
				return
			}
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
	case task_requests.RetrieveAction:
		handleRetrieveTask(level, entity, wc, aiMemory)
	case task_requests.SleepAction:
		handleSleepTask(level, entity, wc, aiMemory)
	case task_requests.PassoutAction:
		handlePassoutTask(level, entity, wc, aiMemory)
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

	if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ, 1, 1, 0) {
		if MoveTowardsTarget(level, entity, aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ) {
			if wc != nil {
				wc.InteractTicks = 0
			}
			return
		}
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
		var toDeposit []*ecs.Entity
		if wc != nil && wc.DropOffItem != nil {
			toDeposit = []*ecs.Entity{wc.DropOffItem}
			wc.DropOffItem = nil
		} else {
			toDeposit = append([]*ecs.Entity{}, inv.Bag...)
		}
		for _, item := range toDeposit {
			storageC.AddItem(item)
			inv.RemoveItem(item)
			if item.HasComponent(rlcomponents.Description) {
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

	// Find the first missing material and compute the exact deficit.
	searching := ""
	deficit := 0
	for name, cost := range buildable.Cost {
		count := 0
		for _, item := range inv.Bag {
			if item.Blueprint != name {
				continue
			}
			if item.HasComponent(components.ResourceItem) {
				rc := item.GetComponent(components.ResourceItem).(*components.ResourceItemComponent)
				count += rc.Quantity
			} else {
				count++
			}
		}
		if count < cost {
			searching = name
			deficit = cost - count
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

		// Detect whether this item type is a stackable resource.
		isResource := false
		for _, item := range storageC.Items {
			if item.Blueprint == searching && item.HasComponent(components.ResourceItem) {
				isResource = true
				break
			}
		}

		if isResource {
			// Deduct exactly the deficit, then carry a new entity with that quantity.
			if storageC.DeductResource(searching, deficit) {
				material, err := factory.Create(searching, 0, 0, 0)
				if err == nil {
					rc := material.GetComponent(components.ResourceItem).(*components.ResourceItemComponent)
					rc.Quantity = deficit
					inv.AddItem(material)
				}
			}
		} else {
			// Non-resource item (equipment, etc.) — take one at a time.
			material := storageC.TakeOne(searching)
			if material != nil {
				inv.AddItem(material)
			}
		}
	}
}

// HandleHaulState drives the haul state: pick up a loose item on the ground
// and deliver it to the nearest available storage.
func HandleHaulState(level *world.Level, entity *ecs.Entity) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)

	// If holding something, go drop it off
	if len(inv.Bag) > 0 {
		storage := FindAvailableStorage(level, sc.Name)
		if storage == nil {
			aiMemory.State = "idle"
			return
		}
		spc := storage.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if !MoveTowardsTarget(level, entity, spc.GetX(), spc.GetY(), spc.GetZ()) {
			storageC := storage.GetComponent(components.Storage).(*components.StorageComponent)
			for _, item := range inv.Bag {
				storageC.AddItem(item)
			}
			inv.Bag = inv.Bag[:0]
			message.PostMessage(rlentity.GetName(entity), "Stored items")
			aiMemory.State = "idle"
		}
		return
	}

	// No target set — find a loose item
	if aiMemory.TargetX == -1 && aiMemory.TargetY == -1 {
		item := FindLooseItemOnGround(level, pc.GetX(), pc.GetY(), pc.GetZ())
		if item == nil {
			aiMemory.State = "idle"
			return
		}
		ipc := item.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		aiMemory.TargetX = ipc.GetX()
		aiMemory.TargetY = ipc.GetY()
		aiMemory.TargetZ = ipc.GetZ()
		return
	}

	if !MoveTowardsTarget(level, entity, aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ) {
		// Pick up any items at this tile
		var tileEntities []*ecs.Entity
		level.GetEntitiesAt(aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ, &tileEntities)
		picked := false
		for _, e := range tileEntities {
			if e.HasComponent(rlcomponents.Item) && e.HasComponent(rlcomponents.Position) {
				inv.AddItem(e)
				level.RemoveEntity(e)
				message.PostMessage(rlentity.GetName(entity), fmt.Sprintf("Picked up %s", e.Blueprint))
				picked = true
			}
		}
		if !picked {
			aiMemory.State = "idle"
		}
		aiMemory.TargetX = -1
		aiMemory.TargetY = -1
	}
}

// HandleFindFood drives the find_food state: eat from inventory first,
// otherwise walk to the nearest storage with food and consume it there.
func HandleFindFood(level *world.Level, entity *ecs.Entity) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)

	// Eat from own inventory first before walking to storage.
	if entity.HasComponent(rlcomponents.Inventory) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for i, item := range inv.Bag {
			if !item.HasComponent(rlcomponents.Food) {
				continue
			}
			foodC := item.GetComponent(rlcomponents.Food).(*rlcomponents.FoodComponent)
			if entity.HasComponent(rlcomponents.Health) {
				hc := entity.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
				hc.Energy += foodC.Amount
			}
			if entity.HasComponent(components.Needs) {
				nc := entity.GetComponent(components.Needs).(*components.NeedsComponent)
				nc.Eat(foodC.Amount)
			}
			inv.Bag = append(inv.Bag[:i], inv.Bag[i+1:]...)
			message.PostMessage(rlentity.GetName(entity), "Ate "+item.Blueprint)
			aiMemory.TargetX = -1
			aiMemory.TargetY = -1
			aiMemory.State = "idle"
			return
		}
	}

	if aiMemory.TargetX == -1 && aiMemory.TargetY == -1 {
		storage := FindClosestStorageWithComponent(level, sc.Name, rlcomponents.Food, pc.GetX(), pc.GetY(), pc.GetZ())
		if storage != nil {
			spc := storage.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			aiMemory.TargetX = spc.GetX()
			aiMemory.TargetY = spc.GetY()
			aiMemory.TargetZ = spc.GetZ()
		} else {
			aiMemory.TargetX = 0
			aiMemory.TargetY = 0
			aiMemory.State = "idle"
		}
		return
	}

	if !MoveTowardsTarget(level, entity, aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ) {
		var tileEntities []*ecs.Entity
		level.GetEntitiesAt(aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ, &tileEntities)
		for _, e := range tileEntities {
			if !e.HasComponent(components.Storage) {
				continue
			}
			storageC := e.GetComponent(components.Storage).(*components.StorageComponent)
			foodEntity := storageC.TakeOneWithComponent(rlcomponents.Food)
			if foodEntity != nil {
				hc := entity.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
				foodC := foodEntity.GetComponent(rlcomponents.Food).(*rlcomponents.FoodComponent)
				hc.Energy += foodC.Amount
				if entity.HasComponent(components.Needs) {
					nc := entity.GetComponent(components.Needs).(*components.NeedsComponent)
					nc.Eat(foodC.Amount)
				}
				message.PostMessage(rlentity.GetName(entity), "Ate "+foodEntity.Blueprint)
				aiMemory.TargetX = -1
				aiMemory.TargetY = -1
				aiMemory.State = "idle"
				return
			}
		}
		// Nothing found at target — reset
		aiMemory.TargetX = -1
		aiMemory.TargetY = -1
		aiMemory.State = "idle"
	}
}

// workerQualifiesForTask returns false only when the task requires a minimum
// Intelligence stat that the entity doesn't meet.
func workerQualifiesForTask(entity *ecs.Entity, t *task.Task) bool {
	if t.Action != task_requests.ResearchAction {
		return true
	}
	rr, ok := t.Data.(*task_requests.ResearchRequest)
	if !ok {
		return true
	}
	tech, found := research.GetTech(rr.TechKey)
	if !found || tech.RequiredInt == 0 {
		return true
	}
	if !entity.HasComponent(rlcomponents.Stats) {
		return false
	}
	sc := entity.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent)
	return sc.Int >= tech.RequiredInt
}

func isExtractionAction(action task.TaskAction) bool {
	return action == task_requests.DigAction || action == task_requests.MineAction
}

func isActionAllowed(wc *components.WorkerComponent, action task.TaskAction) bool {
	if wc.AllowedTasks == nil {
		return true
	}
	for _, a := range wc.AllowedTasks {
		if a == action {
			return true
		}
	}
	return false
}
