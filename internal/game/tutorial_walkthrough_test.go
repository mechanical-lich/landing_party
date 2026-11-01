package game

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/tutorial"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// containerWith finds a colony container on the site holding at least qty of bp.
func containerWith(ms *MainState, bp string, qty int) *ecs.Entity {
	for _, ents := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range ents {
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			if sc.OwnedBy == ms.colonyName() && sc.CountResource(bp) >= qty {
				return e
			}
		}
	}
	return nil
}

// removeHostiles clears a site of anything that would attack the crew, so the
// walkthrough's short stay on the start system is deterministic.
func removeHostiles(ms *MainState) int {
	var gone []*ecs.Entity
	for _, e := range ms.level.Entities {
		if components.IsAttackTarget(e) && !components.IsColonyWorker(e, ms.colonyName()) {
			gone = append(gone, e)
		}
	}
	for _, e := range gone {
		ms.level.RemoveEntity(e)
	}
	return len(gone)
}

// The whole tutorial, driven through the real game methods and the worker AI,
// from StartNewExpedition to the handoff: the director reaches the end, both
// jumps charge fuel, the cargo goes up, survives the jump and lands.
func TestTutorialWalkthrough(t *testing.T) {
	builds, recipes := useRealCosts(t)
	w := newTutorialWorld(t, 11)
	c, wm, b := w.c, w.wm, w.b
	startID := campaign.StartLocationID
	if w.stepID() != "choose_planet" {
		t.Fatalf("starts at %s", w.stepID())
	}
	fuel := holdCount(wm, "fuel")
	if fuel != campaign.GenerationConfig().StartFuel || holdCount(wm, "metal_ore") != tutorialHoldMetal {
		t.Fatalf("hold: fuel %d metal %d", fuel, holdCount(wm, "metal_ore"))
	}

	// 1 choose_planet: the first dashboard frame points the Star Map at the
	// Training Ground: row selected, Travel enabled, marker on that planet and
	// Travel Here highlighted with it. The step waits for the jump itself, so
	// the auto-selection cannot pass it before the prompt is read.
	p := w.d.starMapPanel()
	w.dashStays("starmap", "choose_planet")
	if sel := p.selectedLocation(); sel == nil || sel.ID != campaign.TutorialLocationID {
		t.Fatalf("step 1 selected %v, want the Training Ground", sel)
	}
	if tgt := b.starTarget(); tgt == nil || tgt.ID != campaign.TutorialLocationID {
		t.Fatalf("step 1 marker on %v, want the Training Ground", tgt)
	}
	assertTravelAndRing(t, b, p)
	if !p.travelBtn.IsEnabled() || p.travelBtn.Text != "Travel Here" {
		t.Fatalf("Travel button at step 1: enabled=%v text=%q", p.travelBtn.IsEnabled(), p.travelBtn.Text)
	}
	if p.camX != w.c.Locations[campaign.TutorialLocationID].X {
		t.Errorf("canvas not centred on the Training Ground: camX %v", p.camX)
	}
	// A locked row says so on the button and the status line.
	p.selectByID(startID)
	p.selectByID("home")
	p.refreshTravelLock()
	if p.travelBtn.IsEnabled() || p.travelBtn.Text != "Travel locked" || !strings.Contains(p.status, "Training Ground") {
		t.Fatalf("locked Home: enabled=%v text=%q status=%q", p.travelBtn.IsEnabled(), p.travelBtn.Text, p.status)
	}
	w.dashStays("starmap", "choose_planet")
	cost := c.FuelCost(campaign.TutorialLocationID)
	p.selectByID(campaign.TutorialLocationID)
	p.refreshTravelLock()
	if !p.travelBtn.IsEnabled() {
		t.Fatal("Travel to the Training Ground is locked")
	}
	p.travel()
	if c.CurrentLocationID != campaign.TutorialLocationID {
		t.Fatalf("travel failed: %s (%s)", c.CurrentLocationID, p.status)
	}
	if got := holdCount(wm, "fuel"); got != fuel-cost {
		t.Errorf("fuel after the first jump = %d, want %d", got, fuel-cost)
	}
	fuel -= cost
	w.dash("starmap", "choose_planet")

	// 2 beam_party
	crew := wm.ShipColonists()
	if err := wm.BeamDown(crew[0]); err != nil {
		t.Fatal(err)
	}
	w.dashStays("starmap", "beam_party")
	if err := wm.BeamDown(crew[1]); err != nil {
		t.Fatal(err)
	}
	w.dash("starmap", "beam_party")

	// 3 to_site
	w.enterSite()
	site := w.site
	w.siteFrames(1)
	if w.stepID() != "camera" {
		t.Fatalf("after To Site: %s", w.stepID())
	}

	// 4 camera, 5 z_levels, 6 pause, 7 select
	site.camera.X += 4
	w.siteFrames(1)
	if w.stepID() != "z_levels" {
		t.Fatalf("after a pan: %s", w.stepID())
	}
	site.camera.Z--
	w.siteFrames(1)
	if w.stepID() != "z_levels" {
		t.Fatalf("z step passed before returning: %s", w.stepID())
	}
	site.camera.Z++
	w.siteFrames(1)
	if w.stepID() != "pause" {
		t.Fatalf("after Q then E: %s", w.stepID())
	}
	site.Paused = true
	w.siteFrames(1)
	site.Paused = false
	if w.stepID() != "select" {
		t.Fatalf("after a pause: %s", w.stepID())
	}
	workers := colonyWorkers(site)
	if len(workers) != 2 {
		t.Fatalf("%d colonists on the surface", len(workers))
	}
	// Selecting is not enough; the step waits for the panel itself.
	site.selectedEntity = workers[0]
	w.siteFrames(1)
	site.selectedEntity = nil
	if w.stepID() != "select" {
		t.Fatalf("select passed with no panel open: %s", w.stepID())
	}
	site.guiManager.ShowColonistModal(workers[0], nil)
	w.siteFrames(2)
	if w.stepID() != "close_colonist" {
		t.Fatalf("after opening the colonist panel: %s", w.stepID())
	}

	// 8 close_colonist: the food prompt and its box wait until the panel is shut.
	site.guiManager.CloseModal("colonistModal")
	w.siteFrames(1)
	if w.stepID() != "haul_food" {
		t.Fatalf("after closing the colonist panel: %s", w.stepID())
	}

	// 9 haul_food: click three rations.
	queued := 0
	for _, e := range site.level.Entities {
		if e.Blueprint != "ration_pack" || queued >= 3 {
			continue
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		site.MainSettlement.Tasks.AddTask(&task.Task{
			Action: task_requests.RetrieveAction, Data: task_requests.RetrieveRequest{Item: e},
			X: pc.GetX(), Y: pc.GetY(), Z: pc.GetZ(), Escalated: true,
		})
		queued++
	}
	t.Logf("haul_food took %d ticks", w.tickUntilStep("haul_food", 6000))

	// 10 beam_metal, from the ship: the Global Inventory needs research nobody
	// has yet.
	if !config.Global().UnlockAllResearch && c.HasTech(techGlobalInvCurrent) {
		t.Fatal("a new expedition already has the Global Inventory")
	}
	w.beamFromShip("beam_metal", "metal_ore", tutorialHoldMetal)
	if holdCount(wm, "metal_ore") != 0 {
		t.Fatalf("hold still has %d metal", holdCount(wm, "metal_ore"))
	}
	w.enterSite()

	// 11 build_locker
	sx, sy := site.MainSettlement.CenterX, site.MainSettlement.CenterY
	w.queueBuild("storage_locker", sx, sy-2, builds)
	t.Logf("build_locker took %d ticks", w.tickUntilStep("build_locker", 6000))

	// 12 dig: a marquee cycle.
	site.CursorMode = gui.CursorModeDig
	site.marqueeActive = true
	w.siteFrames(1)
	site.marqueeActive = false
	w.siteFrames(1)
	if w.stepID() != "mine_cargo" {
		t.Fatalf("after a dig marquee: %s", w.stepID())
	}

	// 13 mine_cargo: ore and crystal from the west face.
	// Only the two face tiles reachable from open ground: interior orders put
	// both workers into a one-tile corridor, where they can block each other for
	// thousands of ticks (existing worker behaviour, not the tutorial's).
	for _, p := range [][2]int{{sx + 5, sy}, {sx + 5, sy + 1}} {
		if !site.addMineTask(p[0], p[1]) {
			t.Fatalf("mine order refused at %v", p)
		}
	}
	t.Logf("mine_cargo took %d ticks", w.tickUntilStep("mine_cargo", 20000))
	// Let the miners pull enough for the rest of the run (bed, bench, lab and
	// research cost 17 metal against the 10 beamed down), then cancel the rest of
	// the mining (the player's Cancel Task order): a deposit holds up to 100
	// units, each a hauling trip, and closest-first task picking would keep both
	// workers on the pile for the rest of the run.
	ticks := 0
	for ; ticks < 20000 && (b.siteMeta < tutorialHoldMetal+15 || b.siteCrys < 5); ticks++ {
		w.tickUntilStepOnce()
	}
	t.Logf("mined enough after %d more ticks: site metal %d crystal %d", ticks, b.siteMeta, b.siteCrys)
	for _, tk := range site.MainSettlement.Tasks.GetTasks() {
		if tk.Action == task_requests.MineAction {
			tk.Complete()
			site.MainSettlement.Tasks.RemoveTask(tk)
		}
	}
	for _, e := range colonyWorkers(site) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && wc.CurrentTask.Action == task_requests.MineAction {
			wc.CurrentTask = nil
		}
	}

	// 14 storage: open a locker's inspector.
	locker := containerWith(site, "", 0)
	site.storageInspector.Open(locker)
	w.siteFrames(1)
	site.storageInspector.Visible = false
	w.siteFrames(1)
	if w.stepID() != "needs" {
		t.Fatalf("after opening the inspector: %s", w.stepID())
	}

	// 15 needs, 16 workbench
	w.queueBuild("bed", sx-2, sy-2, builds)
	t.Logf("needs took %d ticks", w.tickUntilStep("needs", 20000))
	w.queueBuild("basic_workbench", sx+2, sy-2, builds)
	t.Logf("workbench took %d ticks", w.tickUntilStep("workbench", 20000))

	// 17 craft: biomass down, then a sleeping bag.
	if err := wm.BeamResourceDown("biomass", 5); err != nil {
		t.Fatal(err)
	}
	var bench *ecs.Entity
	for _, ents := range [][]*ecs.Entity{site.level.Entities, site.level.StaticEntities} {
		for _, e := range ents {
			if e != nil && e.Blueprint == "basic_workbench" {
				bench = e
			}
		}
	}
	if bench == nil {
		t.Fatal("no workbench on site")
	}
	bpc := bench.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	site.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.CraftAction, X: bpc.GetX(), Y: bpc.GetY(), Z: bpc.GetZ(),
		Data: task_requests.CraftRequest{RecipeID: "sleeping_bag", WorkbenchX: bpc.GetX(), WorkbenchY: bpc.GetY(), WorkbenchZ: bpc.GetZ(), Required: recipes["sleeping_bag"].BuildTime},
	})
	t.Logf("craft took %d ticks", w.tickUntilStep("craft", 20000))

	// 18 research: lab, then Basic Metallurgy.
	if w.siteStock()["crystal"] < builds["research_lab"].Cost["crystal"] {
		if err := wm.BeamResourceDown("crystal", 5); err != nil {
			t.Fatal(err)
		}
	}
	w.queueBuild("research_lab", sx, sy+3, builds)
	ticks = 0
	for ; ticks < 20000 && countStatic(site, "research_lab") == 0; ticks++ {
		w.tickUntilStepOnce()
	}
	var lab *ecs.Entity
	for _, ents := range [][]*ecs.Entity{site.level.Entities, site.level.StaticEntities} {
		for _, e := range ents {
			if e != nil && e.Blueprint == "research_lab" {
				lab = e
			}
		}
	}
	if lab == nil {
		t.Fatalf("lab not built in %d ticks (stock %v) %s", ticks, w.siteStock(), w.crewReport())
	}
	lpc := lab.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	site.MainSettlement.Tasks.AddTask(&task.Task{
		Action: task_requests.ResearchAction, X: lpc.GetX(), Y: lpc.GetY(), Z: lpc.GetZ(),
		Data: &task_requests.ResearchRequest{TechKey: "basic_metallurgy", Building: "research_lab", Required: 100},
	})
	t.Logf("research took %d ticks", w.tickUntilStep("research", 20000))
	if !c.HasTech("basic_metallurgy") {
		t.Error("tech not recorded on the campaign")
	}

	// 19 pack_cargo: metal and crystal up. The builds and research may have
	// used the stored metal; wait for the haulers to bring more mined ore in.
	ticks = 0
	for ; ticks < 20000 && (containerWith(site, "metal_ore", b.lockerCost) == nil || containerWith(site, "crystal", 1) == nil); ticks++ {
		w.tickUntilStepOnce()
	}
	t.Logf("cargo in storage after %d more ticks", ticks)
	src := containerWith(site, "metal_ore", b.lockerCost)
	if src == nil {
		t.Fatalf("no container with %d metal: %v %s", b.lockerCost, w.siteStock(), w.crewReport())
	}
	if err := wm.BeamResourceUpFrom(src, "metal_ore", b.lockerCost); err != nil {
		t.Fatal(err)
	}
	w.siteFrames(tutorialScanEvery + 2)
	if w.stepID() != "pack_cargo" {
		t.Fatalf("pack_cargo passed without crystal: %s", w.stepID())
	}
	cs := containerWith(site, "crystal", 1)
	if err := wm.BeamResourceUpFrom(cs, "crystal", 1); err != nil {
		t.Fatal(err)
	}
	w.siteFrames(tutorialScanEvery + 2)
	if w.stepID() != "crew_up" {
		t.Fatalf("after packing: %s (hold metal %d crystal %d)", w.stepID(), holdCount(wm, "metal_ore"), holdCount(wm, "crystal"))
	}
	packedMetal, packedCrystal := holdCount(wm, "metal_ore"), holdCount(wm, "crystal")

	// 20 crew_up
	for _, e := range wm.PlanetColonists() {
		if err := wm.BeamUp(e); err != nil {
			t.Fatal(err)
		}
	}
	w.dash("starmap", "crew_up")
	if w.stepID() != "next_jump" || !c.Tutorial.IsUnlocked(startID) {
		t.Fatalf("after crew up: %s unlocked=%v", w.stepID(), c.Tutorial.Unlocked)
	}
	// Step 21 points at the one unlocked destination.
	if sel := p.selectedLocation(); sel == nil || sel.ID != startID {
		t.Fatalf("step 21 selected %v, want %s", sel, startID)
	}
	if tgt := b.starTarget(); tgt == nil || tgt.ID != startID {
		t.Fatalf("step 21 marker on %v, want %s", tgt, startID)
	}
	assertTravelAndRing(t, b, p)
	if !p.travelBtn.IsEnabled() {
		t.Fatalf("Travel to %s disabled at step 21: %s", startID, p.travelLock)
	}
	if !strings.Contains(b.expand(b.dir.Current().Text), c.Locations[startID].Name) {
		t.Errorf("step 21 text does not name the destination: %q", b.expand(b.dir.Current().Text))
	}
	if c.TutorialTravelLock("home") == "" {
		t.Error("Home unlocked too early")
	}

	// 21 next_jump
	cost = c.FuelCost(startID)
	p.selectByID(startID)
	p.refreshTravelLock()
	if !p.travelBtn.IsEnabled() {
		t.Fatalf("Travel to the start system locked: %s", p.travelLock)
	}
	p.travel()
	if c.CurrentLocationID != startID {
		t.Fatalf("second jump failed: %s", p.status)
	}
	if got := holdCount(wm, "fuel"); got != fuel-cost {
		t.Errorf("fuel after the second jump = %d, want %d", got, fuel-cost)
	}
	if holdCount(wm, "metal_ore") != packedMetal || holdCount(wm, "crystal") != packedCrystal {
		t.Error("cargo did not survive the jump")
	}
	w.dash("starmap", "next_jump")

	// 22 land_again
	crew = wm.ShipColonists()
	if err := wm.BeamDown(crew[0]); err != nil {
		t.Fatal(err)
	}
	if err := wm.BeamDown(crew[1]); err != nil {
		t.Fatal(err)
	}
	w.dash("starmap", "land_again")

	// 23 land_cargo. The generated start's own containers can sit anywhere
	// (its plaza is a random roll and can land on the map edge, with the
	// scripted locker off-map), so make the site bare: the bridge then sets its
	// pad down beside the landing zone, where the beamed metal is reachable.
	next := wm.Current()
	for _, ents := range [][]*ecs.Entity{next.level.Entities, next.level.StaticEntities} {
		for _, e := range append([]*ecs.Entity(nil), ents...) {
			if e != nil && e.HasComponent(components.Storage) &&
				e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy == next.colonyName() {
				next.level.RemoveEntity(e)
			}
		}
	}
	w.c.Tutorial.PadPlaced = false
	b.step("dash:starmap", nil, w.d)
	if !w.c.Tutorial.PadPlaced || wm.SiteHoldEntity() == nil {
		t.Fatal("bare second planet did not get a pad")
	}
	w.beamFromShip("land_cargo", "metal_ore", b.lockerCost)
	if w.stepID() != "first_build" {
		t.Fatalf("after landing cargo: %s", w.stepID())
	}

	// 24 first_build
	w.enterSite()
	site = w.site
	t.Logf("removed %d hostiles from the start system", removeHostiles(site))
	lockersBefore := countStatic(site, "storage_locker")
	w.siteFrames(2)
	if w.stepID() != "first_build" {
		t.Fatalf("a pre-existing locker completed first_build: %s", w.stepID())
	}
	// Build next to a colonist, on ground they can certainly stand on.
	cpc := colonyWorkers(site)[0].GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	bx, by, _ := freeLandingTile(site.level, cpc.GetX(), cpc.GetY()+1, cpc.GetZ())
	w.queueBuild("storage_locker", bx, by, builds)
	t.Logf("first_build took %d ticks", w.tickUntilStep("first_build", 12000))
	if countStatic(site, "storage_locker") != lockersBefore+1 {
		t.Errorf("%d lockers, want %d", countStatic(site, "storage_locker"), lockersBefore+1)
	}
	if !c.Tutorial.Done || !b.dir.Done() || c.TutorialActive() {
		t.Fatalf("not finished: %+v", c.Tutorial)
	}
	if c.TutorialTravelLock("home") != "" {
		t.Error("lock did not lift at the handoff")
	}
	if !b.panel.ending || b.panel.endTitle != "The galaxy is yours" {
		t.Errorf("end panel: ending=%v title=%q", b.panel.ending, b.panel.endTitle)
	}
	b.close()
	if activeTutorial != nil {
		t.Error("Close left the tutorial active")
	}
}

// tickUntilStepOnce is one world tick plus one bridge frame.
func (w *tutorialWorld) tickUntilStepOnce() { w.tickOnce() }

// factoryCreateLocker makes a colony-owned storage locker entity.
func factoryCreateLocker(ms *MainState, x, y, z int) (*ecs.Entity, error) {
	e, err := factory.Create("storage_locker", x, y, z)
	if err != nil {
		return nil, err
	}
	e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = ms.colonyName()
	return e, nil
}

// The second planet: with no container at all the pad appears once and
// survives a save and load; with a pre-placed locker, first_build needs a new
// one.
func TestTutorialSecondPlanetPad(t *testing.T) {
	cases := []struct {
		scenario string
		wantPad  bool
	}{{"worms", true}, {"earth_like", false}}
	for _, tc := range cases {
		t.Run(tc.scenario, func(t *testing.T) {
			loadTutorialData(t)
			isolateSettlements(t)
			c, err := campaign.GenerateCampaign("Pad", 21)
			if err != nil {
				t.Fatal(err)
			}
			start := c.Locations[campaign.StartLocationID]
			start.MapID, start.ScenarioID = "earth_like", tc.scenario
			campaign.AddTutorialLocation(c)
			c.Tutorial.StepID = "land_again"
			c.Tutorial.Unlock(start.ID)
			c.CurrentLocationID = start.ID // the ship has made both jumps
			c.BindQuests()
			wm := NewWorldManager(c)
			wm.SeedShipCrew(2)
			wm.StockShipLocker(100, tutorialHoldStock())
			if err := wm.EnterStartingOrbit(); err != nil {
				t.Fatal(err)
			}
			startTutorial(c, wm)
			t.Cleanup(func() { activeTutorial = nil })
			b := activeTutorial
			if b == nil {
				t.Fatal("no bridge")
			}
			d := NewDashboardState(c, wm)
			hadHold := wm.SiteHoldEntity() != nil
			if hadHold == tc.wantPad {
				t.Fatalf("scenario %s: site hold present=%v before the frame", tc.scenario, hadHold)
			}
			for i := 0; i < 3; i++ {
				b.step("dash:starmap", nil, d)
			}
			if wm.SiteHoldEntity() == nil || !c.Tutorial.PadPlaced {
				t.Fatalf("no hold after the frame: placed=%v", c.Tutorial.PadPlaced)
			}
			pads := countStatic(wm.Current(), "teleporter")
			if tc.wantPad && pads != 1 || !tc.wantPad && pads != 0 {
				t.Fatalf("%d pads on %s", pads, tc.scenario)
			}
			for i := 0; i < 3; i++ {
				b.step("dash:starmap", nil, d)
			}
			if countStatic(wm.Current(), "teleporter") != pads {
				t.Fatal("pad placed twice")
			}

			// Save, reload: still exactly one hold, and no second pad. Saves go to
			// a temp dir; the step data keeps loading by absolute path.
			abs, err := filepath.Abs(tutorial.DefinitionPath)
			if err != nil {
				t.Fatal(err)
			}
			prevPath := tutorial.DefinitionPath
			tutorial.DefinitionPath = abs
			t.Cleanup(func() { tutorial.DefinitionPath = prevPath })
			t.Chdir(t.TempDir())
			if err := wm.SaveCampaign(nil); err != nil {
				t.Fatal(err)
			}
			c2, err := LoadCampaign(c.Name)
			if err != nil {
				t.Fatal(err)
			}
			c2.BindQuests()
			wm2 := NewWorldManager(c2)
			if err := wm2.EnterStartingOrbit(); err != nil {
				t.Fatal(err)
			}
			if !c2.Tutorial.PadPlaced || countStatic(wm2.Current(), "teleporter") != pads {
				t.Fatalf("after reload: placed=%v pads=%d", c2.Tutorial.PadPlaced, countStatic(wm2.Current(), "teleporter"))
			}

			// first_build on the pre-stocked planet waits for a new locker.
			if !tc.wantPad {
				c.Tutorial.StepID = "first_build"
				c.Tutorial.Baselines = nil
				startTutorial(c, wm)
				b = activeTutorial
				ms, err := wm.EnterCurrent()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(ms.teardown)
				before := countStatic(ms, "storage_locker")
				if before == 0 {
					t.Fatal("earth_like start has no locker to begin with")
				}
				for i := 0; i < 3; i++ {
					b.step(tutorial.ContextSite, ms, nil)
				}
				if b.dir.Done() {
					t.Fatal("first_build completed on the pre-placed locker")
				}
				// A new locker appears (placed directly, as a finished build would leave it).
				lx, ly, lz := wm.landingZone()
				x, y, z := freeLandingTile(ms.level, lx+2, ly, lz)
				e, _ := factoryCreateLocker(ms, x, y, z)
				ms.level.AddEntity(e)
				for i := 0; i < tutorialScanEvery+2; i++ {
					b.step(tutorial.ContextSite, ms, nil)
				}
				if !b.dir.Done() || !c.Tutorial.Done {
					t.Fatalf("first_build did not complete on the new locker (count %d -> %d)", before, countStatic(ms, "storage_locker"))
				}
			}
		})
	}
}
