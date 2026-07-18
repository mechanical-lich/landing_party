package world

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rllayered"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
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

	// FeatureAreaScale multiplies areal feature counts (ore veins, scatters,
	// etc.) so resource density stays constant across the map-size roll: a map
	// that rolls larger than its typical footprint gets proportionally more.
	// Set by generation.BuildWorld from rolledArea/referenceArea. 0 means unset
	// and is treated as 1 (see FeatureAreaMultiplier). Generation-only; not
	// persisted.
	FeatureAreaScale float64

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

	// Events is this level's own simulation event bus. Each planet fires and
	// consumes its own sim events (deaths, structures, research, etc.) here so
	// background planets don't cross-contaminate the live one. UI/input events
	// stay on the global event.GetQueuedInstance() bus. See
	// docs/developer/background_simulation.md.
	Events *event.QueuedEventManager

	// ResourceAmount holds the remaining yield of each mineable deposit tile
	// (ore/crystal/radioactive), keyed by packed coordinate. Rolled at
	// generation, decremented as a deposit is mined, and the tile is cleared
	// when it reaches zero. Persisted in saves so partial mining survives
	// save/load and campaign freeze — and so it can't be re-rolled by
	// cancelling and re-issuing a mine order.
	ResourceAmount map[TileCoord]int

	// Lighting config — set from scenario at level creation.
	LightMode    string // "day_night", "fixed", or "pitch_dark"
	FixedAmbient int    // used when LightMode == "fixed"

	// Fog of war: Visible is reset each FOV pass and marks tiles with current
	// line-of-sight from any worker. Seen (on the embedded Level) is permanent.
	Visible []bool

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

// FeatureAreaMultiplier returns the areal-feature count multiplier, defaulting
// to 1 when FeatureAreaScale is unset (0) so callers never divide density to
// zero on levels built without a reference area.
func (l *Level) FeatureAreaMultiplier() float64 {
	if l.FeatureAreaScale <= 0 {
		return 1
	}
	return l.FeatureAreaScale
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
		Level:          base,
		Flags:          make(map[string]any),
		Regions:        make(map[string][][3]int),
		Events:         &event.QueuedEventManager{},
		ResourceAmount: make(map[TileCoord]int),
		Visible:        make([]bool, total),
		WorkerZLevels:  make(map[int]bool),
		op:             &ebiten.DrawImageOptions{},
		entitiesBuffer: make([]*ecs.Entity, 0, 16),
	}
	base.PathCostFunc = getPathCostFunction(level)
	return level
}

// QueueEvent queues a simulation event on this level's own event bus, to be
// dispatched when the level is stepped. Nil-safe so zero-value levels (tests)
// don't panic.
func (l *Level) QueueEvent(evt event.EventData) {
	if l.Events == nil {
		l.Events = &event.QueuedEventManager{}
	}
	l.Events.QueueEvent(evt)
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

// ClearVisibleViewport zeroes only the Visible entries within the given camera
// viewport rect, across all Z levels from 0 up to maxZ (inclusive).
// This is cheaper than ClearVisible when most of the map is off-screen.
func (l *Level) ClearVisibleViewport(vp Viewport, maxZ int) {
	w, h, depth := l.GetWidth(), l.GetHeight(), l.GetDepth()
	if maxZ >= depth {
		maxZ = depth - 1
	}
	x0 := vp.X
	x1 := vp.X + vp.W
	y0 := vp.Y
	y1 := vp.Y + vp.H
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

// ColumnsWithBiome returns the (x, y) columns whose biome label equals id, in
// row-major order. Feature placement samples from this directly so a
// biome-restricted feature finds its tiles even when the biome is a small
// fraction of a large map (uniform sampling would mostly miss it). Returns nil
// if the level has no biome map.
func (l *Level) ColumnsWithBiome(id string) [][2]int {
	if l.BiomeMap == nil {
		return nil
	}
	w, h := l.GetWidth(), l.GetHeight()
	cols := make([][2]int, 0, len(l.BiomeMap)/8)
	for y := 0; y < h; y++ {
		base := y * w
		for x := 0; x < w; x++ {
			if l.BiomeMap[base+x] == id {
				cols = append(cols, [2]int{x, y})
			}
		}
	}
	return cols
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
