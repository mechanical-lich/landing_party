package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

func labSpec(t *testing.T, mapID, scenarioID string) PlanetLabSpec {
	t.Helper()
	if err := LoadPlanetLabData(); err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	t.Cleanup(func() {
		_ = mapdef.Load(empty)
		_ = scenario.Load(empty)
	})
	md := mapdef.ByID(mapID)
	if md == nil {
		t.Fatalf("no map %q", mapID)
	}
	spec := PlanetLabSpec{Map: md.Clone(), Width: 80, Height: 80, Depth: 10, Seed: 5}
	if scenarioID != "" {
		spec.Scenario = scenario.ByID(scenarioID)
		if spec.Scenario == nil {
			t.Fatalf("no scenario %q", scenarioID)
		}
	}
	return spec
}

func labWalkableTile(t *testing.T, lab *PlanetLab) (int, int, int) {
	t.Helper()
	lvl := lab.Level()
	z := lvl.SurfaceZ
	for y := 0; y < lvl.GetHeight(); y++ {
		for x := 0; x < lvl.GetWidth(); x++ {
			if lab.s.tileWalkable(x, y, z) && isStandable(lvl, x, y, z) {
				return x, y, z
			}
		}
	}
	t.Fatal("no walkable tile")
	return 0, 0, 0
}

func TestPlanetLabHonoursSpec(t *testing.T) {
	spec := labSpec(t, "earth_like", "earth_like")
	lab, err := NewPlanetLab(spec)
	if err != nil {
		t.Fatal(err)
	}
	defer lab.Close()
	lvl := lab.Level()
	if lvl.GetWidth() != 80 || lvl.GetHeight() != 80 || lvl.GetDepth() != 10 {
		t.Errorf("size = %dx%dx%d", lvl.GetWidth(), lvl.GetHeight(), lvl.GetDepth())
	}
	if !lvl.RevealAll {
		t.Error("fog of war is not off")
	}
	if len(lab.Colonists()) != 0 {
		t.Errorf("%d colonists on generation, want none until placed", len(lab.Colonists()))
	}
	if lab.Tick() != labSettleTicks {
		t.Errorf("tick = %d after generation", lab.Tick())
	}
}

func TestPlanetLabRunsWithoutScenario(t *testing.T) {
	spec := labSpec(t, "moon", "")
	lab, err := NewPlanetLab(spec)
	if err != nil {
		t.Fatal(err)
	}
	defer lab.Close()
	if lab.Level().LightMode == "" {
		t.Error("no lighting mode without a scenario, so the map renders black")
	}
	for i := 0; i < 50; i++ {
		lab.Step()
	}
}

func hostileCount(lvl *world.Level) int {
	n := 0
	for _, e := range lvl.Entities {
		if e.HasComponent(components.FactionAI) || e.HasComponent(components.ScriptedAI) {
			n++
		}
	}
	return n
}

// Fauna spawns from the scenario's rules unless the tool switches spawns off.
func TestPlanetLabSpawnsFollowSpec(t *testing.T) {
	spec := labSpec(t, "earth_like", "earth_like")
	lab, err := NewPlanetLab(spec)
	if err != nil {
		t.Fatal(err)
	}
	defer lab.Close()
	if hostileCount(lab.Level()) == 0 {
		t.Error("no fauna spawned")
	}

	sc := *spec.Scenario
	sc.SetupScripts = nil // setup scripts can place creatures of their own
	spec.Scenario = &sc
	spec.SuppressSpawns = true
	quiet, err := NewPlanetLab(spec)
	if err != nil {
		t.Fatal(err)
	}
	defer quiet.Close()
	for i := 0; i < 5; i++ {
		quiet.Step()
	}
	if n := hostileCount(quiet.Level()); n != 0 {
		t.Errorf("%d fauna with spawns suppressed", n)
	}
}

// Reveal is draw-only: a colonist still builds up real FOV, so AI under test sees
// what it would see in the game.
func TestPlanetLabRevealLeavesFOVAlone(t *testing.T) {
	spec := labSpec(t, "moon", "")
	lab, err := NewPlanetLab(spec)
	if err != nil {
		t.Fatal(err)
	}
	defer lab.Close()
	lvl := lab.Level()
	x, y, z := labWalkableTile(t, lab)
	if lvl.GetSeen(x, y, z) {
		t.Fatal("tile seen before any colonist exists")
	}
	if _, err := lab.PlaceColonist(x, y, z); err != nil {
		t.Fatal(err)
	}
	lab.Step()
	if !lvl.GetSeen(x, y, z) {
		t.Error("colonist's own tile not seen after an FOV pass")
	}
}

func TestPlaceColonistValidatesTile(t *testing.T) {
	spec := labSpec(t, "moon", "")
	lab, err := NewPlanetLab(spec)
	if err != nil {
		t.Fatal(err)
	}
	defer lab.Close()
	if _, err := lab.PlaceColonist(-1, -1, 0); err == nil {
		t.Error("placed out of bounds")
	}
	sky := lab.Level().GetDepth() - 1
	if _, err := lab.PlaceColonist(5, 5, sky); err == nil {
		t.Error("placed in open sky, with no floor")
	}
	x, y, z := labWalkableTile(t, lab)
	if _, err := lab.PlaceColonist(x, y, z); err != nil {
		t.Fatal(err)
	}
	if _, err := lab.PlaceColonist(x, y, z); err == nil {
		t.Error("placed on an occupied tile")
	}
	if len(lab.Colonists()) != 1 {
		t.Errorf("colonists = %d", len(lab.Colonists()))
	}
}

// A dummy colonist is a real worker: it takes settlement tasks, even on a map whose
// scenario set nothing up.
func TestDummyColonistRunsWorkerAI(t *testing.T) {
	spec := labSpec(t, "moon", "")
	lab, err := NewPlanetLab(spec)
	if err != nil {
		t.Fatal(err)
	}
	defer lab.Close()
	x, y, z := labWalkableTile(t, lab)
	e, err := lab.PlaceColonist(x, y, z)
	if err != nil {
		t.Fatal(err)
	}
	if lab.s.MainSettlement == nil {
		t.Fatal("no settlement to give tasks")
	}
	tk := &task.Task{
		X: x + 3, Y: y, Z: z, Action: task_requests.BuildAction,
		Data: task_requests.BuildRequest{Type: "hull_wall", X: x + 3, Y: y, Z: z, Required: 5},
	}
	lab.s.MainSettlement.Tasks.AddTask(tk)

	wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
	took := false
	for i := 0; i < 500 && !took; i++ {
		lab.Step()
		took = wc.CurrentTask == tk || tk.Completed
	}
	if !took {
		t.Error("colonist never took the queued task")
	}
}

func TestRemoveEntityTakesItOffTheLevel(t *testing.T) {
	spec := labSpec(t, "moon", "")
	lab, err := NewPlanetLab(spec)
	if err != nil {
		t.Fatal(err)
	}
	defer lab.Close()
	x, y, z := labWalkableTile(t, lab)
	var e *ecs.Entity
	if e, err = lab.PlaceColonist(x, y, z); err != nil {
		t.Fatal(err)
	}
	lab.RemoveEntity(e)
	if len(lab.Colonists()) != 0 || lab.Level().GetEntityAt(x, y, z) != nil {
		t.Error("colonist still on the level")
	}
}
