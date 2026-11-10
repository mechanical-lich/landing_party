package director

import (
	"fmt"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/construction"
)

// Digging (phase C2; docs/developer/factions.md, "The director digs"). When
// a resource the ladder wants is short and no face the colonists can reach
// yields it, the director goes down for it: it picks the nearest deposit
// below the centre within DigReach across and DigDepth down (a bounded
// search of the level, the ore a player would find by digging), and works
// toward it one order at a time, through the player's own orders:
//
//   - from the reachable tile nearest the ore (the deepest first, so a shaft
//     already begun is carried on), a Stairs Down while that tile is above
//     the ore's level: on the surface at that tile, below it on the next tile
//     toward the ore (a staircase stepping across as it descends, since one
//     tile cannot hold both an up and a down stair);
//   - on the ore's level, a tunnel toward it: the next tile Dug, or Mined
//     when it is a deposit itself.
//
// Once the ore is a face, the mine rung mines it like any other (and the
// miner hauls each chunk to storage, as for the player). Everything is read
// from the level each pass, so a reloaded settlement picks up its own shaft.
// The stairs still to build are held back from the other rungs' budget, so
// the kit is not spent before the shaft reaches the ore.

const stairsDown = "stairs_down"

// underground is the tiles below the centre's level the colonists can walk
// to: down every Stairs Down in reach, flooding four ways through open
// tiles and on down further stairs, within DigReach of the centre across and
// DigDepth below it.
func underground(s *Site, cfg *campaign.DoctrineDirector, reach map[[2]int]bool) map[[3]int]bool {
	out := map[[3]int]bool{}
	if cfg.DigDepth <= 0 {
		return out
	}
	var queue [][3]int
	push := func(p [3]int) {
		if out[p] || p[2] >= s.CZ || p[2] < s.CZ-cfg.DigDepth || p[2] < 0 {
			return
		}
		if cheb(p[0]-s.CX, p[1]-s.CY) > digReach(cfg) || !s.Level.InBounds(p[0], p[1], p[2]) || !open(s.Level, p[0], p[1], p[2]) {
			return
		}
		out[p] = true
		queue = append(queue, p)
	}
	keys := make([][2]int, 0)
	for k := range reach {
		if _, down := stairs(s.Level, k[0], k[1], s.CZ); down {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i][1] < keys[j][1] || keys[i][1] == keys[j][1] && keys[i][0] < keys[j][0]
	})
	for _, k := range keys {
		push([3]int{k[0], k[1], s.CZ - 1})
	}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			push([3]int{p[0] + d[0], p[1] + d[1], p[2]})
		}
		if up, down := stairs(s.Level, p[0], p[1], p[2]); down {
			push([3]int{p[0], p[1], p[2] - 1})
		} else if up && p[2]+1 < s.CZ {
			push([3]int{p[0], p[1], p[2] + 1})
		}
	}
	return out
}

func digReach(cfg *campaign.DoctrineDirector) int {
	if cfg.DigReach > 0 {
		return cfg.DigReach
	}
	return cfg.Reach
}

// undergroundFace is the nearest deposit yielding res beside a tile in under
// with no order on it: nearest the centre across, then shallowest.
func undergroundFace(s *Site, snap *snapshot, res string, avoid map[[3]int]bool) ([3]int, bool) {
	var best [3]int
	bestCost, found := 1<<30, false
	for p := range snap.under {
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			q := [3]int{p[0] + d[0], p[1] + d[1], p[2]}
			if snap.taskAt[q] || avoid[q] || yields(s.Level, q[0], q[1], q[2]) != res {
				continue
			}
			cost := cheb(q[0]-s.CX, q[1]-s.CY)*16 + (s.CZ - q[2])
			if cost < bestCost || cost == bestCost && lessPos(q, best) {
				best, bestCost, found = q, cost, true
			}
		}
	}
	return best, found
}

func lessPos(a, b [3]int) bool {
	if a[2] != b[2] {
		return a[2] < b[2]
	}
	if a[1] != b[1] {
		return a[1] < b[1]
	}
	return a[0] < b[0]
}

// oreBelow is the nearest deposit yielding res below the centre's level
// within DigReach across and DigDepth down: nearest across, then shallowest.
func oreBelow(s *Site, cfg *campaign.DoctrineDirector, res string, avoid map[[3]int]bool) ([3]int, bool) {
	r := digReach(cfg)
	for ring := 0; ring <= r; ring++ {
		for dz := 1; dz <= cfg.DigDepth; dz++ {
			z := s.CZ - dz
			if z < 0 {
				break
			}
			var hits [][3]int
			for dx := -ring; dx <= ring; dx++ {
				for dy := -ring; dy <= ring; dy++ {
					if max(abs(dx), abs(dy)) != ring {
						continue
					}
					x, y := s.CX+dx, s.CY+dy
					if !s.Level.InBounds(x, y, z) || avoid[[3]int{x, y, z}] {
						continue
					}
					if yields(s.Level, x, y, z) == res {
						hits = append(hits, [3]int{x, y, z})
					}
				}
			}
			if len(hits) > 0 {
				sort.Slice(hits, func(i, j int) bool { return lessPos(hits[i], hits[j]) })
				return hits[0], true
			}
		}
	}
	return [3]int{}, false
}

// dig gives the next order toward ore yielding res below, if there is any
// within range and no dig or stair order is open.
// shaftPlan is where a shaft toward ore yielding res stands: the ore, the
// reached tile nearest it (deepest first), and whether there is one.
func (d *Director) shaftPlan(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, res string) (ore, from [3]int, ok bool) {
	if cfg.DigDepth <= 0 {
		return ore, from, false
	}
	ore, ok = oreBelow(s, cfg, res, d.avoid)
	if !ok {
		return ore, from, false
	}
	// From the reached tile nearest the ore: deepest first (carry on a shaft
	// already begun), then nearest across.
	bestCost, found := 1<<30, false
	consider := func(p [3]int) {
		if p[2] < ore[2] {
			return
		}
		cost := (p[2]-ore[2])*100000 + cheb(p[0]-ore[0], p[1]-ore[1])
		if cost < bestCost || cost == bestCost && lessPos(p, from) {
			from, bestCost, found = p, cost, true
		}
	}
	for p := range snap.under {
		consider(p)
	}
	if !found || from[2] == s.CZ {
		// No shaft yet: a stair site on the surface.
		for k := range snap.reach {
			p := [3]int{k[0], k[1], s.CZ}
			if d.stairSite(s, snap, p) {
				consider(p)
			}
		}
	}
	return ore, from, found
}

// holdShaft holds the stairs a shaft still needs (less the next, which its
// own order promises) back from every rung, once a plan.
func (d *Director) holdShaft(snap *snapshot, levels int) {
	if snap.heldBack {
		return
	}
	snap.heldBack = true
	for k, v := range construction.GetBuildable(stairsDown).Cost {
		snap.free[k] -= v * max(levels-1, 0)
	}
}

// holdBegunShaft runs before the ladder: once a shaft is begun, the stairs
// it still needs are held back from every rung, the ones above the dig
// rung too (a second research must not spend the shaft's last stair).
func (d *Director) holdBegunShaft(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot) {
	if d.shaftRes == "" {
		return
	}
	ore, from, ok := d.shaftPlan(s, cfg, snap, d.shaftRes)
	if !ok || from[2] == s.CZ && d.Ordered["dig:stairs"] == 0 {
		return
	}
	d.holdShaft(snap, from[2]-ore[2])
}

func (d *Director) dig(s *Site, cfg *campaign.DoctrineDirector, snap *snapshot, res string, tick int) {
	ore, from, found := d.shaftPlan(s, cfg, snap, res)
	stairCost := construction.GetBuildable(stairsDown).Cost
	if !found {
		return
	}
	d.shaftRes = res
	// Hold back the rest of the shaft's stairs from the rungs below, every
	// pass until it is dug (the next stair is promised by its own order).
	levels := from[2] - ore[2]
	defer func() { d.holdShaft(snap, levels) }()
	if snap.digging {
		return // one dig or stair order at a time
	}
	before := len(s.Tasks.GetTasks())
	switch {
	case from[2] == s.CZ:
		if !snap.afford(stairCost) || !s.Orders.Build(stairsDown, from[0], from[1], from[2]) {
			return
		}
		d.note("dig:stairs", fmt.Sprintf("stairs down at %d,%d,%d toward %s at %d,%d,%d", from[0], from[1], from[2], res, ore[0], ore[1], ore[2]), tick)
	case from[2] > ore[2]:
		next, ok := d.stepToward(s, from, ore, true)
		if !ok {
			d.avoid[ore] = true // no way down from here: another deposit
			return
		}
		if !snap.afford(stairCost) || !s.Orders.Build(stairsDown, next[0], next[1], next[2]) {
			return
		}
		d.note("dig:stairs", fmt.Sprintf("stairs down at %d,%d,%d toward %s", next[0], next[1], next[2], res), tick)
	default:
		levels = 0
		next, ok := d.stepToward(s, from, ore, false)
		if !ok {
			d.avoid[ore] = true
			return
		}
		if yields(s.Level, next[0], next[1], next[2]) != "" {
			if !s.Orders.Mine(next[0], next[1], next[2]) {
				d.avoid[next] = true
				return
			}
			d.note("dig:tunnel", fmt.Sprintf("tunnel (mine) at %d,%d,%d toward %s", next[0], next[1], next[2], res), tick)
		} else {
			if !s.Orders.Dig(next[0], next[1], next[2]) {
				d.avoid[next] = true
				return
			}
			d.note("dig:tunnel", fmt.Sprintf("tunnel at %d,%d,%d toward %s", next[0], next[1], next[2], res), tick)
		}
	}
	snap.digging = true
	d.watchNew(s, before, tick)
}

// stairSite reports a surface tile a Stairs Down may go on: reachable open
// ground with nothing on it or ordered on it, not the centre, not a stair,
// not a site it gave up on.
func (d *Director) stairSite(s *Site, snap *snapshot, p [3]int) bool {
	if d.avoid[p] || snap.taskAt[p] || snap.blocked[[2]int{p[0], p[1]}] || p[0] == s.CX && p[1] == s.CY {
		return false
	}
	if up, down := stairs(s.Level, p[0], p[1], p[2]); up || down {
		return false
	}
	ti := s.Level.GetTileAt(p[0], p[1], p[2])
	if ti == nil || !open(s.Level, p[0], p[1], p[2]) {
		return false
	}
	if s.Level.GetEntityAt(p[0], p[1], p[2]) != nil {
		return false
	}
	// Not on the build grid, where the ladder puts its structures.
	if (p[0]-s.CX)%2 == 0 && (p[1]-s.CY)%3 == 0 && cheb(p[0]-s.CX, p[1]-s.CY) <= 2*d.radius(s) {
		return false
	}
	return true
}

func (d *Director) radius(s *Site) int {
	if cfg := s.config(); cfg != nil {
		return cfg.Radius
	}
	return 0
}

// stepToward is the tile beside from (on its level) toward to: the longer
// axis first, then the other. For a stair (descend) it must not be a stair or
// have one below; for a tunnel it must be solid (something to dig). Out of
// bounds, given-up and ordered tiles are skipped.
func (d *Director) stepToward(s *Site, from, to [3]int, descend bool) ([3]int, bool) {
	dx, dy := sign(to[0]-from[0]), sign(to[1]-from[1])
	var steps [][2]int
	if abs(to[0]-from[0]) >= abs(to[1]-from[1]) {
		steps = [][2]int{{dx, 0}, {0, dy}}
	} else {
		steps = [][2]int{{0, dy}, {dx, 0}}
	}
	if descend {
		steps = append(steps, [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}...)
	}
	z := from[2]
	for _, st := range steps {
		if st == [2]int{0, 0} {
			continue
		}
		n := [3]int{from[0] + st[0], from[1] + st[1], z}
		if !s.Level.InBounds(n[0], n[1], z) || d.avoid[n] {
			continue
		}
		if up, down := stairs(s.Level, n[0], n[1], z); up || down {
			continue
		}
		if descend {
			if z-1 < 0 || !s.Level.InBounds(n[0], n[1], z-1) {
				continue
			}
			if up, down := stairs(s.Level, n[0], n[1], z-1); up || down {
				continue
			}
			if yields(s.Level, n[0], n[1], z-1) != "" {
				continue // the landing below would pave over ore
			}
			return n, true
		}
		ti := s.Level.GetTileAt(n[0], n[1], z)
		if ti != nil && ti.IsSolid() {
			return n, true
		}
	}
	return [3]int{}, false
}

func sign(a int) int {
	switch {
	case a > 0:
		return 1
	case a < 0:
		return -1
	}
	return 0
}
