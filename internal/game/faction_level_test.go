package game

import (
	"fmt"
	"sort"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

// factionTestSite is a charted site with one hive presence of pop bodies at
// stage, on the moon map with a calm scenario.
func factionTestSite(t *testing.T, scenarioID string, seed int64, pop, stage int, techs ...string) (*MainState, *campaign.Location) {
	t.Helper()
	ms, _, _, _ := loreBuild(t, loreSite{Map: "moon", Scenario: scenarioID, Seed: seed})
	loc := &campaign.Location{ID: "site", Name: "Test Site", ScenarioID: scenarioID, Seed: seed}
	loc.Factions = []campaign.FactionPresence{{
		ID: "fp1", Faction: "hive", Doctrine: "hive", Population: pop, Stage: stage,
		KnownTechs: techs, Seed: seed*977 + 13, LastShotDay: campaign.NeverFired,
	}}
	return ms, loc
}

// presenceSnapshot lists presence id's members on level, sorted.
func presenceSnapshot(level *world.Level, id string) []string {
	var out []string
	for _, e := range presenceMembers(level, id) {
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		k := components.KinOf(e)
		out = append(out, fmt.Sprintf("%s %s @%d,%d,%d", e.Blueprint, k.Type, pc.GetX(), pc.GetY(), pc.GetZ()))
	}
	sort.Strings(out)
	return out
}

// A presence builds the same way every time from its seed: its stamp, its
// launcher at the hive stage, and exactly its population in bodies split by
// the stage's mix; building it again adds nothing.
func TestMaterialiseIsDeterministic(t *testing.T) {
	var first []string
	for run := 0; run < 2; run++ {
		ms, loc := factionTestSite(t, "moon", 7, 20, 2)
		stamps := len(ms.level.StampFootprints)
		materialiseFactions(nil, loc, ms.level, ms.scenario)
		snap := presenceSnapshot(ms.level, "fp1")
		if run == 0 {
			first = snap
		} else if fmt.Sprint(snap) != fmt.Sprint(first) {
			t.Fatalf("second build differs:\n%v\n%v", first, snap)
		}
		p := &loc.Factions[0]
		if got := len(presenceBodies(ms.level, "fp1")); got != 20 {
			t.Errorf("bodies = %d, want 20", got)
		}
		if presenceLauncher(ms.level, "fp1") == nil {
			t.Error("no launcher at the hive stage")
		}
		if len(ms.level.StampFootprints) != stamps+1 || !p.Materialized || p.LevelStage != 2 {
			t.Errorf("stamps %d -> %d, materialized %v stage %d", stamps, len(ms.level.StampFootprints), p.Materialized, p.LevelStage)
		}
		morphs := map[string]int{}
		for _, e := range presenceBodies(ms.level, "fp1") {
			morphs[components.KinOf(e).Morph]++
		}
		if morphs["crawler"] == 0 || morphs["flier"] == 0 || morphs["crawler"] < morphs["flier"] {
			t.Errorf("morphs = %v, want mostly crawlers and some fliers", morphs)
		}
		materialiseFactions(nil, loc, ms.level, ms.scenario)
		if again := presenceSnapshot(ms.level, "fp1"); fmt.Sprint(again) != fmt.Sprint(snap) {
			t.Errorf("a second materialise changed the level: %d -> %d members", len(snap), len(again))
		}
		if len(ms.level.StampFootprints) != stamps+1 {
			t.Error("a second materialise stamped again")
		}
	}
}

// A presence builds clear of the history: its footprint keeps the hazard
// keep-out from the landing reserve and overlaps no stamp, and no body
// stands inside the inert keep-out; over several seeds.
func TestMaterialiseRespectsKeepOuts(t *testing.T) {
	b := generation.Budget()
	for seed := int64(1); seed <= 4; seed++ {
		ms, loc := factionTestSite(t, "moon", seed, 16, 2)
		history := append([][6]int(nil), ms.level.StampFootprints...)
		materialiseFactions(nil, loc, ms.level, ms.scenario)
		fp := ms.level.StampFootprints[len(ms.level.StampFootprints)-1]
		x, y, w, h := fp[0], fp[1], fp[2], fp[3]
		for _, c := range [][2]int{{x, y}, {x + w - 1, y}, {x, y + h - 1}, {x + w - 1, y + h - 1}} {
			if d := generation.LandingDistance(ms.level, c[0], c[1], 0); d < b.HazardKeepOut {
				t.Errorf("seed %d: footprint corner %v is %d from the landing, keep-out %d", seed, c, d, b.HazardKeepOut)
			}
		}
		for _, s := range history {
			if x <= s[0]+s[2]-1 && x+w-1 >= s[0] && y <= s[1]+s[3]-1 && y+h-1 >= s[1] {
				t.Errorf("seed %d: footprint %v overlaps history stamp %v", seed, fp, s)
			}
		}
		for _, e := range presenceMembers(ms.level, "fp1") {
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			if d := generation.LandingDistance(ms.level, pc.GetX(), pc.GetY(), pc.GetZ()); d < b.KeepOut {
				t.Errorf("seed %d: %s %d tiles from the landing", seed, e.Blueprint, d)
			}
		}
	}
}

// Every body carries a type of the stage's mix, among the unlocked ones (an
// old save's tier_advanced is the warrior brood), drawn in its type's colour
// block; growth while away tops the level up, and a stage reached while
// away stands its stamp and launcher.
func TestMaterialiseTopsUpAndTypes(t *testing.T) {
	ms, loc := factionTestSite(t, "moon", 3, 10, 1, "tier_advanced")
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	d := campaign.LoadFactionData().Doctrine("hive")
	types := map[string]int{}
	for _, e := range presenceBodies(ms.level, "fp1") {
		k := components.KinOf(e)
		dt := d.Types[k.Type]
		if dt == nil || dt.Morph != k.Morph {
			t.Fatalf("%s: type %q (morph %s)", e.Blueprint, k.Type, k.Morph)
		}
		types[k.Type]++
		base, _ := factory.Create(e.Blueprint, 0, 0, 0)
		bx := base.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX
		if got := e.GetComponent(components.Appearance).(*components.AppearanceComponent).SpriteX - bx; got != dt.SpriteDX {
			t.Errorf("%s %s drawn %d off its blueprint, want %d", k.Type, e.Blueprint, got, dt.SpriteDX)
		}
	}
	if types["warrior"] == 0 || types["drone"] == 0 || types["hunter"] > 0 || types["burrower"] > 0 {
		t.Errorf("types = %v, want drones and warriors, no locked hunters or burrowers", types)
	}
	if presenceLauncher(ms.level, "fp1") != nil {
		t.Error("a nest has a launcher")
	}
	p := &loc.Factions[0]
	p.Population, p.Stage = 14, 2
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	if n := len(presenceBodies(ms.level, "fp1")); n != 14 {
		t.Errorf("bodies after growth = %d, want 14", n)
	}
	if presenceLauncher(ms.level, "fp1") == nil || p.LevelStage != 2 {
		t.Errorf("hive stage: launcher %v, level stage %d", presenceLauncher(ms.level, "fp1") != nil, p.LevelStage)
	}
	p.Population = 9
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	if n := len(presenceBodies(ms.level, "fp1")); n != 9 {
		t.Errorf("bodies after starving = %d, want 9", n)
	}
}

// On a hive world the presence builds by the chapter's origin: one organism
// with the scenario's own creatures, and the same faction as them.
func TestMaterialiseOnAHiveWorldJoinsTheChapter(t *testing.T) {
	ms, ox, oy, _ := loreBuild(t, loreSite{Map: "moon", Scenario: "troid_outbreak", Seed: 53, Chapter: "impact_colony"})
	loc := &campaign.Location{ID: "site", Name: "Hive World", ScenarioID: "troid_outbreak", Seed: 53}
	loc.Factions = []campaign.FactionPresence{{ID: "fp1", Faction: "hive", Doctrine: "hive", Population: 8, Stage: 1, Seed: 99}}
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	x, y, _, ok := generation.Origin(ms.level, factionOriginKey("fp1"))
	if !ok {
		t.Fatal("no centre recorded")
	}
	if d := max(absI(x-ox), absI(y-oy)); d > presenceSiteRadius+12 {
		t.Errorf("centre %d tiles from the chapter's origin", d)
	}
	fliers := 0
	for _, e := range presenceBodies(ms.level, "fp1") {
		if components.KinOf(e).Morph == "flier" {
			fliers++
		}
	}
	// The fighting types swap to the site's morph; the drones stay the
	// hive's workers, so a troid world's hive has more fliers than a xeno
	// world's (hive_plan.md as built, step 1).
	d := campaign.LoadFactionData().Doctrine("hive")
	elsewhere := 0
	for _, b := range presenceComposition(d, &campaign.Location{ScenarioID: "moon"}, 1, nil, 8, 0) {
		if b.morph == "flier" {
			elsewhere++
		}
	}
	if fliers <= elsewhere {
		t.Errorf("%d fliers of 8 on a troid world, %d elsewhere: want more", fliers, elsewhere)
	}
}

// factionTestCampaign wraps loc in a campaign with a history, for events.
func factionTestCampaign(loc *campaign.Location) *campaign.Campaign {
	return &campaign.Campaign{Version: campaign.CampaignVersion, Seed: 5, History: history.New(5),
		Locations: map[string]*campaign.Location{loc.ID: loc}}
}

// Materialise then reconcile gives back the record: population, stage and
// launcher round-trip. Kills on the level come back as a smaller population,
// a destroyed launcher as lost, and clearing every body as a wipe.
func TestReconcileRoundTrips(t *testing.T) {
	ms, loc := factionTestSite(t, "moon", 5, 18, 2)
	c := factionTestCampaign(loc)
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	want := loc.Factions[0]
	reconcileFactions(c, loc, ms.level)
	got := loc.Factions[0]
	if got.Population != want.Population || got.Stage != want.Stage || got.LauncherLost {
		t.Fatalf("round trip: %+v, want %+v", got, want)
	}
	bodies := presenceBodies(ms.level, "fp1")
	for _, e := range bodies[:5] {
		e.AddComponent(&rlcomponents.DeadComponent{})
	}
	ms.level.RemoveEntity(presenceLauncher(ms.level, "fp1"))
	reconcileFactions(c, loc, ms.level)
	if p := loc.Factions[0]; p.Population != 13 || p.Stage != 2 || !p.LauncherLost {
		t.Errorf("after five kills and the launcher: %+v", p)
	}
	for _, e := range presenceBodies(ms.level, "fp1") {
		ms.level.RemoveEntity(e)
	}
	reconcileFactions(c, loc, ms.level)
	if len(loc.Factions) != 0 {
		t.Fatalf("a cleared presence survived: %+v", loc.Factions)
	}
	wiped := false
	for _, ev := range c.History.Events {
		wiped = wiped || (ev.Type == "faction_wiped" && ev.Vars["place"] == "Test Site")
	}
	if !wiped {
		t.Error("no faction_wiped event")
	}
}

// A husk stamps once where its presence lived, and its creatures die out.
func TestHuskStampsOnce(t *testing.T) {
	ms, loc := factionTestSite(t, "moon", 6, 10, 1)
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	loc.Husks = []campaign.FactionHusk{{Presence: "fp1", Faction: "hive", Doctrine: "hive", Morph: "crawler", Seed: 4}}
	loc.Factions = nil
	stamps := len(ms.level.StampFootprints)
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	if n := len(presenceMembers(ms.level, "fp1")); n != 0 {
		t.Errorf("%d creatures outlived their husk", n)
	}
	if !loc.Husks[0].Stamped || len(ms.level.StampFootprints) != stamps+1 {
		t.Fatalf("husk stamped %v, stamps %d -> %d", loc.Husks[0].Stamped, stamps, len(ms.level.StampFootprints))
	}
	cracked := 0
	for _, e := range ms.level.StaticEntities {
		if e.Blueprint == "xeno_egg_cracked" {
			cracked++
		}
	}
	if cracked == 0 {
		t.Error("the husk shows no cracked eggs")
	}
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	if len(ms.level.StampFootprints) != stamps+1 {
		t.Error("the husk stamped twice")
	}
}

// A hive on a loaded level still launches: by the Tier 1 rule with its live
// count, from its standing launcher, honouring the same LastShotDay
// cooldown; each shot takes its bodies off the level and is recorded.
func TestLiveHiveLaunchCooldown(t *testing.T) {
	ms, loc := factionTestSite(t, "moon", 9, 20, 2)
	loc.X, loc.Y, loc.Kind, loc.Tags = 1000, 1000, "planet", []string{"biomass", "ore"}
	c := factionTestCampaign(loc)
	c.FactionCount = 1
	c.Locations[campaign.AdriftLocationID] = &campaign.Location{ID: campaign.AdriftLocationID, Kind: campaign.AdriftKind}
	for i, id := range []string{"t1", "t2"} {
		c.Locations[id] = &campaign.Location{ID: id, Name: id, Kind: "planet", ScenarioID: "earth_like", X: 1010 + float64(i), Y: 1000 + 5*float64(i), Identified: true}
	}
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	loaded := func(id string) bool { return id == loc.ID }
	d := campaign.LoadFactionData().Doctrine("hive")

	// The launcher down: no shot however ready.
	launcher := presenceLauncher(ms.level, "fp1")
	ms.level.RemoveEntity(launcher)
	c.FactionDay = 1
	if f := stepLiveFaction(c, loc, ms.level, loaded); len(f) != 0 {
		t.Fatal("fired with no launcher standing")
	}
	ms.level.AddEntity(launcher)

	// The launch is physical (phase E2): with no bodies at the spire it waits,
	// the cooldown unspent; with the shot standing there it fires.
	if f := stepLiveFaction(c, loc, ms.level, loaded); len(f) != 0 {
		t.Fatal("fired with nobody at the spire")
	}
	shotToSpire(t, ms, loc, d)
	before := len(presenceBodies(ms.level, "fp1"))
	fired := stepLiveFaction(c, loc, ms.level, loaded)
	if len(fired) != 1 {
		t.Fatalf("a ready live hive did not fire (pop %d)", before)
	}
	p := loc.PresenceByID("fp1")
	if after := len(presenceBodies(ms.level, "fp1")); after != before-d.Mobility.ShotPop || p.Population != after || p.LastShotDay != 1 {
		t.Errorf("bodies %d -> %d, record %d, last shot %d", before, after, p.Population, p.LastShotDay)
	}
	if n := len(c.Fleets); n != 1 || c.Fleets[0].FromID != loc.ID {
		t.Fatalf("fleets %+v", c.Fleets)
	}
	launched := 0
	for _, ev := range c.History.Events {
		if ev.Type == "faction_launched" {
			launched++
		}
	}
	if launched != 1 {
		t.Errorf("%d launch events", launched)
	}
	cool := int(d.CooldownAt(d.CapacityAt(loc)))
	for _, day := range []int{2, cool} {
		c.FactionDay = history.StarDate(day)
		if len(stepLiveFaction(c, loc, ms.level, loaded)) != 0 {
			t.Fatalf("fired again on day %d, inside the %d-day cooldown", day, cool)
		}
	}
	// Top the level back up so it is ready again, then wait out the cooldown.
	p.Population = 20
	materialiseFactions(nil, loc, ms.level, ms.scenario)
	shotToSpire(t, ms, loc, d)
	c.FactionDay = history.StarDate(1 + cool)
	if len(stepLiveFaction(c, loc, ms.level, loaded)) != 1 || len(c.Fleets) != 2 || c.Fleets[1].ToID == c.Fleets[0].ToID {
		t.Errorf("after the cooldown: fleets %+v", c.Fleets)
	}
}

// shotToSpire stands a launch's shot of the hive's workers beside its spire,
// as the launch rung walks them there.
func shotToSpire(t *testing.T, ms *MainState, loc *campaign.Location, d *campaign.Doctrine) {
	t.Helper()
	p := loc.PresenceByID("fp1")
	name := factionSettlementName(p, loc.ID)
	spire := hiveLauncher(ms.level, d, p)
	sp, _ := entityPos(spire)
	for i, e := range settlementWorkers(ms.level, name) {
		if i >= d.Mobility.ShotPop {
			break
		}
		x, y, z := freeLandingTile(ms.level, sp.GetX()+1, sp.GetY()+1, sp.GetZ())
		ms.level.PlaceEntity(x, y, z, e)
		settlementOrders{}.Hold(e, x, y, z) // the launch rung's hold
	}
	if n := len(atSpire(ms.level, d, name, spire)); n < d.Mobility.ShotPop {
		t.Fatalf("%d at the spire", n)
	}
}
