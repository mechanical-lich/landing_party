package world

// RoundRoom is a bordered disc of radius r round its centre (the setup
// scripts' carve_round_room, the grey player's saucer): the cells within
// r + 0.5 of the centre (d² <= r² + r), so its four tips are flat rather
// than single protruding cells, and its outline: every disc cell with any
// neighbour (of 8) outside the disc, less the inner cell of any 2x2 block it
// would lay, so the wall steps diagonally in L-shaped runs, never touching
// only at a corner nor doubled (the autotiler draws both as posts).
type RoundRoom struct {
	R    int
	Wall map[[2]int]bool // outline cells, by offset from the centre
}

// NewRoundRoom computes the disc and outline of radius r (nil below 1).
func NewRoundRoom(r int) *RoundRoom {
	if r < 1 {
		return nil
	}
	rr := &RoundRoom{R: r, Wall: map[[2]int]bool{}}
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if !rr.In(dx, dy) {
				continue
			}
			for _, n := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
				if !rr.In(dx+n[0], dy+n[1]) {
					rr.Wall[[2]int{dx, dy}] = true
					break
				}
			}
		}
	}
	// Where the outline steps diagonally it can lay four walls in a 2x2
	// block; the one nearest the centre is not needed to close the wall.
	for dy := -r; dy < r; dy++ {
		for dx := -r; dx < r; dx++ {
			block := [][2]int{{dx, dy}, {dx + 1, dy}, {dx, dy + 1}, {dx + 1, dy + 1}}
			if !rr.Wall[block[0]] || !rr.Wall[block[1]] || !rr.Wall[block[2]] || !rr.Wall[block[3]] {
				continue
			}
			best := block[0]
			for _, c := range block[1:] {
				if c[0]*c[0]+c[1]*c[1] < best[0]*best[0]+best[1]*best[1] {
					best = c
				}
			}
			delete(rr.Wall, best)
		}
	}
	return rr
}

// In reports whether offset (dx, dy) lies in the disc.
func (rr *RoundRoom) In(dx, dy int) bool { return dx*dx+dy*dy <= rr.R*rr.R+rr.R }

// Cells is every disc offset, row by row.
func (rr *RoundRoom) Cells() [][2]int {
	var out [][2]int
	for dy := -rr.R; dy <= rr.R; dy++ {
		for dx := -rr.R; dx <= rr.R; dx++ {
			if rr.In(dx, dy) {
				out = append(out, [2]int{dx, dy})
			}
		}
	}
	return out
}

// Carve stamps the room round (cx, cy, z): floor on every disc cell (beneath
// the wall too, so a destroyed wall leaves a walkable cell), the wall on the
// outline, the middle cleared inside.
func (rr *RoundRoom) Carve(level *Level, cx, cy, z int, wallTile, floorTile string) {
	for _, c := range rr.Cells() {
		x, y := cx+c[0], cy+c[1]
		level.SetFloor(x, y, z, floorTile, TileVariantAt(floorTile, x, y, z))
		if rr.Wall[c] {
			level.SetMiddle(x, y, z, wallTile, TileVariantAt(wallTile, x, y, z))
		} else {
			level.ClearMiddle(x, y, z)
		}
	}
}
