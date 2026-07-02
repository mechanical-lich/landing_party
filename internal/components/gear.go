package components

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// ItemIsGear reports whether item equips onto a body slot (as opposed to the
// bag / a carried consumable). Non-items are never gear. This is the single
// gear-vs-consumable predicate shared by the colonist modal, the equip task
// handler, and the HUD inventory list.
func ItemIsGear(item *ecs.Entity) bool {
	if item == nil || !item.HasComponent(rlcomponents.Item) {
		return false
	}
	switch item.GetComponent(rlcomponents.Item).(*rlcomponents.ItemComponent).Slot {
	case rlcomponents.HandSlot, rlcomponents.OffHandSlot, rlcomponents.HeadSlot,
		rlcomponents.TorsoSlot, rlcomponents.LegsSlot, rlcomponents.FeetSlot:
		return true
	}
	return false
}
