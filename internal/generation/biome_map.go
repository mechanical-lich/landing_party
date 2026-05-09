package generation

import (
	"time"

	"github.com/aquilax/go-perlin"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

// BiomeMapConfig controls how the per-column biome map is built.
type BiomeMapConfig struct {
	// Type: "perlin_temp_humidity" (default) — two noise channels feed a
	// matcher; "uniform" — single biome ID for the whole map.
	Type   string  `json:"type"`
	Scale  float64 `json:"scale"`  // perlin scale (default 200)
	Biomes []string `json:"biomes"` // candidate biome IDs to consider
	Single string   `json:"single"` // for "uniform"
}

// BuildBiomeMap fills level.BiomeMap based on cfg. If level.BiomeMap is nil
// the level was not allocated for biomes — caller error.
func BuildBiomeMap(level *world.Level, cfg BiomeMapConfig) {
	if level.BiomeMap == nil {
		return
	}
	w, h := level.GetWidth(), level.GetHeight()

	switch cfg.Type {
	case "uniform":
		for i := range level.BiomeMap {
			level.BiomeMap[i] = cfg.Single
		}
		return
	default: // perlin_temp_humidity
	}

	scale := cfg.Scale
	if scale <= 0 {
		scale = 200
	}
	pTemp := perlin.NewPerlin(2, 2, 2, time.Now().UnixNano())
	pHum := perlin.NewPerlin(2, 2, 2, time.Now().UnixNano()+13)

	candidates := cfg.Biomes
	if len(candidates) == 0 {
		// no candidates → leave biome map blank; biome applier will skip
		return
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := (pTemp.Noise2D(float64(x)/scale, float64(y)/scale) + 1) / 2
			hu := (pHum.Noise2D(float64(x)/scale, float64(y)/scale) + 1) / 2
			best := ""
			for _, id := range candidates {
				b := GetBiome(id)
				if b == nil {
					continue
				}
				if t >= b.TempRange[0] && t <= b.TempRange[1] &&
					hu >= b.HumidityRange[0] && hu <= b.HumidityRange[1] {
					best = id
					break
				}
			}
			if best == "" {
				best = candidates[0] // fallback to first listed
			}
			level.SetBiome(x, y, best)
		}
	}
}
