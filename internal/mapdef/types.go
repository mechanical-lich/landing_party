// Package mapdef holds map (terrain) definitions: the recipe for generating a
// world's terrain, biomes, and features. It is deliberately decoupled from the
// scenario package — a scenario picks which maps it supports, but the map
// definition owns generation. generation must not import scenario/mapdef, so
// these types mirror generation's option shapes.
package mapdef

import (
	"fmt"
	"math/rand"
)

// MapDef is the full world-generation recipe for one map type. Loaded from
// data/maps/*.json.
type MapDef struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Size          SizeBlock      `json:"size"`           // generation dimensions; rolled per-location
	Terrain       string         `json:"terrain"`        // primer name: "planet", "asteroid_field", "abandoned_station"
	TerrainParams map[string]any `json:"terrain_params"` // forwarded to the primer
	BiomeMap      BiomeMapBlock  `json:"biome_map"`      // optional
	Features      []FeatureBlock `json:"features"`       // ordered feature placements
}

// IntRange is an inclusive integer range. Authors write {"min": 200, "max": 360}
// in JSON; a fixed-size map sets min == max.
type IntRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// Roll picks a value uniformly in [Min, Max]. Returns Min when Max <= Min.
func (r IntRange) Roll(rng *rand.Rand) int {
	if r.Max <= r.Min {
		return r.Min
	}
	return r.Min + rng.Intn(r.Max-r.Min+1)
}

// SizeBlock declares the per-dimension generation size for a map. Each location
// rolls once at creation time and stores the result in the level's metadata, so
// re-entering the same site reuses the same dimensions.
type SizeBlock struct {
	W IntRange `json:"w"`
	H IntRange `json:"h"`
	Z IntRange `json:"z"`
}

// Validate ensures every dimension has a sane range. Loader rejects map
// files that omit the block or set non-positive / inverted ranges, so callers
// can trust SizeBlock.Roll without fallback paths.
func (s SizeBlock) Validate() error {
	for _, d := range []struct {
		name string
		r    IntRange
	}{{"w", s.W}, {"h", s.H}, {"z", s.Z}} {
		if d.r.Min <= 0 {
			return fmt.Errorf("%s.min must be > 0 (got %d)", d.name, d.r.Min)
		}
		if d.r.Max < d.r.Min {
			return fmt.Errorf("%s.max (%d) must be >= %s.min (%d)", d.name, d.r.Max, d.name, d.r.Min)
		}
	}
	return nil
}

// Roll picks one (w, h, z) triple from this size block using a seeded rng so
// runs of the same campaign reproduce.
func (s SizeBlock) Roll(seed int64) (w, h, z int) {
	rng := rand.New(rand.NewSource(seed))
	w = s.W.Roll(rng)
	h = s.H.Roll(rng)
	z = s.Z.Roll(rng)
	return
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
