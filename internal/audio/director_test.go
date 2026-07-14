package audio

import (
	"testing"

	mlaudio "github.com/mechanical-lich/mlge/audio"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/ui/minui"
)

type recordingPlayer struct {
	plays []played
}

type played struct {
	key string
	bus mlaudio.Bus
}

func (r *recordingPlayer) Play(key string, opts mlaudio.PlayOptions) {
	r.plays = append(r.plays, played{key: key, bus: opts.Bus})
}

func newDirector(rec *recordingPlayer, uiMap map[event.EventType]string) *director {
	return &director{mixer: rec, uiMap: uiMap}
}

// TestDirectorPlaysMappedInteractionOnUIBus is the core behaviour: a mapped UI
// interaction plays its clip on the UI bus.
func TestDirectorPlaysMappedInteractionOnUIBus(t *testing.T) {
	rec := &recordingPlayer{}
	d := newDirector(rec, map[event.EventType]string{
		minui.EventTypeButtonClick: "click",
	})

	d.play(minui.EventTypeButtonClick, "start")

	if len(rec.plays) != 1 {
		t.Fatalf("expected 1 play, got %d", len(rec.plays))
	}
	if rec.plays[0].key != "click" {
		t.Fatalf("played key = %q, want \"click\"", rec.plays[0].key)
	}
	if rec.plays[0].bus != mlaudio.BusUI {
		t.Fatalf("played on bus %v, want BusUI", rec.plays[0].bus)
	}
}

// TestDirectorIgnoresUnmappedInteraction verifies interactions without a mapping
// are silent.
func TestDirectorIgnoresUnmappedInteraction(t *testing.T) {
	rec := &recordingPlayer{}
	d := newDirector(rec, map[event.EventType]string{
		minui.EventTypeButtonClick: "click",
	})

	// A list-box selection has no mapping here → no sound.
	d.play(minui.EventTypeListBoxSelect, "list")

	if len(rec.plays) != 0 {
		t.Fatalf("unmapped interaction produced %d plays, want 0", len(rec.plays))
	}
}
