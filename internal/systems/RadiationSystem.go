package systems

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/scifi_settlements/internal/skills"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

// RadiationSystem deals damage each turn to entities standing on a tile with
// nonzero Radiation, unless the entity has the "radiation_resist" skill
// (innate or via equipped gear). Damage scales with tile radiation level.
type RadiationSystem struct{}

var radiationSystemRequires = []ecs.ComponentType{rlcomponents.Position, rlcomponents.Health, rlcomponents.MyTurn}

func (s *RadiationSystem) Requires() []ecs.ComponentType { return radiationSystemRequires }

func (s *RadiationSystem) UpdateSystem(data interface{}) error { return nil }

const RadiationResistSkill = "radiation_resist"

func (s *RadiationSystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	level := levelInterface.(*world.Level)

	if entity.HasComponent(rlcomponents.Dead) {
		return nil
	}
	if skills.Has(entity, RadiationResistSkill) {
		return nil
	}

	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	tile := level.GetTilePtr(pc.GetX(), pc.GetY(), pc.GetZ())
	if tile == nil || tile.Radiation == 0 {
		return nil
	}

	// 0..255 radiation maps to 0..4 damage per turn (very low so it's
	// survivable for short crossings but lethal for camping).
	dmg := int(tile.Radiation) / 64
	if dmg < 1 {
		dmg = 1
	}
	hc := entity.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
	hc.Health -= dmg
	return nil
}
