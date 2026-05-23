package systems

import (
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// isFactionMember reports whether the entity belongs to a faction the door
// would admit. Unlike rlentity.CanPassThroughDoor, this ignores the door's
// current open state so it can be used to decide whether to keep it open.
func isFactionMember(entity *ecs.Entity, door *rlcomponents.DoorComponent) bool {
	if !entity.HasComponent(rlcomponents.Description) {
		return false
	}
	dc := entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
	if door.OwnedBy != "" && dc.Faction == door.OwnedBy {
		return true
	}
	for _, f := range door.AllowedFactions {
		if dc.Faction == f {
			return true
		}
	}
	return false
}

// FactionDoorSystem auto-opens doors for authorized factions on approach and
// closes them when no authorized entity is adjacent.
type FactionDoorSystem struct {
	autoOpened map[*ecs.Entity]bool
}

var factionDoorRequires = []ecs.ComponentType{rlcomponents.Door, rlcomponents.Position}

func (s *FactionDoorSystem) Requires() []ecs.ComponentType { return factionDoorRequires }

func (s *FactionDoorSystem) UpdateSystem(data interface{}) error { return nil }

func (s *FactionDoorSystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	level := levelInterface.(*world.Level)

	door := entity.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent)
	if door.Locked {
		return nil
	}

	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	dx, dy, dz := pc.GetX(), pc.GetY(), pc.GetZ()

	if s.autoOpened == nil {
		s.autoOpened = make(map[*ecs.Entity]bool)
	}

	authorized := s.hasAuthorizedNeighbor(level, door, dx, dy, dz) || s.hasAuthorizedOccupant(level, door, dx, dy, dz)

	if authorized && !door.Open {
		door.Open = true
		s.autoOpened[entity] = true
		entity.RemoveComponent(rlcomponents.Solid)
	} else if !authorized && door.Open && s.autoOpened[entity] {
		door.Open = false
		delete(s.autoOpened, entity)
		entity.AddComponent(&rlcomponents.SolidComponent{})
	}

	return nil
}

func (s *FactionDoorSystem) hasAuthorizedOccupant(level *world.Level, door *rlcomponents.DoorComponent, x, y, z int) bool {
	var buf []*ecs.Entity
	level.GetEntitiesAt(x, y, z, &buf)
	for _, candidate := range buf {
		if isFactionMember(candidate, door) {
			return true
		}
	}
	return false
}

func (s *FactionDoorSystem) hasAuthorizedNeighbor(level *world.Level, door *rlcomponents.DoorComponent, x, y, z int) bool {
	var buf []*ecs.Entity
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			if dx == 0 && dy == 0 {
				continue
			}
			buf = buf[:0]
			level.GetEntitiesAt(x+dx, y+dy, z, &buf)
			for _, candidate := range buf {
				if isFactionMember(candidate, door) {
					return true
				}
			}
		}
	}
	return false
}
