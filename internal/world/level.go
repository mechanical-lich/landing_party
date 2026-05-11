package world

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rllayered"
	"github.com/mechanical-lich/mlge/ecs"
)

// ZBand describes the semantic role of a z-level range.
type ZBand int

const (
	ZBandUnderground ZBand = iota
	ZBandSurface
	ZBandAtmosphere
	ZBandSpace
)

// LightMode controls how ambient/sun light is computed.
const (
	LightModedayNight  = "day_night"  // hour-based sun cycle (default)
	LightModeFixed     = "fixed"      // constant ambient level
	LightModePitchDark = "pitch_dark" // always 0
)

// TerrainKind labels each tile's role in the terrain skeleton produced by a
// TerrainPrimer. Biome rules use this to decide which concrete tile to paint.
type TerrainKind uint8

const (
	TKVoid        TerrainKind = iota // outside the playable region (e.g. between asteroids)
	TKSpace                          // vacuum
	TKAtmosphere                     // open air above surface
	TKSurface                        // exposed solid at top of column
	TKSubsurface                     // 1-3 tiles below surface (top soil)
	TKUnderground                    // deep solid
	TKCavern                         // carved underground air pocket
	TKBedrock                        // impassable floor
	TKWater                          // surface or sub-surface water
	TKStructure                      // tile placed by station/blueprint primer; biome rules skip
)

// Level embeds rllayered.Level and adds rendering state and z-band metadata.
type Level struct {
	*rllayered.Level

	Flags map[string]any

	// Z-band boundaries (inclusive). Set during generation.
	SurfaceZ     int // the "ground" z-level
	AtmosphereZ  int // first z-level of air above surface
	SpaceZ       int // first z-level of vacuum/space

	// Terrain skeleton produced by a TerrainPrimer. Indexed by (z*H + y)*W + x.
	// Empty until a primer fills it.
	Terrain []TerrainKind

	// Per-column biome label (indexed by y*W + x). Empty until biomes applied.
	BiomeMap []string

	// Per-column surface Z. -1 if column has no surface (pure space, between asteroids).
	SurfaceMap []int16

	// Region tags emitted by primers / scripts (e.g. "starting_asteroid").
	// Maps tag → list of (x,y,z) anchor points.
	Regions map[string][][3]int

	// Lighting config — set from scenario at level creation.
	LightMode     string // "day_night", "fixed", or "pitch_dark"
	FixedAmbient  int    // used when LightMode == "fixed"

	op             *ebiten.DrawImageOptions
	entitiesBuffer []*ecs.Entity
	lightOverlay   *ebiten.Image
	lightPixels    []byte
	lightW, lightH int
}

// EffectiveSunIntensity returns the ambient light level for the current frame.
func (l *Level) EffectiveSunIntensity() int {
	switch l.LightMode {
	case LightModeFixed:
		return l.FixedAmbient
	case LightModePitchDark:
		return 0
	default: // "day_night" or unset
		return l.SunIntensity()
	}
}

func NewLevel(width, height, depth int) *Level {
	base := rllayered.NewLevel(width, height, depth)
	level := &Level{
		Level:          base,
		Flags:          make(map[string]any),
		Regions:        make(map[string][][3]int),
		op:             &ebiten.DrawImageOptions{},
		entitiesBuffer: make([]*ecs.Entity, 0, 16),
	}
	base.PathCostFunc = getPathCostFunction(level)
	return level
}

// ZBandOf returns the semantic band for a given z-level.
func (level *Level) ZBandOf(z int) ZBand {
	switch {
	case z >= level.SpaceZ:
		return ZBandSpace
	case z >= level.AtmosphereZ:
		return ZBandAtmosphere
	case z == level.SurfaceZ:
		return ZBandSurface
	default:
		return ZBandUnderground
	}
}

// AllocTerrain (re)allocates the terrain skeleton + biome/surface maps.
// Call once before a TerrainPrimer fills them.
func (l *Level) AllocTerrain() {
	w, h, d := l.GetWidth(), l.GetHeight(), l.GetDepth()
	l.Terrain = make([]TerrainKind, w*h*d)
	l.BiomeMap = make([]string, w*h)
	l.SurfaceMap = make([]int16, w*h)
	for i := range l.SurfaceMap {
		l.SurfaceMap[i] = -1
	}
}

func (l *Level) terrainIdx(x, y, z int) int {
	w, h := l.GetWidth(), l.GetHeight()
	return (z*h+y)*w + x
}

func (l *Level) GetTerrainKind(x, y, z int) TerrainKind {
	if l.Terrain == nil || x < 0 || y < 0 || z < 0 ||
		x >= l.GetWidth() || y >= l.GetHeight() || z >= l.GetDepth() {
		return TKVoid
	}
	return l.Terrain[l.terrainIdx(x, y, z)]
}

func (l *Level) SetTerrainKind(x, y, z int, k TerrainKind) {
	if l.Terrain == nil || x < 0 || y < 0 || z < 0 ||
		x >= l.GetWidth() || y >= l.GetHeight() || z >= l.GetDepth() {
		return
	}
	l.Terrain[l.terrainIdx(x, y, z)] = k
}

func (l *Level) GetBiome(x, y int) string {
	if l.BiomeMap == nil || x < 0 || y < 0 || x >= l.GetWidth() || y >= l.GetHeight() {
		return ""
	}
	return l.BiomeMap[y*l.GetWidth()+x]
}

func (l *Level) SetBiome(x, y int, biome string) {
	if l.BiomeMap == nil || x < 0 || y < 0 || x >= l.GetWidth() || y >= l.GetHeight() {
		return
	}
	l.BiomeMap[y*l.GetWidth()+x] = biome
}

func (l *Level) GetSurfaceZ(x, y int) int {
	if l.SurfaceMap == nil || x < 0 || y < 0 || x >= l.GetWidth() || y >= l.GetHeight() {
		return -1
	}
	return int(l.SurfaceMap[y*l.GetWidth()+x])
}

func (l *Level) SetSurfaceZ(x, y, z int) {
	if l.SurfaceMap == nil || x < 0 || y < 0 || x >= l.GetWidth() || y >= l.GetHeight() {
		return
	}
	l.SurfaceMap[y*l.GetWidth()+x] = int16(z)
}

// TagRegion records a named anchor point. Multiple anchors per tag are allowed.
func (l *Level) TagRegion(name string, x, y, z int) {
	if l.Regions == nil {
		l.Regions = make(map[string][][3]int)
	}
	l.Regions[name] = append(l.Regions[name], [3]int{x, y, z})
}
