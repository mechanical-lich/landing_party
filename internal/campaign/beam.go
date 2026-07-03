package campaign

import (
	"github.com/mechanical-lich/landing_party/internal/world"
)

// OpenToSky reports whether a beam can reach (x,y,z) from above: no solid Middle
// tile (wall/rock/ore) in any higher Z cell. Beaming may not pass through solid
// ground, but a Floor overhead is only a deck — it does NOT block, so colonists
// can be beamed up from inside an enclosed ship or space station (whose hull
// decks are Floor slots, not solid mass) without climbing to the top floor.
func OpenToSky(level *world.Level, x, y, z int) bool {
	depth := level.GetDepth()
	for zz := z + 1; zz < depth; zz++ {
		ti := level.GetTileAt(x, y, zz)
		if ti == nil {
			continue
		}
		t := ti.(*world.Tile)
		if !t.Middle.IsEmpty() &&
			int(t.Middle.Type) < len(world.TileDefinitions) &&
			world.TileDefinitions[t.Middle.Type].Solid {
			return false // solid ground/wall overhead
		}
	}
	return true
}

