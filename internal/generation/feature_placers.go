package generation

import (
	"math"
	"math/rand"

	"github.com/mechanical-lich/scifi_settlements/internal/factory"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
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
}

// scatter_tile — sparse single-tile decoration matching by terrain kind.
//   params: tile (string), kind (string, default "surface")
func placeScatterTile(level *world.Level, s FeatureSpec) error {
	tile := featureParamString(s, "tile", "")
	if tile == "" {
		return errFeature("scatter_tile", "params.tile required")
	}
	if !requireTile(tile) {
		return nil
	}
	wantKind := featureParamString(s, "kind", "surface")
	count := s.Count
	if count <= 0 {
		count = 50
	}
	w, h := level.GetWidth(), level.GetHeight()
	maxAttempts := count * 8
	for i := 0; i < maxAttempts; i++ {
		if count <= 0 {
			break
		}
		x := rand.Intn(w)
		y := rand.Intn(h)
		if !columnMatchesBiome(level, x, y, s.Biome) {
			continue
		}
		surfZ := level.GetSurfaceZ(x, y)
		if surfZ < 0 {
			continue
		}
		if kindString(level.GetTerrainKind(x, y, surfZ)) != wantKind {
			continue
		}
		level.UpdateTileAt(x, y, surfZ, tile, world.RandomTileVariant(tile))
		count--
	}
	return nil
}

// ore_vein — cluster of ore tiles underground.
//   params: tile (default "ore_deposit"), radius (default 3)
func placeOreVein(level *world.Level, s FeatureSpec) error {
	tile := featureParamString(s, "tile", "ore_deposit")
	if !requireTile(tile) {
		return nil
	}
	radius := featureParamInt(s, "radius", 3)
	count := s.Count
	if count <= 0 {
		count = 20
	}
	w, h, d := level.GetWidth(), level.GetHeight(), level.GetDepth()
	for placed := 0; placed < count; placed++ {
		cx := rand.Intn(w)
		cy := rand.Intn(h)
		minZ := s.MinZ
		maxZ := s.MaxZ
		if maxZ <= 0 {
			maxZ = level.SurfaceZ - 1
		}
		if minZ < 1 {
			minZ = 1
		}
		if maxZ < minZ || maxZ >= d {
			continue
		}
		cz := minZ + rand.Intn(maxZ-minZ+1)
		if !columnMatchesBiome(level, cx, cy, s.Biome) {
			continue
		}
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if dx*dx+dy*dy > radius*radius {
					continue
				}
				if rand.Intn(3) != 0 {
					continue
				}
				k := level.GetTerrainKind(cx+dx, cy+dy, cz)
				if k == world.TKUnderground || k == world.TKSubsurface {
					level.UpdateTileAt(cx+dx, cy+dy, cz, tile, world.RandomTileVariant(tile))
				}
			}
		}
	}
	return nil
}

// radiation_pocket — circular blob of radiation + 1-3 radioactive_ore seeds.
//   params: peak (default 180), radius (default 4), ore_tile (default "radioactive_ore")
func placeRadiationPocket(level *world.Level, s FeatureSpec) error {
	peak := featureParamInt(s, "peak", 180)
	if peak > 255 {
		peak = 255
	}
	radius := featureParamInt(s, "radius", 4)
	oreTile := featureParamString(s, "ore_tile", "radioactive_ore")
	oreOK := requireTile(oreTile)
	count := s.Count
	if count <= 0 {
		count = 5
	}
	w, h := level.GetWidth(), level.GetHeight()
	for i := 0; i < count; i++ {
		cx := rand.Intn(w)
		cy := rand.Intn(h)
		if !columnMatchesBiome(level, cx, cy, s.Biome) {
			continue
		}
		surfZ := level.GetSurfaceZ(cx, cy)
		if surfZ < 0 {
			continue
		}
		z := surfZ
		if s.MinZ != 0 || s.MaxZ != 0 {
			lo, hi := s.MinZ, s.MaxZ
			if hi < lo {
				hi = lo
			}
			z = lo + rand.Intn(hi-lo+1)
		}
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
				}
			}
		}
		if !oreOK {
			continue
		}
		seeds := 1 + rand.Intn(3)
		for j := 0; j < seeds; j++ {
			ox := cx + rand.Intn(3) - 1
			oy := cy + rand.Intn(3) - 1
			t := level.GetTilePtr(ox, oy, z)
			if t == nil {
				continue
			}
			def := world.TileDefinitions[t.Type]
			if def.Air || def.Space || def.Water {
				continue
			}
			world.SetTileTypeAndVariant(t, oreTile, world.RandomTileVariant(oreTile))
		}
	}
	return nil
}

// crystal_grove — surface cluster of crystal tiles + light entities.
//   params: tile (default "crystal"), radius (default 4)
func placeCrystalGrove(level *world.Level, s FeatureSpec) error {
	tile := featureParamString(s, "tile", "crystal")
	if !requireTile(tile) {
		return nil
	}
	radius := featureParamInt(s, "radius", 4)
	count := s.Count
	if count <= 0 {
		count = 8
	}
	w, h := level.GetWidth(), level.GetHeight()
	for i := 0; i < count; i++ {
		cx := rand.Intn(w)
		cy := rand.Intn(h)
		if !columnMatchesBiome(level, cx, cy, s.Biome) {
			continue
		}
		surfZ := level.GetSurfaceZ(cx, cy)
		if surfZ < 0 {
			continue
		}
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if dx*dx+dy*dy > radius*radius {
					continue
				}
				if rand.Intn(4) != 0 {
					continue
				}
				k := level.GetTerrainKind(cx+dx, cy+dy, surfZ)
				if k == world.TKSurface {
					level.UpdateTileAt(cx+dx, cy+dy, surfZ, tile, world.RandomTileVariant(tile))
				}
			}
		}
	}
	return nil
}

// lava_lake — replace surface tiles in a circle with lava.
//   params: tile (default "lava"), radius (default 5)
func placeLavaLake(level *world.Level, s FeatureSpec) error {
	tile := featureParamString(s, "tile", "lava")
	if !requireTile(tile) {
		return nil
	}
	radius := featureParamInt(s, "radius", 5)
	count := s.Count
	if count <= 0 {
		count = 3
	}
	w, h := level.GetWidth(), level.GetHeight()
	for i := 0; i < count; i++ {
		cx := rand.Intn(w)
		cy := rand.Intn(h)
		if !columnMatchesBiome(level, cx, cy, s.Biome) {
			continue
		}
		surfZ := level.GetSurfaceZ(cx, cy)
		if surfZ < 0 {
			continue
		}
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if dx*dx+dy*dy > radius*radius {
					continue
				}
				k := level.GetTerrainKind(cx+dx, cy+dy, surfZ)
				if k == world.TKSurface || k == world.TKSubsurface {
					level.UpdateTileAt(cx+dx, cy+dy, surfZ, tile, world.RandomTileVariant(tile))
				}
			}
		}
	}
	return nil
}

// derelict_pod / botany_bay / structure / fauna_spawner — entity-spawning
// placers. They need the factory, which depends on entity blueprints being
// loaded; if a blueprint is missing the placer silently no-ops the instance.

// derelict_pod — single entity at a random surface location.
//   params: blueprint (required)
func placeDerelictPod(level *world.Level, s FeatureSpec) error {
	bp := featureParamString(s, "blueprint", "")
	if bp == "" {
		return errFeature("derelict_pod", "params.blueprint required")
	}
	count := s.Count
	if count <= 0 {
		count = 1
	}
	w, h := level.GetWidth(), level.GetHeight()
	for placed := 0; placed < count; placed++ {
		for attempt := 0; attempt < 200; attempt++ {
			x := rand.Intn(w)
			y := rand.Intn(h)
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
//   params: blueprint (required), z (optional, default surface+1)
func placeFaunaSpawner(level *world.Level, s FeatureSpec) error {
	bp := featureParamString(s, "blueprint", "")
	if bp == "" {
		return errFeature("fauna_spawner", "params.blueprint required")
	}
	count := s.Count
	if count <= 0 {
		count = 10
	}
	w, h := level.GetWidth(), level.GetHeight()
	for placed := 0; placed < count; placed++ {
		for attempt := 0; attempt < 100; attempt++ {
			x := rand.Intn(w)
			y := rand.Intn(h)
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
//   params: tile (default "dirt"), plant_blueprint (optional), radius (default 3)
func placeBotanyBay(level *world.Level, s FeatureSpec) error {
	tile := featureParamString(s, "tile", "dirt")
	if !requireTile(tile) {
		return nil
	}
	plant := featureParamString(s, "plant_blueprint", "")
	radius := featureParamInt(s, "radius", 3)
	count := s.Count
	if count <= 0 {
		count = 1
	}
	anchors := level.Regions["station_room_large"]
	for i := 0; i < count; i++ {
		var cx, cy, cz int
		if i < len(anchors) {
			a := anchors[i]
			cx, cy, cz = a[0], a[1], a[2]
		} else {
			w, h := level.GetWidth(), level.GetHeight()
			cx = rand.Intn(w)
			cy = rand.Intn(h)
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
				level.UpdateTileAt(cx+dx, cy+dy, cz, tile, world.RandomTileVariant(tile))
				if plant != "" && rand.Intn(3) == 0 {
					if e, err := factory.Create(plant, cx+dx, cy+dy, cz); err == nil {
						level.AddEntity(e)
					}
				}
			}
		}
	}
	return nil
}

// structure — generic entity placer at a region tag or random open tile.
//   params: blueprint (required), region (optional, e.g. "starting_floor")
func placeStructure(level *world.Level, s FeatureSpec) error {
	bp := featureParamString(s, "blueprint", "")
	if bp == "" {
		return errFeature("structure", "params.blueprint required")
	}
	region := featureParamString(s, "region", "")
	count := s.Count
	if count <= 0 {
		count = 1
	}
	anchors := level.Regions[region]
	for i := 0; i < count; i++ {
		var x, y, z int
		if region != "" && i < len(anchors) {
			a := anchors[i]
			x, y, z = a[0], a[1], a[2]
		} else {
			x = rand.Intn(level.GetWidth())
			y = rand.Intn(level.GetHeight())
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
