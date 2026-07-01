// Package ship builds and manages The Ship — the campaign's always-loaded home
// base level where colonists not beamed down live. See
// docs/developer/the_ship.md.
package ship

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/world"
)

// Ship level dimensions. The level is large (100x100x4) so the ship can grow,
// but the v1 starting structure is only a small hull room stamped at the centre
// of the bottom deck — the rest is empty void to build out into.
const (
	LevelWidth  = 100
	LevelHeight = 100
	LevelDepth  = 4

	// HullSize is the side length of the starting hull room; HullDeck is the
	// z-level it sits on.
	HullSize = 10
	HullDeck = 0
)

// hullOrigin returns the bottom-left corner of the starting hull, centred in
// the level so there's room to expand in every direction.
func hullOrigin() (int, int) {
	return (LevelWidth - HullSize) / 2, (LevelHeight - HullSize) / 2
}

// HullCenter returns the centre tile of the starting hull (for camera framing).
func HullCenter() (x, y, z int) {
	ox, oy := hullOrigin()
	return ox + HullSize/2, oy + HullSize/2, HullDeck
}

// BuildShipLevel constructs the ship level: a 100x100x4 void with a fixed
// HullSize x HullSize hull room stamped at the centre of the bottom deck —
// hull_floor enclosed by hull_wall, with a single storage_locker owned by
// storageOwner. Colonists and starting cargo are placed by the caller.
func BuildShipLevel(storageOwner string) *world.Level {
	level := world.NewLevel(LevelWidth, LevelHeight, LevelDepth)
	ox, oy := hullOrigin()
	for dy := 0; dy < HullSize; dy++ {
		for dx := 0; dx < HullSize; dx++ {
			x, y := ox+dx, oy+dy
			level.SetFloor(x, y, HullDeck, "hull_floor", world.RandomTileVariant("hull_floor"))
			if dx == 0 || dy == 0 || dx == HullSize-1 || dy == HullSize-1 {
				level.SetMiddle(x, y, HullDeck, "hull_wall", world.RandomTileVariant("hull_wall"))
			}
		}
	}
	placeShipHold(level, ox+2, oy+2, HullDeck, storageOwner)
	return level
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
