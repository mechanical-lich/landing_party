package game

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)


// loreSite describes a one-chapter history for a headless scenario build.
type loreSite struct {
	Map, Scenario string
	Seed          int64
	Chapter       string         // origin_event; "" for none
	Flags         map[string]any // extra lore flags (origin_civ, has_wreck, ...)
	Features      []mapdef.FeatureBlock
	// Rolled is the chapter's rolled origin as fractions of the map size.
	RolledX, RolledY float64
	// Script, if set, replaces the scenario's setup scripts (a frozen copy of
	// the pre-history script, to prove the no-history path is unchanged).
	Script string
}

// loreBuild generates a site the way buildParked does for a charted site whose
// chapter has an origin, and returns it with the resolved origin. Without a
// Chapter it builds the site with no history at all.
func loreBuild(t *testing.T, s loreSite) (*MainState, int, int, int) {
	t.Helper()
	loreLoadData(t)
	if s.Script != "" {
		scenario.ByID(s.Scenario).SetupScripts = []string{s.Script}
	}
	md := mapdef.ByID(s.Map)
	if md == nil {
		t.Fatalf("no map %s", s.Map)
	}
	const colony = "Lore Colony"
	cfg := SettlementConfig{Name: colony, ColonyName: colony, MapID: s.Map, ScenarioID: s.Scenario, Seed: s.Seed, CampaignMode: true}
	if s.Chapter != "" {
		w, h, _ := md.Size.Roll(s.Seed)
		rx, ry := s.RolledX, s.RolledY
		if rx == 0 && ry == 0 {
			rx, ry = 0.3, 0.3
		}
		cfg.SiteOrigins = map[string][2]int{"ev1": {int(float64(w) * rx), int(float64(h) * ry)}}
		cfg.SiteChapter = "ev1"
		cfg.SiteFeatures = s.Features
		cfg.SiteFlags = map[string]any{"origin_event": s.Chapter, "origin_civ": "", "has_wreck": 0.0, "has_reactor": 0.0, "population": "never"}
		for k, v := range s.Flags {
			cfg.SiteFlags[k] = v
		}
	}
	// The landing plaza search draws from the global source; seed it so two
	// builds of one site land in the same place.
	prev := plazaIntn
	plazaIntn = rand.New(rand.NewSource(s.Seed)).Intn
	t.Cleanup(func() { plazaIntn = prev })
	ms, err := NewMainState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ms.teardown)
	ox, oy, oz, ok := generation.Origin(ms.level, generation.ChapterOrigin)
	if s.Chapter != "" && !ok {
		t.Fatal("chapter origin not resolved")
	}
	if !ok {
		ox, oy, oz = -1, -1, -1
	}
	return ms, ox, oy, oz
}

// loreLoadData loads generation data as the game does, structure scripts
// included (stamps are how history evidence and scenario hulls are built).
func loreLoadData(t *testing.T) {
	t.Helper()
	loadTutorialData(t)
	isolateSettlements(t)
	generation.SetStructureRunner(func(level *world.Level, name string, x, y, w, h int, params map[string]any, seed int64) error {
		return RunStructureScriptWithParams(level, name, x, y, w, h, params, seed)
	})
	t.Cleanup(func() { generation.SetStructureRunner(nil) })
}

// loreNear counts living entities of the given blueprints within r tiles of
// (x, y) (any z) and elsewhere.
func loreNear(lvl *world.Level, blueprints []string, x, y, r int) (near, far int) {
	want := map[string]bool{}
	for _, b := range blueprints {
		want[b] = true
	}
	for _, list := range [][]*ecs.Entity{lvl.Entities, lvl.StaticEntities} {
		for _, e := range list {
			if e == nil || !want[e.Blueprint] || !e.HasComponent(rlcomponents.Position) {
				continue
			}
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			if dx, dy := pc.GetX()-x, pc.GetY()-y; dx*dx+dy*dy <= r*r {
				near++
			} else {
				far++
			}
		}
	}
	return near, far
}

// loreFarFrom counts entities of the blueprints within r of (x, y), for the
// no-history checks against the landing.
func loreWithin(lvl *world.Level, blueprints []string, x, y, r int) int {
	n, _ := loreNear(lvl, blueprints, x, y, r)
	return n
}

// loreStart is the landing's start flags.
func loreStart(lvl *world.Level) (int, int) {
	return int(lvl.Flags["start_x"].(float64)), int(lvl.Flags["start_y"].(float64))
}

// lorePositions lists where entities of the blueprints stand, sorted.
func lorePositions(lvl *world.Level, blueprints []string) []string {
	want := map[string]bool{}
	for _, b := range blueprints {
		want[b] = true
	}
	var out []string
	for _, list := range [][]*ecs.Entity{lvl.Entities, lvl.StaticEntities} {
		for _, e := range list {
			if e == nil || !want[e.Blueprint] || !e.HasComponent(rlcomponents.Position) {
				continue
			}
			pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			out = append(out, fmt.Sprintf("%s@%d,%d,%d", e.Blueprint, pc.GetX(), pc.GetY(), pc.GetZ()))
		}
	}
	sort.Strings(out)
	return out
}

// loreUnchangedWithoutHistory builds a site with no history twice, with the
// converted script and with the frozen pre-history copy, and requires the
// threat to stand in exactly the same places.
func loreUnchangedWithoutHistory(t *testing.T, mapID, scenarioID, oldScript string, seed int64, blueprints []string) {
	t.Helper()
	ms, _, _, _ := loreBuild(t, loreSite{Map: mapID, Scenario: scenarioID, Seed: seed})
	got := lorePositions(ms.level, blueprints)
	old, _, _, _ := loreBuild(t, loreSite{Map: mapID, Scenario: scenarioID, Seed: seed,
		Script: "internal/game/testdata/lore_old_scripts/" + oldScript + ".basic"})
	want := lorePositions(old.level, blueprints)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("without a history the %s threat moved:\n new %v\n old %v", scenarioID, got, want)
	}
	if len(want) == 0 {
		t.Errorf("the old %s script spawned none of %v", scenarioID, blueprints)
	}
}
