package systems

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

const defaultEmoteCooldown = 3 // turns of fade-out gap between emotes

type EmoteSystem struct{}

var emoteSystemRequires = []ecs.ComponentType{components.Emote, rlcomponents.MyTurn}

func (s *EmoteSystem) Requires() []ecs.ComponentType       { return emoteSystemRequires }
func (s *EmoteSystem) UpdateSystem(data interface{}) error { return nil }

func (s *EmoteSystem) UpdateEntity(_ interface{}, entity *ecs.Entity) error {
	ec := entity.GetComponent(components.Emote).(*components.EmoteComponent)

	cooldown := ec.DefaultCooldown
	if cooldown <= 0 {
		cooldown = defaultEmoteCooldown
	}

	// --- Cooldown / fade-out phase ---
	if ec.CooldownTurns > 0 {
		ec.CooldownTurns--
		ec.Alpha = float32(ec.CooldownTurns) / float32(cooldown)
		if ec.CooldownTurns == 0 {
			ec.Active = ""
			ec.Alpha = 0
			activatePending(ec)
		}
		return nil
	}

	// --- Active display phase ---
	if ec.Active != "" {
		ec.DisplayTurns--
		if ec.DisplayTurns <= 0 {
			ec.CooldownTurns = cooldown
			ec.Alpha = 1.0
		}
		return nil
	}

	// --- Idle: nothing showing, no cooldown ---
	activatePending(ec)
	return nil
}

func activatePending(ec *components.EmoteComponent) {
	if ec.Pending == "" {
		return
	}
	ec.Active = ec.Pending
	ec.DisplayTurns = ec.PendingDuration
	ec.Alpha = 1.0
	ec.Pending = ""
	ec.PendingDuration = 0
	ec.PendingPriority = 0
}
