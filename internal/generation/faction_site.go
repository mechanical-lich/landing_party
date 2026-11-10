package generation

import (
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// Faction sites: where a campaign faction presence (docs/developer/factions.md,
// Tier 2) builds on a level. Presences materialise after the site's history,
// so they keep out of its landing reserve and its stamps exactly as hazards do.

// factionSiteTries is how many footprints FactionSite samples.
const factionSiteTries = 400

// LandingDistance is how far (Chebyshev tiles, deck-weighted on a station)
// (x, y, z) lies outside the landing reserve box; large without a reserve.
func LandingDistance(level *world.Level, x, y, z int) int {
	return landingDistanceAt(level, x, y, z)
}

// FactionSite picks the centre of a w×h faction footprint: on open surface,
// with the whole footprint at least keepOut tiles outside the landing reserve
// and clear of every stamp already on the level, taking the best open-ground
// share of the samples drawn from rng. With no sample that keeps keepOut it
// takes the one farthest from the landing (a small map), still clear of
// stamps when it can. On a level with no surface (a station's decks) it takes
// the module farthest from the landing. ok is false when nothing fits.
func FactionSite(level *world.Level, rng *rand.Rand, w, h, keepOut int) (x, y, z int, ok bool) {
	return FactionSiteNear(level, rng, w, h, keepOut, -1, -1, 0)
}

// FactionSiteNear is FactionSite sampling only within radius of (nx, ny) (a
// hive world's presence builds by the chapter's origin), falling back to the
// whole map when nothing there fits. nx < 0 samples the whole map.
func FactionSiteNear(level *world.Level, rng *rand.Rand, w, h, keepOut, nx, ny, radius int) (x, y, z int, ok bool) {
	lw, lh := level.GetWidth(), level.GetHeight()
	if w > lw {
		w = lw
	}
	if h > lh {
		h = lh
	}
	ore := oreColumns(level)
	type cand struct {
		x, y, z, dist int
		share         float64
		clear         bool
	}
	var best, far *cand
	for i := 0; i < factionSiteTries; i++ {
		cx, cy := rng.Intn(lw), rng.Intn(lh)
		if nx >= 0 {
			cx, cy = nx+rng.Intn(2*radius+1)-radius, ny+rng.Intn(2*radius+1)-radius
			if cx < 0 || cy < 0 || cx >= lw || cy >= lh {
				continue
			}
		}
		if !standableSurface(level, cx, cy) {
			continue
		}
		fx := clampFootprint(cx-w/2, w, lw)
		fy := clampFootprint(cy-h/2, h, lh)
		cx, cy = fx+w/2, fy+h/2
		cz := level.GetSurfaceZ(cx, cy)
		if cz < 0 {
			continue
		}
		// The footprint's nearest corner to the reserve decides.
		d := min(
			landingDistance(level, fx, fy), landingDistance(level, fx+w-1, fy),
			landingDistance(level, fx, fy+h-1), landingDistance(level, fx+w-1, fy+h-1),
			landingDistance(level, cx, cy))
		c := &cand{cx, cy, cz, d, stampGroundShare(level, ore, fx, fy, w, h), !stampOverlaps(level, fx, fy, w, h, 2)}
		if c.dist >= keepOut && c.clear && (best == nil || c.share > best.share) {
			best = c
		}
		if far == nil || (c.clear && !far.clear) || (c.clear == far.clear && c.dist > far.dist) {
			far = c
		}
	}
	if best != nil {
		return best.x, best.y, best.z, true
	}
	if nx >= 0 {
		return FactionSiteNear(level, rng, w, h, keepOut, -1, -1, 0)
	}
	if far != nil {
		return far.x, far.y, far.z, true
	}
	// No surface: a station. The module farthest from the landing.
	bestD := -1
	for _, m := range level.Modules {
		mx, my := m.X+m.W/2, m.Y+m.H/2
		if d := landingDistanceAt(level, mx, my, m.Z); d > bestD {
			x, y, z, bestD = mx, my, m.Z, d
		}
	}
	return x, y, z, bestD >= 0
}

// stampOverlaps reports whether a footprint overlaps any stamp already on
// the level, grown by pad tiles.
func stampOverlaps(level *world.Level, x, y, w, h, pad int) bool {
	for _, f := range level.StampFootprints {
		if x-pad <= f[0]+f[2]-1 && x+w-1+pad >= f[0] && y-pad <= f[1]+f[3]-1 && y+h-1+pad >= f[1] {
			return true
		}
	}
	return false
}
