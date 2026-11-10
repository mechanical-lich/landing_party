package campaign

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/mechanical-lich/landing_party/internal/history"
)

// factionTestCampaign is a hand-built map: the Wreck Site at the origin and
// one site per scenario at x (all far beyond the first-jump radius).
func factionTestCampaign(t *testing.T, sites map[string]string) *Campaign {
	t.Helper()
	useRepoData(t)
	c := &Campaign{Version: CampaignVersion, Seed: 5, Locations: map[string]*Location{}, History: history.New(5), FactionCount: 1}
	c.Locations[AdriftLocationID] = &Location{ID: AdriftLocationID, Kind: AdriftKind, Discovered: true, Visited: true}
	c.CurrentLocationID = AdriftLocationID
	x := 100.0
	for id, sc := range sites {
		c.Locations[id] = &Location{ID: id, Name: id, Kind: "planet", ScenarioID: sc, Tags: []string{"biomass", "ore"}, X: x, Y: 1000, Discovered: true}
		x += 1000 // out of each other's launcher range
	}
	return c
}

func hiveDoctrine(t *testing.T) *Doctrine {
	t.Helper()
	d := LoadFactionData().Doctrine("hive")
	if d == nil {
		t.Fatal("no hive doctrine in data/factions.json")
	}
	return d
}

func TestFactionDataLoads(t *testing.T) {
	useRepoData(t)
	fd := LoadFactionData()
	d := hiveDoctrine(t)
	if got := fd.seedable(); !reflect.DeepEqual(got, []string{"autonz", "greyish", "hive", "mirror"}) {
		t.Errorf("seedable = %v, want [autonz greyish hive mirror]", got)
	}
	if m := fd.Doctrine("mirror"); m == nil || m.SeedFrom != 2 || m.Settlement == nil {
		t.Errorf("mirror: seeded from the second rival faction with the colony's settlement, got %+v", m)
	}
	if d.Mobility.Kind != "launcher" || len(d.Stages) == 0 || !d.Stages[len(d.Stages)-1].Launcher {
		t.Errorf("hive doctrine: %+v", d)
	}
	if d.HazardFactor("zombie_plague") >= 1 || d.HazardFactor("crystalline_blight") >= 0 || d.HazardFactor("earth_like") != 1 {
		t.Error("hazard factors: plague should slow, blight should kill, the rest neutral")
	}
}

// Growth is exact and repeatable on a site out of every launcher's range.
func TestFactionGrowthDeterministic(t *testing.T) {
	run := func() FactionPresence {
		c := factionTestCampaign(t, map[string]string{"a": "earth_like"})
		c.newPresence("hive", hiveDoctrine(t), c.Locations["a"], 3)
		c.StarDate = 40
		c.StepFactions(nil)
		return c.Locations["a"].Factions[0]
	}
	a, b := run(), run()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("same seed, different presences:\n%+v\n%+v", a, b)
	}
	d := hiveDoctrine(t)
	cap := d.CapacityAt(&Location{Kind: "planet", Tags: []string{"biomass", "ore"}})
	if a.Population != cap {
		t.Errorf("after 40 days pop = %d, want capacity %d", a.Population, cap)
	}
	if a.Stage != len(d.Stages)-1 {
		t.Errorf("stage = %d, want the last (%d)", a.Stage, len(d.Stages)-1)
	}
	if len(a.KnownTechs) == 0 {
		t.Error("a hive 40 days old has researched nothing")
	}
	if a.LastStepDay != 40 {
		t.Errorf("LastStepDay = %d", a.LastStepDay)
	}
}

// The plague slows a presence, the blight kills it, and the death is recorded.
func TestFactionHazardFactor(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"calm": "earth_like", "plague": "zombie_plague", "blight": "crystalline_blight"})
	d := hiveDoctrine(t)
	for _, id := range []string{"calm", "plague", "blight"} {
		c.newPresence("hive", d, c.Locations[id], 3)
	}
	c.StarDate = 12
	c.StepFactions(nil)
	calm, plague := c.Locations["calm"].Factions[0], c.Locations["plague"].Factions[0]
	if plague.Population >= calm.Population {
		t.Errorf("plague pop %d not below calm pop %d", plague.Population, calm.Population)
	}
	c.StarDate = 60
	c.StepFactions(nil)
	if n := len(c.Locations["blight"].Factions); n != 0 {
		t.Errorf("blight presence survived 60 days: %+v", c.Locations["blight"].Factions)
	}
	if !hasEvent(c, "faction_exhausted", "blight") {
		t.Error("no faction_exhausted event for the blight world")
	}
}

func hasEvent(c *Campaign, typ, place string) bool {
	for _, ev := range c.History.Events {
		if ev.Type == typ && ev.Vars["place"] == place {
			return true
		}
	}
	return false
}

// A grown hive's first shot waits only on growth; the cooldown then holds the
// next one although the hive is still over its threshold; each egg flies
// distance / egg_speed days and founds a presence.
func TestFactionLauncherSpread(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"hive": "earth_like", "near": "earth_like", "near2": "earth_like"})
	hive := c.Locations["hive"]
	c.Locations["near"].X, c.Locations["near"].Y = hive.X+20, hive.Y // in base range
	c.Locations["near2"].X, c.Locations["near2"].Y = hive.X, hive.Y+20
	d := hiveDoctrine(t)
	cap := d.CapacityAt(hive)
	if th := d.FireThreshold(cap); th >= cap-d.Mobility.ShotPop {
		t.Fatalf("threshold %d leaves no room above it at capacity %d", th, cap)
	}
	p := c.newPresence("hive", d, hive, cap)
	if p.Stage != len(d.Stages)-1 {
		t.Fatalf("a full hive is at stage %d", p.Stage)
	}
	c.StarDate = 1
	c.StepFactions(nil)
	if len(c.Fleets) != 1 {
		t.Fatalf("eggs on day 1 = %+v", c.Fleets)
	}
	shot := c.Fleets[0]
	wantFlight := history.StarDate(math.Ceil(20 / d.Mobility.EggSpeed))
	if shot.LaunchDay != 1 || shot.ArriveDay != 1+wantFlight {
		t.Errorf("egg launched %d arrives %d, want 1 and %d", shot.LaunchDay, shot.ArriveDay, 1+wantFlight)
	}
	// Still over the threshold after the shot, but cooling down.
	c.StarDate = history.StarDate(d.Mobility.Cooldown)
	c.StepFactions(nil)
	if n := countEggsFrom(c, "hive"); n != 1 {
		t.Fatalf("hive fired %d eggs inside its %d-day cooldown", n, d.Mobility.Cooldown)
	}
	c.StarDate = history.StarDate(1 + d.Mobility.Cooldown)
	c.StepFactions(nil)
	if n := countEggsFrom(c, "hive"); n != 2 {
		t.Fatalf("hive fired %d eggs once the cooldown passed, want 2", n)
	}
	c.StarDate = shot.ArriveDay + 1
	c.StepFactions(nil)
	if !c.Locations[shot.ToID].HasFaction("hive") {
		t.Fatalf("first egg did not land on %s", shot.ToID)
	}
	if got := c.Locations[shot.ToID].Presence("hive"); got.Population < d.Mobility.EggPop || got.ArrivedDay != shot.ArriveDay {
		t.Errorf("new presence %+v", got)
	}
	if !hasEvent(c, "faction_launched", "hive") || !hasEvent(c, "faction_arrived", shot.ToID) {
		t.Error("launch / arrival not recorded")
	}
	// Every site in range is now held or targeted: the hive holds its fire.
	c.StarDate = history.StarDate(2 + 3*d.Mobility.Cooldown)
	c.StepFactions(nil)
	if last := hive.Presence("hive").LastShotDay; last != history.StarDate(1+d.Mobility.Cooldown) {
		t.Errorf("hive fired again (day %d) with no free target", last)
	}
}

// countEggsFrom counts launches from a site: eggs in flight plus landed
// presences founded by them (one per target).
func countEggsFrom(c *Campaign, from string) int {
	n := 0
	for _, ev := range c.History.Events {
		if ev.Type == "faction_launched" && ev.Vars["place"] == from {
			n++
		}
	}
	return n
}

// The firing threshold is a share of capacity with a floor, never above the
// capacity, so every site can fire.
func TestFactionFireThreshold(t *testing.T) {
	useRepoData(t)
	d := hiveDoctrine(t)
	m := d.Mobility
	for _, cap := range []int{1, 3, 6, 8, 12, 22, 24} {
		th := d.FireThreshold(cap)
		want := int(math.Ceil(m.SpreadShare * float64(cap)))
		if want < m.SpreadMin {
			want = m.SpreadMin
		}
		if want > cap {
			want = cap
		}
		if th != want || th > cap || th < 1 {
			t.Errorf("capacity %d: threshold %d, want %d", cap, th, want)
		}
		// The launcher stage is reachable at or below the threshold.
		last := len(d.Stages) - 1
		if sp := d.stagePop(last, cap); sp > th {
			t.Errorf("capacity %d: launcher stage needs %d, above the threshold %d", cap, sp, th)
		}
	}
}

// A cramped site fires, but rarely: its first shot waits the stretched
// cooldown from arrival, and the next waits it again.
func TestFactionCrampedSiteFiresRarely(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"cramped": "earth_like", "target": "earth_like"})
	cr := c.Locations["cramped"]
	cr.Kind, cr.Tags = "planet", []string{"ore", "radioactive", "hostile"} // 6+2-4+4 = 8
	c.Locations["target"].X, c.Locations["target"].Y = cr.X+15, cr.Y
	d := hiveDoctrine(t)
	cap := d.CapacityAt(cr)
	if cap >= d.Mobility.CrowdedCapacity {
		t.Fatalf("test site carries %d, not under crowded_capacity %d", cap, d.Mobility.CrowdedCapacity)
	}
	cool := int(math.Ceil(d.CooldownAt(cap)))
	if cool <= d.Mobility.Cooldown {
		t.Fatalf("crowded cooldown %d not above the base %d", cool, d.Mobility.Cooldown)
	}
	c.newPresence("hive", d, cr, 3)
	c.StarDate = history.StarDate(cool - 1)
	c.StepFactions(nil)
	if n := countEggsFrom(c, "cramped"); n != 0 {
		t.Fatalf("cramped hive fired before day %d", cool)
	}
	if p := cr.Presence("hive"); p.Population < d.FireThreshold(cap) {
		t.Fatalf("cramped hive only reached %d of threshold %d: growth, not cooldown, is holding it", p.Population, d.FireThreshold(cap))
	}
	c.StarDate = history.StarDate(cool)
	c.StepFactions(nil)
	if n := countEggsFrom(c, "cramped"); n != 1 {
		t.Fatalf("cramped hive fired %d eggs by day %d, want 1", n, cool)
	}
}

// Loaded levels are simulated for real: their presences do not step, and an
// egg bound for one waits in orbit.
func TestFactionSkipsLoaded(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"live": "earth_like"})
	c.newPresence("hive", hiveDoctrine(t), c.Locations["live"], 3)
	c.Fleets = append(c.Fleets, Fleet{ID: "fl1", Kind: FleetProjectile, ToID: "live", ArriveDay: 1, Presence: FactionPresence{Faction: "hive", Doctrine: "hive", Population: 2}})
	c.StarDate = 20
	c.StepFactions(func(id string) bool { return id == "live" })
	p := c.Locations["live"].Factions[0]
	if p.Population != 3 || p.LastStepDay != 0 {
		t.Errorf("loaded presence stepped: %+v", p)
	}
	if len(c.Fleets) != 1 {
		t.Error("egg landed on a loaded planet")
	}
	c.StarDate = 21
	c.StepFactions(nil)
	if len(c.Fleets) != 0 || c.Locations["live"].Presence("hive").Population < 5 {
		t.Errorf("egg did not land once unloaded: %+v / %+v", c.Fleets, c.Locations["live"].Factions)
	}
}

// Same seed and count: the same presences day by day; the opening map is the
// same at every count.
func TestFactionCampaignDeterministic(t *testing.T) {
	useRepoData(t)
	snap := func(n int) (map[string][]FactionPresence, []Fleet, *Campaign) {
		c, err := GenerateCampaignOpts("F", 31, GenOptions{Factions: n})
		if err != nil {
			t.Fatal(err)
		}
		c.StarDate = 100
		c.StepFactions(nil)
		out := map[string][]FactionPresence{}
		for id, l := range c.Locations {
			if len(l.Factions) > 0 {
				out[id] = l.Factions
			}
		}
		return out, c.Fleets, c
	}
	a, ae, ca := snap(3)
	b, be, _ := snap(3)
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(ae, be) {
		t.Fatal("same seed and count stepped to different presences")
	}
	if len(a) == 0 {
		t.Fatal("three rival factions left no presence after 100 days")
	}
	_, _, c0 := snap(0)
	if ca.FactionCount != 3 || c0.FactionCount != 0 {
		t.Errorf("FactionCount = %d / %d", ca.FactionCount, c0.FactionCount)
	}
	for id, l := range c0.Locations {
		if len(l.Factions) != 0 {
			t.Errorf("factions 0 seeded %s", id)
		}
		o := ca.Locations[id]
		if o == nil || o.Name != l.Name || o.MapID != l.MapID || o.ScenarioID != l.ScenarioID || o.X != l.X || o.History != l.History {
			t.Errorf("site %s differs between 0 and 3 factions", id)
		}
	}
}

// Seeding and every spread roll keep off the Wreck Site, Home, the Training
// Ground and the unvisited first jump.
func TestFactionExcludedSites(t *testing.T) {
	useRepoData(t)
	_, max := FactionCountRange()
	for seed := int64(1); seed <= 40; seed++ {
		c, err := GenerateCampaignOpts("X", seed, GenOptions{Factions: max})
		if err != nil {
			t.Fatal(err)
		}
		AddTutorialLocation(c)
		seeded := 0
		for _, l := range c.Locations {
			seeded += len(l.Factions)
		}
		if seeded == 0 {
			t.Errorf("seed %d: no presence seeded", seed)
		}
		check := func(when string) {
			for id, l := range c.Locations {
				if len(l.Factions) == 0 {
					continue
				}
				if !l.HasSite() || l.Tutorial || id == StartLocationID || (c.firstJump(l) && !l.Visited) {
					t.Errorf("seed %d %s: presence on excluded site %s (%s)", seed, when, id, l.Kind)
				}
			}
			for _, e := range c.Fleets {
				if l := c.Locations[e.ToID]; !c.FactionSiteOK(l, false) {
					t.Errorf("seed %d %s: egg bound for excluded %s", seed, when, e.ToID)
				}
			}
		}
		check("seeding")
		for day := 1; day <= 120; day += 7 {
			c.StarDate = history.StarDate(day)
			c.StepFactions(nil)
			check("spread")
		}
	}
}

// Events about an unseen presence stay unknown until the ship reveals it.
func TestFactionEventsHiddenUntilRevealed(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"a": "earth_like"})
	d := hiveDoctrine(t)
	p := c.newPresence("hive", d, c.Locations["a"], 3)
	c.recordFaction("faction_arrived", c.Locations["a"], p, d, nil)
	c.StarDate = 15
	c.StepFactions(nil)
	p = &c.Locations["a"].Factions[0]
	if len(p.Events) < 2 {
		t.Fatalf("expected arrival and stage-up events, got %v", p.Events)
	}
	for _, ev := range p.Events {
		if c.History.Known[ev] {
			t.Errorf("event %s known before the presence was seen", ev)
		}
	}
	evs := append([]string(nil), p.Events...)
	if got := c.RevealPresences(c.Locations["a"]); len(got) != 1 {
		t.Fatalf("revealed %d presences", len(got))
	}
	for _, ev := range evs {
		if !c.History.Known[ev] {
			t.Errorf("event %s still unknown after reveal", ev)
		}
	}
	if len(c.Locations["a"].KnownPresences()) != 1 {
		t.Error("presence not known after reveal")
	}
	if got := c.RevealPresencesWithin(0, 0, 10); len(got) != 0 {
		t.Error("scan out of range revealed something")
	}
}

// A campaign with presences and eggs survives the save round-trip.
func TestFactionSaveRoundTrip(t *testing.T) {
	useRepoData(t)
	c, err := GenerateCampaignOpts("RT", 12, GenOptions{Factions: 2})
	if err != nil {
		t.Fatal(err)
	}
	c.StarDate = 80
	c.StepFactions(nil)
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back Campaign
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if err := MigrateCampaign(&back); err != nil {
		t.Fatal(err)
	}
	if back.FactionCount != 2 || back.FactionDay != 80 || back.FactionSeq != c.FactionSeq || !reflect.DeepEqual(back.Fleets, c.Fleets) {
		t.Errorf("campaign faction fields lost: %d %d %d", back.FactionCount, back.FactionDay, back.FactionSeq)
	}
	for id, l := range c.Locations {
		if !reflect.DeepEqual(l.Factions, back.Locations[id].Factions) || !reflect.DeepEqual(l.Tags, back.Locations[id].Tags) {
			t.Errorf("%s: presences or tags differ after the round-trip", id)
		}
	}
	// Stepping on from the loaded copy matches stepping on from the original.
	c.StarDate, back.StarDate = 120, 120
	c.StepFactions(nil)
	back.StepFactions(nil)
	for id, l := range c.Locations {
		if !reflect.DeepEqual(l.Factions, back.Locations[id].Factions) {
			t.Errorf("%s: diverged after loading", id)
		}
	}
}

// A version 2 save (no factions, no tags) migrates to an empty faction layer
// whose clock starts at the save's star date; version 1 is refused.
func TestFactionMigrateFromV2(t *testing.T) {
	useRepoData(t)
	c := genCampaign(t, 3)
	c.StarDate = 42
	b, _ := json.Marshal(c)
	var raw map[string]any
	_ = json.Unmarshal(b, &raw)
	raw["version"] = 2
	for _, k := range []string{"faction_count", "faction_day", "faction_seq", "eggs"} {
		delete(raw, k)
	}
	for _, l := range raw["locations"].(map[string]any) {
		delete(l.(map[string]any), "tags")
		delete(l.(map[string]any), "factions")
	}
	b, _ = json.Marshal(raw)
	var old Campaign
	if err := json.Unmarshal(b, &old); err != nil {
		t.Fatal(err)
	}
	if err := MigrateCampaign(&old); err != nil {
		t.Fatal(err)
	}
	if old.Version != CampaignVersion || old.FactionDay != 42 || old.FactionCount != 0 {
		t.Errorf("migrated: version %d day %d count %d", old.Version, old.FactionDay, old.FactionCount)
	}
	for id, l := range old.Locations {
		if l.QuestTag != "" && (len(l.Tags) != 1 || l.Tags[0] != l.QuestTag) {
			t.Errorf("%s: tags %v from quest tag %q", id, l.Tags, l.QuestTag)
		}
	}
	old.StarDate = 90
	old.StepFactions(nil)
	if old.FactionDay != 90 || len(old.Fleets) != 0 {
		t.Error("migrated campaign stepped wrongly")
	}
	old.Version = 1
	if err := MigrateCampaign(&old); err == nil {
		t.Error("version 1 migrated")
	}
}

// A station has its own capacity entry and is a site like any other.
func TestFactionStationCapacity(t *testing.T) {
	useRepoData(t)
	d := hiveDoctrine(t)
	st := &Location{Kind: "station", Tags: []string{"salvage"}}
	moon := &Location{Kind: "moon", Tags: []string{"ore", "radioactive"}}
	if d.CapacityAt(st) <= d.CapacityAt(moon) {
		t.Errorf("station capacity %d not above moon %d", d.CapacityAt(st), d.CapacityAt(moon))
	}
	c := factionTestCampaign(t, nil)
	st.ID, st.X = "st", 500
	c.Locations["st"] = st
	if !c.FactionSiteOK(st, true) {
		t.Error("station refused for seeding")
	}
}

// Factions sit on top of the generated history: a hundred days of spread
// leaves every site's events, chapter, scenario and tier as charted, while
// the faction events still name their site for Legends.
func TestFactionLeavesHistoriesAlone(t *testing.T) {
	useRepoData(t)
	c, err := GenerateCampaignOpts("H", 2, GenOptions{Factions: 3})
	if err != nil {
		t.Fatal(err)
	}
	type snap struct {
		Events           []string
		Chapter, Scen    string
		LocScen, LocTier string
	}
	before := map[string]snap{}
	for id, l := range c.Locations {
		if s := c.SiteHistory(l); s != nil {
			before[id] = snap{append([]string(nil), s.Events...), s.Chapter, s.Scenario, l.ScenarioID, l.Tier}
		}
	}
	c.StarDate = 100
	c.StepFactions(nil)
	named := 0
	for _, ev := range c.History.Events {
		if len(ev.Type) > 8 && ev.Type[:8] == "faction_" && ev.Site != "" {
			named++
		}
	}
	if named == 0 {
		t.Fatal("no faction event names a site")
	}
	for id, b := range before {
		l := c.Locations[id]
		s := c.SiteHistory(l)
		got := snap{s.Events, s.Chapter, s.Scenario, l.ScenarioID, l.Tier}
		if !reflect.DeepEqual(got, b) {
			t.Errorf("%s: history changed\nbefore %+v\nafter  %+v", id, b, got)
		}
	}
}

// The pace for Rival factions 1 on the opening map with no player: no spread
// by day 40, one or two hives on every seed at day 100 and two on most (the
// target is about 2-3; a seed whose hive starts on a cramped site lags).
// Retune data/factions.json against make history-dump HFACTIONS=1 and update
// this range on purpose, never by accident.
func TestFactionPaceRivalOne(t *testing.T) {
	useRepoData(t)
	twos := 0
	for seed := int64(1); seed <= 6; seed++ {
		c, err := GenerateCampaignOpts("Pace", seed, GenOptions{Factions: 1})
		if err != nil {
			t.Fatal(err)
		}
		count := func() int {
			n := 0
			for _, l := range c.Locations {
				n += len(l.Factions)
			}
			return n
		}
		c.StarDate = 40
		c.StepFactions(nil)
		if n := count(); n != 1 {
			t.Errorf("seed %d: %d presences at day 40, want 1", seed, n)
		}
		c.StarDate = 100
		c.StepFactions(nil)
		n := count()
		if n < 1 || n > 2 {
			t.Errorf("seed %d: %d presences at day 100, want 1-2", seed, n)
		}
		if n == 2 {
			twos++
		}
	}
	if twos < 4 {
		t.Errorf("only %d of 6 seeds have two hives at day 100, want at least 4", twos)
	}
}

// Until the hive's shared identity is learned, players see a morph, chosen in
// Phase A by the site's scenario: troid_outbreak is fliers, the rest
// crawlers. Legends lines carry the morph's name, never the faction's.
func TestFactionHiveMorphNames(t *testing.T) {
	c := factionTestCampaign(t, map[string]string{"fly": "troid_outbreak", "crawl": "exterminate_xenos", "calm": "earth_like"})
	d := hiveDoctrine(t)
	if m := d.MorphAt(c.Locations["fly"]); m.Singular != "troid" || m.Tag != "TROID" {
		t.Errorf("troid_outbreak morph = %+v", m)
	}
	for _, id := range []string{"crawl", "calm"} {
		if m := d.MorphAt(c.Locations[id]); m.Singular != "xeno" || m.Tag != "XENO" {
			t.Errorf("%s morph = %+v", id, m)
		}
	}
	p := c.newPresence("hive", d, c.Locations["fly"], 3)
	c.recordFaction("faction_arrived", c.Locations["fly"], p, d, nil)
	ev := c.History.Events[len(c.History.Events)-1]
	if ev.Vars["singular"] != "troid" || ev.Vars["faction"] != "Troids" {
		t.Errorf("event vars %v: want the flier's names", ev.Vars)
	}
	for _, v := range ev.Vars {
		if v == d.Name || v == d.Singular {
			t.Errorf("event names the faction (%q) while its identity is hidden", v)
		}
	}
	// The player-facing alerts: no word of them is the faction's name. The
	// orbit alert once ended "(hive)", the top stage's name. The raid alert
	// (phase E2: the hive raids) names the morph.
	if a := d.RaidAlert(c.Locations["fly"]); !strings.Contains(a, "Troids") {
		t.Errorf("raid alert %q does not name the morph", a)
	}
	// Unit names: a type's seen name is its morph's; its name never the
	// faction's.
	for k, ty := range d.Types {
		m := d.Morphs[ty.Morph]
		if !strings.HasSuffix(strings.ToLower(ty.SeenName), m.Singular) {
			t.Errorf("type %s seen as %q, not by its morph %s", k, ty.SeenName, m.Singular)
		}
		for _, n := range []string{ty.SeenName, ty.Name} {
			if strings.Contains(strings.ToLower(n), strings.ToLower(d.Singular)) {
				t.Errorf("type %s named %q: names the faction", k, n)
			}
		}
	}
	for _, id := range []string{"fly", "crawl"} {
		loc := c.Locations[id]
		for _, msg := range []string{d.OrbitAlert(loc), d.LaunchAlert(loc), d.RaidAlert(loc)} {
			for _, w := range strings.FieldsFunc(strings.ToLower(msg), func(r rune) bool { return !unicode.IsLetter(r) }) {
				if w == strings.ToLower(d.Name) || w == strings.ToLower(d.Singular) {
					t.Errorf("alert %q names the faction while its identity is hidden", msg)
				}
			}
		}
	}
}

// troid_outbreak is the hive's own organism now: it feeds growth like
// exterminate_xenos. Unlock research does not touch the range; range research
// acts, at Phase A's cumulative cost.
func TestFactionHiveHazardAndResearch(t *testing.T) {
	useRepoData(t)
	d := hiveDoctrine(t)
	if d.HazardFactor("troid_outbreak") != d.HazardFactor("exterminate_xenos") || d.HazardFactor("troid_outbreak") <= 1 {
		t.Errorf("troid_outbreak %.2f, exterminate_xenos %.2f: both should boost growth equally",
			d.HazardFactor("troid_outbreak"), d.HazardFactor("exterminate_xenos"))
	}
	var ids []string
	cum := map[string]float64{}
	total := 0.0
	for _, r := range d.Research {
		ids = append(ids, r.ID)
		total += r.Cost
		cum[r.ID] = total
	}
	// The spine keeps Phase A's order and costs: the brood unlocks where the
	// tier entries stood, so the range entries land on the same day.
	if want := []string{"hive_warrior_brood", "launcher_range_1", "hive_hunter_brood", "launcher_range_2"}; !reflect.DeepEqual(ids[:4], want) {
		t.Errorf("research spine %v, want %v", ids[:4], want)
	}
	if cum["launcher_range_1"] != 65 || cum["launcher_range_2"] != 205 {
		t.Errorf("range entries at %.0f and %.0f cumulative points, Phase A's 65 and 205", cum["launcher_range_1"], cum["launcher_range_2"])
	}
	base := d.Mobility.BaseRange
	if r := d.Range([]string{"hive_warrior_brood", "hive_hunter_brood", "tier_advanced", "tier_elite"}); r != base {
		t.Errorf("unlock research changed the range: %.0f, base %.0f", r, base)
	}
	if r := d.Range(ids); r <= base {
		t.Errorf("range research did not extend the range: %.0f", r)
	}
}
