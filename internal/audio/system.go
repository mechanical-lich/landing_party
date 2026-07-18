package audio

import (
	"log"

	mlaudio "github.com/mechanical-lich/mlge/audio"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/ui/minui"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// global is the most recently constructed System, so gameplay code (e.g. the
// MainState update) can reach positional audio without threading a reference,
// matching the effect.GetEffectManager() idiom. nil when audio is disabled.
var global *System

// System is the game's audio front end: it owns the mixer, the UI-feedback
// director, and the positional world bridge, and is the single thing
// internal/game talks to. Build it once at startup and tick Update once per
// frame.
// combatCooldownFrames is how long (in ~60fps Update calls) combat music holds
// after the last colonist combat before fading back to the base state — 10s.
const combatCooldownFrames = 600

type System struct {
	mixer    *mlaudio.Mixer
	director *director
	world    *worldBridge

	music          *mlaudio.MusicDirector
	baseMusicState string // "menu" or "field"; combat overrides it while active
	combatCooldown int    // frames of combat music remaining
}

// New builds the audio system from a sound-map file, loads its clips, and
// subscribes the director to the global event bus for the mapped UI events.
//
// Audio is non-critical: a missing config file, an unreadable asset, or an
// unknown format is logged and skipped rather than failing startup, so the game
// always runs (just quietly).
func New(configPath string) (*System, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	mixer := mlaudio.NewMixer()
	loaded, total := 0, 0
	for key, paths := range cfg.Clips {
		for _, path := range paths {
			total++
			t, err := mlaudio.MusicTypeFromExt(path)
			if err != nil {
				log.Printf("audio: skipping clip %q: %v", key, err)
				continue
			}
			if err := mixer.Load(key, path, t); err != nil {
				// A fresh checkout has no audio assets yet; don't spam a line per
				// file. The one-line summary below reports the shortfall.
				continue
			}
			loaded++
		}
	}
	if loaded < total {
		log.Printf("audio: loaded %d/%d clip files (missing assets play silently)", loaded, total)
	}

	d := &director{mixer: mixer, uiMap: make(map[event.EventType]string)}
	for evt, clipKey := range cfg.UI {
		// Map the kind regardless of whether its clip loaded — the mixer no-ops
		// on a missing key, and mapping now means the sound "just works" once the
		// asset is dropped in without a config change.
		d.uiMap[event.EventType(evt)] = clipKey
	}
	// Drive UI feedback off minui's synchronous input hook (fires before the
	// widget's handler), so sounds are immediate even when a click kicks off slow
	// work like world generation.
	minui.InteractionSound = d.play

	wb := &worldBridge{mixer: mixer, tagMap: make(map[world.SoundTag]string)}
	for tag, clipKey := range cfg.World {
		wb.tagMap[world.SoundTag(tag)] = clipKey
	}

	music := mlaudio.NewMusicDirector()
	musicTracks := loadMusicTracks(cfg.Music, music)

	s := &System{
		mixer:          mixer,
		director:       d,
		world:          wb,
		music:          music,
		baseMusicState: "menu",
	}
	if musicTracks == 0 {
		s.music = nil // nothing to play; keep Update cheap
	}
	global = s
	return s, nil
}

// loadMusicTracks decodes each state's tracks (a file shared across states loads
// once) and registers them with the director. Returns how many track slots were
// registered so the caller can skip a music-less director.
func loadMusicTracks(playlists map[string][]string, music *mlaudio.MusicDirector) int {
	loaded, total := 0, 0
	resByPath := map[string]*mlaudio.AudioResource{}
	for state, paths := range playlists {
		for _, path := range paths {
			total++
			res, ok := resByPath[path]
			if !ok {
				t, err := mlaudio.MusicTypeFromExt(path)
				if err != nil {
					log.Printf("audio: skipping music %q: %v", path, err)
					resByPath[path] = nil
					continue
				}
				res, err = mlaudio.LoadAudioFromFile(path, t)
				if err != nil {
					resByPath[path] = nil
					continue
				}
				resByPath[path] = res
			}
			if res == nil {
				continue
			}
			music.AddTrack(state, res)
			loaded++
		}
	}
	if loaded < total {
		log.Printf("audio: loaded %d/%d music tracks (missing tracks are silent)", loaded, total)
	}
	return loaded
}

// PlayWorldSounds plays positional audio for any new in-world sounds on the live
// level. Call once per frame from the gameplay state; safe when audio is
// disabled (global nil) or the System/level is nil.
func PlayWorldSounds(lvl *world.Level) {
	if global == nil {
		return
	}
	global.world.play(lvl)
}

// Update advances the mixer and the music director. Call once per frame.
func (s *System) Update() {
	if s == nil {
		return
	}
	s.mixer.Update()
	if s.music != nil {
		// Combat music overrides the base state while its cooldown is running.
		if s.combatCooldown > 0 {
			s.combatCooldown--
			s.music.SetState("combat")
		} else {
			s.music.SetState(s.baseMusicState)
		}
		s.music.Update()
	}
}

// SetMusicState sets the base music state ("menu" or "field"). Combat overrides
// it automatically (see NotifyCombat). Safe when audio is disabled.
func SetMusicState(state string) {
	if global != nil {
		global.baseMusicState = state
	}
}

// NotifyCombat marks that colonist combat just happened, switching to combat
// music and holding it for combatCooldownFrames after the last call.
func NotifyCombat() {
	if global != nil {
		global.combatCooldown = combatCooldownFrames
	}
}

// Mixer exposes the underlying engine for later phases (positional SFX, music,
// global one-shots) and for settings screens to drive bus volumes.
func (s *System) Mixer() *mlaudio.Mixer {
	if s == nil {
		return nil
	}
	return s.mixer
}

// perceptualGain maps a 0..1 slider position to a bus gain along a squared
// curve. Loudness perception is roughly logarithmic, so a linear slider feels
// dead until it's near zero; squaring makes the middle of the slider audibly
// quieter (0.5 → ~-12 dB) so it actually feels like a volume control.
func perceptualGain(v float64) float64 {
	if v < 0 {
		v = 0
	} else if v > 1 {
		v = 1
	}
	return v * v
}

// setBus sets a bus volume on the active audio system (no-op when disabled).
// The slider value is passed through the perceptual curve first.
func setBus(bus mlaudio.Bus, v float64) {
	if global != nil {
		global.mixer.SetBusVolume(bus, perceptualGain(v))
	}
}

// SetUIVolume, SetGameVolume and SetMusicVolume map the settings screen's three
// sliders onto their buses. Volumes are 0..1.
func SetUIVolume(v float64)   { setBus(mlaudio.BusUI, v) }
func SetGameVolume(v float64) { setBus(mlaudio.BusSFX, v) }

// SetMusicVolume drives the streaming music director (music doesn't ride the
// mixer's buses), through the same perceptual curve as the other sliders.
func SetMusicVolume(v float64) {
	setBus(mlaudio.BusMusic, v) // keep the bus in sync (harmless)
	if global != nil && global.music != nil {
		global.music.SetVolume(perceptualGain(v))
	}
}

// ApplyVolumes pushes all three volumes at once — call on startup (from config)
// and after the settings screen commits a change.
func ApplyVolumes(ui, game, music float64) {
	SetUIVolume(ui)
	SetGameVolume(game)
	SetMusicVolume(music)
}

// SetFootstepAudio toggles whether friendly (colonist/player) and enemy
// footsteps are audible. Off silences the sound but not its event, so AI hearing
// is unchanged. Call on startup (from config) and after a settings change.
func SetFootstepAudio(friendly, enemy bool) {
	if global != nil {
		global.world.muteFriendlyFootsteps = !friendly
		global.world.muteEnemyFootsteps = !enemy
	}
}

// PlayGlobal plays a positionless one-shot on the Global bus — an alert or
// notification heard anywhere, ignoring the camera view (unlike world SFX).
// No-op when audio is disabled or the clip isn't loaded.
func PlayGlobal(key string) {
	if global != nil {
		global.mixer.Play(key, mlaudio.PlayOptions{Bus: mlaudio.BusGlobal})
	}
}
