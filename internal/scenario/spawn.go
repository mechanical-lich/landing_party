package scenario

import (
	"github.com/mechanical-lich/mlge/utility"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

const maxZUnset = 0

func (r SpawnRule) lightMatches(lightLevel int) bool {
	if lightLevel < r.LightMin {
		return false
	}
	if r.LightMax != maxZUnset && lightLevel > r.LightMax {
		return false
	}
	return true
}

func (r SpawnRule) zMatches(z int) bool {
	if z < r.MinZ {
		return false
	}
	if r.MaxZ != maxZUnset && z > r.MaxZ {
		return false
	}
	return true
}

func (r SpawnRule) tileMatches(tileName string) bool {
	if len(r.Tiles) == 0 {
		return true
	}
	return utility.Contains(r.Tiles, tileName)
}

func (r SpawnRule) Matches(tileName string, lightLevel, z int) bool {
	return r.SpawnRate > 0 && r.zMatches(z) && r.lightMatches(lightLevel) && r.tileMatches(tileName)
}

func PickRandom(rules map[string]SpawnRule, tileName string, lightLevel, z int) string {
	total := 0
	for _, r := range rules {
		if r.Matches(tileName, lightLevel, z) {
			total += r.SpawnRate
		}
	}
	if total == 0 {
		return ""
	}
	pick := utility.GetRandom(0, total)
	for bp, r := range rules {
		if r.Matches(tileName, lightLevel, z) {
			pick -= r.SpawnRate
			if pick < 0 {
				return bp
			}
		}
	}
	return ""
}

func SpawnTiles(level *world.Level, z int, rule SpawnRule) [][2]int {
	if !rule.zMatches(z) {
		return nil
	}
	var out [][2]int
	w := level.GetWidth()
	h := level.GetHeight()
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			tI := level.GetTileAt(x, y, z)
			if tI == nil {
				continue
			}
			t := tI.(*world.Tile)
			if t.IsSolid() || t.IsWater() {
				continue
			}
			// Match against whichever slot has a tile — Floor wins for
			// surface checks (e.g. "grass", "regolith").
			matchSlot := t.Floor
			if matchSlot.IsEmpty() {
				matchSlot = t.Middle
			}
			if matchSlot.IsEmpty() {
				continue
			}
			name := world.TileIndexToName[matchSlot.Type]
			if !rule.tileMatches(name) {
				continue
			}
			out = append(out, [2]int{x, y})
		}
	}
	return out
}
