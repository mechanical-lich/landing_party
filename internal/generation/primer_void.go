package generation

import (
	"github.com/mechanical-lich/landing_party/internal/world"
)

// VoidPrimer is open space with one docking deck (phase D: a boarded
// ship's level). Everything is vacuum but a hull-floored deck in the middle
// of the surface slab, where the boarding party beams onto ("starting_floor"
// at its west end) and the boarded ship is stamped (docs/developer/
// factions.md, "Interception").
type VoidPrimer struct{}

func init() {
	RegisterPrimer("void", VoidPrimer{})
}

// Prime params: deck_w, deck_h (the deck, default 40x24).
func (VoidPrimer) Prime(level *world.Level, params map[string]any, seed int64) error {
	w, h, d := level.GetWidth(), level.GetHeight(), level.GetDepth()
	level.AllocTerrain()
	sz := min(1, d-1)
	level.SurfaceZ = sz
	level.AtmosphereZ = sz + 1
	level.SpaceZ = sz + 1
	for z := 0; z < d; z++ {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				paintKind(level, x, y, z, world.TKSpace)
			}
		}
	}
	dw, dh := min(paramInt(params, "deck_w", 40), w-2), min(paramInt(params, "deck_h", 24), h-2)
	x0, y0 := (w-dw)/2, (h-dh)/2
	for y := y0; y < y0+dh; y++ {
		for x := x0; x < x0+dw; x++ {
			level.SetTerrainKind(x, y, sz, world.TKStructure)
			level.SetFloor(x, y, sz, "hull_floor", world.TileVariantAt("hull_floor", x, y, sz))
			level.ClearMiddle(x, y, sz)
			level.SetSurfaceZ(x, y, sz)
		}
	}
	level.TagRegion("starting_floor", x0+3, y0+dh/2, sz)
	level.TagRegion("boarding_ship", x0+dw/2+3, y0+dh/2, sz)
	return nil
}
