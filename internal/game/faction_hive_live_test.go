package game

import (
	"math"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
)

// hiveOnStart is a hive presence on the start site of a New Expedition, with
// one colonist beamed down, entered.
func hiveOnStart(t *testing.T, seed int64, p campaign.FactionPresence) (*WorldManager, *MainState, *campaign.Location) {
	t.Helper()
	useContentTables(t)
	useRealCosts(t)
	loadTutorialData(t)
	isolateSettlements(t)
	c, wm, err := newExpedition("Hive", seed, false)
	if err != nil {
		t.Fatal(err)
	}
	loc := c.Locations[c.StartID]
	p.Faction, p.Doctrine = "hive", "hive"
	if p.ID == "" {
		p.ID = "fph"
	}
	p.Seed = seed*31 + 7
	if p.LastShotDay == 0 {
		p.LastShotDay = campaign.NeverFired
	}
	loc.Factions = append(loc.Factions, p)
	if err := wm.Travel(c.StartID); err != nil {
		t.Fatal(err)
	}
	if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
		t.Fatal(err)
	}
	ms, err := wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	return wm, ms, loc
}

func stepTicks(ms *MainState, n int, stop func() bool) {
	for i := 0; i < n; i++ {
		fspath.ResetFrameCounter()
		ms.stepWorld()
		if ms.tick%30 == 0 {
			ms.purgeCompletedTasks()
		}
		if stop != nil && stop() {
			return
		}
	}
}

// A live hive at the launcher stage launches with its bodies: on a ready day
// the launch rung walks the shot to the spire, and the live rule fires only
// once they stand there, taking those bodies off the level.
func TestHiveLaunchesWithItsBodies(t *testing.T) {
	wm, ms, loc := hiveOnStart(t, 7, campaign.FactionPresence{Population: 22, Stage: 2,
		KnownTechs: []string{"hive_warrior_brood", "launcher_range_1"}, Stockpile: map[string]int{"biomass": 60}})
	c := wm.Campaign
	p := loc.PresenceByID("fph")
	d := campaign.LoadFactionData().Doctrine("hive")
	name := factionSettlementName(p, loc.ID)
	spire := hiveLauncher(ms.level, d, p)
	if spire == nil {
		t.Fatal("no spire at the hive stage")
	}
	for _, l := range c.Locations {
		l.Visited = true // first-jump sites open to an egg once visited
	}
	p.LastShotDay, c.Fleets = campaign.NeverFired, nil // Tier 1 may have fired while the ship flew
	if !c.LaunchReady(loc, p.ID, len(hiveBodies(ms.level, p.ID, name)), wm.IsLoaded) {
		cp := d.CapacityAt(loc)
		near := 0
		for _, l := range c.Locations {
			if l != loc && c.FactionSiteOK(l, false) && math.Hypot(l.X-loc.X, l.Y-loc.Y) <= d.Range(p.KnownTechs) {
				near++
			}
		}
		t.Fatalf("not ready: pop %d stage %d cap %d threshold %d lost %v sites in range %d", len(hiveBodies(ms.level, p.ID, name)), p.Stage, cp, d.FireThreshold(cp), p.LauncherLost, near)
	}
	if f := stepLiveFaction(c, loc, ms.level, wm.IsLoaded); len(f) != 0 && len(atSpire(ms.level, d, name, spire)) < d.Mobility.ShotPop {
		t.Fatal("fired without bodies at the spire")
	}
	stepTicks(ms, 3000, func() bool { return len(atSpire(ms.level, d, name, spire)) >= d.Mobility.ShotPop })
	there := atSpire(ms.level, d, name, spire)
	if len(there) < d.Mobility.ShotPop {
		t.Fatalf("%d bodies at the spire after the walk, want %d", len(there), d.Mobility.ShotPop)
	}
	before := len(hiveBodies(ms.level, p.ID, name))
	fired := stepLiveFaction(c, loc, ms.level, wm.IsLoaded)
	if len(fired) != 1 {
		t.Fatalf("fired %d with the shot at the spire", len(fired))
	}
	if after := len(hiveBodies(ms.level, p.ID, name)); after != before-d.Mobility.ShotPop {
		t.Errorf("bodies %d -> %d, want the shot of %d gone", before, after, d.Mobility.ShotPop)
	}
	for _, e := range there[:d.Mobility.ShotPop] {
		if hasEntity(ms.level, e) && !e.HasComponent(rlcomponents.Dead) {
			t.Error("a body at the spire stayed")
		}
	}
}

// Guards set on a colonist who walks inside the radius; a hostile hive
// raids once a scout has seen the colony, not before.
func TestHiveGuardsAndRaid(t *testing.T) {
	wm, ms, loc := hiveOnStart(t, 5, campaign.FactionPresence{Population: 14, Stage: 1,
		KnownTechs: []string{"hive_warrior_brood"}, Stockpile: map[string]int{"biomass": 40}})
	_ = wm
	p := loc.PresenceByID("fph")
	name := factionSettlementName(p, loc.ID)
	st := settlement.Settlements[name]
	if st == nil {
		t.Fatal("no hive settlement")
	}
	colonist := colonyWorkers(ms)[0]
	stepTicks(ms, 60, nil)
	dir := ms.factionDirectors[name]
	if dir == nil || dir.Ordered["raids"] > 0 {
		t.Fatalf("raided before contact: %v", dir)
	}
	x, y, z := freeLandingTile(ms.level, st.CenterX+5, st.CenterY, st.CenterZ)
	ms.level.PlaceEntity(x, y, z, colonist)
	stepTicks(ms, 60, nil)
	sent := 0
	for _, e := range settlementWorkers(ms.level, name) {
		wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
		if wc.CurrentTask != nil && wc.CurrentTask.Data == colonist {
			sent++
		}
	}
	if sent == 0 && !colonist.HasComponent(rlcomponents.Dead) {
		t.Errorf("no guard on a colonist %d tiles from the heart", 5)
	}
}
