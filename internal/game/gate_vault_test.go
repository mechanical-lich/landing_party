package game

import (
	"fmt"
	"os"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// vaultCheck builds the controls world of a galaxy of shape and size from
// seed and reports what a party meets there: the dormant controls (nil:
// missing), how far under the surface they are, whether a walk reaches
// them from the landing, and the guards within 16 tiles of them.
func vaultCheck(t *testing.T, seed int64, size, shape string) (controls *ecs.Entity, depth int, reached, fromLanding bool, guards []string) {
	t.Helper()
	c, wm, err := newExpeditionOpts("Vault", seed, false, campaign.GenOptions{Size: size, Shape: shape})
	if err != nil {
		t.Fatal(err)
	}
	ctl := c.ControlsLocation()
	ms, err := wm.buildParked(ctl.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ms.teardown)
	lvl := ms.level
	controls = gateControls(lvl)
	if controls == nil {
		return nil, 0, false, false, nil
	}
	pc := controls.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	cx, cy, cz := pc.GetX(), pc.GetY(), pc.GetZ()
	depth = lvl.SurfaceZ - cz
	// The vault's surface stair: the stairs_down highest up near the controls.
	ex, ey, ez := -1, -1, -1
	for z := lvl.Depth - 1; z > cz && ex < 0; z-- {
		for y := cy - 30; y <= cy+30 && ex < 0; y++ {
			for x := cx - 30; x <= cx+30; x++ {
				if ti := lvl.GetTileAt(x, y, z); ti != nil {
					if tt := ti.(*world.Tile); !tt.Middle.IsEmpty() && world.TileDefinitions[tt.Middle.Type].StairsDown {
						ex, ey, ez = x, y, z
						break
					}
				}
			}
		}
	}
	if ex >= 0 {
		reached = walkReaches(lvl, ex, ey, ez, cx, cy, cz)
		ax, ay, az := findStartingPlaza(lvl, lvl.SurfaceZ, 5)
		fromLanding = walkReaches(lvl, ax, ay, az, ex, ey, ez)
	}
	for _, e := range lvl.Entities {
		if e == nil || !e.HasComponent(rlcomponents.Position) || !e.HasComponent(rlcomponents.Health) {
			continue
		}
		p := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		if p.GetZ() == cz && abs(p.GetX()-cx) <= 16 && abs(p.GetY()-cy) <= 16 && len(e.Blueprint) > 7 && e.Blueprint[:7] == "mutant_" {
			guards = append(guards, e.Blueprint)
		}
	}
	return controls, depth, reached, fromLanding, guards
}

// The controls world's vault is completable: on every shape and size the
// dormant controls stand under the surface, a walk reaches them from the
// vault's entrance, and guards stand by them; and the entrance is a walk
// from the landing on at least 95% of sites (the rest land on a plateau too
// small for anything without stairs dug or built). Seeds 1-3 by default; LP_GATE_SWEEP=1
// runs seeds 1-20 (plan step 4: a blocked controls ruin makes the galaxy
// unwinnable).
func TestGateVaultCompletable(t *testing.T) {
	loadTutorialData(t)
	isolateSettlements(t)
	seeds := 3
	if os.Getenv("LP_GATE_SWEEP") != "" {
		seeds = 20
	}
	shapes := []string{"spiral", "barred_spiral", "elliptical", "lenticular", "ring", "irregular"}
	sizes := []string{"small", "medium", "large"}
	if testing.Short() {
		shapes, sizes, seeds = shapes[:1], sizes[:1], 1
	}
	depths := map[int]int{}
	guardCount := map[int]int{}
	walks, all := 0, 0
	for _, sh := range shapes {
		for _, sz := range sizes {
			for seed := int64(1); seed <= int64(seeds); seed++ {
				ctl, depth, reached, fromLanding, guards := vaultCheck(t, seed, sz, sh)
				where := fmt.Sprintf("%s/%s seed %d", sh, sz, seed)
				if ctl == nil {
					t.Errorf("%s: no gate controls on the controls world", where)
					continue
				}
				if depth < 2 || !reached || len(guards) < 3 {
					t.Errorf("%s: controls %d under the surface, reached %v, guards %v", where, depth, reached, guards)
				}
				depths[depth]++
				guardCount[len(guards)]++
				all++
				if fromLanding {
					walks++
				} else {
					t.Logf("%s: the vault's entrance is no walk from the landing", where)
				}
			}
		}
	}
	t.Logf("depths %v, guards beside the controls %v, entrance a walk from the landing on %d of %d", depths, guardCount, walks, all)
	if walks*100 < all*95 {
		t.Errorf("the vault's entrance is a walk from the landing on only %d of %d sites", walks, all)
	}
}

// walkReaches floods the level's standable tiles from (x, y, z), down and up
// paired stairs (entities do not block: a party fights through creatures and chops
// trees), and reports whether a tile beside (tx, ty, tz) is reached.
func walkReaches(lvl *world.Level, x, y, z, tx, ty, tz int) bool {
	type p3 struct{ x, y, z int }
	stand := func(q p3) bool {
		ti := lvl.GetTileAt(q.x, q.y, q.z)
		if ti == nil {
			return false
		}
		t := ti.(*world.Tile)
		if !t.Middle.IsEmpty() {
			def := world.TileDefinitions[t.Middle.Type]
			if def.StairsUp || def.StairsDown {
				return true
			}
			if def.Solid {
				return false
			}
		}
		return !t.Floor.IsEmpty()
	}
	stairs := func(q p3) (up, down bool) {
		t := lvl.GetTileAt(q.x, q.y, q.z).(*world.Tile)
		if t.Middle.IsEmpty() {
			return false, false
		}
		def := world.TileDefinitions[t.Middle.Type]
		return def.StairsUp, def.StairsDown
	}
	seen := map[p3]bool{{x, y, z}: true}
	queue := []p3{{x, y, z}}
	for len(queue) > 0 {
		q := queue[0]
		queue = queue[1:]
		if q.z == tz && abs(q.x-tx) <= 1 && abs(q.y-ty) <= 1 {
			return true
		}
		next := []p3{{q.x + 1, q.y, q.z}, {q.x - 1, q.y, q.z}, {q.x, q.y + 1, q.z}, {q.x, q.y - 1, q.z}}
		// The game's rule: a level changes only from a down stair onto an up
		// stair below it, or the other way.
		if up, down := stairs(q); up || down {
			if b := (p3{q.x, q.y, q.z - 1}); down && lvl.InBounds(b.x, b.y, b.z) {
				if bu, _ := stairs(b); bu {
					next = append(next, b)
				}
			}
			if a := (p3{q.x, q.y, q.z + 1}); up && lvl.InBounds(a.x, a.y, a.z) {
				if _, ad := stairs(a); ad {
					next = append(next, a)
				}
			}
		}
		for _, n := range next {
			if seen[n] || !lvl.InBounds(n.x, n.y, n.z) || !stand(n) {
				continue
			}
			seen[n] = true
			queue = append(queue, n)
		}
	}
	return false
}

// A landing party switches the gate on live: four colonists at the bottom of
// the vault, an ordinary repair order on the dormant controls is worked, the
// level flag is set and the campaign's gate comes on, by the player, for
// everyone; the controls are replaced by the lit machine and the gate is
// identified. (The guards' fight is the gate vmtest's: creature AI scripts
// are read from the repo root, which these tests do not run in.)
func TestGateSwitchedOnLive(t *testing.T) {
	loadTutorialData(t)
	isolateSettlements(t)
	useContentTables(t)
	useRealCosts(t)
	c, wm, err := newExpedition("GateLive", 3, false)
	if err != nil {
		t.Fatal(err)
	}
	ctl := c.ControlsLocation()
	if err := wm.Travel(ctl.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
			t.Fatal(err)
		}
	}
	ms, err := wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ms.teardown)
	e := gateControls(ms.level)
	if e == nil {
		t.Fatal("no gate controls on the controls world")
	}
	pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	// The party at the bottom of the vault, beside the controls (the walk
	// down is TestGateVaultCompletable's).
	crew := colonyWorkers(ms)
	for i, w := range crew {
		x, y, z := freeLandingTile(ms.level, pc.GetX()-1-i%2, pc.GetY()+1+i/2, pc.GetZ())
		w.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent).SetPosition(x, y, z)
	}
	ms.addRepairTask(e)
	on := -1
	stepLevel(ms, 9000, func(tick int) {
		if on < 0 && c.GateIsOn() {
			on = tick
		}
	})
	alive := len(colonyWorkers(ms))
	t.Logf("gate on at tick %d; colonists alive %d of %d", on, alive, len(crew))
	if on < 0 || !c.GateIsOn() || c.Galaxy.GateOnBy != campaign.GateByPlayer {
		t.Fatalf("the gate did not come on (by %q)", c.Galaxy.GateOnBy)
	}
	if gateControls(ms.level) != nil {
		t.Error("the dormant controls are still there")
	}
	if !c.GateLocation().Identified || alive == 0 {
		t.Errorf("gate identified %v, colonists alive %d", c.GateLocation().Identified, alive)
	}
}

// A rival colony's ship landed on the controls world switches the gate on
// live: once its camp stands on the loaded level, the world manager orders
// the controls repaired by its own settlement; its workers walk down and
// work it, and the gate comes on in its name, with an alert for the player.
func TestRivalSwitchesTheGateOnLive(t *testing.T) {
	loadTutorialData(t)
	isolateSettlements(t)
	useContentTables(t)
	useRealCosts(t)
	c, wm, err := newExpeditionOpts("GateRival", 3, false, campaign.GenOptions{Rivals: []campaign.RivalPick{{Doctrine: campaign.RivalColonyDoctrine}}})
	if err != nil {
		t.Fatal(err)
	}
	ctl := c.ControlsLocation()
	var f *campaign.Fleet
	for i := range c.Fleets {
		if c.ShipSeeksGate(&c.Fleets[i]) {
			f = &c.Fleets[i]
		}
	}
	if f == nil {
		t.Fatal("no rival colony ship")
	}
	// Off a loaded level its doctrine switches the gate on after its
	// switch_days; here only the live order may.
	g := campaign.LoadFactionData().Doctrine(f.Presence.Doctrine).Gate
	days := g.SwitchDays
	g.SwitchDays = 1 << 20
	t.Cleanup(func() { g.SwitchDays = days })
	// It has arrived on the controls world (the abstract route is the
	// campaign test's).
	f.LandedID, f.ToID, f.LandedDay = ctl.ID, ctl.ID, c.FactionDay
	f.FromX, f.FromY, f.ToX, f.ToY = ctl.X, ctl.Y, ctl.X, ctl.Y
	f.Known = true
	if err := wm.Travel(ctl.ID); err != nil {
		t.Fatal(err)
	}
	if err := wm.BeamDown(wm.ShipColonists()[0]); err != nil {
		t.Fatal(err)
	}
	ms, err := wm.EnterCurrent()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ms.teardown)
	// The vault's guards fight whoever comes down (the gate vmtest shows
	// it); creature AI scripts are not read in these tests, so here they
	// would stand in the doorways as walls. Take them off.
	for _, g := range append([]*ecs.Entity(nil), ms.level.Entities...) {
		if g != nil && len(g.Blueprint) > 7 && g.Blueprint[:7] == "mutant_" {
			ms.level.RemoveEntity(g)
		}
	}
	on := -1
	stepLevel(ms, 30*750, func(tick int) {
		if on < 0 && c.GateIsOn() {
			on = tick
		}
	})
	t.Logf("materialized %v; gate on at tick %d by %q; news %v", f.Presence.Materialized, on, c.Galaxy.GateOnBy, c.News)
	if !c.GateIsOn() || c.Galaxy.GateOnBy != f.Presence.Faction {
		t.Fatalf("the rival did not switch the gate on (on %v by %q)", c.GateIsOn(), c.Galaxy.GateOnBy)
	}
}
