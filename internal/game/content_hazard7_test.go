package game

import (
	"fmt"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/generation"
	"github.com/mechanical-lich/landing_party/internal/systems"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// contentHazardScene is open ground with one hazard entity and one colonist.
func contentHazardScene(t *testing.T, bp string, dx int) (*world.Level, *ecs.Entity, *ecs.Entity) {
	t.Helper()
	loreLoadData(t)
	level := loreFlatLevel(t, 30, 30)
	z := level.GetSurfaceZ(15, 15)
	h, err := factory.Create(bp, 15, 15, z)
	if err != nil {
		t.Fatal(err)
	}
	level.AddEntity(h)
	c, err := spawnColonist(level, "Hazard Colony", 15+dx, 15, z)
	if err != nil {
		t.Fatal(err)
	}
	return level, h, c
}

func contentHealth(e *ecs.Entity) int {
	return e.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent).Health
}

// contentTurns runs the hazard's AI for n turns.
func contentTurns(level *world.Level, e *ecs.Entity, n int) {
	ai := &systems.ScriptedAISystem{}
	for i := 0; i < n && !e.HasComponent(rlcomponents.Dead); i++ {
		_ = ai.UpdateEntity(level, e)
	}
}

// toxic_bloom: a spore pod hurts the colonist standing beside it, and none
// further off.
func TestContentSporePodHurtsNeighbours(t *testing.T) {
	level, pod, near := contentHazardScene(t, "spore_pod", 1)
	before := contentHealth(near)
	contentTurns(level, pod, 60)
	if contentHealth(near) >= before {
		t.Errorf("a colonist beside a spore pod kept %d health over 60 turns", contentHealth(near))
	}
	level, pod, far := contentHazardScene(t, "spore_pod", 4)
	before = contentHealth(far)
	contentTurns(level, pod, 60)
	if contentHealth(far) != before {
		t.Errorf("a colonist 4 tiles off lost health to a spore pod")
	}
}

// ancient_defences: a sentry turret sleeps while nobody is in range, wakes
// for a colonist within 7 tiles, and fires on them.
func TestContentSentryTurretWakes(t *testing.T) {
	level, turret, _ := contentHazardScene(t, "sentry_turret", 12)
	contentTurns(level, turret, 10)
	ai := turret.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent)
	if fmt.Sprint(ai.Vars["awake"]) == "1" {
		t.Error("the turret woke for a colonist 12 tiles away")
	}
	level, turret, near := contentHazardScene(t, "sentry_turret", 5)
	before := contentHealth(near)
	contentTurns(level, turret, 40)
	ai = turret.GetComponent(components.ScriptedAI).(*components.ScriptedAIComponent)
	if fmt.Sprint(ai.Vars["awake"]) != "1" {
		t.Error("the turret slept through a colonist 5 tiles away")
	}
	if contentHealth(near) >= before {
		t.Errorf("an awake turret did no harm in 40 turns (health %d)", contentHealth(near))
	}
}

// unexploded_ordnance: a munition goes off when a colonist steps beside it,
// leaves a crater and is gone.
func TestContentMunitionGoesOff(t *testing.T) {
	level, mun, _ := contentHazardScene(t, "old_munition", 1)
	z := level.GetSurfaceZ(15, 15)
	contentTurns(level, mun, 1)
	if !mun.HasComponent(rlcomponents.Dead) {
		t.Fatal("the munition did not go off with a colonist beside it")
	}
	if tl := level.GetTilePtr(15, 15, z); world.TileDefinitions[tl.Floor.Type].Name != "crater_ground" {
		t.Errorf("no crater where it went off (floor %s)", world.TileDefinitions[tl.Floor.Type].Name)
	}
	level, mun, _ = contentHazardScene(t, "old_munition", 5)
	contentTurns(level, mun, 20)
	if mun.HasComponent(rlcomponents.Dead) {
		t.Error("the munition went off with nobody near")
	}
}

// unstable_ground: a rubble field buries a cache of relics under rubble at
// its anchor.
func TestContentRubbleFieldBuriesACache(t *testing.T) {
	loreLoadData(t)
	level := loreFlatLevel(t, 40, 40)
	generation.ResolveOrigins(level, map[string][2]int{"ev1": {20, 20}}, "ev1")
	spec := generation.FeatureSpec{Kind: "rubble_field", Count: 1, At: "origin:ev1", Params: map[string]any{"relic": "carved_idol", "cache": 3.0}}
	if err := generation.GetFeature("rubble_field")(level, spec, nil); err != nil {
		t.Fatal(err)
	}
	items, rubble := 0, 0
	var lx, ly, lz int
	for _, e := range level.Entities {
		if e.HasComponent(components.Storage) {
			for _, it := range e.GetComponent(components.Storage).(*components.StorageComponent).Items {
				items++
				if it.HasComponent(components.Material) {
					items += it.GetComponent(components.Material).(*components.MaterialComponent).Quantity - 1
				}
			}
			pc, _ := entityPos(e)
			lx, ly, lz = pc.GetX(), pc.GetY(), pc.GetZ()
		}
	}
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if tl := level.GetTilePtr(lx+dx, ly+dy, lz); tl != nil && !tl.Middle.IsEmpty() && world.TileDefinitions[tl.Middle.Type].Name == "rubble_pile" {
				rubble++
			}
		}
	}
	// In a cave only the open cave takes rubble; at the surface the whole disc.
	if items != 3 || rubble < 4 {
		t.Errorf("cache holds %d relics under %d rubble piles; want 3 under 4 or more", items, rubble)
	}
}

// hull_breach: the breached section is vacuum but for the deck plate its
// stores stand on, so only a suited colonist reaches them.
func TestContentHullBreachIsVacuum(t *testing.T) {
	loreLoadData(t)
	level := loreFlatLevel(t, 25, 21)
	if err := RunStructureScriptWithParams(level, "hull_breach", 5, 5, 11, 9, map[string]any{"occupants": "none"}, 1); err != nil {
		t.Fatal(err)
	}
	z := level.GetSurfaceZ(10, 9)
	space, deck := 0, 0
	for y := 6; y < 13; y++ {
		for x := 6; x < 15; x++ {
			if world.IsSpaceTile(level.GetTilePtr(x, y, z)) {
				space++
			} else {
				deck++
			}
		}
	}
	if deck != 1 || space != 7*9-1 {
		t.Errorf("breached section: %d vacuum tiles, %d deck; want all vacuum but the cache's plate", space, deck)
	}
}
