package generation

import (
	"log"
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
	RegisterFeature("cave_spawner", placeCaveSpawner)
	RegisterFeature("buried_spawner", placeBuriedSpawner)
}

// standableTile reports whether something can stand on this tile: it exists and
// neither slot blocks it. Tile.IsSolid only reads the Middle slot, and lava is a
// solid Floor, so the Floor is checked here too.
func standableTile(level *world.Level, x, y, z int) bool {
	t := level.GetTilePtr(x, y, z)
	if t == nil || t.IsSolid() || t.IsWater() {
		return false
	}
	return t.Floor.IsEmpty() || !world.TileDefinitions[t.Floor.Type].Solid
}

// standableSurface reports whether column (x,y) is open surface. Mountain columns
// have a surface z too, but their tile there is solid rock of another kind.
func standableSurface(level *world.Level, x, y int) bool {
	surfZ := level.GetSurfaceZ(x, y)
	return surfZ >= 0 && level.GetTerrainKind(x, y, surfZ) == world.TKSurface && standableTile(level, x, y, surfZ)
}

// clearBlockedEntities removes whatever stands on (x,y,z) once a paint has made
// the tile unstandable, so nothing is left embedded in it.
func clearBlockedEntities(level *world.Level, x, y, z int) {
	if standableTile(level, x, y, z) {
		return
	}
	for e := level.GetEntityAt(x, y, z); e != nil; e = level.GetEntityAt(x, y, z) {
		level.RemoveEntity(e)
	}
}

// spawnTargets collects the (x,y,z) tiles a cave/buried spawner may use: open
// cavern tiles when solid is false, solid rock (underground/subsurface) when
// true. zone filters by elevation relative to the surface: "underground"
// (below), "mountain" (above), else both. Built in a fixed order so sampling
// from it stays reproducible.
func spawnTargets(level *world.Level, solid bool, zone string) [][3]int {
	w, h, d := level.GetWidth(), level.GetHeight(), level.GetDepth()
	surfZ := level.SurfaceZ
	var out [][3]int
	for z := 0; z < d; z++ {
		switch zone {
		case "underground":
			if z >= surfZ {
				continue
			}
		case "mountain":
			if z <= surfZ {
				continue
			}
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				k := level.GetTerrainKind(x, y, z)
				match := k == world.TKCavern
				if solid {
					match = k == world.TKUnderground || k == world.TKSubsurface
				}
				if match {
					out = append(out, [3]int{x, y, z})
				}
			}
		}
	}
	return out
}

// spawnInTiles places up to a rolled/area-scaled count of bp at random unoccupied
// tiles from targets. Shared by the cave and buried spawners.
func spawnInTiles(level *world.Level, s FeatureSpec, rng *rand.Rand, bp string, targets [][3]int, defCount int) {
	count := rollCount(s, rng, defCount)
	// Anchored at an origin: only targets within reach of it (Jitter, default
	// 12), and the count is absolute rather than scaled by map area.
	if s.At != "" {
		if ox, oy, _, ok := Origin(level, s.At); ok {
			reach := s.Jitter
			if reach <= 0 {
				reach = 12
			}
			var near [][3]int
			for _, t := range targets {
				if dx, dy := t[0]-ox, t[1]-oy; dx*dx+dy*dy <= reach*reach {
					near = append(near, t)
				}
			}
			targets = near
		}
	} else if m := level.FeatureAreaMultiplier(); m != 1 {
		count = int(math.Round(float64(count) * m))
		if count < 1 {
			count = 1
		}
	}
	zone := featureParamString(s, "zone", "any")
	if len(targets) == 0 {
		log.Printf("feature %q: placed 0/%d (no matching tiles for zone %q)", s.Kind, count, zone)
		return
	}
	placed := 0
	for tries := 0; placed < count && tries < count*8; tries++ {
		t := targets[rng.Intn(len(targets))]
		if level.GetEntityAt(t[0], t[1], t[2]) != nil {
			continue
		}
		e, err := factory.Create(bp, t[0], t[1], t[2])
		if err != nil {
			return
		}
		level.AddEntity(e)
		placed++
	}
	if placed < count {
		log.Printf("feature %q: placed %d/%d (not enough free tiles for zone %q)", s.Kind, placed, count, zone)
	}
}

// cave_spawner — spawns entities in open cavern tiles (cave-dwelling creatures,
// loot), reached by mining in.
//
//	params: blueprint (required), zone ("any"|"underground"|"mountain", default any)
func placeCaveSpawner(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	bp := featureParamString(s, "blueprint", "")
	if bp == "" {
		return errFeature("cave_spawner", "params.blueprint required")
	}
	if !factory.BlueprintExists(bp) {
		return nil
	}
	zone := featureParamString(s, "zone", "any")
	spawnInTiles(level, s, rng, bp, spawnTargets(level, false, zone), 10)
	return nil
}

// buried_spawner — spawns entities inside solid rock, revealed by mining
// (buried caches, fossils, dormant creatures).
//
//	params: blueprint (required), zone ("any"|"underground"|"mountain", default any)
func placeBuriedSpawner(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	bp := featureParamString(s, "blueprint", "")
	if bp == "" {
		return errFeature("buried_spawner", "params.blueprint required")
	}
	if !factory.BlueprintExists(bp) {
		return nil
	}
	zone := featureParamString(s, "zone", "any")
	spawnInTiles(level, s, rng, bp, spawnTargets(level, true, zone), 5)
	return nil
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
		if !standableTile(level, cx, cy, surfZ) || level.GetEntityAt(cx, cy, surfZ) != nil {
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
		if level.GetEntityAt(cx, cy, surfZ) != nil {
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
//	        radius_max (optional; when > radius, each vein's radius rolls in [radius, radius_max]),
//	        density (0..1 fill chance per cell in the disk, default 1.0 = solid)
func placeOreVein(level *world.Level, s FeatureSpec, rng *rand.Rand) error {
	tile := featureParamString(s, "tile", "ore_deposit")
	if !requireTile(tile) {
		return nil
	}
	radius := featureParamInt(s, "radius", 3)
	radiusMax := featureParamInt(s, "radius_max", 0)
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
		r := rollRange(rng, radius, radiusMax) // per-vein size
		anyPainted := false
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx*dx+dy*dy > r*r {
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
				if !oreSeedable(level, ox, oy, z) {
					continue
				}
				level.UpdateTileAt(ox, oy, z, oreTile, world.TileVariantAt(oreTile, ox, oy, z))
				level.SetResourceAmount(ox, oy, z, world.RollDepositRichnessRng(oreTile, rng))
			}
		}
		return anyHit
	})
	return nil
}

// oreSeedable reports whether a radiation pocket may put ore on this tile: solid
// ground it replaces, or open ground it stands on. Air, space, water, lava and
// occupied tiles are skipped.
func oreSeedable(level *world.Level, x, y, z int) bool {
	t := level.GetTilePtr(x, y, z)
	if t == nil || level.GetEntityAt(x, y, z) != nil {
		return false
	}
	if t.Middle.IsEmpty() {
		return !t.Floor.IsEmpty() && !world.TileDefinitions[t.Floor.Type].Solid
	}
	def := world.TileDefinitions[t.Middle.Type]
	return !def.Air && !def.Space && !def.Water
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
				if !standableSurface(level, tx, ty) {
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
					clearBlockedEntities(level, cx+dx, cy+dy, surfZ)
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

const (
	// stampAttempts is how many candidate spots each stamp samples.
	stampAttempts = 20
	// stampGoodFit is the open-ground share of a footprint that ends the search.
	stampGoodFit = 0.9
)

// clampFootprint moves a footprint's start so it stays inside the map along one
// axis, or pins it to 0 if the map is smaller than the footprint.
func clampFootprint(start, size, mapSize int) int {
	if start > mapSize-size {
		start = mapSize - size
	}
	if start < 0 {
		start = 0
	}
	return start
}

// openGroundShare returns the fraction of a footprint that is standable surface.
func openGroundShare(level *world.Level, x, y, w, h int) float64 {
	open, total := 0, 0
	for fy := y; fy < y+h && fy < level.GetHeight(); fy++ {
		for fx := x; fx < x+w && fx < level.GetWidth(); fx++ {
			total++
			if standableSurface(level, fx, fy) {
				open++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(open) / float64(total)
}

// stamp — runs a structure script at a surface location found during generation.
// Each stamp samples candidate spots and takes the first that is mostly open
// ground, else the best seen, so a scenario's structure is never silently dropped
// on a map with little open land.
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
	next, ok := featureSampler(level, s, rng)
	if !ok {
		return nil // region/biome not present on this map
	}
	for placed := 0; placed < count; placed++ {
		bestX, bestY, bestShare := 0, 0, -1.0
		for attempt := 0; attempt < stampAttempts && bestShare < stampGoodFit; attempt++ {
			cx, cy := next()
			if s.InRegion != "" && !columnMatchesBiome(level, cx, cy, s.Biome) {
				continue
			}
			if level.GetSurfaceZ(cx, cy) < 0 {
				continue
			}
			x := clampFootprint(cx-w/2, w, level.GetWidth())
			y := clampFootprint(cy-h/2, h, level.GetHeight())
			if share := openGroundShare(level, x, y, w, h); share > bestShare {
				bestX, bestY, bestShare = x, y, share
			}
		}
		if bestShare < 0 {
			continue // every candidate was off the terrain
		}
		// Derive the script's seed from the build stream so it stays
		// reproducible while remaining distinct per stamp. Params other than
		// the footprint pass through to the script.
		if err := runStructure(level, script, bestX, bestY, w, h, stampParams(s.Params), rng.Int63()); err != nil {
			return err
		}
	}
	return nil
}

// stampParams is a stamp's params minus the ones the placer itself reads.
func stampParams(p map[string]any) map[string]any {
	var out map[string]any
	for k, v := range p {
		if k == "script" || k == "w" || k == "h" {
			continue
		}
		if out == nil {
			out = map[string]any{}
		}
		out[k] = v
	}
	return out
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
			// Find a standable surface column — mountain columns are solid at
			// the surface z, so a naive GetSurfaceZ would bury the entity.
			found := false
			for attempt := 0; attempt < 40; attempt++ {
				x = rng.Intn(level.GetWidth())
				y = rng.Intn(level.GetHeight())
				z = level.GetSurfaceZ(x, y)
				if z < 0 {
					continue
				}
				if t := level.GetTilePtr(x, y, z); t != nil && !t.IsSolid() {
					found = true
					break
				}
			}
			if !found {
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
