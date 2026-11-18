package workerai

import (
	"fmt"
	"github.com/mechanical-lich/landing_party/internal/chatter"
	"log"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/eventsystem"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlai"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
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
	wc.DropOffTicks = 0

	// Task was assigned directly (e.g. equip/unequip from UI)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		aiMemory.State = "task"
		return
	}

	if wc.ReturnRetryTicks > 0 {
		wc.ReturnRetryTicks--
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
				wc.LoggedUnassignable = false
				aiMemory.State = "task"
				return
			}
			// No nearby same-type work — clear momentum and fall through
			wc.LastTaskAction = ""
		}

		var t *task.Task
		if wc.AllowedTasks == nil {
			t = pickAffordableTask(level, entity, &mySettlement.Tasks, sc.Name)
		} else if len(wc.AllowedTasks) > 0 {
			// An explicit list (a type's tasks, the player's filters) limits
			// the work a worker takes on its own, never where the player sends
			// it: the ground click's task (no action) is always taken.
			allowed := append(append([]task.TaskAction(nil), wc.AllowedTasks...), task_requests.MoveHereAction)
			t = pickAffordableTask(level, entity, &mySettlement.Tasks, sc.Name, allowed...)
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
				wc.LoggedUnassignable = false
				return
			}
		}
		if startReturningMaterials(level, entity, wc, aiMemory, sc.Name) {
			return
		}
		if mySettlement.Tasks.Count() == 0 {
			wc.LoggedUnassignable = false
		} else if !wc.LoggedUnassignable {
			wc.LoggedUnassignable = true
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

// returnRetryTicks is how long an idle worker waits before trying again to return
// materials it couldn't store.
const returnRetryTicks = 50

// startReturningMaterials sends an idle worker to store one Material item from its
// bag, so surplus and cancelled-task materials count as stock again. Each deposit
// lands back in idle, which sends the next item. Gear stays with the worker, and
// so does one food item; any more food goes to storage, where the others can eat
// it (a worker who hauled or picked up food kept it all, and the rest starved).
func startReturningMaterials(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent, settlementName string) bool {
	if wc.ReturnRetryTicks > 0 || !entity.HasComponent(rlcomponents.Inventory) {
		return false
	}
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	carrying := false
	keptFood := false
	for _, item := range entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag {
		if !item.HasComponent(components.Material) {
			if !item.HasComponent(rlcomponents.Food) {
				continue // gear
			}
			if !keptFood {
				keptFood = true // its own ration
				continue
			}
		}
		carrying = true
		dest := findClosestStorageWithRoomFor(level, settlementName, item, pc.GetX(), pc.GetY(), pc.GetZ())
		if dest == nil {
			continue
		}
		spc := dest.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ = spc.GetX(), spc.GetY(), spc.GetZ()
		wc.DropOffItem = item
		wc.DropOffDestEntity = dest
		wc.DropOffX, wc.DropOffY, wc.DropOffZ = 0, 0, 0
		aiMemory.State = "dropoff"
		return true
	}
	if carrying {
		wc.ReturnRetryTicks = returnRetryTicks
	}
	return false
}

// pickAffordableTask starts and returns the closest task the colony can pay for,
// passing over any it can't without cancelling them, since stock may arrive. The
// closest task alone isn't enough: an unaffordable one nearby must not idle a worker
// while other work is queued. Passed-over tasks are held InProgress only during the
// search so the scheduler offers the next closest.
func pickAffordableTask(level *world.Level, entity *ecs.Entity, tasks *task.TaskScheduler, settlementName string, allowed ...task.TaskAction) *task.Task {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	var skipped []*task.Task
	defer func() {
		for _, t := range skipped {
			t.InProgress = false
		}
	}()
	// Painted orders often share a cost, and each check scans every container.
	checked := map[string]bool{}
	for {
		t := tasks.PeekClosestNextTask(pc.GetX(), pc.GetY(), pc.GetZ(), allowed...)
		if t == nil {
			return nil
		}
		if canAffordTask(level, entity, t, settlementName, checked) {
			t.Start()
			return t
		}
		t.InProgress = true
		skipped = append(skipped, t)
	}
}

// canAffordTask reports whether containers plus the worker's own bag cover what a
// task still needs. Tasks without a material cost are always affordable.
func canAffordTask(level *world.Level, entity *ecs.Entity, t *task.Task, settlementName string, checked map[string]bool) bool {
	cost, ok := taskCost(t)
	if !ok {
		return true
	}
	need := remainingCost(cost.Inputs, cost.Delivered)
	if len(need) == 0 {
		return true
	}
	if !entity.HasComponent(rlcomponents.Inventory) {
		return false
	}
	need = subtractBag(entity, need)
	key := fmt.Sprint(need)
	affordable, seen := checked[key]
	if !seen {
		affordable = checkSettlementStorageForCraft(level, settlementName, need)
		checked[key] = affordable
	}
	return affordable
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
	case task_requests.ConvertAction:
		handleConvertTask(level, entity, wc, aiMemory)
	case task_requests.DeployAction:
		handleDeployTask(level, entity, wc, aiMemory)
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
	case task_requests.RelocateAction:
		handleRelocateTask(level, entity, wc, aiMemory)
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

	// A drop-off that has not delivered in maxDropOffTicks is given up, the
	// item kept in the bag: workers crowding a container with one free side
	// swap places forever, every step counting as progress, and hold it.
	if wc != nil {
		wc.DropOffTicks++
		if wc.DropOffTicks >= maxDropOffTicks {
			AbandonDropOff(wc, aiMemory)
			return
		}
	}
	if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ, 1, 1, 0) {
		if moved, _ := MoveTowardsTarget(level, entity, aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ); moved {
			if wc != nil {
				wc.InteractTicks = 0
				wc.ApproachFailTicks = 0
			}
			return
		}
		// Blocked or unreachable: give up rather than stay pinned here, where the
		// worker would ignore every task. The item stays in the bag.
		if wc != nil {
			wc.ApproachFailTicks++
			if wc.ApproachFailTicks >= maxApproachFailTicks {
				AbandonDropOff(wc, aiMemory)
			}
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

	// Player-issued drop with an explicit destination always takes precedence
	// over the legacy "whatever storage is at the target tile" fallback. The
	// destination is either a specific storage container (DropOffDestEntity)
	// or a ground tile (DropOffDestEntity nil with DropOffX/Y/Z set).
	if wc != nil && wc.DropOffItem != nil && (wc.DropOffDestEntity != nil ||
		(wc.DropOffX != 0 || wc.DropOffY != 0 || wc.DropOffZ != 0)) {
		dest := wc.DropOffDestEntity
		depositDirectedDropOff(level, entity, wc)
		if dest != nil && dest.HasComponent(components.Storage) {
			depositSurplusFood(entity, dest.GetComponent(components.Storage).(*components.StorageComponent))
		}
		aiMemory.TargetX, aiMemory.TargetY = -1, -1
		aiMemory.State = "idle"
		return
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
		// Rejected items intentionally stay in the worker's bag rather than
		// vanishing. The next call to HandleHaulState picks them up via a fresh
		// FindAvailableStorageFor lookup against bag[0], and that state has a
		// "no container takes anything" drop-to-ground safety net — so an
		// empty bag is not this function's responsibility, only this drop-off
		// attempt is.
		for _, item := range toDeposit {
			if !storageC.AddItem(item) {
				continue
			}
			inv.RemoveItem(item)
			if item.HasComponent(rlcomponents.Description) {
				dc := item.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
				level.QueueEvent(eventsystem.ItemStoredEvent{
					ItemName:   dc.Name,
					Blueprint:  item.Blueprint,
					Settlement: storageC.OwnedBy,
				})
			}
		}
		depositSurplusFood(entity, storageC)
	}
	aiMemory.State = "idle"
}

// depositSurplusFood stores the food in the worker's bag beyond the one item it
// keeps, into sc, at any drop-off there: a worker who hauled or picked up
// food otherwise carried it from task to task while the rest of the colony
// found none in storage and starved.
func depositSurplusFood(entity *ecs.Entity, sc *components.StorageComponent) {
	if !entity.HasComponent(rlcomponents.Inventory) {
		return
	}
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	kept := false
	for _, item := range append([]*ecs.Entity(nil), inv.Bag...) {
		if !item.HasComponent(rlcomponents.Food) {
			continue
		}
		if !kept {
			kept = true
			continue
		}
		if sc.AddItem(item) {
			inv.RemoveItem(item)
		}
	}
}

// maxDropOffTicks is how long a drop-off may take before it is given up (a
// walk across a site is well under it).
const maxDropOffTicks = 600

// AbandonDropOff gives up the worker's drop-off, the item kept in its bag,
// and makes it idle; returning materials waits returnRetryTicks, so the
// next idle tick finds other work (or food) first.
func AbandonDropOff(wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	wc.ApproachFailTicks = 0
	wc.DropOffTicks = 0
	wc.DropOffItem = nil
	wc.DropOffDestEntity = nil
	wc.DropOffX, wc.DropOffY, wc.DropOffZ = 0, 0, 0
	wc.ReturnRetryTicks = returnRetryTicks
	aiMemory.TargetX, aiMemory.TargetY = -1, -1
	aiMemory.State = "idle"
}

// depositDirectedDropOff places wc.DropOffItem at the destination chosen by
// the player. For a storage destination the item is added to the container's
// items list; for a bare ground tile the item is placed on the tile (and any
// entity with a Worker component is registered with the carrier's settlement
// and ticked from the live entities list — the "deploy" path for robots).
func depositDirectedDropOff(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent) {
	item := wc.DropOffItem
	dest := wc.DropOffDestEntity
	dx, dy, dz := wc.DropOffX, wc.DropOffY, wc.DropOffZ

	if item == nil {
		wc.DropOffDestEntity = nil
		wc.DropOffX, wc.DropOffY, wc.DropOffZ = 0, 0, 0
		return
	}
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)

	if dest != nil && dest.HasComponent(components.Storage) {
		storageC := dest.GetComponent(components.Storage).(*components.StorageComponent)
		if !storageC.AddItem(item) {
			// Container refused (e.g. tag-filter changed mid-walk); leave the
			// destination intact so the worker doesn't pivot to ground-drop on
			// the next tick — the player needs a real refusal, not a silent
			// fallback. Clear DropOffItem so the dropoff state exits.
			log.Printf("[DROP] %s refused %s — staying in bag", dest.Blueprint, item.Blueprint)
			wc.DropOffItem = nil
			wc.DropOffDestEntity = nil
			wc.DropOffX, wc.DropOffY, wc.DropOffZ = 0, 0, 0
			return
		}
		inv.RemoveItem(item)
		log.Printf("[DROP] deposited %s in %s", item.Blueprint, dest.Blueprint)
		if item.HasComponent(rlcomponents.Description) {
			dc := item.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
			level.QueueEvent(eventsystem.ItemStoredEvent{
				ItemName:   dc.Name,
				Blueprint:  item.Blueprint,
				Settlement: storageC.OwnedBy,
			})
		}
		wc.DropOffItem = nil
		wc.DropOffDestEntity = nil
		wc.DropOffX, wc.DropOffY, wc.DropOffZ = 0, 0, 0
		return
	}

	// Ground drop: take the item out of the bag, give it a position at the
	// drop tile, and register it with the level. AddEntity routes to
	// StaticEntities for Inanimate items and to Entities for everything else
	// (e.g. a robot — which then gets ticked next round).
	log.Printf("[DROP] ground-drop %s at (%d,%d,%d)", item.Blueprint, dx, dy, dz)
	wc.DropOffItem = nil
	wc.DropOffDestEntity = nil
	wc.DropOffX, wc.DropOffY, wc.DropOffZ = 0, 0, 0
	inv.RemoveItem(item)
	if item.HasComponent(rlcomponents.Position) {
		item.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent).SetPosition(dx, dy, dz)
	} else {
		item.AddComponent(&rlcomponents.PositionComponent{X: dx, Y: dy, Z: dz})
	}
	level.AddEntity(item)

	// Worker-bearing entities (robots) join the carrier's settlement and
	// activate on touchdown, matching the print_colonist branch.
	if item.HasComponent(components.Worker) && entity.HasComponent(components.Settlement) {
		sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
		if item.HasComponent(components.Settlement) {
			item.GetComponent(components.Settlement).(*components.SettlementComponent).Name = sc.Name
		} else {
			item.AddComponent(&components.SettlementComponent{Name: sc.Name})
		}
		if item.HasComponent(rlcomponents.AIMemory) {
			am := item.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
			am.State = "idle"
			am.CurrentSteps = nil
		}
		if item.HasComponent(components.Worker) {
			wcRobot := item.GetComponent(components.Worker).(*components.WorkerComponent)
			wcRobot.CurrentTask = nil
		}
		if item.HasComponent(rlcomponents.Description) {
			dc := item.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
			chatter.Post(rlentity.GetName(entity), "Deployed "+dc.Name+".")
		}
		return
	}
	if item.HasComponent(rlcomponents.Description) {
		dc := item.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
		chatter.Post(rlentity.GetName(entity), "Dropped "+dc.Name+".")
	}
}

// HandleGatherMaterialsState is the trip planner: the worker builds a shopping
// list for its task once, then visits the nearest container holding anything on
// it and takes all it can there, so one stop covers every input a chest holds. It
// returns to the task when the list is empty or the bag is full; the task handler
// delivers the load and sends it back out with a fresh list if the cost isn't met.
func HandleGatherMaterialsState(level *world.Level, entity *ecs.Entity) {
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)

	leave := func(state string) {
		wc.GatherList = nil
		wc.GatherTarget = nil
		aiMemory.State = state
	}

	if wc.CurrentTask == nil || wc.CurrentTask.Completed {
		leave("idle")
		return
	}
	cost, ok := taskCost(wc.CurrentTask)
	if !ok {
		leave("idle")
		return
	}
	if !entity.HasComponent(rlcomponents.Inventory) {
		StopTask(wc.CurrentTask)
		wc.CurrentTask = nil
		leave("idle")
		return
	}

	need := remainingCost(cost.Inputs, cost.Delivered)
	if wc.GatherList == nil {
		wc.GatherList = subtractBag(entity, need)
	}
	if len(wc.GatherList) == 0 {
		leave("task")
		return
	}

	free := carryFree(entity)
	if free == 0 {
		if checkInventoryForMaterials(entity, need) {
			leave("task")
			return
		}
		// Full of things the task can't use, so no trip would ever make progress.
		// Hand the task back and store the bag; haul empties it, then goes idle.
		StopTask(wc.CurrentTask)
		wc.CurrentTask = nil
		chatter.Post(rlentity.GetName(entity), "Bag too full to fetch materials")
		leave("haul")
		return
	}

	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if wc.GatherTarget == nil || !storageStillUsable(level, wc.GatherTarget, wc.GatherList) {
		wc.GatherTarget = FindClosestStorageWithAny(level, sc.Name, wc.GatherList, pc.GetX(), pc.GetY(), pc.GetZ())
	}
	if wc.GatherTarget == nil {
		// Stock moved mid-trip. Deliver what was already fetched rather than
		// stranding it; the next gather pass finds nothing and stops the task.
		if checkInventoryForMaterials(entity, need) {
			leave("task")
			return
		}
		StopTask(wc.CurrentTask)
		wc.CurrentTask = nil
		chatter.Post(rlentity.GetName(entity), "Insufficient materials to "+cost.Verb+" "+cost.Name)
		leave("idle")
		return
	}

	storagePC := wc.GatherTarget.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), storagePC.GetX(), storagePC.GetY(), storagePC.GetZ(), 1, 1, 0) {
		grabListedItemsNearby(level, entity, wc.GatherList)
		if len(wc.GatherList) == 0 {
			return
		}
		if moved, pathFound := MoveTowardsTarget(level, entity, storagePC.GetX(), storagePC.GetY(), storagePC.GetZ()); !moved {
			releaseTaskAfterFailedMove(wc, pathFound)
			leave("idle")
		}
		return
	}

	loadFromStorage(entity, wc.GatherTarget.GetComponent(components.Storage).(*components.StorageComponent), wc.GatherList, free)
	wc.GatherTarget = nil
}

// subtractBag returns need minus what the bag already holds, positive entries only.
func subtractBag(entity *ecs.Entity, need map[string]int) map[string]int {
	list := make(map[string]int, len(need))
	for name, qty := range need {
		list[name] = qty
	}
	for _, item := range entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag {
		if _, listed := list[item.Blueprint]; listed {
			list[item.Blueprint] -= carryUnits(item)
		}
	}
	for name, qty := range list {
		if qty <= 0 {
			delete(list, name)
		}
	}
	return list
}

// loadFromStorage moves every listed item the container holds into the bag, up to
// free stack units, and takes what it moved off the list. Blueprints are visited
// in name order so a tight bag splits a load the same way every time.
func loadFromStorage(entity *ecs.Entity, storageC *components.StorageComponent, list map[string]int, free int) {
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	names := make([]string, 0, len(list))
	for name := range list {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		take := min(list[name], storageC.CountResource(name), free)
		if take <= 0 {
			continue
		}
		taken := 0
		if storageIsStackable(storageC, name) {
			// Carry a new entity holding just this quantity; the rest stays stacked.
			material, err := factory.Create(name, 0, 0, 0)
			if err != nil {
				log.Printf("[GATHER] can't create %s: %v", name, err)
				continue
			}
			if storageC.DeductResource(name, take) {
				material.GetComponent(components.Material).(*components.MaterialComponent).Quantity = take
				inv.AddItem(material)
				taken = take
			}
		} else {
			for taken < take {
				item := storageC.TakeOne(name)
				if item == nil {
					break
				}
				inv.AddItem(item)
				taken++
			}
		}
		free -= taken
		if list[name] -= taken; list[name] <= 0 {
			delete(list, name)
		}
	}
}

func storageIsStackable(storageC *components.StorageComponent, blueprint string) bool {
	for _, item := range storageC.PocketItems() {
		if item.Blueprint == blueprint && item.HasComponent(components.Material) {
			return true
		}
	}
	return false
}

// HandleHaulState drives the haul state: pick up a loose item on the ground
// and deliver it to the nearest available storage.
func HandleHaulState(level *world.Level, entity *ecs.Entity) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)

	// If holding something, go drop it off. We pick a container that accepts
	// the FIRST bag item; bag[0] is guaranteed to be deposited (Accepts is the
	// only gate in AddItem). Subsequent items the chosen container rejects
	// stay in the bag and get a fresh container lookup on the next tick —
	// since bag[0] was removed, FindAvailableStorageFor sees the new head.
	//
	// Safety net: if a filter changes mid-trip and ZERO items deposit (e.g.
	// bag[0] is no longer accepted by anyone matching), drop the bag on the
	// current tile so workers don't get permanently stuck carrying.
	if len(inv.Bag) > 0 {
		storage := FindAvailableStorageFor(level, sc.Name, inv.Bag[0])
		if storage == nil {
			aiMemory.State = "idle"
			return
		}
		spc := storage.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if moved, _ := MoveTowardsTarget(level, entity, spc.GetX(), spc.GetY(), spc.GetZ()); !moved {
			storageC := storage.GetComponent(components.Storage).(*components.StorageComponent)
			before := len(inv.Bag)
			kept := inv.Bag[:0]
			for _, item := range inv.Bag {
				if !storageC.AddItem(item) {
					kept = append(kept, item)
				}
			}
			inv.Bag = kept
			if len(inv.Bag) == before {
				// Nothing was accepted — filter must have changed between
				// target pick and arrival. Drop where we stand rather than
				// looping.
				dropBagOnTile(level, inv, pc.GetX(), pc.GetY(), pc.GetZ())
				chatter.Post(rlentity.GetName(entity), "Dropped load (rejected)")
			} else {
				chatter.Post(rlentity.GetName(entity), "Stored items")
			}
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

	if moved, _ := MoveTowardsTarget(level, entity, aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ); !moved {
		// Pick up any items at this tile
		var tileEntities []*ecs.Entity
		level.GetEntitiesAt(aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ, &tileEntities)
		picked := false
		for _, e := range tileEntities {
			if e.HasComponent(rlcomponents.Item) && e.HasComponent(rlcomponents.Position) {
				inv.AddItem(e)
				level.RemoveEntity(e)
				chatter.Post(rlentity.GetName(entity), fmt.Sprintf("Picked up %s", e.Blueprint))
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

// dropBagOnTile empties a worker's bag onto the level at (x,y,z) — used as a
// safety net by HandleHaulState when the chosen destination accepts nothing.
// Each loose item gets its Position updated; level.AddEntity routes it to
// Entities / StaticEntities based on the Inanimate component as usual.
func dropBagOnTile(level *world.Level, inv *rlcomponents.InventoryComponent, x, y, z int) {
	bag := append([]*ecs.Entity{}, inv.Bag...)
	inv.Bag = inv.Bag[:0]
	for _, item := range bag {
		if item == nil {
			continue
		}
		if item.HasComponent(rlcomponents.Position) {
			pc := item.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			pc.SetPosition(x, y, z)
		} else {
			item.AddComponent(&rlcomponents.PositionComponent{X: x, Y: y, Z: z})
		}
		level.AddEntity(item)
	}
}

// HandleFindFood drives the find_food state: eat from inventory first,
// otherwise walk to the nearest storage with food and consume it there.
func HandleFindFood(level *world.Level, entity *ecs.Entity) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)

	// Eat from own inventory first before walking to storage. A settlement's
	// diet (the hive's biomass) is eaten a unit of a stack at a time.
	diet := dietOf(entity)
	if entity.HasComponent(rlcomponents.Inventory) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for i, item := range inv.Bag {
			amount, ok := edible(item, diet)
			if !ok {
				continue
			}
			if entity.HasComponent(rlcomponents.Health) {
				hc := entity.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
				hc.Energy += amount
			}
			if entity.HasComponent(components.Needs) {
				nc := entity.GetComponent(components.Needs).(*components.NeedsComponent)
				nc.Eat(amount)
			}
			if eatOne(item) {
				inv.Bag = append(inv.Bag[:i], inv.Bag[i+1:]...)
			}
			chatter.Post(rlentity.GetName(entity), "Ate "+item.Blueprint)
			aiMemory.TargetX = -1
			aiMemory.TargetY = -1
			aiMemory.State = "idle"
			return
		}
	}

	if aiMemory.TargetX == -1 && aiMemory.TargetY == -1 {
		var storage *ecs.Entity
		if len(diet) > 0 {
			storage = closestStorageWithDiet(level, sc.Name, diet, pc.GetX(), pc.GetY())
		} else {
			storage = FindClosestStorageWithComponent(level, sc.Name, rlcomponents.Food, pc.GetX(), pc.GetY(), pc.GetZ())
		}
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

	if moved, _ := MoveTowardsTarget(level, entity, aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ); !moved {
		var tileEntities []*ecs.Entity
		level.GetEntitiesAt(aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ, &tileEntities)
		for _, e := range tileEntities {
			if !e.HasComponent(components.Storage) {
				continue
			}
			storageC := e.GetComponent(components.Storage).(*components.StorageComponent)
			foodEntity, amount := takeEdible(storageC, diet)
			if foodEntity != nil {
				hc := entity.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
				hc.Energy += amount
				if entity.HasComponent(components.Needs) {
					nc := entity.GetComponent(components.Needs).(*components.NeedsComponent)
					nc.Eat(amount)
				}
				chatter.Post(rlentity.GetName(entity), "Ate "+foodEntity.Blueprint)
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

// HandleFindBedroll drives the find_bedroll state: walk to the nearest storage
// holding a sleeping bag and take it into the bag. Once carried it's kept, so
// the colonist can rest anywhere (see NeedsSystem's rest logic).
func HandleFindBedroll(level *world.Level, entity *ecs.Entity) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)
	// The state persists on AIMemory across ticks/beams, so the colonist may
	// have lost its settlement (e.g. re-tagged on beam) since entering it.
	if !entity.HasComponent(components.Settlement) {
		aiMemory.TargetX, aiMemory.TargetY = -1, -1
		aiMemory.State = "idle"
		return
	}
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)

	if aiMemory.TargetX == -1 && aiMemory.TargetY == -1 {
		storage := FindClosestStorageWith(level, sc.Name, "sleeping_bag", pc.GetX(), pc.GetY(), pc.GetZ())
		if storage == nil {
			aiMemory.TargetX, aiMemory.TargetY = 0, 0
			aiMemory.State = "idle"
			return
		}
		spc := storage.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ = spc.GetX(), spc.GetY(), spc.GetZ()
		return
	}

	if moved, _ := MoveTowardsTarget(level, entity, aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ); !moved {
		var tileEntities []*ecs.Entity
		level.GetEntitiesAt(aiMemory.TargetX, aiMemory.TargetY, aiMemory.TargetZ, &tileEntities)
		for _, e := range tileEntities {
			if !e.HasComponent(components.Storage) {
				continue
			}
			storageC := e.GetComponent(components.Storage).(*components.StorageComponent)
			if bag := storageC.TakeOne("sleeping_bag"); bag != nil && entity.HasComponent(rlcomponents.Inventory) {
				inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
				inv.AddItem(bag)
				chatter.Post(rlentity.GetName(entity), "Grabbed a sleeping bag")
			}
			break
		}
		aiMemory.TargetX, aiMemory.TargetY = -1, -1
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
	tech, found := getTech(rr.TechKey)
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

// isActionAllowed reports whether action is currently enabled for the
// worker. The factory seeds AllowedTasks from AvailableTasks at create
// time, so checking AllowedTasks alone is sufficient — the chassis cap
// is already baked in.
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
