package generation

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// earth_like's four biomes, as climate-space candidates.
func earthlikeCands() []biomeCandidate {
	return []biomeCandidate{
		{id: "tundra", tMin: 0, tMax: 0.35, hMin: 0.3, hMax: 1, ct: 0.175, ch: 0.65},
		{id: "forest", tMin: 0.35, tMax: 0.7, hMin: 0.45, hMax: 1, ct: 0.525, ch: 0.725},
		{id: "plains", tMin: 0.4, tMax: 0.65, hMin: 0.3, hMax: 0.6, ct: 0.525, ch: 0.45},
		{id: "desert", tMin: 0.65, tMax: 1, hMin: 0, hMax: 0.35, ct: 0.825, ch: 0.175},
	}
}

func TestPickBiome(t *testing.T) {
	cands := earthlikeCands()
	cases := []struct {
		name  string
		t, hu float64
		want  string
	}{
		// Gap (hot + wet): no box matches. Old algo dumped this into the
		// first-listed biome (tundra); nearest-centroid gives forest.
		{"hot-wet gap", 0.9, 0.9, "forest"},
		// Gap (warm + dry): nearest centroid is desert, not tundra.
		{"warm-dry gap", 0.5, 0.1, "desert"},
		// Unambiguous single box.
		{"tundra box", 0.2, 0.5, "tundra"},
		// forest∩plains overlap: old algo always took forest (listed first);
		// nearest centroid gives plains here.
		{"overlap → nearest", 0.5, 0.5, "plains"},
	}
	for _, c := range cases {
		if got := pickBiome(cands, c.t, c.hu); got != c.want {
			t.Errorf("%s: pickBiome(%.2f,%.2f) = %q, want %q", c.name, c.t, c.hu, got, c.want)
		}
	}
}

// With full latitude weight, temperature is purely latitudinal, so cold biomes
// band to the poles (top/bottom rows) and hot biomes to the equator (middle).
func TestBuildBiomeMapLatitude(t *testing.T) {
	if err := LoadBiomes("../../data/biomes"); err != nil {
		t.Fatal(err)
	}
	const w, h = 64, 64
	level := world.NewLevel(w, h, 3)
	level.AllocTerrain()
	BuildBiomeMap(level, BiomeMapConfig{
		Type:           "perlin_temp_humidity",
		Scale:          80,
		Biomes:         []string{"tundra", "forest", "plains", "desert"},
		LatitudeWeight: 1,
	}, 999)

	tundraFrac := func(y int) float64 {
		n := 0
		for x := 0; x < w; x++ {
			if level.GetBiome(x, y) == "tundra" {
				n++
			}
		}
		return float64(n) / w
	}
	// Poles should be dominated by the coldest biome; the equator should have
	// essentially none of it.
	if pole := tundraFrac(0); pole < 0.9 {
		t.Errorf("pole row tundra fraction = %.2f, want > 0.9", pole)
	}
	if eq := tundraFrac(h / 2); eq > 0.1 {
		t.Errorf("equator row tundra fraction = %.2f, want < 0.1", eq)
	}
}
