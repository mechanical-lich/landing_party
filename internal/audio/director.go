package audio

import (
	mlaudio "github.com/mechanical-lich/mlge/audio"
	"github.com/mechanical-lich/mlge/event"
)

// player is the slice of *mlge/audio.Mixer the director needs, pulled behind an
// interface so it can be tested without a real mixer or audio device.
type player interface {
	Play(key string, opts mlaudio.PlayOptions)
}

// director turns UI interactions into sounds. It's registered as
// minui.InteractionSound, so it fires synchronously the instant a widget
// registers a user action — before that widget's handler runs — making feedback
// immediate even when the handler does slow work (e.g. generating a world).
//
// Because it's driven by real input paths rather than the event bus, it never
// reacts to programmatic setters (a title-screen SelectByIndex during setup, a
// button re-populating a dropdown), so there's no startup or side-effect chirp
// to suppress. The event→clip mapping is the game's policy (data/audio.json);
// minui stays audio-agnostic.
type director struct {
	mixer player
	uiMap map[event.EventType]string // UI interaction kind → clip key
}

// play is the minui.InteractionSound hook: play the clip mapped to kind on the
// UI bus. Unmapped kinds and unknown clips are ignored (the mixer no-ops on a
// missing key).
func (d *director) play(kind event.EventType, _ string) {
	if key, ok := d.uiMap[kind]; ok {
		d.mixer.Play(key, mlaudio.PlayOptions{Bus: mlaudio.BusUI})
	}
}
