package workerai

import (
	"math"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/progression"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlai"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// handleConvertTask works a ConvertAction (phase C2; docs/developer/
// factions.md, "ConvertAction"): the conversion economy's primitive, what the
// autonz wire_placer does by script as a task any settlement can post. The
// worker goes to the tile (or beside it), works Required ticks on the shared
// progress, and lays the conversion floor there. A tile already converted, or
// with no floor to convert (solid rock, a hole), finishes at once.
func handleConvertTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	req, ok := wc.CurrentTask.Data.(task_requests.ConvertRequest)
	if !ok {
		aiMemory.State = "idle"
		return
	}
	t := wc.CurrentTask
	if !rlai.WithinRange(pc.GetX(), pc.GetY(), pc.GetZ(), t.X, t.Y, t.Z, 1, 1, 0) {
		if moved, pathFound := MoveTowardsTarget(level, entity, t.X, t.Y, t.Z); !moved {
			releaseTaskAfterFailedMove(wc, pathFound)
		}
		return
	}
	if !Convertible(level, req.X, req.Y, req.Z, req.Tile) {
		CompleteTaskWithMessage(entity, t, "Nothing to convert")
		aiMemory.State = "idle"
		return
	}
	req.Progress += multStep(wc, entity, req.Rate)
	t.Data = req
	if req.Progress < ConvertWork(level, req.X, req.Y, req.Z, req.Required) {
		return
	}
	ConvertTile(level, req.X, req.Y, req.Z, req.Tile)
	progression.AwardXP(entity, "Str", progression.XPPerTask)
	CompleteTaskWithMessage(entity, t, "Converted ground")
	aiMemory.State = "idle"
}

// Convertible reports a tile ConvertAction can turn into tile: a floor not
// already tile, with no solid middle.
func Convertible(level *world.Level, x, y, z int, tile string) bool {
	ti, ok := level.GetTileAt(x, y, z).(*world.Tile)
	if !ok || ti == nil || ti.Floor.IsEmpty() {
		return false
	}
	if !ti.Middle.IsEmpty() && world.TileDefinitions[ti.Middle.Type].Solid {
		return false
	}
	return world.TileDefinitions[ti.Floor.Type].Name != tile
}

// ConvertTile lays tile as the floor at (x, y, z).
func ConvertTile(level *world.Level, x, y, z int, tile string) {
	level.SetFloor(x, y, z, tile, world.TileVariantAt(tile, x, y, z))
	level.InvalidateSunColumn(x, y)
}

// convertBoostReach is the farthest a ConvertBoost reaches (the converter
// node's 5): ConvertWork looks no further.
const convertBoostReach = 5

// ConvertWork is the work a ConvertAction at (x, y, z) needs: required,
// times the strongest ConvertBoost factor of a structure within its radius
// (the autonz' converter node halves it), at least 1.
func ConvertWork(level *world.Level, x, y, z, required int) int {
	f := 1.0
	var buf []*ecs.Entity
	for dy := -convertBoostReach; dy <= convertBoostReach; dy++ {
		for dx := -convertBoostReach; dx <= convertBoostReach; dx++ {
			buf = buf[:0]
			level.GetEntitiesAt(x+dx, y+dy, z, &buf)
			for _, e := range buf {
				if e == nil || !e.HasComponent(components.ConvertBoost) {
					continue
				}
				b := e.GetComponent(components.ConvertBoost).(*components.ConvertBoostComponent)
				if b.Factor > 0 && b.Factor < f && max(dx, -dx, dy, -dy) <= b.Radius {
					f = b.Factor
				}
			}
		}
	}
	return max(1, int(math.Ceil(float64(required)*f)))
}
