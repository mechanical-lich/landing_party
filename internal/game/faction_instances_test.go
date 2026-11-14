package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// Two hives on one world (rival_selection_plan.md, step 1): each presence's
// members carry their own faction key, never the blueprint's, so the second
// hive's bodies are its own side; a settled body's young take their
// settlement's faction too.
func TestTwoHivesOnOneLevel(t *testing.T) {
	ms, loc := hiveTestSite(t, 7, 12, 1, 0)
	loc.Factions = append(loc.Factions, campaign.FactionPresence{
		ID: "fp2", Faction: "hive.2", Doctrine: "hive", Population: 12, Stage: 1,
		Seed: 7*977 + 101, LastShotDay: campaign.NeverFired,
	})
	c := &campaign.Campaign{Name: "Two Hives", Player: campaign.NewPlayerPresence("")}
	c.AddFaction("hive", 1, 0)
	c.AddFaction("hive", 2, 0)
	setFactionDoctrines(c)
	t.Cleanup(func() { setFactionDoctrines(nil) })
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	for _, id := range []string{"fp1", "fp2"} {
		want := map[string]string{"fp1": "hive", "fp2": "hive.2"}[id]
		members := presenceMembers(ms.level, id)
		if len(members) == 0 {
			t.Fatalf("%s built nothing", id)
		}
		for _, e := range members {
			if !e.HasComponent(rlcomponents.Description) {
				continue
			}
			if f := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction; f != want {
				t.Errorf("%s's %s is faction %q, want %q", id, e.Blueprint, f, want)
			}
		}
	}
	egg, err := factory.Create("xeno_egg", 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	child, err := factory.Create("xeno_hatchling", 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	child.AddComponent(&components.KinComponent{Settlement: "hive.2:" + loc.ID})
	prepareSettledKin(egg, child, "hatchling")
	if f := child.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction; f != "hive.2" {
		t.Errorf("the second hive's hatchling is faction %q", f)
	}
}

// A rival of the player's group is never the default target (rival
// selection, "A rival in the player's group"): a plain click, the hover
// hint, a rogue bump and the map's red dot pass it by, while the explicit
// Attack order may still target it; a rival of another group is an enemy as
// before.
func TestFriendlyRivalIsNoDefaultTarget(t *testing.T) {
	c := &campaign.Campaign{Name: "Friends", Player: campaign.NewPlayerPresence("")}
	c.EnsureFactionTable()
	c.AddFaction("hive", 1, campaign.PlayerGroup)
	c.AddFaction("hive", 2, 0)
	setFactionRelation(c)
	t.Cleanup(func() { setFactionRelation(nil) })
	body := func(f string) *ecs.Entity {
		e := &ecs.Entity{}
		e.AddComponent(&rlcomponents.HealthComponent{MaxHealth: 10, Health: 10})
		e.AddComponent(&rlcomponents.DescriptionComponent{Faction: f})
		e.AddComponent(&components.ScriptedAIComponent{})
		return e
	}
	friend, foe := body("hive"), body("hive.2")
	if components.IsDefaultAttackTarget(friend) || !components.IsAttackTarget(friend) {
		t.Errorf("the friendly hive: default %v, explicit %v", components.IsDefaultAttackTarget(friend), components.IsAttackTarget(friend))
	}
	if !components.IsDefaultAttackTarget(foe) {
		t.Error("the hostile hive is no default target")
	}
	setFactionRelation(nil)
	if !components.IsDefaultAttackTarget(friend) {
		t.Error("with no campaign every attack target is a default one")
	}
}
