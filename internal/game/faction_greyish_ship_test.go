package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// greyLanded lands a grey ship of tier with techs on the start system of a
// fresh expedition, days ago, its units and gear as given (nil: from the
// stage mix and the floor), and enters the site with one colonist far off.
// The trip there takes days of its own (the ship stays down meanwhile, its
// hold kept small so it does not fill and leave).
func greyLanded(t *testing.T, seed int64, pop, tier int, techs []string, days int, units, gear map[string]int) (*WorldManager, *MainState, *campaign.Location, *campaign.Fleet) {
	t.Helper()
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("GreyShip", seed, false)
	if err != nil {
		t.Fatal(err)
	}
	syncFactionHostility(c)
	t.Cleanup(components.ResetFactionHostility)
	loc := c.Locations[c.StartID]
	c.FactionSeq++
	c.FleetSeq++
	c.Fleets = append(c.Fleets, campaign.Fleet{
		ID: "flg", Kind: campaign.FleetShip, FromID: loc.ID, ToID: loc.ID, ShipTier: tier,
		FromX: loc.X, FromY: loc.Y, ToX: loc.X, ToY: loc.Y, LandedID: loc.ID, LandedDay: c.StarDate - history.StarDate(days),
		Presence: campaign.FactionPresence{ID: "fpg", Faction: "greyish", Doctrine: "greyish", Population: pop,
			Units: units, Gear: gear, KnownTechs: techs,
			Stockpile: map[string]int{"metal_ore": 4, "crystal": 2, "grey_alloy": 2}, Seed: seed*17 + 3, LastShotDay: campaign.NeverFired},
	})
	c.FactionDay = c.StarDate
	if err := wm.Travel(loc.ID); err != nil {
		t.Fatal(err)
	}
	if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
		t.Fatal(err)
	}
	ms, err := wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	return wm, ms, loc, c.FleetByPresence("fpg")
}

// countBP is the standing entities of each blueprint on level.
func countBP(_ any, ms *MainState) map[string]int {
	out := map[string]int{}
	for _, list := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range list {
			if e != nil && !e.HasComponent(rlcomponents.Dead) {
				out[e.Blueprint]++
			}
		}
	}
	return out
}

// typesOf is the settlement's bodies by type.
func typesOf(ms *MainState, name string) map[string]int {
	out := map[string]int{}
	for _, e := range settlementWorkers(ms.level, name) {
		if k := components.KinOf(e); k != nil {
			out[k.Type]++
		}
	}
	return out
}

// A landed scout is its saucer with a vat and a console unpacked, three
// typed greys (a worker, a technician and a soldier), the soldier armed
// with a beam pistol from the gear floor; on the ground what ShipBuilt
// says for its days down (its armory from day 1).
// Materialise is idempotent.
func TestGreyScoutMaterialises(t *testing.T) {
	_, ms, loc, f := greyLanded(t, 1, 3, 0, nil, 0, nil, nil)
	name := factionSettlementName(&f.Presence, loc.ID)
	got := countBP(nil, ms)
	days := int(ms.campaign.FactionDay - f.LandedDay)
	wantArmory := greyishDoctrineT(t).ShipBuilt(0, nil, days)["grey_armory"]
	if got["grey_vat"] != 1 || got["grey_console"] != 1 || got["grey_armory"] != wantArmory || got["bio_printer"] != 0 {
		t.Errorf("scout %d days down: vat %d console %d armory %d (want %d) bio_printer %d", days,
			got["grey_vat"], got["grey_console"], got["grey_armory"], wantArmory, got["bio_printer"])
	}
	if ty := typesOf(ms, name); ty["grey_worker"] != 1 || ty["grey_technician"] != 1 || ty["grey_soldier"] != 1 {
		t.Errorf("scout crew %v", ty)
	}
	armed := false
	for _, e := range settlementWorkers(ms.level, name) {
		inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for _, it := range wornGear(inv) {
			if it.Blueprint == "grey_beam_pistol" {
				armed = true
			}
		}
	}
	if !armed {
		t.Error("the scout's soldier holds no beam pistol (the gear floor)")
	}
	d := greyishDoctrineT(t)
	before := countBP(nil, ms)
	n := len(settlementWorkers(ms.level, name))
	materialiseGreyish(ms.campaign, ms.level, loc, d, &f.Presence, true)
	after := countBP(nil, ms)
	for _, k := range []string{"grey_vat", "grey_console", "storage_locker", "grey_beam_pistol"} {
		if before[k] != after[k] {
			t.Errorf("a second materialise changed %s: %d -> %d", k, before[k], after[k])
		}
	}
	if len(settlementWorkers(ms.level, name)) != n || greyGearHeld(ms.level, d, name)["grey_beam_pistol"] != 1 {
		t.Error("a second materialise changed the crew or its gear")
	}
}

// A warship with the armory module, field emitter and beam weapons, landed
// three weeks with the array known: its modules (vat, console, armory, two
// emitters), its array built on the ground (ShipBuilt), a heavy among its
// crew once cloning is known, rifles for its soldiers. Reconcile reads the
// ground structures (the array; the carried armory and emitters are
// modules), units and gear back.
func TestGreyWarshipMaterialisesAndReconciles(t *testing.T) {
	techs := []string{"grey_hull_plating", "grey_beam_weapons", "grey_cloning", "grey_armory_module", "grey_field_emitter", "grey_research_array"}
	_, ms, loc, f := greyLanded(t, 2, 8, 2, techs, 21, nil, nil)
	name := factionSettlementName(&f.Presence, loc.ID)
	d := greyishDoctrineT(t)
	got := countBP(nil, ms)
	if got["grey_vat"] != 1 || got["grey_console"] != 1 || got["grey_armory"] != 1 || got["grey_emitter"] != 2 || got["grey_array"] != 1 {
		t.Errorf("warship structures: vat %d console %d armory %d emitter %d array %d",
			got["grey_vat"], got["grey_console"], got["grey_armory"], got["grey_emitter"], got["grey_array"])
	}
	ty := typesOf(ms, name)
	if ty["grey_heavy"] != 1 || len(settlementWorkers(ms.level, name)) != 8 {
		t.Errorf("warship crew %v", ty)
	}
	held := greyGearHeld(ms.level, d, name)
	if held["grey_beam_rifle"] != ty["grey_soldier"] || held["grey_beam_pistol"] != 0 {
		t.Errorf("gear %v for %d soldiers: rifles once beam weapons are known", held, ty["grey_soldier"])
	}
	reconcileGreyish(ms.campaign, ms.level, loc, d, &f.Presence)
	if f.Presence.Built["grey_array"] != 1 || f.Presence.Built["grey_armory"] != 0 || f.Presence.Built["grey_emitter"] != 0 {
		t.Errorf("reconciled ground structures %v: the array only (armory and emitters are carried)", f.Presence.Built)
	}
	if f.Presence.Units["grey_heavy"] != 1 || f.Presence.Gear["grey_beam_rifle"] != held["grey_beam_rifle"] {
		t.Errorf("reconciled units %v gear %v", f.Presence.Units, f.Presence.Gear)
	}
	// ShipBuilt, the abstract twin: three weeks at the warship tier.
	if b := d.ShipBuilt(2, techs, 21); b["grey_array"] != 1 || b["grey_armory"] != 0 || b["grey_emitter"] != 0 {
		t.Errorf("ShipBuilt %v", b)
	}
	// A freighter without the armory module builds its armory on the ground
	// on day 1; with only a day landed, nothing else.
	if b := d.ShipBuilt(1, []string{"grey_research_array"}, 1); b["grey_armory"] != 1 || b["grey_array"] != 0 {
		t.Errorf("freighter day 1 ShipBuilt %v", b)
	}
	if b := d.ShipBuilt(0, []string{"grey_research_array"}, 25); b["grey_array"] != 0 {
		t.Errorf("a scout built the array: %v (tier_min 1)", b)
	}
	if b := d.ShipBuilt(1, []string{"grey_research_array"}, 25); b["grey_array"] != 1 {
		t.Errorf("a freighter 25 days down has no array: %v", b)
	}
}

// A Phase D crew (untyped bodies) is adopted once by its type's print: the
// old green miner becomes a worker (open question 2), not a heavy.
func TestGreyPhaseDCrewAdopted(t *testing.T) {
	_, ms, loc, f := greyLanded(t, 3, 3, 0, nil, 0, nil, nil)
	name := factionSettlementName(&f.Presence, loc.ID)
	d := greyishDoctrineT(t)
	for _, e := range settlementWorkers(ms.level, name) {
		removeBody(ms.level, e)
	}
	for _, bp := range []string{"greenish_crew", "whitish_crew", "pinkish_crew"} {
		cx, cy, cz, _ := originOf(ms.level, f.Presence.ID)
		x, y, z := freeLandingTile(ms.level, cx, cy+3, cz)
		if _, err := spawnFactionWorker(ms.level, bp, name, "greyish", x, y, z); err != nil {
			t.Fatal(err)
		}
	}
	f.Presence.Units = nil
	materialiseGreyish(ms.campaign, ms.level, loc, d, &f.Presence, true)
	ty := typesOf(ms, name)
	if ty["grey_heavy"] != 0 || ty["grey_worker"] != 1 || ty["grey_technician"] != 1 || ty["grey_soldier"] != 1 {
		t.Errorf("adopted crew %v: the green miner should be a worker", ty)
	}
	materialiseGreyish(ms.campaign, ms.level, loc, d, &f.Presence, true)
	if again := typesOf(ms, name); len(settlementWorkers(ms.level, name)) != 3 || again["grey_worker"] != 1 {
		t.Errorf("a second materialise re-adopted: %v", again)
	}
}

// Lifting off live packs the modules (no vat, no console, no carried
// armory), leaves each ground structure as its wreck in place, takes the
// hull (a landing scar) and the cockpit locker; a ruin stamped unseen from
// the same record has the same wrecks.
func TestGreyLiftOffLeavesAScar(t *testing.T) {
	techs := []string{"grey_research_array"}
	wm, ms, loc, f := greyLanded(t, 4, 5, 1, techs, 20, nil, nil)
	d := greyishDoctrineT(t)
	got := countBP(nil, ms)
	if got["grey_armory"] != 1 || got["grey_array"] != 1 {
		t.Fatalf("a freighter 20 days down: armory %d array %d", got["grey_armory"], got["grey_array"])
	}
	cx, cy, cz, _ := originOf(ms.level, f.Presence.ID)
	f.LandedDay -= history.StarDate(d.Mobility.StayDays)
	stepLevel(ms, 2*750, nil)
	f = wm.Campaign.FleetByPresence("fpg")
	if f == nil || f.Landed() {
		t.Fatalf("the freighter did not lift off: %+v", f)
	}
	left := countBP(nil, ms)
	for _, k := range []string{"grey_vat", "grey_console", "grey_armory", "grey_array"} {
		if left[k] != 0 {
			t.Errorf("%d %s left standing", left[k], k)
		}
	}
	if left["grey_armory_wreck"] != 1 || left["grey_array_wreck"] != 1 {
		t.Errorf("wrecks: armory %d array %d", left["grey_armory_wreck"], left["grey_array_wreck"])
	}
	hull := 0
	for dy := -8; dy <= 8; dy++ {
		for dx := -8; dx <= 8; dx++ {
			if tileMiddleIs(ms, cx+dx, cy+dy, cz, "hull_wall") {
				hull++
			}
		}
	}
	if hull != 0 {
		t.Errorf("%d hull wall tiles left: the hull goes with the ship", hull)
	}
	if r := loc.Ruins; len(r) != 1 || r[0].Built["grey_array"] != 1 || r[0].Built["grey_armory"] != 1 {
		t.Errorf("ruin record %+v", r)
	}
	// The same ruin, never seen: its wrecks from the record.
	unseen := campaign.FactionRuin{Doctrine: "greyish", Tier: 1, Techs: techs, DaysLanded: 20}
	if b := d.RuinBuilt(&unseen); b["grey_array"] != 1 || b["grey_armory"] != 1 {
		t.Errorf("an unseen ruin's wrecks %v, want the live ones", b)
	}
}

// A boarded warship's deck holds its crew by type, armed from the gear
// floor, and the two emitters its hull carries.
func TestGreyBoardedWarship(t *testing.T) {
	c, wm, to := interceptTrip(t, 12, 2)
	f := c.FleetByPresence("fp777")
	f.Presence.Population = 8
	f.Presence.KnownTechs = []string{"grey_beam_weapons", "grey_field_emitter", "grey_cloning"}
	if err := wm.Travel(to.ID); err != nil {
		t.Fatal(err)
	}
	ms, err := wm.AnswerEncounter(campaign.AnswerBoard)
	if err != nil || ms == nil {
		t.Fatalf("board: %v", err)
	}
	crew := presenceBodies(ms.level, "fp777")
	types := map[string]int{}
	rifles := 0
	for _, e := range crew {
		if k := components.KinOf(e); k != nil {
			types[k.Type]++
		}
		if inv, ok := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent); ok && inv != nil {
			for _, it := range wornGear(inv) {
				if it.Blueprint == "grey_beam_rifle" {
					rifles++
				}
			}
		}
	}
	b := c.Locations[c.CurrentLocationID].Boarding
	if len(crew) != b.Crew || types["grey_soldier"] == 0 || rifles != types["grey_soldier"] {
		t.Errorf("aboard: %d crew (want %d), types %v, %d rifles", len(crew), b.Crew, types, rifles)
	}
	if n := countBP(nil, ms)["grey_emitter"]; n != 2 {
		t.Errorf("%d emitters aboard a warship with the field emitter, want 2", n)
	}
}
