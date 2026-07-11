package generation

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// DefaultPlanetConfig must yield well-ordered bands with an underground cavern
// band (surface >= 4), real mining depth, and sky room for mountains, across
// the planet/moon depth range.
func TestDefaultPlanetConfig(t *testing.T) {
	for depth := 10; depth <= 14; depth++ {
		c := DefaultPlanetConfig(depth)
		if !(0 < c.SurfaceZ && c.SurfaceZ < c.AtmosphereZ && c.AtmosphereZ <= c.SpaceZ && c.SpaceZ <= depth-1) {
			t.Fatalf("depth %d: bad ordering surface=%d atm=%d space=%d", depth, c.SurfaceZ, c.AtmosphereZ, c.SpaceZ)
		}
		if c.SurfaceZ < 4 {
			t.Errorf("depth %d: surfaceZ=%d < 4 leaves no cavern band", depth, c.SurfaceZ)
		}
		if under := c.SurfaceZ - 1; under < 4 {
			t.Errorf("depth %d: only %d underground layers", depth, under)
		}
		if maxMtn := depth - 2 - c.SurfaceZ; maxMtn < 3 {
			t.Errorf("depth %d: only %d z of mountain room", depth, maxMtn)
		}
	}
}

// The primer must carve underground caverns on a planet.
func TestPlanetPrimerCaverns(t *testing.T) {
	level := world.NewLevel(128, 128, 13)
	if err := (PlanetPrimer{}).Prime(level, nil, 42); err != nil {
		t.Fatal(err)
	}
	surfaceZ := level.SurfaceZ
	caverns := 0
	for z := 1; z < surfaceZ; z++ {
		for y := 0; y < 128; y++ {
			for x := 0; x < 128; x++ {
				if level.GetTerrainKind(x, y, z) == world.TKCavern {
					caverns++
				}
			}
		}
	}
	if caverns == 0 {
		t.Fatal("planet carved no underground caverns")
	}
}

// Mountains: solid rock rising above the surface, with their own interior
// caverns, when mountain_frequency is set.
func TestPlanetPrimerMountains(t *testing.T) {
	level := world.NewLevel(128, 128, 13)
	params := map[string]any{"mountain_frequency": 0.5, "mountain_scale": 40.0}
	if err := (PlanetPrimer{}).Prime(level, params, 7); err != nil {
		t.Fatal(err)
	}
	surfaceZ := level.SurfaceZ
	rockAbove, caveAbove := 0, 0
	for z := surfaceZ + 1; z < 13; z++ {
		for y := 0; y < 128; y++ {
			for x := 0; x < 128; x++ {
				switch level.GetTerrainKind(x, y, z) {
				case world.TKUnderground:
					rockAbove++
				case world.TKCavern:
					caveAbove++
				}
			}
		}
	}
	if rockAbove == 0 {
		t.Fatal("no mountain rock generated above the surface")
	}
	if caveAbove == 0 {
		t.Error("mountains generated no interior caverns above the surface")
	}
}
