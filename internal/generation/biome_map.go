package generation

import (
	"math"

	"github.com/aquilax/go-perlin"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// BiomeMapConfig controls how the per-column biome map is built.
type BiomeMapConfig struct {
	// Type: "perlin_temp_humidity" (default) — two noise channels feed a
	// matcher; "uniform" — single biome ID for the whole map.
	Type   string   `json:"type"`
	Scale  float64  `json:"scale"`  // perlin scale (default 200)
	Biomes []string `json:"biomes"` // candidate biome IDs to consider
	Single string   `json:"single"` // for "uniform"
	// LatitudeWeight blends a pole-to-equator temperature gradient into the
	// temperature noise: 0 = pure noise (isotropic blobs), 1 = pure latitude
	// bands (cold at the top/bottom edges, warm in the middle). Humidity stays
	// pure noise. Clamped to [0,1].
	LatitudeWeight float64 `json:"latitude_weight"`
}

// biomeCandidate is a resolved biome plus its climate-space centroid, used by
// the nearest-centroid matcher.
type biomeCandidate struct {
	id                     string
	tMin, tMax, hMin, hMax float64
	ct, ch                 float64 // centroid (temp, humidity)
}

func (c *biomeCandidate) contains(t, hu float64) bool {
	return t >= c.tMin && t <= c.tMax && hu >= c.hMin && hu <= c.hMax
}

// pickBiome chooses a biome for a (temperature, humidity) point. It prefers a
// box that contains the point, resolving overlaps and gaps by nearest centroid
// so the result never depends on candidate list order and uncovered regions
// don't all dump into the first-listed biome:
//   - some box contains the point → nearest centroid among the containing boxes;
//   - no box contains it → nearest centroid among all candidates.
func pickBiome(cands []biomeCandidate, t, hu float64) string {
	containing := false
	for i := range cands {
		if cands[i].contains(t, hu) {
			containing = true
			break
		}
	}
	best := ""
	bestD := math.MaxFloat64
	for i := range cands {
		c := &cands[i]
		if containing && !c.contains(t, hu) {
			continue
		}
		dt, dh := t-c.ct, hu-c.ch
		if d := dt*dt + dh*dh; d < bestD {
			bestD, best = d, c.id
		}
	}
	return best
}

// BuildBiomeMap fills level.BiomeMap based on cfg. If level.BiomeMap is nil
// the level was not allocated for biomes — caller error.
func BuildBiomeMap(level *world.Level, cfg BiomeMapConfig, seed int64) {
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
	pTemp := perlin.NewPerlin(2, 2, 2, seed+7)
	pHum := perlin.NewPerlin(2, 2, 2, seed+13)

	// Resolve candidates and precompute centroids once.
	cands := make([]biomeCandidate, 0, len(cfg.Biomes))
	for _, id := range cfg.Biomes {
		b := GetBiome(id)
		if b == nil {
			continue
		}
		cands = append(cands, biomeCandidate{
			id:   id,
			tMin: b.TempRange[0], tMax: b.TempRange[1],
			hMin: b.HumidityRange[0], hMax: b.HumidityRange[1],
			ct: (b.TempRange[0] + b.TempRange[1]) / 2,
			ch: (b.HumidityRange[0] + b.HumidityRange[1]) / 2,
		})
	}
	if len(cands) == 0 {
		return // no candidates → leave biome map blank; the applier skips it
	}

	latW := cfg.LatitudeWeight
	if latW < 0 {
		latW = 0
	} else if latW > 1 {
		latW = 1
	}
	denom := float64(h - 1)
	if denom < 1 {
		denom = 1
	}

	for y := 0; y < h; y++ {
		// Latitude term: 1 at the equator (vertical middle), 0 at the poles
		// (top/bottom edges). Blended into temperature so cold biomes band
		// toward the poles.
		lat := 1 - math.Abs(2*float64(y)/denom-1)
		for x := 0; x < w; x++ {
			nt := (pTemp.Noise2D(float64(x)/scale, float64(y)/scale) + 1) / 2
			hu := (pHum.Noise2D(float64(x)/scale, float64(y)/scale) + 1) / 2
			t := (1-latW)*nt + latW*lat
			level.SetBiome(x, y, pickBiome(cands, t, hu))
		}
	}
}
