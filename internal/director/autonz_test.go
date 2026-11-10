package director

import (
	"fmt"
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

const autonzColony = "autonz:test"

// autonzDirSite is a 41x41 floor of blue wire centred on (20, 20) with a
// fabricator, a locker of ore, crystal and ferrite, and the given workers by
// type; its table is the autonz' with only rungs (by id), at stage, knowing
// techs.
func autonzDirSite(t *testing.T, workers []string, stage int, techs []string, rungs ...string) (*Site, *fakeOrders) {
	t.Helper()
	d := campaign.LoadFactionData().Doctrine("autonz")
	if d == nil || d.Settlement == nil {
		t.Fatal("no autonz settlement block")
	}
	lvl := world.NewLevel(41, 41, 1)
	lvl.AllocTerrain()
	for x := 0; x < 41; x++ {
		for y := 0; y < 41; y++ {
			lvl.SetFloor(x, y, 0, "blue_wires", 0)
		}
	}
	for i, typ := range workers {
		dt := d.Types[typ]
		e, err := factory.Create(dt.Blueprint, 14+i, 23, 0)
		if err != nil {
			t.Fatal(err)
		}
		e.AddComponent(&components.SettlementComponent{Name: autonzColony})
		e.AddComponent(&components.WorkerComponent{Role: dt.Role})
		e.AddComponent(&components.KinComponent{Type: typ})
		lvl.AddEntity(e)
	}
	for _, st := range []struct {
		bp   string
		x, y int
	}{{"autonz_fabricator", 22, 22}, {"foundry", 18, 18}} {
		e, _ := factory.Create(st.bp, st.x, st.y, 0)
		lvl.AddEntity(e)
	}
	box, _ := factory.Create("storage_locker", 2, 2, 0)
	sc := box.GetComponent(components.Storage).(*components.StorageComponent)
	sc.OwnedBy = autonzColony
	for bp, q := range map[string]int{"metal_ore": 200, "crystal": 40, "ferrite": 40} {
		it, _ := factory.Create(bp, 0, 0, 0)
		it.GetComponent(components.Material).(*components.MaterialComponent).Quantity = q
		sc.AddItem(it)
	}
	lvl.AddEntity(box)
	ds := *d.Settlement
	dir := *ds.Director
	dir.Priorities = nil
	for _, p := range d.Settlement.Director.Priorities {
		for _, r := range rungs {
			if p.ID == r {
				dir.Priorities = append(dir.Priorities, p)
			}
		}
	}
	ds.Director = &dir
	s := &Site{Level: lvl, Colony: autonzColony, CX: 20, CY: 20, CZ: 0, Tasks: &task.TaskScheduler{},
		Doctrine: &ds, CrewCap: 20, Hostile: true, Enemy: colony, Aggression: d.Aggression, Known: techs,
		TypeMix: d.TypeMix(stage, nil, techs), Structures: d.Stages[stage].Structures, Stage: stage,
		ShotPop: d.Mobility.ShotPop, Economy: d.EconomyTypes(), Types: d.Types, Unlocked: d.TypesUnlocked(techs)}
	o := &fakeOrders{s: s}
	s.Orders = o
	return s, o
}

// Short of wire placers the placers rung prints one at the fabricator; at
// the crew cap it does not.
func TestAutonzPlacersFirst(t *testing.T) {
	s, o := autonzDirSite(t, []string{"enforcer", "enforcer"}, 0, nil, "placers")
	New(1).Plan(s, 0)
	if logHas(o, "craft:print_wire_placer") != 1 {
		t.Fatalf("orders %v, want a wire placer print", o.log)
	}
	s, o = autonzDirSite(t, []string{"enforcer", "enforcer"}, 0, nil, "placers")
	s.CrewCap = 2
	New(1).Plan(s, 0)
	if len(o.log) != 0 {
		t.Errorf("at the cap, still printed: %v", o.log)
	}
}

// The grow rung prints the unlocked type most under the stage's mix at its
// station: a hub knowing the foundry pattern prints a harvester at the
// foundry once placers and enforcers are at their share.
func TestAutonzGrowPicksTheMix(t *testing.T) {
	s, o := autonzDirSite(t, []string{"wire_placer", "wire_placer", "wire_placer", "wire_placer", "enforcer", "enforcer", "enforcer", "enforcer"},
		1, []string{"autonz_foundry_pattern"}, "grow")
	New(1).Plan(s, 0)
	if logHas(o, "craft:print_harvester") != 1 {
		t.Errorf("orders %v, want a harvester", o.log)
	}
}

// The hub's structures wait for the hub stage and their patterns.
func TestAutonzHubStructuresGated(t *testing.T) {
	s, o := autonzDirSite(t, []string{"wire_placer", "wire_placer"}, 0, []string{"autonz_foundry_pattern"}, "core", "foundry")
	s.Structures = map[string]int{}
	New(1).Plan(s, 0)
	if logHas(o, "build:") != 0 {
		t.Errorf("a node built %v", o.log)
	}
	s, o = autonzDirSite(t, []string{"wire_placer", "wire_placer"}, 1, nil, "core", "foundry", "converters")
	New(1).Plan(s, 0)
	if logHas(o, "build:signal_core") != 1 || logHas(o, "build:foundry") != 0 || logHas(o, "build:converter_node") != 0 {
		t.Errorf("hub without patterns: %v", o.log)
	}
}

// The relay rung prints a relay drone when none stands, then deploys an idle
// one on wire at the edge, toward the signal's bearing.
func TestAutonzRelayDeploys(t *testing.T) {
	techs := []string{"autonz_relay_pattern"}
	s, o := autonzDirSite(t, []string{"wire_placer"}, 1, techs, "relay")
	New(1).Plan(s, 0)
	if logHas(o, "craft:print_relay_drone") != 1 {
		t.Fatalf("orders %v, want a relay drone print", o.log)
	}
	s, o = autonzDirSite(t, []string{"wire_placer", "relay_drone"}, 1, techs, "relay")
	s.SignalDir = [2]float64{1, 0}
	New(1).Plan(s, 0)
	if logHas(o, "deploy:relay_pylon") != 1 {
		t.Fatalf("orders %v, want a deploy", o.log)
	}
	x := -1
	for _, l := range o.log {
		var y int
		if _, err := fmt.Sscanf(l, "deploy:relay_pylon:%d,%d", &x, &y); err == nil {
			break
		}
	}
	if x < 20+8 {
		t.Errorf("deployed at x %d, want east of the centre past the edge", x)
	}
	s, o = autonzDirSite(t, []string{"wire_placer", "relay_drone"}, 1, nil, "relay")
	New(1).Plan(s, 0)
	if len(o.log) != 0 {
		t.Errorf("without the relay pattern: %v", o.log)
	}
}

// Sentries hold one per defence pylon, beside it.
func TestAutonzSentriesPostAtPylons(t *testing.T) {
	s, o := autonzDirSite(t, []string{"sentinel", "sentinel"}, 1, []string{"autonz_sentinel_pattern", "autonz_foundry_pattern"}, "pylons")
	for _, xy := range [][2]int{{30, 20}, {10, 20}} {
		e, _ := factory.Create("defence_pylon", xy[0], xy[1], 0)
		s.Level.AddEntity(e)
	}
	New(1).Plan(s, 0)
	if logHas(o, "move:") != 2 {
		t.Errorf("orders %v, want two sentries sent to their pylons", o.log)
	}
}

// Transmit walks the placers (never an enforcer or a warden) to the Signal
// Core on a ready day only.
func TestAutonzTransmit(t *testing.T) {
	s, o := autonzDirSite(t, []string{"enforcer", "warden", "wire_placer", "wire_placer", "relay_drone"}, 1, nil, "transmit")
	core, _ := factory.Create("signal_core", 20, 19, 0)
	s.Level.AddEntity(core)
	New(1).Plan(s, 0)
	if len(o.log) != 0 {
		t.Fatalf("not a ready day: %v", o.log)
	}
	s.LaunchReady = true
	New(1).Plan(s, 0)
	if logHas(o, "move:") != 2 {
		t.Errorf("orders %v, want two bodies to the core", o.log)
	}
}

// Once wardens are known a raid forms only around one.
func TestAutonzRaidLeader(t *testing.T) {
	seen := func(s *Site) {
		if s.Level.Flags == nil {
			s.Level.Flags = map[string]any{}
		}
		s.Level.Flags[contactFlag(s.Colony)] = 1.0
		e, _ := factory.Create("colonist", 25, 25, 0)
		e.AddComponent(&components.SettlementComponent{Name: colony})
		e.AddComponent(&components.WorkerComponent{})
		s.Level.AddEntity(e)
	}
	crew := []string{"wire_placer", "wire_placer", "enforcer", "enforcer", "enforcer", "seeker"}
	s, o := autonzDirSite(t, crew, 1, []string{"autonz_foundry_pattern", "autonz_warden_pattern"}, "raid")
	seen(s)
	New(1).Plan(s, 0)
	if logHas(o, "send:") != 0 {
		t.Errorf("raided without a warden: %v", o.log)
	}
	s, o = autonzDirSite(t, append(crew, "warden"), 1, []string{"autonz_foundry_pattern", "autonz_warden_pattern"}, "raid")
	seen(s)
	New(1).Plan(s, 0)
	if logHas(o, "send:") == 0 {
		t.Errorf("no raid around the warden: %v", o.log)
	}
	s, o = autonzDirSite(t, crew, 1, nil, "raid")
	seen(s)
	New(1).Plan(s, 0)
	if logHas(o, "send:") == 0 {
		t.Errorf("wardens unknown: raids as before, got %v", o.log)
	}
}

var _ = task_requests.HoldHere
var _ = rlcomponents.Dead
var _ *ecs.Entity

// A deploy order that stops (its drone blocked, taken off it or dead) no
// longer counts against the rung: the next plan deploys again instead of
// waiting forever on an order that can never complete (review finding 7).
func TestAutonzRelayRungRecoversFromAStoppedDeploy(t *testing.T) {
	techs := []string{"autonz_relay_pattern"}
	s, o := autonzDirSite(t, []string{"wire_placer", "relay_drone"}, 1, techs, "relay")
	s.SignalDir = [2]float64{1, 0}
	d := New(1)
	d.Plan(s, 0)
	if logHas(o, "deploy:relay_pylon") != 1 || len(d.deploys) != 1 {
		t.Fatalf("orders %v, want a deploy", o.log)
	}
	d.deploys[0].ReQueue() // what a blocked move did to it
	o.log = nil
	for tick := 1; tick < 400 && logHas(o, "deploy:relay_pylon") == 0; tick++ {
		d.Plan(s, tick)
	}
	if logHas(o, "deploy:relay_pylon") == 0 {
		t.Errorf("the rung never deployed again after its order stopped: %v", o.log)
	}
}
