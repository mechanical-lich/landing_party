package generation

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// center_start tags the map centre as a starting_floor anchor on the surface, so
// findStartingPlaza lands there instead of rolling a random spot. Off by default.
func TestPlanetPrimerCenterStart(t *testing.T) {
	level := world.NewLevel(64, 64, 10)
	if err := (PlanetPrimer{}).Prime(level, map[string]any{"mountain_frequency": 0.0}, 7); err != nil {
		t.Fatal(err)
	}
	if n := len(level.Regions["starting_floor"]); n != 0 {
		t.Fatalf("default primer tagged %d starting_floor anchors, want none", n)
	}

	level = world.NewLevel(64, 64, 10)
	params := map[string]any{"mountain_frequency": 0.0, "center_start": true}
	if err := (PlanetPrimer{}).Prime(level, params, 7); err != nil {
		t.Fatal(err)
	}
	anchors := level.Regions["starting_floor"]
	if len(anchors) != 1 {
		t.Fatalf("center_start tagged %d anchors, want 1", len(anchors))
	}
	if a := anchors[0]; a[0] != 32 || a[1] != 32 || a[2] != level.SurfaceZ {
		t.Fatalf("anchor = %v, want (32,32,%d)", a, level.SurfaceZ)
	}
	if k := level.GetTerrainKind(32, 32, level.SurfaceZ); k != world.TKSurface {
		t.Fatalf("centre terrain kind = %v, want surface", k)
	}
}
