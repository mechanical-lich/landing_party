package campaign

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// An exhausted presence leaves a husk for its level and a Legends event.
func TestFactionExhaustionLeavesAHusk(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"blight": "crystalline_blight"})
	p := c.newPresence("hive", "hive", hiveDoctrine(t), c.Locations["blight"], 3)
	id := p.ID
	c.StarDate = 60
	c.StepFactions(nil)
	loc := c.Locations["blight"]
	if len(loc.Factions) != 0 || len(loc.Husks) != 1 {
		t.Fatalf("factions %+v husks %+v", loc.Factions, loc.Husks)
	}
	if h := loc.Husks[0]; h.Presence != id || h.Doctrine != "hive" || h.Morph != "crawler" || h.Stamped {
		t.Errorf("husk = %+v", h)
	}
	if !hasEvent(c, "faction_exhausted", "blight") {
		t.Error("no faction_exhausted event")
	}
}

// Reconcile takes the level's count and raises the stage with it; it never
// lowers the stage; a presence not yet built is left alone; no bodies is a
// wipe, recorded and known.
func TestFactionReconcile(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"a": "earth_like"})
	loc := c.Locations["a"]
	p := c.newPresence("hive", "hive", hiveDoctrine(t), loc, 4)
	id := p.ID
	if c.ReconcilePresence(loc, id, 0, false) || loc.Presence("hive") == nil {
		t.Fatal("an unbuilt presence was reconciled")
	}
	p.Materialized = true
	c.ReconcilePresence(loc, id, 13, false)
	if p.Population != 13 || p.Stage != 2 {
		t.Errorf("after 13 bodies: pop %d stage %d", p.Population, p.Stage)
	}
	if p.LauncherLost {
		t.Error("a launcher the level never stood was counted lost")
	}
	p.LevelStage = 2
	// Risen to the launcher stage live: the stage's chamber stands, the
	// director has not built the spire yet. Not a loss (review note).
	c.ReconcilePresence(loc, id, 13, false)
	if p.LauncherLost {
		t.Error("a spire not built yet was counted lost")
	}
	c.ReconcilePresence(loc, id, 5, true)
	if p.Population != 5 || p.Stage != 2 || p.LauncherLost {
		t.Errorf("after 5 bodies: pop %d stage %d lost %v", p.Population, p.Stage, p.LauncherLost)
	}
	c.StarDate = 30
	c.ReconcilePresence(loc, id, 5, false)
	if !p.LauncherLost || p.LauncherLostDay != 30 {
		t.Errorf("launcher destroyed: lost %v day %d", p.LauncherLost, p.LauncherLostDay)
	}
	if !c.ReconcilePresence(loc, id, 0, false) || len(loc.Factions) != 0 {
		t.Fatalf("no bodies did not wipe: %+v", loc.Factions)
	}
	if !hasEvent(c, "faction_wiped", "a") {
		t.Error("no faction_wiped event")
	}
	for _, ev := range c.History.Events {
		if ev.Type == "faction_wiped" && !c.History.Known[ev.ID] {
			t.Error("the wipe the player saw is not known")
		}
	}
}

// A lost launcher stops the hive firing until it regrows after a cooldown.
func TestFactionLostLauncherRegrows(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"a": "earth_like"})
	c.Locations["b"] = &Location{ID: "b", Name: "b", Kind: "planet", ScenarioID: "earth_like", Tags: []string{"biomass"}, X: 110, Y: 1000, Discovered: true}
	d := hiveDoctrine(t)
	p := c.newPresence("hive", "hive", d, c.Locations["a"], 20)
	p.LauncherLost, p.LauncherLostDay = true, 0
	c.StarDate = history.StarDate(d.Mobility.Cooldown - 1)
	c.StepFactions(nil)
	if len(c.Fleets) != 0 {
		t.Fatalf("fired with its launcher down: %+v", c.Fleets)
	}
	c.StarDate += 2
	c.StepFactions(nil)
	if c.Locations["a"].Presence("hive").LauncherLost || len(c.Fleets) != 1 {
		t.Errorf("after the cooldown: lost %v, fleets %d", c.Locations["a"].Presence("hive").LauncherLost, len(c.Fleets))
	}
}
