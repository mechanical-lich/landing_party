package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/tutorial"
)

// assertTravelAndRing fails unless the current highlights include both the
// Travel button's rectangle and a planet ring.
func assertTravelAndRing(t *testing.T, b *tutorialBridge, p *starMapPanel) {
	t.Helper()
	x, y := p.travelBtn.GetAbsolutePosition()
	var travel, ring bool
	for _, hl := range b.highlightTargets() {
		if hl.radius > 0 {
			ring = true
		} else if hl.x == float32(x) && hl.y == float32(y) {
			travel = true
		}
	}
	if !travel || !ring {
		t.Errorf("highlights %+v: Travel button %v, planet ring %v, want both", b.highlightTargets(), travel, ring)
	}
}

// Highlights resolve to every visible target: the Travel button and the planet
// ring on the dashboard, a sidebar menu item or a region on the site, and
// nothing when the target is hidden or off screen.
func TestTutorialHighlightResolution(t *testing.T) {
	w := newTutorialWorld(t, 5)
	b := w.b

	// Step 1 on the Star Map: the Travel button together with the ring on the
	// Training Ground, which the first frame has centred and selected, and
	// whose list row resolves too.
	w.d.selectTabByID("starmap")
	b.step("dash:starmap", nil, w.d)
	p := w.d.starMapPanel()
	assertTravelAndRing(t, b, p)
	if tgt := b.starTarget(); tgt == nil || tgt.ID != campaign.TutorialLocationID {
		t.Fatalf("marker target = %v", tgt)
	}
	if row, ok := p.rowRect(campaign.TutorialLocationID); !ok || row.Height != 20 || row.Y < p.locList.GetBounds().Y {
		t.Errorf("row rect = %+v ok=%v", row, ok)
	}
	if _, ok := p.rowRect("no_such"); ok {
		t.Error("unknown row resolved")
	}
	// The ring: the tutorial planet's canvas position, when on the canvas.
	ring, ok := b.starHighlight(tutorial.AtTutorial)
	loc := w.c.Locations[campaign.TutorialLocationID]
	sx, sy := p.worldToScreen(loc.X, loc.Y)
	if p.inCanvas(int(sx), int(sy)) != ok {
		t.Errorf("ring ok=%v but planet on canvas=%v", ok, p.inCanvas(int(sx), int(sy)))
	}
	if ok && (ring.x != sx || ring.y != sy || ring.radius <= 0) {
		t.Errorf("ring = %+v, want centred on %v,%v", ring, sx, sy)
	}
	// Off the Star Map tab, dashboard targets do not resolve.
	w.d.selectTabByID("crew")
	b.step("dash:crew", nil, w.d)
	if got := b.highlightTargets(); len(got) != 0 {
		t.Errorf("star map widgets highlighted on another tab: %+v", got)
	}

	// On the site: a sidebar category, then its item once the sub-menu opens;
	// nothing when the sidebar is hidden.
	w.enterSite()
	s := w.site
	b.step(tutorial.ContextSite, s, nil)
	x, y, wdt, hgt, ok := s.guiManager.ElementBounds("cat_structures")
	if !ok || wdt <= 0 || hgt <= 0 {
		t.Fatalf("cat_structures bounds = %d,%d %dx%d ok=%v", x, y, wdt, hgt, ok)
	}
	if _, _, _, _, ok := s.guiManager.ElementBounds("build_storage_locker"); ok {
		t.Error("a closed sub-menu's item resolved")
	}
	if _, _, _, _, ok := s.guiManager.ElementBounds("no_such_element"); ok {
		t.Error("an unknown element resolved")
	}
	s.guiManager.SetSidebarVisible(false)
	if _, _, _, _, ok := s.guiManager.ElementBounds("cat_structures"); ok {
		t.Error("a hidden sidebar's item resolved")
	}
	s.guiManager.SetSidebarVisible(true)
	if hl, ok := b.hudHighlight("cat_structures"); !ok || int(hl.x) != x || int(hl.y) != y {
		t.Errorf("hud highlight = %+v ok=%v, want %d,%d", hl, ok, x, y)
	}
}

// Region and entity highlights on the Training Ground follow the camera.
func TestTutorialHighlightRegionAndEntity(t *testing.T) {
	loadTutorialData(t)
	isolateSettlements(t)
	ms := tutorialSite(t, 2)
	c := &campaign.Campaign{Name: "HL", CurrentLocationID: campaign.TutorialLocationID, Locations: map[string]*campaign.Location{
		campaign.TutorialLocationID: {ID: campaign.TutorialLocationID, Tutorial: true, Discovered: true},
	}}
	c.Tutorial = &campaign.TutorialProgress{StepID: "dig", NextLocationID: "x", Unlocked: []string{campaign.TutorialLocationID}}
	wm := NewWorldManager(c)
	wm.setCurrent(campaign.TutorialLocationID, ms)
	ms.wm, ms.campaign = wm, c
	startTutorial(c, wm)
	t.Cleanup(func() { activeTutorial = nil })
	b := activeTutorial
	if b == nil {
		t.Fatal("no bridge")
	}
	b.step(tutorial.ContextSite, ms, nil)

	hl, ok := b.regionHighlight("tut_outcrop")
	pt := ms.level.Regions["tut_outcrop"][0]
	if !ms.camera.Contains(pt[0], pt[1], pt[2]) {
		t.Fatalf("camera %+v does not show the outcrop at %v", ms.camera, pt)
	}
	if !ok {
		t.Fatal("outcrop region did not resolve")
	}
	ex, ey := ms.camera.WorldToScreenF(pt[0]-2, pt[1]-2)
	if hl.x != ex || hl.y != ey || hl.w != float32(5*ms.camera.TileW) {
		t.Errorf("region highlight = %+v, want 5x5 tiles at %v,%v", hl, ex, ey)
	}
	// dig lists the region alongside its menu items, so it draws with them,
	// and stands down while a storage inspector covers the screen.
	hasRegion := func() bool {
		for _, got := range b.highlightTargets() {
			if got == hl {
				return true
			}
		}
		return false
	}
	if !hasRegion() {
		t.Errorf("dig highlights %+v do not include the region", b.highlightTargets())
	}
	ms.storageInspector = &StorageInspectorModal{Visible: true}
	if hasRegion() {
		t.Error("region drew across an open storage inspector")
	}
	ms.storageInspector = nil
	if _, ok := b.entityHighlight("teleporter"); !ok {
		t.Error("the pad did not resolve as an entity highlight")
	}
	if _, ok := b.entityHighlight("research_lab"); ok {
		t.Error("an absent entity resolved")
	}
	// Scroll the camera away: nothing resolves.
	ms.camera.X += 200
	if _, ok := b.regionHighlight("tut_outcrop"); ok {
		t.Error("region resolved off screen")
	}
	if _, ok := b.entityHighlight("teleporter"); ok {
		t.Error("entity resolved off screen")
	}
}
