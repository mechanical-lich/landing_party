package generation

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// TestPlanetMapsGenerateOre guards against "no metal ore" regressions: every
// planet map must place mineable ore_deposit tiles underground. (Builds the
// real maps with real tile/biome data.)
func TestPlanetMapsGenerateOre(t *testing.T) {
	if err := world.LoadTileDefinitionsDir("../../data/tiledefinitions"); err != nil {
		t.Fatal(err)
	}
	if err := LoadBiomes("../../data/biomes"); err != nil {
		t.Fatal(err)
	}
	if err := mapdef.Load("../../data/maps"); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"earth_like", "alien", "desert_planet", "frozen_planet", "moon"} {
		md := mapdef.ByID(id)
		if md == nil {
			t.Fatalf("map %s missing", id)
		}
		w, h, d := md.Size.Roll(12345)
		opts := BuildWorldOptions{
			Width: w, Height: h, Depth: d, Seed: 12345,
			Terrain: md.Terrain, TerrainParams: md.TerrainParams,
			BiomeMapType: md.BiomeMap.Type, BiomeMapScale: md.BiomeMap.Scale,
			BiomeIDs: md.BiomeMap.Biomes, BiomeSingle: md.BiomeMap.Single,
			ReferenceArea: md.Size.MidArea(),
		}
		for _, fb := range md.Features {
			opts.Features = append(opts.Features, FeatureSpec{
				Kind: fb.Kind, Count: fb.Count, CountMax: fb.CountMax, Biome: fb.Biome,
				InRegion: fb.InRegion, Jitter: fb.Jitter, MinZ: fb.MinZ, MaxZ: fb.MaxZ, Params: fb.Params,
			})
		}
		level, err := BuildWorld(opts)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		ore := 0
		for z := 0; z < d; z++ {
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					tp := level.GetTilePtr(x, y, z)
					if tp != nil && !tp.Middle.IsEmpty() &&
						world.TileDefinitions[tp.Middle.Type].Name == "ore_deposit" {
						ore++
					}
				}
			}
		}
		if ore == 0 {
			t.Errorf("map %s generated no ore_deposit tiles", id)
		}
	}
}
