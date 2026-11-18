package game

import (
	"math"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/history"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

// interceptTrip: a fresh expedition in orbit of its start system and a
// greyish ship of tier flying across the line to another site, timed to
// meet the jump halfway (it crosses the jump's midpoint, perpendicular,
// as the player passes). It returns the destination.
func interceptTrip(t *testing.T, seed int64, tier int) (*campaign.Campaign, *WorldManager, *campaign.Location) {
	t.Helper()
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Contact", seed, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := wm.Travel(c.StartID); err != nil {
		t.Fatal(err)
	}
	from := c.Locations[c.StartID]
	var to *campaign.Location
	for _, l := range c.Locations {
		if l.ID != from.ID && c.FactionSiteOK(l, false) && c.FuelCost(l.ID) >= 6 && (to == nil || l.ID < to.ID) {
			to = l
		}
	}
	days := campaign.JumpDays(c.FuelCost(to.ID))
	mx, my := (from.X+to.X)/2, (from.Y+to.Y)/2
	// Perpendicular to the jump through its midpoint.
	dx, dy := to.X-from.X, to.Y-from.Y
	l := math.Hypot(dx, dy)
	px, py := -dy/l*30, dx/l*30
	day := c.StarDate
	c.FleetSeq++
	c.FactionSeq++
	c.Fleets = append(c.Fleets, campaign.Fleet{
		ID: "fl777", Kind: campaign.FleetShip, ShipTier: tier,
		FromID: "", ToID: to.ID, FromX: mx - px, FromY: my - py, ToX: mx + px, ToY: my + py,
		LaunchDay: day, ArriveDay: day + history.StarDate(days),
		Presence: campaign.FactionPresence{ID: "fp777", Faction: "greyish", Doctrine: "greyish", Population: 3,
			Stockpile: map[string]int{"metal_ore": 25, "crystal": 6}, Seed: seed*101 + 7, LastShotDay: campaign.NeverFired},
	})
	return c, wm, to
}

// A jump across a greyish ship's path meets it: on arrival the encounter
// waits for an answer and the clock stands still. Evading takes the tier's
// fuel from the hold.
func TestInterceptionOnTravelAndEvade(t *testing.T) {
	c, wm, to := interceptTrip(t, 11, 0)
	if err := wm.Travel(to.ID); err != nil {
		t.Fatal(err)
	}
	e := c.Encounter
	if !e.Pending() || e.FleetID != "fl777" {
		t.Fatalf("no encounter on the jump: %+v", e)
	}
	if !wm.ClockStopped() || !wm.PopupUp() {
		t.Error("the clock runs under the encounter")
	}
	hold := storage.ShipProvider{Level: wm.ShipLevel()}
	owners := []string{wm.shipSettlementName()}
	before := storage.CountResource(hold, owners, "fuel")
	if _, err := wm.AnswerEncounter(campaign.AnswerEvade); err != nil {
		t.Fatal(err)
	}
	if c.Encounter.Pending() {
		t.Fatal("still pending after the answer")
	}
	paid := before - storage.CountResource(hold, owners, "fuel")
	if paid != min(before, c.Encounter.FuelCost) || c.Encounter.FuelCost != c.Encounter.EvadeFuel() {
		t.Errorf("evading paid %d fuel (had %d), cost %d", paid, before, c.Encounter.FuelCost)
	}
	t.Logf("evade: %s at %d fuel", c.Encounter.Outcome, paid)
}

// Boarding takes the player to the ship's deck: a docking deck in open
// space with the ship's hull stamped beside it, its crew (fighting greys of
// the fleet) aboard and its hold in a locker. Killing the crew takes the
// ship: the fleet is gone.
func TestInterceptionBoardingLevel(t *testing.T) {
	c, wm, to := interceptTrip(t, 12, 0)
	if err := wm.Travel(to.ID); err != nil {
		t.Fatal(err)
	}
	ms, err := wm.AnswerEncounter(campaign.AnswerBoard)
	if err != nil || ms == nil {
		t.Fatalf("board: %v", err)
	}
	loc := c.Locations[c.CurrentLocationID]
	if loc.Kind != campaign.EncounterKind || loc.Boarding == nil || !loc.Boarding.Built {
		t.Fatalf("not aboard an encounter: %+v", loc)
	}
	crew := presenceBodies(ms.level, "fp777")
	if len(crew) != 3 {
		t.Fatalf("%d crew aboard, want 3", len(crew))
	}
	for _, e := range crew {
		if !components.IsAttackTarget(e) {
			t.Errorf("crew %s is not a target", e.Blueprint)
		}
	}
	loot := 0
	for _, e := range ms.level.Entities {
		if e != nil && e.HasComponent(components.Storage) {
			for _, it := range e.GetComponent(components.Storage).(*components.StorageComponent).Items {
				if it.Blueprint == "metal_ore" {
					loot += it.GetComponent(components.Material).(*components.MaterialComponent).Quantity
				}
			}
		}
	}
	if loot < 25 {
		t.Errorf("the hold aboard holds %d metal, want the ship's 25", loot)
	}
	hull := 0
	for y := 0; y < ms.level.GetHeight(); y++ {
		for x := 0; x < ms.level.GetWidth(); x++ {
			if tileMiddleIs(ms, x, y, ms.level.SurfaceZ, "hull_wall") {
				hull++
			}
		}
	}
	if hull < 30 {
		t.Errorf("%d hull wall tiles: no ship stamped", hull)
	}
	party := wm.ShipColonists()[0]
	if err := wm.BeamDown(party); err != nil {
		t.Fatal(err)
	}
	if pc, ok := entityPos(party); !ok || world.IsSpaceTile(ms.level.GetTilePtr(pc.GetX(), pc.GetY(), pc.GetZ())) ||
		ms.level.GetTilePtr(pc.GetX(), pc.GetY(), pc.GetZ()).Floor.IsEmpty() {
		t.Error("the boarding party did not land on the deck")
	}
	for _, e := range crew {
		e.AddComponent(&rlcomponents.DeadComponent{})
	}
	wm.stepBoardings()
	if c.FleetByPresence("fp777") != nil {
		t.Error("the ship survived its crew")
	}
	if eventsOf(c, "faction_ship_taken") != 1 {
		t.Error("no faction_ship_taken event")
	}
}

// A boarded mirror ship's crew (colonists, whose own AI is the worker's)
// fight back: each gets the humanoid fighting script under the mirror's
// faction, hostile, and goes for the boarding party (phase E, D's gap).
func TestBoardedMirrorCrewFightBack(t *testing.T) {
	c, wm, to := interceptTrip(t, 12, 0)
	f := c.FleetByPresence("fp777")
	f.Presence.Faction, f.Presence.Doctrine, f.Presence.Population = "mirror", campaign.RivalColonyDoctrine, 3
	if err := wm.Travel(to.ID); err != nil {
		t.Fatal(err)
	}
	ms, err := wm.AnswerEncounter(campaign.AnswerBoard)
	if err != nil || ms == nil {
		t.Fatalf("board: %v", err)
	}
	crew := presenceBodies(ms.level, "fp777")
	if len(crew) != 3 {
		t.Fatalf("%d crew aboard, want 3", len(crew))
	}
	for _, e := range crew {
		if !e.HasComponent(components.ScriptedAI) {
			t.Fatalf("boarder %s has no AI", e.Blueprint)
		}
		if got := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction; got != "mirror" {
			t.Errorf("boarder faction %q", got)
		}
		if h := e.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent).Vars["hostility"]; h == nil || h.(float64) < 5 {
			t.Errorf("boarder hostility %v", h)
		}
	}
	party := wm.ShipColonists()[0]
	if err := wm.BeamDown(party); err != nil {
		t.Fatal(err)
	}
	// The party steps aboard beside one of them (the crew defend their ship:
	// they hunt within the script's range, 14, and sight, not across the
	// deck and through the hull).
	if bp, ok := entityPos(crew[0]); ok {
		x, y, z := freeLandingTile(ms.level, bp.GetX()+1, bp.GetY(), bp.GetZ())
		ms.level.PlaceEntity(x, y, z, party)
	}
	hp := party.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
	start := hp.Health
	for tick := 0; tick < 3000 && hp.Health == start && !party.HasComponent(rlcomponents.Dead); tick++ {
		fspath.ResetFrameCounter()
		ms.stepWorld()
		ms.advanceClock()
	}
	if hp.Health == start && !party.HasComponent(rlcomponents.Dead) {
		pp, _ := entityPos(party)
		for _, e := range crew {
			ep, _ := entityPos(e)
			t.Logf("boarder at %d,%d,%d party %d,%d,%d vars %v faction %q party %q bag %d", ep.GetX(), ep.GetY(), ep.GetZ(), pp.GetX(), pp.GetY(), pp.GetZ(),
				e.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent).Vars,
				e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction,
				party.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction,
				len(e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag))
		}
		t.Error("the mirror's boarded crew never hurt the boarding party in 3,000 ticks")
	}
}

// A boarded ship the player leaves without taking flies on with its deck:
// the encounter location goes (secondary finding: every boarding added one
// that was never removed).
func TestReleasedBoardingDeckGoes(t *testing.T) {
	c, wm, to := interceptTrip(t, 12, 0)
	if err := wm.Travel(to.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := wm.AnswerEncounter(campaign.AnswerBoard); err != nil {
		t.Fatalf("board: %v", err)
	}
	enc := c.CurrentLocationID
	if loc := c.Locations[enc]; loc == nil || loc.Kind != campaign.EncounterKind {
		t.Fatalf("not aboard an encounter: %q", enc)
	}
	if err := wm.Travel(to.ID); err != nil {
		t.Fatal(err)
	}
	if ms := wm.loaded[enc]; ms != nil { // the viewer left it: it freezes
		if err := wm.Freeze(c.Locations[enc], ms); err != nil {
			t.Fatal(err)
		}
	}
	if c.Locations[enc] != nil {
		t.Errorf("the released deck %s is still a location", enc)
	}
	if f := c.FleetByPresence("fp777"); f == nil || f.BoardedAt != "" {
		t.Errorf("the ship did not go on: %+v", f)
	}
}
