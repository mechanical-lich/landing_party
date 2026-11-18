package campaign

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Every save from before the pregenerated galaxy is refused: no migration.
func TestOlderSavesRefused(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 1)
	if err := MigrateCampaign(c); err != nil {
		t.Fatalf("a current save is refused: %v", err)
	}
	for v := 0; v < CampaignVersion; v++ {
		old := *c
		old.Version = v
		if err := MigrateCampaign(&old); err == nil {
			t.Errorf("a version %d save loads", v)
		}
	}
	b, _ := json.Marshal(c)
	var back Campaign
	if err := json.Unmarshal(b, &back); err != nil || MigrateCampaign(&back) != nil || back.Galaxy == nil || back.StartID != c.StartID {
		t.Fatalf("round trip: %v, galaxy %v", err, back.Galaxy)
	}
}

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
	// Phase F: the hive is playable, a spreading race (no ship); the
	// colony stays a ship run and the default.
	if !PlayerDoctrine(h).Playable || !h.SpreadRun() || c.SpreadRun() {
		t.Errorf("hive playable %v, hive run spreads %v, colony run spreads %v", PlayerDoctrine(h).Playable, h.SpreadRun(), c.SpreadRun())
	}
	if got := PlayableDoctrines(); len(got) < 2 || got[0] != DefaultPlayerDoctrine {
		t.Errorf("playable doctrines %v, want the colony first", got)
	}
	// The colony is never seeded as a rival.
	for _, k := range LoadFactionData().seedable() {
		if k == DefaultPlayerDoctrine {
			t.Error("colony is seedable")
		}
	}
}
