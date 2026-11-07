package generation

import (
	"math/rand"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/world"
)

func loadPlacementData(t *testing.T) {
	t.Helper()
	if err := world.LoadTileDefinitionsDir("../../data/tiledefinitions"); err != nil {
		t.Fatal(err)
	}
	if err := LoadBiomes("../../data/biomes"); err != nil {
		t.Fatal(err)
	}
	if err := mapdef.Load("../../data/maps"); err != nil {
		t.Fatal(err)
	}
	if err := factory.FactoryLoadDir("../../data/blueprints/entities"); err != nil {
		t.Fatal(err)
	}
}

// flatLevel is a mountain-free planet: every column is open surface.
func flatLevel(t *testing.T, w, h int) *world.Level {
	t.Helper()
	level := world.NewLevel(w, h, 10)
	if err := (PlanetPrimer{}).Prime(level, map[string]any{"mountain_frequency": 0.0}, 1); err != nil {
		t.Fatal(err)
	}
	return level
}

func middleName(level *world.Level, x, y, z int) string {
	tile := level.GetTilePtr(x, y, z)
	if tile == nil || tile.Middle.IsEmpty() {
		return ""
	}
	return world.TileDefinitions[tile.Middle.Type].Name
}

// Every biome a map lists must actually appear. Two biomes with the same climate
// box tie on every point, so the second never generates.
func TestEveryMapBiomeAppears(t *testing.T) {
	loadPlacementData(t)
	for _, md := range mapdef.All() {
		if len(md.BiomeMap.Biomes) < 2 {
			continue
		}
		w, h := (md.Size.W.Min+md.Size.W.Max)/2, (md.Size.H.Min+md.Size.H.Max)/2
		counts := map[string]int{}
		for seed := int64(1); seed <= 6; seed++ {
			level := world.NewLevel(w, h, 1)
			level.AllocTerrain()
			BuildBiomeMap(level, BiomeMapConfig{
				Type:           md.BiomeMap.Type,
				Scale:          md.BiomeMap.Scale,
				Biomes:         md.BiomeMap.Biomes,
				LatitudeWeight: md.BiomeMap.LatitudeWeight,
			}, seed*1000)
			for _, id := range level.BiomeMap {
				counts[id]++
			}
		}
		for _, id := range md.BiomeMap.Biomes {
			if counts[id] == 0 {
				t.Errorf("map %s: biome %s never generated", md.ID, id)
			}
		}
	}
}

// Crater ground must read differently from the regolith around it.
func TestCraterGroundIsDistinct(t *testing.T) {
	loadPlacementData(t)
	surfaceTile := func(id string) string {
		for _, r := range GetBiome(id).Rules {
			if r.Kind == "surface" {
				return r.Tile
			}
		}
		return ""
	}
	if a, b := surfaceTile("crater"), surfaceTile("regolith"); a == b {
		t.Errorf("crater and regolith share surface tile %q", a)
	}
}

// Every spawn rule that can use regolith on the moon must also accept crater
// ground, or creatures would avoid every crater.
func TestMoonSpawnRulesAcceptCraterGround(t *testing.T) {
	if err := scenario.Load("../../data/scenarios"); err != nil {
		t.Fatal(err)
	}
	for _, sc := range scenario.ForMap("moon") {
		for name, rule := range sc.SpawnRules {
			hasRegolith, hasCrater := false, false
			for _, tile := range rule.Tiles {
				hasRegolith = hasRegolith || tile == "regolith"
				hasCrater = hasCrater || tile == "crater_ground"
			}
			if hasRegolith && !hasCrater {
				t.Errorf("scenario %s rule %s allows regolith but not crater_ground", sc.ID, name)
			}
		}
	}
}

// Pockets on open ground must still seed ore, and the new solid tile must mark
// its column for the sun pass.
func TestRadiationPocketSeedsOreOnOpenGround(t *testing.T) {
	loadPlacementData(t)
	level := flatLevel(t, 32, 32)
	level.DirtyColumns = nil
	spec := FeatureSpec{Kind: "radiation_pocket", Count: 10, Params: map[string]any{"radius": 3}}
	if err := placeRadiationPocket(level, spec, rand.New(rand.NewSource(7))); err != nil {
		t.Fatal(err)
	}
	ore := 0
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if middleName(level, x, y, level.SurfaceZ) == "radioactive_ore" {
				ore++
			}
		}
	}
	if ore == 0 {
		t.Fatal("no radioactive ore seeded on open ground")
	}
	if len(level.DirtyColumns) == 0 {
		t.Error("seeding ore did not invalidate any sun column")
	}
}

// Ore must not seed into water, lava or the void.
func TestRadiationPocketSkipsUnusableTiles(t *testing.T) {
	loadPlacementData(t)
	cases := map[string]func(level *world.Level, x, y, z int){
		"water": func(level *world.Level, x, y, z int) { level.SetMiddle(x, y, z, "water", 0) },
		"lava":  func(level *world.Level, x, y, z int) { level.SetFloor(x, y, z, "lava", 0) },
	}
	for name, paint := range cases {
		level := flatLevel(t, 16, 16)
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				paint(level, x, y, level.SurfaceZ)
			}
		}
		spec := FeatureSpec{Kind: "radiation_pocket", Count: 10, Params: map[string]any{"radius": 3}}
		if err := placeRadiationPocket(level, spec, rand.New(rand.NewSource(7))); err != nil {
			t.Fatal(err)
		}
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				if middleName(level, x, y, level.SurfaceZ) == "radioactive_ore" {
					t.Fatalf("%s: ore seeded at %d,%d", name, x, y)
				}
			}
		}
	}
}

// A lake that makes tiles unwalkable removes what was standing on them.
func TestLakeClearsEntitiesItCovers(t *testing.T) {
	loadPlacementData(t)
	for _, tile := range []string{"lava", "ice"} {
		level := flatLevel(t, 16, 16)
		z := level.SurfaceZ
		e, err := factory.Create("round_tree", 8, 8, z)
		if err != nil {
			t.Fatal(err)
		}
		level.AddEntity(e)
		// Radius 20 covers the whole map from any anchor.
		spec := FeatureSpec{Kind: "lava_lake", Count: 1, Params: map[string]any{"tile": tile, "radius": 20}}
		if err := placeLavaLake(level, spec, rand.New(rand.NewSource(1))); err != nil {
			t.Fatal(err)
		}
		if level.GetEntityAt(8, 8, z) != nil {
			t.Errorf("%s lake left an entity on a tile it covered", tile)
		}
	}
}

// Surface scatter must not place into lakes, solid tiles or water.
func TestScatterEntityAvoidsBlockedTiles(t *testing.T) {
	loadPlacementData(t)
	level := flatLevel(t, 16, 16)
	z := level.SurfaceZ
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			switch {
			case x < 5:
				level.SetFloor(x, y, z, "lava", 0)
			case x < 10:
				level.SetMiddle(x, y, z, "ice", 0)
			case x < 15:
				level.SetMiddle(x, y, z, "water", 0)
			}
		}
	}
	spec := FeatureSpec{Kind: "scatter_entity", Count: 40, Params: map[string]any{"blueprint": "round_tree"}}
	if err := placeScatterEntity(level, spec, rand.New(rand.NewSource(3))); err != nil {
		t.Fatal(err)
	}
	if len(level.StaticEntities)+len(level.Entities) == 0 {
		t.Fatal("nothing placed on the open strip")
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 15; x++ {
			if level.GetEntityAt(x, y, z) != nil {
				t.Fatalf("entity placed on blocked tile %d,%d", x, y)
			}
		}
	}
}

// Painting a solid scatter tile must not bury an existing entity.
func TestScatterTileSkipsOccupiedTiles(t *testing.T) {
	loadPlacementData(t)
	level := flatLevel(t, 4, 4)
	z := level.SurfaceZ
	e, err := factory.Create("round_tree", 1, 1, z)
	if err != nil {
		t.Fatal(err)
	}
	level.AddEntity(e)
	spec := FeatureSpec{Kind: "scatter_tile", Count: 40, Params: map[string]any{"tile": "ice"}}
	if err := placeScatterTile(level, spec, rand.New(rand.NewSource(3))); err != nil {
		t.Fatal(err)
	}
	if got := middleName(level, 1, 1, z); got == "ice" {
		t.Error("ice painted over an entity")
	}
}

// captureStamps records where a stamp placer runs its script.
func captureStamps(t *testing.T) *[][2]int {
	t.Helper()
	got := &[][2]int{}
	prev := structureRunner
	t.Cleanup(func() { structureRunner = prev })
	SetStructureRunner(func(_ *world.Level, _ string, x, y, _, _ int, _ map[string]any, _ int64) error {
		*got = append(*got, [2]int{x, y})
		return nil
	})
	return got
}

// A stamp prefers open ground when the map has room for it.
func TestStampPrefersOpenGround(t *testing.T) {
	loadPlacementData(t)
	level := flatLevel(t, 64, 64)
	z := level.SurfaceZ
	// Sea everywhere except one 20x20 island.
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if x < 20 || x >= 40 || y < 20 || y >= 40 {
				level.SetMiddle(x, y, z, "water", 0)
			}
		}
	}
	stamps := captureStamps(t)
	spec := FeatureSpec{Kind: "stamp", Count: 1, Params: map[string]any{"script": "x", "w": 9, "h": 9}}
	onLand := 0
	for seed := int64(0); seed < 50; seed++ {
		*stamps = nil
		if err := placeStamp(level, spec, rand.New(rand.NewSource(seed))); err != nil {
			t.Fatal(err)
		}
		if len(*stamps) != 1 {
			t.Fatalf("seed %d: placed %d stamps, want 1", seed, len(*stamps))
		}
		p := (*stamps)[0]
		cx, cy := p[0]+4, p[1]+4
		if cx >= 20 && cx < 40 && cy >= 20 && cy < 40 {
			onLand++
		}
	}
	// A random spot hits this island about 10% of the time; scoring must beat that
	// by a wide margin even though 20 samples can miss it.
	if onLand < 35 {
		t.Errorf("stamp centred on the island in %d/50 seeds", onLand)
	}
}

// With nowhere good to go a stamp still lands, inside the map.
func TestStampAlwaysLandsInsideMap(t *testing.T) {
	loadPlacementData(t)
	level := flatLevel(t, 24, 24)
	z := level.SurfaceZ
	for y := 0; y < 24; y++ {
		for x := 0; x < 24; x++ {
			level.SetMiddle(x, y, z, "water", 0)
		}
	}
	stamps := captureStamps(t)
	spec := FeatureSpec{Kind: "stamp", Count: 2, Params: map[string]any{"script": "x", "w": 17, "h": 17}}
	for seed := int64(0); seed < 50; seed++ {
		*stamps = nil
		if err := placeStamp(level, spec, rand.New(rand.NewSource(seed))); err != nil {
			t.Fatal(err)
		}
		if len(*stamps) != 2 {
			t.Fatalf("seed %d: placed %d stamps, want 2", seed, len(*stamps))
		}
		for _, p := range *stamps {
			if p[0] < 0 || p[1] < 0 || p[0]+17 > 24 || p[1]+17 > 24 {
				t.Fatalf("seed %d: footprint at %v leaves the map", seed, p)
			}
		}
	}
}
