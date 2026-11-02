package components

import (
	"fmt"
	"math/rand/v2"

	"github.com/mechanical-lich/mlge/ecs"
)

// StableIDComponent gives an entity an identity that outlives its pointer, so a
// saved order can point at it across a save and load. The ID is a string because
// saved components pass through generic JSON, where a large number loses digits.
type StableIDComponent struct {
	ID string
}

func (s *StableIDComponent) GetType() ecs.ComponentType { return StableID }

// NewStableID returns a component holding a fresh random ID. Random rather than
// counted, so no counter has to be saved and levels saved apart can't collide.
func NewStableID() *StableIDComponent {
	return &StableIDComponent{ID: fmt.Sprintf("%016x", rand.Uint64())}
}

// EnsureStableID returns e's ID, assigning one if it has none (entities from saves
// older than IDs, or built by hand).
func EnsureStableID(e *ecs.Entity) string {
	if e.HasComponent(StableID) {
		if sc := e.GetComponent(StableID).(*StableIDComponent); sc.ID != "" {
			return sc.ID
		}
	}
	sc := NewStableID()
	e.AddComponent(sc)
	return sc.ID
}
