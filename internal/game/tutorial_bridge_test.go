package game

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/gui"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/tutorial"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// tutorialWorld is a tutorial expedition with the bridge running, its dashboard
// built, and activeTutorial cleared at the end.
type tutorialWorld struct {
	t    *testing.T
	c    *campaign.Campaign
	wm   *WorldManager
	b    *tutorialBridge
	d    *DashboardState
	site *MainState

	ticks      int    // world ticks driven through tickOnce
	crewCount  int    // colonists expected on the live site
	lastReport string // crew state sampled every 100 ticks
}

func newTutorialWorld(t *testing.T, seed int64) *tutorialWorld {
	t.Helper()
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Walk", seed, true)
	if err != nil {
		t.Fatal(err)
	}
	if activeTutorial == nil {
		t.Fatal("startTutorial did not install the bridge")
	}
	t.Cleanup(func() { activeTutorial = nil })
	w := &tutorialWorld{t: t, c: c, wm: wm, b: activeTutorial}
	w.d = NewDashboardState(c, wm)
	return w
}

func (w *tutorialWorld) stepID() string {
	if s := w.b.dir.Current(); s != nil {
		return s.ID
	}
	return ""
}

// dash runs dashboard frames on a tab until the step changes, failing if it
// does not within a scan period and a half.
func (w *tutorialWorld) dash(tab, from string) {
	w.t.Helper()
	w.d.selectTabByID(tab)
	for i := 0; i < tutorialScanEvery+15; i++ {
		w.b.step("dash:"+tab, nil, w.d)
		if w.stepID() != from {
			return
		}
	}
	w.t.Fatalf("step %s did not complete on dash:%s", from, tab)
}

// dashStays runs a few dashboard frames and asserts the step did not move.
func (w *tutorialWorld) dashStays(tab, at string) {
	w.t.Helper()
	w.d.selectTabByID(tab)
	for i := 0; i < tutorialScanEvery+5; i++ {
		w.b.step("dash:"+tab, nil, w.d)
	}
	if w.stepID() != at {
		w.t.Fatalf("step moved from %s to %s with nothing done", at, w.stepID())
	}
}

// siteFrames runs site frames without ticking the world.
func (w *tutorialWorld) siteFrames(n int) {
	for i := 0; i < n; i++ {
		w.b.step(tutorial.ContextSite, w.site, nil)
	}
}

// tickUntilStep steps the world and the bridge until the step leaves from.
func (w *tutorialWorld) tickUntilStep(from string, max int) int {
	w.t.Helper()
	for i := 0; i < max; i++ {
		if w.stepID() != from {
			return i
		}
		w.tickOnce()
	}
	w.t.Fatalf("step %s not completed in %d ticks (stock %v) %s", from, max, w.siteStock(), w.crewReport())
	return 0
}

// tickOnce is one world tick plus one bridge frame, with a death watch: a
// colonist leaving the level fails the test at once with the crew's state
// shortly before, since a dead colonist is cleaned off the level.
func (w *tutorialWorld) tickOnce() {
	w.t.Helper()
	w.ticks++
	if w.ticks%100 == 0 {
		w.lastReport = w.crewReport()
	}
	// Keep the crew fed and rested. The walkthrough works them for tens of
	// thousands of ticks without a break, and a colonist who passes out while
	// starving cannot go and eat, so one dies with rations in the pad (a needs
	// bug outside the tutorial). The tutorial is what is under test here.
	if w.ticks%1500 == 0 {
		for _, e := range colonyWorkers(w.site) {
			if e.HasComponent(components.Needs) {
				n := e.GetComponent(components.Needs).(*components.NeedsComponent)
				n.Hunger, n.Exhaustion = 0, 0
			}
		}
	}
	fspath.ResetFrameCounter()
	w.site.stepWorld()
	w.b.step(tutorial.ContextSite, w.site, nil)
	if n := len(colonyWorkers(w.site)); n < w.crewCount {
		w.t.Fatalf("crew fell from %d to %d at tick %d; shortly before: %s; now: %s", w.crewCount, n, w.ticks, w.lastReport, w.crewReport())
	}
}

// crewReport describes what each colonist is doing and what is queued, for
// failure messages.
func (w *tutorialWorld) crewReport() string {
	var out strings.Builder
	for _, e := range w.site.level.Entities {
		if e == nil || e.Blueprint != "colonist" {
			continue
		}
		state, taskName, pos, extra := "?", "none", "?", ""
		if e.HasComponent(rlcomponents.AIMemory) {
			state = e.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent).State
		}
		if e.HasComponent(components.Worker) {
			if tk := e.GetComponent(components.Worker).(*components.WorkerComponent).CurrentTask; tk != nil {
				taskName = fmt.Sprintf("%s@%d,%d", tk.Action, tk.X, tk.Y)
			}
		}
		if e.HasComponent(rlcomponents.Position) {
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			pos = fmt.Sprintf("%d,%d,%d", pc.GetX(), pc.GetY(), pc.GetZ())
		}
		if e.HasComponent(components.Needs) {
			n := e.GetComponent(components.Needs).(*components.NeedsComponent)
			extra += fmt.Sprintf(" hunger=%d/%d", n.Hunger, n.MaxHunger)
		}
		if e.HasComponent(rlcomponents.Health) {
			hc := e.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
			extra += fmt.Sprintf(" hp=%d", hc.Health)
		}
		if e.HasComponent(rlcomponents.Dead) {
			extra += " DEAD"
		}
		if !components.IsColonyWorker(e, w.site.colonyName()) {
			extra += " not-colony-worker"
		}
		fmt.Fprintf(&out, " [%s %s at %s%s]", state, taskName, pos, extra)
	}
	counts := map[string]int{}
	for _, q := range w.site.MainSettlement.Tasks.GetTasks() {
		counts[string(q.Action)]++
	}
	fmt.Fprintf(&out, " queue=%v", counts)
	return out.String()
}

func (w *tutorialWorld) enterSite() {
	w.t.Helper()
	if w.c.IsAdrift() {
		// Every expedition starts adrift; a site needs a jump first.
		if err := w.wm.Travel(w.wm.Campaign.StartID); err != nil {
			w.t.Fatal(err)
		}
	}
	ms, err := w.wm.EnterCurrent()
	if err != nil {
		w.t.Fatal(err)
	}
	ms.gm.suppressSpawns = true
	w.site = ms
	w.crewCount = len(colonyWorkers(ms))
}

// beamFromShip does what the player does without the Global Inventory research:
// board the ship, click the highlighted supply pad and Beam Down through its
// inspector, then frames the bridge until the step leaves from.
func (w *tutorialWorld) beamFromShip(from, blueprint string, qty int) {
	w.t.Helper()
	ship, err := w.wm.EnterShip()
	if err != nil {
		w.t.Fatal(err)
	}
	w.b.step(tutorial.ContextShip, ship, nil)
	if _, ok := w.b.shipHoldHighlight(); !ok {
		w.t.Fatal("the ship's supply pad is not highlighted")
	}
	insp := ship.storageInspector
	insp.Open(w.wm.ShipHoldEntity())
	w.b.step(tutorial.ContextShip, ship, nil)
	if _, ok := w.b.inspectorHighlight("beam"); !ok {
		w.t.Fatal("the inspector's Beam Down button is not highlighted")
	}
	if _, ok := w.b.shipHoldHighlight(); ok {
		w.t.Error("the pad frame drew across the open inspector")
	}
	insp.selectedBP = blueprint
	insp.amountInput.SetText(strconv.Itoa(qty))
	insp.performAction()
	insp.Visible = false
	for i := 0; i < tutorialScanEvery+15 && w.stepID() == from; i++ {
		w.b.step(tutorial.ContextShip, ship, nil)
	}
	if w.stepID() == from {
		w.t.Fatalf("step %s did not complete aboard the ship", from)
	}
}

func (w *tutorialWorld) siteStock() map[string]int {
	return w.site.buildEvalContext().ResourceCounts
}

func (w *tutorialWorld) queueBuild(kind string, x, y int, builds map[string]construction.Buildable) {
	w.t.Helper()
	w.site.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.BuildAction, X: x, Y: y, Z: w.site.camera.Z,
		Data: task_requests.BuildRequest{Type: kind, X: x, Y: y, Z: w.site.camera.Z, Required: builds[kind].BuildTime},
	})
}

// A marquee cycle with a dig cursor sets ui.marquee_committed; with the default
// cursor it does not; and the level's own flags never gain a ui.* key.
func TestTutorialMarqueeFlagAndCleanLevelFlags(t *testing.T) {
	w := newTutorialWorld(t, 5)
	w.enterSite()
	w.siteFrames(2)
	w.site.CursorMode = gui.CursorModeDefault
	w.site.marqueeActive = true
	w.siteFrames(1)
	w.site.marqueeActive = false
	w.siteFrames(1)
	if w.b.flags["ui.marquee_committed"] == true {
		t.Error("a default-cursor drag counted as a dig marquee")
	}
	w.site.CursorMode = gui.CursorModeDig
	w.site.marqueeActive = true
	w.siteFrames(1)
	w.site.marqueeActive = false
	w.siteFrames(1)
	if w.b.flags["ui.marquee_committed"] != true {
		t.Error("dig marquee cycle did not set ui.marquee_committed")
	}
	ev := w.b.evalWithFlags(tutorial.ContextSite, w.site, nil, w.b.at())
	if ev.Flags["ui.marquee_committed"] != true {
		t.Error("eval context missing the ui flag")
	}
	for k := range w.site.level.Flags {
		if strings.HasPrefix(k, "ui.") {
			t.Errorf("level flag %q leaked from the tutorial", k)
		}
	}
}

// A world click under the panel is swallowed; one beside it is not.
func TestTutorialBlocksClickUnderPanel(t *testing.T) {
	w := newTutorialWorld(t, 5)
	w.enterSite()
	w.siteFrames(1)
	r := w.b.panel.rect
	if r.Width == 0 {
		t.Fatal("panel has no rect after a frame")
	}
	if !tutorialBlocksClick(r.X+5, r.Y+5) || !tutorialBlocksClick(r.X+r.Width-1, r.Y+r.Height-1) {
		t.Error("click inside the panel not blocked")
	}
	if tutorialBlocksClick(r.X-1, r.Y+5) || tutorialBlocksClick(r.X+5, r.Y+r.Height) {
		t.Error("click beside the panel blocked")
	}
	// Collapsed: only the title row blocks.
	w.b.panel.collapsed = true
	w.siteFrames(1)
	if tutorialBlocksClick(r.X+5, r.Y+tutPanelBarH+5) {
		t.Error("collapsed panel still blocks its old body")
	}
	activeTutorial = nil
	if tutorialBlocksClick(r.X+5, r.Y+5) {
		t.Error("no tutorial, yet a click was blocked")
	}
}

// Skip takes two clicks, sets Skipped, lifts the lock everywhere and leaves
// the end panel; Close removes the tutorial.
func TestTutorialSkip(t *testing.T) {
	w := newTutorialWorld(t, 5)
	w.d.selectTabByID("starmap")
	w.b.step("dash:starmap", nil, w.d)
	if w.c.TutorialTravelLock("home") == "" {
		t.Fatal("Home should be locked at the start")
	}
	w.b.panel.onSkip()
	if w.c.Tutorial.Skipped || w.b.panel.skipBtn.Text != "Really skip?" {
		t.Fatalf("first click: skipped=%v label=%q", w.c.Tutorial.Skipped, w.b.panel.skipBtn.Text)
	}
	// The arm times out.
	w.b.frame += tutSkipArmFrames + 1
	w.b.step("dash:starmap", nil, w.d)
	if w.b.panel.skipBtn.Text != "Skip tutorial" {
		t.Fatalf("arm did not time out: %q", w.b.panel.skipBtn.Text)
	}
	w.b.panel.onSkip()
	w.b.panel.onSkip()
	if !w.c.Tutorial.Skipped || !w.b.ended || !w.b.panel.ending {
		t.Fatalf("second click did not skip: %+v", w.c.Tutorial)
	}
	if w.c.TutorialTravelLock("home") != "" || w.c.TutorialActive() {
		t.Error("lock did not lift on Skip")
	}
	if w.c.Tutorial.StepID != "choose_planet" {
		t.Errorf("StepID after skip = %q", w.c.Tutorial.StepID)
	}
	w.b.step("dash:starmap", nil, w.d) // the end panel is up and harmless
	w.b.close()
	if activeTutorial != nil {
		t.Error("Close left the tutorial active")
	}
}

// colonyWorkers lists the live colony workers on a level.
func colonyWorkers(ms *MainState) []*ecs.Entity {
	var out []*ecs.Entity
	for _, e := range ms.level.Entities {
		if components.IsColonyWorker(e, ms.colonyName()) {
			out = append(out, e)
		}
	}
	return out
}
