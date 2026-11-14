package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/workerai"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// mirrorOnStart puts a planet-carried mirror presence of pop on the start
// system of a fresh expedition, enters it with the party and returns the
// level, the location and the presence.
func mirrorOnStart(t *testing.T, seed int64, pop int) (*WorldManager, *MainState, *campaign.Location) {
	t.Helper()
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Mirror", seed, false)
	if err != nil {
		t.Fatal(err)
	}
	loc := c.Locations[campaign.StartLocationID]
	d := campaign.LoadFactionData().Doctrine("mirror")
	if d == nil || d.Settlement == nil {
		t.Fatal("no mirror doctrine with a settlement")
	}
	loc.Factions = append(loc.Factions, campaign.FactionPresence{
		ID: "fpm", Faction: "mirror", Doctrine: campaign.RivalColonyDoctrine, Population: pop,
		Stockpile: map[string]int{"metal_ore": 30, "crystal": 15}, Seed: seed*31 + 7,
		LastShotDay: campaign.NeverFired,
	})
	if err := wm.Travel(campaign.StartLocationID); err != nil {
		t.Fatal(err)
	}
	crew := wm.ShipColonists()
	if err := wm.BeamDown(crew[0]); err != nil {
		t.Fatal(err)
	}
	ms, err := wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	return wm, ms, loc
}

// neutralMirror undoes the contact a beam-down onto the mirror's site made,
// for tests of its building rather than its fighting (phase D: a hostile
// mirror raids).
func neutralMirror(t *testing.T, ms *MainState, loc *campaign.Location) {
	t.Helper()
	ms.campaign.SetStance("mirror", campaign.StanceNeutral, loc)
	components.SetFactionHostile("mirror", false)
	t.Cleanup(components.ResetFactionHostility)
}

// A planet-carried presence with a settlement block becomes a settlement on
// its level: a camp, a record keyed "<faction>:<loc>", workers that are not
// the player's, the stockpile in its own locker, and a director that builds
// with the player's order paths. Saving and freezing write population,
// techs and stockpile back; a thaw restores the settlement from the save
// without doubling anyone.
func TestFactionSettlementBuildsAndReconciles(t *testing.T) {
	wm, ms, loc := mirrorOnStart(t, 5, 3)
	neutralMirror(t, ms, loc)
	name := "mirror:" + loc.ID
	st := settlement.Settlements[name]
	if st == nil {
		t.Fatalf("no settlement %q", name)
	}
	workers := settlementWorkers(ms.level, name)
	if len(workers) != 3 {
		t.Fatalf("%d faction workers, want 3", len(workers))
	}
	for _, e := range workers {
		if components.IsColonyWorker(e, ms.colonyName()) {
			t.Error("a faction worker counts as the player's")
		}
		if f := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction; f != "mirror" {
			t.Errorf("faction worker's faction %q, want mirror", f)
		}
	}
	for _, e := range colonyWorkers(ms) {
		if e.GetComponent(components.Settlement).(*components.SettlementComponent).Name == name {
			t.Error("the player's crew list holds a faction worker")
		}
	}
	p := loc.PresenceByID("fpm")
	if !p.Materialized {
		t.Fatal("presence not marked built")
	}

	for tick := 1; tick <= 2500; tick++ {
		fspath.ResetFrameCounter()
		ms.stepWorld()
		if ms.tick%30 == 0 {
			ms.purgeCompletedTasks()
		}
		ms.advanceClock()
	}
	dir := ms.factionDirectors[name]
	if dir == nil || dir.Ordered["build:bed"] == 0 {
		t.Fatalf("the faction director ordered no bed: %v", dir)
	}
	beds := 0
	for _, e := range ms.level.Entities {
		if e != nil && e.Blueprint == "bed" {
			if pc, ok := entityPos(e); ok && absI(pc.GetX()-st.CenterX) <= 12 && absI(pc.GetY()-st.CenterY) <= 12 {
				beds++
			}
		}
	}
	if beds == 0 {
		t.Error("no bed built at the faction camp")
	}
	for _, tk := range ms.MainSettlement.Tasks.GetTasks() {
		if tk.X >= st.CenterX-12 && tk.X <= st.CenterX+12 && tk.Y >= st.CenterY-12 && tk.Y <= st.CenterY+12 {
			t.Errorf("a faction order landed on the player's queue: %s at %d,%d", tk.Action, tk.X, tk.Y)
		}
	}

	st.UnlockTech("basic_metallurgy") // as a finished research would
	if err := wm.Freeze(loc, ms); err != nil {
		t.Fatal(err)
	}
	p = loc.PresenceByID("fpm")
	if p == nil || p.Population != 3 {
		t.Fatalf("after freeze the presence is %+v, want 3 alive", p)
	}
	if p.Stockpile["metal_ore"] >= 30 || p.Stockpile["metal_ore"]+p.Stockpile["crystal"] == 0 {
		t.Errorf("stockpile %v: want the kit less what the builds took", p.Stockpile)
	}
	if !containsStr(p.KnownTechs, "basic_metallurgy") {
		t.Errorf("techs %v lack the settlement's", p.KnownTechs)
	}
	if settlement.Settlements[name] != nil {
		t.Error("the faction settlement stayed in the global map after the freeze")
	}

	thawed, err := wm.buildParked(loc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if settlement.Settlements[name] == nil {
		t.Fatal("the thaw did not restore the faction settlement")
	}
	if n := len(settlementWorkers(thawed.level, name)); n != 3 {
		t.Errorf("%d faction workers after the thaw, want 3", n)
	}
}

// A mirror ship landed on a site is built onto its level like a planet
// presence (the record rides the fleet); its crew is neutral until the
// player beams down onto the site it holds, then hostile everywhere: the
// player's colonists may attack them, they defend themselves, the stance is
// recorded and the Star Map tag reads it. Reconcile writes onto the fleet.
func TestMirrorShipLandsAndTurnsHostileOnContact(t *testing.T) {
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Mirror ship", 6, false)
	if err != nil {
		t.Fatal(err)
	}
	syncFactionHostility(c)
	loc := c.Locations[campaign.StartLocationID]
	c.FactionSeq++
	c.Fleets = append(c.Fleets, campaign.Fleet{
		ID: "flm", Kind: campaign.FleetShip, FromID: loc.ID, ToID: loc.ID,
		FromX: loc.X, FromY: loc.Y, ToX: loc.X, ToY: loc.Y, LandedID: loc.ID,
		Presence: campaign.FactionPresence{ID: "fpm", Faction: "mirror", Doctrine: campaign.RivalColonyDoctrine, Population: 4,
			Stockpile: map[string]int{"metal_ore": 30}, Seed: 99, LastShotDay: campaign.NeverFired},
	})
	if err := wm.Travel(loc.ID); err != nil {
		t.Fatal(err)
	}
	if !c.Fleets[0].Known {
		t.Error("a ship landed on the site in orbit is not seen")
	}
	ms := wm.Current()
	name := "mirror:" + loc.ID
	workers := settlementWorkers(ms.level, name)
	if len(workers) != 4 {
		t.Fatalf("%d mirror workers on the level, want 4", len(workers))
	}
	if c.Stance("mirror") != campaign.StanceNeutral || components.IsAttackTarget(workers[0]) {
		t.Fatal("the mirror should start neutral and not be a target")
	}
	if !workers[0].GetComponent(components.Worker).(*components.WorkerComponent).SelfDefend {
		t.Error("a mirror worker does not defend itself")
	}
	crew := wm.ShipColonists()
	if err := wm.BeamDown(crew[0]); err != nil {
		t.Fatal(err)
	}
	if c.Stance("mirror") != campaign.StanceHostile {
		t.Fatal("landing on a site the mirror holds did not turn it hostile")
	}
	if !components.IsAttackTarget(workers[0]) {
		t.Error("a hostile mirror worker is not an attack target")
	}
	if components.IsAttackTarget(crew[0]) {
		t.Error("the player's own colonist became a target")
	}
	ms, err = wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if err := wm.Freeze(loc, ms); err != nil {
		t.Fatal(err)
	}
	if f := c.FleetByPresence("fpm"); f == nil || f.Presence.Population != 4 || f.Presence.Stockpile["metal_ore"] != 30 {
		t.Errorf("reconcile onto the fleet: %+v", f)
	}
	// A fresh session marks the hostile stance from the save.
	components.ResetFactionHostility()
	syncFactionHostility(c)
	if !components.IsFactionHostile("mirror") {
		t.Error("the stance was not restored from the campaign")
	}
}

// Finding 3's hook: a doctrine's scripted unit has its hostility seeded
// before its first turn, the aggression when hostile, passive when neutral.
func TestSeedHostilityOnScriptedUnits(t *testing.T) {
	d := &campaign.Doctrine{Aggression: 8}
	for _, tc := range []struct {
		hostile bool
		want    float64
	}{{true, 8}, {false, 1}} {
		e := &ecs.Entity{}
		e.AddComponent(&components.ScriptedAIComponent{Script: "ai/default_humanoid"})
		seedHostility(e, d, tc.hostile)
		if got := e.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent).Vars["hostility"]; got != tc.want {
			t.Errorf("hostile=%v: hostility %v, want %v", tc.hostile, got, tc.want)
		}
	}
}

// ConvertAction end to end: a faction settlement's ConvertAction is worked
// by its own workers and lays the conversion floor; the player's order is
// refused until the settlement knows ConvertTech (no research gives it yet).
func TestConvertActionLaysTheTile(t *testing.T) {
	_, ms, loc := mirrorOnStart(t, 5, 2)
	neutralMirror(t, ms, loc)
	name := "mirror:" + loc.ID
	st := settlement.Settlements[name]
	x, y, z := st.CenterX, st.CenterY+1, st.CenterZ
	if !orderConvert(st, ms.level, x, y, z, "blue_wires") {
		t.Fatal("the convert order was refused")
	}
	if orderConvert(st, ms.level, x, y, z, "blue_wires") {
		t.Error("a second order on the same tile was taken")
	}
	done := false
	for tick := 1; tick <= 1500 && !done; tick++ {
		fspath.ResetFrameCounter()
		ms.stepWorld()
		done = workerai.Convertible(ms.level, x, y, z, "blue_wires") == false
	}
	if !done {
		t.Fatal("the faction's workers never converted the tile")
	}
	if ms.queueConvert(x+1, y, z, "blue_wires") {
		t.Error("the player converted without the tech")
	}
	ms.campaign.UnlockTech(ConvertTech)
	if !ms.queueConvert(x+1, y, z, "blue_wires") {
		t.Error("the player's order refused with the tech known")
	}
}

// syncFactionHostility skips only the player's own faction, never a rival
// that runs the player's doctrine: with the player's presence on the hive
// doctrine under its own faction key, a hostile rival hive's workers are
// still hostile (before phase F the skip compared doctrine keys, so the rival
// hive was never marked).
func TestHostilitySkipsOnlyThePlayersFaction(t *testing.T) {
	t.Cleanup(components.ResetFactionHostility)
	c := &campaign.Campaign{Player: &campaign.FactionPresence{ID: campaign.PlayerPresenceID, Faction: "player", Doctrine: "hive"},
		Diplomacy: map[string]string{"hive": campaign.StanceHostile}}
	syncFactionHostility(c)
	if !components.IsFactionHostile("hive") {
		t.Error("a rival hive is not hostile to a player on the hive doctrine")
	}
	if components.IsFactionHostile("player") {
		t.Error("the player's own faction is marked hostile")
	}
	colony := &campaign.Campaign{Player: campaign.NewPlayerPresence(""), Diplomacy: map[string]string{"colony": campaign.StanceHostile}}
	syncFactionHostility(colony)
	if components.IsFactionHostile("colony") {
		t.Error("the colony player's own faction is marked hostile")
	}
}
