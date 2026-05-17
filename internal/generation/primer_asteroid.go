package generation

import (
	"log"
	"math/rand"
	"sort"

	"github.com/aquilax/go-perlin"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// AsteroidFieldPrimer fills the level with vacuum, then carves N asteroids
// out of Perlin noise across a single Z slab. The largest asteroid is tagged
// "starting_asteroid" so scenarios can land colonists there. Each asteroid
// also gets its own region tag "asteroid_<i>" at its centroid.
type AsteroidFieldPrimer struct{}

func init() {
	RegisterPrimer("asteroid_field", AsteroidFieldPrimer{})
}

// Prime params:
//
//	threshold       float (default 0.15) — noise > threshold = solid
//	min_size        int   (default 40)   — drop components smaller than this
//	noise_scale     float (default 22)   — smaller = chunkier asteroids
func (AsteroidFieldPrimer) Prime(level *world.Level, params map[string]any, seed int64) error {
	w, h, d := level.GetWidth(), level.GetHeight(), level.GetDepth()
	level.AllocTerrain()

	// Asteroid field is essentially flat: one surface slab in the middle.
	cfg := DefaultPlanetConfig(d)
	level.SurfaceZ = cfg.SurfaceZ
	level.AtmosphereZ = cfg.SurfaceZ + 1
	level.SpaceZ = cfg.SurfaceZ + 1 // no atmosphere; everything above surface is space

	threshold := paramFloat(params, "threshold", 0.15)
	minSize := paramInt(params, "min_size", 40)
	scale := paramFloat(params, "noise_scale", 22)

	p := perlin.NewPerlin(4, 4, 2, seed)

	// Build solid mask for the surface slab.
	mask := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			n := p.Noise2D(float64(x)/scale, float64(y)/scale)
			if n > threshold {
				mask[y*w+x] = true
			}
		}
	}

	// Connected-component label.
	labels := make([]int, w*h)
	sizes := []int{0} // labels 1..N
	centroids := [][2]int{{0, 0}}
	nextLabel := 1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !mask[y*w+x] || labels[y*w+x] != 0 {
				continue
			}
			// BFS
			queue := [][2]int{{x, y}}
			labels[y*w+x] = nextLabel
			size := 0
			cx, cy := 0, 0
			for len(queue) > 0 {
				p := queue[0]
				queue = queue[1:]
				size++
				cx += p[0]
				cy += p[1]
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := p[0]+d[0], p[1]+d[1]
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					idx := ny*w + nx
					if mask[idx] && labels[idx] == 0 {
						labels[idx] = nextLabel
						queue = append(queue, [2]int{nx, ny})
					}
				}
			}
			sizes = append(sizes, size)
			if size > 0 {
				centroids = append(centroids, [2]int{cx / size, cy / size})
			} else {
				centroids = append(centroids, [2]int{x, y})
			}
			nextLabel++
		}
	}

	// Drop components below min_size.
	keep := make(map[int]bool)
	for i := 1; i < len(sizes); i++ {
		if sizes[i] >= minSize {
			keep[i] = true
		}
	}
	if len(keep) == 0 {
		// Fall back: keep largest component so we have at least one asteroid.
		biggest := 1
		for i := 2; i < len(sizes); i++ {
			if sizes[i] > sizes[biggest] {
				biggest = i
			}
		}
		keep[biggest] = true
	}

	// Paint the level.
	for z := 0; z < d; z++ {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				lab := labels[y*w+x]
				inAsteroid := lab != 0 && keep[lab]
				switch {
				case z == cfg.SurfaceZ && inAsteroid:
					paintKind(level, x, y, z, world.TKSurface)
					level.SetSurfaceZ(x, y, z)
				case z == 0 && inAsteroid:
					paintKind(level, x, y, z, world.TKBedrock)
				case z < cfg.SurfaceZ && inAsteroid:
					if z >= cfg.SurfaceZ-2 {
						paintKind(level, x, y, z, world.TKSubsurface)
					} else {
						paintKind(level, x, y, z, world.TKUnderground)
					}
				default:
					paintKind(level, x, y, z, world.TKSpace)
				}
			}
		}
	}

	// Tag asteroids: largest = starting_asteroid, others = asteroid_<rank>.
	type ast struct {
		label int
		size  int
	}
	asts := []ast{}
	for lab := range keep {
		asts = append(asts, ast{lab, sizes[lab]})
	}
	sort.Slice(asts, func(i, j int) bool { return asts[i].size > asts[j].size })
	for i, a := range asts {
		c := centroids[a.label]
		if i == 0 {
			level.TagRegion("starting_asteroid", c[0], c[1], cfg.SurfaceZ)
		}
		level.TagRegion("asteroid", c[0], c[1], cfg.SurfaceZ)
	}

	log.Printf("AsteroidFieldPrimer: %d asteroids kept (of %d candidates)", len(asts), len(sizes)-1)
	_ = rand.Int // reserved for future jitter
	return nil
}
