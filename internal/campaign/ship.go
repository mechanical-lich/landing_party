package campaign

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
)

// ShipSettlementName is the synthetic settlement that owns everything in the
// ship hold. It never appears in a per-location level save.
const ShipSettlementName = "ship"

// ShipState is the abstract hub: the persistent stockpile (Hold) and the
// colonists currently aboard (Roster), both serialized — neither lives on a
// live *world.Level while the ship is in space.
type ShipState struct {
	// Hold holds StorageComponent-bearing entities owned by ShipSettlementName.
	Hold []*world.SaveEntity `json:"hold"`
	// Roster holds colonist entities currently aboard the ship.
	Roster []*world.SaveEntity `json:"roster"`
	// RosterCap is the maximum number of colonists that may be aboard.
	RosterCap int `json:"roster_cap"`

	// liveHold is the rebuilt, mutable form of Hold used while a campaign is
	// active. Not serialized; rebuilt lazily and flushed back via Sync.
	liveHold []*ecs.Entity
}

// LiveHold lazily rebuilds the hold into mutable entities (so crafting/fuel
// deduction operates on real StorageComponents) and caches the result.
func (s *ShipState) LiveHold() []*ecs.Entity {
	if s.liveHold == nil {
		s.liveHold = make([]*ecs.Entity, 0, len(s.Hold))
		for _, se := range s.Hold {
			if se == nil {
				continue
			}
			s.liveHold = append(s.liveHold, world.RebuildLiveEntity(se))
		}
		if len(s.liveHold) == 0 {
			s.liveHold = append(s.liveHold, world.RebuildLiveEntity(newHoldStorage()))
		}
	}
	return s.liveHold
}

// Sync flushes the live hold back into the serializable Hold slice. Call
// before any campaign save.
func (s *ShipState) Sync() {
	if s.liveHold == nil {
		return
	}
	s.Hold = s.Hold[:0]
	for _, e := range s.liveHold {
		s.Hold = append(s.Hold, world.EntityToSaveEntity(e))
	}
}

// NewShipState returns an empty hold with a single hold-storage entity so
// beamed-up resources have somewhere to stack.
func NewShipState(rosterCap int) *ShipState {
	return &ShipState{
		Hold:      []*world.SaveEntity{newHoldStorage()},
		Roster:    nil,
		RosterCap: rosterCap,
	}
}

// newHoldStorage builds the persistent "ship hold" container as a SaveEntity so
// it round-trips through the existing world serialization unchanged.
func newHoldStorage() *world.SaveEntity {
	return &world.SaveEntity{
		Blueprint: "ship_hold",
		Components: map[ecs.ComponentType]any{
			components.Storage: map[string]any{
				"Capacity": float64(100000),
				"OwnedBy":  ShipSettlementName,
			},
		},
	}
}
