package components

import "github.com/mechanical-lich/mlge/ecs"

// SkillsComponent lists the skill IDs an entity (or item) has. On an actor it
// represents innate skills; on an equippable item it represents skills granted
// to whoever equips the item.
//
// Skills are simple string IDs for now — used as boolean checks (e.g.
// "radiation_resist"). A registry can be layered on top later for richer
// effects without requiring a component-shape change.
type SkillsComponent struct {
	Skills []string `json:"Skills"`
}

func (s *SkillsComponent) GetType() ecs.ComponentType { return Skills }

// Has returns true if this component lists the given skill ID.
func (s *SkillsComponent) Has(id string) bool {
	for _, k := range s.Skills {
		if k == id {
			return true
		}
	}
	return false
}
