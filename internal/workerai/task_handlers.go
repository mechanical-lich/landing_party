package workerai

import (
	"github.com/mechanical-lich/landing_party/internal/chatter"
	"log"
	"strings"
	"time"

	"github.com/mechanical-lich/landing_party/internal/audio"
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
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
	"github.com/mechanical-lich/mlge/utility"
)

const sleepRecoveryPerCycle = 10

// Cost registry lookups, vars so tests can supply definitions: the registries read
// data/ relative to the working directory at package init.
var (
	getBuildable = construction.GetBuildable
	getRecipe    = crafting.GetRecipe
	getTech      = research.GetTech
)

// SetCostLookups replaces the cost registry lookups and returns a func restoring
// them, for tests in packages whose working directory hid data/ at package init.
func SetCostLookups(buildable func(string) construction.Buildable, recipe func(string) (crafting.Recipe, bool), tech func(string) (research.Tech, bool)) (restore func()) {
	prevB, prevR, prevT := getBuildable, getRecipe, getTech
	getBuildable, getRecipe, getTech = buildable, recipe, tech
	return func() { getBuildable, getRecipe, getTech = prevB, prevR, prevT }
}

// maxApproachFailTicks cancels an equip/unequip task after this many consecutive
// ticks of failing to advance toward its storage container, so a worker whose
// storage is unreachable doesn't stay pinned to the task forever.
const maxApproachFailTicks = 5

func CompleteTaskWithMessage(entity *ecs.Entity, t *task.Task, msg string) {
	t.Complete()
	wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)
	wc.CurrentTask = nil
	if entity.HasComponent(components.Settlement) &&
		strings.Contains(entity.GetComponent(components.Settlement).(*components.SettlementComponent).Name, ":") {
		return // a faction settlement's ("<faction>:<loc>") work is not the player's chatter
	}
	chatter.Post(rlentity.GetName(entity), msg)
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

// sameSettlement reports whether b is a's settlement's (or has none): a
// worker of another settlement (a faction's) is never picked up.
func sameSettlement(a, b *ecs.Entity) bool {
	if !b.HasComponent(components.Settlement) {
		return true
	}
	if !a.HasComponent(components.Settlement) {
		return false
	}
	return a.GetComponent(components.Settlement).(*components.SettlementComponent).Name ==
		b.GetComponent(components.Settlement).(*components.SettlementComponent).Name
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
		if candidate.HasComponent(rlcomponents.Health) && candidate.HasComponent(rlcomponents.HostileAI) || components.IsHostileWorker(candidate) {
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
		// Deployed Worker entities (recalled robots) are pickup targets too,
		// not just rlcomponents.Item-tagged objects.
		if candidate.HasComponent(rlcomponents.Item) || candidate.HasComponent(components.Worker) && sameSettlement(entity, candidate) {
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
		if world.IsDepositTileName(tileName) {
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

	if moved, pathFound := MoveTowardsTarget(level, entity, tx, ty, tz); !moved {
		if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), tx, ty, tz, 1, 1, 0) {
			releaseTaskAfterFailedMove(wc, pathFound)
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
				// Prefer the explicit Dest from the Store order; otherwise fall
				// back to the first colony locker that accepts the item.
				dest := req.Dest
				if dest != nil {
					if !dest.HasComponent(components.Storage) || !dest.HasComponent(rlcomponents.Position) {
						dest = nil
					} else {
						destSc := dest.GetComponent(components.Storage).(*components.StorageComponent)
						if !destSc.Accepts(e) {
							// Filter changed between order and pickup — fall through to default.
							dest = nil
						}
					}
				}
				if dest == nil {
					dest = FindAvailableStorageFor(level, sc.Name, e)
				}
				if dest != nil {
					spc := dest.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
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

	// Walk to storage and deposit — pick one that accepts the unequipped item.
	storage := FindAvailableStorageFor(level, sc.Name, item)
	if storage == nil {
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "dropoff"
		return
	}

	storagePC := storage.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	moved, _ := MoveTowardsTarget(level, entity, storagePC.GetX(), storagePC.GetY(), storagePC.GetZ())
	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), storagePC.GetX(), storagePC.GetY(), storagePC.GetZ(), 1, 1, 0) {
		wc.ApproachFailTicks = 0
		storageC := storage.GetComponent(components.Storage).(*components.StorageComponent)
		for _, bagItem := range append([]*ecs.Entity{}, inv.Bag...) {
			if storageC.AddItem(bagItem) {
				inv.RemoveItem(bagItem)
			}
		}
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
		return
	}
	if moved {
		wc.ApproachFailTicks = 0
		return
	}
	wc.ApproachFailTicks++
	if wc.ApproachFailTicks >= maxApproachFailTicks {
		wc.ApproachFailTicks = 0
		CompleteTaskWithMessage(entity, wc.CurrentTask, "Can't reach storage — cancelled stowing gear")
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

	// Gear the colonist already carries just equips in place. Consumables fall
	// through to fetch another from storage so the player can stock up.
	for _, item := range inv.Bag {
		if item.Blueprint == req.ItemBlueprint && components.ItemIsGear(item) {
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
	moved, _ := MoveTowardsTarget(level, entity, storagePC.GetX(), storagePC.GetY(), storagePC.GetZ())
	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), storagePC.GetX(), storagePC.GetY(), storagePC.GetZ(), 1, 1, 0) {
		wc.ApproachFailTicks = 0
		storageC := storageEntity.GetComponent(components.Storage).(*components.StorageComponent)
		item := storageC.TakeOne(req.ItemBlueprint)
		if item != nil {
			if components.ItemIsGear(item) {
				inv.Equip(item)
			} else {
				inv.AddItem(item) // carry consumables (rations, sleeping bags…) in the bag
			}
		}
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
		return
	}
	if moved {
		wc.ApproachFailTicks = 0
		return
	}
	wc.ApproachFailTicks++
	if wc.ApproachFailTicks >= maxApproachFailTicks {
		wc.ApproachFailTicks = 0
		CompleteTaskWithMessage(entity, wc.CurrentTask, "Can't reach storage — cancelled equipping "+req.ItemBlueprint)
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

	recipe, found := getRecipe(cr.RecipeID)
	if !found {
		log.Printf("[CRAFT] recipe not found: %s", cr.RecipeID)
		StopTask(wc.CurrentTask)
		wc.CurrentTask = nil
		aiMemory.State = "idle"
		return
	}

	t := wc.CurrentTask
	delivered, paid := deliverMaterials(level, entity, wc, aiMemory, recipe.Cost, cr.Delivered, cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ)
	cr.Delivered = delivered
	t.Data = cr
	if !paid {
		return
	}

	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ, 1, 1, 0) {
		if moved, pathFound := MoveTowardsTarget(level, entity, cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ); !moved {
			log.Printf("[CRAFT] %s can't reach workbench at (%d,%d,%d) from (%d,%d,%d)",
				rlentity.GetName(entity), cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ,
				pc.GetX(), pc.GetY(), pc.GetZ())
			releaseTaskAfterFailedMove(wc, pathFound)
			aiMemory.State = "idle"
		}
		return
	}

	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
	if recipe.RequiresTech != "" && !SettlementKnowsTech(sc.Name, recipe.RequiresTech) {
		CompleteTaskWithMessage(entity, wc.CurrentTask, "Can't craft "+recipe.Name+" yet")
		aiMemory.State = "idle"
		return
	}
	var station *ecs.Entity
	if recipe.ConsumeStation {
		station = stationAt(level, recipe.Station, cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ)
		if station == nil {
			// The grey egg is gone (eaten by another order, killed): the
			// materials stay in the bag and go back to storage.
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Nothing left to work for "+recipe.Name)
			aiMemory.State = "idle"
			return
		}
	}
	cr.Progress += workStep(wc, entity, "Int")
	wc.CurrentTask.Data = cr
	if cr.Progress >= cr.Required {
		if recipe.KinType != "" {
			// A typed output (an egg of the crafter's hive): its type,
			// presence and settlement, laid in the used-up station's place.
			presence := ""
			if k := components.KinOf(entity); k != nil {
				presence = k.Presence
			}
			if station != nil {
				level.RemoveEntity(station)
			}
			if out, err := factory.Create(recipe.Output, cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ); err == nil {
				if recipe.DeployOnCraft && !recipe.ConsumeStation {
					// A printed unit (the autonz'; an egg laid in a grey
					// egg's place is not one): a worker of the crafter's
					// settlement, by the station, before its type gives it
					// its role and tasks.
					deployWorker(out, sc.Name)
				}
				factory.ApplyType(out, recipe.KinType, presence)
				components.KinOf(out).Settlement = sc.Name
				level.AddEntity(out)
			}
			progression.AwardXP(entity, "Int", progression.XPPerTask)
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Crafted "+recipe.Name)
			aiMemory.State = "idle"
			return
		}
		if station != nil {
			level.RemoveEntity(station)
		}
		output, err := factory.Create(recipe.Output, cr.WorkbenchX, cr.WorkbenchY, cr.WorkbenchZ)
		if err == nil && recipe.Count > 1 && output.HasComponent(components.Material) {
			output.GetComponent(components.Material).(*components.MaterialComponent).Quantity = recipe.Count
		}
		if err == nil {
			if recipe.DeployOnCraft {
				// A deployed unit (a bio-printed colonist) joins the crafter's
				// settlement at the station rather than going into the
				// crafter's inventory.
				deployWorker(output, sc.Name)
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

	tech, techFound := getTech(rr.TechKey)
	delivered, paid := deliverMaterials(level, entity, wc, aiMemory, researchInputs(rr, tech), rr.Delivered, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z)
	rr.Delivered = delivered
	if !paid {
		return
	}

	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z, 1, 1, 0) {
		if moved, pathFound := MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z); !moved {
			releaseTaskAfterFailedMove(wc, pathFound)
			aiMemory.State = "idle"
		}
		return
	}

	rr.Progress += workStep(wc, entity, "Int")
	if rr.Progress >= rr.Required {
		who := ""
		if entity.HasComponent(components.Settlement) {
			who = entity.GetComponent(components.Settlement).(*components.SettlementComponent).Name
		}
		level.QueueEvent(eventsystem.ResearchDoneEvent{TechKey: rr.TechKey, Settlement: who})
		if !strings.Contains(who, ":") { // not a faction settlement ("<faction>:<loc>")
			audio.PlayGlobal("research_done") // milestone chime, heard on any level
		}
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
			if moved, pathFound := MoveTowardsTarget(level, entity, sr.X, sr.Y, sr.Z); !moved {
				releaseTaskAfterFailedMove(wc, pathFound)
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
	if pr.Bedroll {
		emotes.Set(entity, emotes.Sleep, 2, 2)
	} else {
		emotes.Set(entity, emotes.Sleeps, 2, 2)
	}
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

	msg := "Woke up after passing out"
	if pr.Bedroll {
		msg = "Woke up, rested in a sleeping bag"
	}
	CompleteTaskWithMessage(entity, wc.CurrentTask, msg)
	aiMemory.State = "idle"
}

// WakeFromRest ends a sleep or passout task early, climbing off the bed if the
// worker had reached it. Exhaustion stays as it is.
func WakeFromRest(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent) {
	if sr, ok := wc.CurrentTask.Data.(*task_requests.SleepRequest); ok && sr.OnBed {
		dismountSleeper(level, entity, sr)
	}
	wc.CurrentTask = nil
}

// digSoundLoudness is the emitted loudness for a dig/mine "break". Modest — a
// pick strike carries a few tiles for AI hearing, and the audio bridge plays it
// positionally near the camera.
const digSoundLoudness float32 = 5

// chopSwingProgress is how much harvest progress passes between chop "swing"
// impact sounds. Tied to work done (not frame rate), so chopping a tougher node
// simply produces more swings; a rock (Required = Health*10 = 30) yields ~5.
const chopSwingProgress = 6

func handleMineTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	req, ok := wc.CurrentTask.Data.(task_requests.MineRequest)
	if !ok {
		aiMemory.State = "idle"
		return
	}

	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z, 1, 1, 0) {
		prevProgress := req.Progress
		req.Progress += workStep(wc, entity, "Str")
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
			// A swing impact every chopSwingProgress of work — composed from the
			// chopper's weight and the target's surface (a glass tink for crystal,
			// a generic thud for rock) — so harvesting sounds active, not just the
			// shatter/crumble on the final blow.
			if prevProgress/chopSwingProgress != req.Progress/chopSwingProgress {
				impactKey := components.ImpactClipKey(entity, req.Target)
				level.EmitSoundClip(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z, digSoundLoudness, world.SoundTagImpact, impactKey, entity)
			}
			if req.Progress >= req.Required {
				req.Target.AddComponent(&rlcomponents.DeadComponent{})
				CompleteTaskWithMessage(entity, wc.CurrentTask, "Harvested")
				aiMemory.State = "idle"
			}
			return
		}

		// Tile branch — a deposit holds a rolled richness in level.ResourceAmount.
		// Extract one stack every mineChunkTicks of progress, decrementing the
		// stored amount (the single source of truth), and clear the tile once the
		// deposit is empty. Because extraction draws down the persistent amount,
		// cancelling and re-issuing a mine order resumes from what's left rather
		// than re-rolling — no infinite-ore exploit. Drops land on a standable
		// tile next to the miner (never the solid deposit), spreading out across
		// neighbours as they pile up so haulers can reach each one.
		const (
			mineChunkTicks = 3 // progress ticks per extracted unit
			mineChunkSize  = 1 // units per stack (drop one resource at a time as mined)
		)
		tx, ty, tz := wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z
		tile := level.GetTileAt(tx, ty, tz).(*world.Tile)
		tileName := ""
		if !tile.Middle.IsEmpty() {
			tileName = world.TileDefinitions[tile.Middle.Type].Name
		}
		dropBlueprint := world.DepositDrop(tileName)

		// Seed richness lazily if a deposit has no entry (legacy save, or a tile
		// placed outside the feature placers) so every deposit is minable.
		if dropBlueprint != "" && level.ResourceAmountAt(tx, ty, tz) <= 0 {
			level.SetResourceAmount(tx, ty, tz, world.RollDepositRichness(tileName))
		}

		extracted := false
		for milestone := prevProgress/mineChunkTicks + 1; dropBlueprint != "" && milestone <= req.Progress/mineChunkTicks; milestone++ {
			got := level.ConsumeResource(tx, ty, tz, mineChunkSize)
			if got <= 0 {
				break
			}
			extracted = true
			dropX, dropY, dropZ := dropTileNear(level, pc.GetX(), pc.GetY(), pc.GetZ())
			ore, err := factory.Create(dropBlueprint, dropX, dropY, dropZ)
			if err == nil {
				if ore.HasComponent(components.Material) {
					ore.GetComponent(components.Material).(*components.MaterialComponent).Quantity = got
				}
				level.AddEntity(ore)
				queueRetrieveTask(entity, ore, dropX, dropY, dropZ)
			}
		}

		// A "chunk" thunk each time ore is pulled; for non-deposit solid rock
		// (no chunks) the mined-out clear below emits a single break instead.
		if extracted {
			level.EmitSoundClip(tx, ty, tz, digSoundLoudness, world.SoundTagBreak, components.MiningClipKey(level.MiddleMaterialAt(tx, ty, tz)), entity)
		}

		// Mined out once the deposit is empty (or the tile was never a deposit).
		if dropBlueprint == "" || level.ResourceAmountAt(tx, ty, tz) <= 0 {
			if !extracted {
				level.EmitSoundClip(tx, ty, tz, digSoundLoudness, world.SoundTagBreak, components.MiningClipKey(level.MiddleMaterialAt(tx, ty, tz)), entity)
			}
			clearTileRadiation(tile)
			// Layered mine: just remove the Middle. Floor stays.
			level.ClearMiddle(tx, ty, tz)
			level.SetResourceAmount(tx, ty, tz, 0) // drop any leftover entry
			level.InvalidateSunColumn(tx, ty)
			progression.AwardXP(entity, "Str", progression.XPPerTask)
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Mined out deposit")
			aiMemory.State = "idle"
		}
	} else {
		if moved, pathFound := MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z); !moved {
			releaseTaskAfterFailedMove(wc, pathFound)
		}
	}
}

// dropTileNear picks where a mined resource lands: the first empty, standable
// tile adjacent to (cx,cy,cz) — so successive drops spread out instead of
// stacking on the miner — falling back to an occupied standable neighbour, and
// finally the miner's own tile if none is standable.
func dropTileNear(level *world.Level, cx, cy, cz int) (int, int, int) {
	standable := func(x, y int) bool {
		t, ok := level.GetTileAt(x, y, cz).(*world.Tile)
		if !ok || t == nil || t.Floor.IsEmpty() {
			return false
		}
		if !t.Middle.IsEmpty() {
			if def := world.TileDefinitions[t.Middle.Type]; def.Solid || def.Water {
				return false
			}
		}
		return true
	}
	fx, fy, haveFallback := 0, 0, false
	for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		x, y := cx+d[0], cy+d[1]
		if !standable(x, y) {
			continue
		}
		if level.GetEntityAt(x, y, cz) == nil {
			return x, y, cz // empty standable neighbour
		}
		if !haveFallback {
			fx, fy, haveFallback = x, y, true
		}
	}
	if haveFallback {
		return fx, fy, cz
	}
	return cx, cy, cz
}

func handlePickupTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	// Re-resolve the target's current position each tick. A deployed
	// Worker target (e.g. a robot) can shift one tile via the colonist
	// swap rule when we try to step onto it, so the original Task.X/Y is
	// stale by the time we arrive. Falling back to the task tile keeps
	// the legacy "pick up whatever's on this tile" semantics for items.
	tx, ty, tz := wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z
	target, _ := wc.CurrentTask.Data.(*ecs.Entity)
	if target != nil && target.HasComponent(rlcomponents.Position) {
		tpc := target.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		tx, ty, tz = tpc.GetX(), tpc.GetY(), tpc.GetZ()
	}
	if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), tx, ty, tz, 1, 1, 0) {
		moved, pathFound := MoveTowardsTarget(level, entity, tx, ty, tz)
		if moved {
			wc.InteractTicks = 0
			return
		}
		releaseTaskAfterFailedMove(wc, pathFound)
		wc.InteractTicks = 0
		return
	}
	const pickupHoldTicks = 6
	wc.InteractTicks++
	if wc.InteractTicks < pickupHoldTicks {
		return
	}
	// Pick up the specific target entity if we still have a handle on it
	// (it may have shifted tiles via swap). Otherwise fall through to the
	// tile-based pickup used by autonomous retrievals.
	if target != nil && target.HasComponent(rlcomponents.Position) {
		tpc := target.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		PickupItemFromTile(level, entity, tpc.GetX(), tpc.GetY(), tpc.GetZ())
	} else {
		PickupItemFromTile(level, entity, tx, ty, tz)
	}
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
			level.QueueEvent(eventsystem.EntityKilledEvent{
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
		combat.Hit(level, entity, target, true)
		rlentity.Face(entity, targetPC.GetX()-pc.GetX(), targetPC.GetY()-pc.GetY())
		if target.HasComponent(rlcomponents.Dead) {
			level.QueueEvent(eventsystem.EntityKilledEvent{
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
	if moved, _ := MoveTowardsTarget(level, entity, tx, ty, tz); moved {
		return
	}
	// Arrived. A hold (a director's: the hive's shot waiting by its spire)
	// stands there until it is released.
	if wc.CurrentTask.Data == task_requests.HoldHere {
		return
	}
	if wc.CurrentTask.Action == task_requests.MoveAction {
		// A director's walk (a post, a waypoint): arriving is all of it. The
		// tile's interactions (picking up what stands there) are the
		// player's Move Here's.
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
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

	buildable := getBuildable(buildRequest.Type)
	t := wc.CurrentTask
	delivered, paid := deliverMaterials(level, entity, wc, aiMemory, buildable.Cost, buildRequest.Delivered, t.X, t.Y, t.Z)
	buildRequest.Delivered = delivered
	t.Data = buildRequest
	if !paid {
		return
	}

	if rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z, 1, 1, 0) {
		// Progress lives in the request data so multiple workers share the same counter
		buildRequest.Progress += workStep(wc, entity, "Str")
		wc.CurrentTask.Data = buildRequest
		if buildRequest.Progress >= buildRequest.Required {
			sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
			if buildRequest.Replace {
				replaceRepairable(level, buildRequest.X, buildRequest.Y, buildRequest.Z)
			}
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
									b := level.GetSolidEntityAt(startX+dx, startY+dy, buildRequest.Z)
									if b == nil {
										continue
									}
									if b.HasComponent(components.Worker) {
										// Someone is standing in the footprint (a
										// worker crossing a bed's second tile, or the
										// builder itself): rather than finish with
										// nothing placed and the materials gone, hold
										// the work at done and wait. The builder steps
										// out of the footprint first.
										buildRequest.Progress = buildRequest.Required
										if b == entity {
											wc.CurrentTask.Data = buildRequest
											stepOutOfFootprint(level, entity, startX, startY, sizec.Width, sizec.Height, buildRequest.X, buildRequest.Y, buildRequest.Z)
											return
										}
										buildRequest.Waited++
										if buildRequest.Waited > maxFootprintWait {
											// Someone who never moves (a rival's
											// sentry): back to the queue, its
											// materials kept, for a later try.
											buildRequest.Waited = 0
											wc.CurrentTask.Data = buildRequest
											if !strings.Contains(sc.Name, ":") {
												chatter.Post(rlentity.GetName(entity), "Someone is standing where the "+buildable.Name+" goes") // DRAFT
											}
											StopTask(wc.CurrentTask)
											wc.CurrentTask = nil
											aiMemory.State = "idle"
											return
										}
										wc.CurrentTask.Data = buildRequest
										return
									}
									canPlace = false
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
			level.QueueEvent(eventsystem.StructureBuiltEvent{
				Type: buildRequest.Type, Settlement: sc.Name,
				X: buildRequest.X, Y: buildRequest.Y, Z: buildRequest.Z,
			})
			progression.AwardXP(entity, "Str", progression.XPPerTask)
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Built "+buildRequest.Type)
			aiMemory.State = "idle"
		}
	} else {
		if moved, pathFound := MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z); !moved {
			releaseTaskAfterFailedMove(wc, pathFound)
		}
	}
}

// stepOutOfFootprint moves a builder standing inside the w x h footprint at
// (sx, sy) one step to a free tile outside it, still beside the build site
// (tx, ty), so the structure can be placed (a walk, not a teleport).
func stepOutOfFootprint(level *world.Level, entity *ecs.Entity, sx, sy, w, h, tx, ty, z int) {
	in := func(x, y int) bool { return x >= sx && x < sx+w && y >= sy && y < sy+h }
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			x, y := pc.GetX()+dx, pc.GetY()+dy
			if (dx == 0 && dy == 0) || in(x, y) || !level.InBounds(x, y, z) {
				continue
			}
			if utility.Abs(x-tx) > 1 || utility.Abs(y-ty) > 1 || level.GetSolidEntityAt(x, y, z) != nil {
				continue
			}
			if moved, _ := MoveTowardsTarget(level, entity, x, y, z); moved {
				return
			}
		}
	}
}

// replaceRepairable removes the broken machine a repair replaces, setting
// the level flag it names (Repairable.SetsFlag).
func replaceRepairable(level *world.Level, x, y, z int) {
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range append([]*ecs.Entity(nil), list...) {
			if e == nil || !e.HasComponent(components.Repairable) || !e.HasComponent(rlcomponents.Position) {
				continue
			}
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			if pc.GetX() != x || pc.GetY() != y || pc.GetZ() != z {
				continue
			}
			if f := e.GetComponent(components.Repairable).(*components.RepairableComponent).SetsFlag; f != "" {
				level.Flags[f] = 1.0
			}
			level.RemoveEntity(e)
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
		req.Progress += workStep(wc, entity, "Str")
		wc.CurrentTask.Data = req
		if req.Progress >= req.Required {
			level.EmitSoundClip(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z, digSoundLoudness, world.SoundTagBreak, components.MiningClipKey(level.MiddleMaterialAt(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z)), entity)
			clearTileRadiation(tile)
			// Layered dig: just remove the Middle. Floor stays as whatever
			// was there.
			level.ClearMiddle(wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z)
			level.InvalidateSunColumn(wc.CurrentTask.X, wc.CurrentTask.Y)
			progression.AwardXP(entity, "Str", progression.XPPerTask)
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Cleared debris")
			aiMemory.State = "idle"
		}
	} else {
		if moved, pathFound := MoveTowardsTarget(level, entity, wc.CurrentTask.X, wc.CurrentTask.Y, wc.CurrentTask.Z); !moved {
			releaseTaskAfterFailedMove(wc, pathFound)
		}
	}
}

// handleRelocateTask moves a quantity of one material from a source container
// to either a destination container or a ground tile. Phases are driven by
// req.PickedUp: false → walk to source and withdraw; true → walk to dest and
// deposit.
func handleRelocateTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)

	req, ok := wc.CurrentTask.Data.(task_requests.RelocateRequest)
	if !ok {
		wc.CurrentTask.Complete()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
		return
	}

	if !req.PickedUp {
		// Phase 1: walk to source and withdraw.
		if req.Source == nil || !req.Source.HasComponent(components.Storage) {
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocate: source missing")
			aiMemory.State = "idle"
			return
		}
		srcPC := req.Source.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		sx, sy, sz := srcPC.GetX(), srcPC.GetY(), srcPC.GetZ()
		if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), sx, sy, sz, 1, 1, 0) {
			if moved, pathFound := MoveTowardsTarget(level, entity, sx, sy, sz); !moved {
				releaseTaskAfterFailedMove(wc, pathFound)
				aiMemory.State = "idle"
			}
			return
		}
		srcSc := req.Source.GetComponent(components.Storage).(*components.StorageComponent)
		// "Take what's there" — clamp to whatever the source still has.
		take := req.Qty
		if have := srcSc.CountResource(req.Blueprint); have < take {
			take = have
		}
		if take <= 0 {
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocate: nothing to take")
			aiMemory.State = "idle"
			return
		}
		// Non-stackable Material entities (e.g. a stowed robot) must be
		// carried as the actual stored instance to preserve per-instance
		// state — Settlement, AIMemory, Health damage. The mint-from-
		// blueprint path below would re-create a fresh entity and reset
		// all of that. Stackable Material falls through to the mint path
		// since stacks are fungible by definition.
		if isNonStackableBlueprint(srcSc, req.Blueprint) {
			carried := 0
			for ; carried < take; carried++ {
				taken := srcSc.TakeOne(req.Blueprint)
				if taken == nil {
					break
				}
				inv.AddItem(taken)
			}
			if carried == 0 {
				CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocate: withdraw failed")
				aiMemory.State = "idle"
				return
			}
			req.PickedUp = true
			req.Carried = carried
			wc.CurrentTask.Data = req
			return
		}
		if !srcSc.DeductResource(req.Blueprint, take) {
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocate: withdraw failed")
			aiMemory.State = "idle"
			return
		}
		item, err := factory.Create(req.Blueprint, 0, 0, 0)
		if err != nil {
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocate: unknown material")
			aiMemory.State = "idle"
			return
		}
		if item.HasComponent(components.Material) {
			item.GetComponent(components.Material).(*components.MaterialComponent).Quantity = take
		}
		inv.AddItem(item)
		req.PickedUp = true
		req.Carried = take
		wc.CurrentTask.Data = req
		return
	}

	// Phase 2: walk to dest and deposit.
	var dx, dy, dz int
	if req.DestEntity != nil {
		if !req.DestEntity.HasComponent(rlcomponents.Position) {
			// Dest entity lost its position somehow — fall back to drop at task X/Y/Z.
			dx, dy, dz = req.DestX, req.DestY, req.DestZ
		} else {
			dpc := req.DestEntity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			dx, dy, dz = dpc.GetX(), dpc.GetY(), dpc.GetZ()
		}
	} else {
		dx, dy, dz = req.DestX, req.DestY, req.DestZ
	}
	if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), dx, dy, dz, 1, 1, 0) {
		if moved, _ := MoveTowardsTarget(level, entity, dx, dy, dz); !moved {
			// Can't reach — drop the load wherever we ended up so it isn't lost.
			dropMaterialAt(level, inv, req.Blueprint, req.Carried, pc.GetX(), pc.GetY(), pc.GetZ())
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocate: blocked, dropped load")
			aiMemory.State = "idle"
			return
		}
		return
	}

	// Find the carried payload in the bag. Non-Material entities (e.g. a
	// stowed robot) are carried as themselves rather than as a Material
	// stack, so don't require the Material component here.
	var carried *ecs.Entity
	for _, item := range inv.Bag {
		if item.Blueprint == req.Blueprint {
			carried = item
			break
		}
	}
	if carried == nil {
		CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocate: load lost")
		aiMemory.State = "idle"
		return
	}
	// Non-stackable Material (e.g. robot) — each carried instance is its
	// own entity. Stackable Material — one consolidated stack representing
	// the whole load.
	mc := carried.GetComponent(components.Material).(*components.MaterialComponent)
	stackable := mc.IsStackable()

	if req.DestEntity != nil && req.DestEntity.HasComponent(components.Storage) {
		destSc := req.DestEntity.GetComponent(components.Storage).(*components.StorageComponent)
		if stackable {
			if destSc.AddItem(carried) {
				inv.RemoveItem(carried)
				CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocated "+carried.Blueprint)
				aiMemory.State = "idle"
				return
			}
			dropMaterialAt(level, inv, req.Blueprint, req.Carried, pc.GetX(), pc.GetY(), pc.GetZ())
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocate: dest filter rejected, dropped")
			aiMemory.State = "idle"
			return
		}
		// Non-stackable: deposit each carried instance individually so
		// per-instance state is preserved through the move.
		anyDeposited, anyRefused := false, false
		for _, item := range append([]*ecs.Entity{}, inv.Bag...) {
			if item.Blueprint != req.Blueprint {
				continue
			}
			if destSc.AddItem(item) {
				inv.RemoveItem(item)
				anyDeposited = true
			} else {
				anyRefused = true
			}
		}
		if anyRefused {
			dropNonStackableBlueprintAt(level, entity, inv, req.Blueprint, pc.GetX(), pc.GetY(), pc.GetZ())
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocate: dest refused, dropped")
		} else if anyDeposited {
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocated "+carried.Blueprint)
		} else {
			CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocate: load lost")
		}
		aiMemory.State = "idle"
		return
	}

	// Ground drop.
	if stackable {
		dropMaterialAt(level, inv, req.Blueprint, req.Carried, dx, dy, dz)
	} else {
		dropNonStackableBlueprintAt(level, entity, inv, req.Blueprint, dx, dy, dz)
	}
	CompleteTaskWithMessage(entity, wc.CurrentTask, "Relocated "+carried.Blueprint)
	aiMemory.State = "idle"
}

// isNonStackableBlueprint reports whether the first item in sc.Items matching
// blueprint is a non-stackable Material (e.g. a stowed robot). Stackable
// Material and non-Material entities both return false; the caller should
// only reach this for items that are known to be present at all.
func isNonStackableBlueprint(sc *components.StorageComponent, blueprint string) bool {
	for _, it := range sc.Items {
		if it.Blueprint != blueprint {
			continue
		}
		if !it.HasComponent(components.Material) {
			return false
		}
		mc := it.GetComponent(components.Material).(*components.MaterialComponent)
		return !mc.IsStackable()
	}
	return false
}

// dropNonStackableBlueprintAt removes every bag item matching blueprint and
// places it on the tile (x,y,z), giving any Worker-bearing item the carrier's
// settlement and an idle AI state so it deploys instead of sitting inert.
func dropNonStackableBlueprintAt(level *world.Level, carrier *ecs.Entity, inv *rlcomponents.InventoryComponent, blueprint string, x, y, z int) {
	for _, item := range append([]*ecs.Entity{}, inv.Bag...) {
		if item.Blueprint != blueprint {
			continue
		}
		inv.RemoveItem(item)
		if item.HasComponent(rlcomponents.Position) {
			item.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent).SetPosition(x, y, z)
		} else {
			item.AddComponent(&rlcomponents.PositionComponent{X: x, Y: y, Z: z})
		}
		level.AddEntity(item)
		if item.HasComponent(components.Worker) && carrier.HasComponent(components.Settlement) {
			sc := carrier.GetComponent(components.Settlement).(*components.SettlementComponent)
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
			wc := item.GetComponent(components.Worker).(*components.WorkerComponent)
			wc.CurrentTask = nil
		}
	}
}

// dropMaterialAt removes the carried material stack from inv and places a new
// material entity with the given quantity at (x,y,z) on the level. Used as the
// ground-drop deposit and as the fallback when the destination is unreachable
// or rejects the load.
func dropMaterialAt(level *world.Level, inv *rlcomponents.InventoryComponent, blueprint string, qty, x, y, z int) {
	// Remove the carried stack from the bag.
	for i, item := range inv.Bag {
		if item.Blueprint == blueprint && item.HasComponent(components.Material) {
			inv.Bag = append(inv.Bag[:i], inv.Bag[i+1:]...)
			break
		}
	}
	dropped, err := factory.Create(blueprint, x, y, z)
	if err != nil {
		return
	}
	if dropped.HasComponent(components.Material) {
		dropped.GetComponent(components.Material).(*components.MaterialComponent).Quantity = qty
	}
	level.AddEntity(dropped)
}

// stationAt is the crafting station of id standing at (x, y, z), or nil.
func stationAt(level *world.Level, id string, x, y, z int) *ecs.Entity {
	var buf []*ecs.Entity
	level.GetEntitiesAt(x, y, z, &buf)
	for _, e := range buf {
		if e.HasComponent(components.CraftingStation) && !e.HasComponent(rlcomponents.Dead) &&
			e.GetComponent(components.CraftingStation).(*components.CraftingStationComponent).StationID == id {
			return e
		}
	}
	return nil
}

// deployWorker makes a crafted output a worker of settlement name (a printed
// colonist or unit), standing at the station.
func deployWorker(output *ecs.Entity, name string) {
	if output.HasComponent(components.Settlement) {
		output.GetComponent(components.Settlement).(*components.SettlementComponent).Name = name
	} else {
		output.AddComponent(&components.SettlementComponent{Name: name})
	}
	if !output.HasComponent(components.Worker) {
		output.AddComponent(&components.WorkerComponent{SelfDefend: true})
	}
}

// maxFootprintWait is how many turns finished build work waits on someone
// else standing in its footprint before the builder lets the task go.
const maxFootprintWait = 300
