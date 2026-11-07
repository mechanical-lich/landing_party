package generation

import (
	"strings"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// Origins are the places on a map where a site's history happened (the wreck,
// the reactor, where a plague began). History rolls each as a surface tile when
// the site is charted; BuildWorld resolves every one, once, to the nearest open
// surface before any feature is placed, and stores the result on the level so
// every consumer (an anchored feature, a setup script, a datapad) uses the same
// tile.

// ChapterOrigin is the origin key "at": "origin" means in map and scenario
// features: the site's chapter.
const ChapterOrigin = "origin"

// originRadius bounds how far an origin may move to reach open ground.
const originRadius = 48

// originFlag is the level flag holding one coordinate of a resolved origin.
func originFlag(key, axis string) string {
	return "origin:" + strings.TrimPrefix(key, "origin:") + ":" + axis
}

// SetOrigin records a resolved origin on the level (in its saved flags).
func SetOrigin(level *world.Level, key string, x, y, z int) {
	level.Flags[originFlag(key, "x")] = float64(x)
	level.Flags[originFlag(key, "y")] = float64(y)
	level.Flags[originFlag(key, "z")] = float64(z)
}

// Origin returns a resolved origin by key: "origin" for the chapter's, or an
// event ID (with or without an "origin:" prefix).
func Origin(level *world.Level, key string) (x, y, z int, ok bool) {
	fx, okx := level.Flags[originFlag(key, "x")].(float64)
	fy, oky := level.Flags[originFlag(key, "y")].(float64)
	fz, okz := level.Flags[originFlag(key, "z")].(float64)
	if !okx || !oky || !okz {
		return 0, 0, 0, false
	}
	return int(fx), int(fy), int(fz), true
}

// ResolveOrigin moves a rolled tile to the nearest open surface, scoring each
// candidate like a stamp footprint (the share of standable surface in a 7x7
// box). It takes the nearest candidate that is a good fit, else the best seen,
// else the rolled tile clamped to the map at the level's surface. Deterministic:
// no randomness, so the same terrain gives the same tile.
func ResolveOrigin(level *world.Level, x, y int) (int, int, int) {
	w, h := level.GetWidth(), level.GetHeight()
	x = clampInt(x, 0, w-1)
	y = clampInt(y, 0, h-1)
	const box = 7
	bestX, bestY, best := -1, -1, -1.0
	for r := 0; r <= originRadius; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dy)) != r {
					continue // the ring at distance r only
				}
				cx, cy := x+dx, y+dy
				if cx < 0 || cy < 0 || cx >= w || cy >= h || !standableSurface(level, cx, cy) {
					continue
				}
				share := openGroundShare(level, clampFootprint(cx-box/2, box, w), clampFootprint(cy-box/2, box, h), box, box)
				if share > best {
					bestX, bestY, best = cx, cy, share
				}
			}
		}
		if best >= stampGoodFit {
			break
		}
	}
	if bestX >= 0 {
		return bestX, bestY, level.GetSurfaceZ(bestX, bestY)
	}
	// No open surface in reach (a station, where everything is decks in
	// space): the nearest walkable floor on the level's main deck.
	if fx, fy, ok := nearestFloor(level, x, y, level.SurfaceZ); ok {
		return fx, fy, level.SurfaceZ
	}
	z := level.GetSurfaceZ(x, y)
	if z < 0 {
		z = level.SurfaceZ
	}
	return x, y, z
}

// nearestFloor finds the closest standable, non-space tile at z by rings.
func nearestFloor(level *world.Level, x, y, z int) (int, int, bool) {
	w, h := level.GetWidth(), level.GetHeight()
	for r := 0; r < max(w, h); r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dy)) != r {
					continue
				}
				cx, cy := x+dx, y+dy
				if cx < 0 || cy < 0 || cx >= w || cy >= h {
					continue
				}
				if t := level.GetTilePtr(cx, cy, z); t != nil && standableTile(level, cx, cy, z) && !world.IsSpaceTile(t) {
					return cx, cy, true
				}
			}
		}
	}
	return 0, 0, false
}

// ResolveOrigins resolves every rolled origin and stores it on the level.
// Keys are event IDs; chapter, if set, is also stored as the "origin" key.
func ResolveOrigins(level *world.Level, rolled map[string][2]int, chapter string) {
	for _, key := range sortedOriginKeys(rolled) {
		p := rolled[key]
		x, y, z := ResolveOrigin(level, p[0], p[1])
		SetOrigin(level, key, x, y, z)
		if key == chapter {
			SetOrigin(level, ChapterOrigin, x, y, z)
		}
	}
}

func sortedOriginKeys(m map[string][2]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// anchorable lists the feature kinds whose placer draws candidates from the
// shared sampler, so "at" can anchor them.
var anchorable = map[string]bool{
	"stamp": true, "scatter_entity": true, "scatter_tile": true, "ore_vein": true,
	"radiation_pocket": true, "crystal_grove": true, "lava_lake": true,
	"cave_spawner": true, "buried_spawner": true,
}

// Anchorable reports whether a feature kind honours "at".
func Anchorable(kind string) bool { return anchorable[kind] }
