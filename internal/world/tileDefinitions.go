package world

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rllayered"
)

type TileVariant = rllayered.TileVariant

// TileDefinition extends the base with scifi-specific fields.
type TileDefinition struct {
	rllayered.TileDefinition
	Space bool `json:"space"` // true for vacuum/void tiles above the atmosphere
	// Mineable marks a tile as a resource deposit (ore, crystal, etc.). It is
	// drawn distinctly on the minimap, and a resource-scanner upgrade reveals it
	// there even before a colonist has discovered it.
	Mineable bool `json:"mineable"`
}

// EmptyTileName is the engine-reserved sentinel. rllayered.Slot.IsEmpty
// hardcodes Type == 0, so the "empty" definition must always live at index 0.
// The loader enforces this so JSON file order can't silently break the
// contract (e.g. a "hull.json" sorting before "tile_definitions.json" and
// stealing index 0, which would make painted hull_floor render as nothing).
const EmptyTileName = "empty"

var (
	TileDefinitions []TileDefinition
	TileNameToIndex map[string]int
	TileIndexToName []string
)

// LoadTileDefinitions loads tile definitions from a single JSON file containing
// an array of TileDefinition objects.
func LoadTileDefinitions(path string) error {
	defs, err := readTileDefsFile(path)
	if err != nil {
		return err
	}
	return applyTileDefinitions(defs)
}

// LoadTileDefinitionsDir loads tile definitions from every *.json file in dir,
// concatenating them in alphabetical filename order. This lets the catalog be
// split across logical files (hull, biomes, props, …). Filenames govern tile
// index assignment, so prefix with numbers (e.g. "01-base.json") if indices
// need to be pinned for persisted saves.
func LoadTileDefinitionsDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var all []TileDefinition
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		defs, err := readTileDefsFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		all = append(all, defs...)
	}
	if len(all) == 0 {
		return fmt.Errorf("no tile definition JSON files found in %s", dir)
	}
	return applyTileDefinitions(all)
}

func readTileDefsFile(path string) ([]TileDefinition, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var defs []TileDefinition
	if err := json.NewDecoder(file).Decode(&defs); err != nil {
		return nil, err
	}
	return defs, nil
}

// pinEmptySentinel hoists the "empty" definition to the front of defs so it
// lands at index 0, the slot rllayered.Slot.IsEmpty hardcodes. Order of the
// remaining entries is preserved, so non-sentinel tile indices stay stable
// (modulo where empty was). No-op if empty is already first or absent — the
// caller validates presence.
func pinEmptySentinel(defs []TileDefinition) []TileDefinition {
	for i, d := range defs {
		if d.Name == EmptyTileName {
			if i == 0 {
				return defs
			}
			out := make([]TileDefinition, 0, len(defs))
			out = append(out, d)
			out = append(out, defs[:i]...)
			out = append(out, defs[i+1:]...)
			return out
		}
	}
	return defs
}

// applyTileDefinitions installs defs into the package-level tables and syncs
// the base rllayered registry. Returns an error if a Name appears twice or if
// the required "empty" sentinel is missing, so a split catalog can't silently
// mask duplicates or break the index-0 contract.
func applyTileDefinitions(defs []TileDefinition) error {
	defs = pinEmptySentinel(defs)
	if len(defs) == 0 || defs[0].Name != EmptyTileName {
		return fmt.Errorf("tile definitions missing required %q sentinel (must be present and lands at index 0)", EmptyTileName)
	}
	TileDefinitions = make([]TileDefinition, len(defs))
	TileNameToIndex = make(map[string]int, len(defs))
	TileIndexToName = make([]string, len(defs))
	for i, def := range defs {
		if _, dup := TileNameToIndex[def.Name]; dup {
			return fmt.Errorf("duplicate tile definition name %q", def.Name)
		}
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
