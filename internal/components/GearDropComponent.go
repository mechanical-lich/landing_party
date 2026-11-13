package components

import "github.com/mechanical-lich/mlge/ecs"

// GearDropComponent is an item's chance of surviving its holder's death
// (greyish_plan.md, "Items": the greys' beam weapons and field armour
// usually break with their owner). When a body dies, each item in its hands
// and armour slots that has one rolls once: a pass drops it intact by the
// Drops path, a fail loses it. An item without GearDrop is lost, as every
// item on a dead body always was. A stored item in a spilled storage pocket
// rolls the same chance.
type GearDropComponent struct {
	Chance float64
}

func (g *GearDropComponent) GetType() ecs.ComponentType { return GearDrop }
