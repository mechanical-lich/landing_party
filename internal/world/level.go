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

// Level embeds rlworld.Level and adds rendering state and z-band metadata.
type Level struct {
	*rlworld.Level

	Flags map[string]any

	// Z-band boundaries (inclusive). Set during generation.
	SurfaceZ     int // the "ground" z-level
	AtmosphereZ  int // first z-level of air above surface
	SpaceZ       int // first z-level of vacuum/space

	op             *ebiten.DrawImageOptions
	entitiesBuffer []*ecs.Entity
	lightOverlay   *ebiten.Image
	lightPixels    []byte
	lightW, lightH int
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
