package game

import (
	"fmt"
	"math"

	"github.com/mechanical-lich/landing_party/internal/audio"
	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/task"
)

// The gate on a level (pregenerated_galaxy_plan.md step 4; campaign/gate.go):
// the controls world's vault holds the dormant gate controls, repaired by an
// ordinary repair order (click it with orders on). The repair sets the level
// flag campaign.GateFlag; checkGate turns that into the campaign's gate
// switched on, by whoever's settlement repaired it.

// checkGate switches the gate on once the loaded controls world's level
// carries the flag (every frame: one map lookup while it is off).
func (wm *WorldManager) checkGate() {
	c := wm.Campaign
	if c == nil || c.GateIsOn() {
		return
	}
	ctl := c.ControlsLocation()
	if ctl == nil {
		return
	}
	ms := wm.loaded[ctl.ID]
	if ms == nil || ms.level == nil || ms.level.Flags[campaign.GateFlag] == nil {
		return
	}
	by := gateRepairer(c, ctl, ms.level.Flags[campaign.GateFlagBy])
	if !c.SwitchGateOn(by) {
		return
	}
	if by == campaign.GateByPlayer {
		audio.PlayGlobal("notify")
		message.PostMessage("Mission", fmt.Sprintf("The gate's controls are on. The gate beside %s is lit: from beside it, the ship can leave the galaxy.", ctl.Name)) // DRAFT
	}
	wm.postNews()
}

// gateRepairer is the faction whose settlement named by repaired the
// controls on loc: the player's settlement, else a faction settlement there.
func gateRepairer(c *campaign.Campaign, loc *campaign.Location, by any) string {
	name, _ := by.(string)
	ps, _ := settlementPresences(c, loc)
	for _, p := range ps {
		if factionSettlementName(p, loc.ID) == name {
			return p.Faction
		}
	}
	return campaign.GateByPlayer
}

// postNews posts the campaign's waiting alerts (the gate switched on by a
// rival, a rival leaving through it).
func (wm *WorldManager) postNews() {
	c := wm.Campaign
	if c == nil || len(c.News) == 0 {
		return
	}
	for _, n := range c.News {
		message.PostMessage("Alert", n)
	}
	audio.PlayGlobal("notify")
	c.News = nil
}

// orderGateRepairs has a rival colony landed on the loaded controls world,
// gate off, order the controls repaired by its own settlement (once; its
// workers take it like any build), and so switch the gate on live.
func (wm *WorldManager) orderGateRepairs() {
	c := wm.Campaign
	if c == nil || c.GateIsOn() {
		return
	}
	ctl := c.ControlsLocation()
	if ctl == nil {
		return
	}
	ms := wm.loaded[ctl.ID]
	if ms == nil || ms.level == nil {
		return
	}
	for i := range c.Fleets {
		f := &c.Fleets[i]
		if !f.Landed() || f.LandedID != ctl.ID || !f.Presence.Materialized || !c.ShipSeeksGate(f) {
			continue
		}
		st := settlement.Settlements[factionSettlementName(&f.Presence, ctl.ID)]
		if st == nil {
			continue
		}
		if e := gateControls(ms.level); e != nil {
			orderRepair(st, e)
		}
	}
}

// gateControls is the dormant gate controls on lvl, or nil.
func gateControls(lvl *world.Level) *ecs.Entity {
	for _, list := range [][]*ecs.Entity{lvl.Entities, lvl.StaticEntities} {
		for _, e := range list {
			if e != nil && e.Blueprint == campaign.GateControlsDormant && e.HasComponent(components.Repairable) {
				return e
			}
		}
	}
	return nil
}

// orderRepair queues the repair of a broken machine for st (as the
// player's repair order does), unless one is queued; it reports whether it
// queued one.
func orderRepair(st *settlement.Settlement, ent *ecs.Entity) bool {
	rc := ent.GetComponent(components.Repairable).(*components.RepairableComponent)
	if !ent.HasComponent(rlcomponents.Position) || rc.Into == "" {
		return false
	}
	pc := ent.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	x, y, z := pc.GetX(), pc.GetY(), pc.GetZ()
	for _, t := range st.Tasks.GetTasks() {
		if t.X == x && t.Y == y && t.Z == z && t.Action == task_requests.BuildAction {
			return false
		}
	}
	b := construction.GetBuildable(rc.Into)
	st.Tasks.AddTask(&task.Task{
		Action: task_requests.BuildAction,
		Data:   task_requests.BuildRequest{X: x, Y: y, Z: z, Type: rc.Into, Required: b.BuildTime, Replace: true},
		X:      x, Y: y, Z: z,
	})
	return true
}

// vaultSite is where the gate's vault goes on lvl: a footprint of w x h,
// inside the map, centred some 16 to 62 tiles from the landing (ax, ay, az), on ground the
// landing can walk to on its own level (trees aside; 80% of the footprint
// and its centre), so the vault's door is
// always a walk away. h picks the order candidates are tried in. ok is false
// when no candidate qualifies (the caller falls back to a fixed offset).
func vaultSite(lvl *world.Level, ax, ay, az, w, hgt, h int) (x, y int, ok bool) {
	const span = 80
	walk := func(x, y int) bool {
		t := lvl.GetTilePtr(x, y, az)
		return t != nil && !t.IsSolid() && !t.IsWater() && !world.IsSpaceTile(t) && !t.Floor.IsEmpty()
	}
	reach := map[[2]int]bool{}
	if walk(ax, ay) {
		reach[[2]int{ax, ay}] = true
		queue := [][2]int{{ax, ay}}
		for len(queue) > 0 {
			p := queue[0]
			queue = queue[1:]
			for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				n := [2]int{p[0] + d[0], p[1] + d[1]}
				if reach[n] || abs(n[0]-ax) > span || abs(n[1]-ay) > span || !lvl.InBounds(n[0], n[1], az) || !walk(n[0], n[1]) {
					continue
				}
				reach[n] = true
				queue = append(queue, n)
			}
		}
	}
	fits := func(cx, cy int) bool {
		in, all := 0, 0
		for yy := cy - hgt/2; yy < cy-hgt/2+hgt; yy++ {
			for xx := cx - w/2; xx < cx-w/2+w; xx++ {
				all++
				if reach[[2]int{xx, yy}] {
					in++
				}
			}
		}
		return in*100 >= all*80 && reach[[2]int{cx, cy}] &&
			lvl.InBounds(cx-w/2, cy-hgt/2, az) && lvl.InBounds(cx-w/2+w-1, cy-hgt/2+hgt-1, az)
	}
	radii := []int{38, 30, 46, 22, 54, 16, 62}
	for _, r := range radii {
		for i := 0; i < 16; i++ {
			a := float64((i+h)%16) * math.Pi / 8
			cx, cy := ax+int(math.Round(math.Cos(a)*float64(r))), ay+int(math.Round(math.Sin(a)*float64(r)))
			if fits(cx, cy) {
				return cx, cy, true
			}
		}
	}
	return 0, 0, false
}
