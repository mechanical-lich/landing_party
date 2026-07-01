// Package ship builds and manages The Ship — the campaign's always-loaded home
// base level where colonists not beamed down live. See
// docs/developer/the_ship.md.
package ship

import (
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// Ship level dimensions. The level is large (100x100x4) so the ship can grow,
// but the starting structure is a compact grid of rooms stamped at the centre of
// the bottom deck — the rest is empty void to build out into.
const (
	LevelWidth  = 100
	LevelHeight = 100
	LevelDepth  = 4
	HullDeck    = 0

	// The starting hull is a roomsX × roomsY grid of rooms, each roomW × roomH
	// interior, separated by 1-tile walls. Doorways between rooms are blocked by
	// rubble, and some interior walls are damaged (rubble walls) — the player
	// digs it all out to connect the rooms (onboarding to the dig/build loop).
	roomsX = 3
	roomsY = 2
	roomW  = 6
	roomH  = 6

	// wallDamageOneIn: 1-in-N interior wall tiles start as a damaged (rubble)
	// wall the player must clear.
	wallDamageOneIn = 4
)

// gridW/gridH are the total footprint of the room grid (rooms + separating
// walls + outer hull).
func gridW() int { return roomsX*roomW + (roomsX + 1) }
func gridH() int { return roomsY*roomH + (roomsY + 1) }

// gridOrigin is the top-left corner of the room grid, centred in the level.
func gridOrigin() (int, int) {
	return (LevelWidth - gridW()) / 2, (LevelHeight - gridH()) / 2
}

// roomInterior returns the top-left interior floor tile of room (rx, ry).
func roomInterior(rx, ry int) (int, int) {
	gx, gy := gridOrigin()
	return gx + 1 + rx*(roomW+1), gy + 1 + ry*(roomH+1)
}

// startRoom is the room the crew and ship hold begin in.
func startRoom() (int, int) { return roomsX / 2, roomsY / 2 }

// HullCenter returns the centre tile of the starting room (crew spawn + camera).
func HullCenter() (x, y, z int) {
	rx, ry := startRoom()
	ix, iy := roomInterior(rx, ry)
	return ix + roomW/2, iy + roomH/2, HullDeck
}

// BuildShipLevel constructs the ship level: a large void with a grid of rooms
// (hull_floor enclosed by hull_wall) at the centre of the bottom deck. Doorways
// between adjacent rooms are blocked with rubble, some interior walls are
// damaged, and the ship hold (owned by storageOwner) sits in the start room.
// Colonists are placed by the caller.
func BuildShipLevel(storageOwner string) *world.Level {
	level := world.NewLevel(LevelWidth, LevelHeight, LevelDepth)
	gx, gy := gridOrigin()

	// Paint the grid: floor everywhere, hull_wall on the separating grid lines.
	for ly := 0; ly < gridH(); ly++ {
		for lx := 0; lx < gridW(); lx++ {
			x, y := gx+lx, gy+ly
			level.SetFloor(x, y, HullDeck, "hull_floor", world.RandomTileVariant("hull_floor"))
			if isWallCell(lx, ly) {
				level.SetMiddle(x, y, HullDeck, "hull_wall", world.RandomTileVariant("hull_wall"))
			}
		}
	}

	damageWalls(level)   // some interior walls become oriented rubble walls
	placeDoorways(level) // rubble blocks the passage between adjacent rooms

	rx, ry := startRoom()
	ix, iy := roomInterior(rx, ry)
	placeShipHold(level, ix+1, iy+1, HullDeck, storageOwner)
	return level
}

// isWallCell reports whether grid-local (lx, ly) falls on a separating wall line.
func isWallCell(lx, ly int) bool {
	return lx%(roomW+1) == 0 || ly%(roomH+1) == 0
}

// damageWalls converts a fraction of the *interior* walls (never the outer hull)
// into oriented rubble walls the player must clear.
func damageWalls(level *world.Level) {
	gx, gy := gridOrigin()
	for ly := 1; ly < gridH()-1; ly++ {
		for lx := 1; lx < gridW()-1; lx++ {
			wallCol := lx%(roomW+1) == 0
			wallRow := ly%(roomH+1) == 0
			if !wallCol && !wallRow {
				continue // room interior, not a wall
			}
			if rand.Intn(wallDamageOneIn) != 0 {
				continue
			}
			x, y := gx+lx, gy+ly
			name := "rubble_wall_h" // horizontal walls and junctions
			if wallCol && !wallRow {
				// Vertical wall: "left-facing" sits on the left side of a room
				// (open floor to its right); "right-facing" on the right side.
				if middleEmpty(level, x+1, y) {
					name = "rubble_wall_vl"
				} else {
					name = "rubble_wall_vr"
				}
			}
			level.SetMiddle(x, y, HullDeck, name, world.RandomTileVariant(name))
		}
	}
}

// placeDoorways drops a rubble pile in the wall between each pair of adjacent
// rooms — the "door" the player clears to open the passage.
func placeDoorways(level *world.Level) {
	gx, gy := gridOrigin()
	rubble := func(lx, ly int) {
		level.SetMiddle(gx+lx, gy+ly, HullDeck, "rubble_pile", world.RandomTileVariant("rubble_pile"))
	}
	// Vertical shared walls (horizontally-adjacent rooms).
	for ry := 0; ry < roomsY; ry++ {
		for rx := 0; rx < roomsX-1; rx++ {
			lx := (rx + 1) * (roomW + 1)
			ly := ry*(roomH+1) + 1 + roomH/2
			rubble(lx, ly)
		}
	}
	// Horizontal shared walls (vertically-adjacent rooms).
	for rx := 0; rx < roomsX; rx++ {
		for ry := 0; ry < roomsY-1; ry++ {
			ly := (ry + 1) * (roomH + 1)
			lx := rx*(roomW+1) + 1 + roomW/2
			rubble(lx, ly)
		}
	}
}

// middleEmpty reports whether the tile's Middle slot is empty (an open room
// interior rather than a wall).
func middleEmpty(level *world.Level, x, y int) bool {
	t := level.GetTilePtr(x, y, HullDeck)
	return t != nil && t.Middle.IsEmpty()
}

// placeShipHold spawns the ship's starting hold — a Teleporter Storage (the beam
// staging pad, high capacity) owned by owner at (x,y,z).
func placeShipHold(level *world.Level, x, y, z int, owner string) {
	e, err := factory.Create("teleporter", x, y, z)
	if err != nil {
		return
	}
	if e.HasComponent(components.Storage) {
		e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = owner
	}
	level.AddEntity(e)
}
