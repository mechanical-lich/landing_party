package ai

import (
	"fmt"
	"log"

	"github.com/mechanical-lich/landing_party/internal/components"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/skills"
	"github.com/mechanical-lich/landing_party/internal/world"
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

func FindClosestStorageWithComponent(level *world.Level, settlementName string, compType ecs.ComponentType, x, y, z int) *ecs.Entity {
	var closest *ecs.Entity
	minDist := 99999
	for _, e := range level.Entities {
		if !e.HasComponent(components.Storage) {
			continue
		}
		sc := e.GetComponent(components.Storage).(*components.StorageComponent)
		if sc.OwnedBy == settlementName && sc.HasItemWithComponent(compType) {
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
		aiMemory.CurrentSteps = fspath.GetPossiblePathForEntity(level, entity, from.(*world.Tile), to.(*world.Tile), aiMemory.CurrentSteps)
		if len(aiMemory.CurrentSteps) == 0 {
			log.Printf("[PATH] %s no path from (%d,%d,%d) to (%d,%d,%d)", rlentity.GetName(entity), pc.GetX(), pc.GetY(), pc.GetZ(), targetX, targetY, targetZ)
		}
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
			if isFlying(entity) {
				flyMove(entity, level, dx, dy, dz)
			} else {
				rlentity.Move(entity, level, dx, dy, dz)
			}
			rlentity.Face(entity, dx, dy)
			return true
		}
		if tryColonistSwap(level, entity, pc, ntX, ntY, ntZ) {
			aiMemory.CurrentSteps = aiMemory.CurrentSteps[1:]
			return true
		}
		aiMemory.CurrentSteps = nil
		break
	}
	return false
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

// FindLooseItemOnGround returns the nearest item entity lying on the ground (not in storage/inventory).
func FindLooseItemOnGround(level *world.Level, x, y, z int) *ecs.Entity {
	var closest *ecs.Entity
	minDist := 99999
	for _, e := range level.Entities {
		if !e.HasComponent(rlcomponents.Item) {
			continue
		}
		if !e.HasComponent(rlcomponents.Position) {
			continue
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if pc.GetZ() != z {
			continue
		}
		d := utility.Distance(pc.GetX(), pc.GetY(), x, y)
		if closest == nil || d < minDist {
			closest = e
			minDist = d
		}
	}
	return closest
}

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

func PickupItemFromTile(level *world.Level, entity *ecs.Entity, x, y, z int) {
	tileEntity := level.GetEntityAt(x, y, z)
	if tileEntity != nil && tileEntity.HasComponent(rlcomponents.Item) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		inv.AddItem(tileEntity)
		level.RemoveEntity(tileEntity)
		message.PostMessage(rlentity.GetName(entity), fmt.Sprintf("Picked up %s", tileEntity.Blueprint))
	}
}
