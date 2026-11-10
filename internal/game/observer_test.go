package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/eventsystem"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
)

func useObserver(t *testing.T) {
	t.Helper()
	cfg := config.Global()
	prev := cfg.ObserverMode
	cfg.ObserverMode = true
	observerLock = nil
	t.Cleanup(func() { cfg.ObserverMode, observerLock = prev, nil })
}

// The observer lock on a normal campaign: the director takes the
// settlement over from the world step, the player's order paths from the
// HUD and the marquee give nothing, and the HUD shows the marker. The ship
// and its popups stay the player's: an incident raises its popup and the
// Star Map pauses (observer mode v2; v1 answered Ignore and refused the
// pause).
func TestObserverMode(t *testing.T) {
	useContentTables(t)
	useRealCosts(t)
	useObserver(t)
	c := newE2EColony(t)
	ms := c.ms

	// The cursor's paths give no orders.
	ms.CursorMode = gui.CursorModeDig
	ms.marqueeStartX, ms.marqueeStartY, ms.marqueeEndX, ms.marqueeEndY = 0, 0, 5, 4
	ms.commitMarquee()
	_ = ms.HandleEvent(gui.CraftRequestedEvent{RecipeID: "medkit", Station: nil})
	_ = ms.HandleEvent(gui.ResearchRequestedEvent{TechKey: "basic_metallurgy", Station: nil})
	if n := len(ms.MainSettlement.Tasks.GetTasks()); n != 0 {
		t.Fatalf("%d orders from the player's paths in observer mode", n)
	}

	// The world step runs the director.
	c.addChest(t, 1, 1, map[string]int{"metal_ore": 20})
	for i := 0; i < 5; i++ {
		ms.stepWorld()
	}
	if ms.director == nil || ms.director.Ordered["build:bed"] == 0 {
		t.Fatalf("director did not order a bed: %+v", ms.director)
	}
	builds := 0
	for _, tk := range ms.MainSettlement.Tasks.GetTasks() {
		if tk.Action == task_requests.BuildAction {
			builds++
		}
	}
	if builds == 0 {
		t.Fatal("no build task on the settlement")
	}

	// An incident is the player's: its popup comes up.
	c.wm.noteAwayDamage(ms, eventsystem.EntityDamagedEvent{Victim: c.colonist})
	if c.wm.ActiveIncident() == nil || c.wm.observerIgnored != 0 {
		t.Error("the lock answered the player's incident")
	}
	c.wm.IgnoreIncident()

	// The Star Map pauses.
	d := NewDashboardState(c.wm.Campaign, c.wm)
	d.starMapPanel().togglePause()
	if !c.wm.Paused() {
		t.Error("the Star Map did not pause under the lock")
	}
	c.wm.SetPaused(false)

	// The marker.
	ms.Update()
	if !ms.guiManager.ObserverIndicator() {
		t.Error("no observer marker")
	}
}

// Without the flag nothing changes: no director, the marquee orders.
func TestObserverOffByDefault(t *testing.T) {
	if config.Global().ObserverMode {
		t.Skip("observerMode set in the local config")
	}
	c := newE2EColony(t)
	for i := 0; i < 5; i++ {
		c.ms.stepWorld()
	}
	if c.ms.director != nil {
		t.Error("director runs with observer mode off")
	}
	c.ms.Update()
	if c.ms.guiManager.ObserverIndicator() {
		t.Error("observer marker with observer mode off")
	}
}

// The stats count living colonists, deaths, beds, the colony's containers,
// food stored or carried, open orders, techs and finished builds.
func TestSettlementStats(t *testing.T) {
	useContentTables(t)
	useRealCosts(t)
	c := newE2EColony(t)
	ms := c.ms
	c.addChest(t, 4, 1, map[string]int{"metal_ore": 10})
	ration, _ := factory.Create("ration_pack", 0, 0, 0)
	ms.level.Entities[len(ms.level.Entities)-1].GetComponent(components.Storage).(*components.StorageComponent).AddItem(ration)
	bed, _ := factory.Create("bed", 10, 2, 0)
	ms.level.AddEntity(bed)
	ms.queueBuild("storage_locker", 14, 2, 0)
	st := ms.settlementStats()
	if st.Alive != 1 || st.Dead != 0 || st.Beds != 1 || st.Lockers != 1 || st.Food != 1 || st.BuildQueue != 1 {
		t.Fatalf("stats %+v", st)
	}
	c.tickUntil(t, "locker built", func() bool { return ms.settlementStats().Built["storage_locker"] == 1 })
	if st := ms.settlementStats(); st.Lockers != 2 || st.BuildQueue != 0 {
		t.Errorf("after the build: %+v", st)
	}
}
