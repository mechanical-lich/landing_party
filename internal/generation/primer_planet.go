package generation

import (
	"log"
	"math"
	"runtime"
	"sync"

	"github.com/aquilax/go-perlin"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// PlanetConfig controls which z-levels map to which semantic bands.
type PlanetConfig struct {
	SurfaceZ    int
	AtmosphereZ int
	SpaceZ      int
}

// DefaultPlanetConfig allocates the semantic z-bands: bedrock at z=0, an
// underground band (rock + caverns) below the surface, the single walkable
// surface near the middle, then a tall "sky" band (air where flat, mountain
// rock where the terrain rises) and a thin space cap at the very top.
//
// The surface is floored at z>=4 so there is always an underground cavern band
// (caverns carve z in [2, surface-2]) and real mining depth, and capped so a
// couple of sky levels remain for atmosphere/mountains. A plain depth/2 split
// used to collapse caverns to zero on shallow maps.
func DefaultPlanetConfig(depth int) PlanetConfig {
	surface := depth / 2
	if surface < 4 {
		surface = 4 // keep an underground cavern band
	}
	if surface > depth-3 {
		surface = depth - 3 // leave sky room above the surface
	}
	if surface < 1 {
		surface = 1
	}
	space := depth - 1
	atmosphere := surface + 1
	if atmosphere > space {
		atmosphere = space
	}
	return PlanetConfig{
		SurfaceZ:    surface,
		AtmosphereZ: atmosphere,
		SpaceZ:      space,
	}
}

// PlanetPrimer paints a layered planet: bedrock at z=0, an underground band with
// caverns, the walkable surface, mountains rising into the sky (solid rock with
// their own interior caverns), atmosphere, and a space cap. Used for Earth-like,
// moon, and alien scenarios — the differences come from biomes, features, and
// the mountain params, not a separate primer.
type PlanetPrimer struct{}

func init() {
	RegisterPrimer("planet", PlanetPrimer{})
}

// mountainHeight returns how many z-levels a mountain rises above the surface at
// (x,y), from a low-frequency 2D noise field: 0 over most of the map, ramping up
// to maxHeight at the peaks where the noise exceeds (1 - frequency). Deterministic
// for a given seed (the perlin source is seeded).
func mountainHeight(pm *perlin.Perlin, x, y int, scale, frequency float64, maxHeight int) int {
	if frequency <= 0 || maxHeight <= 0 || scale <= 0 {
		return 0
	}
	n := (pm.Noise2D(float64(x)/scale, float64(y)/scale) + 1) / 2 // nominal [0,1]
	// Perlin output concentrates near 0.5, so a high threshold would leave the
	// map essentially flat. Spread it around the midpoint (clamped) so
	// `frequency` roughly controls the mountainous fraction of the map.
	m := 0.5 + (n-0.5)*4
	if m < 0 {
		m = 0
	} else if m > 1 {
		m = 1
	}
	thresh := 1 - frequency
	if m <= thresh {
		return 0
	}
	h := int(math.Ceil((m - thresh) / (1 - thresh) * float64(maxHeight)))
	if h > maxHeight {
		h = maxHeight
	}
	return h
}

// Prime params:
//
//	terrain_alpha       float (default 6)
//	terrain_beta        float (default 5)
//	terrain_n           int   (default 2)
//	cavern_density      float (default 0.25) — higher = fewer caves
//	cavern_scale_xy     float (default 40)   — larger = wider chambers
//	cavern_scale_z      float (default 10)   — larger = taller chambers
//	mountain_frequency  float (default 0.15) — fraction of the map that is mountainous (0 = none)
//	mountain_scale      float (default 110)  — larger = broader mountain ranges
func (PlanetPrimer) Prime(level *world.Level, params map[string]any, seed int64) error {
	w, h, d := level.GetWidth(), level.GetHeight(), level.GetDepth()
	level.AllocTerrain()

	cfg := DefaultPlanetConfig(d)
	level.SurfaceZ = cfg.SurfaceZ
	level.AtmosphereZ = cfg.AtmosphereZ
	level.SpaceZ = cfg.SpaceZ

	alpha := paramFloat(params, "terrain_alpha", 6)
	beta := paramFloat(params, "terrain_beta", 5)
	nOct := paramInt(params, "terrain_n", 2)
	cavernThreshold := paramFloat(params, "cavern_density", 0.25)
	cavernScaleXY := paramFloat(params, "cavern_scale_xy", 40)
	cavernScaleZ := paramFloat(params, "cavern_scale_z", 10)
	mountainFreq := paramFloat(params, "mountain_frequency", 0.15)
	mountainScale := paramFloat(params, "mountain_scale", 110)

	// Tallest peak leaves the top level as a clear space cap.
	maxMountain := d - 2 - cfg.SurfaceZ
	if maxMountain < 0 {
		maxMountain = 0
	}

	pc := perlin.NewPerlin(alpha, beta, int32(nOct), seed+1) // caverns (3D)
	pm := perlin.NewPerlin(alpha, beta, int32(nOct), seed+2) // mountain height (2D)

	const chunkSize = 64
	type chunk struct{ z, xStart, xEnd int }

	var wg sync.WaitGroup
	chunkChan := make(chan chunk, 32)

	// Each cell is written exactly once (mountain height is 2D, caverns are 3D
	// noise — no cell reads a neighbour), so workers need no locking and z-order
	// doesn't matter.
	worker := func() {
		for c := range chunkChan {
			for x := c.xStart; x < c.xEnd; x++ {
				for y := 0; y < h; y++ {
					mh := mountainHeight(pm, x, y, mountainScale, mountainFreq, maxMountain)
					summit := cfg.SurfaceZ + mh
					cavern := pc.Noise3D(float64(x)/cavernScaleXY, float64(y)/cavernScaleXY, float64(c.z)/cavernScaleZ)

					switch {
					case c.z == 0:
						paintKind(level, x, y, c.z, world.TKBedrock)

					case c.z < cfg.SurfaceZ:
						switch {
						case c.z > 1 && c.z < cfg.SurfaceZ-1 && cavern > cavernThreshold:
							paintKind(level, x, y, c.z, world.TKCavern)
						case c.z >= cfg.SurfaceZ-3:
							paintKind(level, x, y, c.z, world.TKSubsurface)
						default:
							paintKind(level, x, y, c.z, world.TKUnderground)
						}

					case c.z == cfg.SurfaceZ:
						if mh > 0 {
							// A mountain occupies the surface level — solid base.
							paintKind(level, x, y, c.z, world.TKUnderground)
						} else {
							paintKind(level, x, y, c.z, world.TKSurface)
						}
						level.SetSurfaceZ(x, y, cfg.SurfaceZ)

					case c.z <= summit:
						// Inside a mountain. Carve enclosed caverns between the
						// base and summit; the outer shell stays solid.
						if c.z > cfg.SurfaceZ+1 && c.z < summit && cavern > cavernThreshold {
							paintKind(level, x, y, c.z, world.TKCavern)
						} else {
							paintKind(level, x, y, c.z, world.TKUnderground)
						}

					case c.z >= cfg.SpaceZ:
						paintKind(level, x, y, c.z, world.TKSpace)

					default:
						paintKind(level, x, y, c.z, world.TKAtmosphere)
					}
				}
			}
			wg.Done()
		}
	}

	numWorkers := runtime.NumCPU() - 1
	if numWorkers < 1 {
		numWorkers = 1
	}
	for i := 0; i < numWorkers; i++ {
		go worker()
	}
	for z := 0; z < d; z++ {
		for xStart := 0; xStart < w; xStart += chunkSize {
			xEnd := xStart + chunkSize
			if xEnd > w {
				xEnd = w
			}
			wg.Add(1)
			chunkChan <- chunk{z: z, xStart: xStart, xEnd: xEnd}
		}
	}
	wg.Wait()
	close(chunkChan)

	log.Printf("PlanetPrimer: %dx%dx%d done (surfaceZ=%d, maxMountain=%d)", w, h, d, cfg.SurfaceZ, maxMountain)
	return nil
}
