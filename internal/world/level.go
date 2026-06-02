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

	// Tick is the world-round counter, advanced by stepWorld each round.
	// Used by sense systems (hearing, scent) for event TTLs and inbox diffs.
	Tick uint64

	// Sounds is the level's per-tick list of one-shot sound events.
	// Appended via EmitSound, swept by HearingSystem.
	Sounds []SoundEvent

	// SoundSeq is a strictly monotonic counter assigned to each emitted
	// sound. Listeners track the highest seq they've ingested so they
	// never miss or double-count events regardless of emit/listen ordering
	// within a tick.
	SoundSeq uint64

	// SmellMap is a sparse per-tag map of scent intensities at tiles.
	// Decayed every round; one tag is diffused per round (sharded). Sub-
	// epsilon entries are removed for sparsity.
	SmellMap map[SmellTag]map[TileCoord]float32

	// SmellTagOrder is the rotation order for sharded diffusion. Appended
	// to when a new tag is first emitted on the level.
	SmellTagOrder []SmellTag

	// SmellDiffuseIndex is the next tag to diffuse (advances each round).
	SmellDiffuseIndex int

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
	LightMode    string // "day_night", "fixed", or "pitch_dark"
	FixedAmbient int    // used when LightMode == "fixed"

	// Fog of war: Visible is reset each FOV pass and marks tiles with current
	// line-of-sight from any worker. Seen (on the embedded Level) is permanent.
	Visible []bool

	// Camera viewport set by the game state before each draw/update so FOVSystem
	// can limit Visible clears and writes to the on-screen region.
	CameraX, CameraY, CameraZ int
	ViewW, ViewH               int

	// WorkerZLevels is the set of z-levels that have active worker FOV this tick.
	// Reset by FOVSystem each pass. DrawLevel uses it to decide whether to apply
	// live fog-of-war (worker Z) or just use Seen as visible (camera Z).
	WorkerZLevels map[int]bool

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
	total := width * height * depth
	level := &Level{
		Level:         base,
		Flags:         make(map[string]any),
		Regions:       make(map[string][][3]int),
		Visible:       make([]bool, total),
		WorkerZLevels: make(map[int]bool),
		op:            &ebiten.DrawImageOptions{},
		entitiesBuffer: make([]*ecs.Entity, 0, 16),
	}
	base.PathCostFunc = getPathCostFunction(level)
	return level
}

func (l *Level) visibleIdx(x, y, z int) int {
	return (z*l.GetHeight()+y)*l.GetWidth() + x
}

func (l *Level) GetVisible(x, y, z int) bool {
	if !l.InBounds(x, y, z) {
		return false
	}
	return l.Visible[l.visibleIdx(x, y, z)]
}

func (l *Level) SetVisible(x, y, z int) {
	if !l.InBounds(x, y, z) {
		return
	}
	l.Visible[l.visibleIdx(x, y, z)] = true
}

func (l *Level) ClearVisible() {
	for i := range l.Visible {
		l.Visible[i] = false
	}
}

// ClearVisibleViewport zeroes only the Visible entries within the camera
// viewport rect, across all Z levels from 0 up to maxZ (inclusive).
// This is cheaper than ClearVisible when most of the map is off-screen.
func (l *Level) ClearVisibleViewport(maxZ int) {
	w, h, depth := l.GetWidth(), l.GetHeight(), l.GetDepth()
	if maxZ >= depth {
		maxZ = depth - 1
	}
	x0 := l.CameraX
	x1 := l.CameraX + l.ViewW
	y0 := l.CameraY
	y1 := l.CameraY + l.ViewH
	if x0 < 0 {
		x0 = 0
	}
	if x1 > w {
		x1 = w
	}
	if y0 < 0 {
		y0 = 0
	}
	if y1 > h {
		y1 = h
	}
	for z := 0; z <= maxZ; z++ {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				l.Visible[l.visibleIdx(x, y, z)] = false
			}
		}
	}
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
