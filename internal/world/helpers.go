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

// TileVariantAt deterministically selects a tile variant from the tile's type
// and position. World generation runs primers across multiple goroutines, so
// variant choice must not depend on goroutine scheduling or draw order from a
// shared RNG — otherwise the same seed renders differently each run. Hashing
// the coordinates keeps generation reproducible down to the sprite while
// staying lock-free. Use this in generation; RandomTileVariant remains for
// gameplay-time tile changes where reproducibility doesn't matter.
func TileVariantAt(tileName string, x, y, z int) int {
	idx, ok := TileNameToIndex[tileName]
	if !ok {
		return 0
	}
	n := len(TileDefinitions[idx].Variants)
	if n == 0 {
		return 0
	}
	// SplitMix64-style avalanche over a mix of type + coordinates.
	h := uint64(idx)*0x9E3779B97F4A7C15 ^
		uint64(uint32(x))*0xC2B2AE3D27D4EB4F ^
		uint64(uint32(y))*0x165667B19E3779F9 ^
		uint64(uint32(z))*0xD6E8FEB86659FD93
	h ^= h >> 30
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 27
	h *= 0x94D049BB133111EB
	h ^= h >> 31
	return int(h % uint64(n))
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
