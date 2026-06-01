package components

import "github.com/mechanical-lich/mlge/ecs"

// PerceivedSound is one entry in a HearingComponent's inbox: a sound the
// entity heard since its last turn, with the loudness already attenuated
// by distance. Fields are copied from the originating SoundEvent so the
// component doesn't depend on the world package.
type PerceivedSound struct {
	X, Y, Z   int
	Tag       string
	Perceived float32
	EmitTick  uint64
}

// HearingComponent declares that an entity can perceive sounds and stores
// its per-turn inbox + bookkeeping. Sensitivity is the minimum perceived
// loudness (after distance falloff) that registers.
type HearingComponent struct {
	Sensitivity float32 `json:"sensitivity,omitempty"`

	// Runtime — not persisted.
	// LastSeq is the highest SoundEvent.Seq this entity has ingested.
	// Events with Seq > LastSeq are picked up on the next inbox rebuild.
	LastSeq uint64           `json:"-"`
	Inbox   []PerceivedSound `json:"-"`
	// InboxRead is the index of the next unread inbox entry for new_sounds().
	// Lets scripts call new_sounds() multiple times in a turn without
	// re-consuming, while still draining across turns.
	InboxRead int `json:"-"`
}

func (h *HearingComponent) GetType() ecs.ComponentType { return Hearing }
