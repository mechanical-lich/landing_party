package generation

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

// BuildWorldOptions describes a world to build. It deliberately mirrors the
// scenario JSON shape but is decoupled from the scenario package so this
// package doesn't import scenario (which would import generation).
type BuildWorldOptions struct {
	Width, Height, Depth int
	// Seed makes terrain generation reproducible: primers and the biome map
	// derive their noise from it, and math/rand is seeded for incidental
	// generation variance (best-effort — not every rand call is audited).
	Seed                 int64
	Terrain              string
	TerrainParams        map[string]any
	BiomeMapType         string
	BiomeMapScale        float64
	BiomeIDs             []string
	BiomeSingle          string
	Features             []FeatureSpec
}

// BuildWorld constructs a level by running primer → biome map → biome apply
// → features. Returns the level even on partial failure so callers can still
// render something for debugging.
func BuildWorld(opts BuildWorldOptions) (*world.Level, error) {
	if opts.Width <= 0 || opts.Height <= 0 || opts.Depth <= 0 {
		return nil, fmt.Errorf("BuildWorld: invalid dimensions %dx%dx%d", opts.Width, opts.Height, opts.Depth)
	}
	if opts.Terrain == "" {
		opts.Terrain = "planet"
	}
	primer, err := GetPrimer(opts.Terrain)
	if err != nil {
		return nil, err
	}
	// Best-effort: seed the global RNG so incidental rand use during priming
	// and feature placement is reproducible for a given seed.
	rand.Seed(opts.Seed)
	level := world.NewLevel(opts.Width, opts.Height, opts.Depth)
	if err := primer.Prime(level, opts.TerrainParams, opts.Seed); err != nil {
		return level, fmt.Errorf("primer %s: %w", opts.Terrain, err)
	}

	if opts.BiomeMapType != "" || len(opts.BiomeIDs) > 0 {
		BuildBiomeMap(level, BiomeMapConfig{
			Type:   opts.BiomeMapType,
			Scale:  opts.BiomeMapScale,
			Biomes: opts.BiomeIDs,
			Single: opts.BiomeSingle,
		}, opts.Seed)
		ApplyBiomes(level)
		ExposeMountainTops(level)
	}

	if len(opts.Features) > 0 {
		PlaceFeatures(level, opts.Features)
	}

	log.Printf("BuildWorld: %s primer + %d biomes + %d features",
		opts.Terrain, len(opts.BiomeIDs), len(opts.Features))
	return level, nil
}
