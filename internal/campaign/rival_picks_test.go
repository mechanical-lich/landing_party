package campaign

import (
	"fmt"
	"testing"
)

// Each rival box is a faction of its own (step 3): two hives picked are two
// factions, here sharing a group; a Random box rolls a doctrine; each lands
// on a site of its own; the picks are recorded.
func TestRivalPicksSeedEachFaction(t *testing.T) {
	useRepoData(t)
	picks := []RivalPick{{Doctrine: "hive", Group: 2}, {Doctrine: "hive", Group: 2}, {}, {Doctrine: "colony!rival", Group: 5}}
	c, err := GenerateCampaignOpts("Picks", 9, GenOptions{Rivals: picks})
	if err != nil {
		t.Fatal(err)
	}
	if c.FactionCount != 4 || len(c.RivalPicks) != 4 {
		t.Fatalf("count %d, picks %v", c.FactionCount, c.RivalPicks)
	}
	if c.FactionOf("hive") == nil || c.FactionOf("hive.2") == nil || c.Hostile("hive", "hive.2") {
		t.Errorf("two hives in group 2: rows %v %v, hostile %v", c.FactionOf("hive"), c.FactionOf("hive.2"), c.Hostile("hive", "hive.2"))
	}
	if c.FactionOf("mirror") == nil || c.DoctrineOf("mirror") != RivalColonyDoctrine || c.GroupOf("mirror") != 5 {
		t.Errorf("the rival colony: %+v", c.FactionOf("mirror"))
	}
	sites := map[string]string{}
	place := func(f, loc string) {
		if other, ok := sites[loc]; ok {
			t.Errorf("%s and %s both on %s", f, other, loc)
		}
		sites[loc] = f
	}
	for _, loc := range c.Locations {
		for _, p := range loc.Factions {
			place(p.Faction, loc.ID)
		}
	}
	for _, f := range c.Fleets {
		if f.Landed() && !f.Mothership {
			place(f.Presence.Faction, f.LandedID)
		}
	}
	if len(sites) != 4 {
		t.Errorf("%d factions placed: %v", len(sites), sites)
	}
	slots := map[int]bool{}
	for _, f := range c.Factions() {
		if f.Slot > 0 {
			slots[f.Slot] = true
		}
	}
	if len(slots) != 4 {
		t.Errorf("slots %v", slots)
	}
}

// Random boxes roll from the campaign seed: the same seed rolls the same,
// and no doctrine repeats until every one has appeared.
func TestRivalRandomRoll(t *testing.T) {
	useRepoData(t)
	all := LoadFactionData().seedable()
	picks := make([]RivalPick, len(all))
	a, err := GenerateCampaignOpts("Roll", 4, GenOptions{Rivals: picks})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := GenerateCampaignOpts("Roll", 4, GenOptions{Rivals: picks})
	got, again := a.rollRivals(picks), b.rollRivals(picks)
	if fmt.Sprint(got) != fmt.Sprint(again) {
		t.Errorf("two rolls of one seed: %v / %v", got, again)
	}
	seen := map[string]bool{}
	for _, k := range got {
		if seen[k] {
			t.Errorf("%s rolled twice before every doctrine appeared: %v", k, got)
		}
		seen[k] = true
	}
}

// A hive run may face a rival hive: a faction of its own, hostile.
func TestRaceRunAgainstItsOwnRace(t *testing.T) {
	useRepoData(t)
	c, err := GenerateCampaignOpts("Hives", 6, GenOptions{Doctrine: "hive", Rivals: []RivalPick{{Doctrine: "hive"}}})
	if err != nil {
		t.Fatal(err)
	}
	if f := c.FactionOf("hive"); f == nil || f.Ordinal != 0 || f.Name != "" {
		t.Errorf("a hive player's one rival hive is numbered as a second: %+v", f)
	}
	if c.Player.Faction != PlayerFaction || c.FactionOf("hive") == nil || !c.Hostile(PlayerFaction, "hive") {
		t.Errorf("player %q, rival hive %v, hostile %v", c.Player.Faction, c.FactionOf("hive"), c.Hostile(PlayerFaction, "hive"))
	}
}

// A rival in the player's group is friendly: neutral, and never turns.
func TestRivalInThePlayersGroup(t *testing.T) {
	useRepoData(t)
	c, err := GenerateCampaignOpts("Friends", 6, GenOptions{PlayerGroup: 3, Rivals: []RivalPick{{Doctrine: "hive", Group: 3}, {Doctrine: "hive"}}})
	if err != nil {
		t.Fatal(err)
	}
	if c.GroupOf(c.Player.Faction) != 3 || c.Stance("hive") != StanceNeutral || c.SetStance("hive", StanceHostile, nil) {
		t.Errorf("the friendly hive: group %d, stance %q", c.GroupOf(c.Player.Faction), c.Stance("hive"))
	}
	if c.Stance("hive.2") != StanceHostile {
		t.Errorf("the other hive is %q", c.Stance("hive.2"))
	}
}
