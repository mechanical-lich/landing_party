package workerai

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/utility"
)

// A settlement's diet (hive_plan.md, "The settlement block"): material tags
// its workers eat besides food, one unit of a stack at a time (the hive eats
// biomass). The game wires SettlementDiet to the doctrine's settlement block.

// SettlementDiet returns the diet tags of settlement name, nil for none.
var SettlementDiet func(name string) []string

// DietNutrition is what one unit of a diet material feeds.
const DietNutrition = 30

// dietOf is entity's settlement's diet.
func dietOf(entity *ecs.Entity) []string {
	if SettlementDiet == nil || !entity.HasComponent(components.Settlement) {
		return nil
	}
	return SettlementDiet(entity.GetComponent(components.Settlement).(*components.SettlementComponent).Name)
}

// edible reports whether item feeds a worker with diet, and how much: food
// by its Food amount, a diet material by DietNutrition a unit.
func edible(item *ecs.Entity, diet []string) (int, bool) {
	if item == nil {
		return 0, false
	}
	if item.HasComponent(rlcomponents.Food) {
		return item.GetComponent(rlcomponents.Food).(*rlcomponents.FoodComponent).Amount, true
	}
	if len(diet) == 0 || !item.HasComponent(components.Material) {
		return 0, false
	}
	mc := item.GetComponent(components.Material).(*components.MaterialComponent)
	if mc.Quantity <= 0 {
		return 0, false
	}
	for _, t := range mc.Tags {
		for _, d := range diet {
			if t == d {
				return DietNutrition, true
			}
		}
	}
	return 0, false
}

// eatOne takes one bite of item: a diet material's stack loses a unit
// (reports true when it is used up); a food item is used up.
func eatOne(item *ecs.Entity) (gone bool) {
	if item.HasComponent(rlcomponents.Food) || !item.HasComponent(components.Material) {
		return true
	}
	mc := item.GetComponent(components.Material).(*components.MaterialComponent)
	mc.Quantity--
	return mc.Quantity <= 0
}

// closestStorageWithDiet is the nearest storage of settlementName holding
// something a worker with diet eats.
func closestStorageWithDiet(level *world.Level, settlementName string, diet []string, x, y int) *ecs.Entity {
	var closest *ecs.Entity
	minDist := 99999
	for _, e := range level.Entities {
		if !e.HasComponent(components.Storage) {
			continue
		}
		sc := e.GetComponent(components.Storage).(*components.StorageComponent)
		if sc.OwnedBy != settlementName {
			continue
		}
		ok := false
		for _, it := range sc.Items {
			if _, ok = edible(it, diet); ok {
				break
			}
		}
		if !ok {
			continue
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if d := utility.Distance(pc.GetX(), pc.GetY(), x, y); closest == nil || d < minDist {
			closest, minDist = e, d
		}
	}
	return closest
}

// takeEdible takes one bite from storage: a food item out of it, or a unit of
// a diet material stack (the stack stays while any is left). The item eaten
// (nil for none) and what it fed.
func takeEdible(sc *components.StorageComponent, diet []string) (*ecs.Entity, int) {
	if it := sc.TakeOneWithComponent(rlcomponents.Food); it != nil {
		return it, it.GetComponent(rlcomponents.Food).(*rlcomponents.FoodComponent).Amount
	}
	for i, it := range sc.Items {
		if amount, ok := edible(it, diet); ok {
			if eatOne(it) {
				sc.Items = append(sc.Items[:i], sc.Items[i+1:]...)
			}
			return it, amount
		}
	}
	return nil, 0
}

// KnowsTech reports whether settlement name knows tech key, for the bench's
// requires_tech check. The game wires it so a campaign player's settlement
// reads the campaign record (where the player's techs live); nil reads the
// settlement's own list.
var KnowsTech func(name, key string) bool

// SettlementKnowsTech is KnowsTech, else the settlement's own techs; an
// unknown settlement refuses nothing.
func SettlementKnowsTech(name, key string) bool {
	if KnowsTech != nil {
		return KnowsTech(name, key)
	}
	st, ok := settlement.Settlements[name]
	return !ok || st.HasTech(key)
}
