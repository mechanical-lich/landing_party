package world

import (
	"encoding/json"
	"os"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlworld"
)

type TileVariant = rlworld.TileVariant

// TileDefinition extends the base with scifi-specific fields.
type TileDefinition struct {
	rlworld.TileDefinition
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

	// Sync base definitions so Tile.IsSolid(), IsAir(), etc. work
	baseDefs := make([]rlworld.TileDefinition, len(defs))
	for i, def := range defs {
		baseDefs[i] = def.TileDefinition
	}
	rlworld.SetTileDefinitions(baseDefs)

	return nil
}

// IsSpace reports whether the tile at position (x,y,z) is a space/void tile.
func IsSpaceTile(t *Tile) bool {
	if t == nil {
		return false
	}
	return TileDefinitions[t.Type].Space
}
