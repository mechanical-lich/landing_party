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

func checkSettlementStorageForCraft(level *world.Level, settlementName string, cost map[string]int) bool {
	p, owners := StorageProviderFor(level, settlementName)
	return storage.Check(p, owners, cost)
}

func deductFromSettlementStorage(level *world.Level, settlementName string, cost map[string]int) {
	p, owners := StorageProviderFor(level, settlementName)
	storage.Deduct(p, owners, cost)
}
