package campaign

import (
	"encoding/json"
	"reflect"
	"testing"
)

// A version 3 save kept the player's techs on the campaign (known_techs). It
// migrates to version 4 with them on the player's presence, which runs the
// default doctrine; research checks read the same techs as before.
func TestMigrateKnownTechsFromV3(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 1)
	b, _ := json.Marshal(c)
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	raw["version"] = 3
	delete(raw, "player")
	techs := []string{"basic_metallurgy", "ship_scanner_1", "ship_scanner_2"}
	raw["known_techs"] = techs
	b, _ = json.Marshal(raw)
	var old Campaign
	if err := json.Unmarshal(b, &old); err != nil {
		t.Fatal(err)
	}
	if old.Player != nil || len(old.KnownTechs()) != 0 {
		t.Fatal("the v3 shape should load with no player presence")
	}
	if err := MigrateCampaign(&old); err != nil {
		t.Fatal(err)
	}
	if old.Version != CampaignVersion || CampaignVersion != 4 {
		t.Errorf("version %d, CampaignVersion %d", old.Version, CampaignVersion)
	}
	if old.Player == nil || old.Player.Doctrine != DefaultPlayerDoctrine || old.Player.ID != PlayerPresenceID {
		t.Fatalf("player presence %+v", old.Player)
	}
	if !reflect.DeepEqual(old.KnownTechs(), techs) || old.LegacyKnownTechs != nil {
		t.Errorf("techs %v (legacy %v), want %v", old.KnownTechs(), old.LegacyKnownTechs, techs)
	}
	if !old.HasTech("basic_metallurgy") || old.ScannerLevel() != 2 {
		t.Errorf("HasTech / ScannerLevel after migration: %v %d", old.HasTech("basic_metallurgy"), old.ScannerLevel())
	}
	// Saved again, the techs are under player and known_techs is gone.
	b, _ = json.Marshal(&old)
	var again map[string]any
	_ = json.Unmarshal(b, &again)
	if _, ok := again["known_techs"]; ok {
		t.Error("known_techs still written at version 4")
	}
	var back Campaign
	_ = json.Unmarshal(b, &back)
	if err := MigrateCampaign(&back); err != nil || !reflect.DeepEqual(back.KnownTechs(), techs) {
		t.Errorf("v4 round-trip: %v %v", err, back.KnownTechs())
	}
}

// A new campaign's player presence runs the chosen doctrine, the default
// when none is chosen, and is on no location: Tier 1 never steps it.
func TestPlayerPresenceOnNewCampaign(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 2)
	if c.Player == nil || c.Player.Doctrine != "colony" || len(c.KnownTechs()) != 0 {
		t.Fatalf("player %+v", c.Player)
	}
	for _, loc := range c.Locations {
		for _, p := range loc.Factions {
			if p.Faction == "colony" || p.ID == PlayerPresenceID {
				t.Errorf("player presence on %s", loc.ID)
			}
		}
	}
	c.UnlockTech("robotics")
	c.StarDate = 30
	c.StepFactions(nil)
	if !reflect.DeepEqual(c.KnownTechs(), []string{"robotics"}) || c.Player.Population != 0 {
		t.Errorf("player presence changed by the step: %+v", c.Player)
	}
	// A hand-built campaign has none until a tech is unlocked.
	var bare Campaign
	if bare.HasTech("robotics") {
		t.Error("bare campaign has a tech")
	}
	bare.UnlockTech("robotics")
	bare.UnlockTech("robotics")
	if !reflect.DeepEqual(bare.KnownTechs(), []string{"robotics"}) {
		t.Errorf("bare techs %v", bare.KnownTechs())
	}
}

// The player's doctrine is picked in one place: the campaign's player
// presence (GenOptions.Doctrine at generation), else the colony.
func TestPlayerDoctrineSelection(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 3)
	d := PlayerDoctrine(c)
	if d == nil || d.Settlement == nil || !d.Playable || d.Seedable || d.Name != "Colony" {
		t.Fatalf("colony doctrine %+v", d)
	}
	if PlayerDoctrine(nil) != d || PlayerDoctrine(&Campaign{}) != d {
		t.Error("no campaign or no presence is not the colony")
	}
	h, err := GenerateCampaignOpts("Test", 3, GenOptions{Doctrine: "hive"})
	if err != nil {
		t.Fatal(err)
	}
	if h.Player.Doctrine != "hive" || PlayerDoctrine(h) != LoadFactionData().Doctrine("hive") {
		t.Errorf("chosen doctrine: %+v", h.Player)
	}
	if PlayerDoctrine(h).Playable {
		t.Error("the hive is not playable yet (phase F); its settlement block runs its rival hives (phase E2)")
	}
	// The colony is never seeded as a rival.
	for _, k := range LoadFactionData().seedable() {
		if k == DefaultPlayerDoctrine {
			t.Error("colony is seedable")
		}
	}
}
