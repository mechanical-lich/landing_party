package audio

import (
	"math"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	mlaudio "github.com/mechanical-lich/mlge/audio"
)

// edgeRolloff controls how much a sound quiets toward the edge of the view: at
// the center a sound plays at full volume, at a view corner at (1 - edgeRolloff).
const edgeRolloff = 0.6

// minAudibleVolume is the volume below which a positional sound is dropped
// rather than played (inaudible, not worth a voice).
const minAudibleVolume = 0.05

// footstepVolumeScale keeps footsteps at half the volume of other world SFX —
// they're frequent, so a lighter mix keeps them from dominating.
const footstepVolumeScale = 0.5

// worldBridge turns in-world SoundEvents into positional one-shots. Each frame it
// reads the live level's Sounds slice, plays any it hasn't seen yet (tracked by
// the level's monotonic SoundSeq), gated to the camera's view and attenuated by
// distance from the view center.
//
// It reads only the level it's handed — the live, on-screen one — so sounds a
// background-sim planet emits are never heard. The Seq cursor guarantees each
// event plays exactly once even though events linger in the slice for their TTL.
type worldBridge struct {
	mixer  player
	tagMap map[world.SoundTag]string

	level  *world.Level // the level the cursor belongs to
	cursor uint64       // highest SoundSeq already played on that level

	// Footstep mutes silence the audible footstep of one side while the sound
	// event still fires (AI hearing is unaffected). Zero value = audible.
	muteFriendlyFootsteps bool
	muteEnemyFootsteps    bool
}

// play emits audio for new sounds on lvl, gated to the given viewport. It's a
// no-op until a viewport exists, and resets its cursor when the live level
// changes (so a freshly entered level doesn't replay the backlog already
// sitting in its slice).
func (b *worldBridge) play(lvl *world.Level, vp world.Viewport) {
	if b == nil || lvl == nil {
		return
	}
	if lvl != b.level {
		// New live level: skip whatever is already buffered (older sim/gen
		// sounds) and start listening from here.
		b.level = lvl
		b.cursor = lvl.SoundSeq
		return
	}

	vw, vh := vp.W, vp.H
	if vw <= 0 || vh <= 0 {
		// No viewport yet (first frame before a draw); don't strand the cursor.
		b.cursor = lvl.SoundSeq
		return
	}
	cx := vp.X + vw/2
	cy := vp.Y + vh/2

	for i := range lvl.Sounds {
		ev := &lvl.Sounds[i]
		if ev.Seq <= b.cursor {
			continue
		}
		// An explicit clip (resolved from an entity's SoundComponent + gear) wins
		// when it's actually loaded; otherwise fall back to the tag's generic
		// clip. The fallback lets a composed key like "impact_metal_heavy" degrade
		// to the plain impact sound when that asset set isn't present.
		key := ev.Clip
		if key == "" || !b.mixer.Has(key) {
			k, ok := b.tagMap[ev.Tag]
			if !ok {
				continue // no loaded explicit clip and tag not mapped
			}
			key = k
		}
		// V1: only the viewed z-slice is audible; cross-z muffling (via
		// world.VerticalSoundFalloff) is a future refinement.
		if ev.Z != vp.Z {
			continue
		}
		// Hard-gate to the on-screen rect.
		if ev.X < vp.X || ev.X >= vp.X+vw ||
			ev.Y < vp.Y || ev.Y >= vp.Y+vh {
			continue
		}
		vol := frustumVolume(ev.X, ev.Y, cx, cy, vw, vh)
		if ev.Tag == world.SoundTagFootstep {
			// Colonists/player units carry Worker; everything else that walks
			// (mobs, wildlife) is "enemy" for this toggle.
			friendly := ev.Source != nil && ev.Source.HasComponent(components.Worker)
			if friendly && b.muteFriendlyFootsteps {
				continue
			}
			if !friendly && b.muteEnemyFootsteps {
				continue
			}
			vol *= footstepVolumeScale
		}
		if vol < minAudibleVolume {
			continue
		}
		b.mixer.Play(key, mlaudio.PlayOptions{Bus: mlaudio.BusSFX, Volume: vol})
	}
	b.cursor = lvl.SoundSeq
}

// frustumVolume returns a 0..1 gain for a tile at (x,y) given the view center
// (cx,cy) and view size: 1.0 at the center, falling linearly with distance to
// about (1-edgeRolloff) at a view corner.
func frustumVolume(x, y, cx, cy, vw, vh int) float64 {
	half := math.Hypot(float64(vw)/2, float64(vh)/2)
	if half <= 0 {
		return 1
	}
	dist := math.Hypot(float64(x-cx), float64(y-cy))
	v := 1 - edgeRolloff*(dist/half)
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
