package director

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

const hiveColony = "hive:test"

// hiveDirSite is a 41x41 floor of resin with the heart's centre at (20, 20),
// a pool of biomass, larvae beside the centre, and the given workers (type
// "drone", "warrior", ...); its table is the hive's with only rungs (by id),
// or the whole of it with none.
func hiveDirSite(t *testing.T, workers []string, larvae int, biomass int, rungs ...string) (*Site, *fakeOrders) {
	t.Helper()
	d := campaign.LoadFactionData().Doctrine("hive")
	if d == nil || d.Settlement == nil {
		t.Fatal("no hive settlement block")
	}
	lvl := world.NewLevel(41, 41, 1)
	lvl.AllocTerrain()
	for x := 0; x < 41; x++ {
		for y := 0; y < 41; y++ {
			lvl.SetFloor(x, y, 0, "hive_resin", 0)
		}
	}
	for i, typ := range workers {
		ht := d.Types[typ]
		bp := map[string]string{"crawler": "xeno_crew", "flier": "troid_crew"}[ht.Morph]
		e, err := factory.Create(bp, 16+i, 23, 0)
		if err != nil {
			t.Fatal(err)
		}
		e.AddComponent(&components.SettlementComponent{Name: hiveColony})
		e.AddComponent(&components.WorkerComponent{Role: ht.Role})
		k := components.KinOf(e)
		k.Type, k.Settlement = typ, hiveColony
		lvl.AddEntity(e)
	}
	for i := 0; i < larvae; i++ {
		e, _ := factory.Create("hive_larva", 18+i, 19, 0)
		components.KinOf(e).Settlement = hiveColony
		lvl.AddEntity(e)
	}
	pool, _ := factory.Create("biomass_pool", 2, 2, 0)
	sc := pool.GetComponent(components.Storage).(*components.StorageComponent)
	sc.OwnedBy = hiveColony
	if biomass > 0 {
		it, _ := factory.Create("biomass", 0, 0, 0)
		it.GetComponent(components.Material).(*components.MaterialComponent).Quantity = biomass
		sc.AddItem(it)
	}
	lvl.AddEntity(pool)
	ds := *d.Settlement
	dir := *ds.Director
	if len(rungs) > 0 {
		dir.Priorities = nil
		for _, p := range d.Settlement.Director.Priorities {
			for _, r := range rungs {
				if p.ID == r {
					dir.Priorities = append(dir.Priorities, p)
				}
			}
		}
	}
	ds.Director = &dir
	s := &Site{Level: lvl, Colony: hiveColony, CX: 20, CY: 20, CZ: 0, Tasks: &task.TaskScheduler{},
		Doctrine: &ds, CrewCap: 20, Hostile: true, Enemy: colony, Aggression: d.Aggression,
		TypeMix: d.TypeMix(1, nil, nil), Structures: d.Stages[1].Structures, Stage: 1,
		Launcher: d.Level.Launcher, ShotPop: d.Mobility.ShotPop, Economy: d.EconomyTypes()}
	o := &fakeOrders{s: s}
	s.Orders = o
	return s, o
}

func logHas(o *fakeOrders, prefix string) int {
	n := 0
	for _, l := range o.log {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

// Short of drones, the drone rung lays a drone egg on a free larva; with
// the drones at min + share, it does not; with no larva free, nothing.
func TestHiveDronesFirst(t *testing.T) {
	s, o := hiveDirSite(t, []string{"drone", "warrior", "warrior"}, 2, 20, "drones")
	New(1).Plan(s, 0)
	if logHas(o, "craft:hive_egg_drone") != 1 {
		t.Fatalf("orders %v, want one drone egg", o.log)
	}
	s, o = hiveDirSite(t, []string{"drone", "drone", "drone", "drone"}, 2, 20, "drones")
	New(1).Plan(s, 0)
	if len(o.log) != 0 {
		t.Errorf("drones enough (4 of want 2 + 0.35 x 4): %v", o.log)
	}
	s, o = hiveDirSite(t, []string{"warrior"}, 0, 20, "drones")
	New(1).Plan(s, 0)
	if len(o.log) != 0 {
		t.Errorf("no larva, still ordered: %v", o.log)
	}
}

// The brood rung lays the egg of the type most under the stage's mix, among
// the unlocked: warriors locked, the nest's mix is drones and scouts.
func TestHiveBroodFollowsTheMix(t *testing.T) {
	s, o := hiveDirSite(t, []string{"drone", "drone", "drone", "drone"}, 1, 20, "brood")
	New(1).Plan(s, 0)
	if logHas(o, "craft:hive_egg_scout") != 1 {
		t.Fatalf("orders %v, want a scout egg (4 drones, no scouts)", o.log)
	}
	d := campaign.LoadFactionData().Doctrine("hive")
	s, o = hiveDirSite(t, []string{"drone", "drone", "drone", "scout"}, 1, 20, "brood")
	s.Known = []string{"hive_warrior_brood"}
	s.TypeMix = d.TypeMix(1, nil, s.Known)
	New(1).Plan(s, 0)
	if logHas(o, "craft:hive_egg_warrior") != 1 {
		t.Errorf("orders %v, want a warrior egg once the brood is known", o.log)
	}
}

// stage_min gates the chamber and the spire; the stage's structures cap the
// clusters.
func TestHiveStageMin(t *testing.T) {
	s, o := hiveDirSite(t, []string{"drone", "drone"}, 0, 100, "chamber", "spire")
	o.instant = true
	s.Stage = 0
	New(1).Plan(s, 0)
	if len(o.log) != 0 {
		t.Errorf("a brood built %v", o.log)
	}
	s.Stage = 1
	New(1).Plan(s, 0)
	if logHas(o, "build:evolution_chamber") != 1 || logHas(o, "build:egg_launcher") != 0 {
		t.Errorf("a nest built %v: want the chamber, not the spire", o.log)
	}
	s.Stage = 2
	New(1).Plan(s, 100)
	if logHas(o, "build:egg_launcher") != 1 {
		t.Errorf("a hive built %v: want the spire", o.log)
	}
	workers := make([]string, 40)
	for i := range workers {
		workers[i] = "drone"
	}
	s, o = hiveDirSite(t, workers, 0, 200, "clusters")
	o.instant = true
	for tick := 0; tick < 5; tick++ {
		New(1).Plan(s, tick*100)
	}
	if n := logHas(o, "build:egg_cluster"); n != s.Structures["egg_cluster"] {
		t.Errorf("%d clusters for 40 bodies, the nest's cap is %d", n, s.Structures["egg_cluster"])
	}
}

// The guard rung keeps warriors at the share and sets every guard on a
// hostile inside the radius; with none, idle guards walk to their posts.
func TestHiveGuard(t *testing.T) {
	s, o := hiveDirSite(t, []string{"drone", "warrior", "warrior"}, 0, 0, "guard")
	New(1).Plan(s, 0)
	if logHas(o, "move:") != 2 {
		t.Errorf("orders %v: want both guards to their posts", o.log)
	}
	s, o = hiveDirSite(t, []string{"drone", "warrior", "warrior"}, 0, 0, "guard")
	c, _ := factory.Create("colonist", 26, 20, 0)
	c.AddComponent(&components.SettlementComponent{Name: colony})
	c.AddComponent(&components.WorkerComponent{})
	s.Level.AddEntity(c)
	New(1).Plan(s, 0)
	if logHas(o, "send:colonist") != 2 {
		t.Errorf("orders %v: want both guards on the colonist inside the radius", o.log)
	}
	s.Level.PlaceEntity(39, 39, 0, c)
	o.log = nil
	New(1).Plan(s, 30)
	if logHas(o, "send:") != 0 {
		t.Errorf("a colonist beyond the radius drew %v", o.log)
	}
}

// The launch rung sends the shot's bodies, drones first, to the spire only
// on a day the launcher would fire.
func TestHiveLaunchWalksToTheSpire(t *testing.T) {
	s, o := hiveDirSite(t, []string{"warrior", "drone", "scout", "drone", "drone"}, 0, 0, "launch")
	spire, _ := factory.Create("egg_launcher", 24, 24, 0)
	s.Level.AddEntity(spire)
	New(1).Plan(s, 0)
	if len(o.log) != 0 {
		t.Errorf("not ready, still walked: %v", o.log)
	}
	s.LaunchReady = true
	New(1).Plan(s, 30)
	if logHas(o, "move:") != s.ShotPop {
		t.Fatalf("orders %v: want %d to the spire", o.log, s.ShotPop)
	}
	for _, l := range o.log {
		var x, y int
		if _, err := fmtSscanf(l, &x, &y); err == nil && (absI(x-24) > 3 || absI(y-24) > 3) {
			t.Errorf("%s: not within 3 of the spire", l)
		}
	}
}

// A patrolling scout that sees the player's colonist records contact; the
// raid waits for it.
func TestHivePatrolContactAndRaid(t *testing.T) {
	s, o := hiveDirSite(t, []string{"scout", "warrior", "warrior", "warrior", "warrior", "warrior", "drone", "drone"}, 0, 0, "scouts", "raid")
	c, _ := factory.Create("colonist", 36, 36, 0)
	c.AddComponent(&components.SettlementComponent{Name: colony})
	c.AddComponent(&components.WorkerComponent{})
	s.Level.AddEntity(c)
	d := New(1)
	d.Plan(s, 0)
	if Contact(s.Level, hiveColony) || logHas(o, "send:") > 0 {
		t.Fatalf("contact %v, raid %v before a scout saw anything", Contact(s.Level, hiveColony), o.log)
	}
	if logHas(o, "move:") == 0 {
		t.Error("the scout was not sent on patrol")
	}
	var scout *ecs.Entity
	for _, e := range s.Level.Entities {
		if components.KinOf(e) != nil && components.KinOf(e).Type == "scout" {
			scout = e
		}
	}
	s.Level.PlaceEntity(32, 34, 0, scout)
	d.Plan(s, 6000)
	if !Contact(s.Level, hiveColony) {
		t.Fatal("a scout 4 tiles from the colonist saw nothing")
	}
	if logHas(o, "send:colonist") == 0 {
		t.Errorf("orders %v: no raid after contact", o.log)
	}
}

// Excavate digs only when the resin rung has no floor left to lay on.
func TestHiveExcavate(t *testing.T) {
	s, o := hiveDirSite(t, []string{"drone", "burrower"}, 0, 0, "resin", "excavate")
	New(1).Plan(s, 0)
	if logHas(o, "dig") != 0 {
		t.Errorf("dug with resin to spread: %v", o.log)
	}
	// Every floor already resin, and rock round a small hollow.
	s, o = hiveDirSite(t, []string{"drone", "burrower"}, 0, 0, "resin", "excavate")
	for x := 0; x < 41; x++ {
		for y := 0; y < 41; y++ {
			if absI(x-20) > 3 || absI(y-20) > 3 {
				s.Level.SetMiddle(x, y, 0, "dungeon_wall", 0)
			}
		}
	}
	for _, e := range s.Level.Entities {
		if e.HasComponent(components.Worker) {
			s.Level.PlaceEntity(20, 21, 0, e)
		}
	}
	New(1).Plan(s, 0)
	if logHas(o, "dig") == 0 {
		t.Errorf("orders %v: no dig with nothing left to lay resin on", o.log)
	}
}

// The hive hunts any creature not of its own faction, by its biomass
// stock; its own wild kin are never prey.
func TestHiveHuntsForBiomass(t *testing.T) {
	s, o := hiveDirSite(t, []string{"hunter", "drone"}, 0, 0, "hunt")
	critter, err := factory.Create("xeno_stage2", 25, 20, 0) // wild kin: same faction
	if err != nil {
		t.Fatal(err)
	}
	s.Level.AddEntity(critter)
	New(1).Plan(s, 0)
	if logHas(o, "attack:") != 0 {
		t.Fatalf("hunted its own kind: %v", o.log)
	}
	yeti, err := factory.Create("yeti", 26, 20, 0)
	if err != nil {
		t.Skip("no yeti blueprint")
	}
	s.Level.AddEntity(yeti)
	New(1).Plan(s, 30)
	if logHas(o, "attack:yeti") != 1 {
		t.Errorf("orders %v: want the yeti hunted", o.log)
	}
}

// keep the task_requests and rlcomponents imports honest for helpers.
var _ = task_requests.MoveAction
var _ = rlcomponents.Dead

func absI(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// fmtSscanf reads a "move:x,y" log line.
func fmtSscanf(l string, x, y *int) (int, error) {
	return fmt.Sscanf(l, "move:%d,%d", x, y)
}
