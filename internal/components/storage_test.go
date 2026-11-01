package components

import (
	"testing"

	"github.com/mechanical-lich/mlge/ecs"
)

func stack(blueprint string, qty, max int) *ecs.Entity {
	e := &ecs.Entity{Blueprint: blueprint}
	e.AddComponent(&MaterialComponent{Quantity: qty, MaxStack: max})
	return e
}

// A stack that only partly fits a full container must leave the caller
// holding exactly the remainder: the units merged into existing stacks were
// previously also left on the rejected item, duplicating them.
func TestAddItem_PartialFitInFullContainerReducesRemainder(t *testing.T) {
	s := &StorageComponent{Capacity: 1}
	s.Items = []*ecs.Entity{stack("metal_ore", 90, 100)}

	incoming := stack("metal_ore", 30, 100)
	if s.AddItem(incoming) {
		t.Fatal("AddItem should report false: 20 units need a slot and the container is full")
	}
	if got := s.Items[0].GetComponent(Material).(*MaterialComponent).Quantity; got != 100 {
		t.Fatalf("existing stack = %d, want 100 (topped up by 10)", got)
	}
	if got := incoming.GetComponent(Material).(*MaterialComponent).Quantity; got != 20 {
		t.Fatalf("rejected item quantity = %d, want 20 (the un-merged remainder)", got)
	}
	if got := s.CountResource("metal_ore"); got != 100 {
		t.Fatalf("container count = %d, want 100", got)
	}
}

// Control: when the remainder does get a slot, the item is stored with just
// that remainder and nothing is lost or doubled.
func TestAddItem_PartialFitWithFreeSlotStoresRemainder(t *testing.T) {
	s := &StorageComponent{Capacity: 2}
	s.Items = []*ecs.Entity{stack("metal_ore", 90, 100)}

	incoming := stack("metal_ore", 30, 100)
	if !s.AddItem(incoming) {
		t.Fatal("AddItem should succeed with a free slot")
	}
	if got := s.CountResource("metal_ore"); got != 120 {
		t.Fatalf("container count = %d, want 120", got)
	}
	if got := incoming.GetComponent(Material).(*MaterialComponent).Quantity; got != 20 {
		t.Fatalf("stored remainder stack = %d, want 20", got)
	}
}
