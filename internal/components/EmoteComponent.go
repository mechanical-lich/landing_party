package components

import "github.com/mechanical-lich/mlge/ecs"

// EmoteComponent drives the speech-bubble icons that appear above entities.
// Turn-based state (DisplayTurns, CooldownTurns) is managed by EmoteSystem.
// Frame-level animation (bounce, fade) is driven by the render pass using
// Alpha and wall-clock time — no per-frame component mutation needed.
// Use emotes.Set to queue an emote from any system.
type EmoteComponent struct {
	// Active emote
	Active       string  // emote key (see internal/emotes), empty = nothing showing
	DisplayTurns int     // turns remaining before fade begins
	Alpha        float32 // render alpha; 1.0 while active, fades to 0 during cooldown

	// Cooldown — the gap/fade phase between emotes
	CooldownTurns   int // turns remaining in fade-out gap
	DefaultCooldown int // config: turns of fade gap (default 3)

	// Pending queue — single slot; higher priority wins
	Pending         string
	PendingDuration int
	PendingPriority int
}

func (e *EmoteComponent) GetType() ecs.ComponentType { return Emote }
