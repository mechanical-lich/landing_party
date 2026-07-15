package audio

import (
	"math"

	"github.com/mechanical-lich/landing_party/internal/world"
	mlaudio "github.com/mechanical-lich/mlge/audio"
)

// edgeRolloff controls how much a sound quiets toward the edge of the view: at
// the center a sound plays at full volume, at a view corner at (1 - edgeRolloff).
const edgeRolloff = 0.6

// minAudibleVolume is the volume below which a positional sound is dropped
// rather than played (inaudible, not worth a voice).
const minAudibleVolume = 0.05

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
}

// play emits audio for new sounds on lvl. It's a no-op until a viewport exists,
// and resets its cursor when the live level changes (so a freshly entered level
// doesn't replay the backlog already sitting in its slice).
func (b *worldBridge) play(lvl *world.Level) {
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

	vw, vh := lvl.ViewW, lvl.ViewH
	if vw <= 0 || vh <= 0 {
		// No viewport yet (first frame before a draw); don't strand the cursor.
		b.cursor = lvl.SoundSeq
		return
	}
	cx := lvl.CameraX + vw/2
	cy := lvl.CameraY + vh/2

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
		if ev.Z != lvl.CameraZ {
			continue
		}
		// Hard-gate to the on-screen rect.
		if ev.X < lvl.CameraX || ev.X >= lvl.CameraX+vw ||
			ev.Y < lvl.CameraY || ev.Y >= lvl.CameraY+vh {
			continue
		}
		vol := frustumVolume(ev.X, ev.Y, cx, cy, vw, vh)
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
