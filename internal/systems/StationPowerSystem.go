package systems

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// PowerFlag is the level flag that powers a station grid ("power:main").
func PowerFlag(grid string) string { return "power:" + grid }

// StationPowerSystem runs a station's dead grid (station_plan.md, section
// 10). A Powered entity's light is held at 0 and, with Unlock, its door is
// held locked while its grid's flag is unset; once set, the light comes back
// to its Level and Range, the door unlocks, and the Powered component goes:
// the system never touches it again.
type StationPowerSystem struct{}

var stationPowerRequires = []ecs.ComponentType{components.Powered}

func (s *StationPowerSystem) Requires() []ecs.ComponentType { return stationPowerRequires }

func (s *StationPowerSystem) UpdateSystem(data interface{}) error { return nil }

func (s *StationPowerSystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	level := levelInterface.(*world.Level)
	p := entity.GetComponent(components.Powered).(*components.PoweredComponent)
	on := isOn(level.Flags[PowerFlag(p.Grid)])
	if entity.HasComponent(rlcomponents.Light) {
		lc := entity.GetComponent(rlcomponents.Light).(*rlcomponents.LightComponent)
		if on {
			lc.Level, lc.Range = p.Level, p.Range
		} else {
			lc.Level = 0
		}
	}
	if p.Unlock && entity.HasComponent(rlcomponents.Door) {
		entity.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent).Locked = !on
	}
	if on {
		entity.RemoveComponent(components.Powered)
	}
	return nil
}

// isOn reads a level flag as set: a non-zero number, true or a non-empty
// string.
func isOn(v any) bool {
	switch t := v.(type) {
	case float64:
		return t != 0
	case int:
		return t != 0
	case bool:
		return t
	case string:
		return t != ""
	}
	return false
}

// KeyDoorSystem opens a locked door that names a key (DoorComponent.KeyId,
// an item blueprint) when a colonist stands beside it and the colony holds
// the key: in a colonist's bag or hands, or in one of its storage containers.
// The door is then the colony's (OwnedBy), so its colonists path through it,
// and the level flag "door:<module kind>" is set for the module it seals.
type KeyDoorSystem struct {
	// Faction is the faction it unlocks for ("": PlayerFaction).
	Faction string
}

var keyDoorRequires = []ecs.ComponentType{rlcomponents.Door, rlcomponents.Position}

func (s *KeyDoorSystem) Requires() []ecs.ComponentType { return keyDoorRequires }

func (s *KeyDoorSystem) UpdateSystem(data interface{}) error { return nil }

func (s *KeyDoorSystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	level := levelInterface.(*world.Level)
	door := entity.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent)
	if !door.Locked || door.KeyId == "" || entity.HasComponent(components.Powered) {
		return nil
	}
	faction := s.Faction
	if faction == "" {
		faction = PlayerFaction
	}
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	x, y, z := pc.GetX(), pc.GetY(), pc.GetZ()
	beside := false
	var buf []*ecs.Entity
	for dx := -1; dx <= 1 && !beside; dx++ {
		for dy := -1; dy <= 1 && !beside; dy++ {
			buf = buf[:0]
			level.GetEntitiesAt(x+dx, y+dy, z, &buf)
			for _, e := range buf {
				if isFaction(e, faction) {
					beside = true
					break
				}
			}
		}
	}
	if !beside || !colonyHolds(level, faction, door.KeyId) {
		return nil
	}
	door.Locked = false
	door.OwnedBy = faction
	if m := level.ModuleAt(x, y, z); m != nil {
		level.Flags["door:"+m.Kind] = 1.0
	}
	return nil
}

func isFaction(e *ecs.Entity, faction string) bool {
	if e == nil || !e.HasComponent(rlcomponents.Description) {
		return false
	}
	return e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction == faction
}

// colonyHolds reports whether the colony has an item of blueprint bp: in a
// member's inventory, or in a storage container one of its members' storage
// rules own (OwnedBy any settlement of the faction's members) or that holds
// it beside a member.
func colonyHolds(level *world.Level, faction, bp string) bool {
	settlements := map[string]bool{}
	for _, e := range level.Entities {
		if !isFaction(e, faction) {
			continue
		}
		if e.HasComponent(components.Settlement) {
			settlements[e.GetComponent(components.Settlement).(*components.SettlementComponent).Name] = true
		}
		if e.HasComponent(rlcomponents.Inventory) {
			inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
			for _, it := range append([]*ecs.Entity{inv.LeftHand, inv.RightHand}, inv.Bag...) {
				if it != nil && it.Blueprint == bp {
					return true
				}
			}
		}
	}
	for _, list := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range list {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			st := e.GetComponent(components.Storage).(*components.StorageComponent)
			if !settlements[st.OwnedBy] {
				continue
			}
			for _, it := range st.Items {
				if it != nil && it.Blueprint == bp {
					return true
				}
			}
		}
	}
	return false
}
