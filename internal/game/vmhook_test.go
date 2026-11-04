//go:build ebitenginevmguest

package game

import (
	"encoding/json"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/vmproto"
)

// The guest hook's state summary and targets, on a tutorial expedition. Run
// with: go test -tags ebitenginevmguest ./internal/game
func TestVMHookStateAndTargets(t *testing.T) {
	w := newTutorialWorld(t, 5)
	t.Cleanup(func() { vm.cur = nil })

	vm.cur = NewTitleState()
	st := vmHandle(vmproto.Request{Op: vmproto.OpState}).State
	if st["state"] != "title" || st["title_screen"] != "main" {
		t.Errorf("title state = %v", st)
	}
	if rep := vmHandle(vmproto.Request{Op: vmproto.OpTarget, Name: "title:new"}); !rep.OK || rep.W != 220 {
		t.Errorf("title:new = %+v", rep)
	}
	if rep := vmHandle(vmproto.Request{Op: vmproto.OpTarget, Name: "title:launch"}); rep.OK {
		t.Error("title:launch resolved on the main screen")
	}

	w.d.selectTabByID("starmap")
	w.b.step("dash:starmap", nil, w.d)
	vm.cur = w.d
	st = vmHandle(vmproto.Request{Op: vmproto.OpState}).State
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
		if rep := vmHandle(vmproto.Request{Op: vmproto.OpTarget, Name: name}); !rep.OK {
			t.Errorf("%s: %s", name, rep.Error)
		}
	}
	// The Crew tab sits right of the Star Map tab, inside the tab bar.
	sm := vmHandle(vmproto.Request{Op: vmproto.OpTarget, Name: "tab:starmap"})
	crew := vmHandle(vmproto.Request{Op: vmproto.OpTarget, Name: "tab:crew"})
	if crew.X != sm.X+sm.W || crew.Y != 6 {
		t.Errorf("tabs starmap %+v crew %+v", sm, crew)
	}
	names := vmHandle(vmproto.Request{Op: vmproto.OpTargets}).Targets
	if len(names) < 5 {
		t.Errorf("targets = %v", names)
	}

	w.enterSite()
	vm.cur = w.site
	st = vmHandle(vmproto.Request{Op: vmproto.OpState}).State
	if st["state"] != "site" || st["campaign"].(map[string]any)["in_orbit"] != true {
		t.Errorf("site state = %v", st)
	}
	if rep := vmHandle(vmproto.Request{Op: vmproto.OpTarget, Name: "hud:cat_orders"}); !rep.OK {
		t.Errorf("hud:cat_orders: %s", rep.Error)
	}
	if rep := vmHandle(vmproto.Request{Op: vmproto.OpTarget, Name: "nope:x"}); rep.OK || rep.Error == "" {
		t.Errorf("unknown target = %+v", rep)
	}

	// debug incident raises the popup and pauses the clock.
	vm.cur = w.d
	if rep := vmHandle(vmproto.Request{Op: vmproto.OpDebug, Cmd: "incident ship"}); !rep.OK {
		t.Fatalf("incident: %s", rep.Error)
	}
	st = vmHandle(vmproto.Request{Op: vmproto.OpState}).State
	if st["incident"] == nil || !w.wm.ClockStopped() {
		t.Errorf("incident state = %v, clock stopped %v", st["incident"], w.wm.ClockStopped())
	}
	w.wm.IgnoreIncident()
}
