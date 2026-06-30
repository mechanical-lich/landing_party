package campaign

import (
	"github.com/mechanical-lich/landing_party/internal/world"
)

// OpenToSky reports whether (x,y,z) has clear sky above it — no solid Middle
// tile in any higher Z column. Beaming may not pass through ground.
func OpenToSky(level *world.Level, x, y, z int) bool {
	depth := level.GetDepth()
	for zz := z; zz < depth; zz++ {
		ti := level.GetTileAt(x, y, zz)
		if ti == nil {
			continue
		}
		t := ti.(*world.Tile)
		if zz > z && !t.Middle.IsEmpty() {
			if int(t.Middle.Type) < len(world.TileDefinitions) && world.TileDefinitions[t.Middle.Type].Solid {
				return false
			}
		}
		if !t.Ceiling.IsEmpty() {
			return false
		}
	}
	return true
}

