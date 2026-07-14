package audio

import (
	"log"

	mlaudio "github.com/mechanical-lich/mlge/audio"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/ui/minui"
)

// System is the game's audio front end: it owns the mixer and the event
// director, and is the single thing internal/game talks to. Build it once at
// startup and tick Update once per frame.
type System struct {
	mixer    *mlaudio.Mixer
	director *director
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
	loaded := 0
	for key, path := range cfg.Clips {
		t, err := mlaudio.MusicTypeFromExt(path)
		if err != nil {
			log.Printf("audio: skipping clip %q: %v", key, err)
			continue
		}
		if err := mixer.Load(key, path, t); err != nil {
			// A fresh checkout has no audio assets yet; don't spam a line per
			// clip. The one-line summary below reports the shortfall.
			continue
		}
		loaded++
	}
	if loaded < len(cfg.Clips) {
		log.Printf("audio: loaded %d/%d clips (missing assets play silently)", loaded, len(cfg.Clips))
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

	return &System{mixer: mixer, director: d}, nil
}

// Update advances the mixer (reclaims finished voices, restarts loops). Call
// once per frame from the game loop.
func (s *System) Update() {
	if s == nil {
		return
	}
	s.mixer.Update()
}

// Mixer exposes the underlying engine for later phases (positional SFX, music,
// global one-shots) and for settings screens to drive bus volumes.
func (s *System) Mixer() *mlaudio.Mixer {
	if s == nil {
		return nil
	}
	return s.mixer
}
