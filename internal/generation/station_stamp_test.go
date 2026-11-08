package generation

import (
	"math/rand"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// spaceLevel is a level of nothing but space: no surface anywhere, as on a
// station's margins.
func spaceLevel(t *testing.T, w, h, d int) *world.Level {
	t.Helper()
	level := world.NewLevel(w, h, d)
	level.AllocTerrain()
	for z := 0; z < d; z++ {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				paintKind(level, x, y, z, world.TKSpace)
			}
		}
	}
	level.SurfaceZ = 0
	return level
}

// A stamp whose script builds nothing (a ground structure on a level with no
// surface returns at get_surface_z) is not recorded as placed; one that
// paints is.
func TestStationStampThatBuildsNothingIsNotRecorded(t *testing.T) {
	loadPlacementData(t)
	level := spaceLevel(t, 60, 60, 3)
	SetOrigin(level, "ev1", 30, 30, 1)
	prev := structureRunner
	t.Cleanup(func() { structureRunner = prev })
	paint := false
	SetStructureRunner(func(l *world.Level, _ string, x, y, w, h int, _ map[string]any, _ int64) error {
		if paint {
			l.SetFloor(x+1, y+1, 1, "hull_floor", 0)
		}
		return nil
	})
	spec := FeatureSpec{Kind: "stamp", Count: 1, At: "origin:ev1", Params: map[string]any{"script": "underground_bunker", "w": 15, "h": 13}}
	if err := placeStamp(level, spec, rand.New(rand.NewSource(1))); err != nil {
		t.Fatal(err)
	}
	if n := len(level.StampFootprints); n != 0 {
		t.Errorf("a stamp that built nothing was recorded: %d footprints", n)
	}
	paint = true
	if err := placeStamp(level, spec, rand.New(rand.NewSource(1))); err != nil {
		t.Fatal(err)
	}
	if n := len(level.StampFootprints); n != 1 {
		t.Errorf("a stamp that painted was not recorded: %d footprints", n)
	}
}
