package director

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// The hive's rungs (docs/brainstorm/hive_plan.md, "The director table"). A
// typed doctrine grows by laying eggs: a drone carries an egg recipe's
// biomass to a free larva (a grey egg, the recipe's station, used up) and
// works it; the egg hatches, and its hatchling grows into the settlement's
// worker of that type. The bodies a rung counts are the settlement's typed
// workers and its young and eggs (Kin.Settlement), by Kin.Type.

// larvaStation is the station id of a hive egg recipe.
const larvaStation = "hive_larva"

// typedCount is the settlement's bodies by type (workers, hatchlings, eggs)
// plus the eggs ordered and not yet laid, and their total.
func typedCount(s *Site, snap *snapshot) (map[string]int, int) {
	count := map[string]int{}
	total := 0
	for _, e := range snap.colonists {
		if k := components.KinOf(e); k != nil && k.Type != "" {
			count[k.Type]++
			total++
		}
	}
	for _, e := range s.Level.Entities {
		if e == nil || e.HasComponent(components.Worker) || e.HasComponent(rlcomponents.Dead) || e.HasComponent(rlcomponents.Inanimate) {
			continue
		}
		if k := components.KinOf(e); k != nil && k.Settlement == s.Colony && k.Type != "" {
			count[k.Type]++
			total++
		}
	}
	for key, n := range snap.openCraft {
		if r, ok := crafting.GetRecipe(key); ok && r.KinType != "" && n > 0 {
			count[r.KinType] += n
			total += n
		}
	}
	return count, total
}

// freeLarvae is the settlement's larvae with no order on them, nearest the
// centre first.
func freeLarvae(s *Site, snap *snapshot) []*ecs.Entity {
	type cand struct {
		e    *ecs.Entity
		dist int
		x, y int
	}
	var cs []cand
	for _, list := range [][]*ecs.Entity{s.Level.Entities, s.Level.StaticEntities} {
		for _, e := range list {
			if e == nil || e.HasComponent(rlcomponents.Dead) || !e.HasComponent(components.CraftingStation) {
				continue
			}
			if e.GetComponent(components.CraftingStation).(*components.CraftingStationComponent).StationID != larvaStation {
				continue
			}
			if k := components.KinOf(e); k == nil || k.Settlement != s.Colony {
				continue
			}
			x, y, z, ok := pos(e)
			if !ok || snap.taskAt[[3]int{x, y, z}] {
				continue
			}
			cs = append(cs, cand{e, cheb(x-s.CX, y-s.CY), x, y})
		}
	}
	sort.Slice(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		if a.dist != b.dist {
			return a.dist < b.dist
		}
		return a.y < b.y || a.y == b.y && a.x < b.x
	})
	out := make([]*ecs.Entity, len(cs))
	for i, c := range cs {
		out[i] = c.e
	}
	return out
}

// eggRecipe is the doctrine's unit recipe laying type typ, "" for none or
// one whose research the settlement lacks.
func eggRecipe(s *Site, typ string) (crafting.Recipe, bool) {
	for _, key := range s.Doctrine.Units {
		r, ok := crafting.GetRecipe(key)
		if !ok || r.KinType != typ {
			continue
		}
		if r.RequiresTech != "" && !has(s.Known, r.RequiresTech) {
			return crafting.Recipe{}, false
		}
		return r, true
	}
	return crafting.Recipe{}, false
}

// growTyped is the typed grow rung: with g.Type, the unit of that type while
// its bodies are under Min + Share x bodies (the drones first; the guard's
// warriors), whatever the cap; without, the unit of the type most under the
// stage's mix among unlocked types while the bodies are under the crew cap.
// A hive lays an egg on a free larva, when the stock covers it and leaves the
// reserve. A printing settlement (the autonz) prints at the recipe's station,
// one print a station at a time, and never past the crew cap (its cap is its
// conversion: autonz_plan.md). One order a pass.
func (d *Director) growTyped(s *Site, snap *snapshot, g campaign.DirectorGrow, tick int) bool {
	count, bodies := typedCount(s, snap)
	typ := g.Type
	if typ == "" && s.CrewCap > 0 && bodies >= s.CrewCap {
		return false // the brood stops at the cap; drones and guards do not
	}
	if typ != "" {
		want := g.Min + int(math.Ceil(g.Share*float64(bodies)))
		if g.Per != "" {
			want = snap.built[g.Per] // one for each (the greys' lancer per emitter)
		}
		if count[typ] >= want {
			return false
		}
	} else {
		if s.CrewCap <= 0 {
			return false
		}
		keys := make([]string, 0, len(s.TypeMix))
		total := 0.0
		for k, v := range s.TypeMix {
			keys = append(keys, k)
			total += v
		}
		sort.Strings(keys)
		best := math.Inf(-1)
		for _, k := range keys {
			if r, ok := eggRecipe(s, k); !ok || total <= 0 || !stationStands(snap, r) {
				continue
			}
			if def := s.TypeMix[k]/total*float64(bodies+1) - float64(count[k]); def > best {
				best, typ = def, k
			}
		}
		if typ == "" {
			return false
		}
	}
	r, ok := eggRecipe(s, typ)
	if !ok {
		return false
	}
	var station *ecs.Entity
	if r.Station == larvaStation {
		larvae := freeLarvae(s, snap)
		if len(larvae) == 0 {
			return false
		}
		station = larvae[0]
	} else {
		// A post's unit (Per: the greys' lancer at an emitter) is printed
		// over the crew cap, one for each post: the crew fills its cap first.
		if s.CrewCap <= 0 || bodies >= s.CrewCap && g.Per == "" || printing(snap, r.Station) {
			return false
		}
		if station = snap.stations[r.Station]; station == nil {
			return false
		}
	}
	if g.Reserve > 0 {
		// Feed before breeding: the egg must leave Reserve a body behind.
		for k, v := range r.Cost {
			if float64(snap.free[k]-v) < g.Reserve*float64(bodies) {
				return false
			}
		}
	}
	if !snap.afford(r.Cost) {
		return false
	}
	before := len(s.Tasks.GetTasks())
	if !s.Orders.Craft(r.Key, station) {
		return false
	}
	x, y, z, _ := pos(station)
	snap.taskAt[[3]int{x, y, z}] = true
	snap.openCraft[r.Key]++
	snap.consuming++
	d.watchNew(s, before, tick)
	verb := "lay %s egg (%d bodies, cap %d)"
	if r.Station != larvaStation {
		verb = "print %s (%d bodies, cap %d)"
	}
	d.note("grow:"+typ, fmt.Sprintf(verb, strings.ToLower(typ), bodies, s.CrewCap), tick)
	return true
}

// stationStands reports whether a unit recipe has where to be made: a hive's
// larvae are checked when it is laid; a print needs its station standing.
func stationStands(snap *snapshot, r crafting.Recipe) bool {
	return r.Station == larvaStation || snap.stations[r.Station] != nil
}

// printing reports an open unit print at station (one at a time).
func printing(snap *snapshot, station string) bool {
	for key, n := range snap.openCraft {
		if r, ok := crafting.GetRecipe(key); ok && n > 0 && r.KinType != "" && r.Station == station {
			return true
		}
	}
	return false
}

// roleOf is a worker's role: its own (a typed body's, set by its type),
// else its crew entry's.
func roleOf(s *Site, e *ecs.Entity) string {
	if e.HasComponent(components.Worker) {
		if r := e.GetComponent(components.Worker).(*components.WorkerComponent).Role; r != "" {
			return r
		}
	}
	return s.Doctrine.RoleOf(e.Blueprint)
}

// withRoles is the settlement's workers of roles, in a stable order.
func withRoles(s *Site, snap *snapshot, roles []string) []*ecs.Entity {
	var out []*ecs.Entity
	for _, e := range snap.colonists {
		if has(roles, roleOf(s, e)) {
			out = append(out, e)
		}
	}
	return out
}

// busy reports whether a worker has a task in hand.
func busy(e *ecs.Entity) *task.Task {
	wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		return wc.CurrentTask
	}
	return nil
}

// faction is an entity's faction string.
func faction(e *ecs.Entity) string {
	if e.HasComponent(rlcomponents.Description) {
		return e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction
	}
	return ""
}

// hostileNear is the hostile nearest the centre within r: the player's
// colonist (when the settlement is hostile), else a fighting creature of
// another faction (a scripted or faction mob with stats).
func hostileNear(s *Site, r int) *ecs.Entity {
	own := ""
	if i := strings.IndexByte(s.Colony, ':'); i > 0 {
		own = s.Colony[:i]
	}
	var best *ecs.Entity
	bestD := 1 << 30
	for _, e := range s.Level.Entities {
		if e == nil || e.HasComponent(rlcomponents.Dead) || !e.HasComponent(rlcomponents.Health) {
			continue
		}
		x, y, z, ok := pos(e)
		if !ok || z != s.CZ {
			continue
		}
		dist := cheb(x-s.CX, y-s.CY)
		if dist > r {
			continue
		}
		hostile := false
		switch {
		case s.Hostile && s.Enemy != "" && components.IsColonyWorker(e, s.Enemy):
			hostile = true
		case !e.HasComponent(components.Worker) && (e.HasComponent(components.ScriptedAI) || e.HasComponent(components.FactionAI)) &&
			e.HasComponent(rlcomponents.Stats) && faction(e) != own && faction(e) != "" &&
			(own == "" || components.HostileFactions(own, faction(e))):
			hostile = true
		}
		if hostile && (dist < bestD || dist == bestD && less(e, best)) {
			best, bestD = e, dist
		}
	}
	return best
}

// guard is the guard rung's guarding: a hostile within Radius of the heart
// is set upon by every guard at once; with none, a guard sent earlier is
// called back from a target gone past the radius, and idle guards walk to
// their posts, Post tiles out round the heart.
func (d *Director) guard(s *Site, snap *snapshot, g *campaign.DirectorGuard, tick int) {
	guards := withRoles(s, snap, g.Roles)
	for t := range d.guardTasks {
		if t.Completed {
			delete(d.guardTasks, t)
		}
	}
	radius := g.Radius
	if radius <= 0 {
		radius = 12
	}
	if target := hostileNear(s, radius); target != nil {
		sent := 0
		for _, e := range guards {
			if t := busy(e); t != nil && t.Action == task_requests.AttackAction && t.Data == target {
				continue
			}
			if t := s.Orders.Send(e, target); t != nil {
				d.guardTasks[t] = true
				sent++
			}
		}
		if sent > 0 {
			d.note("guard:"+target.Blueprint, fmt.Sprintf("%d guards on %s", sent, target.Blueprint), tick)
		}
		return
	}
	for t := range d.guardTasks {
		if target, ok := t.Data.(*ecs.Entity); ok {
			if x, y, _, ok := pos(target); !ok || cheb(x-s.CX, y-s.CY) > radius+4 || target.HasComponent(rlcomponents.Dead) {
				t.Complete() // gone: back to the post
				delete(d.guardTasks, t)
			}
		}
	}
	if g.Posts != "" {
		d.holdPosts(s, guards, g.Posts, tick)
		return
	}
	if g.Post <= 0 {
		return
	}
	for i, e := range guards {
		if busy(e) != nil {
			continue
		}
		a := 2 * math.Pi * float64(i) / float64(max(len(guards), 1))
		px := s.CX + int(math.Round(float64(g.Post)*math.Cos(a)))
		py := s.CY + int(math.Round(float64(g.Post)*math.Sin(a)))
		x, y, _, _ := pos(e)
		if cheb(x-px, y-py) <= 2 || !open(s.Level, px, py, s.CZ) {
			continue
		}
		if s.Orders.Move(e, px, py, s.CZ) != nil {
			d.note("post", fmt.Sprintf("guard to post %d,%d", px, py), tick)
		}
	}
}

// launch is the launch rung: on a day the launcher would fire (the Tier 1
// rule, Site.LaunchReady), ShotPop bodies walk to stand within Walk tiles of
// the spire, the economy types first (drones, then scouts), then the rest;
// the day's live rule takes them once they are there.
func (d *Director) launch(s *Site, snap *snapshot, l *campaign.DirectorLaunch, tick int) {
	live := d.holds[:0]
	for _, t := range d.holds {
		if !t.Completed {
			live = append(live, t)
		}
	}
	d.holds = live
	launcher := s.Launcher
	if l.At != "" {
		launcher = l.At // the autonz' Signal Core
	}
	if !s.LaunchReady || s.ShotPop <= 0 || launcher == "" {
		for _, t := range d.holds {
			t.Complete() // not today: back to work
		}
		d.holds = nil
		return
	}
	var spire *ecs.Entity
	for _, list := range [][]*ecs.Entity{s.Level.Entities, s.Level.StaticEntities} {
		for _, e := range list {
			if e != nil && e.Blueprint == launcher && !e.HasComponent(rlcomponents.Dead) {
				if x, y, z, ok := pos(e); ok && z == s.CZ && cheb(x-s.CX, y-s.CY) <= d.cfg(s).Radius+2 {
					spire = e
				}
			}
		}
	}
	if spire == nil {
		return
	}
	sx, sy, sz, _ := pos(spire)
	walk := max(l.Walk, 1)
	there, going := 0, 0
	rank := map[string]int{}
	for i, t := range s.Economy {
		rank[t] = i
	}
	var cands []*ecs.Entity
	for _, e := range snap.colonists {
		x, y, _, _ := pos(e)
		if t := busy(e); cheb(x-sx, y-sy) <= walk && t != nil && t.Data == task_requests.HoldHere {
			there++ // waiting by the spire
			continue
		}
		if t := busy(e); t != nil && t.Action == task_requests.MoveAction && t.Data == task_requests.HoldHere && cheb(t.X-sx, t.Y-sy) <= walk {
			going++
			continue
		}
		if len(l.Roles) > 0 && !has(l.Roles, roleOf(s, e)) {
			continue // never a warden into the core
		}
		cands = append(cands, e)
	}
	need := s.ShotPop - there - going
	if need <= 0 {
		return
	}
	order := func(e *ecs.Entity) int {
		for i, r := range l.Roles {
			if roleOf(s, e) == r {
				return i
			}
		}
		if k := components.KinOf(e); k != nil {
			if r, ok := rank[k.Type]; ok {
				return r
			}
		}
		return len(rank)
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if a, b := order(cands[i]), order(cands[j]); a != b {
			return a < b
		}
		ax, ay, _, _ := pos(cands[i])
		bx, by, _, _ := pos(cands[j])
		return cheb(ax-sx, ay-sy) < cheb(bx-sx, by-sy)
	})
	k := 0
	for _, e := range cands {
		if need <= 0 {
			break
		}
		// A tile round the spire, walking the ring out to Walk.
		for ; k < 8*walk*walk; k++ {
			r := 1 + k/8
			if r > walk {
				break
			}
			dx := []int{-1, 0, 1, 1, 1, 0, -1, -1}[k%8] * r
			dy := []int{-1, -1, -1, 0, 1, 1, 1, 0}[k%8] * r
			if open(s.Level, sx+dx, sy+dy, sz) && s.Level.GetSolidEntityAt(sx+dx, sy+dy, sz) == nil {
				if t := s.Orders.Hold(e, sx+dx, sy+dy, sz); t != nil {
					d.holds = append(d.holds, t)
					need--
				}
				k++
				break
			}
		}
	}
	d.note("launch", fmt.Sprintf("bodies walk to the spire (%d there)", there), tick)
}

// contactFlag is the level flag a settlement's scouts set on seeing the
// player's colony.
func contactFlag(colony string) string { return "contact:" + colony }

// Contact reports whether settlement colony's scouts have seen the
// player's colony on lvl.
func Contact(lvl *world.Level, colony string) bool {
	return lvl != nil && lvl.Flags != nil && lvl.Flags[contactFlag(colony)] != nil
}

// scoutSight is how far a scout sees the player's colony.
const scoutSight = 8

// patrol is the patrol rung: a scout within sight of the player's colonist
// or storage records contact on the level; every EveryTicks each idle scout
// is sent to a waypoint on a ring Radius out from the centre.
func (d *Director) patrol(s *Site, snap *snapshot, pt *campaign.DirectorPatrol, tick int) {
	scouts := withRoles(s, snap, pt.Roles)
	if s.Enemy != "" && !Contact(s.Level, s.Colony) {
		for _, sc := range scouts {
			x, y, z, _ := pos(sc)
			for _, list := range [][]*ecs.Entity{s.Level.Entities, s.Level.StaticEntities} {
				for _, e := range list {
					if e == nil || e.HasComponent(rlcomponents.Dead) {
						continue
					}
					seen := components.IsColonyWorker(e, s.Enemy) ||
						e.HasComponent(components.Storage) && e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy == s.Enemy
					if !seen {
						continue
					}
					if ex, ey, ez, ok := pos(e); ok && ez == z && cheb(ex-x, ey-y) <= scoutSight {
						if s.Level.Flags == nil {
							s.Level.Flags = map[string]any{}
						}
						s.Level.Flags[contactFlag(s.Colony)] = float64(tick)
						d.note("contact", fmt.Sprintf("a scout saw %s at %d,%d", e.Blueprint, ex, ey), tick)
						goto seen
					}
				}
			}
		}
	}
seen:
	every := max(pt.EveryTicks, 30)
	if d.lastPatrol >= 0 && tick-d.lastPatrol < every {
		return
	}
	d.lastPatrol = tick
	for i, sc := range scouts {
		if busy(sc) != nil {
			continue
		}
		a := float64(int64(i)*7919+d.Seed%1000+int64(tick/every)) * 2.399963 // the golden angle
		x := s.CX + int(math.Round(float64(pt.Radius)*math.Cos(a)))
		y := s.CY + int(math.Round(float64(pt.Radius)*math.Sin(a)))
		x = min(max(x, 1), s.Level.GetWidth()-2)
		y = min(max(y, 1), s.Level.GetHeight()-2)
		if s.Orders.Move(sc, x, y, s.CZ) != nil {
			d.note("patrol", fmt.Sprintf("scout to %d,%d", x, y), tick)
		}
	}
}

// excavate is the excavate rung: when the convert rung found nothing to lay
// its floor on and the settlement has diggers, dig orders on the rock
// nearest the centre within Radius that stands beside reachable ground, at
// most MaxMines open.
func (d *Director) excavate(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, ex *campaign.DirectorExcavate, tick int) {
	if !snap.noConvert || len(withRoles(s, snap, ex.Roles)) == 0 {
		return
	}
	open := 0
	for _, t := range s.Tasks.GetTasks() {
		if !t.Completed && t.Action == task_requests.DigAction {
			open++
		}
	}
	limit := max(cfg.MaxMines, 1)
	type cand struct{ x, y, dist int }
	var cands []cand
	for dy := -ex.Radius; dy <= ex.Radius; dy++ {
		for dx := -ex.Radius; dx <= ex.Radius; dx++ {
			x, y := s.CX+dx, s.CY+dy
			ti := s.Level.GetTileAt(x, y, s.CZ)
			if ti == nil || !ti.IsSolid() || snap.taskAt[[3]int{x, y, s.CZ}] || d.avoid[[3]int{x, y, s.CZ}] {
				continue
			}
			if !nearReach(snap, x, y) {
				continue
			}
			cands = append(cands, cand{x, y, cheb(dx, dy)})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.dist != b.dist {
			return a.dist < b.dist
		}
		return a.y < b.y || a.y == b.y && a.x < b.x
	})
	for _, c := range cands {
		if open >= limit {
			return
		}
		before := len(s.Tasks.GetTasks())
		if !s.Orders.Dig(c.x, c.y, s.CZ) {
			d.avoid[[3]int{c.x, c.y, s.CZ}] = true
			continue
		}
		open++
		snap.taskAt[[3]int{c.x, c.y, s.CZ}] = true
		d.watchNew(s, before, tick)
		d.note("excavate", fmt.Sprintf("dig %d,%d for resin", c.x, c.y), tick)
	}
}

// holdPosts sends each idle guard to a tile beside its own standing
// structure of blueprint bp within the radius, one guard a structure (the
// autonz' sentinels at their defence pylons).
func (d *Director) holdPosts(s *Site, guards []*ecs.Entity, bp string, tick int) {
	var posts []*ecs.Entity
	for _, list := range [][]*ecs.Entity{s.Level.Entities, s.Level.StaticEntities} {
		for _, e := range list {
			if e == nil || e.Blueprint != bp || e.HasComponent(rlcomponents.Dead) {
				continue
			}
			if x, y, z, ok := pos(e); ok && z == s.CZ && cheb(x-s.CX, y-s.CY) <= d.cfg(s).Radius+2 {
				posts = append(posts, e)
			}
		}
	}
	sort.Slice(posts, func(i, j int) bool { return less(posts[i], posts[j]) })
	for i, e := range guards {
		if i >= len(posts) {
			return
		}
		if busy(e) != nil {
			continue
		}
		px, py, _, _ := pos(posts[i])
		x, y, _, _ := pos(e)
		if cheb(x-px, y-py) <= 1 {
			continue
		}
		for _, n := range [][2]int{{px, py + 1}, {px - 1, py}, {px + 1, py}, {px, py - 1}} {
			if open(s.Level, n[0], n[1], s.CZ) && s.Level.GetSolidEntityAt(n[0], n[1], s.CZ) == nil {
				if s.Orders.Move(e, n[0], n[1], s.CZ) != nil {
					d.note("post", fmt.Sprintf("%s to its %s at %d,%d", roleOf(s, e), bp, px, py), tick)
				}
				break
			}
		}
	}
}
