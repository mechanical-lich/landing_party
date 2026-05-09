// Package skills provides aggregate skill queries for entities.
//
// An entity's effective skills are the union of:
//   - its own SkillsComponent (innate skills)
//   - the SkillsComponent on each equipped item (gear-granted skills)
package skills

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
)

// Has returns true if the entity has the named skill, either innate or via
// equipped gear.
func Has(entity *ecs.Entity, skill string) bool {
	if entity == nil {
		return false
	}
	if entity.HasComponent(components.Skills) {
		sc := entity.GetComponent(components.Skills).(*components.SkillsComponent)
		if sc.Has(skill) {
			return true
		}
	}
	if entity.HasComponent(rlcomponents.Inventory) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for _, item := range []*ecs.Entity{inv.RightHand, inv.LeftHand, inv.Head, inv.Torso, inv.Legs, inv.Feet} {
			if item == nil {
				continue
			}
			if !item.HasComponent(components.Skills) {
				continue
			}
			isc := item.GetComponent(components.Skills).(*components.SkillsComponent)
			if isc.Has(skill) {
				return true
			}
		}
	}
	return false
}

// All returns the deduplicated set of skill IDs effective on the entity.
func All(entity *ecs.Entity) []string {
	if entity == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(ids []string) {
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
	}
	if entity.HasComponent(components.Skills) {
		add(entity.GetComponent(components.Skills).(*components.SkillsComponent).Skills)
	}
	if entity.HasComponent(rlcomponents.Inventory) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for _, item := range []*ecs.Entity{inv.RightHand, inv.LeftHand, inv.Head, inv.Torso, inv.Legs, inv.Feet} {
			if item == nil || !item.HasComponent(components.Skills) {
				continue
			}
			add(item.GetComponent(components.Skills).(*components.SkillsComponent).Skills)
		}
	}
	return out
}
