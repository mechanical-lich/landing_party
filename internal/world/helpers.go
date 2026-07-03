package world

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rllayered"
	"github.com/mechanical-lich/mlge/utility"
)

func RandomTileVariant(tileName string) int {
	idx, ok := TileNameToIndex[tileName]
	if !ok {
		return 0
	}
	n := len(TileDefinitions[idx].Variants)
	if n == 0 {
		return 0
	}
	return utility.GetRandom(0, n)
}

// SetTileTypeAndVariant paints the named tile into whichever slot its
// TileDefinition declares via its Layer field (default Middle).
func SetTileTypeAndVariant(tile *Tile, tileName string, variant int) {
	idx, ok := TileNameToIndex[tileName]
	if !ok || idx < 0 || idx >= len(TileDefinitions) {
		return
	}
	if variant < 0 || variant >= len(TileDefinitions[idx].Variants) {
		variant = 0
	}
	slot := rllayered.Slot{Type: idx, Variant: variant}
	switch TileDefinitions[idx].LayerOf() {
	case rllayered.LayerFloor:
		tile.Floor = slot
	default:
		tile.Middle = slot
	}
}
