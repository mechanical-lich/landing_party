package director

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

// Phase D (docs/developer/factions.md, "Raids and hunts"): the two rungs
// that close C2's gaps. A hostile faction settlement raids the player; every
// settlement hunts for food, so a rival colony on a long stay does not
// starve once its kit's rations are eaten.

// defaultAggression is the doctrine aggression the raid numbers are written
// for.
const defaultAggression = 5

// scaled is n at aggression 5, scaled by 5 / aggression, at least 1.
func scaled(n, aggression int) int {
	if aggression <= 0 {
		aggression = defaultAggression
	}
	v := (n*defaultAggression + aggression - 1) / aggression
	return max(v, 1)
}

// forgetDone drops finished orders from a since-map.
func forgetDone(m map[*task.Task]int, s *Site) {
	live := map[*task.Task]bool{}
	for _, t := range s.Tasks.GetTasks() {
		if !t.Completed {
			live[t] = true
		}
	}
	for t := range m {
		if !live[t] {
			delete(m, t)
		}
	}
}

// ---- hunting ----

// dropsFood reports whether a creature's drops include something edible.
func (d *Director) dropsFood(e *ecs.Entity) bool {
	if !e.HasComponent(components.Drops) {
		return false
	}
	for _, bp := range e.GetComponent(components.Drops).(*components.DropsComponent).Items {
		if d.isFood(bp) {
			return true
		}
	}
	return false
}

// hunt keeps up to MaxHunts attack orders open on the nearest reachable
// creatures that drop food, while the food stored and carried (with one
// per open hunt) is under the rung's want. A hunt that runs HuntTicks is
// called off (the creature fled out of reach).
func (d *Director) hunt(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, p campaign.DirectorPriority, tick int) {
	forgetDone(d.huntSince, s)
	limit := cfg.HuntTicks
	if limit <= 0 {
		limit = 750
	}
	open := snap.hunts[:0:0]
	for _, t := range snap.hunts {
		since, ok := d.huntSince[t]
		if !ok {
			d.huntSince[t] = tick // given before a reload
			since = tick
		}
		if tick-since >= limit {
			s.Orders.Cancel(t)
			delete(d.huntSince, t)
			d.Log = append(d.Log, fmt.Sprintf("%d called off a hunt", tick))
			continue
		}
		open = append(open, t)
	}
	maxHunts := cfg.MaxHunts
	if maxHunts <= 0 {
		maxHunts = 2
	}
	// "food": creatures that drop food, against the food stored and
	// carried; "biomass" (the hive, whose every kill is biomass): any
	// creature not of its own faction, against the biomass stored and
	// carried.
	have := snap.food
	prey := d.dropsFood
	if p.Hunt != "food" {
		// A kill-drop item (the hive's biomass, the greys' specimens): any
		// creature not of its own faction, against that item stored and
		// carried.
		have = snap.stock[p.Hunt] + snap.carried[p.Hunt]
		own := ""
		if i := strings.IndexByte(s.Colony, ':'); i > 0 {
			own = s.Colony[:i]
		}
		prey = func(e *ecs.Entity) bool {
			return !e.HasComponent(rlcomponents.Inanimate) && faction(e) != own &&
				(own == "" || faction(e) == "" || components.HostileFactions(own, faction(e))) &&
				(e.HasComponent(components.ScriptedAI) || e.HasComponent(components.FactionAI) || e.HasComponent(components.Drops))
		}
	}
	if have+len(open) >= p.Want(len(snap.colonists)) || len(open) >= maxHunts {
		return
	}
	reach := cfg.HuntReach
	if reach <= 0 {
		reach = 2 * cfg.Reach
	}
	type cand struct {
		e    *ecs.Entity
		dist int
	}
	var cands []cand
	for _, e := range s.Level.Entities {
		if e == nil || snap.hunted[e] || e.HasComponent(rlcomponents.Dead) || e.HasComponent(components.Worker) ||
			e.HasComponent(rlcomponents.Item) || !e.HasComponent(rlcomponents.Health) || !prey(e) {
			continue
		}
		x, y, z, ok := pos(e)
		if !ok || z != s.CZ {
			continue
		}
		dist := cheb(x-s.CX, y-s.CY)
		if dist > reach || dist <= cfg.Reach && !nearReach(snap, x, y) {
			continue // beyond the flood, game wanders: HuntTicks calls it off
		}
		cands = append(cands, cand{e, dist})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		return less(cands[i].e, cands[j].e)
	})
	for _, c := range cands {
		if len(open) >= maxHunts || have+len(open) >= p.Want(len(snap.colonists)) {
			return
		}
		before := len(s.Tasks.GetTasks())
		if !s.Orders.Attack(c.e) {
			continue
		}
		ts := s.Tasks.GetTasks()
		for _, t := range ts[min(before, len(ts)):] {
			d.huntSince[t] = tick
			open = append(open, t)
		}
		snap.hunted[c.e] = true
		d.note("hunt:"+c.e.Blueprint, "hunt "+c.e.Blueprint, tick)
	}
}

// feed is the food rung's other source: while food is short, a recipe of
// the doctrine's whose output is food (the colony's protein_press), at its
// station, when the stock covers it. One at a time.
func (d *Director) feed(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, p campaign.DirectorPriority, rng *rand.Rand, tick int) bool {
	if snap.food >= p.Want(len(snap.colonists)) {
		return false
	}
	for _, key := range append(append([]string(nil), s.Doctrine.Units...), s.Doctrine.Recipes...) {
		r, ok := crafting.GetRecipe(key)
		if !ok || !d.isFood(r.Output) {
			continue
		}
		if snap.openCraft[key] > 0 {
			return false
		}
		if r.RequiresTech != "" && !has(s.Known, r.RequiresTech) {
			continue
		}
		station := snap.stations[r.Station]
		if station == nil {
			continue
		}
		if !snap.afford(r.Cost) {
			// Its inputs first: chop or mine them (twice the recipe's).
			want := map[string]int{}
			for k, v := range r.Cost {
				if snap.free[k] < v {
					want[k] = 2 * v
				}
			}
			d.mine(s, cfg, snap, campaign.DirectorPriority{ID: p.ID, Mine: want}, rng, tick)
			continue
		}
		before := len(s.Tasks.GetTasks())
		if !s.Orders.Craft(key, station) {
			continue
		}
		snap.openCraft[key]++
		snap.consuming++
		d.watchNew(s, before, tick)
		d.note("craft:"+key, "craft "+key+" (food)", tick)
		return true
	}
	return false
}

// isFood reports whether blueprint is edible.
func (d *Director) isFood(bp string) bool {
	food, ok := d.foodDrop[bp]
	if !ok {
		if it, err := factory.Create(bp, 0, 0, 0); err == nil {
			food = it.HasComponent(rlcomponents.Food)
		}
		d.foodDrop[bp] = food
	}
	return food
}

// nearReach reports whether (x, y) or a tile beside it is reachable.
func nearReach(snap *snapshot, x, y int) bool {
	for _, n := range [][2]int{{x, y}, {x - 1, y}, {x + 1, y}, {x, y - 1}, {x, y + 1}} {
		if snap.reach[n] {
			return true
		}
	}
	return false
}

// ---- raiding ----

// armed reports whether a worker holds a weapon.
func armed(e *ecs.Entity) bool {
	if !e.HasComponent(rlcomponents.Inventory) {
		return false
	}
	inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for _, it := range []*ecs.Entity{inv.LeftHand, inv.RightHand} {
		if it != nil && it.HasComponent(rlcomponents.Weapon) {
			return true
		}
	}
	return false
}

// raid is the raid rung. Only a hostile settlement raids. While a party is
// out it waits, and calls it home (ends its attack orders) after MaxTicks.
// Otherwise, once the cooldown since the last raid has passed: it equips
// stored weapons for unarmed workers, and when its fighters reach the
// threshold sends a party of them (the nearest the target first) against
// the player's colonist nearest its centre (else a storage container of the
// player's that can be hit). Party members are set on the target at once:
// the task queue hands out the nearest work, and a raid is never near.
func (d *Director) raid(s *Site, snap *snapshot, r *campaign.DirectorRaid, tick int) {
	out := d.party[:0]
	for _, t := range d.party {
		if !t.Completed {
			out = append(out, t)
		}
	}
	d.party = out
	if len(d.party) > 0 {
		if r.MaxTicks > 0 && tick-d.raidStart >= r.MaxTicks {
			for _, t := range d.party {
				t.Complete() // home: back to the queue's work
			}
			d.party = nil
			d.Log = append(d.Log, fmt.Sprintf("%d called the raid home", tick))
		}
		return
	}
	if !s.Hostile || s.Enemy == "" {
		return
	}
	if r.Scouted && !Contact(s.Level, s.Colony) {
		return // the hive raids what its scouts have found
	}
	if d.lastRaid >= 0 && tick-d.lastRaid < scaled(r.CooldownTicks, s.Aggression) {
		return
	}
	var fighters []*ecs.Entity
	unarmed := 0
	for _, e := range snap.colonists {
		if len(r.Roles) > 0 && !has(r.Roles, roleOf(s, e)) {
			continue // not a fighter
		}
		if r.Unarmed || armed(e) {
			fighters = append(fighters, e)
		} else {
			unarmed++
		}
	}
	if !r.Unarmed && unarmed > 0 && snap.equips == 0 {
		d.arm(s, unarmed, tick)
	}
	if len(fighters) < scaled(r.Fighters, s.Aggression) {
		return
	}
	leading := r.Leader != "" && s.Unlocked[r.Leader]
	isLeader := func(e *ecs.Entity) bool {
		k := components.KinOf(e)
		return leading && k != nil && k.Type == r.Leader
	}
	if leading {
		// Once wardens are known a party forms only around one.
		found := false
		for _, e := range fighters {
			found = found || isLeader(e)
		}
		if !found {
			return
		}
	}
	target := d.raidTarget(s, r)
	if target == nil {
		return
	}
	tx, ty, _, _ := pos(target)
	sort.SliceStable(fighters, func(i, j int) bool {
		if a, b := isLeader(fighters[i]), isLeader(fighters[j]); a != b {
			return a // the leader first: it is always in the party
		}
		ax, ay, _, _ := pos(fighters[i])
		bx, by, _, _ := pos(fighters[j])
		da, db := cheb(ax-tx, ay-ty), cheb(bx-tx, by-ty)
		if da != db {
			return da < db
		}
		return less(fighters[i], fighters[j])
	})
	// Someone always stays home (Home, at least 1).
	party := min(len(fighters), max(r.Party, 1), len(snap.colonists)-max(r.Home, 1))
	if party <= 0 {
		return
	}
	for _, e := range fighters[:party] {
		if t := s.Orders.Send(e, target); t != nil {
			d.party = append(d.party, t)
		}
	}
	if len(d.party) == 0 {
		return
	}
	d.lastRaid, d.raidStart = tick, tick
	d.Ordered["raids"]++
	d.note("raid:"+target.Blueprint, fmt.Sprintf("raid %s with %d", target.Blueprint, len(d.party)), tick)
}

// Raiding reports whether a raid party is out.
func (d *Director) Raiding() bool { return len(d.party) > 0 }

// arm orders up to n stored weapons into workers' hands.
func (d *Director) arm(s *Site, n, tick int) {
	for _, list := range [][]*ecs.Entity{s.Level.Entities, s.Level.StaticEntities} {
		for _, e := range list {
			if n <= 0 {
				return
			}
			if e == nil || !e.HasComponent(components.Storage) {
				continue
			}
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			if sc.OwnedBy != s.Colony {
				continue
			}
			for _, it := range sc.Items {
				if n <= 0 {
					return
				}
				if it.HasComponent(rlcomponents.Weapon) && s.Orders.Equip(it.Blueprint, e) {
					n--
					d.note("equip:"+it.Blueprint, "equip "+it.Blueprint, tick)
				}
			}
		}
	}
}

// raidTarget is the player's colonist nearest the centre within Reach on
// the centre's level, else the player's nearest storage container that can
// be hit, else nil.
func (d *Director) raidTarget(s *Site, r *campaign.DirectorRaid) *ecs.Entity {
	reach := r.Reach
	if reach <= 0 {
		reach = 120
	}
	var best, store *ecs.Entity
	bestD, storeD := 1<<30, 1<<30
	for _, list := range [][]*ecs.Entity{s.Level.Entities, s.Level.StaticEntities} {
		for _, e := range list {
			if e == nil || e.HasComponent(rlcomponents.Dead) {
				continue
			}
			x, y, z, ok := pos(e)
			if !ok || z != s.CZ {
				continue
			}
			dist := cheb(x-s.CX, y-s.CY)
			if dist > reach {
				continue
			}
			switch {
			case components.IsColonyWorker(e, s.Enemy):
				if dist < bestD || dist == bestD && less(e, best) {
					best, bestD = e, dist
				}
			case e.HasComponents(components.Storage, rlcomponents.Health) &&
				e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy == s.Enemy:
				if dist < storeD || dist == storeD && less(e, store) {
					store, storeD = e, dist
				}
			}
		}
	}
	if best != nil {
		return best
	}
	return store
}
