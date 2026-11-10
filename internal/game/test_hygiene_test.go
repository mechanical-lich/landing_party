package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/research"
)

// Every game test runs on the real content tables, without asking for
// them (review note: they were empty unless a test loaded them, so what a
// test saw depended on which tests ran before it).
func TestContentTablesLoadedForEveryTest(t *testing.T) {
	if construction.GetBuildable("hull_wall").Name == "" {
		t.Error("build.json not loaded")
	}
	if _, ok := research.GetTech("combat_armor"); !ok {
		t.Error("research.json not loaded")
	}
	if _, ok := crafting.GetRecipe("marine_head"); !ok {
		t.Error("crafting_recipes.json not loaded")
	}
}

// A new campaign forgets the last one's presences: its IDs (fp1, ...)
// start again, and fp1's techs and settlement were the old campaign's
// (review note: presenceTechs and presenceSettlement were never cleared).
func TestNewCampaignForgetsPresenceTechs(t *testing.T) {
	isolateSettlements(t)
	t.Chdir(t.TempDir())
	notePresenceTechs("fp1", []string{"hive_warrior_brood"})
	presenceSettlement["fp1"] = "hive:old"
	NewWorldManager(&campaign.Campaign{Version: campaign.CampaignVersion, Name: "Next",
		Locations: map[string]*campaign.Location{}})
	if len(techsOf("fp1")) != 0 || presenceSettlement["fp1"] != "" {
		t.Errorf("the last campaign's fp1 still knows %v (settlement %q)", techsOf("fp1"), presenceSettlement["fp1"])
	}
}
