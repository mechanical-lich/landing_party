package systems

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// ScentSystem runs every round to:
//  1. Decay all tag strengths and drop sub-epsilon entries (UpdateSystem).
//  2. Diffuse one tag per round, sharded across tags (UpdateSystem).
//  3. Deposit passive scent for any entity with a ScentEmitterComponent
//     that has MyTurn this round (UpdateEntity).
//
// Order: this must run before SmellSystem so the listener inbox sees the
// freshest scent state. Both must run before AI systems.
type ScentSystem struct{}

var scentSystemRequires = []ecs.ComponentType{
	rlcomponents.Position,
	components.ScentEmitter,
	rlcomponents.MyTurn,
}

func (s *ScentSystem) Requires() []ecs.ComponentType { return scentSystemRequires }

func (s *ScentSystem) UpdateSystem(data interface{}) error {
	level, ok := data.(*world.Level)
	if !ok || level == nil {
		return nil
	}
	// Decay/diffuse on a slower cadence than stepWorld. Both for performance
	// (sparse-map iteration is the bulk of cost) and so trails persist over
	// many rounds. Passive emission still happens per-entity-turn in
	// UpdateEntity below — only the maintenance pass is throttled.
	if world.ScentTickInterval > 0 && level.Tick%world.ScentTickInterval != 0 {
		return nil
	}
	level.DecayScent()
	level.DiffuseNextTag()
	return nil
}

func (s *ScentSystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	if entity.HasComponent(rlcomponents.Dead) {
		return nil
	}
	level := levelInterface.(*world.Level)
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	em := entity.GetComponent(components.ScentEmitter).(*components.ScentEmitterComponent)
	x, y, z := pc.GetX(), pc.GetY(), pc.GetZ()
	for _, e := range em.Emissions {
		if e.Tag == "" || e.Strength <= 0 {
			continue
		}
		level.EmitScent(x, y, z, world.SmellTag(e.Tag), e.Strength)
	}
	return nil
}
