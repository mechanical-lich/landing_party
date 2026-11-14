package campaign

import (
	"encoding/json"
	"testing"
)

// twoOfAKind is a colony campaign with no seeded rivals, for placing
// faction instances by hand.
func twoOfAKind(t *testing.T) *Campaign {
	t.Helper()
	useRepoData(t)
	c, err := GenerateCampaignOpts("Instances", 11, GenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c.EnsureFactionTable()
	return c
}

// rivalSites is n sites rivals may hold, in a fixed order.
func rivalSites(t *testing.T, c *Campaign, n int) []*Location {
	t.Helper()
	var out []*Location
	for _, loc := range c.sortedLocations() {
		if c.FactionSiteOK(loc, true) {
			out = append(out, loc)
		}
		if len(out) == n {
			return out
		}
	}
	t.Fatalf("only %d rival sites", len(out))
	return nil
}

// A doctrine picked twice is two factions: opaque keys (the doctrine's,
// then <doctrine>.2), each its own group by default, the second named and
// coloured apart; DoctrineOf answers for both, the player and a key with no
// row.
func TestFactionInstancesKeysAndGroups(t *testing.T) {
	c := twoOfAKind(t)
	a := c.AddFaction("hive", 1, 0)
	b := c.AddFaction("hive", 2, 0)
	if a.Key != "hive" || b.Key != "hive.2" {
		t.Fatalf("keys %q, %q", a.Key, b.Key)
	}
	if c.DoctrineOf("hive.2") != "hive" || c.DoctrineOf(c.Player.Faction) != "colony" || c.DoctrineOf("wildlife") != "wildlife" {
		t.Errorf("DoctrineOf: %q %q %q", c.DoctrineOf("hive.2"), c.DoctrineOf(c.Player.Faction), c.DoctrineOf("wildlife"))
	}
	if a.Group == b.Group || a.Group == PlayerGroup || !c.Hostile("hive", "hive.2") {
		t.Errorf("groups %d, %d: hostile %v", a.Group, b.Group, c.Hostile("hive", "hive.2"))
	}
	if a.Name != "" || b.Name == "" || b.Badge == [3]int{} || c.FactionSuffix("hive.2") != " 2" || c.FactionSuffix("hive") != "" {
		t.Errorf("look: %+v %+v, suffix %q", a, b, c.FactionSuffix("hive.2"))
	}
	g := c.AddFaction("greyish", 3, b.Group)
	if c.Hostile("greyish", "hive.2") || !c.Hostile("greyish", "hive") {
		t.Error("a grey faction in hive.2's group should be friendly to it and hostile to hive")
	}
	_ = g
}

// Stance toward the player is per faction: one rival colony turning hostile
// leaves the other neutral. A faction in the player's group is neutral and
// never turns.
func TestFactionStanceIsPerFaction(t *testing.T) {
	c := twoOfAKind(t)
	c.AddFaction(RivalColonyDoctrine, 1, 0)
	c.AddFaction(RivalColonyDoctrine, 2, 0)
	friend := c.AddFaction(RivalColonyDoctrine, 3, PlayerGroup)
	if c.Stance("mirror") != StanceNeutral || c.Stance("mirror.2") != StanceNeutral {
		t.Fatalf("starting stances %q %q", c.Stance("mirror"), c.Stance("mirror.2"))
	}
	c.SetStance("mirror.2", StanceHostile, nil)
	if c.Stance("mirror.2") != StanceHostile || c.Stance("mirror") != StanceNeutral {
		t.Errorf("after contact: %q %q", c.Stance("mirror"), c.Stance("mirror.2"))
	}
	if c.SetStance(friend.Key, StanceHostile, nil) || c.Stance(friend.Key) != StanceNeutral {
		t.Errorf("a faction of the player's group turned: %q", c.Stance(friend.Key))
	}
}

// A faction never spreads onto a world its own side holds (itself or a
// faction of its group), but may onto a hostile faction's.
func TestHeldBySide(t *testing.T) {
	c := twoOfAKind(t)
	d := LoadFactionData().Doctrine("hive")
	c.AddFaction("hive", 1, 0)
	two := c.AddFaction("hive", 2, 0)
	sites := rivalSites(t, c, 2)
	c.newPresence("hive.2", "hive", d, sites[0], 3)
	if c.HeldBySide(sites[0], "hive") {
		t.Error("hive counts hive.2's world as its own side's")
	}
	if !c.HeldBySide(sites[0], "hive.2") || c.HeldBySide(sites[1], "hive.2") {
		t.Error("hive.2's own world")
	}
	c.FactionTable["hive"].Group = two.Group
	if !c.HeldBySide(sites[0], "hive") {
		t.Error("a friendly faction's world should be off limits")
	}
}

// Two grey factions are two motherships with their own hoards.
func TestTwoGreyFactionsTwoMotherships(t *testing.T) {
	c := twoOfAKind(t)
	d := LoadFactionData().Doctrine("greyish")
	for _, slot := range []int{1, 2} {
		f := c.AddFaction("greyish", slot, 0)
		c.newMothership(f.Key, d, map[string]bool{})
	}
	a, b := c.Mothership("greyish"), c.Mothership("greyish.2")
	if a == nil || b == nil || a == b || a.Presence.Faction == b.Presence.Faction {
		t.Fatalf("motherships %v %v", a, b)
	}
	if c.DoctrineOf(b.Presence.Faction) != "greyish" {
		t.Errorf("the second mothership's doctrine is %q", c.DoctrineOf(b.Presence.Faction))
	}
}

// A campaign with no table (a save from before it, the count path) gets a
// row for every faction it holds, the player in group 1, each rival in a
// group of its own; every presence's doctrine agrees with its row, then and
// after a month of the daily step.
func TestEnsureFactionTable(t *testing.T) {
	useRepoData(t)
	c, err := GenerateCampaignOpts("Old", 5, GenOptions{Factions: 4})
	if err != nil {
		t.Fatal(err)
	}
	c.FactionTable = nil
	c.EnsureFactionTable()
	if g := c.GroupOf(c.Player.Faction); g != PlayerGroup {
		t.Errorf("player group %d", g)
	}
	groups := map[int]string{}
	check := func(when string) {
		for _, loc := range c.Locations {
			for _, p := range loc.Factions {
				if c.DoctrineOf(p.Faction) != p.Doctrine {
					t.Errorf("%s: %s on %s runs %s, its row says %s", when, p.Faction, loc.ID, p.Doctrine, c.DoctrineOf(p.Faction))
				}
			}
		}
		for _, f := range c.Fleets {
			if c.DoctrineOf(f.Presence.Faction) != f.Presence.Doctrine {
				t.Errorf("%s: fleet %s of %s runs %s", when, f.ID, f.Presence.Faction, f.Presence.Doctrine)
			}
		}
	}
	for _, f := range c.Factions() {
		if f.Key == c.Player.Faction {
			continue
		}
		if other, ok := groups[f.Group]; ok {
			t.Errorf("%s and %s share group %d", f.Key, other, f.Group)
		}
		groups[f.Group] = f.Key
	}
	if len(groups) < 3 {
		t.Errorf("only %d rival rows: %v", len(groups), groups)
	}
	check("seeded")
	for i := 0; i < 30; i++ {
		c.AdvanceDays(1)
		c.StepFactions(nil)
	}
	check("a month on")
}

// The faction table survives a save: keys, doctrines, names, groups and
// ordinals come back as they were.
func TestFactionTableSaves(t *testing.T) {
	c := twoOfAKind(t)
	c.AddFaction("hive", 1, 0)
	c.AddFaction("hive", 2, 7)
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back Campaign
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	f := back.FactionOf("hive.2")
	if f == nil || f.Doctrine != "hive" || f.Group != 7 || f.Ordinal != 2 || f.Name == "" || back.FactionOf(c.Player.Faction) == nil {
		t.Errorf("after a save: %+v", f)
	}
}

// The rival colony is the colony doctrine run by a rival (step 2): its view
// is the colony with the rival block (its name, badge, a fleet's mobility,
// the ore economy, its kit and seeding; no morphs), the same settlement and
// expedition table; the player's colony reads the colony unchanged; the old
// key "mirror" resolves to the view.
func TestRivalColonyIsTheColonyDoctrine(t *testing.T) {
	useRepoData(t)
	fd := LoadFactionData()
	base, v := fd.Doctrine(DefaultPlayerDoctrine), fd.Doctrine(RivalColonyDoctrine)
	if base == nil || v == nil {
		t.Fatal("no colony or no rival view")
	}
	if fd.Doctrine("mirror") != v || fd.Resolve("mirror") != RivalColonyDoctrine || fd.Doctrines["mirror"] != nil {
		t.Error("mirror should be an alias of the view, not a doctrine")
	}
	if v.Name != "Rival Colony" || !v.Flies() || v.Economy != "ore" || len(v.Kit) == 0 || !v.Seedable || v.SeedFrom != 2 ||
		len(v.Morphs) != 0 || v.Playable || v.Settlement != base.Settlement || v.Expedition != base.Expedition || v.ViewOf != DefaultPlayerDoctrine {
		t.Errorf("the rival view: %+v", v)
	}
	if base.Name != "Colony" || base.Flies() || base.Seedable || !base.Playable || base.DefaultMorph != "expedition" {
		t.Errorf("the player's colony changed: name %q flies %v seedable %v", base.Name, base.Flies(), base.Seedable)
	}
	if BaseDoctrine(RivalColonyDoctrine) != DefaultPlayerDoctrine {
		t.Error("BaseDoctrine")
	}
}

// A count-seeded rival colony runs the view under the faction key "mirror";
// an old save's "mirror" doctrine becomes the view on load, its key kept.
func TestRivalColonySeedingAndOldSaves(t *testing.T) {
	useRepoData(t)
	c, err := GenerateCampaignOpts("Two", 3, GenOptions{Factions: 2})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range c.Fleets {
		if f.Presence.Doctrine == RivalColonyDoctrine {
			found = true
			if f.Presence.Faction != "mirror" || c.DoctrineOf("mirror") != RivalColonyDoctrine {
				t.Errorf("rival colony faction %q, row doctrine %q", f.Presence.Faction, c.DoctrineOf("mirror"))
			}
		}
	}
	if !found {
		t.Fatal("no rival colony seeded at Rival factions 2")
	}
	for i := range c.Fleets {
		if c.Fleets[i].Presence.Faction == "mirror" {
			c.Fleets[i].Presence.Doctrine = "mirror" // as an old save has it
		}
	}
	c.FactionTable = nil
	if err := MigrateCampaign(c); err != nil {
		t.Fatal(err)
	}
	for _, f := range c.Fleets {
		if f.Presence.Faction == "mirror" && f.Presence.Doctrine != RivalColonyDoctrine {
			t.Errorf("an old mirror ship still runs %q", f.Presence.Doctrine)
		}
	}
	if c.DoctrineOf("mirror") != RivalColonyDoctrine {
		t.Errorf("the old mirror's row runs %q", c.DoctrineOf("mirror"))
	}
}

// A second faction of a doctrine takes the doctrine's first instance name
// (the rival colony through its view, from the colony's list).
func TestInstanceNames(t *testing.T) {
	c := twoOfAKind(t)
	c.AddFaction("hive", 1, 0)
	if f := c.AddFaction("hive", 2, 0); f.Name != LoadFactionData().Doctrine("hive").InstanceNames[0] {
		t.Errorf("the second hive is named %q", f.Name)
	}
	c.AddFaction(RivalColonyDoctrine, 3, 0)
	if f := c.AddFaction(RivalColonyDoctrine, 4, 0); f.Name != LoadFactionData().Doctrine("colony").InstanceNames[0] {
		t.Errorf("the second rival colony is named %q", f.Name)
	}
}
