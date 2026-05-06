package systems

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcombat"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/utility"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/eventsystem"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

type FactionAISystem struct {
	entitiesBuf []*ecs.Entity
}

func NewFactionAISystem() *FactionAISystem {
	return &FactionAISystem{entitiesBuf: make([]*ecs.Entity, 0, 8)}
}

var factionAIRequires = []ecs.ComponentType{rlcomponents.Position, components.FactionAI, rlcomponents.MyTurn}

func (s *FactionAISystem) Requires() []ecs.ComponentType { return factionAIRequires }

func (s *FactionAISystem) UpdateSystem(data interface{}) error { return nil }

func (s *FactionAISystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	level := levelInterface.(*world.Level)

	if entity.HasComponent(rlcomponents.Dead) {
		return nil
	}

	if entity.HasComponent(rlcomponents.Health) {
		hc := entity.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		if hc.Health <= 0 {
			fac := entity.GetComponent(components.FactionAI).(*components.FactionAIComponent)
			entity.RemoveComponent(components.FactionAI)
			entity.AddComponent(&rlcomponents.DeadComponent{})
			event.GetQueuedInstance().QueueEvent(eventsystem.EntityDiedEvent{
				EntityName: entity.Blueprint,
				Faction:    fac.Faction,
				Blueprint:  entity.Blueprint,
			})
			return nil
		}
	}

	fac := entity.GetComponent(components.FactionAI).(*components.FactionAIComponent)
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)

	switch fac.BehaviorKey {
	case "hostile":
		s.entitiesBuf = s.entitiesBuf[:0]
		closest := level.GetClosestEntityMatching(
			pc.GetX(), pc.GetY(), pc.GetZ(),
			8, 8,
			entity,
			func(candidate *ecs.Entity) bool {
				if candidate.HasComponent(rlcomponents.Dead) {
					return false
				}
				if !candidate.HasComponent(components.Worker) {
					return false
				}
				return true
			},
		)
		if closest != nil {
			targetPC := closest.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			dx, dy := 0, 0
			if pc.GetX() < targetPC.GetX() {
				dx = 1
			} else if pc.GetX() > targetPC.GetX() {
				dx = -1
			}
			if pc.GetY() < targetPC.GetY() {
				dy = 1
			} else if pc.GetY() > targetPC.GetY() {
				dy = -1
			}
			hit := false
			s.entitiesBuf = s.entitiesBuf[:0]
			level.GetEntitiesAt(pc.GetX()+dx, pc.GetY()+dy, pc.GetZ(), &s.entitiesBuf)
			for _, e := range s.entitiesBuf {
				if e != entity && e.HasComponent(rlcomponents.Health) && !rlcombat.IsFriendly(entity, e) {
					rlcombat.Hit(level, entity, e, true)
					hit = true
				}
			}
			if !hit {
				rlentity.Move(entity, level, dx, dy, 0)
				rlentity.Face(entity, dx, dy)
			}
			return nil
		}
		fallthrough
	default:
		dx := utility.GetRandom(-1, 2)
		dy := 0
		if dx == 0 {
			dy = utility.GetRandom(-1, 2)
		}
		rlentity.Move(entity, level, dx, dy, 0)
		rlentity.Face(entity, dx, dy)
	}

	return nil
}
