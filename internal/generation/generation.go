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
type PlanetConfig struct {
	SurfaceZ    int
	AtmosphereZ int
	SpaceZ      int
}

func DefaultPlanetConfig(depth int) PlanetConfig {
	surface := depth / 2
	return PlanetConfig{
		SurfaceZ:    surface,
		AtmosphereZ: surface + 1,
		SpaceZ:      surface + 3,
	}
}

func NewPlanetLevel(width, height, depth int, cfg PlanetConfig) *world.Level {
	log.Printf("Generating planet level %dx%dx%d", width, height, depth)

	level := world.NewLevel(width, height, depth)
	level.SurfaceZ = cfg.SurfaceZ
	level.AtmosphereZ = cfg.AtmosphereZ
	level.SpaceZ = cfg.SpaceZ

	// Two noise generators: one for terrain shape, one for caverns
	p := perlin.NewPerlin(alpha, beta, n, time.Now().UnixNano())
	pc := perlin.NewPerlin(alpha, beta, n, time.Now().UnixNano()+1)

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
					// Primary terrain noise
					terrain := p.Noise3D(float64(x)/80, float64(y)/80, float64(c.z)/8)
					value := int(terrain * 10)

					// Cavern noise — different scale for organic cave shapes
					cavern := pc.Noise3D(float64(x)/40, float64(y)/40, float64(c.z)/6)

					switch {
					case c.z >= cfg.SpaceZ:
						level.UpdateTileAt(x, y, c.z, "space", 0)

					case c.z >= cfg.AtmosphereZ:
						// Rocky outcrops: push solid rock into the atmosphere band where
						// terrain is high and the tile directly below is also solid.
						// This creates cliffs and ridges that cast shadows.
						if value >= 2 {
							belowTile := level.GetTileAt(x, y, c.z-1)
							if belowTile != nil && !belowTile.IsAir() {
								level.UpdateTileAt(x, y, c.z, "rock", world.RandomTileVariant("rock"))
							} else {
								level.UpdateTileAt(x, y, c.z, "air", 0)
							}
						} else {
							level.UpdateTileAt(x, y, c.z, "air", 0)
						}

					case c.z == cfg.SurfaceZ:
						placeSurfaceTile(level, p, x, y, c.z, value)

					case c.z == 0:
						level.UpdateTileAt(x, y, c.z, "bedrock", 0)

					default:
						placeUndergroundTile(level, x, y, c.z, value, cavern, cfg)
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

	log.Println("Planet generation complete")
	return level
}

func placeSurfaceTile(level *world.Level, p *perlin.Perlin, x, y, z, value int) {
	biome := p.Noise3D(float64(x)/200, float64(y)/200, 0)

	switch {
	case biome > 0.3:
		if value >= 1 {
			level.UpdateTileAt(x, y, z, "rock", world.RandomTileVariant("rock"))
		} else {
			level.UpdateTileAt(x, y, z, "grass", world.RandomTileVariant("grass"))
		}
	case biome < -0.3:
		if value >= 1 {
			level.UpdateTileAt(x, y, z, "rock", world.RandomTileVariant("rock"))
		} else {
			level.UpdateTileAt(x, y, z, "ice", 0)
		}
	default:
		if value >= 2 {
			level.UpdateTileAt(x, y, z, "rock", world.RandomTileVariant("rock"))
		} else {
			level.UpdateTileAt(x, y, z, "regolith", world.RandomTileVariant("regolith"))
		}
	}
}

func placeUndergroundTile(level *world.Level, x, y, z, value int, cavern float64, cfg PlanetConfig) {
	// Carve caverns: open air pockets in mid-underground levels.
	// Keep z==1 solid (just above bedrock) so caverns don't punch through the floor.
	if z > 1 && z < cfg.SurfaceZ-1 && cavern > 0.35 {
		level.UpdateTileAt(x, y, z, "air", 0)
		return
	}

	switch {
	case value >= 4:
		level.UpdateTileAt(x, y, z, "ore_deposit", 0)
	case value >= 3:
		level.UpdateTileAt(x, y, z, "crystal_vein", 0)
	default:
		level.UpdateTileAt(x, y, z, "rock", world.RandomTileVariant("rock"))
	}
}

// placeScatter adds sparse surface details after the main pass.
func placeScatter(level *world.Level, width, height, surfaceZ int) {
	for i := 0; i < 200; i++ {
		x := utility.GetRandom(1, width-1)
		y := utility.GetRandom(1, height-1)
		tile := level.GetTilePtr(x, y, surfaceZ)
		if tile != nil && !tile.Floor.IsEmpty() && world.TileDefinitions[tile.Floor.Type].Name == "grass" {
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
