package generation

import (
	"math"
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/world"
)

func init() {
	RegisterFeature("ore_vein", placeOreVein)
	RegisterFeature("radiation_pocket", placeRadiationPocket)
	RegisterFeature("crystal_grove", placeCrystalGrove)
	RegisterFeature("lava_lake", placeLavaLake)
	RegisterFeature("derelict_pod", placeDerelictPod)
	RegisterFeature("fauna_spawner", placeFaunaSpawner)
	RegisterFeature("botany_bay", placeBotanyBay)
	RegisterFeature("structure", placeStructure)
	RegisterFeature("scatter_tile", placeScatterTile)
	RegisterFeature("scatter_entity", placeScatterEntity)
	RegisterFeature("stamp", placeStamp)
}

// scatter_entity — sparse single-entity decoration on surface tiles matching
// a kind. Spawns via the entity factory; failures (missing blueprint) skip
// silently.
//
//	params: blueprint (string, required), kind (string, default "surface")
func placeScatterEntity(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	bp := featureParamString(s, "blueprint", "")
	if bp == "" {
		return errFeature("scatter_entity", "params.blueprint required")
	}
	// Skip the whole feature if the blueprint is unknown — otherwise every
	// anchor attempt re-hits the same failing factory.Create.
	if !factory.BlueprintExists(bp) {
		return nil
	}
	wantKind := featureParamString(s, "kind", "surface")
	placeAnchors(level, s, rng, 50, func(cx, cy int) bool {
		surfZ := level.GetSurfaceZ(cx, cy)
		if surfZ < 0 {
			return false
		}
		if kindString(level.GetTerrainKind(cx, cy, surfZ)) != wantKind {
			return false
		}
		if level.GetEntityAt(cx, cy, surfZ) != nil {
			return false
		}
		e, err := factory.Create(bp, cx, cy, surfZ)
		if err != nil {
			return false
		}
		level.AddEntity(e)
		return true
	})
	return nil
}

// scatter_tile — sparse single-tile decoration matching by terrain kind.
//
//	params: tile (string), kind (string, default "surface")
func placeScatterTile(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	tile := featureParamString(s, "tile", "")
	if tile == "" {
		return errFeature("scatter_tile", "params.tile required")
	}
	if !requireTile(tile) {
		return nil
	}
	wantKind := featureParamString(s, "kind", "surface")
	placeAnchors(level, s, rng, 50, func(cx, cy int) bool {
		surfZ := level.GetSurfaceZ(cx, cy)
		if surfZ < 0 {
			return false
		}
		if kindString(level.GetTerrainKind(cx, cy, surfZ)) != wantKind {
			return false
		}
		level.UpdateTileAt(cx, cy, surfZ, tile, world.TileVariantAt(tile, cx, cy, surfZ))
		return true
	})
	return nil
}

// ore_vein — cluster of ore tiles underground.
//
//	params: tile (default "ore_deposit"), radius (default 3),
//	        density (0..1 fill chance per cell in the disk, default 1.0 = solid)
func placeOreVein(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	tile := featureParamString(s, "tile", "ore_deposit")
	if !requireTile(tile) {
		return nil
	}
	radius := featureParamInt(s, "radius", 3)
	// density is the per-cell chance to paint within the disk: 1.0 fills it
	// solid, lower values scatter the vein. Clamped to (0, 1].
	density := featureParamFloat(s, "density", 1.0)
	if density <= 0 {
		density = 1.0
	}
	d := level.GetDepth()
	placeAnchors(level, s, rng, 20, func(cx, cy int) bool {
		minZ := s.MinZ
		maxZ := s.MaxZ
		if maxZ <= 0 {
			maxZ = level.SurfaceZ - 1
		}
		if minZ < 1 {
			minZ = 1
		}
		if maxZ < minZ || maxZ >= d {
			return false
		}
		cz := minZ + randIntn(rng, maxZ-minZ+1)
		anyPainted := false
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if dx*dx+dy*dy > radius*radius {
					continue
				}
				if density < 1.0 && rng.Float64() >= density {
					continue
				}
				k := level.GetTerrainKind(cx+dx, cy+dy, cz)
				if k == world.TKUnderground || k == world.TKSubsurface {
					level.UpdateTileAt(cx+dx, cy+dy, cz, tile, world.TileVariantAt(tile, cx+dx, cy+dy, cz))
					level.SetResourceAmount(cx+dx, cy+dy, cz, world.RollDepositRichnessRng(tile, rng))
					anyPainted = true
				}
			}
		}
		return anyPainted
	})
	return nil
}

// radiation_pocket — circular blob of radiation + 1-3 radioactive_ore seeds.
//
//	params: peak (default 180), radius (default 4), ore_tile (default "radioactive_ore")
func placeRadiationPocket(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	peak := featureParamInt(s, "peak", 180)
	if peak > 255 {
		peak = 255
	}
	radius := featureParamInt(s, "radius", 4)
	oreTile := featureParamString(s, "ore_tile", "radioactive_ore")
	oreOK := requireTile(oreTile)
	placeAnchors(level, s, rng, 5, func(cx, cy int) bool {
		surfZ := level.GetSurfaceZ(cx, cy)
		if surfZ < 0 {
			return false
		}
		z := surfZ
		if s.MinZ != 0 || s.MaxZ != 0 {
			lo, hi := s.MinZ, s.MaxZ
			if hi < lo {
				hi = lo
			}
			z = lo + randIntn(rng, hi-lo+1)
		}
		anyHit := false
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				dist2 := dx*dx + dy*dy
				if dist2 > radius*radius {
					continue
				}
				t := level.GetTilePtr(cx+dx, cy+dy, z)
				if t == nil {
					continue
				}
				dist := math.Sqrt(float64(dist2))
				lv := int(float64(peak) * (1.0 - dist/float64(radius)))
				if lv > int(t.Radiation) {
					t.Radiation = uint8(lv)
					anyHit = true
				}
			}
		}
		if oreOK {
			seeds := 1 + rng.Intn(3)
			for j := 0; j < seeds; j++ {
				ox := cx + rng.Intn(3) - 1
				oy := cy + rng.Intn(3) - 1
				t := level.GetTilePtr(ox, oy, z)
				if t == nil || t.Middle.IsEmpty() {
					continue
				}
				def := world.TileDefinitions[t.Middle.Type]
				if def.Air || def.Space || def.Water {
					continue
				}
				world.SetTileTypeAndVariant(t, oreTile, world.TileVariantAt(oreTile, ox, oy, z))
				level.SetResourceAmount(ox, oy, z, world.RollDepositRichnessRng(oreTile, rng))
			}
		}
		return anyHit
	})
	return nil
}

// crystal_grove — surface cluster of crystal entities (spawned via factory).
//
//	params: blueprint (default "alien_crystal"), radius (default 4)
func placeCrystalGrove(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	bp := featureParamString(s, "blueprint", "alien_crystal")
	// Skip the whole feature if the blueprint is unknown — otherwise every
	// anchor attempt re-hits the same failing factory.Create.
	if !factory.BlueprintExists(bp) {
		return nil
	}
	radius := featureParamInt(s, "radius", 4)
	placeAnchors(level, s, rng, 8, func(cx, cy int) bool {
		surfZ := level.GetSurfaceZ(cx, cy)
		if surfZ < 0 {
			return false
		}
		anySpawned := false
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if dx*dx+dy*dy > radius*radius {
					continue
				}
				if rng.Intn(4) != 0 {
					continue
				}
				tx, ty := cx+dx, cy+dy
				if level.GetTerrainKind(tx, ty, surfZ) != world.TKSurface {
					continue
				}
				if level.GetEntityAt(tx, ty, surfZ) != nil {
					continue
				}
				e, err := factory.Create(bp, tx, ty, surfZ)
				if err != nil {
					return anySpawned // unexpected create failure; keep what landed
				}
				level.AddEntity(e)
				anySpawned = true
			}
		}
		return anySpawned
	})
	return nil
}

// lava_lake — replace surface tiles in a circle with lava.
//
//	params: tile (default "lava"), radius (default 5)
func placeLavaLake(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	tile := featureParamString(s, "tile", "lava")
	if !requireTile(tile) {
		return nil
	}
	radius := featureParamInt(s, "radius", 5)
	placeAnchors(level, s, rng, 3, func(cx, cy int) bool {
		surfZ := level.GetSurfaceZ(cx, cy)
		if surfZ < 0 {
			return false
		}
		anyPainted := false
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if dx*dx+dy*dy > radius*radius {
					continue
				}
				k := level.GetTerrainKind(cx+dx, cy+dy, surfZ)
				if k == world.TKSurface || k == world.TKSubsurface {
					level.UpdateTileAt(cx+dx, cy+dy, surfZ, tile, world.TileVariantAt(tile, cx+dx, cy+dy, surfZ))
					anyPainted = true
				}
			}
		}
		return anyPainted
	})
	return nil
}

// derelict_pod / botany_bay / structure / fauna_spawner — entity-spawning
// placers. They need the factory, which depends on entity blueprints being
// loaded; if a blueprint is missing the placer silently no-ops the instance.

// derelict_pod — single entity at a random surface location.
//
//	params: blueprint (required)
func placeDerelictPod(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	bp := featureParamString(s, "blueprint", "")
	if bp == "" {
		return errFeature("derelict_pod", "params.blueprint required")
	}
	count := rollCount(s, rng, 1)
	w, h := level.GetWidth(), level.GetHeight()
	for placed := 0; placed < count; placed++ {
		for attempt := 0; attempt < 200; attempt++ {
			x := rng.Intn(w)
			y := rng.Intn(h)
			if !columnMatchesBiome(level, x, y, s.Biome) {
				continue
			}
			surfZ := level.GetSurfaceZ(x, y)
			if surfZ < 0 {
				continue
			}
			z := surfZ + 1 // place on the air tile above surface
			if z >= level.GetDepth() {
				continue
			}
			t := level.GetTilePtr(x, y, z)
			if t == nil || t.IsSolid() {
				continue
			}
			e, err := factory.Create(bp, x, y, z)
			if err != nil {
				continue
			}
			level.AddEntity(e)
			break
		}
	}
	return nil
}

// fauna_spawner — repeated entity creation at random open tiles.
//
//	params: blueprint (required), z (optional, default surface+1)
func placeFaunaSpawner(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	bp := featureParamString(s, "blueprint", "")
	if bp == "" {
		return errFeature("fauna_spawner", "params.blueprint required")
	}
	count := rollCount(s, rng, 10)
	w, h := level.GetWidth(), level.GetHeight()
	for placed := 0; placed < count; placed++ {
		for attempt := 0; attempt < 100; attempt++ {
			x := rng.Intn(w)
			y := rng.Intn(h)
			if !columnMatchesBiome(level, x, y, s.Biome) {
				continue
			}
			surfZ := level.GetSurfaceZ(x, y)
			if surfZ < 0 {
				continue
			}
			z := surfZ + 1
			if z >= level.GetDepth() {
				continue
			}
			t := level.GetTilePtr(x, y, z)
			if t == nil || t.IsSolid() {
				continue
			}
			e, err := factory.Create(bp, x, y, z)
			if err != nil {
				continue
			}
			level.AddEntity(e)
			break
		}
	}
	return nil
}

// botany_bay — stamps a small "garden plot" of soil tiles + plant entities at
// a random region tagged station_room_large (if present), else any surface
// tile in the matching biome. Placeholder until blueprint stamper exists.
//
//	params: tile (default "dirt"), plant_blueprint (optional), radius (default 3)
func placeBotanyBay(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	tile := featureParamString(s, "tile", "dirt")
	if !requireTile(tile) {
		return nil
	}
	plant := featureParamString(s, "plant_blueprint", "")
	radius := featureParamInt(s, "radius", 3)
	count := rollCount(s, rng, 1)
	anchors := level.Regions["station_room_large"]
	for i := 0; i < count; i++ {
		var cx, cy, cz int
		if i < len(anchors) {
			a := anchors[i]
			cx, cy, cz = a[0], a[1], a[2]
		} else {
			w, h := level.GetWidth(), level.GetHeight()
			cx = rng.Intn(w)
			cy = rng.Intn(h)
			cz = level.GetSurfaceZ(cx, cy)
			if cz < 0 {
				continue
			}
		}
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if dx*dx+dy*dy > radius*radius {
					continue
				}
				level.UpdateTileAt(cx+dx, cy+dy, cz, tile, world.TileVariantAt(tile, cx+dx, cy+dy, cz))
				if plant != "" && rng.Intn(3) == 0 {
					if e, err := factory.Create(plant, cx+dx, cy+dy, cz); err == nil {
						level.AddEntity(e)
					}
				}
			}
		}
	}
	return nil
}

// stamp — runs a structure script at a surface location found during generation.
//
//	params: script (string, required — name of script in data/scripts/structures/),
//	        w (int, default 9), h (int, default 7)
func placeStamp(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	script := featureParamString(s, "script", "")
	if script == "" {
		return errFeature("stamp", "params.script required")
	}
	w := featureParamInt(s, "w", 9)
	h := featureParamInt(s, "h", 7)
	count := rollCount(s, rng, 1)
	placed := 0
	for attempt := 0; placed < count && attempt < count*20; attempt++ {
		cx, cy, ok := pickFeatureCenter(level, s, rng)
		if !ok {
			break
		}
		if !columnMatchesBiome(level, cx, cy, s.Biome) {
			continue
		}
		surfZ := level.GetSurfaceZ(cx, cy)
		if surfZ < 0 {
			continue
		}
		x := cx - w/2
		y := cy - h/2
		// Derive the script's seed from the build stream so it stays
		// reproducible while remaining distinct per stamp.
		if err := runStructure(level, script, x, y, w, h, rng.Int63()); err != nil {
			return err
		}
		placed++
	}
	return nil
}

// structure — generic entity placer at a region tag or random open tile.
//
//	params: blueprint (required), region (optional, e.g. "starting_floor")
func placeStructure(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	bp := featureParamString(s, "blueprint", "")
	if bp == "" {
		return errFeature("structure", "params.blueprint required")
	}
	region := featureParamString(s, "region", "")
	count := rollCount(s, rng, 1)
	anchors := level.Regions[region]
	for i := 0; i < count; i++ {
		var x, y, z int
		if region != "" && i < len(anchors) {
			a := anchors[i]
			x, y, z = a[0], a[1], a[2]
		} else {
			x = rng.Intn(level.GetWidth())
			y = rng.Intn(level.GetHeight())
			z = level.GetSurfaceZ(x, y)
			if z < 0 {
				continue
			}
		}
		e, err := factory.Create(bp, x, y, z)
		if err != nil {
			continue
		}
		level.AddEntity(e)
	}
	return nil
}
