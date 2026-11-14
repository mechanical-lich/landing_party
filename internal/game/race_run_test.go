package game

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/gui"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// hiveRun starts a hive run (phase F) on seed with factions rivals and views
// its first world, as the title screen does.
func hiveRun(t *testing.T, seed int64, factions int) (*campaign.Campaign, *WorldManager, *MainState) {
	t.Helper()
	return raceRun(t, "hive", seed, factions)
}

// raceRun starts a run of race (phase F) as the title screen does and views
// its first world.
func raceRun(t *testing.T, race string, seed int64, factions int) (*campaign.Campaign, *WorldManager, *MainState) {
	t.Helper()
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	t.Cleanup(func() { spreadRunActive = false; setFactionDoctrines(nil) })
	c, wm, err := newExpeditionOpts("Brood", seed, false, campaign.GenOptions{Factions: factions, Doctrine: race})
	if err != nil {
		t.Fatal(err)
	}
	ms, err := wm.ViewLocation(campaign.StartLocationID)
	if err != nil {
		t.Fatal(err)
	}
	return c, wm, ms
}

// stepRun steps a viewed planet n ticks the way the game does.
func stepRun(wm *WorldManager, ms *MainState, n int) {
	for i := 0; i < n; i++ {
		fspath.ResetFrameCounter()
		ms.stepWorld()
		wm.TickBackground(ms)
		if ms.tick%30 == 0 {
			ms.purgeCompletedTasks()
		}
		ms.advanceClock()
	}
}

// A hive run has no ship crew or hold, a presence of the player's faction
// on the start system at the race's start, and opens viewing it with the
// cursor bound to that presence's settlement.
func TestHiveRunStarts(t *testing.T) {
	c, wm, ms := hiveRun(t, 7, 0)
	if !c.SpreadRun() || !spreadRun() || !shiplessRun() {
		t.Fatalf("spread run %v / %v", c.SpreadRun(), spreadRun())
	}
	if n := len(wm.ShipColonists()); n != 0 {
		t.Errorf("%d crew aboard a hive run's ship", n)
	}
	loc := c.Locations[campaign.StartLocationID]
	p := loc.Presence(campaign.PlayerFaction)
	if p == nil || p.Doctrine != "hive" || p.Population != 8 || !p.Known || c.Player.Faction != campaign.PlayerFaction {
		t.Fatalf("player presence %+v (player %+v)", p, c.Player)
	}
	name := playerSettlementName(loc.ID)
	if ms.MainSettlement == nil || ms.MainSettlement.Name != name || ms.colonyName() != name || !ms.ordersHere() {
		t.Fatalf("bound settlement %v, want %s", ms.MainSettlement, name)
	}
	if n := len(settlementWorkers(ms.level, name)); n < 4 {
		t.Errorf("%d workers on the brood, want the start's adults", n)
	}
	if got := len(wm.PlanetColonists()); got == 0 {
		t.Error("the crew panel lists none of the brood")
	}
	var hive bool
	for _, cat := range buildMenuOf(ms.doctrineSettlement()) {
		for _, k := range cat.Types {
			hive = hive || k == "biomass_pool"
		}
	}
	if !hive {
		t.Error("the build menu is not the hive's")
	}
	if c.Stance(campaign.PlayerFaction) != campaign.StanceNeutral {
		t.Error("the player's faction is not neutral to itself")
	}
	for _, e := range settlementWorkers(ms.level, name) {
		if components.IsAttackTarget(e) {
			t.Fatal("the player's own drone is an attack target")
		}
	}
}

// The player's orders go into the brood's queue: an egg queued from a
// larva's crafting modal is laid by a drone, and a pool ordered by the
// build cursor is placed on resin.
func TestHiveRunOrdersBindToTheViewedSite(t *testing.T) {
	_, wm, ms := hiveRun(t, 7, 0)
	name := ms.MainSettlement.Name
	var larva *ecs.Entity
	for _, list := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range list {
			if e != nil && e.Blueprint == "hive_larva" {
				larva = e
			}
		}
	}
	if larva == nil {
		t.Fatal("no larva at the start")
	}
	if ms.foreignStation(larva) {
		t.Fatalf("the brood's larva reads as %q's", ms.stationOwner(larva))
	}
	if err := ms.HandleEvent(gui.StationClickedEvent{Station: larva}); err != nil {
		t.Fatal(err)
	}
	ids := ms.guiManager.CraftingRecipeIDs()
	found := false
	for _, id := range ids {
		found = found || id == "hive_egg_drone"
	}
	if !found {
		t.Fatalf("larva rows %v, want hive_egg_drone", ids)
	}
	if err := ms.HandleEvent(gui.CraftRequestedEvent{RecipeID: "hive_egg_drone", Station: larva}); err != nil {
		t.Fatal(err)
	}
	queued := false
	for _, tk := range settlement.Settlements[name].Tasks.GetTasks() {
		if cr, ok := tk.Data.(task_requests.CraftRequest); ok && cr.RecipeID == "hive_egg_drone" {
			queued = true
		}
	}
	if !queued {
		t.Fatal("the egg was not queued on the brood")
	}
	stepRun(wm, ms, 1500)
	if !larva.HasComponent(rlcomponents.Dead) && hasEntity(ms.level, larva) {
		t.Error("the larva was never laid on in 1500 ticks")
	}
}

// A rival colony camped on the hive's world turns hostile the day a body of
// the brood comes within reach of one of its workers; its workers then are
// attack targets for the brood.
func TestRaceContactOnTheGround(t *testing.T) {
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	t.Cleanup(func() { spreadRunActive = false; setFactionDoctrines(nil); components.ResetFactionHostility() })
	c, wm, err := newExpeditionOpts("Brood", 7, false, campaign.GenOptions{Doctrine: "hive"})
	if err != nil {
		t.Fatal(err)
	}
	loc := c.Locations[campaign.StartLocationID]
	loc.Factions = append(loc.Factions, campaign.FactionPresence{ID: "fpm", Faction: "mirror", Doctrine: campaign.RivalColonyDoctrine, Population: 3,
		Stockpile: map[string]int{"metal_ore": 30}, Seed: 77, LastShotDay: campaign.NeverFired})
	ms, err := wm.ViewLocation(loc.ID)
	if err != nil {
		t.Fatal(err)
	}
	theirs := settlementWorkers(ms.level, "mirror:"+loc.ID)
	mine := settlementWorkers(ms.level, ms.colonyName())
	if len(theirs) == 0 || len(mine) == 0 {
		t.Fatalf("mirror workers %d, brood %d", len(theirs), len(mine))
	}
	wm.raceContacts()
	if c.Stance("mirror") != campaign.StanceNeutral {
		t.Fatal("hostile before the two met")
	}
	tp, _ := entityPos(theirs[0])
	mp, _ := entityPos(mine[0])
	mp.X, mp.Y, mp.Z = tp.GetX()+1, tp.GetY(), tp.GetZ()
	wm.raceContacts()
	if c.Stance("mirror") != campaign.StanceHostile || !components.IsAttackTarget(theirs[0]) {
		t.Errorf("after meeting: stance %q, attack target %v", c.Stance("mirror"), components.IsAttackTarget(theirs[0]))
	}
}

// Hidden identity inverted: the brood's own bodies carry their types'
// names, its Factions block and Legends lines name the hive, and a wild
// xeno stays a stranger (another faction).
func TestHiveRunNamesTheHive(t *testing.T) {
	c, wm, ms := hiveRun(t, 7, 0)
	for _, e := range settlementWorkers(ms.level, ms.colonyName()) {
		dc := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
		if dc.Name == "Xeno" || dc.Name == "Troid" || dc.Faction != campaign.PlayerFaction {
			t.Errorf("a body of the brood is %q of %q", dc.Name, dc.Faction)
		}
	}
	d := campaign.LoadFactionData().Doctrine("hive")
	loc := c.Locations[campaign.StartLocationID]
	if _, name := factionKey(c, d, loc.Presence(campaign.PlayerFaction), loc); !strings.Contains(name, "Hive") {
		t.Errorf("the Factions block is %q", name)
	}
	if hudFactionLabel(campaign.PlayerFaction) == campaign.PlayerFaction {
		t.Error("the HUD names the faction key")
	}
	for i := 0; i < broodLossThreshold; i++ {
		wm.noteBroodLoss(loc.ID)
	}
	before := len(c.History.Events)
	wm.recordBroodLosses()
	if len(c.History.Events) != before+1 || c.History.Events[len(c.History.Events)-1].Type != "brood_losses" {
		t.Error("five losses in a day made no brood_losses event")
	}
}

// An autonz run: no ship, the node start on the start system with the
// cursor bound, the autonz's HUD (metal, crystal, ferrite, conversion; Lay
// Wire and Deploy Relay; Assimilation), and the Signal Core's panel over the
// signal source.
func TestAutonzRunStarts(t *testing.T) {
	c, _, ms := raceRun(t, "autonz", 7, 0)
	if !c.SpreadRun() || len(ms.wm.ShipColonists()) != 0 {
		t.Fatal("not a spread run, or crew aboard")
	}
	loc := c.Locations[campaign.StartLocationID]
	p := loc.Presence(campaign.PlayerFaction)
	if p == nil || p.Doctrine != "autonz" || p.Population != 6 {
		t.Fatalf("player presence %+v", p)
	}
	if ms.colonyName() != playerSettlementName(loc.ID) || !ms.ordersHere() {
		t.Fatalf("cursor bound to %q", ms.colonyName())
	}
	gm := ms.guiManager
	if got := gm.ResourceSlots(); !reflect.DeepEqual(got, []string{"metal_ore", "crystal", "ferrite", "conversion"}) {
		t.Errorf("slots %v", got)
	}
	ms.refreshHUD()
	if gm.ResourceValue("metal_ore") <= 0 {
		t.Errorf("metal %d at the start", gm.ResourceValue("metal_ore"))
	}
	if orders := gm.OrderIDs(); !contains(orders, "convert") || !contains(orders, "deploy") || gm.ResearchTitle() != "Assimilation" {
		t.Errorf("orders %v, research %q", orders, gm.ResearchTitle())
	}
	if ms.convertTile() != "blue_wires" {
		t.Errorf("convert tile %q", ms.convertTile())
	}
	units := settlementWorkers(ms.level, ms.colonyName())
	if len(units) == 0 {
		t.Fatal("no units")
	}
	ms.openColonistModal(units[0])
	if got := gm.UnitModalShown(); contains(got, gui.SectionNeeds) || contains(got, gui.SectionEquipment) || !contains(got, gui.SectionTypeTier) {
		t.Errorf("unit modal %v", got)
	}
	core, err := factory.Create("signal_core", ms.MainSettlement.CenterX+2, ms.MainSettlement.CenterY+2, ms.MainSettlement.CenterZ)
	if err != nil {
		t.Fatal(err)
	}
	ms.level.AddEntity(core)
	ms.openStructurePanel(core)
	if got := gm.StructurePanelShown(); !reflect.DeepEqual(got, []string{"readiness", "target_list", "action", "action", "world_toggle", "group_toggles"}) || gm.StructurePanelTitle() != "Signal Core" {
		t.Errorf("core panel %q %v", gm.StructurePanelTitle(), got)
	}
}

// Deploy Relay: a click on wired ground sends the nearest idle relay drone
// of the player's to fold into a relay pylon there; with none idle, nothing.
func TestAutonzDeployOrder(t *testing.T) {
	_, _, ms := raceRun(t, "autonz", 7, 0)
	st := ms.MainSettlement
	x, y, z := st.CenterX+1, st.CenterY+1, st.CenterZ
	if ms.deployAt(x, y, z) {
		t.Fatal("deployed with no relay drone")
	}
	e, err := spawnFactionWorker(ms.level, "relay_drone_crew", st.Name, campaign.PlayerFaction, st.CenterX, st.CenterY, z)
	if err != nil {
		t.Fatal(err)
	}
	if k := components.KinOf(e); k != nil {
		k.Type = "relay_drone"
	} else {
		e.AddComponent(&components.KinComponent{Type: "relay_drone", Settlement: st.Name})
	}
	if !ms.deployAt(x, y, z) {
		t.Fatal("the relay drone was not sent to deploy on wire")
	}
	wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
	if wc.CurrentTask == nil {
		t.Error("the drone holds no deploy task")
	}
}

// Leaving a world hands it back to its director: every group taken over
// there is released, and the aim stays.
func TestHandBackOnLeaving(t *testing.T) {
	_, wm, ms := hiveRun(t, 7, 0)
	pr := ms.playerPresenceHere()
	ms.panelToggle(worldToggle, true)
	pr.LaunchTarget = "somewhere"
	if len(pr.TakenOver) == 0 {
		t.Fatal("nothing taken over")
	}
	wm.EndView()
	if len(pr.TakenOver) != 0 || pr.LaunchTarget != "somewhere" {
		t.Errorf("after leaving: taken over %v, aim %q", pr.TakenOver, pr.LaunchTarget)
	}
}
