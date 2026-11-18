package game

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	shippkg "github.com/mechanical-lich/landing_party/internal/ship"
	"github.com/mechanical-lich/landing_party/internal/systems"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// greyRun starts a grey run (phase F step 6) as the title screen does: a
// ship run with the greys' crew aboard.
func greyRun(t *testing.T, seed int64, factions int) (*campaign.Campaign, *WorldManager) {
	t.Helper()
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	t.Cleanup(func() {
		setFactionDoctrines(nil)
		systems.PlayerFaction = campaign.DefaultPlayerDoctrine
		playerPocketBase = ""
	})
	c, wm, err := newExpeditionOpts("Saucer", seed, false, campaign.GenOptions{Factions: factions, Doctrine: "greyish"})
	if err != nil {
		t.Fatal(err)
	}
	return c, wm
}

func faction(e *ecs.Entity) string {
	if !e.HasComponent(rlcomponents.Description) {
		return ""
	}
	return e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction
}

// A grey run is a ship run: its crew is the race's start (a worker, a
// technician, a soldier with a beam pistol), typed, of the player's
// faction, without needs; the vat and console stand aboard, owned by the
// ship; the hold is the race's, with no rations.
func TestGreyRunStarts(t *testing.T) {
	c, wm := greyRun(t, 7, 3)
	if c.SpreadRun() || spreadRun() || shiplessRun() {
		t.Fatal("a grey run is a ship run")
	}
	crew := wm.ShipColonists()
	types := map[string]int{}
	for _, e := range crew {
		k := components.KinOf(e)
		if k == nil || k.Type == "" {
			t.Errorf("%s has no type", e.Blueprint)
			continue
		}
		types[k.Type]++
		if f := faction(e); f != campaign.PlayerFaction {
			t.Errorf("%s faction %q, want %q", k.Type, f, campaign.PlayerFaction)
		}
		if e.HasComponent(components.Needs) {
			t.Errorf("%s has needs", k.Type)
		}
		if k.Type == "grey_soldier" {
			inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
			if inv.RightHand == nil || inv.RightHand.Blueprint != "grey_beam_pistol" {
				t.Errorf("the soldier holds %v, want a beam pistol", inv.RightHand)
			}
		}
	}
	if len(crew) != 3 || types["grey_worker"] != 1 || types["grey_technician"] != 1 || types["grey_soldier"] != 1 {
		t.Errorf("crew %v, want a worker, a technician and a soldier", types)
	}
	ship := wm.ShipLevel()
	aboard := map[string]bool{}
	for _, list := range [][]*ecs.Entity{ship.Entities, ship.StaticEntities} {
		for _, e := range list {
			if e != nil && (e.Blueprint == "grey_vat" || e.Blueprint == "grey_console") {
				if k := components.KinOf(e); k == nil || k.Settlement != wm.shipSettlementName() {
					t.Errorf("%s aboard is not the ship's", e.Blueprint)
				}
				aboard[e.Blueprint] = true
			}
		}
	}
	if !aboard["grey_vat"] || !aboard["grey_console"] {
		t.Errorf("aboard %v, want the vat and the console", aboard)
	}
	hold := wm.shipLocker()
	if hold == nil || hold.CountResource("grey_alloy") != 4 || hold.CountResource("ration_pack") != 0 {
		t.Errorf("hold: alloy %d, rations %d", hold.CountResource("grey_alloy"), hold.CountResource("ration_pack"))
	}
}

// Rival greys are hostile to a grey player, and the player's own crew
// never is; the ship's doors take the player's faction.
func TestGreyRunHostility(t *testing.T) {
	c, wm := greyRun(t, 7, 3)
	syncFactionHostility(c)
	if !components.IsFactionHostile("greyish") {
		t.Error("rival greys are not hostile to a grey player")
	}
	for _, e := range wm.ShipColonists() {
		if components.IsHostileWorker(e) {
			t.Errorf("the player's own %s counts as hostile", e.Blueprint)
		}
	}
	if systems.PlayerFaction != campaign.PlayerFaction {
		t.Errorf("door faction %q, want %q", systems.PlayerFaction, campaign.PlayerFaction)
	}
}

// The colony keeps its own faction for its doors.
func TestColonyRunDoorFaction(t *testing.T) {
	useContentTables(t)
	isolateSettlements(t)
	if _, _, err := newExpedition("Colony", 7, false); err != nil {
		t.Fatal(err)
	}
	if systems.PlayerFaction != "colony" {
		t.Errorf("colony door faction %q", systems.PlayerFaction)
	}
}

// The vat, console and array are built only aboard the ship.
func TestShipOnlyBuildables(t *testing.T) {
	_, wm := greyRun(t, 7, 0)
	ship := wm.shipLevel
	if ship == nil || !ship.aboardShip() {
		t.Fatal("the ship state is not aboard")
	}
	if !ship.queueBuild("grey_vat", 50, 50, 0) {
		t.Error("the vat refused aboard")
	}
	planet := &MainState{wm: wm, level: ship.level, MainSettlement: ship.MainSettlement}
	for _, key := range []string{"grey_vat", "grey_console", "grey_array"} {
		if planet.queueBuild(key, 52, 52, 0) {
			t.Errorf("%s ordered off the ship", key)
		}
	}
	if !planet.queueBuild("storage_locker", 52, 52, 0) {
		t.Error("a locker refused off the ship")
	}
}

// greyLocker stands a storage locker owned by name on level.
func greyLocker(t *testing.T, level *world.Level, name string, x, y, z int) (*ecs.Entity, *components.StorageComponent) {
	t.Helper()
	e, err := factory.Create("storage_locker", x, y, z)
	if err != nil {
		t.Fatal(err)
	}
	sc := e.GetComponent(components.Storage).(*components.StorageComponent)
	sc.OwnedBy = name
	level.AddEntity(e)
	return e, sc
}

// greyInOrbit is a grey run in orbit of the start system, its level loaded.
func greyInOrbit(t *testing.T) (*campaign.Campaign, *WorldManager) {
	t.Helper()
	c, wm := greyRun(t, 7, 0)
	if err := wm.Travel(c.StartID); err != nil {
		t.Fatalf("travel: %v", err)
	}
	if wm.Current() == nil || wm.Current().level == nil {
		t.Fatal("no site loaded in orbit")
	}
	t.Cleanup(wm.Current().teardown)
	return c, wm
}

// Every grey locker of the player's on a level is a door onto one pocket
// (phase F step 6b): aboard, a locker and the hold share one stock; on the
// site, two lockers share another; a beam up from either site door takes
// from the whole site pocket into the ship's.
func TestGreyLockersArePockets(t *testing.T) {
	_, wm := greyInOrbit(t)
	ship := wm.ShipLevel()
	_, aboard := greyLocker(t, ship, wm.shipSettlementName(), 48, 52, 0)
	site := wm.Current().level
	cx, cy, cz := wm.landingZone()
	_, a := greyLocker(t, site, wm.currentColonyName(), cx+2, cy, cz)
	doorB, _ := greyLocker(t, site, wm.currentColonyName(), cx-2, cy, cz)
	wm.relinkPlayerPockets()
	if aboard.CountResource("grey_alloy") != 4 {
		t.Errorf("a locker aboard sees %d alloy, want the hold's 4", aboard.CountResource("grey_alloy"))
	}
	a.AddItem(stack(t, "crystal", 5))
	if err := wm.BeamResourceUpFrom(doorB, "crystal", 5); err != nil {
		t.Fatalf("beam up from the other site door: %v", err)
	}
	if a.CountResource("crystal") != 0 || aboard.CountResource("crystal") != 12+5 {
		t.Errorf("site crystal %d, ship crystal %d", a.CountResource("crystal"), aboard.CountResource("crystal"))
	}
	if err := wm.BeamResourceDown("metal_ore", 10); err != nil {
		t.Fatalf("beam down: %v", err)
	}
	if a.CountResource("metal_ore") != 10 {
		t.Errorf("the site pocket holds %d metal after the beam down", a.CountResource("metal_ore"))
	}
}

// A colony run keeps ordinary lockers: no pocket, its global inventory
// gated by research; a grey run's tiers are open from the start.
func TestColonyLockersAreNotPockets(t *testing.T) {
	useContentTables(t)
	isolateSettlements(t)
	c, _, err := newExpedition("Colony", 7, false)
	if err != nil {
		t.Fatal(err)
	}
	base := campaignColonyName(c)
	if pocketOwner(base) || pocketOwner(base+" Ship") || pocketOwner(base+" @ Somewhere") {
		t.Error("a colony settlement counts as a pocket")
	}
	if (&globalInvPanel{campaign: c}).invOpen(techGlobalInvAll) {
		t.Error("a colony's All Sites view is open without research")
	}
	g, _ := greyRun(t, 7, 0)
	if !(&globalInvPanel{campaign: g}).invOpen(techGlobalInvDetailed) {
		t.Error("a grey run's By Site view is shut")
	}
}

// A grey run's ship is the saucer (phase F step 6c): its doctrine's hull
// tiles (which the tile catalogue has), a round hull of the start radius
// round the hull centre, the crew, the hold and the modules inside it.
func TestGreyShipIsASaucer(t *testing.T) {
	c, wm := greyRun(t, 7, 0)
	sp := playerSettlementOf(c).SaucerShip()
	if sp == nil || sp.Start.Radius != 8 {
		t.Fatalf("grey saucer spec %+v", sp)
	}
	for _, tile := range []string{sp.Floor, sp.Wall} {
		if _, ok := world.TileNameToIndex[tile]; !ok {
			t.Errorf("no tile %q", tile)
		}
	}
	lvl := wm.ShipLevel()
	cx, cy, cz := shippkg.HullCenter()
	rr := world.NewRoundRoom(sp.Start.Radius)
	if f := world.TileIndexToName[lvl.GetTilePtr(cx, cy, cz).Floor.Type]; f != sp.Floor {
		t.Errorf("the hull centre's floor is %q", f)
	}
	inside := func(e *ecs.Entity) bool {
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		dx, dy := pc.GetX()-cx, pc.GetY()-cy
		return pc.GetZ() == cz && rr.In(dx, dy) && !rr.Wall[[2]int{dx, dy}]
	}
	n := 0
	for _, list := range [][]*ecs.Entity{lvl.Entities, lvl.StaticEntities} {
		for _, e := range list {
			if e == nil || !e.HasComponent(rlcomponents.Position) {
				continue
			}
			n++
			if !inside(e) {
				t.Errorf("%s stands outside the saucer", e.Blueprint)
			}
		}
	}
	if n < 6 {
		t.Errorf("%d things aboard, want the crew, the hold, the vat and the console", n)
	}
}

// greyStock tops the ship's hold up for hull work.
func greyStock(wm *WorldManager, metal, alloy int) {
	wm.addToShipHold("metal_ore", metal)
	wm.addToShipHold("grey_alloy", alloy)
}

// The saucer grows by work (phase F step 6d): Expand Hull on deck 1 queues
// a breach in the old rim, the new ring's floor, the new rim, then the old
// rim dug out, phase by phase; the crew does it all from the hold; the deck
// ends a radius-10 round room with no wall inside it. The next ring needs
// the hull lattice, and a second job waits for the first.
func TestGreyHullExpands(t *testing.T) {
	_, wm := greyRun(t, 7, 0)
	greyStock(wm, 300, 120)
	ship := wm.shipLevel
	if err := ship.expandHull(0); err != nil {
		t.Fatal(err)
	}
	if err := ship.expandHull(0); err == nil {
		t.Error("a second expansion started while one runs")
	}
	if err := ship.addDeck(); err == nil {
		t.Error("a deck started while the hull grows")
	}
	sp := ship.saucerSpec()
	done := false
	for tick := 0; tick < 80000 && !done; tick++ {
		if done = false; tick%10000 == 0 {
			j, _ := readHullJob(ship.level)
			t.Logf("tick %d: %s, %d left", tick, j, ship.hullPhaseLeft(j))
		}
		wm.TickBackground(nil)
		_, running := readHullJob(ship.level)
		done = !running
	}
	if !done {
		j, _ := readHullJob(ship.level)
		t.Fatalf("the expansion did not finish: %s, %d tiles left", j, ship.hullPhaseLeft(j))
	}
	if r := hullRadius(ship.level, sp, 0); r != 10 {
		t.Errorf("deck 1 radius %d, want 10", r)
	}
	cx, cy, cz := shippkg.HullCenter()
	rr := world.NewRoundRoom(10)
	for _, c := range rr.Cells() {
		tile := ship.level.GetTilePtr(cx+c[0], cy+c[1], cz)
		if tile.Floor.IsEmpty() {
			t.Fatalf("no floor at %v", c)
		}
		if rr.Wall[c] != !tile.Middle.IsEmpty() {
			t.Fatalf("at %v: wall %v, middle empty %v", c, rr.Wall[c], tile.Middle.IsEmpty())
		}
	}
	if err := ship.expandHull(0); err == nil || !strings.Contains(err.Error(), "Hull Lattice") {
		t.Errorf("the ring to 12 without the lattice: %v", err)
	}
}

// Add Deck (phase F step 6d) builds stairs up from the top deck, then the
// new deck's floor out from the stairs' head and its rim: deck 2, a
// radius-8 round room above deck 1, reached by the stairs. The third deck
// needs the hull lattice.
func TestGreyHullAddsADeck(t *testing.T) {
	_, wm := greyRun(t, 7, 0)
	greyStock(wm, 400, 120)
	ship := wm.shipLevel
	if err := ship.addDeck(); err != nil {
		t.Fatal(err)
	}
	sp := ship.saucerSpec()
	done := false
	for tick := 0; tick < 400000 && !done; tick++ {
		wm.TickBackground(nil)
		_, running := readHullJob(ship.level)
		done = !running
	}
	if !done {
		j, _ := readHullJob(ship.level)
		t.Fatalf("the deck did not finish: %s, %d tiles left", j, ship.hullPhaseLeft(j))
	}
	if n, r := hullDecks(ship.level, sp), hullRadius(ship.level, sp, 1); n != 2 || r != 8 {
		t.Errorf("decks %d, deck 2 radius %d", n, r)
	}
	cx, cy, cz := shippkg.HullCenter()
	sx, sy := hullStairs()
	if up := ship.level.GetTilePtr(sx, sy, cz); world.TileIndexToName[up.Middle.Type] != "stairs_up" {
		t.Errorf("no stairs up on deck 1")
	}
	rr := world.NewRoundRoom(8)
	for _, c := range rr.Cells() {
		tile := ship.level.GetTilePtr(cx+c[0], cy+c[1], cz+1)
		if c == [2]int{sx - cx, sy - cy} {
			continue // the stairs' head
		}
		if tile.Floor.IsEmpty() || rr.Wall[c] != !tile.Middle.IsEmpty() {
			t.Fatalf("deck 2 at %v: floor empty %v, middle empty %v, wall %v", c, tile.Floor.IsEmpty(), tile.Middle.IsEmpty(), rr.Wall[c])
		}
	}
	if err := ship.addDeck(); err == nil || !strings.Contains(err.Error(), "Hull Lattice") {
		t.Errorf("the third deck without the lattice: %v", err)
	}
}

// A grey takes the player's ground click (Move Here: a task with no
// action), as a colonist does: a type's task list limits the work it takes
// on its own, never where the player sends it.
func TestGreysTakeMoveHere(t *testing.T) {
	_, wm := greyRun(t, 7, 0)
	ship := wm.shipLevel
	cx, cy, cz := shippkg.HullCenter()
	tx, ty := cx+5, cy-2
	tk := &task.Task{X: tx, Y: ty, Z: cz, Escalated: true}
	ship.MainSettlement.Tasks.AddTask(tk)
	for i := 0; i < 600 && !tk.Completed; i++ {
		wm.TickBackground(nil)
	}
	if !tk.Completed {
		t.Fatal("no grey took the Move Here order")
	}
	near := false
	for _, e := range wm.ShipColonists() {
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if absI(pc.GetX()-tx) <= 1 && absI(pc.GetY()-ty) <= 1 {
			near = true
		}
	}
	if !near {
		t.Error("no grey stands at the clicked tile")
	}
}

// A colonist with a task filter switched off still takes Move Here: its
// explicit list limits only the work it picks for itself.
func TestFilteredColonistTakesMoveHere(t *testing.T) {
	useContentTables(t)
	isolateSettlements(t)
	_, wm, err := newExpedition("Filters", 7, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range wm.ShipColonists() {
		e.GetComponent(components.Worker).(*components.WorkerComponent).AllowedTasks = []task.TaskAction{"mine"}
	}
	cx, cy, cz := shippkg.HullCenter()
	tk := &task.Task{X: cx + 2, Y: cy - 2, Z: cz, Escalated: true}
	wm.shipLevel.MainSettlement.Tasks.AddTask(tk)
	for i := 0; i < 600 && !tk.Completed; i++ {
		wm.TickBackground(nil)
	}
	if !tk.Completed {
		t.Fatal("no filtered colonist took the Move Here order")
	}
}

// A grey player's research screen leaves out the entries that act only for
// a rival grey ship (interface.hidden_research: Tractor Lock, Cloak,
// Mothership Bulwark, Armory Module), even once their prerequisites are
// known; the shared tree a rival walks keeps them.
func TestGreyRunHidesRivalOnlyResearch(t *testing.T) {
	_, wm := greyRun(t, 7, 0)
	ms := wm.shipLevel
	ui := ms.doctrineSettlement().UI()
	hidden := []string{"grey_tractor_lock", "grey_cloak", "grey_mothership_bulwark", "grey_armory_module"}
	for _, k := range hidden {
		if !ui.ResearchHidden(k) || !ms.doctrineSettlement().CanResearch(k) {
			t.Errorf("%s: hidden %v, in the shared tree %v", k, ui.ResearchHidden(k), ms.doctrineSettlement().CanResearch(k))
		}
	}
	// The hidden entries' prerequisites known (Tractor Lock's too, so Cloak
	// would be available).
	for _, k := range []string{"grey_beam_weapons", "grey_field_emitter", "grey_tractor_lock"} {
		ms.unlockTech(k)
	}
	available, locked, _, _, _ := ms.researchSnapshot()
	shown := 0
	for _, list := range [][]research.Tech{available, locked} {
		for _, tech := range list {
			shown++
			if contains(hidden, tech.Key) {
				t.Errorf("%s (%s) is on the research screen", tech.Name, tech.Key)
			}
		}
	}
	if shown == 0 {
		t.Fatal("fixture: an empty research screen")
	}
}
