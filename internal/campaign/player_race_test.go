package campaign

import (
	"math"
	"math/rand"
	"testing"
)

func hiveRunCampaign(t *testing.T, seed int64) *Campaign {
	t.Helper()
	useRepoData(t)
	c, err := GenerateCampaignOpts("Brood", seed, GenOptions{Doctrine: "hive"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddPlayerBrood(); err != nil {
		t.Fatal(err)
	}
	return c
}

// The campaign modes: the hive and the autonz spread, the colony and the
// fleet doctrines fly; an observer run is never a race run.
func TestCampaignModes(t *testing.T) {
	useRepoData(t)
	fd := LoadFactionData()
	for key, want := range map[string]string{"colony": ModeShip, "hive": ModeSpread, "autonz": ModeSpread, "greyish": ModeShip, "mirror": ModeShip} {
		if got := fd.Doctrine(key).CampaignMode(); got != want {
			t.Errorf("%s: mode %q, want %q", key, got, want)
		}
	}
	if got := PlayableDoctrines(); len(got) != 4 || got[0] != "colony" || got[1] != "hive" || got[2] != "autonz" || got[3] != "greyish" {
		t.Errorf("playable %v, want colony, hive, autonz, greyish (race.order)", got)
	}
	c, err := GenerateCampaignOpts("Watch", 3, GenOptions{Doctrine: "hive", Observer: true})
	if err != nil {
		t.Fatal(err)
	}
	if c.SpreadRun() {
		t.Error("an observer run counts as a race run")
	}
}

// A hive run's start is the race block's brood on the start system, Known,
// of the player's faction; its eggs land as the player's, Known, running
// the hive doctrine.
func TestPlayerBroodAndItsEggs(t *testing.T) {
	c := hiveRunCampaign(t, 5)
	loc := c.Locations[StartLocationID]
	p := loc.Presence(PlayerFaction)
	if p == nil || p.Doctrine != "hive" || p.Population != 8 || p.Units["drone"] != 6 || p.Larvae != 3 ||
		p.Stockpile["biomass"] != 30 || !p.Known || p.Stage != 1 || !loc.Visited {
		t.Fatalf("brood %+v", p)
	}
	if got := c.PlayerSites(); len(got) != 1 || got[0] != loc {
		t.Fatalf("player sites %v", got)
	}
	var target *Location
	for _, l := range c.sortedLocations() {
		if l != loc && c.FactionSiteOK(l, false) && len(l.Factions) == 0 {
			target = l
			break
		}
	}
	if target == nil {
		t.Skip("no free site on this map")
	}
	c.landFleet(LoadFactionData(), Fleet{ToID: target.ID, Presence: FactionPresence{Faction: PlayerFaction, Doctrine: "hive", Population: 2, KnownTechs: []string{"hive_warrior_brood"}}})
	q := target.Presence(PlayerFaction)
	if q == nil || q.Doctrine != "hive" || !q.Known || len(q.KnownTechs) != 1 {
		t.Fatalf("landed %+v", q)
	}
}

// Lost when nothing of the player's is left anywhere (a frozen last world
// counts as anywhere); not while an egg is in flight or a node is dormant;
// won at the race's win.sites awake worlds.
func TestSpreadOutcome(t *testing.T) {
	c := hiveRunCampaign(t, 5)
	if got := c.SpreadOutcome(); got != "" {
		t.Fatalf("outcome %q at the start", got)
	}
	loc := c.Locations[StartLocationID]
	brood := loc.Factions
	loc.Factions = nil
	c.Fleets = append(c.Fleets, Fleet{ID: "egg", Kind: FleetProjectile, Presence: FactionPresence{Faction: PlayerFaction, Doctrine: "hive"}})
	if got := c.SpreadOutcome(); got != "" {
		t.Errorf("outcome %q with an egg in flight", got)
	}
	c.Fleets = nil
	if got := c.SpreadOutcome(); got != "lost" {
		t.Errorf("outcome %q with nothing left, want lost", got)
	}
	loc.Factions = append(loc.Factions, FactionPresence{ID: "d", Faction: PlayerFaction, Doctrine: "hive", Dormant: true})
	if got := c.SpreadOutcome(); got != "" {
		t.Errorf("outcome %q with a dormant presence", got)
	}
	loc.Factions = brood
	want := PlayerDoctrine(c).Race.Win.Sites
	n := 1
	for _, l := range c.sortedLocations() {
		if n >= want {
			break
		}
		if l != loc && l.HasSite() {
			l.Factions = append(l.Factions, FactionPresence{ID: l.ID, Faction: PlayerFaction, Doctrine: "hive", Population: 3})
			n++
		}
	}
	if n < want {
		t.Skip("not enough sites on this map")
	}
	if got := c.SpreadOutcome(); got != "won" {
		t.Errorf("outcome %q holding %d worlds, want won", got, n)
	}
}

// The player's aim: with the launch taken over the launcher fires only at
// the armed target, and not at all unarmed; given back, it picks as a
// rival's does.
func TestPlayerAimedLaunch(t *testing.T) {
	c := hiveRunCampaign(t, 5)
	fd := LoadFactionData()
	loc := c.Locations[StartLocationID]
	p := loc.Presence(PlayerFaction)
	d := fd.Doctrine("hive")
	p.Population = 40
	p.Stage = len(d.Stages) - 1
	p.Units = nil
	none := func(string) bool { return false }
	st := c.LaunchStatus(loc, p, none)
	if len(st.Targets) < 2 {
		t.Skip("fewer than two sites in range on this map")
	}
	target := st.Targets[len(st.Targets)-1]
	rng := rand.New(rand.NewSource(1))
	p.SetManual(GroupLaunch, true)
	if c.tryLaunch(rng, fd, d, loc, p, c.FactionDay, none) {
		t.Fatal("fired with the launch taken over and nothing aimed")
	}
	p.LaunchTarget = target.ID
	if !c.tryLaunch(rng, fd, d, loc, p, c.FactionDay, none) {
		t.Fatal("did not fire at the aimed target")
	}
	if f := c.Fleets[len(c.Fleets)-1]; f.ToID != target.ID || f.Presence.Faction != PlayerFaction || p.LaunchTarget != "" {
		t.Fatalf("egg to %s (faction %s), aim left %q; want %s", f.ToID, f.Presence.Faction, p.LaunchTarget, target.ID)
	}
	// Not taken over and unaimed, it still holds fire: the director never
	// picks a world for the player (the owner, 2026-11-12).
	p.SetManual(GroupLaunch, false)
	p.LastShotDay = NeverFired
	p.Population = 40
	if c.tryLaunch(rng, fd, d, loc, p, c.FactionDay, none) {
		t.Error("an unaimed player launcher fired on the director's choice")
	}
	// Tier 1 too: a frozen world's day never fires unaimed.
	for i := 0; i < 60; i++ {
		c.AdvanceDays(1)
		c.StepFactions(none)
	}
	held := 0
	for _, l := range c.Locations {
		if c.HoldsSite(l) {
			held++
		}
	}
	inFlight := 0
	for _, f := range c.Fleets {
		if f.Presence.Faction == PlayerFaction && f.ToID != target.ID {
			inFlight++
		}
	}
	if held > 2 || inFlight > 0 {
		t.Errorf("Tier 1 spread unaimed: %d worlds held (the start and the aimed one at most), %d eggs elsewhere", held, inFlight)
	}
}

// A hive run's rivals: the rival colony first (from Rival factions 1), no
// rival hive; the colony's own seeding is untouched (the history-dump table
// checks that).
func TestRaceSeeding(t *testing.T) {
	useRepoData(t)
	for _, n := range []int{1, 2, 3} {
		c, err := GenerateCampaignOpts("Brood", 9, GenOptions{Doctrine: "hive", Factions: n})
		if err != nil {
			t.Fatal(err)
		}
		mirror, hives := 0, 0
		for _, f := range c.Fleets {
			if f.Presence.Faction == "mirror" {
				mirror++
			}
		}
		for _, l := range c.Locations {
			for _, p := range l.Factions {
				if p.Faction == "hive" {
					hives++
				}
			}
		}
		if mirror != 1 || hives != 0 {
			t.Errorf("rival factions %d: %d mirror ships, %d rival hives; want 1 and 0", n, mirror, hives)
		}
	}
}

// The rival colony turns hostile when its ship lands on a world the hive
// holds; a faction already hostile, or the player's own, never changes.
func TestRaceContactOnLanding(t *testing.T) {
	c := hiveRunCampaign(t, 5)
	loc := c.Locations[StartLocationID]
	if c.Stance("mirror") != StanceNeutral {
		t.Fatalf("mirror starts %q", c.Stance("mirror"))
	}
	c.Fleets = append(c.Fleets, Fleet{ID: "m", Kind: FleetShip, ToID: loc.ID, Presence: FactionPresence{ID: "fpm", Faction: "mirror", Doctrine: RivalColonyDoctrine, Population: 4}})
	c.LandShip(&c.Fleets[len(c.Fleets)-1], c.FactionDay)
	if c.Stance("mirror") != StanceHostile {
		t.Error("the mirror landed on the hive's world and stayed neutral")
	}
	if c.RaceContact(PlayerFaction, loc) || c.Stance(PlayerFaction) != StanceNeutral {
		t.Error("the player's own faction turned")
	}
}

// The race run's fog: worlds within the reach of a world at its launcher
// stage are charted and viewable; before that stage nothing is in reach.
func TestRaceReachCharts(t *testing.T) {
	c := hiveRunCampaign(t, 5)
	loc := c.Locations[StartLocationID]
	p := loc.Presence(PlayerFaction)
	d := LoadFactionData().Doctrine("hive")
	var far *Location
	for _, l := range c.sortedLocations() {
		if l.HasSite() && l != loc && math.Hypot(l.X-loc.X, l.Y-loc.Y) <= d.Range(nil) {
			far = l
			break
		}
	}
	if far == nil {
		t.Skip("no site within the launcher's range")
	}
	far.Discovered = false
	if c.InRaceReach(far) || len(c.ChartRaceReach()) != 0 {
		t.Fatal("a nest-stage brood already reaches the stars")
	}
	for i, st := range d.Stages {
		if st.Launcher {
			p.Stage = i
		}
	}
	if !c.InRaceReach(far) {
		t.Fatal("not in reach at the launcher stage")
	}
	c.ChartRaceReach()
	if !far.Discovered {
		t.Error("the world in reach was not charted")
	}
}

// An autonz run's signal, aimed: with the launch taken over it founds a
// dormant node of the player's only at the armed site; the dormant node keeps
// the run alive but is not a world held toward the win.
func TestPlayerAimedSignal(t *testing.T) {
	useRepoData(t)
	c, err := GenerateCampaignOpts("Node", 5, GenOptions{Doctrine: "autonz"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddPlayerBrood(); err != nil {
		t.Fatal(err)
	}
	fd := LoadFactionData()
	d := fd.Doctrine("autonz")
	loc := c.Locations[StartLocationID]
	p := loc.Presence(PlayerFaction)
	p.Population, p.Units = 20, nil
	for i, st := range d.Stages {
		if st.Signal {
			p.Stage = i
		}
	}
	none := func(string) bool { return false }
	st := c.SignalStatus(loc, p, none)
	if len(st.Targets) == 0 {
		t.Skip("no site in signal range on this map")
	}
	target := st.Targets[len(st.Targets)-1]
	rng := rand.New(rand.NewSource(3))
	p.SetManual(GroupLaunch, true)
	if c.trySignal(rng, fd, d, loc, p, c.FactionDay, none, -1) != nil {
		t.Fatal("signalled unarmed")
	}
	p.LaunchTarget = target.ID
	if got := c.trySignal(rng, fd, d, loc, p, c.FactionDay, none, -1); got != target {
		t.Fatalf("signalled %v, want %s", got, target.ID)
	}
	q := target.Presence(PlayerFaction)
	if q == nil || !q.Dormant || !q.Known || q.Doctrine != "autonz" {
		t.Fatalf("node %+v", q)
	}
	if len(c.PlayerSites()) != 1 || !c.HoldsSite(target) {
		t.Errorf("player sites %d (want 1: a dormant node is not held toward the win), holds %v", len(c.PlayerSites()), c.HoldsSite(target))
	}
	loc.Factions = nil
	if c.SpreadOutcome() != "" {
		t.Error("lost with a dormant node left")
	}
}

// A ship race (the greys) plays under the player's faction key, neutral to
// itself, so rival greys stay a rival faction; its start is the race
// block's crew. The colony keeps its own key.
func TestShipRaceFaction(t *testing.T) {
	useRepoData(t)
	c, err := GenerateCampaignOpts("Saucer", 3, GenOptions{Doctrine: "greyish"})
	if err != nil {
		t.Fatal(err)
	}
	if c.SpreadRun() || c.Player.Faction != PlayerFaction || c.Player.Doctrine != "greyish" || c.Stance(PlayerFaction) != StanceNeutral {
		t.Errorf("grey run: spread %v, faction %q, doctrine %q, own stance %q", c.SpreadRun(), c.Player.Faction, c.Player.Doctrine, c.Stance(PlayerFaction))
	}
	st := PlayerDoctrine(c).ShipStart()
	if st == nil || st.Units["grey_soldier"] != 1 || len(st.Aboard) == 0 {
		t.Errorf("grey ship start %+v", st)
	}
	col, err := GenerateCampaignOpts("Colony", 3, GenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if col.Player.Faction != DefaultPlayerDoctrine || PlayerDoctrine(col).ShipStart() != nil {
		t.Errorf("colony run: faction %q, ship start %v", col.Player.Faction, PlayerDoctrine(col).ShipStart())
	}
}
