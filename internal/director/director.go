// Package director is the faction director (docs/brainstorm/enemy_factions_plan.md,
// "Simulate: the faction director"; docs/developer/factions.md, "The
// director"): what the player's cursor is to colonists, a director is to a
// settlement. Every few ticks it reads the settlement's level and its
// doctrine's director table, a needs ladder, and gives the orders a player
// would, through the same order paths the cursor uses (Orders). It never
// moves materials or makes things itself: workers fetch, haul and build as
// they do for the player, so physical storage holds.
//
// Phase C1 runs it on the player's own settlement (observer mode); a faction
// settlement is a doctrine with a different table.
package director

import (
	"fmt"
	"math/rand"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// Orders are the player's order paths, which the game implements with the
// cursor's own functions (internal/game/director_orders.go). Each reports
// whether it queued a task.
type Orders interface {
	Build(kind string, x, y, z int) bool
	Mine(x, y, z int) bool
	// Dig is the Dig order on a solid tile.
	Dig(x, y, z int) bool
	// Convert is a ConvertAction laying tile at (x, y, z).
	Convert(x, y, z int, tile string) bool
	Retrieve(item *ecs.Entity) bool
	Research(tech string, station *ecs.Entity) bool
	Craft(recipe string, station *ecs.Entity) bool
	// Cancel is the Cancel Task order on one task.
	Cancel(t *task.Task)
	// Attack is the attack order on target (a hunt), queued for whoever
	// takes it.
	Attack(target *ecs.Entity) bool
	// Send sets worker on target now (a raid party member), dropping what
	// it was doing; the task it gets, or nil.
	Send(worker, target *ecs.Entity) *task.Task
	// Equip has a worker take blueprint out of storage and hold it.
	Equip(blueprint string, storage *ecs.Entity) bool
	// Move walks worker to (x, y, z) now, dropping what it was doing (a
	// guard's post, a scout's waypoint, the walk to the spire); the task it
	// gets, or nil.
	Move(worker *ecs.Entity, x, y, z int) *task.Task
	// Hold is Move that stays there on arrival until the task is completed
	// (the hive's shot waiting by its spire).
	Hold(worker *ecs.Entity, x, y, z int) *task.Task
	// Deploy walks worker onto (x, y, z) and replaces it with the as
	// structure after ticks there, when the floor is one of floor (the
	// autonz' relay drone into a relay pylon); the task, or nil.
	Deploy(worker *ecs.Entity, x, y, z int, as string, floor []string, ticks int) *task.Task
}

// Site is what the director plans for: one settlement on one level.
type Site struct {
	Level  *world.Level
	Colony string
	// CX, CY, CZ is the settlement centre (the landing).
	CX, CY, CZ int
	Tasks      *task.TaskScheduler
	Known      []string
	Doctrine   *campaign.DoctrineSettlement
	Orders     Orders
	// Hostile: the settlement's faction is hostile to the player, whose
	// colony (on this level) is Enemy; only then does a raid rung raid.
	// Aggression is the doctrine's (scales the raid threshold and cooldown).
	Hostile    bool
	Enemy      string
	Aggression int
	// CrewCap is how many crew the settlement may grow to (a ship's tier;
	// 0: the grow rung does nothing).
	CrewCap int
	// Stage is the presence's stage (a rung's StageMin gates on it).
	Stage int
	// TypeMix is a typed doctrine's share of bodies per unlocked type at
	// its stage (the hive; nil for an untyped one): the grow rung lays the
	// egg of the type most under it.
	TypeMix map[string]float64
	// Structures caps a build rung's count per blueprint at the stage (the
	// hive's egg clusters); nil: no cap.
	Structures map[string]int
	// LaunchReady: the launcher would fire today by the Tier 1 rule (the
	// hive); ShotPop bodies walk to the Launcher blueprint's standing one.
	LaunchReady bool
	ShotPop     int
	Launcher    string
	// Economy is the types a shot takes first (the drone, then the scout).
	Economy []string
	// Types are a typed doctrine's unit types (the deploy rung reads a
	// type's deploy); Unlocked the ones its settlement can raise (the raid
	// rung's leader). SignalDir points from the centre toward the nearest
	// site its signal could found a presence on (0, 0: none known), where
	// the autonz deploy their relay pylons.
	Types     map[string]*campaign.DoctrineType
	Unlocked  map[string]bool
	SignalDir [2]float64
}

// Director plans for one settlement. Its choices are deterministic given the
// level, the tick and Seed.
type Director struct {
	Seed int64
	// LastPlan is the tick of the last plan (-1 before the first).
	LastPlan int
	// Ordered is every order it has given, by kind ("build:bed",
	// "research:basic_metallurgy", "mine:metal_ore", "haul:ration_pack",
	// "craft:stun_baton"), Cancelled the ones it gave up as stuck.
	Ordered   map[string]int
	Cancelled int
	// Log is the latest decisions, newest last (at most logMax).
	Log []string

	// shaftRes is the resource its shaft digs for ("" before the first).
	shaftRes string

	// Raids and hunts (phase D): when the last raid set out (-1 never),
	// when each open order was given, and the food-dropping blueprints.
	lastRaid  int
	raidStart int
	party     []*task.Task
	huntSince map[*task.Task]int
	foodDrop  map[string]bool

	// The hive's rungs: its guards' open attack orders, the last patrol.
	guardTasks map[*task.Task]bool
	lastPatrol int
	holds      []*task.Task
	// The autonz' deploy orders out (a relay drone walking to its site).
	deploys []*task.Task

	watch   map[*task.Task]*watched
	avoid   map[[3]int]bool
	sizeFor map[string][2]int
}

type watched struct {
	progress int
	since    int
}

const logMax = 20

// New is a director seeded for its settlement (the same seed gives the same
// choices on the same level).
func New(seed int64) *Director {
	return &Director{Seed: seed, LastPlan: -1, Ordered: map[string]int{}, lastRaid: -1,
		huntSince: map[*task.Task]int{}, foodDrop: map[string]bool{}, guardTasks: map[*task.Task]bool{}, lastPatrol: -1,
		watch: map[*task.Task]*watched{}, avoid: map[[3]int]bool{}, sizeFor: map[string][2]int{}}
}

// Update plans when the doctrine's interval has passed since the last plan.
func (d *Director) Update(s *Site, tick int) {
	cfg := s.config()
	if cfg == nil {
		return
	}
	every := cfg.EveryTicks
	if every <= 0 {
		every = 30
	}
	if d.LastPlan >= 0 && tick-d.LastPlan < every {
		return
	}
	d.Plan(s, tick)
}

func (s *Site) config() *campaign.DoctrineDirector {
	if s == nil || s.Doctrine == nil || s.Level == nil || s.Tasks == nil || s.Orders == nil {
		return nil
	}
	return s.Doctrine.Director
}

// snapshot is what one plan reads from the level.
type snapshot struct {
	colonists []*ecs.Entity
	maxInt    int
	built     map[string]int         // structures within Radius by blueprint
	stations  map[string]*ecs.Entity // a crafting station per station id, nearest the centre
	lab       *ecs.Entity            // a research building, nearest the centre
	stock     map[string]int         // colony storage contents
	carried   map[string]int         // colonists' bags
	openBuild map[string]int         // open build orders by type
	openCraft map[string]int         // open craft orders by recipe
	research  bool                   // a research order is open
	mines     map[string]int         // open mine orders by the resource they yield
	hauls     map[*ecs.Entity]bool   // items an open haul order targets
	taskAt    map[[3]int]bool        // tiles with an open order
	consuming int                    // open build + research + craft orders
	// free is the stock and carried materials not yet promised to an open
	// order (its cost less what has been delivered to it).
	free map[string]int
	// reach is the tiles on the centre's level the colonists can walk to
	// (within Reach of the centre): open ground through nothing solid but
	// workers and doors their faction may pass.
	reach map[[2]int]bool
	// blocked is the footprints of open build orders: walls-to-be.
	blocked map[[2]int]bool
	// under is the tiles below the centre's level the colonists can walk to
	// down the settlement's stairs (within DigReach and DigDepth).
	under map[[3]int]bool
	// digging: a dig or stair order is open (one at a time), or one was
	// given this pass.
	digging bool
	// converts is the open ConvertAction orders; noConvert: the convert
	// rung wanted more and found no ground to lay it on (the excavate rung
	// digs then).
	converts  int
	noConvert bool
	// heldBack: the shaft's remaining stairs are held back this pass.
	heldBack bool
	// short is the resources a mine rung wanted and found no face for (the
	// ones short first, then the ones under twice their target).
	short []string
	// food is the food items stored or carried; hunted the creatures an
	// open hunt targets; hunts the open hunt orders; equips the open equip
	// orders.
	food   int
	hunger int // the hungriest colonist's
	hunted map[*ecs.Entity]bool
	hunts  []*task.Task
	equips int
}

// afford reports whether free covers cost, and if so promises it.
func (snap *snapshot) afford(cost map[string]int) bool {
	for k, v := range cost {
		if snap.free[k] < v {
			return false
		}
	}
	for k, v := range cost {
		snap.free[k] -= v
	}
	return true
}

// Plan runs one pass of the ladder at tick.
func (d *Director) Plan(s *Site, tick int) {
	cfg := s.config()
	if cfg == nil {
		return
	}
	d.LastPlan = tick
	d.cancelStuck(s, cfg, tick)
	snap := d.look(s, cfg)
	d.holdBegunShaft(s, cfg, snap)
	d.forgetDeploys()
	rng := rand.New(rand.NewSource(d.Seed*1000003 + int64(tick)))
	budget := max(cfg.MaxOpen, 1) - snap.consuming
	n := len(snap.colonists)
	unmet := false // a rung so far has a build order still open
	for _, p := range cfg.Priorities {
		if p.StageMin > s.Stage {
			continue // not at this stage yet (the hive's chamber, its spire)
		}
		if p.Guard != nil && p.Grow == nil {
			d.guard(s, snap, p.Guard, tick) // a structure's guards (the autonz' sentries)
		}
		switch {
		case p.Deploy != nil:
			if d.deploy(s, cfg, snap, p.Deploy, budget > 0, tick) {
				budget--
			}
		case p.Haul != "":
			d.haul(s, cfg, snap, p, tick)
		case len(p.Mine) > 0:
			d.mine(s, cfg, snap, p, rng, tick)
		case p.Convert != "":
			d.convert(s, cfg, snap, p, tick)
		case p.Hunt != "":
			if snap.hunger < p.Hunger {
				continue // nobody hungry yet: the opening's builds first
			}
			d.hunt(s, cfg, snap, p, tick)
			if p.Hunt == "food" && budget > 0 && d.feed(s, cfg, snap, p, rng, tick) {
				budget--
			}
		case p.Raid != nil:
			d.raid(s, snap, p.Raid, tick)
		case p.Launch != nil:
			d.launch(s, snap, p.Launch, tick)
		case p.Patrol != nil:
			d.patrol(s, snap, p.Patrol, tick)
		case p.Excavate != nil:
			d.excavate(s, cfg, snap, p.Excavate, tick)
		case p.Grow != nil:
			if p.Guard != nil {
				d.guard(s, snap, p.Guard, tick) // the guard rung grows its warriors too
			}
			if budget <= 0 {
				continue
			}
			if s.TypeMix != nil {
				if d.growTyped(s, snap, *p.Grow, tick) {
					budget--
				}
			} else if d.grow(s, snap, tick) {
				budget--
			}
		case p.Dig:
			// Not while a structure a rung above ordered is still going up:
			// the rungs above (a lab, then its research) have first claim on
			// the kit, and a shaft begun meanwhile would hold back what they
			// need next; the rungs below wait too, or they would spend it
			// first. What the rungs above cannot afford does not hold the
			// shaft up.
			if unmet {
				if len(snap.short) > 0 {
					budget = 0 // and the rungs below wait with it
				}
				continue
			}
			for _, res := range snap.short {
				d.dig(s, cfg, snap, res, tick)
			}
		case p.Build != "":
			if budget <= 0 {
				if snap.openBuild[p.Build] > 0 {
					unmet = true
				}
				continue
			}
			want := p.Want(n)
			if c, ok := s.Structures[p.Build]; ok && s.Structures != nil && c < want {
				want = c // the stage's count (the hive's egg clusters)
			}
			if snap.openBuild[p.Build] > 0 {
				unmet = true
			}
			if snap.built[p.Build]+snap.openBuild[p.Build] >= want {
				continue
			}
			if d.build(s, cfg, snap, p.Build, rng, tick) {
				budget--
				unmet = true
			}
		case p.Research:
			if budget <= 0 || snap.research || snap.lab == nil {
				continue
			}
			if d.research(s, snap, tick) {
				budget--
			}
		case p.Craft != "":
			if budget <= 0 {
				continue
			}
			if d.craft(s, snap, p, tick) {
				budget--
			}
		}
	}
}

func (d *Director) note(kind, line string, tick int) {
	d.Ordered[kind]++
	d.Log = append(d.Log, fmt.Sprintf("%d %s", tick, line))
	if len(d.Log) > logMax {
		d.Log = d.Log[len(d.Log)-logMax:]
	}
}

// look reads the level and the settlement's open orders.
func (d *Director) look(s *Site, cfg *campaign.DoctrineDirector) *snapshot {
	snap := &snapshot{built: map[string]int{}, stations: map[string]*ecs.Entity{}, stock: map[string]int{},
		carried: map[string]int{}, openBuild: map[string]int{}, openCraft: map[string]int{},
		mines: map[string]int{}, hauls: map[*ecs.Entity]bool{}, taskAt: map[[3]int]bool{}, hunted: map[*ecs.Entity]bool{}}
	lvl := s.Level
	stationDist := map[string]int{}
	labDist := 1 << 30
	for _, list := range [][]*ecs.Entity{lvl.Entities, lvl.StaticEntities} {
		for _, e := range list {
			if e == nil {
				continue
			}
			if components.IsColonyWorker(e, s.Colony) {
				snap.colonists = append(snap.colonists, e)
				if e.HasComponent(components.Needs) {
					snap.hunger = max(snap.hunger, e.GetComponent(components.Needs).(*components.NeedsComponent).Hunger)
				}
				if e.HasComponent(rlcomponents.Stats) {
					snap.maxInt = max(snap.maxInt, e.GetComponent(rlcomponents.Stats).(*rlcomponents.StatsComponent).Int)
				}
				if e.HasComponent(rlcomponents.Inventory) {
					for _, it := range e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag {
						snap.carried[it.Blueprint] += units(it)
						if it.HasComponent(rlcomponents.Food) {
							snap.food++
						}
					}
				}
				continue
			}
			if e.HasComponent(components.Storage) {
				sc := e.GetComponent(components.Storage).(*components.StorageComponent)
				if sc.OwnedBy == s.Colony {
					for _, it := range sc.Items {
						snap.stock[it.Blueprint] += units(it)
						if it.HasComponent(rlcomponents.Food) {
							snap.food++
						}
					}
				}
			}
			x, y, z, ok := pos(e)
			if !ok || z != s.CZ {
				continue
			}
			dist := cheb(x-s.CX, y-s.CY)
			if dist > cfg.Radius+2 {
				continue
			}
			if e.HasComponent(rlcomponents.Item) {
				continue // loose things are not structures
			}
			snap.built[e.Blueprint]++
			if e.HasComponent(components.CraftingStation) {
				id := e.GetComponent(components.CraftingStation).(*components.CraftingStationComponent).StationID
				if old, ok := stationDist[id]; !ok || dist < old {
					stationDist[id], snap.stations[id] = dist, e
				}
			}
			if e.HasComponent(components.ResearchBuilding) && dist < labDist {
				labDist, snap.lab = dist, e
			}
		}
	}
	sort.Slice(snap.colonists, func(i, j int) bool { return less(snap.colonists[i], snap.colonists[j]) })
	snap.blocked = map[[2]int]bool{}
	snap.free = map[string]int{}
	for k, v := range snap.stock {
		snap.free[k] += v
	}
	for k, v := range snap.carried {
		snap.free[k] += v
	}
	promise := func(cost, delivered map[string]int) {
		for k, v := range cost {
			snap.free[k] -= max(v-delivered[k], 0)
		}
	}
	for _, t := range s.Tasks.GetTasks() {
		if t.Completed {
			continue
		}
		snap.taskAt[[3]int{t.X, t.Y, t.Z}] = true
		switch r := t.Data.(type) {
		case task_requests.BuildRequest:
			snap.openBuild[r.Type]++
			if r.Type == stairsDown {
				snap.digging = true
			}
			snap.consuming++
			promise(construction.GetBuildable(r.Type).Cost, r.Delivered)
		case task_requests.CraftRequest:
			snap.openCraft[r.RecipeID]++
			snap.consuming++
			if rc, ok := crafting.GetRecipe(r.RecipeID); ok {
				promise(rc.Cost, r.Delivered)
			}
		case *task_requests.ResearchRequest:
			snap.research = true
			snap.consuming++
			if tc, ok := research.GetTech(r.TechKey); ok {
				promise(tc.Cost, r.Delivered)
			}
		case task_requests.MineRequest:
			snap.mines[mineYield(lvl, r)]++
		case task_requests.DigRequest:
			snap.digging = true
		case task_requests.ConvertRequest:
			snap.converts++
		case task_requests.RetrieveRequest:
			if r.Item != nil {
				snap.hauls[r.Item] = true
			}
		case task_requests.EquipRequest:
			snap.equips++
		case *ecs.Entity:
			if t.Action == task_requests.AttackAction {
				snap.hunts = append(snap.hunts, t)
				snap.hunted[r] = true
			}
		}
	}
	for _, t := range s.Tasks.GetTasks() {
		if br, ok := t.Data.(task_requests.BuildRequest); ok && !t.Completed && br.Z == s.CZ {
			for _, f := range d.footprint(br.Type, construction.GetBuildable(br.Type).IsEntity, br.X, br.Y) {
				snap.blocked[f] = true
			}
		}
	}
	snap.reach = reachableExcept(s, cfg, snap.colonists, snap.blocked)
	snap.under = underground(s, cfg, snap.reach)
	return snap
}

// cancelStuck gives up its own orders that made no progress in StuckTicks:
// the Cancel Task order, and the site is not used again.
func (d *Director) cancelStuck(s *Site, cfg *campaign.DoctrineDirector, tick int) {
	if cfg.StuckTicks <= 0 {
		return
	}
	live := map[*task.Task]bool{}
	for _, t := range s.Tasks.GetTasks() {
		live[t] = true
		w := d.watch[t]
		if w == nil {
			continue
		}
		if t.Completed {
			delete(d.watch, t)
			continue
		}
		if p := progressOf(t); p != w.progress {
			w.progress, w.since = p, tick
			continue
		}
		if tick-w.since >= cfg.StuckTicks {
			d.avoid[[3]int{t.X, t.Y, t.Z}] = true
			delete(d.watch, t)
			s.Orders.Cancel(t)
			d.Cancelled++
			d.Log = append(d.Log, fmt.Sprintf("%d cancelled stuck %s at %d,%d", tick, t.Action, t.X, t.Y))
		}
	}
	for t := range d.watch {
		if !live[t] {
			delete(d.watch, t)
		}
	}
}

// watchNew starts watching the orders it just gave (the open tasks it has
// not seen).
func (d *Director) watchNew(s *Site, before int, tick int) {
	ts := s.Tasks.GetTasks()
	for _, t := range ts[min(before, len(ts)):] {
		if _, ok := d.watch[t]; !ok {
			d.watch[t] = &watched{progress: progressOf(t), since: tick}
		}
	}
}

// progressOf is an order's progress: work done plus materials delivered,
// so a build that is still being supplied is not stuck.
func progressOf(t *task.Task) int {
	switch r := t.Data.(type) {
	case task_requests.BuildRequest:
		return r.Progress*1000 + sum(r.Delivered)
	case task_requests.CraftRequest:
		return r.Progress*1000 + sum(r.Delivered)
	case *task_requests.ResearchRequest:
		return r.Progress*1000 + sum(r.Delivered)
	case task_requests.MineRequest:
		return r.Progress
	case task_requests.DigRequest:
		return r.Progress
	case task_requests.ConvertRequest:
		return r.Progress
	}
	if t.InProgress {
		return 1
	}
	return 0
}

// ---- the ladder's orders ----

func (d *Director) build(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, kind string, rng *rand.Rand, tick int) bool {
	b := construction.GetBuildable(kind)
	if b.Name == "" || !s.Doctrine.CanBuild(kind) {
		return false
	}
	if b.RequiredTech != "" && !has(s.Known, b.RequiredTech) {
		return false
	}
	x, y, ok := d.siteFor(s, cfg, snap, kind, b.IsEntity, rng)
	if !ok || !snap.afford(b.Cost) {
		return false
	}
	before := len(s.Tasks.GetTasks())
	if !s.Orders.Build(kind, x, y, s.CZ) {
		return false
	}
	snap.openBuild[kind]++
	snap.consuming++
	for _, t := range d.footprint(kind, b.IsEntity, x, y) {
		snap.taskAt[[3]int{t[0], t[1], s.CZ}] = true
		snap.blocked[t] = true
	}
	snap.reach = reachableExcept(s, cfg, snap.colonists, snap.blocked)
	d.watchNew(s, before, tick)
	d.note("build:"+kind, fmt.Sprintf("build %s at %d,%d", kind, x, y), tick)
	return true
}

func (d *Director) research(s *Site, snap *snapshot, tick int) bool {
	for _, key := range s.Doctrine.Research {
		t, ok := research.GetTech(key)
		if !ok || has(s.Known, key) || (t.RequiresTech != "" && !has(s.Known, t.RequiresTech)) {
			continue
		}
		if t.RequiredInt > snap.maxInt {
			continue // no one here could work it
		}
		if !snap.afford(t.Cost) {
			return false // the next tech waits for its materials
		}
		before := len(s.Tasks.GetTasks())
		if !s.Orders.Research(key, snap.lab) {
			return false
		}
		snap.research = true
		snap.consuming++
		d.watchNew(s, before, tick)
		d.note("research:"+key, "research "+key, tick)
		return true
	}
	return false
}

func (d *Director) craft(s *Site, snap *snapshot, p campaign.DirectorPriority, tick int) bool {
	r, ok := crafting.GetRecipe(p.Craft)
	if !ok || !s.Doctrine.CanCraft(p.Craft) {
		return false
	}
	if r.RequiresTech != "" && !has(s.Known, r.RequiresTech) {
		return false
	}
	station := snap.stations[r.Station]
	if station == nil || snap.openCraft[p.Craft] > 0 {
		return false
	}
	if snap.stock[r.Output]+snap.carried[r.Output] >= p.Want(len(snap.colonists)) || !snap.afford(r.Cost) {
		return false
	}
	before := len(s.Tasks.GetTasks())
	if !s.Orders.Craft(p.Craft, station) {
		return false
	}
	snap.openCraft[p.Craft]++
	snap.consuming++
	d.watchNew(s, before, tick)
	d.note("craft:"+p.Craft, "craft "+p.Craft, tick)
	return true
}

// mine keeps up to MaxMines mining orders per resource open while storage
// (with what is carried) is short of the stock wanted: the nearest deposit
// face or choppable thing that drops it, within Reach of the centre.
func (d *Director) mine(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, p campaign.DirectorPriority, rng *rand.Rand, tick int) {
	keys := make([]string, 0, len(p.Mine))
	for k := range p.Mine {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var low []string
	for _, res := range keys {
		if snap.free[res] >= p.Mine[res] {
			// Enough beyond what open orders need. With no face to mine it
			// from, a stock under twice the target is still worth a shaft
			// (the dig rung): the colony will need it, and the stairs are
			// cheaper to fund now than once the kit is spent.
			if snap.free[res] < 2*p.Mine[res] && snap.mines[res] == 0 {
				if _, _, _, ok := d.nearestSource(s, cfg, snap, res, rng); !ok {
					low = append(low, res)
				}
			}
			continue
		}
		for snap.mines[res] < max(cfg.MaxMines, 1) {
			x, y, z, ok := d.nearestSource(s, cfg, snap, res, rng)
			if !ok {
				if snap.mines[res] == 0 {
					snap.short = append(snap.short, res)
				}
				break
			}
			before := len(s.Tasks.GetTasks())
			if !s.Orders.Mine(x, y, z) {
				d.avoid[[3]int{x, y, z}] = true
				break
			}
			snap.mines[res]++
			snap.taskAt[[3]int{x, y, z}] = true
			d.watchNew(s, before, tick)
			d.note("mine:"+res, fmt.Sprintf("mine %s at %d,%d,%d", res, x, y, z), tick)
		}
	}
	snap.short = append(snap.short, low...) // short ones first
}

// haul orders loose items of the entry's kind ("food": anything edible)
// within Reach brought into storage, nearest first. Food is fetched as far
// as the hunt rung hunts (HuntReach): a hunted creature drops its meat where
// it dies, often well beyond Reach, and nothing else brings it home (the
// TestMirrorFedFortyDays flake: crew starved beside a field of uncollected
// protein bars). Past Reach, like a hunt, there is no flood to check; a
// haul that cannot get there is cancelled as stuck.
func (d *Director) haul(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, p campaign.DirectorPriority, tick int) {
	open := len(snap.hauls)
	if open >= max(cfg.MaxHauls, 1) {
		return
	}
	reach := cfg.Reach
	if p.Haul == "food" {
		reach = max(reach, cfg.HuntReach)
	}
	type cand struct {
		e    *ecs.Entity
		dist int
	}
	var cands []cand
	for _, e := range s.Level.Entities {
		if e == nil || !e.HasComponent(rlcomponents.Item) || snap.hauls[e] {
			continue
		}
		if p.Haul == "food" {
			if !e.HasComponent(rlcomponents.Food) {
				continue
			}
		} else if e.Blueprint != p.Haul {
			continue
		}
		x, y, z, ok := pos(e)
		if !ok || z != s.CZ || d.avoid[[3]int{x, y, z}] {
			continue
		}
		if dist := cheb(x-s.CX, y-s.CY); dist <= reach && (dist > cfg.Reach || snap.reach[[2]int{x, y}]) {
			cands = append(cands, cand{e, dist})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		return less(cands[i].e, cands[j].e)
	})
	for _, c := range cands {
		if open >= max(cfg.MaxHauls, 1) {
			return
		}
		before := len(s.Tasks.GetTasks())
		if !s.Orders.Retrieve(c.e) {
			continue
		}
		open++
		snap.hauls[c.e] = true
		d.watchNew(s, before, tick)
		d.note("haul:"+c.e.Blueprint, "haul "+c.e.Blueprint, tick)
	}
}

// ---- the level ----

// siteFor is the build site nearest the centre on the director's grid:
// columns two apart and rows three apart around the centre, so every
// structure (a bed is two tall) keeps a walkway beside it. The seed picks
// where in each ring the search starts.
func (d *Director) siteFor(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, kind string, entity bool, rng *rand.Rand) (int, int, bool) {
	start := rng.Intn(8)
	for r := 1; r*2 <= cfg.Radius; r++ {
		var ring [][2]int
		for gx := -r; gx <= r; gx++ {
			for gy := -r; gy <= r; gy++ {
				if max(abs(gx), abs(gy)) != r {
					continue
				}
				ring = append(ring, [2]int{s.CX + gx*2, s.CY + gy*3})
			}
		}
		sort.Slice(ring, func(i, j int) bool {
			return ring[i][1] < ring[j][1] || ring[i][1] == ring[j][1] && ring[i][0] < ring[j][0]
		})
		for k := range ring {
			p := ring[(k+start)%len(ring)]
			if d.free(s, snap, kind, entity, p[0], p[1]) {
				return p[0], p[1], true
			}
		}
	}
	return 0, 0, false
}

// footprint is the tiles a build at (x, y) covers (BuildRequest's
// convention: a sized entity is centred, start = pos - size/2).
func (d *Director) footprint(kind string, entity bool, x, y int) [][2]int {
	w, h := 1, 1
	if entity {
		sz, ok := d.sizeFor[kind]
		if !ok {
			sz = [2]int{1, 1}
			if sc := factory.GetSize(kind); sc != nil && sc.Width > 0 && sc.Height > 0 {
				sz = [2]int{sc.Width, sc.Height}
			}
			d.sizeFor[kind] = sz
		}
		w, h = sz[0], sz[1]
	}
	var out [][2]int
	for dx := 0; dx < w; dx++ {
		for dy := 0; dy < h; dy++ {
			out = append(out, [2]int{x - w/2 + dx, y - h/2 + dy})
		}
	}
	return out
}

// free reports whether a build of kind can go at (x, y): every footprint
// tile open ground with nothing on it and no order on it, not a site it gave
// up on, and an open tile beside it to work from.
func (d *Director) free(s *Site, snap *snapshot, kind string, entity bool, x, y int) bool {
	z := s.CZ
	if d.avoid[[3]int{x, y, z}] || (x == s.CX && y == s.CY) {
		return false
	}
	if !construction.GetBuildable(kind).FloorOK(d.footprint(kind, entity, x, y), func(tx, ty int) string { return floorName(s.Level, tx, ty, z) }) {
		return false // a hive structure stands on resin
	}
	var buf []*ecs.Entity
	for _, t := range d.footprint(kind, entity, x, y) {
		if !s.Level.InBounds(t[0], t[1], z) || !open(s.Level, t[0], t[1], z) || snap.taskAt[[3]int{t[0], t[1], z}] {
			return false
		}
		buf = buf[:0]
		s.Level.GetEntitiesAt(t[0], t[1], z, &buf)
		if len(buf) > 0 {
			return false
		}
	}
	workable := false
	for _, n := range [][2]int{{x - 1, y}, {x + 1, y}} {
		if snap.reach[n] && s.Level.GetSolidEntityAt(n[0], n[1], z) == nil {
			workable = true
		}
	}
	if !workable {
		return false
	}
	// It must not cut anyone off: with the footprint blocked, everything
	// else they reach now stays reachable (a bed in a doorway would wall the
	// crew in).
	blocked := map[[2]int]bool{}
	for k := range snap.blocked {
		blocked[k] = true
	}
	lost := 0
	for _, t := range d.footprint(kind, entity, x, y) {
		blocked[t] = true
		if snap.reach[t] {
			lost++
		}
	}
	return len(reachableExcept(s, d.cfg(s), snap.colonists, blocked)) >= len(snap.reach)-lost
}

func (d *Director) cfg(s *Site) *campaign.DoctrineDirector { return s.config() }

// reachable is the tiles the colonists can walk to on the centre's level,
// within Reach of the centre (a flood from where they stand, four ways).
func reachable(s *Site, cfg *campaign.DoctrineDirector, colonists []*ecs.Entity) map[[2]int]bool {
	return reachableExcept(s, cfg, colonists, nil)
}

// reachableExcept is reachable with the blocked tiles walled off.
func reachableExcept(s *Site, cfg *campaign.DoctrineDirector, colonists []*ecs.Entity, blocked map[[2]int]bool) map[[2]int]bool {
	out := map[[2]int]bool{}
	z := s.CZ
	faction := ""
	for _, e := range colonists {
		if e.HasComponent(rlcomponents.Description) {
			faction = e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction
			break
		}
	}
	// One flood from the centre (the landing), so everything counted is
	// connected to it: seeding from every colonist would join two areas a
	// build has just cut apart. With the centre blocked, from the colonist
	// nearest it.
	seed, ok := [2]int{s.CX, s.CY}, open(s.Level, s.CX, s.CY, z) && !blocked[[2]int{s.CX, s.CY}]
	if !ok {
		best := 1 << 30
		for _, e := range colonists {
			x, y, ez, has := pos(e)
			if has && ez == z && !blocked[[2]int{x, y}] && cheb(x-s.CX, y-s.CY) < best {
				best, seed, ok = cheb(x-s.CX, y-s.CY), [2]int{x, y}, true
			}
		}
	}
	if !ok {
		return out
	}
	out[seed] = true
	queue := [][2]int{seed}
	reach := max(cfg.Reach, cfg.Radius+2)
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := [2]int{p[0] + d[0], p[1] + d[1]}
			if out[n] || blocked[n] || cheb(n[0]-s.CX, n[1]-s.CY) > reach || !s.Level.InBounds(n[0], n[1], z) || !open(s.Level, n[0], n[1], z) {
				continue
			}
			if b := s.Level.GetSolidEntityAt(n[0], n[1], z); b != nil && !b.HasComponent(components.Worker) &&
				!(faction != "" && world.IsDoorPassableByFaction(b, faction)) {
				continue
			}
			out[n] = true
			queue = append(queue, n)
		}
	}
	return out
}

// open reports a standable tile: a floor and no solid middle.
func open(lvl *world.Level, x, y, z int) bool {
	ti := lvl.GetTileAt(x, y, z)
	if ti == nil {
		return false
	}
	t := ti.(*world.Tile)
	if !t.Middle.IsEmpty() {
		def := world.TileDefinitions[t.Middle.Type]
		if def.StairsUp || def.StairsDown {
			return true // stairs stand on their own
		}
		if def.Solid {
			return false
		}
	}
	return !t.Floor.IsEmpty()
}

// stairs reports a stair tile.
func stairs(lvl *world.Level, x, y, z int) (up, down bool) {
	ti := lvl.GetTileAt(x, y, z)
	if ti == nil {
		return false, false
	}
	t := ti.(*world.Tile)
	if t.Middle.IsEmpty() {
		return false, false
	}
	def := world.TileDefinitions[t.Middle.Type]
	return def.StairsUp, def.StairsDown
}

// nearestSource is the nearest deposit face (a deposit tile beside open
// ground) or choppable entity that yields res, within Reach, with no order
// on it. Ties go to the seeded order.
func (d *Director) nearestSource(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, res string, rng *rand.Rand) (int, int, int, bool) {
	z := s.CZ
	type cand struct{ x, y, z, dist int }
	var best []cand
	bestDist := 1 << 30
	consider := func(x, y int) {
		if snap.taskAt[[3]int{x, y, z}] || d.avoid[[3]int{x, y, z}] {
			return
		}
		dist := cheb(x-s.CX, y-s.CY)
		if dist < bestDist {
			best, bestDist = best[:0], dist
		}
		if dist == bestDist {
			best = append(best, cand{x, y, z, dist})
		}
	}
	for r := 0; r <= cfg.Reach; r++ {
		if r > bestDist {
			break
		}
		for dx := -r; dx <= r; dx++ {
			for dy := -r; dy <= r; dy++ {
				if max(abs(dx), abs(dy)) != r {
					continue
				}
				x, y := s.CX+dx, s.CY+dy
				if !s.Level.InBounds(x, y, z) {
					continue
				}
				if yields(s.Level, x, y, z) == res && face(snap.reach, x, y) {
					consider(x, y)
				}
			}
		}
	}
	// Flora is mostly static (trees): look in both lists.
	for _, list := range [][]*ecs.Entity{s.Level.Entities, s.Level.StaticEntities} {
		for _, e := range list {
			if e == nil || !e.HasComponent(components.Choppable) || !e.HasComponent(components.Drops) {
				continue
			}
			x, y, ez, ok := pos(e)
			if !ok || ez != z || cheb(x-s.CX, y-s.CY) > cfg.Reach {
				continue
			}
			if has(e.GetComponent(components.Drops).(*components.DropsComponent).Items, res) && (snap.reach[[2]int{x, y}] || face(snap.reach, x, y)) {
				consider(x, y)
			}
		}
	}
	if len(best) == 0 {
		// Nothing on the surface: a face below, down the settlement's stairs,
		// nearest the centre across, then shallowest.
		if c, ok := undergroundFace(s, snap, res, d.avoid); ok {
			return c[0], c[1], c[2], true
		}
		return 0, 0, 0, false
	}
	sort.Slice(best, func(i, j int) bool { return best[i].y < best[j].y || best[i].y == best[j].y && best[i].x < best[j].x })
	c := best[rng.Intn(len(best))]
	return c.x, c.y, c.z, true
}

// yields is the resource the deposit tile at (x, y, z) mines out as, "" for
// none or an exhausted one.
func yields(lvl *world.Level, x, y, z int) string {
	ti := lvl.GetTileAt(x, y, z)
	if ti == nil {
		return ""
	}
	t := ti.(*world.Tile)
	if t.Middle.IsEmpty() {
		return ""
	}
	return world.DepositDrop(world.TileDefinitions[t.Middle.Type].Name)
}

// mineYield is what a mine order yields: its target's drops or its tile's.
func mineYield(lvl *world.Level, r task_requests.MineRequest) string {
	if r.Target != nil && r.Target.HasComponent(components.Drops) {
		if items := r.Target.GetComponent(components.Drops).(*components.DropsComponent).Items; len(items) > 0 {
			return items[0]
		}
	}
	return yields(lvl, r.X, r.Y, r.Z)
}

// face reports a tile with a reachable tile beside it.
func face(reach map[[2]int]bool, x, y int) bool {
	for _, n := range [][2]int{{x - 1, y}, {x + 1, y}, {x, y - 1}, {x, y + 1}} {
		if reach[n] {
			return true
		}
	}
	return false
}

// ---- helpers ----

func pos(e *ecs.Entity) (int, int, int, bool) {
	if !e.HasComponent(rlcomponents.Position) {
		return 0, 0, 0, false
	}
	pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	return pc.GetX(), pc.GetY(), pc.GetZ(), true
}

// less orders entities by position then blueprint, for deterministic picks.
func less(a, b *ecs.Entity) bool {
	ax, ay, az, _ := pos(a)
	bx, by, bz, _ := pos(b)
	if az != bz {
		return az < bz
	}
	if ay != by {
		return ay < by
	}
	if ax != bx {
		return ax < bx
	}
	return a.Blueprint < b.Blueprint
}

func units(e *ecs.Entity) int {
	if e.HasComponent(components.Material) {
		return e.GetComponent(components.Material).(*components.MaterialComponent).Quantity
	}
	return 1
}

func has(list []string, k string) bool {
	for _, x := range list {
		if x == k {
			return true
		}
	}
	return false
}

func sum(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func cheb(dx, dy int) int { return max(abs(dx), abs(dy)) }
