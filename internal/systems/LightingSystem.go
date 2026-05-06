package systems

import (
	"log"
	"runtime"
	"sync"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

// isSunBlocking returns true only for tiles that physically block sunlight —
// i.e. not air and not space (space is open sky, not a solid surface).
func isSunBlocking(ti interface{}) bool {
	tile, ok := ti.(*world.Tile)
	if !ok || tile == nil {
		return false
	}
	if tile.IsAir() {
		return false
	}
	def := world.TileDefinitions[tile.Type]
	return !def.Space
}

const (
	lightDecayFrames = 10
	lightDecayAmount = 5
	shadowRadius     = 3
)

type tileLightSource struct {
	tile       *world.Tile
	x, y, z    int
	level, rng int
}

type LightingSystem struct {
	litTiles     map[*world.Tile]bool
	frameCounter int
	sunTiles     []*world.Tile
	sunShadow    []int
	heightMap    []int
	cacheBuilt   bool
	tileSources  []tileLightSource
}

var lightingRequires = []ecs.ComponentType{rlcomponents.Light}

func (s *LightingSystem) Requires() []ecs.ComponentType { return lightingRequires }

func (s *LightingSystem) UpdateSystem(data interface{}) error {
	level := data.(*world.Level)

	if s.litTiles == nil {
		s.litTiles = make(map[*world.Tile]bool)
	}

	s.frameCounter++
	if s.frameCounter >= lightDecayFrames {
		s.frameCounter = 0
		s.decayLitTiles()
	}

	sunIntensity := level.EffectiveSunIntensity()

	if !s.cacheBuilt {
		s.buildFullCache(level)
		level.DirtyColumns = level.DirtyColumns[:0]
	}

	if len(level.DirtyColumns) > 0 {
		s.updateDirtyColumns(level)
		level.DirtyColumns = level.DirtyColumns[:0]
	}

	mapSize := level.Width * level.Height
	for i := 0; i < mapSize; i++ {
		tile := s.sunTiles[i]
		if tile == nil {
			continue
		}
		shadow := s.sunShadow[i]
		effective := sunIntensity - shadow
		if effective < 0 {
			effective = 0
		}
		tile.LightLevel = effective
	}

	s.applyTileLights(level)

	return nil
}

func (s *LightingSystem) buildTileSourceCache(level *world.Level) {
	s.tileSources = s.tileSources[:0]
	for z := 0; z < level.Depth; z++ {
		for y := 0; y < level.Height; y++ {
			for x := 0; x < level.Width; x++ {
				ti := level.GetTileAt(x, y, z)
				if ti == nil {
					continue
				}
				tile := ti.(*world.Tile)
				def := world.TileDefinitions[tile.Type]
				if def.LightLevel <= 0 {
					continue
				}
				r := def.LightRange
				if r <= 0 {
					r = 3
				}
				s.tileSources = append(s.tileSources, tileLightSource{
					tile: tile, x: x, y: y, z: z,
					level: def.LightLevel, rng: r,
				})
			}
		}
	}
}

func (s *LightingSystem) applyTileLights(level *world.Level) {
	if s.litTiles == nil {
		s.litTiles = make(map[*world.Tile]bool)
	}
	for _, src := range s.tileSources {
		for dx := -src.rng; dx <= src.rng; dx++ {
			for dy := -src.rng; dy <= src.rng; dy++ {
				neighbor := level.GetTileAt(src.x+dx, src.y+dy, src.z)
				if neighbor == nil {
					continue
				}
				nt := neighbor.(*world.Tile)
				dist := dx*dx + dy*dy
				lightLevel := src.level - dist
				if lightLevel > 0 && lightLevel > nt.LightLevel {
					nt.LightLevel = lightLevel
					s.litTiles[nt] = true
				}
			}
		}
	}
}

func (s *LightingSystem) buildFullCache(level *world.Level) {
	mapSize := level.Width * level.Height
	s.sunTiles = make([]*world.Tile, mapSize)
	s.sunShadow = make([]int, mapSize)
	s.heightMap = make([]int, mapSize)

	sunIntensity := level.EffectiveSunIntensity()
	numWorkers := runtime.NumCPU()
	chunkSize := (mapSize + numWorkers - 1) / numWorkers
	log.Println("LightingSystem: building sun cache with", numWorkers, "workers")

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > mapSize {
			end = mapSize
		}
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for i := start; i < end; i++ {
				s.heightMap[i] = -1
				x := i % level.Width
				y := i / level.Width
				for z := level.Depth - 1; z >= 0; z-- {
					ti := level.GetTileAt(x, y, z)
					if isSunBlocking(ti) {
						tile := ti.(*world.Tile)
						s.sunTiles[i] = tile
						s.heightMap[i] = z
						tile.LightLevel = sunIntensity
						break
					}
				}
			}
		}(start, end)
	}
	wg.Wait()

	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > mapSize {
			end = mapSize
		}
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for i := start; i < end; i++ {
				s.recalcColumnShadow(level, i)
			}
		}(start, end)
	}
	wg.Wait()

	log.Println("LightingSystem: sun cache built")
	s.buildTileSourceCache(level)
	s.cacheBuilt = true
}

func (s *LightingSystem) updateDirtyColumns(level *world.Level) {
	dirty := level.DirtyColumns
	numDirty := len(dirty)
	numWorkers := runtime.NumCPU()
	chunkSize := (numDirty + numWorkers - 1) / numWorkers

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > numDirty {
			end = numDirty
		}
		if start >= end {
			break
		}
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for _, idx := range dirty[start:end] {
				x := idx % level.Width
				y := idx / level.Width
				s.heightMap[idx] = -1
				s.sunTiles[idx] = nil
				for z := level.Depth - 1; z >= 0; z-- {
					ti := level.GetTileAt(x, y, z)
					if isSunBlocking(ti) {
						tile := ti.(*world.Tile)
						s.sunTiles[idx] = tile
						s.heightMap[idx] = z
						break
					}
				}
			}
		}(start, end)
	}
	wg.Wait()

	recalc := make(map[int]bool, numDirty*((2*shadowRadius+1)*(2*shadowRadius+1)))
	for _, idx := range dirty {
		x := idx % level.Width
		y := idx / level.Width
		for dx := -shadowRadius; dx <= shadowRadius; dx++ {
			for dy := -shadowRadius; dy <= shadowRadius; dy++ {
				nx, ny := x+dx, y+dy
				if nx >= 0 && ny >= 0 && nx < level.Width && ny < level.Height {
					recalc[ny*level.Width+nx] = true
				}
			}
		}
	}

	recalcList := make([]int, 0, len(recalc))
	for idx := range recalc {
		recalcList = append(recalcList, idx)
	}
	numRecalc := len(recalcList)
	chunkSize = (numRecalc + numWorkers - 1) / numWorkers

	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > numRecalc {
			end = numRecalc
		}
		if start >= end {
			break
		}
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for _, idx := range recalcList[start:end] {
				s.recalcColumnShadow(level, idx)
			}
		}(start, end)
	}
	wg.Wait()
	s.buildTileSourceCache(level)
}

func (s *LightingSystem) recalcColumnShadow(level *world.Level, idx int) {
	if s.heightMap[idx] < 0 {
		s.sunShadow[idx] = 0
		return
	}
	x := idx % level.Width
	y := idx / level.Width
	z := s.heightMap[idx]

	maxShadow := 0
	for dx := -shadowRadius; dx <= shadowRadius; dx++ {
		for dy := -shadowRadius; dy <= shadowRadius; dy++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			if nx < 0 || ny < 0 || nx >= level.Width || ny >= level.Height {
				continue
			}
			neighborHeight := s.heightMap[ny*level.Width+nx]
			if neighborHeight <= z {
				continue
			}
			heightDiff := neighborHeight - z
			dist := dx
			if dist < 0 {
				dist = -dist
			}
			if dy < 0 && -dy > dist {
				dist = -dy
			} else if dy > dist {
				dist = dy
			}
			shadow := heightDiff * 20 / dist
			if shadow > 60 {
				shadow = 60
			}
			if shadow > maxShadow {
				maxShadow = shadow
			}
		}
	}
	s.sunShadow[idx] = maxShadow
}

func (s *LightingSystem) decayLitTiles() {
	for tile := range s.litTiles {
		tile.LightLevel -= lightDecayAmount
		if tile.LightLevel <= 0 {
			tile.LightLevel = 0
			delete(s.litTiles, tile)
		}
	}
}

func (s *LightingSystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	level := levelInterface.(*world.Level)

	if s.litTiles == nil {
		s.litTiles = make(map[*world.Tile]bool)
	}

	lc := entity.GetComponent(rlcomponents.Light).(*rlcomponents.LightComponent)
	if lc.Level <= 0 {
		return nil
	}

	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	x, y, z := pc.GetX(), pc.GetY(), pc.GetZ()

	for dx := -lc.Range; dx <= lc.Range; dx++ {
		for dy := -lc.Range; dy <= lc.Range; dy++ {
			ti := level.GetTileAt(x+dx, y+dy, z)
			if ti == nil {
				continue
			}
			tile := ti.(*world.Tile)
			dist := dx*dx + dy*dy
			lightLevel := lc.Level - dist
			if lightLevel > 0 && lightLevel > tile.LightLevel {
				tile.LightLevel = lightLevel
				s.litTiles[tile] = true
			}
		}
	}
	return nil
}
