package world

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlworld"
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

// Level embeds rlworld.Level and adds rendering state and z-band metadata.
type Level struct {
	*rlworld.Level

	Flags map[string]any

	// Z-band boundaries (inclusive). Set during generation.
	SurfaceZ     int // the "ground" z-level
	AtmosphereZ  int // first z-level of air above surface
	SpaceZ       int // first z-level of vacuum/space

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
	base := rlworld.NewLevel(width, height, depth)
	level := &Level{
		Level:          base,
		Flags:          make(map[string]any),
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
