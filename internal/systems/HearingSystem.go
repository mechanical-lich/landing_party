package systems

import (
	"math"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// HearingSystem builds each listener's per-turn inbox of newly perceived
// sounds and sweeps expired sound events off the level.
//
// Order requirement: this must run before any AI system that consumes the
// inbox (FactionAISystem, ScriptedAISystem) so the inbox is fresh when
// scripts query it.
type HearingSystem struct{}

var hearingSystemRequires = []ecs.ComponentType{
	rlcomponents.Position,
	components.Hearing,
	rlcomponents.MyTurn,
}

func (s *HearingSystem) Requires() []ecs.ComponentType { return hearingSystemRequires }

// UpdateSystem runs once per round, before per-entity updates. Sweeps any
// sound events that have aged out.
func (s *HearingSystem) UpdateSystem(data interface{}) error {
	level, ok := data.(*world.Level)
	if !ok || level == nil {
		return nil
	}
	level.SweepExpiredSounds()
	return nil
}

func (s *HearingSystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	if entity.HasComponent(rlcomponents.Dead) {
		return nil
	}
	level := levelInterface.(*world.Level)
	hc := entity.GetComponent(components.Hearing).(*components.HearingComponent)
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	sx, sy, sz := pc.GetX(), pc.GetY(), pc.GetZ()

	hc.Inbox = hc.Inbox[:0]
	hc.InboxRead = 0

	maxSeq := hc.LastSeq
	for _, ev := range level.Sounds {
		if ev.Seq <= hc.LastSeq {
			continue
		}
		if ev.Seq > maxSeq {
			maxSeq = ev.Seq
		}
		if ev.Source == entity {
			continue
		}
		perceived := PerceivedLoudness(ev, sx, sy, sz)
		if perceived < hc.Sensitivity {
			continue
		}
		hc.Inbox = append(hc.Inbox, components.PerceivedSound{
			X:         ev.X,
			Y:         ev.Y,
			Z:         ev.Z,
			Tag:       string(ev.Tag),
			Perceived: perceived,
			EmitTick:  ev.Tick,
		})
	}
	hc.LastSeq = maxSeq
	return nil
}

// PerceivedLoudness applies weighted-euclidean distance falloff to a sound
// event. Vertical distance is multiplied by world.VerticalSoundFalloff so
// floors/ceilings muffle sound more than open space. Loudness is the raw
// emit value minus this distance; clamps at 0.
func PerceivedLoudness(ev world.SoundEvent, lx, ly, lz int) float32 {
	dx := float64(ev.X - lx)
	dy := float64(ev.Y - ly)
	dz := float64(ev.Z-lz) * world.VerticalSoundFalloff
	dist := float32(math.Sqrt(dx*dx + dy*dy + dz*dz))
	if dist >= ev.Loudness {
		return 0
	}
	return ev.Loudness - dist
}
