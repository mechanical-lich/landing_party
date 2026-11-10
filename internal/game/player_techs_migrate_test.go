package game

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
)

// A campaign saved at version 3 (the player's techs on the campaign's
// known_techs) loads through LoadCampaign with its techs intact, now on the
// player's presence (phase C0).
func TestLoadCampaignV3KeepsTechs(t *testing.T) {
	t.Chdir(t.TempDir())
	techs := []string{"basic_metallurgy", "robotics", "ship_scanner_1"}
	v3 := map[string]any{
		"version":             3,
		"name":                "OldRun",
		"seed":                7,
		"current_location_id": "planet",
		"locations":           map[string]any{"planet": map[string]any{"id": "planet", "name": "Planet"}},
		"known_techs":         techs,
	}
	root := campaignRoot("OldRun")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeGz(filepath.Join(root, "campaign.json.gz"), v3); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCampaign("OldRun")
	if err != nil {
		t.Fatal(err)
	}
	if c.Version != campaign.CampaignVersion || c.Player == nil || c.Player.Doctrine != campaign.DefaultPlayerDoctrine {
		t.Fatalf("version %d player %+v", c.Version, c.Player)
	}
	if !reflect.DeepEqual(c.KnownTechs(), techs) || !c.HasTech("robotics") || c.ScannerLevel() != 1 {
		t.Errorf("techs after load: %v", c.KnownTechs())
	}
	// Saved and loaded again at version 4, nothing is lost.
	if err := writeGz(filepath.Join(root, "campaign.json.gz"), c); err != nil {
		t.Fatal(err)
	}
	back, err := LoadCampaign("OldRun")
	if err != nil || !reflect.DeepEqual(back.KnownTechs(), techs) {
		t.Errorf("v4 reload: %v %v", err, back.KnownTechs())
	}
}
