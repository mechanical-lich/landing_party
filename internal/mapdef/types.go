// Package mapdef holds map (terrain) definitions: the recipe for generating a
// world's terrain, biomes, and features. It is deliberately decoupled from the
// scenario package — a scenario picks which maps it supports, but the map
// definition owns generation. generation must not import scenario/mapdef, so
// these types mirror generation's option shapes.
package mapdef

// MapDef is the full world-generation recipe for one map type. Loaded from
// data/maps/*.json.
type MapDef struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Terrain       string         `json:"terrain"`        // primer name: "planet", "asteroid_field", "abandoned_station"
	TerrainParams map[string]any `json:"terrain_params"` // forwarded to the primer
	BiomeMap      BiomeMapBlock  `json:"biome_map"`      // optional
	Features      []FeatureBlock `json:"features"`       // ordered feature placements
}

// BiomeMapBlock mirrors generation.BiomeMapConfig.
type BiomeMapBlock struct {
	Type   string   `json:"type"`
	Scale  float64  `json:"scale"`
	Biomes []string `json:"biomes"`
	Single string   `json:"single"`
}

// FeatureBlock mirrors generation.FeatureSpec.
type FeatureBlock struct {
	Kind     string         `json:"kind"`
	Count    int            `json:"count"`
	Biome    string         `json:"biome,omitempty"`
	InRegion string         `json:"in_region,omitempty"`
	Jitter   int            `json:"jitter,omitempty"`
	MinZ     int            `json:"min_z,omitempty"`
	MaxZ     int            `json:"max_z,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}
