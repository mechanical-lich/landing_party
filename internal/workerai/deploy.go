package workerai

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// handleDeployTask walks the worker onto the request's tile and works there;
// when the work is done the worker is replaced by the structure on that
// tile, its Kin presence and settlement carried over (the relay drone's
// matter is the pylon's: nothing is made from nothing). A tile off the
// required floor refuses the deploy.
func handleDeployTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	req, ok := wc.CurrentTask.Data.(task_requests.DeployRequest)
	if !ok {
		aiMemory.State = "idle"
		return
	}
	t := wc.CurrentTask
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if pc.GetX() != req.X || pc.GetY() != req.Y || pc.GetZ() != req.Z {
		if moved, pathFound := MoveTowardsTarget(level, entity, req.X, req.Y, req.Z); !moved {
			if !pathFound {
				CompleteTaskWithMessage(entity, t, "Can't reach the deploy site")
				aiMemory.State = "idle"
			}
			// Blocked for now (a body in the way): the deploy is the
			// worker's own, never queued, so it keeps it and tries again
			// next turn (re-queuing it would put it nowhere).
		}
		return
	}
	if !DeployFloorOK(level, req.X, req.Y, req.Z, req.Floor) {
		CompleteTaskWithMessage(entity, t, "Can't deploy here")
		aiMemory.State = "idle"
		return
	}
	req.Progress += workStep(wc, entity, "Str")
	t.Data = req
	if req.Progress < req.Required {
		return
	}
	out, err := factory.Create(req.As, req.X, req.Y, req.Z)
	if err != nil {
		CompleteTaskWithMessage(entity, t, "Can't deploy")
		aiMemory.State = "idle"
		return
	}
	if k := components.KinOf(entity); k != nil {
		ok := components.KinOf(out)
		if ok == nil {
			ok = &components.KinComponent{Morph: "structure"}
			out.AddComponent(ok)
		}
		ok.Presence, ok.Settlement = k.Presence, k.Settlement
		if ok.Settlement == "" && entity.HasComponent(components.Settlement) {
			ok.Settlement = entity.GetComponent(components.Settlement).(*components.SettlementComponent).Name
		}
	}
	t.Complete()
	wc.CurrentTask = nil
	aiMemory.State = "idle"
	level.RemoveEntity(entity)
	level.AddEntity(out)
	if OnDeployed != nil {
		OnDeployed(level, entity, out)
	}
}

// OnDeployed, when set, is told of every unit that became a structure (the
// game's alert).
var OnDeployed func(level *world.Level, unit, structure *ecs.Entity)

// DeployFloorOK reports whether (x, y, z)'s floor is one of floors (none:
// any floor).
func DeployFloorOK(level *world.Level, x, y, z int, floors []string) bool {
	if len(floors) == 0 {
		return true
	}
	ti := level.GetTilePtr(x, y, z)
	if ti == nil || ti.Floor.IsEmpty() {
		return false
	}
	name := world.TileDefinitions[ti.Floor.Type].Name
	for _, f := range floors {
		if f == name {
			return true
		}
	}
	return false
}
