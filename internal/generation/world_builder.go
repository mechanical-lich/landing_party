package generation

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// BuildWorldOptions describes a world to build. It deliberately mirrors the
// scenario JSON shape but is decoupled from the scenario package so this
// package doesn't import scenario (which would import generation).
type BuildWorldOptions struct {
	Width, Height, Depth int
	// Seed makes terrain generation reproducible: primers and the biome map
	// derive their noise from it, and math/rand is seeded for incidental
	// generation variance (best-effort — not every rand call is audited).
	Seed          int64
	Terrain       string
	TerrainParams map[string]any
	BiomeMapType        string
	BiomeMapScale       float64
	BiomeLatitudeWeight float64
	BiomeIDs            []string
	BiomeSingle         string
	Features            []FeatureSpec
	// ReferenceArea is the map's expected footprint (width×height at the size
	// range midpoints). Areal feature counts are scaled by
	// Width*Height / ReferenceArea so resource density stays constant across the
	// size roll. 0 disables scaling (counts used as authored).
	ReferenceArea int
	// SkipBiomeFeatures leaves out the features biomes declare for themselves
	// (forest trees etc.), keeping only those listed in Features.
	SkipBiomeFeatures bool
	// Origins are the site history's rolled origin tiles by event ID, resolved
	// to open ground once terrain and biomes exist and before any feature is
	// placed (see ResolveOrigins). ChapterEvent's is also the "origin" anchor.
	Origins      map[string][2]int
	ChapterEvent string
	// OriginKeepOut is each origin's keep-out from the landing reserve, by
	// event ID (tiles from the reserve box); missing ones keep the budget's
	// inert keep_out.
	OriginKeepOut map[string]int
	// Flags are set on the level before anything is placed, so structure
	// scripts run by features can read them (the site's lore flags).
	Flags map[string]any
}

// BuildWorld constructs a level by running primer → biome map → biome apply
// → features. For any valid dimensions it returns a non-nil level even when the
// primer lookup or a later phase fails (so callers can render something), with
// the error describing what went wrong. Only invalid dimensions return a nil
// level.
func BuildWorld(opts BuildWorldOptions) (*world.Level, error) {
	if opts.Width <= 0 || opts.Height <= 0 || opts.Depth <= 0 {
		return nil, fmt.Errorf("BuildWorld: invalid dimensions %dx%dx%d", opts.Width, opts.Height, opts.Depth)
	}
	if opts.Terrain == "" {
		opts.Terrain = "planet"
	}
	// All non-primer randomness (feature placement) draws from this local,
	// seed-derived source so a given location reproduces exactly. Primers get
	// the raw seed and manage their own noise/RNG. Tile-variant selection is
	// position-hashed (world.TileVariantAt), which is why the concurrent primer
	// pass needs no shared RNG.
	rng := rand.New(rand.NewSource(opts.Seed))
	// Allocate before the primer lookup so an unknown/unregistered terrain still
	// yields a usable (empty) level rather than nil — callers render an empty
	// world instead of nil-panicking. (Invalid dimensions above are the only
	// path that can't allocate and thus still returns nil.)
	level := world.NewLevel(opts.Width, opts.Height, opts.Depth)
	// Scale areal feature counts by how the rolled footprint compares to the
	// map's typical (midpoint) footprint, so density is constant across the
	// size roll. placeAnchors reads this off the level.
	if opts.ReferenceArea > 0 {
		level.FeatureAreaScale = float64(opts.Width*opts.Height) / float64(opts.ReferenceArea)
	}
	primer, err := GetPrimer(opts.Terrain)
	if err != nil {
		return level, err
	}
	if err := primer.Prime(level, opts.TerrainParams, opts.Seed); err != nil {
		return level, fmt.Errorf("primer %s: %w", opts.Terrain, err)
	}

	for k, v := range opts.Flags {
		level.Flags[k] = v
	}
	originsDone := false
	if opts.BiomeMapType != "" || len(opts.BiomeIDs) > 0 {
		BuildBiomeMap(level, BiomeMapConfig{
			Type:           opts.BiomeMapType,
			Scale:          opts.BiomeMapScale,
			Biomes:         opts.BiomeIDs,
			Single:         opts.BiomeSingle,
			LatitudeWeight: opts.BiomeLatitudeWeight,
		}, opts.Seed)
		ApplyBiomes(level)
		ExposeMountainTops(level)
		ReserveLanding(level, opts.Seed)
		ResolveOriginsKeepOut(level, opts.Origins, opts.ChapterEvent, opts.OriginKeepOut)
		originsDone = true

		// Collect and run features declared in each active biome.
		if !opts.SkipBiomeFeatures {
			for _, id := range opts.BiomeIDs {
				if b := GetBiome(id); b != nil && len(b.Features) > 0 {
					PlaceFeatures(level, b.Features, rng)
				}
			}
			if opts.BiomeSingle != "" {
				if b := GetBiome(opts.BiomeSingle); b != nil && len(b.Features) > 0 {
					PlaceFeatures(level, b.Features, rng)
				}
			}
		}
	}

	if !originsDone {
		ReserveLanding(level, opts.Seed)
		ResolveOriginsKeepOut(level, opts.Origins, opts.ChapterEvent, opts.OriginKeepOut)
	}
	if len(opts.Features) > 0 {
		PlaceFeatures(level, opts.Features, rng)
	}

	log.Printf("BuildWorld: %s primer + %d biomes + %d features",
		opts.Terrain, len(opts.BiomeIDs), len(opts.Features))
	return level, nil
}
