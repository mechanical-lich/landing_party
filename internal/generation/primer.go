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

// floorForKind returns the Floor tile name for a TerrainKind. The Floor is
// the persistent ground that survives mining/digging; the Middle slot (set
// separately) is what blocks movement.
func floorForKind(k world.TerrainKind) string {
	switch k {
	case world.TKSurface:
		return "regolith"
	case world.TKSubsurface:
		return "dirt_floor"
	case world.TKUnderground, world.TKCavern:
		return "rock_floor"
	case world.TKBedrock:
		return "bedrock"
	case world.TKStructure:
		return "hull_floor"
	}
	return ""
}

// middleForKind returns the Middle tile name (the blocking/visual layer) for
// a TerrainKind. Returns empty string for kinds that leave Middle empty.
func middleForKind(k world.TerrainKind) string {
	switch k {
	case world.TKSpace:
		return "space"
	case world.TKAtmosphere, world.TKCavern:
		return "air"
	case world.TKWater:
		return "water"
	case world.TKSubsurface:
		return "dirt"
	case world.TKUnderground:
		return "rock"
	case world.TKBedrock:
		return "bedrock"
	}
	return ""
}

// ExposeMountainTops clears the Middle slot of any solid tile above SurfaceZ
// whose tile directly above is non-solid. This makes mountain tops walkable
// after biomes have been applied (biomes run after the primer and would
// otherwise re-stamp solid Middle tiles on exposed rock tops).
func ExposeMountainTops(level *world.Level) {
	w, h, d := level.GetWidth(), level.GetHeight(), level.GetDepth()
	surfZ := level.SurfaceZ
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			for z := surfZ + 1; z < d-1; z++ {
				tile := level.GetTilePtr(x, y, z)
				if tile == nil || tile.Middle.IsEmpty() {
					continue
				}
				if !world.TileDefinitions[tile.Middle.Type].Solid {
					continue
				}
				above := level.GetTilePtr(x, y, z+1)
				if above == nil {
					continue
				}
				if above.Middle.IsEmpty() || !world.TileDefinitions[above.Middle.Type].Solid {
					level.ClearMiddle(x, y, z)
				}
			}
		}
	}
}

// paintKind paints the layered defaults for a TerrainKind. Floor and Middle
// are set independently so digging out the Middle reveals the Floor cleanly.
func paintKind(level *world.Level, x, y, z int, k world.TerrainKind) {
	level.SetTerrainKind(x, y, z, k)

	if floorName := floorForKind(k); floorName != "" {
		level.SetFloor(x, y, z, floorName, world.RandomTileVariant(floorName))
	}
	if middleName := middleForKind(k); middleName != "" {
		level.SetMiddle(x, y, z, middleName, world.RandomTileVariant(middleName))
	}
}
