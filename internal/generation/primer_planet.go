package generation

import (
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/aquilax/go-perlin"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

// PlanetPrimer paints a layered planet: bedrock at z=0, underground rock with
// caverns, a single surface band, atmosphere, and space at the top. Used for
// Earth-like, moon, and alien scenarios — the differences come from biomes
// and features, not the terrain primer.
type PlanetPrimer struct{}

func init() {
	RegisterPrimer("planet", PlanetPrimer{})
}

// Prime params:
//
//	terrain_alpha   float (default 6)
//	terrain_beta    float (default 5)
//	terrain_n       int   (default 2)
//	cavern_density  float (default 0.25) — higher = fewer caves
//	cavern_scale_xy float (default 40)   — larger = wider chambers
//	cavern_scale_z  float (default 10)   — larger = taller chambers (more vertical connectivity)
func (PlanetPrimer) Prime(level *world.Level, params map[string]any) error {
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

	p := perlin.NewPerlin(alpha, beta, int32(nOct), time.Now().UnixNano())
	pc := perlin.NewPerlin(alpha, beta, int32(nOct), time.Now().UnixNano()+1)

	const chunkSize = 64
	type chunk struct{ z, xStart, xEnd int }

	var wg sync.WaitGroup
	chunkChan := make(chan chunk, 32)

	worker := func() {
		for c := range chunkChan {
			for x := c.xStart; x < c.xEnd; x++ {
				for y := 0; y < h; y++ {
					terrain := p.Noise3D(float64(x)/80, float64(y)/80, float64(c.z)/8)
					value := int(terrain * 10)
					cavern := pc.Noise3D(float64(x)/cavernScaleXY, float64(y)/cavernScaleXY, float64(c.z)/cavernScaleZ)

					switch {
					case c.z >= cfg.SpaceZ:
						paintKind(level, x, y, c.z, world.TKSpace)

					case c.z >= cfg.AtmosphereZ:
						if value >= 2 {
							belowKind := level.GetTerrainKind(x, y, c.z-1)
							if belowKind == world.TKSurface || belowKind == world.TKUnderground || belowKind == world.TKSubsurface {
								paintKind(level, x, y, c.z, world.TKUnderground)
								// Anchor the mountain at surface level so the
								// camera-z=SurfaceZ view actually shows the
								// rock obstacle instead of grass with a
								// disconnected rock floating one z above.
								if belowKind == world.TKSurface {
									paintKind(level, x, y, c.z-1, world.TKUnderground)
									level.SetSurfaceZ(x, y, c.z-1)
								}
								continue
							}
						}
						paintKind(level, x, y, c.z, world.TKAtmosphere)

					case c.z == cfg.SurfaceZ:
						paintKind(level, x, y, c.z, world.TKSurface)
						level.SetSurfaceZ(x, y, c.z)

					case c.z == 0:
						paintKind(level, x, y, c.z, world.TKBedrock)

					default:
						if c.z > 1 && c.z < cfg.SurfaceZ-1 && cavern > cavernThreshold {
							paintKind(level, x, y, c.z, world.TKCavern)
						} else if c.z >= cfg.SurfaceZ-3 {
							paintKind(level, x, y, c.z, world.TKSubsurface)
						} else {
							paintKind(level, x, y, c.z, world.TKUnderground)
						}
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

	// Generate atmosphere/space LAST so cliffs can read the surface kind
	// from below. Order: bedrock + underground first, then surface, then
	// atmosphere+space.
	dispatch := func(z int) {
		for xStart := 0; xStart < w; xStart += chunkSize {
			xEnd := xStart + chunkSize
			if xEnd > w {
				xEnd = w
			}
			wg.Add(1)
			chunkChan <- chunk{z: z, xStart: xStart, xEnd: xEnd}
		}
	}
	for z := 0; z < cfg.SurfaceZ; z++ {
		dispatch(z)
	}
	wg.Wait()
	dispatch(cfg.SurfaceZ)
	wg.Wait()
	// Atmosphere/cliff phase: each z's cliff check reads belowKind at z-1,
	// so we must serialize by z. Without the per-z wait, upper cliff cells
	// race against lower ones and get painted as TKAtmosphere (no Floor),
	// producing rendering glitches at mountain tops.
	for z := cfg.SurfaceZ + 1; z < d; z++ {
		dispatch(z)
		wg.Wait()
	}
	close(chunkChan)

	log.Printf("PlanetPrimer: %dx%dx%d done", w, h, d)
	return nil
}
