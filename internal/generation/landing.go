package generation

import (
	"log"
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// The landing reserve. BuildWorld picks where the colony will land before any
// feature runs, from the site's own seed (never the global rand), and tags it
// as the LandingZoneRegion. Stamps, anchored features and resolved origins
// keep out of it, so history evidence never stands on the landing and a hazard
// never spawns beside it. The game's plaza search (findStartingPlaza) lands on
// it after the primers' own anchors.

// LandingZoneRegion is the region tag at the landing reserve's centre.
const LandingZoneRegion = "landing_zone"

// landingSalt keeps the reserve's search on its own stream, so picking it moves
// none of the feature rolls.
const landingSalt int64 = 0x4c616e64 // "Land"

// landingPrimerTags are the primers' landing anchors, in the order the plaza
// search tries them.
var landingPrimerTags = []string{"starting_floor", "starting_asteroid", "station_hub"}

// landingSlots is the plaza width the game needs (findStartingPlaza's slots).
const landingSlots = 5

// plazaFits reports whether a row of landingSlots tiles centred on (x, y) at z
// can be stood on: the game's own plaza test.
func plazaFits(level *world.Level, x, y, z int) bool {
	half := landingSlots / 2
	for i := -half; i < landingSlots-half; i++ {
		t := level.GetTilePtr(x+i, y, z)
		if t == nil || t.IsSolid() || t.IsWater() || world.IsSpaceTile(t) || level.GetEntityAt(x+i, y, z) != nil {
			return false
		}
	}
	return true
}

// PickLanding chooses the landing reserve: the first primer anchor whose plaza
// fits, else the first primer anchor at all (an asteroid's, which its setup
// script carves out), else a seeded search at the level's surface z. ok is
// false only when nothing fits anywhere.
func PickLanding(level *world.Level, seed int64) (x, y, z int, ok bool) {
	var first *[3]int
	for _, tag := range landingPrimerTags {
		for _, p := range level.Regions[tag] {
			if first == nil {
				p := p
				first = &p
			}
			if plazaFits(level, p[0], p[1], p[2]) {
				return p[0], p[1], p[2], true
			}
		}
	}
	if first != nil {
		return first[0], first[1], first[2], true
	}
	rng := rand.New(rand.NewSource(seed ^ landingSalt))
	w, h, sz := level.GetWidth(), level.GetHeight(), level.SurfaceZ
	for i := 0; i < 1000; i++ {
		x, y := rng.Intn(w), rng.Intn(h)
		if plazaFits(level, x, y, sz) {
			return x, y, sz, true
		}
	}
	return 0, 0, 0, false
}

// ReserveLanding picks the landing reserve and tags it on the level. It
// reports whether one was found.
func ReserveLanding(level *world.Level, seed int64) bool {
	x, y, z, ok := PickLanding(level, seed)
	if !ok {
		log.Printf("landing reserve: no plaza fits on this level")
		return false
	}
	level.TagRegion(LandingZoneRegion, x, y, z)
	return true
}

// Landing returns the level's landing reserve centre, if it has one.
func Landing(level *world.Level) (x, y, z int, ok bool) {
	if a := level.Regions[LandingZoneRegion]; len(a) > 0 {
		return a[0][0], a[0][1], a[0][2], true
	}
	return 0, 0, 0, false
}

// landingDistance is how far (Chebyshev, in tiles) (x, y) lies outside the
// reserve box: 0 inside it, and a large number without a reserve.
func landingDistance(level *world.Level, x, y int) int {
	lx, ly, _, ok := Landing(level)
	if !ok {
		return 1 << 20
	}
	half := Budget().LandingHalf
	return max(abs(x-lx), abs(y-ly)) - half
}

// rectInLanding reports whether a footprint (x, y, w, h) overlaps the reserve
// box grown by pad tiles.
func rectInLanding(level *world.Level, x, y, w, h, pad int) bool {
	lx, ly, _, ok := Landing(level)
	if !ok {
		return false
	}
	half := Budget().LandingHalf + pad
	return x <= lx+half && x+w-1 >= lx-half && y <= ly+half && y+h-1 >= ly-half
}

// pushFromLanding moves a rolled origin out of the reserve's keep-out: along
// the line from the reserve centre through it to keepOut tiles beyond the box,
// clamped to the map with margin. If the map is too small on that side it
// takes, of the eight compass directions, the one that gets furthest out.
func pushFromLanding(level *world.Level, x, y, keepOut int) (int, int) {
	lx, ly, _, ok := Landing(level)
	if !ok || landingDistance(level, x, y) >= keepOut {
		return x, y
	}
	want := Budget().LandingHalf + keepOut
	w, h := level.GetWidth(), level.GetHeight()
	const margin = 4
	place := func(dx, dy int) (int, int, int) {
		m := max(abs(dx), abs(dy))
		if m == 0 {
			dx, dy, m = 1, 0, 1
		}
		px := clampInt(lx+dx*want/m, margin, w-1-margin)
		py := clampInt(ly+dy*want/m, margin, h-1-margin)
		return px, py, max(abs(px-lx), abs(py-ly))
	}
	bx, by, bd := place(x-lx, y-ly)
	if bd >= want {
		return bx, by
	}
	for _, d := range [][2]int{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}} {
		if px, py, pd := place(d[0], d[1]); pd > bd {
			bx, by, bd = px, py, pd
		}
	}
	return bx, by
}
