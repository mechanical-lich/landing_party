package game

import (
	"reflect"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/systems"
	"github.com/mechanical-lich/mlge/ecs"
)

func bgSystemNames(m *ecs.SystemManager) map[string]bool {
	out := map[string]bool{}
	// The manager keeps its list unexported; reflection can still read the types.
	list := reflect.ValueOf(m).Elem().FieldByName("systems")
	for i := 0; i < list.Len(); i++ {
		t := list.Index(i).Elem().Type()
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		out[t.Name()] = true
	}
	return out
}

// stepBackground runs a parked level on the logic stack alone: needs drain, a
// worker takes its task, a hostile spawns from the level's own scenario, and no
// rendering system is registered.
func TestStepBackgroundAdvancesParkedLevel(t *testing.T) {
	useRealCosts(t)
	c := b0Campaign(t, "a")
	installCampaignStorageHook(c)
	ms := b0Planet(t, locationColonyName(c, c.Locations["a"]))
	ms.teardown() // parked
	ms.scenario = b0SpawnScenario("a", "mutant_grunt")

	names := bgSystemNames(ms.bgSystemManager)
	// FOVSystem stays on the background stack on purpose: findBed reads level.Visible.
	for _, render := range []any{systems.LightingSystem{}, systems.EmoteSystem{}} {
		name := reflect.TypeOf(render).Name()
		if names[name] {
			t.Errorf("%s registered on the background manager", name)
		}
		if !bgSystemNames(ms.systemManager)[name] {
			t.Errorf("%s missing from the live manager; this guard checks nothing", name)
		}
	}

	worker := b0Worker(t, ms, 1, 1)
	b0Stock(t, ms, 2, 2)
	needs := worker.GetComponent(components.Needs).(*components.NeedsComponent)
	build := b0QueueBuild(ms, 8, 4)

	tookTask := false
	startTick := ms.tick
	const ticks = 400
	for i := 0; i < ticks; i++ {
		fspath.ResetFrameCounter()
		ms.stepBackground()
		tookTask = tookTask || b0CurrentTask(worker) == build
	}
	if ms.tick != startTick+ticks || ms.level.Tick != uint64(ms.tick) {
		t.Errorf("tick = %d, level tick = %d, want %d", ms.tick, ms.level.Tick, startTick+ticks)
	}
	if needs.Hunger == 0 {
		t.Error("hunger did not drain")
	}
	if !tookTask {
		t.Error("worker never took its task")
	}
	if b0Spawned(ms)["mutant_grunt"] == 0 {
		t.Error("no hostile spawned from the level's scenario")
	}
}
