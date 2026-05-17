package generation

import (
	"log"
	"math"
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/world"
)

// AbandonedStationPrimer draws a hub-and-spokes station per floor: a circular
// central hub, 3-4 hallway spokes, and rooms budded off the spokes' walls.
// Mirrors spaceplant's ring/spokes layout in spirit without depending on it.
//
// Doors are placed where corridors meet rooms. Stair columns connect floors.
type AbandonedStationPrimer struct{}

func init() {
	RegisterPrimer("abandoned_station", AbandonedStationPrimer{})
}

// Prime params:
//
//	floors           int (default = level depth, capped)
//	station_size     int (default 50) — overall station diameter in tiles
//	hub_radius       int (default 5)
//	spokes           int (default 4)
//	bud_attempts     int (default 60)
//	min_room         int (default 4)
//	max_room         int (default 8)
//
// Prime ignores seed directly — it uses the package global rand, which
// BuildWorld seeds for reproducibility before priming.
func (AbandonedStationPrimer) Prime(level *world.Level, params map[string]any, seed int64) error {
	w, h, d := level.GetWidth(), level.GetHeight(), level.GetDepth()
	level.AllocTerrain()

	floors := paramInt(params, "floors", d)
	if floors > d {
		floors = d
	}
	if floors < 1 {
		floors = 1
	}
	stationSize := paramInt(params, "station_size", 50)
	hubR := paramInt(params, "hub_radius", 5)
	spokes := paramInt(params, "spokes", 4)
	budAttempts := paramInt(params, "bud_attempts", 60)
	minR := paramInt(params, "min_room", 4)
	maxR := paramInt(params, "max_room", 8)

	// Cap station to fit within the map.
	maxFootprint := minInt(w, h) - 4
	if stationSize > maxFootprint {
		stationSize = maxFootprint
	}

	level.SurfaceZ = 0
	level.AtmosphereZ = floors
	level.SpaceZ = floors

	// Fill everything with space.
	for z := 0; z < d; z++ {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				paintKind(level, x, y, z, world.TKSpace)
			}
		}
	}

	cx, cy := w/2, h/2

	for z := 0; z < floors; z++ {
		// Central hub (circle).
		carveStationCircle(level, cx, cy, z, hubR)

		// Spokes — 3 or 4 radial hallways.
		angles := make([]float64, spokes)
		angleOffset := rand.Float64() * math.Pi
		for i := 0; i < spokes; i++ {
			angles[i] = angleOffset + float64(i)*2*math.Pi/float64(spokes)
		}
		spokeLen := stationSize/2 - 4
		if spokeLen < hubR+3 {
			spokeLen = hubR + 3
		}
		spokeEndpoints := make([][2]int, spokes)
		for i, a := range angles {
			ex := cx + int(math.Cos(a)*float64(spokeLen))
			ey := cy + int(math.Sin(a)*float64(spokeLen))
			carveStationLine(level, cx, cy, ex, ey, z, 2)
			spokeEndpoints[i] = [2]int{ex, ey}
		}

		// Bud rooms off existing hull-floor cells. We pick a floor tile that
		// has at least one solid neighbor and try to attach a rectangular
		// room there with a door connecting them.
		rooms := budRooms(level, z, budAttempts, minR, maxR)

		// Drop a small "node" room at the end of each spoke for visual
		// punctuation.
		for _, p := range spokeEndpoints {
			rw := minR + rand.Intn(3)
			rh := minR + rand.Intn(3)
			rx := p[0] - rw/2
			ry := p[1] - rh/2
			if rx < 1 || ry < 1 || rx+rw >= w-1 || ry+rh >= h-1 {
				continue
			}
			stampRoom(level, rx, ry, z, rw, rh)
			// Knock a door toward the spoke.
			dx := sign(cx - p[0])
			dy := sign(cy - p[1])
			doorX := p[0] + dx
			doorY := p[1] + dy
			if level.GetTerrainKind(doorX, doorY, z) == world.TKStructure {
				// Knock through the wall: keep the Floor, clear the Middle.
				level.SetFloor(doorX, doorY, z, "hull_floor", world.RandomTileVariant("hull_floor"))
				level.ClearMiddle(doorX, doorY, z)
			}
			rooms = append(rooms, rect{rx, ry, rw, rh})
		}

		// Tag the largest room for "botany_bay" and the like.
		if len(rooms) > 0 {
			big := rooms[0]
			for _, r := range rooms[1:] {
				if r.ww*r.hh > big.ww*big.hh {
					big = r
				}
			}
			level.TagRegion("station_room_large", big.x+big.ww/2, big.y+big.hh/2, z)
		}

		// Hub itself is also a useful anchor.
		level.TagRegion("station_hub", cx, cy, z)
	}

	// Stair columns near the hub between consecutive floors. Stairs go in
	// Middle; the hull_floor underneath keeps the cell walkable.
	for z := 0; z < floors-1; z++ {
		sx, sy := cx+2, cy
		level.SetFloor(sx, sy, z, "hull_floor", world.RandomTileVariant("hull_floor"))
		level.SetFloor(sx, sy, z+1, "hull_floor", world.RandomTileVariant("hull_floor"))
		level.SetMiddle(sx, sy, z, "stairs_up", 0)
		level.SetMiddle(sx, sy, z+1, "stairs_down", 0)
		level.SetTerrainKind(sx, sy, z, world.TKStructure)
		level.SetTerrainKind(sx, sy, z+1, world.TKStructure)
	}

	// Ceiling pass: for every structure tile at z whose z+1 is not also a
	// structure floor, stamp hull_floor at z+1 to act as a sealed ceiling.
	// This prevents flying/spacefaring entities from exiting through the roof.
	// Multi-story transitions are unaffected — z+1 of a lower floor is already
	// TKStructure (the upper floor), so the check skips it.
	for z := 0; z < floors; z++ {
		ceilZ := z + 1
		if ceilZ >= d {
			continue
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if level.GetTerrainKind(x, y, z) != world.TKStructure {
					continue
				}
				if level.GetTerrainKind(x, y, ceilZ) == world.TKStructure {
					continue
				}
				level.SetFloor(x, y, ceilZ, "hull_floor", world.RandomTileVariant("hull_floor"))
				level.SetTerrainKind(x, y, ceilZ, world.TKStructure)
			}
		}
	}

	level.TagRegion("starting_floor", cx, cy, 0)
	log.Printf("AbandonedStationPrimer: %d floors (hub-spokes)", floors)
	return nil
}

type rect struct{ x, y, ww, hh int }

// stampStationCell paints a station floor cell with optional wall middle.
// Floor is always hull_floor (so destroyed walls leave a walkable cell).
// If isWall, Middle gets hull_wall — unless this cell was already cleared
// by a previous interior pass (lets corridors / doorways punch through).
func stampStationCell(level *world.Level, x, y, z int, isWall bool) {
	preserveInterior := level.GetTerrainKind(x, y, z) == world.TKStructure && interiorMiddle(level, x, y, z)
	level.SetFloor(x, y, z, "hull_floor", world.RandomTileVariant("hull_floor"))
	if isWall && !preserveInterior {
		level.SetMiddle(x, y, z, "hull_wall", world.RandomTileVariant("hull_wall"))
	} else if !isWall {
		level.ClearMiddle(x, y, z)
	}
	level.SetTerrainKind(x, y, z, world.TKStructure)
}

func interiorMiddle(level *world.Level, x, y, z int) bool {
	t := level.GetTilePtr(x, y, z)
	return t != nil && t.Middle.IsEmpty()
}

func stampRoom(level *world.Level, x, y, z, w, h int) {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			tx, ty := x+dx, y+dy
			edge := dx == 0 || dy == 0 || dx == w-1 || dy == h-1
			stampStationCell(level, tx, ty, z, edge)
		}
	}
}

func currentTileName(level *world.Level, x, y, z int) string {
	t := level.GetTilePtr(x, y, z)
	if t == nil {
		return ""
	}
	// Inspect Middle first (walls/doors/etc.), fall back to Floor.
	if !t.Middle.IsEmpty() {
		return world.TileIndexToName[t.Middle.Type]
	}
	if !t.Floor.IsEmpty() {
		return world.TileIndexToName[t.Floor.Type]
	}
	return ""
}

// carveStationCircle paints a filled disc as walls + interior floor.
func carveStationCircle(level *world.Level, cx, cy, z, r int) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			d2 := dx*dx + dy*dy
			if d2 > r*r {
				continue
			}
			stampStationCell(level, cx+dx, cy+dy, z, d2 >= (r-1)*(r-1))
		}
	}
}

// carveStationLine carves a thick corridor between two points: floor tiles in
// the centre, walls along the edges. width = total corridor width.
func carveStationLine(level *world.Level, x0, y0, x1, y1, z, width int) {
	dx := x1 - x0
	dy := y1 - y0
	steps := absInt(dx)
	if absInt(dy) > steps {
		steps = absInt(dy)
	}
	if steps == 0 {
		return
	}
	half := width / 2
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		px := x0 + int(float64(dx)*t)
		py := y0 + int(float64(dy)*t)
		// Floor strip + wall border perpendicular to direction.
		for ox := -half - 1; ox <= half+1; ox++ {
			for oy := -half - 1; oy <= half+1; oy++ {
				tx, ty := px+ox, py+oy
				if tx < 1 || ty < 1 || tx >= level.GetWidth()-1 || ty >= level.GetHeight()-1 {
					continue
				}
				edge := absInt(ox) == half+1 || absInt(oy) == half+1
				stampStationCell(level, tx, ty, z, edge)
			}
		}
	}
}

// budRooms scans existing wall tiles, picks ones adjacent to floor on the
// inside, and tries to attach a rectangular room on the outside with a door
// at the contact wall.
func budRooms(level *world.Level, z, attempts, minR, maxR int) []rect {
	w, h := level.GetWidth(), level.GetHeight()
	rooms := []rect{}

	for tries := 0; tries < attempts; tries++ {
		// Pick a random wall tile.
		x := 2 + rand.Intn(w-4)
		y := 2 + rand.Intn(h-4)
		if currentTileName(level, x, y, z) != "hull_wall" {
			continue
		}
		// Find which side has open hull_floor ("inside") and which side is
		// empty space ("outside" — where the new room goes).
		var outDX, outDY int
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			if currentTileName(level, x+d[0], y+d[1], z) == "hull_floor" {
				outDX = -d[0]
				outDY = -d[1]
				break
			}
		}
		if outDX == 0 && outDY == 0 {
			continue
		}
		rw := minR + rand.Intn(maxR-minR+1)
		rh := minR + rand.Intn(maxR-minR+1)
		var rx, ry int
		if outDX != 0 {
			// Room extends horizontally.
			if outDX > 0 {
				rx = x + 1
			} else {
				rx = x - rw
			}
			ry = y - rh/2
		} else {
			if outDY > 0 {
				ry = y + 1
			} else {
				ry = y - rh
			}
			rx = x - rw/2
		}
		if rx < 1 || ry < 1 || rx+rw >= w-1 || ry+rh >= h-1 {
			continue
		}
		// Reject if any cell of the new room is already a hull tile.
		conflict := false
		for dy := 0; dy < rh && !conflict; dy++ {
			for dx := 0; dx < rw && !conflict; dx++ {
				if level.GetTerrainKind(rx+dx, ry+dy, z) == world.TKStructure {
					conflict = true
				}
			}
		}
		if conflict {
			continue
		}
		stampRoom(level, rx, ry, z, rw, rh)
		// Knock a door at the bud wall: clear the Middle, keep Floor.
		level.SetFloor(x, y, z, "hull_floor", world.RandomTileVariant("hull_floor"))
		level.ClearMiddle(x, y, z)
		rooms = append(rooms, rect{rx, ry, rw, rh})
	}
	return rooms
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func sign(v int) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
