package game

import "testing"

func assertPauseIndicator(t *testing.T, ms *MainState, want string) {
	t.Helper()
	ms.guiManager.SetPauseIndicator(ms.pauseIndicatorText())
	visible, text := ms.guiManager.PauseIndicator()
	if visible != (want != "") || text != want {
		t.Errorf("indicator visible=%v %q, want %q", visible, text, want)
	}
}

func TestPauseIndicatorNamesEachCause(t *testing.T) {
	wm, live := bgAway(t, "a")
	assertPauseIndicator(t, live, "")

	live.Paused = true // Space
	assertPauseIndicator(t, live, "PAUSED")
	live.Paused = false
	assertPauseIndicator(t, live, "")

	p := newStarMapPanel(wm.Campaign, wm)
	p.togglePause()
	assertPauseIndicator(t, live, "PAUSED - star map")
	p.togglePause()
	assertPauseIndicator(t, live, "")

	wm.incidents = []Incident{{LocationID: "a", Where: "a", Victim: "v"}}
	assertPauseIndicator(t, live, "PAUSED - incident")
	wm.IgnoreIncident()
	assertPauseIndicator(t, live, "")

	// The popup's Pause leaves the clock stopped, and says why.
	wm.incidents = []Incident{{LocationID: "a", Where: "a", Victim: "v"}}
	wm.PauseIncident()
	live.adoptWorldPause()
	assertPauseIndicator(t, live, "PAUSED - incident")
}

// A real frame on arrival shows the Star Map's pause, with no key pressed.
func TestStarMapPauseShowsOnArrival(t *testing.T) {
	wm, _ := bgAway(t, "a")
	newStarMapPanel(wm.Campaign, wm).togglePause()
	ms, err := wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ms.teardown)
	ms.Update()
	if visible, text := ms.guiManager.PauseIndicator(); !visible || text != "PAUSED - star map" {
		t.Errorf("indicator visible=%v %q", visible, text)
	}
	if !ms.Paused || wm.Paused() {
		t.Errorf("pause not adopted: level %v, world %v", ms.Paused, wm.Paused())
	}
	// Leaving hands the reason back; returning shows it again.
	ms.openDashboard()
	if ms, err = wm.EnterCurrent(); err != nil {
		t.Fatal(err)
	}
	ms.Update()
	if _, text := ms.guiManager.PauseIndicator(); text != "PAUSED - star map" {
		t.Errorf("after a round trip: %q", text)
	}
}
