package director

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/construction"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/lore"
	"github.com/mechanical-lich/landing_party/internal/research"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

func TestMain(m *testing.M) {
	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	must(world.LoadTileDefinitionsDir("../../data/tiledefinitions"))
	must(factory.FactoryLoadDir("../../data/blueprints"))
	must(construction.Load("../../data/build.json"))
	must(research.Load("../../data/research.json"))
	must(crafting.Load("../../data/crafting_recipes.json"))
	campaign.FactionsPath = "../../data/factions.json"
	lore.NamesPath = "../../data/names.json"
	os.Exit(m.Run())
}

const colony = "test colony"

// fakeOrders records orders. With instant set, a build is finished on the
// spot (the structure appears) instead of queued, so a test can walk the
// ladder without workers; otherwise each order is an open task, as the
// game's order paths make.
type fakeOrders struct {
	s       *Site
	instant bool
	log     []string
}

func (o *fakeOrders) Build(kind string, x, y, z int) bool {
	o.log = append(o.log, "build:"+kind)
	if o.instant {
		e, err := factory.Create(kind, x, y, z)
		if err != nil {
			panic(err)
		}
		if e.HasComponent(components.Storage) {
			e.GetComponent(components.Storage).(*components.StorageComponent).OwnedBy = colony
		}
		o.s.Level.AddEntity(e)
		return true
	}
	o.s.Tasks.AddTask(&task.Task{Action: task_requests.BuildAction, X: x, Y: y, Z: z,
		Data: task_requests.BuildRequest{Type: kind, X: x, Y: y, Z: z}})
	return true
}

func (o *fakeOrders) Mine(x, y, z int) bool {
	o.log = append(o.log, "mine")
	o.s.Tasks.AddTask(&task.Task{Action: task_requests.MineAction, X: x, Y: y, Z: z,
		Data: task_requests.MineRequest{X: x, Y: y, Z: z, Required: 50}})
	return true
}

func (o *fakeOrders) Dig(x, y, z int) bool {
	o.log = append(o.log, "dig")
	o.s.Tasks.AddTask(&task.Task{Action: task_requests.DigAction, X: x, Y: y, Z: z,
		Data: task_requests.DigRequest{X: x, Y: y, Z: z, Required: 5}})
	return true
}

func (o *fakeOrders) Convert(x, y, z int, tile string) bool {
	o.log = append(o.log, "convert")
	o.s.Tasks.AddTask(&task.Task{Action: task_requests.ConvertAction, X: x, Y: y, Z: z,
		Data: task_requests.ConvertRequest{X: x, Y: y, Z: z, Tile: tile, Required: 8}})
	return true
}

func (o *fakeOrders) Retrieve(item *ecs.Entity) bool {
	o.log = append(o.log, "haul:"+item.Blueprint)
	x, y, z, _ := pos(item)
	o.s.Tasks.AddTask(&task.Task{Action: task_requests.RetrieveAction, X: x, Y: y, Z: z,
		Data: task_requests.RetrieveRequest{Item: item}})
	return true
}

func (o *fakeOrders) Research(tech string, station *ecs.Entity) bool {
	o.log = append(o.log, "research:"+tech)
	if o.instant {
		o.s.Known = append(o.s.Known, tech)
		return true
	}
	o.s.Tasks.AddTask(&task.Task{Action: task_requests.ResearchAction, Data: &task_requests.ResearchRequest{TechKey: tech}})
	return true
}

func (o *fakeOrders) Craft(recipe string, station *ecs.Entity) bool {
	o.log = append(o.log, "craft:"+recipe)
	o.s.Tasks.AddTask(&task.Task{Action: task_requests.CraftAction, Data: task_requests.CraftRequest{RecipeID: recipe}})
	return true
}

func (o *fakeOrders) Attack(target *ecs.Entity) bool {
	o.log = append(o.log, "attack:"+target.Blueprint)
	x, y, z, _ := pos(target)
	o.s.Tasks.AddTask(&task.Task{Action: task_requests.AttackAction, X: x, Y: y, Z: z, Data: target})
	return true
}

func (o *fakeOrders) Send(worker, target *ecs.Entity) *task.Task {
	o.log = append(o.log, "send:"+target.Blueprint)
	return &task.Task{Action: task_requests.AttackAction, Data: target}
}

func (o *fakeOrders) Equip(blueprint string, storage *ecs.Entity) bool {
	o.log = append(o.log, "equip:"+blueprint)
	return true
}

func (o *fakeOrders) Move(worker *ecs.Entity, x, y, z int) *task.Task {
	o.log = append(o.log, fmt.Sprintf("move:%d,%d", x, y))
	return &task.Task{Action: task_requests.MoveAction, X: x, Y: y, Z: z}
}

func (o *fakeOrders) Hold(worker *ecs.Entity, x, y, z int) *task.Task {
	o.log = append(o.log, fmt.Sprintf("move:%d,%d", x, y))
	return &task.Task{Action: task_requests.MoveAction, X: x, Y: y, Z: z, Data: task_requests.HoldHere}
}

func (o *fakeOrders) Deploy(worker *ecs.Entity, x, y, z int, as string, floor []string, ticks int) *task.Task {
	o.log = append(o.log, fmt.Sprintf("deploy:%s:%d,%d", as, x, y))
	t := &task.Task{Action: task_requests.DeployAction, X: x, Y: y, Z: z,
		Data: task_requests.DeployRequest{X: x, Y: y, Z: z, As: as, Floor: floor, Required: ticks}}
	t.Start() // as orderDeploy: handed straight to the worker
	return t
}

func (o *fakeOrders) Cancel(t *task.Task) {
	o.log = append(o.log, "cancel")
	t.Complete()
	o.s.Tasks.RemoveTask(t)
}

// testSite is a 41x41 open floor with the centre at (20, 20), n colonists
// beside it, a locker holding stock, and an ore deposit face to the east.
func testSite(t *testing.T, n int, stock map[string]int) (*Site, *fakeOrders) {
	t.Helper()
	lvl := world.NewLevel(41, 41, 1)
	lvl.AllocTerrain()
	for x := 0; x < 41; x++ {
		for y := 0; y < 41; y++ {
			lvl.SetFloor(x, y, 0, "hull_floor", 0)
		}
	}
	for y := 18; y <= 22; y++ {
		lvl.SetMiddle(36, y, 0, "ore_deposit", 0)
	}
	for i := 0; i < n; i++ {
		e, err := factory.Create("colonist", 19+i, 21, 0)
		if err != nil {
			t.Fatal(err)
		}
		e.AddComponent(&components.SettlementComponent{Name: colony})
		if !e.HasComponent(components.Worker) {
			e.AddComponent(&components.WorkerComponent{})
		}
		e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag = nil
		lvl.AddEntity(e)
	}
	locker, _ := factory.Create("storage_locker", 2, 2, 0)
	sc := locker.GetComponent(components.Storage).(*components.StorageComponent)
	sc.OwnedBy = colony
	for bp, q := range stock {
		it, err := factory.Create(bp, 0, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		it.GetComponent(components.Material).(*components.MaterialComponent).Quantity = q
		sc.AddItem(it)
	}
	lvl.AddEntity(locker)
	s := &Site{Level: lvl, Colony: colony, CX: 20, CY: 20, CZ: 0, Tasks: &task.TaskScheduler{},
		Doctrine: campaign.PlayerDoctrine(nil).Settlement}
	o := &fakeOrders{s: s}
	s.Orders = o
	return s, o
}

// The ladder in the colony's order: shelter first, then storage, a
// workbench, a lab, research once a lab stands (in tree order, skipping
// techs no colonist is clever enough for), defences, arms; build, research
// and craft share max_open (2) orders per plan. Mining waits while the stock
// covers the doctrine's targets.
func TestDirectorPriorityOrder(t *testing.T) {
	s, o := testSite(t, 2, map[string]int{"metal_ore": 50, "crystal": 20})
	o.instant = true
	d := New(1)
	for tick := 0; tick < 6; tick++ {
		d.Plan(s, tick*100)
	}
	want := []string{
		"build:bed", "build:storage_locker", // the locker in the corner is outside the radius
		"build:bed", "build:storage_locker",
		"build:basic_workbench", "build:research_lab",
		"research:basic_metallurgy", "build:gun_bench",
		// advanced_metallurgy needs Int 12, more than these colonists have.
		"research:combat_armor", "craft:stun_baton",
	}
	if len(o.log) < len(want) || !reflect.DeepEqual(o.log[:len(want)], want) {
		t.Errorf("orders\n got %v\nwant %v", o.log, want)
	}
	for _, l := range o.log {
		if l == "mine" {
			t.Error("mined with the stock already covering the targets")
		}
	}
}

// Short of stock, it mines the nearest deposit face, at most max_mines (2)
// orders at a time, and the builds still go in (the workers fetch).
func TestDirectorMinesWhenShort(t *testing.T) {
	s, o := testSite(t, 2, nil)
	d := New(1)
	d.Plan(s, 0)
	d.Plan(s, 100)
	mines := 0
	for _, tk := range s.Tasks.GetTasks() {
		if tk.Action == task_requests.MineAction {
			mines++
			if tk.X != 36 {
				t.Errorf("mining %d,%d, not the deposit face at x 36", tk.X, tk.Y)
			}
		}
	}
	if mines != 2 {
		t.Errorf("%d mine orders, want max_mines 2 (log %v)", mines, o.log)
	}
	if o.log[0] != "mine" && o.log[1] != "mine" {
		t.Errorf("materials before shelter in the table, got %v", o.log)
	}
}

// The director only gives orders: a plan changes no storage, makes or moves
// no entity, and its orders arrive with nothing delivered, so every unit a
// build uses is carried there by a worker.
func TestDirectorNeverMovesMaterials(t *testing.T) {
	s, _ := testSite(t, 3, map[string]int{"metal_ore": 30, "crystal": 10})
	stockOf := func() map[string]int {
		out := map[string]int{}
		for _, e := range s.Level.Entities {
			if e.HasComponent(components.Storage) {
				for _, it := range e.GetComponent(components.Storage).(*components.StorageComponent).Items {
					out[it.Blueprint] += units(it)
				}
			}
		}
		return out
	}
	before, n := stockOf(), len(s.Level.Entities)
	d := New(5)
	for tick := 0; tick < 5; tick++ {
		d.Plan(s, tick*50)
	}
	if !reflect.DeepEqual(stockOf(), before) || len(s.Level.Entities) != n {
		t.Errorf("a plan changed the level: stock %v -> %v, entities %d -> %d", before, stockOf(), n, len(s.Level.Entities))
	}
	if len(s.Tasks.GetTasks()) == 0 {
		t.Fatal("no orders")
	}
	for _, tk := range s.Tasks.GetTasks() {
		if br, ok := tk.Data.(task_requests.BuildRequest); ok && (len(br.Delivered) > 0 || br.Progress > 0) {
			t.Errorf("a build order arrived supplied: %+v", br)
		}
	}
}

// The same level and seed give the same orders at the same places.
func TestDirectorDeterministic(t *testing.T) {
	run := func(seed int64) []string {
		s, _ := testSite(t, 2, map[string]int{"metal_ore": 6})
		d := New(seed)
		for tick := 0; tick < 4; tick++ {
			d.Plan(s, tick*40)
		}
		var out []string
		for _, tk := range s.Tasks.GetTasks() {
			out = append(out, string(tk.Action)+"@"+itoa(tk.X)+","+itoa(tk.Y))
		}
		return out
	}
	a, b := run(7), run(7)
	if !reflect.DeepEqual(a, b) || len(a) == 0 {
		t.Errorf("same seed, different orders:\n%v\n%v", a, b)
	}
}

// An order that makes no progress for stuck_ticks is cancelled and its site
// not used again.
func TestDirectorCancelsStuckOrders(t *testing.T) {
	s, o := testSite(t, 1, map[string]int{"metal_ore": 50, "crystal": 20})
	d := New(1)
	d.Plan(s, 0)
	var bed *task.Task
	for _, tk := range s.Tasks.GetTasks() {
		if br, ok := tk.Data.(task_requests.BuildRequest); ok && br.Type == "bed" {
			bed = tk
		}
	}
	if bed == nil {
		t.Fatalf("no bed ordered: %v", o.log)
	}
	stuck := s.Doctrine.Director.StuckTicks
	d.Plan(s, stuck-1)
	if bed.Completed {
		t.Fatal("cancelled early")
	}
	d.Plan(s, stuck+1)
	if !bed.Completed || d.Cancelled == 0 {
		t.Fatal("stuck bed not cancelled")
	}
	d.Plan(s, stuck+2)
	for _, tk := range s.Tasks.GetTasks() {
		if br, ok := tk.Data.(task_requests.BuildRequest); ok && br.Type == "bed" && tk.X == bed.X && tk.Y == bed.Y {
			t.Error("re-ordered the bed on the site it gave up on")
		}
	}
}

// Loose food in reach is hauled in, nearest first, at most max_hauls (3).
func TestDirectorHaulsFood(t *testing.T) {
	s, o := testSite(t, 1, map[string]int{"metal_ore": 50, "crystal": 20})
	for i := 0; i < 5; i++ {
		e, _ := factory.Create("ration_pack", 25+i, 25, 0)
		s.Level.AddEntity(e)
	}
	d := New(1)
	d.Plan(s, 0)
	hauls := 0
	for _, l := range o.log {
		if l == "haul:ration_pack" {
			hauls++
		}
	}
	if hauls != 3 || o.log[0] != "haul:ration_pack" {
		t.Errorf("hauls %d, log %v", hauls, o.log)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
