package generation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

// BiomeRule maps a TerrainKind (and optional Z offset from surface) to a
// concrete tile type, plus optional baseline tile properties.
//
// Match order:
//   1. TerrainKind must equal Kind (string form).
//   2. If YOffset is non-nil, the tile's z relative to its column surface
//      must equal it. Negative = below surface, positive = above.
//   3. Otherwise the rule matches any z with that kind.
//
// Rules are evaluated in order; first match wins.
type BiomeRule struct {
	Kind      string `json:"kind"`               // "surface", "subsurface", "underground", "cavern", "atmosphere", "space", "bedrock"
	YOffset   *int   `json:"y_offset,omitempty"` // optional: relative to surface column
	Tile      string `json:"tile"`               // primary tile (paints into its declared layer)
	Floor     string `json:"floor,omitempty"`    // optional explicit Floor override
	Middle    string `json:"middle,omitempty"`   // optional explicit Middle override
	Radiation int    `json:"radiation,omitempty"`
}

// Biome describes one biome's appearance + features.
type Biome struct {
	ID            string         `json:"id"`
	TempRange     [2]float64     `json:"temp_range"`     // 0..1, inclusive
	HumidityRange [2]float64     `json:"humidity_range"` // 0..1, inclusive
	Rules         []BiomeRule    `json:"rules"`
	Features      []FeatureSpec  `json:"features,omitempty"` // biome-specific features
}

var biomes = map[string]*Biome{}

// LoadBiomes reads every .json file in dir and registers it as a biome.
func LoadBiomes(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// missing dir is fine — scenarios may not use biomes
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("biomes: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("biomes %s: %w", entry.Name(), err)
		}
		var b Biome
		if err := json.Unmarshal(data, &b); err != nil {
			return fmt.Errorf("biomes %s: %w", entry.Name(), err)
		}
		if b.ID == "" {
			return fmt.Errorf("biomes %s: missing id", entry.Name())
		}
		biomes[b.ID] = &b
	}
	return nil
}

// GetBiome returns the registered biome by ID, or nil.
func GetBiome(id string) *Biome {
	return biomes[id]
}

// kindString stringifies TerrainKind to match JSON BiomeRule.Kind.
func kindString(k world.TerrainKind) string {
	switch k {
	case world.TKSpace:
		return "space"
	case world.TKAtmosphere:
		return "atmosphere"
	case world.TKSurface:
		return "surface"
	case world.TKSubsurface:
		return "subsurface"
	case world.TKUnderground:
		return "underground"
	case world.TKCavern:
		return "cavern"
	case world.TKBedrock:
		return "bedrock"
	case world.TKWater:
		return "water"
	}
	return ""
}
