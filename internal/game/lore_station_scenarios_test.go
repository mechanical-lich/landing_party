package game

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// stationCharted builds a station site whose history ends in chapter, as the
// game builds a charted site: its evidence, plan, lore flags and scenario.
func stationCharted(t *testing.T, seed int64, chapter string) (*MainState, *history.History, *history.SiteHistory) {
	t.Helper()
	lib, err := history.DefaultLibrary()
	if err != nil {
		t.Fatal(err)
	}
	h := history.New(seed)
	site, err := h.BindSiteEnding(lib, "abandoned_station", seed, chapter)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	const colony = "Station Colony"
	cfg := SettlementConfig{Name: colony, ColonyName: colony, MapID: "abandoned_station", ScenarioID: site.Scenario, Seed: seed, CampaignMode: true}
	lore, _ := siteLoreFor(h, site)
	lore.apply(&cfg)
	prev := plazaIntn
	plazaIntn = rand.New(rand.NewSource(seed)).Intn
	t.Cleanup(func() { plazaIntn = prev })
	ms, err := NewMainState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ms.teardown)
	return ms, h, site
}

// Each scenario that plays on a station puts its threat where its chapter
// is, not on the landing: over 20 sites per scenario its creatures are
// there, none stands on space, none on the dock's deck within the hazard
// keep-out of the landing, and the chapter's origin is in a module off the
// dock's deck.
func TestLoreStationScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("builds 80 stations")
	}
	loreLoadData(t)
	b := generation.Budget()
	for _, c := range []struct {
		chapter string
		threats []string
	}{
		{"station_experiment", []string{"mutant_grunt"}},
		{"salvage_expedition", []string{"greyish", "whitish", "greenish", "pinkish"}},
		{"infested_colonists", []string{"xeno_egg", "xeno_hatchling", "xeno_stage2", "xeno_stage3"}},
		{"swarm_landing", []string{"enforcer", "wire_placer"}},
	} {
		want := map[string]bool{}
		for _, bp := range c.threats {
			want[bp] = true
		}
		for seed := int64(1); seed <= 20; seed++ {
			ms, _, site := stationCharted(t, seed*101, c.chapter)
			what := fmt.Sprintf("%s seed %d (%s)", c.chapter, seed*101, site.Scenario)
			lvl := ms.level
			lx, ly, lz, ok := generation.Landing(lvl)
			if !ok {
				t.Fatalf("%s: no landing", what)
			}
			ox, oy, oz, _ := generation.Origin(lvl, generation.ChapterOrigin)
			if m := lvl.ModuleAt(ox, oy, oz); m == nil || m.Z == lz {
				t.Errorf("%s: chapter origin (%d,%d,%d) not in a module off the dock's deck", what, ox, oy, oz)
			}
			n := 0
			for _, list := range [][]*ecs.Entity{lvl.Entities, lvl.StaticEntities} {
				for _, e := range list {
					if e == nil || !want[e.Blueprint] {
						continue
					}
					pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
					x, y, z := pc.GetX(), pc.GetY(), pc.GetZ()
					n++
					if tl := lvl.GetTilePtr(x, y, z); tl == nil || world.IsSpaceTile(tl) {
						t.Errorf("%s: %s on space at (%d,%d,%d)", what, e.Blueprint, x, y, z)
					}
					if d := max(abs(x-lx), abs(y-ly)) - b.LandingHalf; z == lz && d < b.HazardKeepOut {
						t.Errorf("%s: %s on the dock's deck %d tiles from the landing", what, e.Blueprint, d)
					}
				}
			}
			if n == 0 {
				t.Errorf("%s: no threat on the station", what)
			}
		}
	}
}
