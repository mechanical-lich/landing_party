package director

import (
	"fmt"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/mlge/ecs"
)

// The autonz' deploy rung (docs/brainstorm/autonz_plan.md, "The director
// table"): a relay pylon is never built; a relay drone walks to a wired tile
// at the edge of the hub's ground and becomes one.

// forgetDeploys drops deploy orders that are done or dead: completed, or no
// longer in progress (a deploy is never queued, so a stopped or re-queued
// one, its drone dead or taken off it, will never complete and must not
// count against the rung forever).
func (d *Director) forgetDeploys() {
	out := d.deploys[:0]
	for _, t := range d.deploys {
		if !t.Completed && t.InProgress {
			out = append(out, t)
		}
	}
	d.deploys = out
}

// deploy is the deploy rung: while the As structures standing (and on their
// way) are under the stage's count and the settlement knows the structure,
// an idle unit of Type (or one on a patrol walk) is sent to deploy on the
// best wired tile at least Edge from the centre; with none, one is printed
// when canPrint. It reports whether it spent the budget (a print).
func (d *Director) deploy(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, dp *campaign.DirectorDeploy, canPrint bool, tick int) bool {
	dt := s.Types[dp.Type]
	if dt == nil || dt.Deploy == nil || dt.Deploy.As != dp.As {
		return false
	}
	b := construction.GetBuildable(dp.As)
	if b.RequiredTech != "" && !has(s.Known, b.RequiredTech) {
		return false
	}
	want := s.Structures[dp.As]
	if want <= 0 || snap.built[dp.As]+len(d.deploys) >= want {
		return false
	}
	var unit *ecs.Entity
	for _, e := range snap.colonists {
		k := components.KinOf(e)
		if k == nil || k.Type != dp.Type {
			continue
		}
		if t := busy(e); t == nil || t.Action == task_requests.MoveAction && t.Data != task_requests.HoldHere {
			unit = e
			break
		}
	}
	if unit == nil {
		if !canPrint {
			return false
		}
		return d.growTyped(s, snap, campaign.DirectorGrow{Type: dp.Type, Min: 1}, tick)
	}
	x, y, ok := d.deploySite(s, cfg, snap, dt.Deploy.Floor, dp.Edge)
	if !ok {
		return false
	}
	if t := s.Orders.Deploy(unit, x, y, s.CZ, dt.Deploy.As, dt.Deploy.Floor, dt.Deploy.Ticks); t != nil {
		d.deploys = append(d.deploys, t)
		snap.taskAt[[3]int{x, y, s.CZ}] = true
		d.note("deploy:"+dp.As, fmt.Sprintf("deploy %s as %s at %d,%d", dp.Type, dp.As, x, y), tick)
	}
	return false
}

// deploySite is the reachable, open, empty tile of one of floors within the
// director's radius and at least edge from the centre that lies farthest
// toward Site.SignalDir (with no direction, farthest from the centre); ties
// by distance, then position.
func (d *Director) deploySite(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, floors []string, edge int) (int, int, bool) {
	bx, by, found := 0, 0, false
	bestA, bestB := -1e18, -1
	dir := s.SignalDir
	for k := range snap.reach {
		x, y := k[0], k[1]
		dist := cheb(x-s.CX, y-s.CY)
		if dist < edge || dist > cfg.Radius || snap.taskAt[[3]int{x, y, s.CZ}] || d.avoid[[3]int{x, y, s.CZ}] {
			continue
		}
		f := floorName(s.Level, x, y, s.CZ)
		okFloor := len(floors) == 0
		for _, fl := range floors {
			okFloor = okFloor || fl == f
		}
		if !okFloor || !open(s.Level, x, y, s.CZ) {
			continue
		}
		var buf []*ecs.Entity
		s.Level.GetEntitiesAt(x, y, s.CZ, &buf)
		if len(buf) > 0 {
			continue
		}
		a := float64(dist)
		if dir != [2]float64{} {
			a = float64(x-s.CX)*dir[0] + float64(y-s.CY)*dir[1]
		}
		better := a > bestA || a == bestA && (dist > bestB || dist == bestB && (y < by || y == by && x < bx))
		if !found || better {
			bx, by, bestA, bestB, found = x, y, a, dist, true
		}
	}
	return bx, by, found
}
