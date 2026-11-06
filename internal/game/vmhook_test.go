//go:build ebitenginevmguest

package game

import (
	"encoding/json"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/mlge/state"
)

// The guest hook's state summary and targets, on a tutorial expedition. Run
// with: go test -tags ebitenginevmguest ./internal/game
func TestVMHookStateAndTargets(t *testing.T) {
	w := newTutorialWorld(t, 5)
	h := vmHook(&Game{}).(*lpHook)
	// show makes s the current state, as a push from the running state would.
	show := func(s state.StateInterface) { h.g.StateMachine.PushState(s) }
	stateOf := func() map[string]any { return h.State().(map[string]any) }

	show(NewTitleState())
	st := stateOf()
	if st["state"] != "title" || st["title_screen"] != "main" {
		t.Errorf("title state = %v", st)
	}
	if r, ok := h.Target("title:new"); !ok || r.W != 220 {
		t.Errorf("title:new = %+v %v", r, ok)
	}
	if _, ok := h.Target("title:launch"); ok {
		t.Error("title:launch resolved on the main screen")
	}

	w.d.selectTabByID("starmap")
	w.b.step("dash:starmap", nil, w.d)
	show(w.d)
	st = stateOf()
	camp := st["campaign"].(map[string]any)
	if st["state"] != "dashboard" || st["dash_tab"] != "starmap" || camp["adrift"] != true {
		t.Errorf("dashboard state = %v", st)
	}
	if crew := st["crew"].([]any); len(crew) == 0 || crew[0].(map[string]any)["where"] != "ship" {
		t.Errorf("crew = %v", crew)
	}
	if tut := st["tutorial"].(map[string]any); tut["count"] != w.b.dir.Count() {
		t.Errorf("tutorial = %v", tut)
	}
	if _, err := json.Marshal(st); err != nil {
		t.Errorf("state does not marshal: %v", err)
	}
	for _, name := range []string{"tab:crew", "dash:travelBtn", "row:locList:" + campaign.TutorialLocationID, "dash:saveBtn"} {
		if _, ok := h.Target(name); !ok {
			t.Errorf("%s does not resolve", name)
		}
	}
	// The Crew tab sits right of the Star Map tab, inside the tab bar.
	sm, _ := h.Target("tab:starmap")
	crew, _ := h.Target("tab:crew")
	if crew.X != sm.X+sm.W || crew.Y != 6 {
		t.Errorf("tabs starmap %+v crew %+v", sm, crew)
	}
	var names []string
	for _, n := range h.Candidates() {
		if _, ok := h.Target(n); ok {
			names = append(names, n)
		}
	}
	if len(names) < 5 {
		t.Errorf("targets = %v", names)
	}

	w.enterSite()
	show(w.site)
	st = stateOf()
	if st["state"] != "site" || st["campaign"].(map[string]any)["in_orbit"] != true {
		t.Errorf("site state = %v", st)
	}
	if _, ok := h.Target("hud:cat_orders"); !ok {
		t.Error("hud:cat_orders does not resolve")
	}
	if r, ok := h.Target("nope:x"); ok {
		t.Errorf("unknown target = %+v", r)
	}

	// debug incident raises the popup and pauses the clock.
	show(w.d)
	if _, err := h.Debug("incident ship"); err != nil {
		t.Fatalf("incident: %v", err)
	}
	st = stateOf()
	if st["incident"] == nil || !w.wm.ClockStopped() {
		t.Errorf("incident state = %v, clock stopped %v", st["incident"], w.wm.ClockStopped())
	}
	w.wm.IgnoreIncident()
}
