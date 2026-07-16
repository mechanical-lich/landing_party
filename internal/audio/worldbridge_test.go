package audio

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	mlaudio "github.com/mechanical-lich/mlge/audio"
	"github.com/mechanical-lich/mlge/ecs"
)

// newBridgeLevel builds a level whose camera view covers the whole map, so
// tile coordinates map directly onto the frustum.
func newBridgeLevel(w, h, d int) *world.Level {
	lvl := world.NewLevel(w, h, d)
	lvl.CameraX, lvl.CameraY, lvl.CameraZ = 0, 0, 0
	lvl.ViewW, lvl.ViewH = w, h
	return lvl
}

func newBridge(rec *recordingPlayer) *worldBridge {
	return &worldBridge{
		mixer: rec,
		tagMap: map[world.SoundTag]string{
			world.SoundTagGunshot: "sfx_gunshot",
			world.SoundTagImpact:  "sfx_impact",
		},
	}
}

// attach performs the first play() call, which binds the bridge to the level and
// skips whatever is already buffered. Sounds emitted after this are heard.
func attach(b *worldBridge, lvl *world.Level) { b.play(lvl) }

// TestWorldBridgePlaysMappedInFrustum: a mapped sound at the camera z inside the
// view plays on the SFX bus.
func TestWorldBridgePlaysMappedInFrustum(t *testing.T) {
	rec := &recordingPlayer{}
	b := newBridge(rec)
	lvl := newBridgeLevel(20, 20, 4)
	attach(b, lvl)

	lvl.EmitSound(10, 10, 0, 8, world.SoundTagGunshot, nil) // near center
	b.play(lvl)

	if len(rec.plays) != 1 {
		t.Fatalf("expected 1 play, got %d", len(rec.plays))
	}
	if rec.plays[0].key != "sfx_gunshot" {
		t.Fatalf("key = %q, want sfx_gunshot", rec.plays[0].key)
	}
	if rec.plays[0].bus != mlaudio.BusSFX {
		t.Fatalf("bus = %v, want BusSFX", rec.plays[0].bus)
	}
	if rec.plays[0].vol <= 0 || rec.plays[0].vol > 1 {
		t.Fatalf("vol = %v, want in (0,1]", rec.plays[0].vol)
	}
}

// TestWorldBridgeGatesOffscreenAndCrossZ: sounds outside the view rect or on a
// different z than the camera are dropped.
func TestWorldBridgeGatesOffscreenAndCrossZ(t *testing.T) {
	rec := &recordingPlayer{}
	b := newBridge(rec)
	lvl := newBridgeLevel(20, 20, 4)
	// Camera looks at the top-left quadrant only.
	lvl.CameraX, lvl.CameraY, lvl.CameraZ = 0, 0, 1
	lvl.ViewW, lvl.ViewH = 8, 8
	attach(b, lvl)

	lvl.EmitSound(15, 15, 1, 8, world.SoundTagGunshot, nil) // off-screen (x,y)
	lvl.EmitSound(4, 4, 3, 8, world.SoundTagGunshot, nil)   // in view x,y but wrong z
	lvl.EmitSound(4, 4, 1, 8, world.SoundTagImpact, nil)    // in view + camera z → heard
	b.play(lvl)

	if len(rec.plays) != 1 {
		t.Fatalf("expected only the in-view, on-z sound, got %d plays", len(rec.plays))
	}
	if rec.plays[0].key != "sfx_impact" {
		t.Fatalf("key = %q, want sfx_impact", rec.plays[0].key)
	}
}

// TestWorldBridgeIgnoresUnmappedTag: a tag with no clip mapping is silent.
func TestWorldBridgeIgnoresUnmappedTag(t *testing.T) {
	rec := &recordingPlayer{}
	b := newBridge(rec)
	lvl := newBridgeLevel(20, 20, 4)
	attach(b, lvl)

	lvl.EmitSound(10, 10, 0, 8, world.SoundTagFootstep, nil) // not in tagMap
	b.play(lvl)

	if len(rec.plays) != 0 {
		t.Fatalf("unmapped tag produced %d plays, want 0", len(rec.plays))
	}
}

// TestWorldBridgeCursorPlaysEachSoundOnce: replaying without new emits is silent
// (the Seq cursor advances), and the backlog on a fresh level is skipped.
func TestWorldBridgeCursorPlaysEachSoundOnce(t *testing.T) {
	rec := &recordingPlayer{}
	b := newBridge(rec)
	lvl := newBridgeLevel(20, 20, 4)

	// Backlog present BEFORE the bridge attaches → must be skipped.
	lvl.EmitSound(10, 10, 0, 8, world.SoundTagGunshot, nil)
	attach(b, lvl)
	b.play(lvl)
	if len(rec.plays) != 0 {
		t.Fatalf("backlog before attach should be skipped, got %d plays", len(rec.plays))
	}

	// New sound after attach → heard exactly once.
	lvl.EmitSound(11, 10, 0, 8, world.SoundTagImpact, nil)
	b.play(lvl)
	b.play(lvl) // no new emits → no additional play
	if len(rec.plays) != 1 {
		t.Fatalf("expected the post-attach sound once, got %d", len(rec.plays))
	}
}

// TestWorldBridgeSwitchingLevelSkipsBacklog: handing the bridge a different level
// rebinds it and skips that level's existing sounds.
func TestWorldBridgeSwitchingLevelSkipsBacklog(t *testing.T) {
	rec := &recordingPlayer{}
	b := newBridge(rec)

	a := newBridgeLevel(20, 20, 4)
	attach(b, a)
	a.EmitSound(10, 10, 0, 8, world.SoundTagGunshot, nil)
	b.play(a) // heard on level a

	other := newBridgeLevel(20, 20, 4)
	other.EmitSound(10, 10, 0, 8, world.SoundTagGunshot, nil) // backlog on the new level
	b.play(other)                                             // rebinds, skips backlog
	if got := len(rec.plays); got != 1 {
		t.Fatalf("switching levels should not replay the new level's backlog, got %d plays", got)
	}
}

// TestWorldBridgeExplicitClipWins: an event carrying an explicit clip plays that
// clip even when its tag has no mapping (entity-resolved sounds bypass the tag
// table).
func TestWorldBridgeExplicitClipWins(t *testing.T) {
	rec := &recordingPlayer{}
	b := newBridge(rec) // tagMap knows only gunshot + impact
	lvl := newBridgeLevel(20, 20, 4)
	attach(b, lvl)

	// "scream" is unmapped here, but the explicit clip carries it through.
	lvl.EmitSoundClip(10, 10, 0, 8, world.SoundTagScream, "worm_death", nil)
	b.play(lvl)

	if len(rec.plays) != 1 {
		t.Fatalf("expected 1 play, got %d", len(rec.plays))
	}
	if rec.plays[0].key != "worm_death" {
		t.Fatalf("key = %q, want worm_death (explicit clip)", rec.plays[0].key)
	}
}

// TestWorldBridgeComposedClipPlaysWhenLoaded: a composed impact key plays
// directly when its variation set is registered.
func TestWorldBridgeComposedClipPlaysWhenLoaded(t *testing.T) {
	rec := &recordingPlayer{known: map[string]bool{"impact_metal_heavy": true}}
	b := &worldBridge{mixer: rec, tagMap: map[world.SoundTag]string{world.SoundTagImpact: "sfx_impact"}}
	lvl := newBridgeLevel(20, 20, 4)
	attach(b, lvl)

	lvl.EmitSoundClip(10, 10, 0, 6, world.SoundTagImpact, "impact_metal_heavy", nil)
	b.play(lvl)

	if len(rec.plays) != 1 || rec.plays[0].key != "impact_metal_heavy" {
		t.Fatalf("expected impact_metal_heavy, got %+v", rec.plays)
	}
}

// TestWorldBridgeComposedClipFallsBackToTag: when the composed set isn't loaded,
// the bridge falls back to the tag's generic clip instead of going silent.
func TestWorldBridgeComposedClipFallsBackToTag(t *testing.T) {
	rec := &recordingPlayer{known: map[string]bool{"sfx_impact": true}} // composed key NOT loaded
	b := &worldBridge{mixer: rec, tagMap: map[world.SoundTag]string{world.SoundTagImpact: "sfx_impact"}}
	lvl := newBridgeLevel(20, 20, 4)
	attach(b, lvl)

	lvl.EmitSoundClip(10, 10, 0, 6, world.SoundTagImpact, "impact_wood_light", nil)
	b.play(lvl)

	if len(rec.plays) != 1 || rec.plays[0].key != "sfx_impact" {
		t.Fatalf("expected fallback to sfx_impact, got %+v", rec.plays)
	}
}

// TestWorldBridgeFootstepsAreHalfVolume: a footstep at the view center plays at
// half the volume a non-footstep would at the same spot.
func TestWorldBridgeFootstepsAreHalfVolume(t *testing.T) {
	rec := &recordingPlayer{}
	b := &worldBridge{
		mixer:  rec,
		tagMap: map[world.SoundTag]string{world.SoundTagImpact: "sfx_impact", world.SoundTagFootstep: "footstep_wood"},
	}
	lvl := newBridgeLevel(20, 20, 4)
	attach(b, lvl)

	cx, cy := lvl.CameraX+lvl.ViewW/2, lvl.CameraY+lvl.ViewH/2
	lvl.EmitSound(cx, cy, 0, 6, world.SoundTagImpact, nil)   // reference at full
	lvl.EmitSound(cx, cy, 0, 6, world.SoundTagFootstep, nil) // half
	b.play(lvl)

	if len(rec.plays) != 2 {
		t.Fatalf("expected 2 plays, got %d", len(rec.plays))
	}
	impact, footstep := rec.plays[0], rec.plays[1]
	if footstep.vol != impact.vol*0.5 {
		t.Fatalf("footstep vol = %v, want half of impact vol %v", footstep.vol, impact.vol)
	}
}

// TestWorldBridgeFootstepMutes: muting a side silences that side's footsteps
// while the other still plays. (The sound event still fires — the bridge only
// skips playback.)
func TestWorldBridgeFootstepMutes(t *testing.T) {
	friendly := &ecs.Entity{}
	friendly.AddComponent(&components.WorkerComponent{}) // colonists carry Worker
	enemy := &ecs.Entity{}                               // mob: no Worker

	emitBoth := func(lvl *world.Level) {
		cx, cy := lvl.CameraX+lvl.ViewW/2, lvl.CameraY+lvl.ViewH/2
		lvl.EmitSoundClip(cx, cy, 0, 2, world.SoundTagFootstep, "footstep_wood", friendly)
		lvl.EmitSoundClip(cx, cy, 0, 2, world.SoundTagFootstep, "footstep_wood", enemy)
	}

	// Mute friendly → only the enemy footstep plays.
	rec := &recordingPlayer{}
	b := &worldBridge{mixer: rec, tagMap: map[world.SoundTag]string{}, muteFriendlyFootsteps: true}
	lvl := newBridgeLevel(20, 20, 4)
	attach(b, lvl)
	emitBoth(lvl)
	b.play(lvl)
	if len(rec.plays) != 1 {
		t.Fatalf("mute-friendly: expected 1 play (enemy), got %d", len(rec.plays))
	}

	// Mute enemy → only the friendly footstep plays.
	rec2 := &recordingPlayer{}
	b2 := &worldBridge{mixer: rec2, tagMap: map[world.SoundTag]string{}, muteEnemyFootsteps: true}
	lvl2 := newBridgeLevel(20, 20, 4)
	attach(b2, lvl2)
	emitBoth(lvl2)
	b2.play(lvl2)
	if len(rec2.plays) != 1 {
		t.Fatalf("mute-enemy: expected 1 play (friendly), got %d", len(rec2.plays))
	}
}

// TestFrustumVolumeCenterLouderThanEdge locks the attenuation shape.
func TestFrustumVolumeCenterLouderThanEdge(t *testing.T) {
	center := frustumVolume(10, 10, 10, 10, 20, 20)
	edge := frustumVolume(0, 0, 10, 10, 20, 20)
	if center <= edge {
		t.Fatalf("center volume (%v) should exceed edge (%v)", center, edge)
	}
	if center != 1 {
		t.Fatalf("center volume = %v, want 1.0", center)
	}
	if edge < 0 || edge >= center {
		t.Fatalf("edge volume = %v, want in [0, center)", edge)
	}
}
