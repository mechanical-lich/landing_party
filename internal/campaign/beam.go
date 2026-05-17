package campaign

import (
	"fmt"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

// OpenToSky reports whether (x,y,z) has clear sky above it — no solid Middle
// tile in any higher Z column. Beaming may not pass through ground.
func OpenToSky(level *world.Level, x, y, z int) bool {
	depth := level.GetDepth()
	for zz := z; zz < depth; zz++ {
		ti := level.GetTileAt(x, y, zz)
		if ti == nil {
			continue
		}
		t := ti.(*world.Tile)
		if zz > z && !t.Middle.IsEmpty() {
			if int(t.Middle.Type) < len(world.TileDefinitions) && world.TileDefinitions[t.Middle.Type].Solid {
				return false
			}
		}
		if !t.Ceiling.IsEmpty() {
			return false
		}
	}
	return true
}

// BeamUp transfers a colonist from the live level to the ship roster. Fails if
// the roster is full or the colonist is not open to the sky.
func BeamUp(level *world.Level, ship *ShipState, e *ecs.Entity) error {
	if e == nil {
		return fmt.Errorf("beam up: nil entity")
	}
	if len(ship.Roster) >= ship.RosterCap {
		return fmt.Errorf("ship roster is full (%d/%d)", len(ship.Roster), ship.RosterCap)
	}
	if e.HasComponent(rlcomponents.Position) {
		pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if !OpenToSky(level, pc.GetX(), pc.GetY(), pc.GetZ()) {
			return fmt.Errorf("colonist is not open to the sky — no beaming through ground")
		}
	}
	// Stale task/AI pointers will not survive serialization; clear via the
	// game-side cleanup before calling BeamUp (see hud trigger).
	ship.Roster = append(ship.Roster, world.EntityToSaveEntity(e))
	level.RemoveEntity(e)
	return nil
}
