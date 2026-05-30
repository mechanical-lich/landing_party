package workerai

import (
	"fmt"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/utility"
)

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

func PickupItemFromTile(level *world.Level, entity *ecs.Entity, x, y, z int) {
	tileEntity := level.GetEntityAt(x, y, z)
	if tileEntity != nil && tileEntity.HasComponent(rlcomponents.Item) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		inv.AddItem(tileEntity)
		level.RemoveEntity(tileEntity)
		message.PostMessage(rlentity.GetName(entity), fmt.Sprintf("Picked up %s", tileEntity.Blueprint))
	}
}

func checkInventoryForMaterials(entity *ecs.Entity, buildable construction.Buildable) bool {
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for name, cost := range buildable.Cost {
		count := 0
		for _, item := range inv.Bag {
			if item.Blueprint != name {
				continue
			}
			if item.HasComponent(components.Material) {
				mc := item.GetComponent(components.Material).(*components.MaterialComponent)
				count += mc.Quantity
			} else {
				count++
			}
		}
		if count < cost {
			return false
		}
	}
	return true
}

func removeMaterialsFromInventory(entity *ecs.Entity, buildable construction.Buildable) {
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for name, cost := range buildable.Cost {
		remaining := cost
		i := 0
		for remaining > 0 && i < len(inv.Bag) {
			item := inv.Bag[i]
			if item.Blueprint != name {
				i++
				continue
			}
			if item.HasComponent(components.Material) {
				mc := item.GetComponent(components.Material).(*components.MaterialComponent)
				if mc.Quantity <= remaining {
					remaining -= mc.Quantity
					inv.Bag = append(inv.Bag[:i], inv.Bag[i+1:]...)
				} else {
					mc.Quantity -= remaining
					remaining = 0
					i++
				}
			} else {
				remaining--
				inv.Bag = append(inv.Bag[:i], inv.Bag[i+1:]...)
			}
		}
	}
}
