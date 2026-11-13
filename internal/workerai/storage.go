package workerai

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/utility"
)

// StorageProviderFor resolves which storage a settlement may draw from. The
// default scans only the live level; MainState overrides this to also include
// the campaign ship hold (so a landing party crafts from ship + on-planet
// stock). settlementName is the colony; the ship hold ("ship") is always
// included so its entities match too.
var StorageProviderFor = func(level *world.Level, settlementName string) (storage.Provider, []string) {
	return storage.LevelProvider{Level: level}, []string{settlementName}
}

func FindAvailableStorage(level *world.Level, settlementName string) *ecs.Entity {
	for _, src := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range src {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			if sc.OwnedBy == settlementName {
				return e
			}
		}
	}
	return nil
}

// FindAvailableStorageFor returns the first colony-owned storage container
// whose tag filter accepts item, or nil if none will. Scans BOTH Entities and
// StaticEntities so a placed locker that ends up static (Inanimate-tagged
// blueprints) is still reachable — every other storage-lookup path in the
// game iterates both slices, so this one matches.
//
// When called with a nil item, falls through to FindAvailableStorage's "any
// colony locker" semantics for backward compatibility.
func FindAvailableStorageFor(level *world.Level, settlementName string, item *ecs.Entity) *ecs.Entity {
	if item == nil {
		return FindAvailableStorage(level, settlementName)
	}
	for _, src := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range src {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			if sc.OwnedBy == settlementName && sc.Accepts(item) {
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
		if e == nil || !e.HasComponent(components.Storage) {
			continue
		}
		sc := e.GetComponent(components.Storage).(*components.StorageComponent)
		if sc.OwnedBy == settlementName && sc.HasItem(itemName) {
			d := storageDist(e, sc, x, y, z)
			if closest == nil || d < minDist {
				closest = e
				minDist = d
			}
		}
	}
	return closest
}

// FindClosestStorageWithAny returns the colony container nearest to (x,y) holding
// at least one blueprint on list. Scans both Entities and StaticEntities so it
// agrees with the storage.Provider counts that gate task picking.
func FindClosestStorageWithAny(level *world.Level, settlementName string, list map[string]int, x, y, z int) *ecs.Entity {
	var closest *ecs.Entity
	minDist := 99999
	for _, src := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range src {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			if sc.OwnedBy != settlementName || !storageHoldsAny(sc, list) {
				continue
			}
			d := storageDist(e, sc, x, y, z)
			if closest == nil || d < minDist {
				closest = e
				minDist = d
			}
		}
	}
	return closest
}

// findClosestStorageWithRoomFor returns the nearest colony container that accepts
// item and has a free slot. A full container is skipped even if it could merge part
// of a stack, since AddItem refuses whatever doesn't fit in a slot.
func findClosestStorageWithRoomFor(level *world.Level, settlementName string, item *ecs.Entity, x, y, z int) *ecs.Entity {
	var closest *ecs.Entity
	minDist := 99999
	for _, src := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range src {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			if sc.OwnedBy != settlementName || !sc.Accepts(item) || !sc.HasRoom() {
				continue
			}
			d := storageDist(e, sc, x, y, z)
			if closest == nil || d < minDist {
				closest = e
				minDist = d
			}
		}
	}
	return closest
}

func storageHoldsAny(sc *components.StorageComponent, list map[string]int) bool {
	for name := range list {
		if sc.HasItem(name) {
			return true
		}
	}
	return false
}

// storageStillUsable reports whether a container picked earlier is still on the
// level and still holds something on the list; it can be emptied or destroyed
// while the worker walks to it.
func storageStillUsable(level *world.Level, e *ecs.Entity, list map[string]int) bool {
	for _, src := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, candidate := range src {
			if candidate == e {
				return storageHoldsAny(e.GetComponent(components.Storage).(*components.StorageComponent), list)
			}
		}
	}
	return false
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
			d := storageDist(e, sc, x, y, z)
			if closest == nil || d < minDist {
				closest = e
				minDist = d
			}
		}
	}
	return closest
}

// FoodAvailable reports whether HandleFindFood would find something for this
// worker: food in their own bag or in settlement storage.
func FoodAvailable(level *world.Level, entity *ecs.Entity) bool {
	diet := dietOf(entity)
	if entity.HasComponent(rlcomponents.Inventory) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for _, item := range inv.Bag {
			if _, ok := edible(item, diet); ok {
				return true
			}
		}
	}
	if !entity.HasComponent(components.Settlement) {
		return false
	}
	sc := entity.GetComponent(components.Settlement).(*components.SettlementComponent)
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if len(diet) > 0 {
		return closestStorageWithDiet(level, sc.Name, diet, pc.GetX(), pc.GetY()) != nil
	}
	return FindClosestStorageWithComponent(level, sc.Name, rlcomponents.Food, pc.GetX(), pc.GetY(), pc.GetZ()) != nil
}

// checkSettlementStorageForCraft answers "is there any point starting" at task
// pick time. It counts the pool; the materials themselves are fetched physically.
func checkSettlementStorageForCraft(level *world.Level, settlementName string, cost map[string]int) bool {
	p, owners := StorageProviderFor(level, settlementName)
	return storage.Check(p, owners, cost)
}

// deductFromSettlementStorage has no callers: nothing consumes from the pool since
// crafting and research went physical.
func deductFromSettlementStorage(level *world.Level, settlementName string, cost map[string]int) {
	p, owners := StorageProviderFor(level, settlementName)
	storage.Deduct(p, owners, cost)
}

// pocketOtherLevel is what a pocket door on another level than the worker
// adds to its distance: any door of a pocket will do, so one on the
// worker's own level wins over one straight below in a far mine (the greys'
// door by their deep mine). An ordinary container is measured as before.
const pocketOtherLevel = 1000

// storageDist is a container's distance from (x, y, z) for the finders.
func storageDist(e *ecs.Entity, sc *components.StorageComponent, x, y, z int) int {
	pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	d := utility.Distance(pc.GetX(), pc.GetY(), x, y)
	if sc.Pocket != nil && pc.GetZ() != z {
		d += pocketOtherLevel
	}
	return d
}
