package audio

import (
	"testing"

	mlaudio "github.com/mechanical-lich/mlge/audio"
)

func TestPerceptualGainCurve(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{0, 0},
		{1, 1},
		{0.5, 0.25}, // middle of the slider is ~-12 dB, not -6
		{-0.5, 0},   // clamped
		{2, 1},      // clamped
	}
	for _, c := range cases {
		if got := perceptualGain(c.in); got != c.want {
			t.Fatalf("perceptualGain(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestVolumeHelpersDriveTheirBuses is the end-to-end wiring check: the settings
// screen's SetUIVolume/SetGameVolume/SetMusicVolume must land on the same
// mixer's UI/SFX/Music buses (through the perceptual curve) — i.e. the sliders
// really do control the sounds that play.
func TestVolumeHelpersDriveTheirBuses(t *testing.T) {
	prev := global
	defer func() { global = prev }()
	global = &System{mixer: mlaudio.NewMixer()}

	SetUIVolume(0.5)
	SetGameVolume(1)
	SetMusicVolume(0)

	if got := global.mixer.BusVolume(mlaudio.BusUI); got != 0.25 {
		t.Fatalf("BusUI = %v, want 0.25 (0.5 curved)", got)
	}
	if got := global.mixer.BusVolume(mlaudio.BusSFX); got != 1 {
		t.Fatalf("BusSFX = %v, want 1", got)
	}
	if got := global.mixer.BusVolume(mlaudio.BusMusic); got != 0 {
		t.Fatalf("BusMusic = %v, want 0 (muted)", got)
	}
}
