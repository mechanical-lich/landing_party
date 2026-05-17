package generation

import (
	"github.com/mechanical-lich/landing_party/internal/world"
)

// ApplyBiomes walks every (x,y,z), looks up the column's biome, finds the
// first matching rule for the tile's TerrainKind, and rewrites the tile.
// Tiles tagged TKStructure are skipped — biomes don't override hull tiles.
func ApplyBiomes(level *world.Level) {
	if level.Terrain == nil || level.BiomeMap == nil {
		return
	}
	w, h, d := level.GetWidth(), level.GetHeight(), level.GetDepth()

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			biomeID := level.GetBiome(x, y)
			b := GetBiome(biomeID)
			if b == nil {
				continue
			}
			surfZ := level.GetSurfaceZ(x, y)
			for z := 0; z < d; z++ {
				k := level.GetTerrainKind(x, y, z)
				if k == world.TKStructure || k == world.TKVoid {
					continue
				}
				kStr := kindString(k)
				var matched *BiomeRule
				for i := range b.Rules {
					r := &b.Rules[i]
					if r.Kind != kStr {
						continue
					}
					if r.YOffset != nil {
						if surfZ < 0 {
							continue
						}
						if z-surfZ != *r.YOffset {
							continue
						}
					}
					matched = r
					break
				}
				if matched == nil {
					continue
				}
				if matched.Tile != "" {
					level.UpdateTileAt(x, y, z, matched.Tile, world.RandomTileVariant(matched.Tile))
				}
				if matched.Floor != "" {
					level.SetFloor(x, y, z, matched.Floor, world.RandomTileVariant(matched.Floor))
				}
				if matched.Middle != "" {
					level.SetMiddle(x, y, z, matched.Middle, world.RandomTileVariant(matched.Middle))
				}
				if matched.Radiation > 0 {
					if t := level.GetTilePtr(x, y, z); t != nil {
						lv := matched.Radiation
						if lv > 255 {
							lv = 255
						}
						if int(t.Radiation) < lv {
							t.Radiation = uint8(lv)
						}
					}
				}
			}
		}
	}
}
