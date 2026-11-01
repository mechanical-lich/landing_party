package game

import (
	"log"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/landing_party/internal/audio"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/objective"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/tutorial"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/message"
)

// activeTutorial is the running tutorial, nil when there is none. A package
// variable (like workerai.StorageProviderFor) so WorldManager stays out of it;
// startTutorial always assigns it, so a previous game cannot leak into a new one.
var activeTutorial *tutorialBridge

// tutorialScanEvery is how many real frames pass between entity scans of the
// level in orbit. UI flags are read every frame; only the scan is throttled, as
// evaluateQuests already does.
const tutorialScanEvery = 30

// Baseline keys saved in TutorialProgress.Baselines.
const (
	baseSiteMetal   = "site.metal_ore"
	baseSiteCrystal = "site.crystal"
	baseHoldMetal   = "hold.metal_ore"
	baseHoldCrystal = "hold.crystal"
	baseSiteLockers = "site.storage_locker"
)

// tutorialBridge is the game side of the tutorial: it reads the live state
// each frame, turns it into ui.* flags and a snapshot for the director, owns
// the prompt panel and highlights, and copies progress back onto the campaign.
type tutorialBridge struct {
	c   *campaign.Campaign
	wm  *WorldManager
	def tutorial.Definition
	dir *tutorial.Director

	panel *tutorialPanel

	frame int
	// scan is the last entity scan of the level in orbit, refreshed every
	// tutorialScanEvery frames and at every step change.
	scan     objective.EvalContext
	holdMeta int
	holdCrys int
	carrying map[string]bool
	// siteMeta and siteCrys are the site's metal and crystal in storage plus
	// what mining has dropped on the ground and not yet hauled: mined output
	// counts as soon as it exists, since hauling one unit per trip can take
	// minutes.
	siteMeta int
	siteCrys int
	scanned  bool

	// Latches, reset at each step change.
	flags       map[string]any
	camBaseSet  bool
	camX, camY  int
	camTileW    int
	camZ        int
	zLeft       bool
	marqueeSeen bool

	// lockerCost is the storage locker's metal cost, for ui.cargo_packed.
	lockerCost int

	// ended is set once the end panel is up (finished or skipped), so the
	// director is left alone and only the panel's Close remains.
	ended bool
	// lastCtx is the context of the most recent frame, for drawing.
	lastCtx string
	// needBaselines asks the first frame to capture baselines (new start).
	needBaselines bool
	// lastSite and lastDash are the hosts of the most recent frame, used by the
	// highlights drawn in the same frame. Never used across frames: the
	// dashboard is rebuilt every time it opens.
	lastSite *MainState
	lastDash *DashboardState
}

// startTutorial installs (or clears) the running tutorial for a campaign. Called
// for new and loaded games alike.
func startTutorial(c *campaign.Campaign, wm *WorldManager) {
	activeTutorial = nil
	if c == nil || wm == nil || !c.Tutorial.Active() {
		return
	}
	def, err := tutorial.Load(tutorial.DefinitionPath)
	if err != nil {
		log.Printf("tutorial: %v", err)
		return
	}
	b := &tutorialBridge{
		c: c, wm: wm, def: def,
		dir:        tutorial.New(def, c.Tutorial.StepID),
		flags:      map[string]any{},
		lockerCost: tutorialLockerMetalCost(),
	}
	if len(c.Tutorial.Baselines) == 0 {
		// A fresh start, or a save from before this step had baselines: capture
		// them on the first frame, so a pre-placed locker is not "added".
		c.Tutorial.Baselines = map[string]int{}
		b.needBaselines = true
	}
	b.panel = newTutorialPanel(b)
	activeTutorial = b
	if s := b.dir.Current(); s != nil {
		message.PostMessage("Tutorial", b.expand(s.Title))
	}
}

// tutorialLockerMetalCost reads the storage locker's metal cost from the
// build registry, falling back to 2 when the registry is empty (tests).
func tutorialLockerMetalCost() int {
	if m := construction.GetBuildable("storage_locker").Cost["metal_ore"]; m > 0 {
		return m
	}
	return 2
}

// ---- hooks called by the states ----

// tutorialFrameSite runs the tutorial for one frame of a level: the ship or a
// planet surface.
func tutorialFrameSite(s *MainState) {
	b := activeTutorial
	if b == nil || s == nil || s.wm != b.wm {
		return
	}
	ctx := tutorial.ContextSite
	if s == b.wm.shipLevel {
		ctx = tutorial.ContextShip
	}
	b.step(ctx, s, nil)
}

// tutorialFrameDash runs the tutorial for one frame of the dashboard.
func tutorialFrameDash(d *DashboardState) {
	b := activeTutorial
	if b == nil || d == nil || d.wm != b.wm {
		return
	}
	ctx := "dash:"
	if d.active >= 0 && d.active < len(d.tabs) {
		ctx += d.tabs[d.active].id
	}
	b.step(ctx, nil, d)
}

// tutorialDraw draws the prompt panel and highlights above the host state.
func tutorialDraw(screen *ebiten.Image) {
	if b := activeTutorial; b != nil {
		b.drawHighlight(screen)
		b.panel.Draw(screen)
	}
}

// tutorialBlocksClick reports whether a world click at (x, y) lands on the
// prompt panel, so the host state must not act on it.
func tutorialBlocksClick(x, y int) bool {
	b := activeTutorial
	return b != nil && b.panel.Contains(x, y)
}

// ---- per frame ----

// step is one frame: read the state, feed the director, apply what changed,
// then let the panel handle its buttons.
func (b *tutorialBridge) step(ctx string, s *MainState, d *DashboardState) {
	b.frame++
	b.lastCtx = ctx
	newDash := d != nil && d != b.lastDash
	b.lastSite, b.lastDash = s, d
	at := b.at()
	if newDash {
		b.pointAtTarget()
	}
	if !b.ended {
		if !b.scanned || b.frame%tutorialScanEvery == 0 {
			b.rescan()
		}
		b.placePadIfBare(at)
		if b.needBaselines {
			b.needBaselines = false
			b.captureBaselines()
		}
		b.updateLatches(ctx, s, d)
		snap := tutorial.Snapshot{Context: ctx, At: at, Eval: b.evalWithFlags(ctx, s, d, at)}
		b.dir.Observe(snap)
		// Progress (and the unlock) first, so the new step's Star Map pointing
		// sees the destination already unlocked.
		b.syncProgress()
		if b.dir.StepChanged() {
			b.onStepChanged(s)
		}
	}
	b.panel.Update(ctx, s, d)
}

// at names the location in orbit for the director: "tutorial", "next" or "".
func (b *tutorialBridge) at() string {
	switch b.c.CurrentLocationID {
	case campaign.TutorialLocationID:
		return tutorial.AtTutorial
	case b.c.Tutorial.NextLocationID:
		return tutorial.AtNext
	}
	return ""
}

// rescan refreshes the entity scan of the level in orbit and the hold counts.
func (b *tutorialBridge) rescan() {
	b.scanned = true
	b.carrying = map[string]bool{}
	b.siteMeta, b.siteCrys = 0, 0
	if cur := b.wm.Current(); cur != nil && cur.level != nil {
		b.scan = cur.buildEvalContext()
		b.siteMeta = b.scan.ResourceCounts["metal_ore"]
		b.siteCrys = b.scan.ResourceCounts["crystal"]
		for _, e := range cur.level.Entities {
			if e == nil {
				continue
			}
			// Loose mined stacks on the ground.
			if e.HasComponent(components.Material) && e.HasComponent(rlcomponents.Position) {
				qty := e.GetComponent(components.Material).(*components.MaterialComponent).Quantity
				switch e.Blueprint {
				case "metal_ore":
					b.siteMeta += qty
				case "crystal":
					b.siteCrys += qty
				}
			}
			if !components.IsColonyWorker(e, cur.colonyName()) || !e.HasComponent(rlcomponents.Inventory) {
				continue
			}
			for _, item := range e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag {
				if item != nil {
					b.carrying[item.Blueprint] = true
				}
			}
		}
	} else {
		b.scan = objective.EvalContext{}
	}
	b.holdMeta = b.holdCount("metal_ore")
	b.holdCrys = b.holdCount("crystal")
}

func (b *tutorialBridge) holdCount(bp string) int {
	if b.wm.ShipLevel() == nil {
		return 0
	}
	return storage.CountResource(storage.ShipProvider{Level: b.wm.ShipLevel()}, []string{b.wm.shipSettlementName()}, bp)
}

// updateLatches records the one-shot UI facts a frame can show: camera moves,
// a pause, a marquee cycle, an inspector. They are flags for the
// rest of the step and are cleared when the step changes.
func (b *tutorialBridge) updateLatches(ctx string, s *MainState, d *DashboardState) {
	if s == nil || s != b.wm.Current() {
		return
	}
	cam := s.camera
	if !b.camBaseSet {
		b.captureCamera(s)
	}
	if cam.X != b.camX || cam.Y != b.camY {
		b.flags["ui.camera_panned"] = true
	}
	if cam.TileW != b.camTileW {
		b.flags["ui.camera_zoomed"] = true
	}
	if cam.Z != b.camZ {
		b.zLeft = true
		b.flags["ui.z_changed"] = true
	} else if b.zLeft {
		b.flags["ui.z_returned"] = true
	}
	if s.clockStopped() {
		b.flags["ui.paused"] = true
	}
	if s.marqueeActive && (s.CursorMode == gui.CursorModeDig || s.CursorMode == gui.CursorModeMine) {
		b.marqueeSeen = true
	} else if b.marqueeSeen && !s.marqueeActive {
		b.flags["ui.marquee_committed"] = true
	}
	if s.storageInspector != nil && s.storageInspector.Visible {
		b.flags["ui.inspector_opened"] = true
	}
}

// evalWithFlags builds the director's eval context: the cached scan with a
// copy of the level flags plus every ui.* flag. Level flags are never written
// to, since they are saved with the level.
func (b *tutorialBridge) evalWithFlags(ctx string, s *MainState, d *DashboardState, at string) objective.EvalContext {
	ev := b.scan
	flags := make(map[string]any, len(ev.Flags)+len(b.flags)+16)
	for k, v := range ev.Flags {
		flags[k] = v
	}
	for k, v := range b.flags {
		flags[k] = v
	}
	base := b.c.Tutorial.Baselines

	flags["ui.at_tutorial"] = at == tutorial.AtTutorial
	flags["ui.at_next"] = at == tutorial.AtNext
	flags["ui.at_site"] = ctx == tutorial.ContextSite
	flags["ui.crew_aboard"] = len(b.wm.PlanetColonists()) == 0 && len(b.wm.ShipColonists()) > 0
	flags["ui.cargo_mined"] = b.siteMeta > base[baseSiteMetal] && b.siteCrys >= 3
	flags["ui.cargo_packed"] = b.holdMeta > base[baseHoldMetal] && b.holdMeta >= b.lockerCost && b.holdCrys > base[baseHoldCrystal]
	flags["ui.cargo_landed"] = b.siteMeta > base[baseSiteMetal]
	flags["ui.locker_added"] = ev.StructuresBuilt["storage_locker"] > base[baseSiteLockers]
	for bp := range b.carrying {
		flags["ui.carrying:"+bp] = true
	}
	if cur := b.wm.Current(); cur != nil && cur.guiManager != nil {
		open := cur.guiManager.ColonistModalVisible()
		flags["ui.colonist_panel_open"] = open
		flags["ui.colonist_panel_closed"] = !open
	}
	if cur := b.wm.Current(); cur != nil && cur.MainSettlement != nil {
		for _, t := range cur.MainSettlement.Tasks.GetTasks() {
			switch t.Action {
			case task_requests.DigAction:
				flags["ui.dig_queued"] = true
			case task_requests.MineAction:
				flags["ui.mine_queued"] = true
			}
		}
	}
	if step := b.dir.Current(); step != nil && step.Target != "" && d != nil {
		if p := d.starMapPanel(); p != nil {
			if sel := p.selectedLocation(); sel != nil && sel.ID == b.targetID(step.Target) {
				flags["ui.dest_selected"] = true
			}
		}
	}
	ev.Flags = flags
	return ev
}

// targetID resolves a step's Target ("tutorial" or "next") to a location ID.
func (b *tutorialBridge) targetID(target string) string {
	if target == tutorial.AtNext {
		return b.c.Tutorial.NextLocationID
	}
	return campaign.TutorialLocationID
}

// captureCamera records the camera the "moved" latches compare against.
func (b *tutorialBridge) captureCamera(s *MainState) {
	b.camBaseSet = true
	b.camX, b.camY, b.camTileW, b.camZ = s.camera.X, s.camera.Y, s.camera.TileW, s.camera.Z
}

// onStepChanged resets the latches, captures fresh baselines and announces the
// new prompt. It runs once per advance, after any catch-up. s is the level of
// this frame, if any, so the camera baseline is this frame's view rather than
// the next frame's.
func (b *tutorialBridge) onStepChanged(s *MainState) {
	b.rescan()
	b.flags = map[string]any{}
	b.camBaseSet = false
	if s != nil && s == b.wm.Current() {
		b.captureCamera(s)
	}
	b.zLeft = false
	b.marqueeSeen = false
	b.captureBaselines()
	b.panel.onStepChanged()
	b.pointAtTarget()
	audio.PlayGlobal("notify")
	if s := b.dir.Current(); s != nil {
		message.PostMessage("Tutorial", b.expand(s.Title))
	}
}

// captureBaselines records the counts the "above baseline" flags compare
// against, into the saved progress.
func (b *tutorialBridge) captureBaselines() {
	base := b.c.Tutorial.Baselines
	base[baseSiteMetal] = b.siteMeta
	base[baseSiteCrystal] = b.siteCrys
	base[baseHoldMetal] = b.holdMeta
	base[baseHoldCrystal] = b.holdCrys
	base[baseSiteLockers] = b.scan.StructuresBuilt["storage_locker"]
}

// syncProgress copies the director's position onto the campaign and applies
// the unlock and the finish.
func (b *tutorialBridge) syncProgress() {
	t := b.c.Tutorial
	t.StepID = b.dir.StepID()
	if b.dir.NextUnlocked() {
		t.Unlock(t.NextLocationID)
	}
	if b.dir.Done() && !t.Done {
		t.Done = true
		b.ended = true
		audio.PlayGlobal("notify")
		message.PostMessage("Tutorial", "First landing complete. The galaxy is yours.")
		b.panel.showEnd(false)
	}
}

// skip is the panel's confirmed Skip: the lock lifts at once and the world is
// left exactly as it is.
func (b *tutorialBridge) skip() {
	if b.ended {
		return
	}
	b.dir.Skip()
	b.c.Tutorial.StepID = b.dir.StepID()
	b.c.Tutorial.Skipped = true
	b.ended = true
	message.PostMessage("Tutorial", "Tutorial skipped.")
	b.panel.showEnd(true)
}

// close removes the tutorial after the end panel.
func (b *tutorialBridge) close() {
	if activeTutorial == b {
		activeTutorial = nil
	}
}

// placePadIfBare gives the second planet a supply pad on first arrival when it
// has no colony container at all, so Beam Resources Down can work there.
func (b *tutorialBridge) placePadIfBare(at string) {
	t := b.c.Tutorial
	if at != tutorial.AtNext || t.PadPlaced {
		return
	}
	cur := b.wm.Current()
	if cur == nil || cur.level == nil {
		return
	}
	if b.wm.SiteHoldEntity() != nil {
		t.PadPlaced = true // nothing to do: the site has storage of its own
		return
	}
	lx, ly, lz := b.wm.landingZone()
	x, y, z := freeLandingTile(cur.level, lx+2, ly+2, lz)
	pad, err := factory.Create("teleporter", x, y, z)
	if err != nil {
		log.Printf("tutorial: pad: %v", err)
		return
	}
	pad.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = b.wm.currentColonyName()
	cur.level.AddEntity(pad)
	t.PadPlaced = true
	message.PostMessage("Tutorial", "A supply pad was set down for you: this site had no storage.")
	b.rescan()
}

// expand fills the step-text placeholders: {next} is the second destination's
// name and {tutorial} the Training Ground's.
func (b *tutorialBridge) expand(text string) string {
	next := b.c.Tutorial.NextLocationID
	if loc := b.c.Locations[next]; loc != nil && loc.Name != "" {
		next = loc.Name
	}
	tut := "Training Ground"
	if loc := b.c.Locations[campaign.TutorialLocationID]; loc != nil && loc.Name != "" {
		tut = loc.Name
	}
	return strings.NewReplacer("{next}", next, "{tutorial}", tut).Replace(text)
}

// nudgeText says where to go when the current step is live somewhere else.
func (b *tutorialBridge) nudgeText(ctx, at string) string {
	step := b.dir.Current()
	if step == nil {
		return ""
	}
	if step.At != "" && step.At != at {
		if step.At == tutorial.AtNext {
			if loc := b.c.Locations[b.c.Tutorial.NextLocationID]; loc != nil {
				return "Travel to " + loc.Name + " to continue."
			}
			return "Travel to the next destination to continue."
		}
		return "Return to the Training Ground to continue."
	}
	switch {
	case step.Context == tutorial.ContextSite:
		if ctx == tutorial.ContextShip {
			return "Open the Star Map (O), then press To Site to continue."
		}
		return "Press To Site to continue."
	case step.Context == tutorial.ContextShip:
		if ctx == tutorial.ContextSite {
			return "Open the Star Map (O), then press To Ship to continue."
		}
		return "Press To Ship (top right) to board the ship and continue."
	case strings.HasPrefix(step.Context, "dash:"):
		tab := strings.TrimPrefix(step.Context, "dash:")
		if ctx == tutorial.ContextSite || ctx == tutorial.ContextShip {
			if tab == "starmap" {
				return "Open the Star Map (O) to continue."
			}
			return "Open the Star Map (O), then the " + dashTabLabel(tab) + " tab."
		}
		return "Open the " + dashTabLabel(tab) + " tab to continue."
	}
	return ""
}

func dashTabLabel(id string) string {
	switch id {
	case "starmap":
		return "Star Map"
	case "inventory":
		return "Global Inventory"
	case "crew":
		return "Crew"
	case "quests":
		return "Quests"
	case "encyclopedia":
		return "Encyclopedia"
	}
	return id
}

// pointAtTarget makes the Star Map show the step's destination: the row is
// selected (so its details and the enabled Travel button are up) and the canvas
// is centred on the planet. Runs when a step with a target starts and whenever
// a fresh dashboard opens, since each one starts with the current location
// selected and the view fitted to the cluster.
func (b *tutorialBridge) pointAtTarget() {
	loc := b.starTarget()
	d := b.lastDash
	if loc == nil || d == nil {
		return
	}
	p := d.starMapPanel()
	if p == nil {
		return
	}
	p.selectByID(loc.ID)
	p.refreshTravelLock()
	p.centerOn(loc)
}

// openManual shows the Field Manual article for topic in the Encyclopedia: on
// the dashboard it switches tab; from a level it opens the dashboard first.
// openDashboard parks the level and Esc returns, so no new navigation is needed.
func (b *tutorialBridge) openManual(topic string) {
	if topic == "" {
		return
	}
	show := func(d *DashboardState) {
		if d == nil {
			return
		}
		d.selectTabByID("encyclopedia")
		if e := d.encyclopedia(); e != nil {
			e.selectTopic(topic)
		}
	}
	if d := b.lastDash; d != nil && strings.HasPrefix(b.lastCtx, "dash:") {
		show(d)
		return
	}
	if s := b.lastSite; s != nil && s.wm != nil {
		s.openDashboard()
		if d, ok := s.next.(*DashboardState); ok {
			show(d)
		}
	}
}

// encyclopedia returns the dashboard's Encyclopedia tab, or nil.
func (d *DashboardState) encyclopedia() *encyclopediaPanel {
	for _, t := range d.tabs {
		if p, ok := t.panel.(*encyclopediaPanel); ok {
			return p
		}
	}
	return nil
}

// starMapPanel returns the dashboard's Star Map tab, or nil.
func (d *DashboardState) starMapPanel() *starMapPanel {
	for _, t := range d.tabs {
		if p, ok := t.panel.(*starMapPanel); ok {
			return p
		}
	}
	return nil
}
