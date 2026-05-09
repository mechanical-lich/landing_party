package generation

import (
	"fmt"

	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

// TerrainPrimer produces the terrain skeleton for a level: it allocates the
// level, tags every (x,y,z) with a TerrainKind, fills the SurfaceMap, and may
// emit named regions (e.g. "starting_asteroid"). Concrete tile selection is
// the biome applier's job — primers only paint a default tile per kind so
// scenarios without a biome map still produce something playable.
type TerrainPrimer interface {
	Prime(level *world.Level, params map[string]any) error
}

var primers = map[string]TerrainPrimer{}

// RegisterPrimer adds a primer under the given name (used by scenario JSON).
func RegisterPrimer(name string, p TerrainPrimer) {
	primers[name] = p
}

// GetPrimer returns the primer registered under name.
func GetPrimer(name string) (TerrainPrimer, error) {
	p, ok := primers[name]
	if !ok {
		return nil, fmt.Errorf("generation: no terrain primer named %q", name)
	}
	return p, nil
}

// paramInt extracts an int from a JSON map[string]any (numbers decode as
// float64). Returns def if the key is missing.
func paramInt(params map[string]any, key string, def int) int {
	if params == nil {
		return def
	}
	v, ok := params[key]
	if !ok {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	}
	return def
}

func paramFloat(params map[string]any, key string, def float64) float64 {
	if params == nil {
		return def
	}
	v, ok := params[key]
	if !ok {
		return def
	}
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	}
	return def
}

func paramString(params map[string]any, key, def string) string {
	if params == nil {
		return def
	}
	if v, ok := params[key].(string); ok {
		return v
	}
	return def
}

// defaultTileForKind returns a placeholder tile for a TerrainKind so the level
// is renderable even before biome rules run. Biome applier will overwrite.
func defaultTileForKind(k world.TerrainKind) string {
	switch k {
	case world.TKSpace:
		return "space"
	case world.TKAtmosphere:
		return "air"
	case world.TKSurface:
		return "regolith"
	case world.TKSubsurface:
		return "dirt"
	case world.TKUnderground:
		return "rock"
	case world.TKCavern:
		return "air"
	case world.TKBedrock:
		return "bedrock"
	case world.TKWater:
		return "water"
	case world.TKStructure:
		return "hull_floor"
	}
	return "air"
}

// paintKind sets both the terrain skeleton and a placeholder tile.
func paintKind(level *world.Level, x, y, z int, k world.TerrainKind) {
	level.SetTerrainKind(x, y, z, k)
	level.UpdateTileAt(x, y, z, defaultTileForKind(k), world.RandomTileVariant(defaultTileForKind(k)))
}
