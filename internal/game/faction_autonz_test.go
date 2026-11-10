package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/storage"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// autonzTestSite is a charted moon site holding one autonz presence.
func autonzTestSite(t *testing.T, scenarioID, chapter string, seed int64, p campaign.FactionPresence) (*MainState, *campaign.Location, *campaign.Campaign) {
	t.Helper()
	isolateSettlements(t)
	ms, _, _, _ := loreBuild(t, loreSite{Map: "moon", Scenario: scenarioID, Seed: seed, Chapter: chapter})
	loc := &campaign.Location{ID: "site", Name: "Wire World", ScenarioID: scenarioID, Seed: seed, Tags: []string{"ore"}}
	p.Faction, p.Doctrine = "autonz", "autonz"
	if p.ID == "" {
		p.ID = "fpa"
	}
	if p.Seed == 0 {
		p.Seed = seed*977 + 13
	}
	p.LastShotDay = campaign.NeverFired
	loc.Factions = []campaign.FactionPresence{p}
	return ms, loc, factionTestCampaign(loc)
}

func autonzDoc(t *testing.T) *campaign.Doctrine {
	t.Helper()
	d := campaign.LoadFactionData().Doctrine("autonz")
	if d == nil {
		t.Fatal("no autonz doctrine")
	}
	return d
}

// Materialise lays the node stamp, the settlement's locker and fabricator,
// crew by the stage's mix, and the conversion share as blue wire around the
// centre; reconcile reads the share back, and wire laid on the level raises
// it.
func TestAutonzMaterialiseLaysConversionByShare(t *testing.T) {
	ms, loc, c := autonzTestSite(t, "moon", "", 5, campaign.FactionPresence{Population: 4, Conversion: 0.3,
		Stockpile: map[string]int{"metal_ore": 12}})
	d := autonzDoc(t)
	materialiseFactions(c, loc, ms.level, ms.scenario)
	p := loc.PresenceByID("fpa")
	if !p.Materialized || p.LevelStage != 0 {
		t.Fatalf("not built: %+v", p)
	}
	name := factionSettlementName(p, loc.ID)
	if settlement.Settlements[name] == nil {
		t.Fatalf("no settlement %s", name)
	}
	cx, cy, cz, ok := generation.Origin(ms.level, factionOriginKey("fpa"))
	if !ok {
		t.Fatal("no centre")
	}
	workers := settlementWorkers(ms.level, name)
	kinds := map[string]int{}
	for _, e := range workers {
		kinds[e.Blueprint]++
		if components.IsColonyWorker(e, ms.colonyName()) {
			t.Error("an autonz worker counts as the player's")
		}
	}
	if len(workers) != 4 || kinds["wire_placer_crew"] != 3 || kinds["enforcer_crew"] != 1 {
		t.Errorf("crew %v, want 3 wire placers and an enforcer (node mix)", kinds)
	}
	fab := 0
	for _, list := range [][]*ecs.Entity{ms.level.Entities, ms.level.StaticEntities} {
		for _, e := range list {
			if e != nil && e.Blueprint == "autonz_fabricator" {
				fab++
			}
		}
	}
	if fab != 1 {
		t.Errorf("%d fabricators", fab)
	}
	if got := storage.Summarize(storage.LevelProvider{Level: ms.level}, []string{name})["metal_ore"]; got != 12 {
		t.Errorf("stockpile in the locker: %d metal", got)
	}
	r := d.Conversion.Radius
	share := generation.ConversionShare(ms.level, cx, cy, cz, r, d.Conversion.Tile)
	if share < 0.28 || share > 0.45 {
		t.Errorf("converted share %.3f after materialising 0.3", share)
	}
	reconcileFactions(c, loc, ms.level)
	if p = loc.PresenceByID("fpa"); p.Conversion < 0.28 || p.Population != 4 {
		t.Fatalf("reconciled %+v", p)
	}
	before := p.Conversion
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r && floorAt(ms, cx+dx, cy+dy, cz) != "" {
				ms.level.SetFloor(cx+dx, cy+dy, cz, d.Conversion.Tile, 0)
			}
		}
	}
	reconcileFactions(c, loc, ms.level)
	if p = loc.PresenceByID("fpa"); p.Conversion <= before || p.Conversion < 0.95 {
		t.Errorf("after wiring the patch: conversion %.3f (was %.3f)", p.Conversion, before)
	}
	// Idempotent: a second build adds no one.
	materialiseFactions(c, loc, ms.level, ms.scenario)
	if n := len(settlementWorkers(ms.level, name)); n != 4 {
		t.Errorf("a second build: %d workers", n)
	}
}

func floorAt(ms *MainState, x, y, z int) string {
	t := ms.level.GetTilePtr(x, y, z)
	if t == nil || t.Floor.IsEmpty() {
		return ""
	}
	return world.TileDefinitions[t.Floor.Type].Name
}

// Reaching the hub stamps it over the node with the hold lifted out and put
// back, and the crew follows the hub's mix.
func TestAutonzHubStampsOverTheNode(t *testing.T) {
	ms, loc, c := autonzTestSite(t, "moon", "", 7, campaign.FactionPresence{Population: 4,
		Stockpile: map[string]int{"metal_ore": 20}})
	materialiseFactions(c, loc, ms.level, ms.scenario)
	p := loc.PresenceByID("fpa")
	name := factionSettlementName(p, loc.ID)
	p.Stage, p.Population = 1, 8
	materialiseFactions(c, loc, ms.level, ms.scenario)
	if p.LevelStage != 1 {
		t.Fatalf("level stage %d after the hub", p.LevelStage)
	}
	hub := false
	for _, rm := range ms.level.Rooms {
		hub = hub || rm.Name == "Hub"
	}
	if !hub {
		t.Error("no Hub room stamped")
	}
	if got := storage.Summarize(storage.LevelProvider{Level: ms.level}, []string{name})["metal_ore"]; got != 20 {
		t.Errorf("hold after the re-stamp: %d metal, want 20", got)
	}
	if n := len(settlementWorkers(ms.level, name)); n != 8 {
		t.Errorf("%d workers at the hub, want 8", n)
	}
}

// A dormant presence is one sleeper node and nothing else: no camp, no
// settlement, no workers, no wire. Reconcile keeps it while the node
// stands; the wake day (live) builds the node camp and its crew; a node
// destroyed before then wipes it.
func TestAutonzDormantSleeperAndWake(t *testing.T) {
	ms, loc, c := autonzTestSite(t, "moon", "", 9, campaign.FactionPresence{Population: 1, Dormant: true, WakeDay: 30})
	d := autonzDoc(t)
	materialiseFactions(c, loc, ms.level, ms.scenario)
	p := loc.PresenceByID("fpa")
	name := factionSettlementName(p, loc.ID)
	if sleeperOf(ms.level, d, "fpa") == nil {
		t.Fatal("no sleeper node")
	}
	if p.Materialized || settlement.Settlements[name] != nil || len(settlementWorkers(ms.level, name)) != 0 {
		t.Fatal("a dormant presence built a settlement")
	}
	materialiseFactions(c, loc, ms.level, ms.scenario)
	if n := len(presenceMembers(ms.level, "fpa")); n != 1 {
		t.Errorf("%d members after a second build, want the one node", n)
	}
	reconcileFactions(c, loc, ms.level)
	if loc.PresenceByID("fpa") == nil {
		t.Fatal("reconcile dropped a sleeping presence")
	}
	// Not yet its day.
	c.FactionDay = 29
	if a := stepLiveAutonz(c, loc, ms.level, nil); len(a) != 0 || !loc.PresenceByID("fpa").Dormant {
		t.Fatal("woke early")
	}
	c.FactionDay = 30
	alerts := stepLiveAutonz(c, loc, ms.level, nil)
	p = loc.PresenceByID("fpa")
	if len(alerts) != 1 || p.Dormant || !p.Materialized {
		t.Fatalf("on its day: alerts %v, %+v", alerts, p)
	}
	if sleeperOf(ms.level, d, "fpa") != nil || len(settlementWorkers(ms.level, name)) != 1 {
		t.Error("woken: the node should be gone and the crew of one there")
	}

	// A second sleeper, destroyed.
	loc.Factions = append(loc.Factions, campaign.FactionPresence{ID: "fpb", Faction: "autonz", Doctrine: "autonz",
		Population: 1, Dormant: true, WakeDay: 90, Seed: 4242, LastShotDay: campaign.NeverFired})
	materialiseFactions(c, loc, ms.level, ms.scenario)
	node := sleeperOf(ms.level, d, "fpb")
	if node == nil {
		t.Fatal("no second sleeper")
	}
	node.AddComponent(&rlcomponents.DeadComponent{})
	reconcileFactions(c, loc, ms.level)
	if loc.PresenceByID("fpb") != nil {
		t.Error("a destroyed sleeper's presence survived")
	}
}

// On an autonz_incursion world the presence builds by the chapter's origin
// (one organism with the scenario's swarm: the same faction).
func TestAutonzIncursionWorldJoinsTheChapter(t *testing.T) {
	ms, ox, oy, _ := loreBuild(t, loreSite{Map: "moon", Scenario: "autonz_incursion", Seed: 61, Chapter: "swarm_landing"})
	isolateSettlements(t)
	loc := &campaign.Location{ID: "site", Name: "Wired", ScenarioID: "autonz_incursion", Seed: 61}
	loc.Factions = []campaign.FactionPresence{{ID: "fpa", Faction: "autonz", Doctrine: "autonz", Population: 3, Seed: 77,
		LastShotDay: campaign.NeverFired}}
	materialiseFactions(factionTestCampaign(loc), loc, ms.level, ms.scenario)
	x, y, _, ok := generation.Origin(ms.level, factionOriginKey("fpa"))
	if !ok {
		t.Fatal("no centre")
	}
	if d := max(absI(x-ox), absI(y-oy)); d > presenceSiteRadius+12 {
		t.Errorf("centre %d tiles from the swarm's origin", d)
	}
	placers := 0
	for _, e := range ms.level.Entities {
		if e != nil && e.Blueprint == "wire_placer" {
			placers++
			if f := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction; f != "autonz" {
				t.Errorf("scenario wire placer faction %q", f)
			}
		}
	}
	if placers == 0 {
		t.Error("the scenario's own wire placers are gone")
	}
}

// The Star Map's Destinations note names a known autonz presence by stage
// and a known dormant one as a signal; unseen ones are not drawn.
func TestAutonzStarMapNotes(t *testing.T) {
	loc := &campaign.Location{ID: "site", Name: "Wire World"}
	loc.Factions = []campaign.FactionPresence{
		{ID: "a", Faction: "autonz", Doctrine: "autonz", Population: 8, Stage: 1, Known: true},
		{ID: "b", Faction: "autonz", Doctrine: "autonz", Population: 1, Dormant: true, WakeDay: 9, Known: true},
		{ID: "c", Faction: "autonz", Doctrine: "autonz", Population: 1, Dormant: true, WakeDay: 9},
	}
	c := factionTestCampaign(loc)
	if got := factionListTag(c, loc); got != "  [autonz hub]  [autonz signal]" {
		t.Errorf("Destinations note %q", got)
	}
}
