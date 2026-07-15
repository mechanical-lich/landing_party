package world

import (
	"github.com/mechanical-lich/mlge/ecs"
)

// SoundTag categorizes a sound for script-side filtering. Define common tags
// as constants near their emitter; arbitrary strings are accepted.
type SoundTag string

const (
	SoundTagGunshot  SoundTag = "gunshot"
	SoundTagFootstep SoundTag = "footstep"
	SoundTagScream   SoundTag = "scream"
	SoundTagBreak    SoundTag = "break"
	SoundTagImpact   SoundTag = "impact"
)

// SoundEvent is a one-shot sound emitted at a tile. The HearingSystem reads
// the level's slice each tick to build per-entity inboxes, then sweeps
// expired events.
type SoundEvent struct {
	X, Y, Z  int
	Loudness float32
	Tag      SoundTag
	Clip     string // optional explicit audio clip key; "" falls back to the tag's mapped clip
	Source   *ecs.Entity
	Tick     uint64 // tick when emitted (used for TTL sweep)
	Seq      uint64 // strictly monotonic — used by listeners to detect new events
}

// VerticalSoundFalloff is the multiplier applied to z-distance when computing
// perceived loudness. One floor up costs ~this many tiles of horizontal travel.
const VerticalSoundFalloff = 3.0

// SoundEventTTL is how many ticks a sound stays in the level's slice before
// being swept. Must exceed the slowest listener's initiative period so every
// entity gets at least one turn to react while the event is alive (current
// slowest mob period in data is ~14).
const SoundEventTTL uint64 = 20

// EmitSound appends a sound to the level's per-tick event list. Callers
// should always go through this function rather than appending directly so
// the tick stamp stays correct.
func (l *Level) EmitSound(x, y, z int, loudness float32, tag SoundTag, source *ecs.Entity) {
	l.EmitSoundClip(x, y, z, loudness, tag, "", source)
}

// EmitSoundClip is EmitSound with an explicit audio clip key, so an
// entity-specific sound (resolved from a SoundComponent + equipment) plays
// instead of the tag's generic clip. The tag still drives AI hearing; clip only
// affects audio playback. An empty clip is equivalent to EmitSound.
func (l *Level) EmitSoundClip(x, y, z int, loudness float32, tag SoundTag, clip string, source *ecs.Entity) {
	if loudness <= 0 {
		return
	}
	l.SoundSeq++
	l.Sounds = append(l.Sounds, SoundEvent{
		X: x, Y: y, Z: z,
		Loudness: loudness,
		Tag:      tag,
		Clip:     clip,
		Source:   source,
		Tick:     l.Tick,
		Seq:      l.SoundSeq,
	})
}

// SweepExpiredSounds drops sound events older than SoundEventTTL relative to
// the current level tick. Called once per round by HearingSystem.
func (l *Level) SweepExpiredSounds() {
	if len(l.Sounds) == 0 {
		return
	}
	cutoff := uint64(0)
	if l.Tick > SoundEventTTL {
		cutoff = l.Tick - SoundEventTTL
	}
	w := 0
	for _, s := range l.Sounds {
		if s.Tick >= cutoff {
			l.Sounds[w] = s
			w++
		}
	}
	l.Sounds = l.Sounds[:w]
}
