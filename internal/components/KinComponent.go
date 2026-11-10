package components

import (
	"encoding/json"

	"github.com/mechanical-lich/mlge/ecs"
)

// Kin is the component type of KinComponent.
const Kin ecs.ComponentType = "Kin"

// KinComponent marks a creature of a faction with life stages and unit types
// (the hive: docs/developer/factions.md). The blueprint declares Morph; Type
// and Presence are set when it is spawned and passed to every later life
// stage by the spawn_kin script builtin.
type KinComponent struct {
	// Morph is the body plan ("crawler", "flier", "structure").
	Morph string `json:"morph,omitempty"`
	// Type is the doctrine's unit type ("drone", "warrior"; "" for a wild
	// creature); factory.ApplyType set it.
	Type string `json:"type,omitempty"`
	// Presence is the campaign FactionPresence ID this creature belongs to,
	// "" for one spawned by a scenario or the GameMaster.
	Presence string `json:"presence,omitempty"`
	// Settlement is the faction settlement that laid it ("hive:<locID>"):
	// a settled egg's hatchling grows into that settlement's working adult
	// (a crew body), where a wild one ages into a scripted creature.
	Settlement string `json:"settlement,omitempty"`
	// Tier is the print tier it spawned at ("steel", "bronze"; the autonz:
	// autonz_plan.md), "" for a doctrine without tiers. It never changes
	// after spawn.
	Tier string `json:"tier_name,omitempty"`
}

func (k *KinComponent) GetType() ecs.ComponentType { return Kin }

// legacyTierTypes is the type of a Phase B body's tier by morph: the colour
// block it was drawn in (red crawler warrior, orange flier hunter, purple
// crawler burrower, green flier spitter), so a save's sprites stay right.
var legacyTierTypes = map[string][3]string{
	"crawler": {"", "warrior", "burrower"},
	"flier":   {"", "hunter", "spitter"},
}

// UnmarshalJSON reads a Kin component, mapping a save's Phase B "tier" to the
// type of that colour.
func (k *KinComponent) UnmarshalJSON(b []byte) error {
	type plain KinComponent
	var v struct {
		plain
		LegacyTier int `json:"tier,omitempty"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*k = KinComponent(v.plain)
	if k.Type == "" && v.LegacyTier > 0 {
		if t, ok := legacyTierTypes[k.Morph]; ok {
			k.Type = t[min(v.LegacyTier, 2)]
		}
	}
	return nil
}

// KinOf returns e's Kin component, or nil.
func KinOf(e *ecs.Entity) *KinComponent {
	if e == nil || !e.HasComponent(Kin) {
		return nil
	}
	return e.GetComponent(Kin).(*KinComponent)
}
