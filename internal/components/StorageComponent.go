package components

import (
	"github.com/mechanical-lich/mlge/ecs"
)

const StackMax = 100

type StorageComponent struct {
	Capacity int
	Items    []*ecs.Entity
	OwnedBy  string
}

func (s *StorageComponent) GetType() ecs.ComponentType { return Storage }

func (s *StorageComponent) AddItem(item *ecs.Entity) {
	if item.HasComponent(ResourceItem) {
		rc := item.GetComponent(ResourceItem).(*ResourceItemComponent)
		qty := rc.Quantity
		if qty <= 0 {
			qty = 1
		}
		for _, existing := range s.Items {
			if existing.Blueprint != item.Blueprint || !existing.HasComponent(ResourceItem) {
				continue
			}
			ec := existing.GetComponent(ResourceItem).(*ResourceItemComponent)
			room := StackMax - ec.Quantity
			if room <= 0 {
				continue
			}
			if qty <= room {
				ec.Quantity += qty
				return
			}
			ec.Quantity = StackMax
			qty -= room
		}
		rc.Quantity = qty
		s.Items = append(s.Items, item)
		return
	}
	s.Items = append(s.Items, item)
}

// TakeOne removes and returns one non-resource item by blueprint name.
// For food, equipment, and other non-stackable items.
func (s *StorageComponent) TakeOne(name string) *ecs.Entity {
	for i, item := range s.Items {
		if item.Blueprint == name {
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
			return item
		}
	}
	return nil
}

// TakeResourceStack removes and returns the entire stack entity for a resource.
func (s *StorageComponent) TakeResourceStack(blueprint string) *ecs.Entity {
	for i, item := range s.Items {
		if item.Blueprint == blueprint && item.HasComponent(ResourceItem) {
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
			return item
		}
	}
	return nil
}

// CountResource returns total quantity of a resource across all stacks,
// or the entity count for non-resource items.
func (s *StorageComponent) CountResource(blueprint string) int {
	count := 0
	for _, item := range s.Items {
		if item.Blueprint != blueprint {
			continue
		}
		if item.HasComponent(ResourceItem) {
			rc := item.GetComponent(ResourceItem).(*ResourceItemComponent)
			count += rc.Quantity
		} else {
			count++
		}
	}
	return count
}

// DeductResource subtracts amount units from resource stacks, removing depleted
// stack entities. Returns true if the full amount was deducted.
func (s *StorageComponent) DeductResource(blueprint string, amount int) bool {
	if s.CountResource(blueprint) < amount {
		return false
	}
	remaining := amount
	i := 0
	for remaining > 0 && i < len(s.Items) {
		item := s.Items[i]
		if item.Blueprint != blueprint || !item.HasComponent(ResourceItem) {
			i++
			continue
		}
		rc := item.GetComponent(ResourceItem).(*ResourceItemComponent)
		if rc.Quantity <= remaining {
			remaining -= rc.Quantity
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
		} else {
			rc.Quantity -= remaining
			remaining = 0
			i++
		}
	}
	return true
}

func (s *StorageComponent) HasItem(name string) bool {
	return s.CountResource(name) > 0
}

func (s *StorageComponent) CountItem(name string) int {
	return s.CountResource(name)
}

func (s *StorageComponent) HasItemWithComponent(compType ecs.ComponentType) bool {
	for _, item := range s.Items {
		if item.HasComponent(compType) {
			return true
		}
	}
	return false
}

func (s *StorageComponent) TakeOneWithComponent(compType ecs.ComponentType) *ecs.Entity {
	for i, item := range s.Items {
		if item.HasComponent(compType) {
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
			return item
		}
	}
	return nil
}
