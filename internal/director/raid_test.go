package director

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

const enemy = "player colony"

// raidSite is testSite with a ladder of one raid rung, n faction workers
// (armed holding a stun baton) and a player colonist at (30, 20).
func raidSite(t *testing.T, n, armedN int, r campaign.DirectorRaid) (*Site, *fakeOrders, *ecs.Entity) {
	t.Helper()
	s, o := testSite(t, n, nil)
	ws := 0
	for _, e := range s.Level.Entities {
		if components.IsColonyWorker(e, colony) {
			inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
			inv.LeftHand, inv.RightHand = nil, nil // the blueprint's own gear
		}
		if components.IsColonyWorker(e, colony) && ws < armedN {
			b, err := factory.Create("stun_baton", 0, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).RightHand = b
			ws++
		}
	}
	victim, err := factory.Create("colonist", 30, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	victim.AddComponent(&components.SettlementComponent{Name: enemy})
	if !victim.HasComponent(components.Worker) {
		victim.AddComponent(&components.WorkerComponent{})
	}
	s.Level.AddEntity(victim)
	ds := *s.Doctrine
	dir := *ds.Director
	dir.Priorities = []campaign.DirectorPriority{{ID: "raid", Raid: &r}}
	ds.Director = &dir
	s.Doctrine = &ds
	s.Enemy, s.Aggression = enemy, 5
	return s, o, victim
}

func attacks(o *fakeOrders) int { return count(o, "attack:") }

// sent is the raid party members sent.
func sent(o *fakeOrders) int { return count(o, "send:") }

func count(o *fakeOrders, prefix string) int {
	n := 0
	for _, l := range o.log {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

var testRaid = campaign.DirectorRaid{Fighters: 2, Party: 3, CooldownTicks: 1000, MaxTicks: 500, Reach: 60}

// A raid needs a hostile settlement and enough armed workers: a neutral
// settlement never raids, a hostile one under the threshold arms itself and
// waits, and one at it sends its party against the player's nearest
// colonist.
func TestRaidOnlyWhenHostileAndAboveThreshold(t *testing.T) {
	s, o, victim := raidSite(t, 4, 2, testRaid)
	d := New(1)
	d.Plan(s, 0)
	if sent(o) != 0 {
		t.Fatalf("a neutral settlement raided: %v", o.log)
	}

	s, o, _ = raidSite(t, 4, 1, testRaid)
	s.Hostile = true
	d = New(1)
	d.Plan(s, 0)
	if sent(o) != 0 {
		t.Fatalf("one armed worker raided, the threshold is two: %v", o.log)
	}

	s, o, victim = raidSite(t, 4, 2, testRaid)
	s.Hostile = true
	d = New(1)
	d.Plan(s, 0)
	if sent(o) != 2 {
		t.Fatalf("hostile with two armed: %d sent, want a party of 2 (%v)", sent(o), o.log)
	}
	for _, tk := range d.party {
		if tk.Data != victim {
			t.Errorf("raid target %v, want the player's colonist", tk.Data)
		}
	}
	if d.Ordered["raids"] != 1 {
		t.Errorf("raids %d, want 1", d.Ordered["raids"])
	}
}

// Aggression scales the threshold: an aggressive faction raids with one
// fighter where a timid one waits for more.
func TestRaidThresholdScalesWithAggression(t *testing.T) {
	s, o, _ := raidSite(t, 4, 1, testRaid)
	s.Hostile, s.Aggression = true, 10
	New(1).Plan(s, 0)
	if sent(o) != 1 {
		t.Errorf("aggression 10, one armed: %d attacks, want 1", sent(o))
	}
	s, o, _ = raidSite(t, 4, 3, testRaid)
	s.Hostile, s.Aggression = true, 2
	New(1).Plan(s, 0)
	if sent(o) != 0 {
		t.Errorf("aggression 2 needs 5 fighters, raided with 3: %v", o.log)
	}
	if scaled(2, 2) != 5 || scaled(2, 8) != 2 || scaled(2, 10) != 1 {
		t.Errorf("scaled: %d %d %d", scaled(2, 2), scaled(2, 8), scaled(2, 10))
	}
}

// A raid out too long is called home (its orders cancelled), and the next
// waits out the cooldown.
func TestRaidReturnsAndCoolsDown(t *testing.T) {
	s, o, _ := raidSite(t, 4, 2, testRaid)
	s.Hostile = true
	d := New(1)
	d.Plan(s, 0)
	d.Plan(s, 499)
	if sent(o) != 2 {
		t.Fatalf("a second party went out while the first was out: %v", o.log)
	}
	party := append([]*task.Task(nil), d.party...)
	d.Plan(s, 500)
	for _, tk := range party {
		if !tk.Completed {
			t.Fatal("after max_ticks the party was not called home")
		}
	}
	if d.Raiding() {
		t.Fatal("still raiding after the call home")
	}
	d.Plan(s, 900)
	if sent(o) != 2 {
		t.Errorf("raided again inside the cooldown: %v", o.log)
	}
	d.Plan(s, 1000)
	if sent(o) != 4 {
		t.Errorf("no raid after the cooldown: %v", o.log)
	}
}

// Unarmed workers are armed from storage before a raid.
func TestRaidArmsFromStorage(t *testing.T) {
	s, o, _ := raidSite(t, 3, 0, testRaid)
	s.Hostile = true
	for _, e := range s.Level.Entities {
		if e.HasComponent(components.Storage) {
			b, _ := factory.Create("stun_baton", 0, 0, 0)
			e.GetComponent(components.Storage).(*components.StorageComponent).AddItem(b)
		}
	}
	New(1).Plan(s, 0)
	if sent(o) != 0 || len(o.log) == 0 || o.log[0] != "equip:stun_baton" {
		t.Errorf("orders %v, want an equip and no raid", o.log)
	}
}

// The hunt rung attacks the nearest food-dropping creature while food is
// short, and stops once enough is stored.
func TestHuntForFood(t *testing.T) {
	s, o := testSite(t, 2, nil)
	ds := *s.Doctrine
	dir := *ds.Director
	dir.Priorities = []campaign.DirectorPriority{{ID: "hunt", Hunt: "food", Min: 1, PerColonist: 0.5}}
	ds.Director = &dir
	s.Doctrine = &ds
	near, _ := factory.Create("critter", 25, 20, 0)
	far, _ := factory.Create("critter", 32, 25, 0)
	s.Level.AddEntity(far)
	s.Level.AddEntity(near)
	d := New(1)
	d.Plan(s, 0)
	if attacks(o) != 2 || s.Tasks.GetTasks()[0].Data != near {
		t.Fatalf("hunts %v, want both critters, the nearest first", o.log)
	}

	s, o = testSite(t, 2, nil)
	s.Doctrine = &ds
	for _, e := range s.Level.Entities {
		if e.HasComponent(components.Storage) {
			sc := e.GetComponent(components.Storage).(*components.StorageComponent)
			for i := 0; i < 2; i++ {
				r, _ := factory.Create("ration_pack", 0, 0, 0)
				sc.AddItem(r)
			}
		}
	}
	c, _ := factory.Create("critter", 25, 20, 0)
	s.Level.AddEntity(c)
	New(1).Plan(s, 0)
	if attacks(o) != 0 {
		t.Errorf("hunted with two rations stored (want 2): %v", o.log)
	}
}

// A raid always leaves someone at the camp.
func TestRaidLeavesSomeoneHome(t *testing.T) {
	s, o, _ := raidSite(t, 2, 2, testRaid)
	s.Hostile = true
	New(1).Plan(s, 0)
	if sent(o) != 1 {
		t.Errorf("two crew, both armed: %d sent, want 1 (one stays home)", sent(o))
	}
}
