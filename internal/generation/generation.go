package generation

import (
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/aquilax/go-perlin"
	"github.com/mechanical-lich/mlge/utility"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

const (
	alpha = 6.
	beta  = 5.
	n     = 2
)

// PlanetConfig controls which z-levels map to which semantic bands.
// These are written into Level so the rest of the game can query them.
type PlanetConfig struct {
	SurfaceZ    int // the main ground layer
	AtmosphereZ int // first air layer above surface
	SpaceZ      int // first space/vacuum layer
}

// DefaultPlanetConfig returns sensible defaults matching config.json startingZ=5, depth=10.
func DefaultPlanetConfig(depth int) PlanetConfig {
	surface := depth / 2
	return PlanetConfig{
		SurfaceZ:    surface,
		AtmosphereZ: surface + 1,
		SpaceZ:      surface + 3,
	}
}

// NewPlanetLevel generates a layered 3-D world:
//
//	z < surfaceZ          — underground (bedrock at 0, rock/ore above)
//	z == surfaceZ         — planet surface (regolith, alien flora, water, ice)
//	surfaceZ < z < spaceZ — atmosphere (air tiles)
//	z >= spaceZ           — space/void (space tiles)
func NewPlanetLevel(width, height, depth int, cfg PlanetConfig) *world.Level {
	log.Printf("Generating planet level %dx%dx%d", width, height, depth)

	level := world.NewLevel(width, height, depth)
	level.SurfaceZ = cfg.SurfaceZ
	level.AtmosphereZ = cfg.AtmosphereZ
	level.SpaceZ = cfg.SpaceZ

	p := perlin.NewPerlin(alpha, beta, n, time.Now().UnixNano())

	const chunkSize = 64

	type chunk struct {
		z, xStart, xEnd int
	}

	var wg sync.WaitGroup
	chunkChan := make(chan chunk, 32)

	worker := func() {
		for c := range chunkChan {
			for x := c.xStart; x < c.xEnd; x++ {
				for y := 0; y < height; y++ {
					noise := p.Noise3D(float64(x)/80, float64(y)/80, float64(c.z)/8)
					value := int(noise * 10)

					switch {
					case c.z >= cfg.SpaceZ:
						level.UpdateTileAt(x, y, c.z, "space", 0)

					case c.z >= cfg.AtmosphereZ:
						level.UpdateTileAt(x, y, c.z, "air", 0)

					case c.z == cfg.SurfaceZ:
						placeSurfaceTile(level, p, x, y, c.z, value)

					case c.z == 0:
						level.UpdateTileAt(x, y, c.z, "bedrock", 0)

					default:
						placeUndergroundTile(level, x, y, c.z, value)
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

	for z := 0; z < depth; z++ {
		for xStart := 0; xStart < width; xStart += chunkSize {
			xEnd := xStart + chunkSize
			if xEnd > width {
				xEnd = width
			}
			wg.Add(1)
			chunkChan <- chunk{z: z, xStart: xStart, xEnd: xEnd}
		}
	}
	wg.Wait()
	close(chunkChan)

	//placeScatter(level, width, height, cfg.SurfaceZ)

	log.Println("Planet generation complete")
	return level
}

func placeSurfaceTile(level *world.Level, p *perlin.Perlin, x, y, z, value int) {
	// Second noise pass at larger scale for biome blending
	biome := p.Noise3D(float64(x)/200, float64(y)/200, 0)

	switch {
	case biome > 0.3:
		// Lush alien biome — grass/flora
		if value >= 1 {
			level.UpdateTileAt(x, y, z, "rock", world.RandomTileVariant("rock"))
		} else {
			level.UpdateTileAt(x, y, z, "grass", world.RandomTileVariant("grass"))
		}
	case biome < -0.3:
		// Ice/frozen biome
		if value >= 1 {
			level.UpdateTileAt(x, y, z, "rock", world.RandomTileVariant("rock"))
		} else {
			level.UpdateTileAt(x, y, z, "ice", 0)
		}
	default:
		// Arid/regolith default
		if value >= 2 {
			level.UpdateTileAt(x, y, z, "rock", world.RandomTileVariant("rock"))
		} else {
			level.UpdateTileAt(x, y, z, "regolith", world.RandomTileVariant("regolith"))
		}
	}
}

func placeUndergroundTile(level *world.Level, x, y, z, value int) {
	switch {
	case value >= 4:
		// Rare ore pockets
		level.UpdateTileAt(x, y, z, "ore_deposit", 0)
	case value >= 3:
		// Crystal veins
		level.UpdateTileAt(x, y, z, "crystal_vein", 0)
	default:
		level.UpdateTileAt(x, y, z, "rock", world.RandomTileVariant("rock"))
	}
}

// placeScatter adds sparse surface details after the main pass.
func placeScatter(level *world.Level, width, height, surfaceZ int) {
	// Alien flora clusters on grass tiles
	for i := 0; i < 200; i++ {
		x := utility.GetRandom(1, width-1)
		y := utility.GetRandom(1, height-1)
		tile := level.GetTilePtr(x, y, surfaceZ)
		if tile != nil && world.TileDefinitions[tile.Type].Name == "grass" {
			scatterCluster(level, x, y, surfaceZ, "alien_flora", 6)
		}
	}
}

func scatterCluster(level *world.Level, cx, cy, z int, tileName string, radius int) {
	for dx := -radius; dx <= radius; dx++ {
		for dy := -radius; dy <= radius; dy++ {
			if dx*dx+dy*dy <= radius*radius {
				if utility.GetRandom(0, 3) == 0 {
					level.UpdateTileAt(cx+dx, cy+dy, z, tileName, world.RandomTileVariant(tileName))
				}
			}
		}
	}
}
