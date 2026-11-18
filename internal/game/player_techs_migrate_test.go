package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A save from before the pregenerated galaxy (here version 4) is refused by
// LoadCampaign with a plain message: no migration.
func TestLoadCampaignOlderVersionRefused(t *testing.T) {
	t.Chdir(t.TempDir())
	v4 := map[string]any{
		"version":             4,
		"name":                "OldRun",
		"seed":                7,
		"current_location_id": "planet",
		"locations":           map[string]any{"planet": map[string]any{"id": "planet", "name": "Planet"}},
	}
	root := campaignRoot("OldRun")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeGz(filepath.Join(root, "campaign.json.gz"), v4); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCampaign("OldRun")
	if err == nil || !strings.Contains(err.Error(), "older version") {
		t.Fatalf("a version 4 save: %v", err)
	}
}
