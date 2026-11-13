package game

import (
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// Rare gear drops (greyish_plan.md, "Items", decided 2026-11-09): a dead
// body's hands and armour slots are lost with it, as always, except an item
// with a GearDrop chance, which rolls once against the simulation's seeded
// stream (seedSimRand, so a replay drops the same) and on a pass drops
// intact by the Drops path: into the adjacent killer's bag (who then equips
// the best of it), else on the ground. Items without GearDrop never draw a
// number, so no other faction's deaths change.

// gearSurvives reports whether item survives its holder: it has GearDrop
// and the roll passes. Items without one are lost and draw nothing.
func gearSurvives(item *ecs.Entity) bool {
	if item == nil || !item.HasComponent(components.GearDrop) {
		return false
	}
	g := item.GetComponent(components.GearDrop).(*components.GearDropComponent)
	if g.Chance <= 0 {
		return false
	}
	return rand.Float64() < g.Chance
}

// wornGear is the items in a body's hands and armour slots, each once (a
// two-handed weapon fills both hands), in slot order.
func wornGear(inv *rlcomponents.InventoryComponent) []*ecs.Entity {
	var out []*ecs.Entity
	seen := map[*ecs.Entity]bool{}
	for _, it := range []*ecs.Entity{inv.RightHand, inv.LeftHand, inv.Head, inv.Torso, inv.Legs, inv.Feet} {
		if it != nil && !seen[it] {
			seen[it] = true
			out = append(out, it)
		}
	}
	return out
}

// dropSurvivingGear rolls each GearDrop item a dying body wears or holds and
// drops the ones that survive (killer: the adjacent worker that gets them,
// nil for the ground). It reports how many dropped.
func dropSurvivingGear(level *world.Level, body, killer *ecs.Entity) int {
	if body == nil || !body.HasComponent(rlcomponents.Inventory) {
		return 0
	}
	pc, ok := entityPos(body)
	if !ok {
		return 0
	}
	inv := body.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	n := 0
	for _, it := range wornGear(inv) {
		if !gearSurvives(it) {
			continue
		}
		unslot(inv, it)
		dropToKiller(level, killer, it, pc.GetX(), pc.GetY(), pc.GetZ())
		n++
	}
	if n > 0 && killer != nil && killer.HasComponent(rlcomponents.Inventory) {
		killer.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).EquipAllBest()
	}
	return n
}

// unslot takes it out of every slot of inv.
func unslot(inv *rlcomponents.InventoryComponent, it *ecs.Entity) {
	for _, s := range []**ecs.Entity{&inv.RightHand, &inv.LeftHand, &inv.Head, &inv.Torso, &inv.Legs, &inv.Feet} {
		if *s == it {
			*s = nil
		}
	}
	inv.RemoveItem(it)
}

// dropToKiller puts it in killer's bag, else on the ground at (x, y, z).
func dropToKiller(level *world.Level, killer, it *ecs.Entity, x, y, z int) {
	if killer != nil && killer.HasComponent(rlcomponents.Inventory) {
		killer.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).AddItem(it)
		return
	}
	if p, ok := entityPos(it); ok {
		p.SetPosition(x, y, z)
	} else {
		it.AddComponent(&rlcomponents.PositionComponent{})
		it.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent).SetPosition(x, y, z)
	}
	level.AddEntity(it)
}

// hasGearDrop reports whether a body wears or holds any GearDrop item (so a
// death with none skips the killer search).
func hasGearDrop(body *ecs.Entity) bool {
	inv, ok := body.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	if !ok || inv == nil {
		return false
	}
	for _, it := range wornGear(inv) {
		if it.HasComponent(components.GearDrop) {
			return true
		}
	}
	return false
}
