package generation

import (
	"fmt"
	"log"

	"github.com/mechanical-lich/scifi_settlements/internal/world"
)


// FeatureSpec is the JSON shape for a feature placement directive.
type FeatureSpec struct {
	Kind   string         `json:"kind"`
	Count  int            `json:"count"`
	Biome  string         `json:"biome,omitempty"`  // restrict to columns with this biome ("" = any)
	MinZ   int            `json:"min_z,omitempty"`
	MaxZ   int            `json:"max_z,omitempty"`
	Params map[string]any `json:"params,omitempty"`
}

// FeaturePlacer places a single instance of a feature at a chosen anchor.
// The placer is responsible for picking suitable locations.
type FeaturePlacer func(level *world.Level, spec FeatureSpec) error

var featurePlacers = map[string]FeaturePlacer{}

// RegisterFeature adds a placer.
func RegisterFeature(kind string, p FeaturePlacer) {
	featurePlacers[kind] = p
}

// GetFeature returns the placer for kind, or nil.
func GetFeature(kind string) FeaturePlacer {
	return featurePlacers[kind]
}

// PlaceFeatures runs every spec in order. Failures are logged but don't abort.
func PlaceFeatures(level *world.Level, specs []FeatureSpec) {
	for _, s := range specs {
		p := GetFeature(s.Kind)
		if p == nil {
			log.Printf("PlaceFeatures: unknown feature kind %q", s.Kind)
			continue
		}
		if err := p(level, s); err != nil {
			log.Printf("PlaceFeatures %s: %v", s.Kind, err)
		}
	}
}

// columnMatchesBiome returns true if the (x,y) column's biome label matches
// the spec restriction (empty = any).
func columnMatchesBiome(level *world.Level, x, y int, biome string) bool {
	if biome == "" {
		return true
	}
	return level.GetBiome(x, y) == biome
}

func featureParamInt(s FeatureSpec, key string, def int) int {
	return paramInt(s.Params, key, def)
}
func featureParamFloat(s FeatureSpec, key string, def float64) float64 {
	return paramFloat(s.Params, key, def)
}
func featureParamString(s FeatureSpec, key, def string) string {
	return paramString(s.Params, key, def)
}

// errFeature wraps a feature error with kind context.
func errFeature(kind, msg string) error {
	return fmt.Errorf("%s: %s", kind, msg)
}

var warnedUnknownTile = map[string]bool{}

// requireTile returns true if the tile name exists in the loaded definitions.
// Unknown names are dangerous because UpdateTileAt silently maps them to
// index 0 (which is "space"). Logs a one-time warning per name.
func requireTile(name string) bool {
	if _, ok := world.TileNameToIndex[name]; ok {
		return true
	}
	if !warnedUnknownTile[name] {
		warnedUnknownTile[name] = true
		log.Printf("generation: unknown tile %q — feature skipped (add it to tile_definitions.json)", name)
	}
	return false
}
