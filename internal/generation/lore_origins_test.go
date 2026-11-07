package generation

import (
	"math/rand"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// islandLevel is sea everywhere except one 20x20 island at (20..39, 20..39).
func islandLevel(t *testing.T) *world.Level {
	t.Helper()
	loadPlacementData(t)
	level := flatLevel(t, 64, 64)
	z := level.SurfaceZ
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if x < 20 || x >= 40 || y < 20 || y >= 40 {
				level.SetMiddle(x, y, z, "water", 0)
			}
		}
	}
	return level
}

// A rolled origin in the sea moves to the nearest open ground, and it is stored
// once on the level under its event ID and, for the chapter, as "origin".
func TestLoreOriginResolvesToNearestOpenGround(t *testing.T) {
	level := islandLevel(t)
	ResolveOrigins(level, map[string][2]int{"ev3": {5, 30}, "ev7": {30, 30}}, "ev3")
	x, y, z, ok := Origin(level, "ev3")
	if !ok {
		t.Fatal("ev3 not stored")
	}
	if !standableSurface(level, x, y) || z != level.GetSurfaceZ(x, y) {
		t.Errorf("ev3 resolved to (%d,%d,%d), not open surface", x, y, z)
	}
	if x < 20 || x > 26 {
		t.Errorf("ev3 resolved to x=%d, want the island's near shore", x)
	}
	cx, cy, cz, ok := Origin(level, ChapterOrigin)
	if !ok || cx != x || cy != y || cz != z {
		t.Errorf("chapter origin (%d,%d,%d) differs from ev3 (%d,%d,%d)", cx, cy, cz, x, y, z)
	}
	if x2, y2, _, _ := Origin(level, "origin:ev7"); x2 != 30 || y2 != 30 {
		t.Errorf("an origin already on open ground moved to (%d,%d)", x2, y2)
	}
}

// A feature with "at" is placed around the resolved origin, and a stamp's extra
// params reach its script.
func TestLoreAnchoredFeaturesUseTheOrigin(t *testing.T) {
	level := islandLevel(t)
	ResolveOrigins(level, map[string][2]int{"ev1": {33, 33}}, "ev1")
	ox, oy, _, _ := Origin(level, "ev1")
	var gotParams map[string]any
	prev := structureRunner
	t.Cleanup(func() { structureRunner = prev })
	var at [2]int
	SetStructureRunner(func(_ *world.Level, _ string, x, y, w, h int, p map[string]any, _ int64) error {
		at, gotParams = [2]int{x + w/2, y + h/2}, p
		return nil
	})
	spec := FeatureSpec{Kind: "stamp", Count: 1, At: "origin:ev1",
		Params: map[string]any{"script": "crashed_ufo", "w": 5, "h": 5, "registry": "VX-9"}}
	if err := placeStamp(level, spec, rand.New(rand.NewSource(1))); err != nil {
		t.Fatal(err)
	}
	if abs(at[0]-ox) > 4 || abs(at[1]-oy) > 4 {
		t.Errorf("anchored stamp centred at %v, origin (%d,%d)", at, ox, oy)
	}
	if gotParams["registry"] != "VX-9" || gotParams["script"] != nil {
		t.Errorf("stamp params %v, want registry passed through and script kept back", gotParams)
	}
	// With no such origin (a scenario stamp on a site with no history) it
	// places as an ordinary stamp.
	missing := FeatureSpec{Kind: "stamp", Count: 1, At: "origin:nope", Params: map[string]any{"script": "x"}}
	at = [2]int{-1, -1}
	if err := placeStamp(level, missing, rand.New(rand.NewSource(1))); err != nil || at == [2]int{-1, -1} {
		t.Errorf("a stamp anchored at a missing origin was not placed (%v)", err)
	}
}

// A stamp clears what stands inside its footprint before it paints: flora
// scattered over the ground is gone inside, untouched outside.
func TestLoreStampClearsItsFootprint(t *testing.T) {
	loadPlacementData(t)
	level := flatLevel(t, 64, 64)
	z := level.SurfaceZ
	for y := 2; y < 62; y += 3 {
		for x := 2; x < 62; x += 3 {
			e, err := factory.Create("alien_flora", x, y, z)
			if err != nil {
				t.Fatal(err)
			}
			level.AddEntity(e)
		}
	}
	prev := structureRunner
	t.Cleanup(func() { structureRunner = prev })
	var fx, fy, fw, fh int
	SetStructureRunner(func(_ *world.Level, _ string, x, y, w, h int, _ map[string]any, _ int64) error {
		fx, fy, fw, fh = x, y, w, h
		return nil
	})
	spec := FeatureSpec{Kind: "stamp", Count: 1, Params: map[string]any{"script": "castle", "w": 28, "h": 22}}
	if err := placeStamp(level, spec, rand.New(rand.NewSource(3))); err != nil {
		t.Fatal(err)
	}
	inside, outside := 0, 0
	for _, e := range append(append([]*ecs.Entity{}, level.Entities...), level.StaticEntities...) {
		if e == nil || e.Blueprint != "alien_flora" {
			continue
		}
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if pc.GetX() >= fx && pc.GetX() < fx+fw && pc.GetY() >= fy && pc.GetY() < fy+fh {
			inside++
		} else {
			outside++
		}
	}
	want := 0
	for y := 2; y < 62; y += 3 {
		for x := 2; x < 62; x += 3 {
			if !(x >= fx && x < fx+fw && y >= fy && y < fy+fh) {
				want++
			}
		}
	}
	if inside != 0 || outside != want {
		t.Errorf("after the stamp at (%d,%d %dx%d): %d flora inside, %d outside (want 0 and %d)", fx, fy, fw, fh, inside, outside, want)
	}
}
