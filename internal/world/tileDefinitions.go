package world

import (
	"encoding/json"
	"os"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rllayered"
)

type TileVariant = rllayered.TileVariant

// TileDefinition extends the base with scifi-specific fields.
type TileDefinition struct {
	rllayered.TileDefinition
	Space bool `json:"space"` // true for vacuum/void tiles above the atmosphere
}

var (
	TileDefinitions []TileDefinition
	TileNameToIndex map[string]int
	TileIndexToName []string
)

func LoadTileDefinitions(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var defs []TileDefinition
	if err := json.NewDecoder(file).Decode(&defs); err != nil {
		return err
	}

	TileDefinitions = make([]TileDefinition, len(defs))
	TileNameToIndex = make(map[string]int, len(defs))
	TileIndexToName = make([]string, len(defs))
	for i, def := range defs {
		TileDefinitions[i] = def
		TileNameToIndex[def.Name] = i
		TileIndexToName[i] = def.Name
	}

	// Sync base definitions so layered Tile helpers (IsSolid, IsAir, etc.)
	// resolve correctly and PaintTile can dispatch by layer.
	baseDefs := make([]rllayered.TileDefinition, len(defs))
	for i, def := range defs {
		baseDefs[i] = def.TileDefinition
	}
	rllayered.SetTileDefinitions(baseDefs)

	return nil
}

// IsSpaceTile reports whether the tile's Middle slot is a space/void tile.
func IsSpaceTile(t *Tile) bool {
	if t == nil || t.Middle.IsEmpty() {
		return false
	}
	return TileDefinitions[t.Middle.Type].Space
}
