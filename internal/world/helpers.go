package world

import "github.com/mechanical-lich/mlge/utility"

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

func SetTileTypeAndVariant(tile *Tile, tileName string, variant int) {
	idx := TileNameToIndex[tileName]
	if idx < 0 || idx >= len(TileDefinitions) {
		return
	}
	if variant < 0 || variant >= len(TileDefinitions[idx].Variants) {
		variant = 0
	}
	tile.Type = idx
	tile.Variant = variant
}
