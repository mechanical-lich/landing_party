package components

import "github.com/mechanical-lich/mlge/ecs"

type StorageComponent struct {
	Capacity int
	Items    []*ecs.Entity
	OwnedBy  string
}

func (s *StorageComponent) GetType() ecs.ComponentType { return Storage }

func (s *StorageComponent) AddItem(item *ecs.Entity) {
	s.Items = append(s.Items, item)
}

func (s *StorageComponent) TakeOne(name string) *ecs.Entity {
	for i, item := range s.Items {
		if item.Blueprint == name {
			s.Items = append(s.Items[:i], s.Items[i+1:]...)
			return item
		}
	}
	return nil
}

func (s *StorageComponent) HasItem(name string) bool {
	for _, item := range s.Items {
		if item.Blueprint == name {
			return true
		}
	}
	return false
}

func (s *StorageComponent) CountItem(name string) int {
	count := 0
	for _, item := range s.Items {
		if item.Blueprint == name {
			count++
		}
	}
	return count
}
