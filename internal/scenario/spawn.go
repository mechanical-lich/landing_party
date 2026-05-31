package scenario

import (
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/utility"
)

const lightMaxUnset = 0

func (r SpawnRule) lightMatches(lightLevel int) bool {
	if lightLevel < r.LightMin {
		return false
	}
	if r.LightMax != lightMaxUnset && lightLevel > r.LightMax {
		return false
	}
	return true
}

// zMatches converts the rule's surface-relative Z deltas into absolute Z
// bounds for surfaceZ and checks z against them. nil deltas mean "no bound on
// that side", so a rule with both deltas nil always matches.
func (r SpawnRule) zMatches(z, surfaceZ int) bool {
	if r.MinZDelta != nil && z < surfaceZ+*r.MinZDelta {
		return false
	}
	if r.MaxZDelta != nil && z > surfaceZ+*r.MaxZDelta {
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

// Matches reports whether rule applies at the tile described by (tileName,
// lightLevel, z) on a level whose surface lives at surfaceZ.
func (r SpawnRule) Matches(tileName string, lightLevel, z, surfaceZ int) bool {
	return r.SpawnRate > 0 && r.zMatches(z, surfaceZ) && r.lightMatches(lightLevel) && r.tileMatches(tileName)
}

// PickRandom picks a blueprint key from rules weighted by SpawnRate, honoring
// each rule's surface-relative Z range against surfaceZ.
func PickRandom(rules map[string]SpawnRule, tileName string, lightLevel, z, surfaceZ int) string {
	total := 0
	for _, r := range rules {
		if r.Matches(tileName, lightLevel, z, surfaceZ) {
			total += r.SpawnRate
		}
	}
	if total == 0 {
		return ""
	}
	pick := utility.GetRandom(0, total)
	for bp, r := range rules {
		if r.Matches(tileName, lightLevel, z, surfaceZ) {
			pick -= r.SpawnRate
			if pick < 0 {
				return bp
			}
		}
	}
	return ""
}

// SpawnTiles enumerates the tiles on layer z where rule's tile constraint
// matches, after gating on the rule's Z delta range against level.SurfaceZ.
func SpawnTiles(level *world.Level, z int, rule SpawnRule) [][2]int {
	if !rule.zMatches(z, level.SurfaceZ) {
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
