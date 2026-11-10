package game

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/generation"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
)

// observerRunWorld is a New Expedition started in observer mode: no player
// ship, the expedition a colony-doctrine fleet adrift at the Wreck Site.
func observerRunWorld(t *testing.T, seed int64, factions int) (*campaign.Campaign, *WorldManager) {
	t.Helper()
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	t.Cleanup(func() { observerRunActive = false })
	c, wm, err := newExpeditionOpts(fmt.Sprintf("Observer %d", seed), seed, false,
		campaign.GenOptions{Factions: factions, Observer: true})
	if err != nil {
		t.Fatal(err)
	}
	return c, wm
}

// flyDashboard steps the world as the dashboard does (TickBackground with
// no live level) for ticks, calling each after every tick.
func flyDashboard(wm *WorldManager, ticks int, each func(tick int)) {
	for tick := 1; tick <= ticks; tick++ {
		fspath.ResetFrameCounter()
		wm.TickBackground(nil)
		if each != nil {
			each(tick)
		}
	}
}

// untilLanded steps until the expedition is on a site, at most days.
func untilLanded(t *testing.T, c *campaign.Campaign, wm *WorldManager, days int) *campaign.Fleet {
	t.Helper()
	for i := 0; i < days*750; i++ {
		flyDashboard(wm, 1, nil)
		if f := c.Expedition(); f != nil && f.Landed() && c.Locations[f.LandedID].HasSite() {
			return f
		}
	}
	t.Fatalf("the expedition never landed on a site in %d days: %+v", days, c.Expedition())
	return nil
}

// An observer run has no player ship: the crew, hold and fuel are the
// expedition fleet's. It leaves the Wreck Site on its first day by the
// table, lands on a site (visiting it), pays the jump's fuel above the
// reserve, and the same seed flies the same way.
func TestObserverRunExpeditionFleet(t *testing.T) {
	fly := func() (string, *campaign.Campaign, *WorldManager) {
		c, wm := observerRunWorld(t, 777, 1)
		if !observerRun() || !observerMode() {
			t.Fatal("not an observer run")
		}
		if n := len(wm.ShipColonists()); n != 0 {
			t.Fatalf("%d colonists aboard a ship that is not the player's", n)
		}
		f := c.Expedition()
		if f == nil || f.Presence.Population != campaign.GenerationConfig().StartColonists ||
			f.Presence.Fuel != campaign.GenerationConfig().StartFuel || f.Presence.Stockpile["metal_ore"] == 0 {
			t.Fatalf("expedition fleet %+v", f)
		}
		var trace []string
		flyDashboard(wm, 40*750, func(tick int) {
			if tick%750 == 0 {
				b, _ := json.Marshal(c.Expedition())
				trace = append(trace, string(b))
			}
		})
		return strings.Join(trace, "\n"), c, wm
	}
	a, c, _ := fly()
	b, _, _ := fly()
	if a != b {
		t.Fatal("seed 777 flew two ways")
	}
	c2, wm2 := observerRunWorld(t, 777, 1)
	f := untilLanded(t, c2, wm2, 30)
	tbl := campaign.PlayerDoctrine(c2).Expedition
	if f.Presence.Fuel < tbl.FuelReserve || f.Presence.Fuel >= campaign.GenerationConfig().StartFuel {
		t.Errorf("fuel %d after the first jump", f.Presence.Fuel)
	}
	if !c2.Locations[f.LandedID].Visited {
		t.Error("the expedition's landing did not visit its site")
	}
	t.Logf("first landing %s on day %d; after 40 days: %s", f.LandedID, c2.Day(), a[strings.LastIndex(a, "\n")+1:])
	_ = c
}

// The viewer's whole interface: View opens the expedition's planet, where
// its colony is a settlement run by the settlement director; leaving it
// freezes the level (no colonist of the player's is there) and the fleet
// goes on under Tier 1. The ship's Star Map surface does nothing.
func TestObserverRunViewTheExpedition(t *testing.T) {
	c, wm := observerRunWorld(t, 777, 1)
	f := untilLanded(t, c, wm, 30)
	site, crew := f.LandedID, f.Presence.Population

	d := NewDashboardState(c, wm)
	p := d.starMapPanel()
	p.selectByID(site)
	here := c.CurrentLocationID
	p.travel()
	p.refreshCrew()
	p.beamDown()
	if c.CurrentLocationID != here || wm.Current() != nil {
		t.Fatal("the Star Map moved a ship in an observer run")
	}
	p.Update()
	if !p.viewBtn.IsEnabled() {
		t.Fatal("View disabled on a site")
	}
	p.view()
	ms, ok := p.pending.(*MainState)
	if !ok || ms.locationID != site {
		t.Fatalf("View did not open %s: %T", site, p.pending)
	}
	name := campaign.ExpeditionFaction + ":" + site
	if n := len(settlementWorkers(ms.level, name)); n != crew {
		t.Fatalf("%d expedition workers on its planet, want %d", n, crew)
	}
	if got := wm.viewLabel(ms); got != "VIEWING "+c.Locations[site].Name {
		t.Errorf("HUD view line %q", got)
	}
	stepLevel(ms, 300, nil)
	ms.checkTotalWipe()
	if c.Lost {
		t.Fatal("an observer run counted no colonists of the player's as a wipe")
	}
	if dir := ms.factionDirectors[name]; dir == nil || len(dir.Ordered) == 0 {
		t.Fatalf("the settlement director gave the expedition no orders: %+v", dir)
	}
	ms.openDashboard()
	if wm.IsLoaded(site) {
		t.Fatal("the expedition's planet stayed loaded after the viewer left")
	}
	if f := c.Expedition(); f == nil || f.Presence.Population <= 0 {
		t.Fatalf("the expedition did not survive the view: %+v", f)
	}
	flyDashboard(wm, 5*750, nil)
	if c.Expedition() == nil {
		t.Fatal("Tier 1 lost the expedition after the view")
	}
}

// pickUnloadedSite is a charted site with a level that is not in memory,
// with a faction presence when want is set.
func pickUnloadedSite(t *testing.T, c *campaign.Campaign, wm *WorldManager, want bool) *campaign.Location {
	t.Helper()
	var ids []string
	for id, loc := range c.Locations {
		if loc.Discovered && loc.HasSite() && !wm.IsLoaded(id) && id != c.CurrentLocationID &&
			(!want || len(loc.Factions) > 0) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		t.Skip("no unloaded site to view")
	}
	sort.Strings(ids)
	return c.Locations[ids[0]]
}

// expeditionWorld is a normal New Expedition (a player ship) with the
// title screen's default rivals.
func expeditionWorld(t *testing.T, seed int64) (*campaign.Campaign, *WorldManager) {
	t.Helper()
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	def, _ := campaign.FactionCountRange()
	c, wm, err := newExpeditionOpts(fmt.Sprintf("Expedition %d", seed), seed, false, campaign.GenOptions{Factions: def})
	if err != nil {
		t.Fatal(err)
	}
	return c, wm
}

// View under the observer lock on a normal campaign: a planet the ship is
// not at loads, stays loaded and ticks while viewed though no one is on
// it, and unloads on leaving with its Visited as it was; a planet with a
// colonist on it stays loaded.
func TestViewLoadsAndUnloads(t *testing.T) {
	c, wm := expeditionWorld(t, 11)
	loc := pickUnloadedSite(t, c, wm, false)
	visited := loc.Visited
	ms, err := wm.ViewLocation(loc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !wm.IsLoaded(loc.ID) || wm.Viewing() != loc.ID || !wm.ViewedElsewhere() {
		t.Fatal("the viewed planet is not loaded and viewed")
	}
	if got := wm.viewLabel(ms); !strings.HasPrefix(got, "VIEWING "+loc.Name) {
		t.Errorf("HUD view line %q", got)
	}
	before := ms.tick
	stepLevel(ms, 60, nil)
	if !wm.IsLoaded(loc.ID) || ms.tick <= before {
		t.Fatal("an unstaffed viewed planet was frozen or did not tick")
	}
	ms.openDashboard()
	if wm.IsLoaded(loc.ID) || wm.Viewing() != "" {
		t.Fatal("leaving a viewed-only planet left it loaded")
	}
	if loc.Visited != visited || loc.SaveFile == "" {
		t.Errorf("after leaving: visited %v (was %v), save %q", loc.Visited, visited, loc.SaveFile)
	}
	if err := wm.Travel(loc.ID); err != nil {
		t.Fatal(err)
	}
	if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
		t.Fatal(err)
	}
	ms, err = wm.ViewLocation(loc.ID)
	if err != nil {
		t.Fatal(err)
	}
	other := pickUnloadedSite(t, c, wm, false)
	if err := wm.Travel(other.ID); err != nil {
		t.Fatal(err)
	}
	ms.openDashboard()
	if !wm.IsLoaded(loc.ID) {
		t.Fatal("a staffed planet was unloaded when the viewer left")
	}
}

// presenceRecord is what Tier 1 keeps of a site's presences, without the
// Tier 2 marks a view leaves (Materialized, LevelStage).
func presenceRecord(loc *campaign.Location) string {
	ps := append([]campaign.FactionPresence(nil), loc.Factions...)
	for i := range ps {
		ps[i].Materialized, ps[i].LevelStage = false, 0
	}
	b, _ := json.Marshal(ps)
	return string(b)
}

// A planet only viewed steps as one never viewed once the viewer has left:
// the same campaign twice, one viewing a faction site and leaving at once,
// then thirty days of Tier 1 on both, ends with the same presence record.
func TestViewedPlanetStepsAsBeforeOnceLeft(t *testing.T) {
	ca, wma := expeditionWorld(t, 21)
	cb, wmb := expeditionWorld(t, 21)
	la := pickUnloadedSite(t, ca, wma, true)
	lb := cb.Locations[la.ID]
	ms, err := wma.ViewLocation(la.ID)
	if err != nil {
		t.Fatal(err)
	}
	ms.openDashboard()
	if wma.IsLoaded(la.ID) {
		t.Fatal("still loaded after leaving")
	}
	if a, b := presenceRecord(la), presenceRecord(lb); a != b {
		t.Fatalf("viewing changed the record:\n%s\n%s", a, b)
	}
	for day := 0; day < 30; day++ {
		ca.AdvanceDays(1)
		ca.StepFactions(wma.IsLoaded)
		cb.AdvanceDays(1)
		cb.StepFactions(wmb.IsLoaded)
	}
	if a, b := presenceRecord(la), presenceRecord(lb); a != b {
		t.Fatalf("thirty days after a view the planet steps differently:\n%s\n%s", a, b)
	}
}

// The console's observer on|off on a normal campaign flips the lock for the
// session: the marker, the reveal and the order block follow. In an
// observer run it refuses to turn off.
func TestConsoleObserverToggle(t *testing.T) {
	useObserver(t)
	setObserverMode(false)
	c := newE2EColony(t)
	ms := c.ms
	cm := newCheatModal(ms)
	cm.runCommand("observer on")
	if !observerMode() || observerRun() {
		t.Fatal("observer on did not turn the lock on (or made a run)")
	}
	ms.Update()
	if !ms.guiManager.ObserverIndicator() || !ms.level.RevealAll {
		t.Error("no marker or reveal after observer on")
	}
	ms.enterRogueMode(c.colonist)
	if ms.rogueEntity != nil {
		t.Fatal("rogue mode took a colonist under the lock")
	}
	cm.runCommand("observer off")
	if observerMode() {
		t.Fatal("observer off did not turn it off")
	}
	ms.Update()
	if ms.guiManager.ObserverIndicator() || ms.level.RevealAll {
		t.Error("marker or the reveal still up after observer off")
	}
	ms.enterRogueMode(c.colonist)
	if ms.rogueEntity == nil {
		t.Error("rogue mode still refused after observer off")
	}
	ms.exitRogueMode()

	// An observer run stays one.
	observerRunActive = true
	t.Cleanup(func() { observerRunActive = false })
	cm.runCommand("observer off")
	if !observerMode() {
		t.Error("observer off ended an observer run")
	}
}

// TestObserverRunLog prints what the expedition does on a seed over a
// number of days (LP_EXPEDITION_DAYS; skipped without it): its lift-offs,
// landings, crew and fuel. LP_EXPEDITION_SEED, default 777; Rival
// factions LP_EXPEDITION_FACTIONS, default 1.
func TestObserverRunLog(t *testing.T) {
	days, err := strconv.Atoi(os.Getenv("LP_EXPEDITION_DAYS"))
	if err != nil || days <= 0 {
		t.Skip("set LP_EXPEDITION_DAYS to print the expedition's moves")
	}
	seed := int64(777)
	if v, err := strconv.ParseInt(os.Getenv("LP_EXPEDITION_SEED"), 10, 64); err == nil {
		seed = v
	}
	factions := envInt("LP_EXPEDITION_FACTIONS", 1)
	c, wm := observerRunWorld(t, seed, factions)
	name := func(id string) string {
		if l := c.Locations[id]; l != nil {
			return l.Name + " (" + id + ", " + c.LocationTier(l) + ")"
		}
		return id
	}
	var log []string
	prev := ""
	for c.Day() < days {
		flyDashboard(wm, 1, nil)
		f := c.Expedition()
		if f == nil {
			log = append(log, fmt.Sprintf("day %d: the expedition is gone", c.Day()))
			break
		}
		state := "landed " + f.LandedID
		if !f.Landed() {
			state = "flying " + f.ToID
		}
		if state != prev {
			prev = state
			if f.Landed() {
				log = append(log, fmt.Sprintf("day %d: landed on %s, crew %d, fuel %d", c.Day(), name(f.LandedID), f.Presence.Population, f.Presence.Fuel))
			} else {
				log = append(log, fmt.Sprintf("day %d: lifted off (%s) for %s, arriving day %d, crew %d, fuel %d", c.Day(), f.LastMove, name(f.ToID), f.ArriveDay, f.Presence.Population, f.Presence.Fuel))
			}
		}
	}
	t.Logf("seed %d, %d days, Rival factions %d:\n%s", seed, days, factions, strings.Join(log, "\n"))
}

// The title screen's tutorial toggle starts off under observer mode (an
// observer run has no ship to teach on) and on otherwise (by default).
func TestTutorialToggleOffInObserverMode(t *testing.T) {
	useObserver(t) // the config flag, which the title reads
	ts := &TitleState{}
	ts.buildNewSettlementScreen()
	if ts.tutorialToggle.IsOn() {
		t.Error("the tutorial toggle starts on in observer mode")
	}
	config.Global().ObserverMode = false
	ts = &TitleState{}
	ts.buildNewSettlementScreen()
	if want := !config.Global().TutorialOffDefault; ts.tutorialToggle.IsOn() != want {
		t.Errorf("tutorial toggle %v without observer mode, want %v", ts.tutorialToggle.IsOn(), want)
	}
}

// Viewing an unstaffed planet with a presence builds it: its stamp (origin)
// and bodies are on the level, the camera opens on it, and leaving
// reconciles and unloads so the next View finds the same presence, neither
// doubled nor lost.
func TestViewBuildsThePresence(t *testing.T) {
	c, wm := observerRunWorld(t, 777, 1)
	var loc *campaign.Location
	for _, id := range sortedLocIDs(c) {
		if l := c.Locations[id]; l.HasSite() && len(l.Factions) > 0 && !wm.IsLoaded(id) {
			loc = l
			break
		}
	}
	if loc == nil {
		t.Skip("no unloaded presence site")
	}
	id, pid := loc.ID, loc.Factions[0].ID
	view := func() (int, int) {
		ms, err := wm.ViewLocation(id)
		if err != nil {
			t.Fatal(err)
		}
		x, y, z, ok := generationOrigin(ms, pid)
		if !ok {
			t.Fatalf("presence %s has no stamp on the viewed level", pid)
		}
		bodies := len(presenceBodies(ms.level, pid))
		if bodies == 0 {
			t.Fatalf("no bodies of %s on the viewed level", pid)
		}
		cam := ms.camera
		st := cam.SidebarTiles()
		cx, cy := cam.X+st+(cam.ViewW()-st)/2, cam.Y+cam.ViewH()/2
		if absI(cx-x) > 2 || absI(cy-y) > 2 || cam.Z != z {
			t.Errorf("camera centred on %d,%d,%d, the presence at %d,%d,%d", cx, cy, cam.Z, x, y, z)
		}
		ms.openDashboard()
		if wm.IsLoaded(id) {
			t.Fatal("the viewed planet stayed loaded")
		}
		p := c.Locations[id].PresenceByID(pid)
		if p == nil {
			t.Fatal("the presence was lost on leaving")
		}
		return bodies, p.Population
	}
	b1, pop1 := view()
	b2, pop2 := view()
	if b2 != b1 || pop2 != pop1 || len(c.Locations[id].Factions) != 1 {
		t.Fatalf("second View: %d bodies, pop %d (first %d, %d), %d presences", b2, pop2, b1, pop1, len(c.Locations[id].Factions))
	}
	t.Logf("%s: %d bodies, population %d, both Views", id, b1, pop1)
}

func sortedLocIDs(c *campaign.Campaign) []string {
	ids := make([]string, 0, len(c.Locations))
	for id := range c.Locations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func generationOrigin(ms *MainState, pid string) (int, int, int, bool) {
	return generation.Origin(ms.level, factionOriginKey(pid))
}

// The console's observer on in a normal campaign is that campaign's lock:
// it never writes the config flag the title reads as "start an observer
// run", and the next campaign starts without it (review finding 5).
func TestConsoleObserverLockDoesNotLeakIntoNextRun(t *testing.T) {
	cfg := config.Global()
	prev := cfg.ObserverMode
	cfg.ObserverMode = false
	t.Cleanup(func() { cfg.ObserverMode, observerLock = prev, nil })
	c := newE2EColony(t)
	cm := newCheatModal(c.ms)
	cm.runCommand("observer on")
	if !observerMode() {
		t.Fatal("observer on did not lock the campaign")
	}
	if cfg.ObserverMode {
		t.Fatal("observer on wrote the config flag that seeds New Expedition")
	}
	cm.runCommand("observer off")
	if observerMode() {
		t.Fatal("observer off left the lock on")
	}
	cm.runCommand("observer on")
	NewWorldManager(&campaign.Campaign{Version: campaign.CampaignVersion, Name: "Next",
		Locations: map[string]*campaign.Location{}})
	if observerMode() {
		t.Error("the last campaign's lock carried into a new one")
	}
}
