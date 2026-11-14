package systems

import (
	"fmt"
	"os"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mechanical-basic/pkg/basic"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFindNearestWorkerRadiusInclusive is the regression test for
// find_nearest_worker(r) being one tile short east and south: the search box
// must reach exactly r tiles in every direction, and no further.
func TestFindNearestWorkerRadiusInclusive(t *testing.T) {
	const cx, cy, z = 10, 10, 3
	for _, r := range []int{1, 2, 5} {
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1}} {
			for _, dist := range []int{r, r + 1} {
				name := fmt.Sprintf("r%d_d%+d%+d_at%d", r, d[0], d[1], dist)
				t.Run(name, func(t *testing.T) {
					lvl := world.NewLevel(24, 24, 6)
					self := mob(lvl, "monsters", cx, cy, z)
					worker := mob(lvl, "colony", cx+d[0]*dist, cy+d[1]*dist, z)
					worker.AddComponent(&components.WorkerComponent{})
					ai := &components.ScriptedAIComponent{Vars: map[string]any{}}
					interp := basic.NewMechanicalBasic()
					registerScriptedAIFuncs(interp, self, lvl, ai)
					require.NoError(t, interp.Load(fmt.Sprintf(`
function probe_worker()
    return find_nearest_worker(%d)
endfunction
function probe_enemy()
    return find_nearest_enemy(%d)
endfunction
function probe_blueprint()
    return find_nearest_blueprint("target", %d)
endfunction
`, r, r, r)))
					worker.Blueprint = "target"
					want := 0.0
					if dist <= r {
						want = 1.0
					}
					assert.Equal(t, want, callNum(t, interp, "probe_worker"), "find_nearest_worker")
					assert.Equal(t, want, callNum(t, interp, "probe_enemy"), "find_nearest_enemy")
					assert.Equal(t, want, callNum(t, interp, "probe_blueprint"), "find_nearest_blueprint")
				})
			}
		}
	}
}

// TestFindNearestWorkerOneEast is the case from the todo list: r = 1 and a
// colonist one tile east.
func TestFindNearestWorkerOneEast(t *testing.T) {
	lvl := world.NewLevel(16, 16, 4)
	self := mob(lvl, "monsters", 5, 5, 1)
	w := mob(lvl, "colony", 6, 5, 1)
	w.AddComponent(&components.WorkerComponent{})
	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, self, lvl, &components.ScriptedAIComponent{Vars: map[string]any{}})
	require.NoError(t, interp.Load(`
function probe()
    return find_nearest_worker(1)
endfunction
function px()
    find_nearest_worker(1)
    return get_nearest_x()
endfunction
`))
	assert.Equal(t, 1.0, callNum(t, interp, "probe"))
	assert.Equal(t, 6.0, callNum(t, interp, "px"))
}

// TestFindNearestWorkerCacheRespectsRadius: a cached colonist who steps out of
// the radius is no longer found, so a touch-range hazard (spore pod, old
// munition) need not re-check the distance itself.
func TestFindNearestWorkerCacheRespectsRadius(t *testing.T) {
	lvl := world.NewLevel(16, 16, 4)
	self := mob(lvl, "monsters", 5, 5, 1)
	w := mob(lvl, "colony", 6, 6, 1)
	w.AddComponent(&components.WorkerComponent{})
	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, self, lvl, &components.ScriptedAIComponent{Vars: map[string]any{}})
	require.NoError(t, interp.Load(`
function probe()
    return find_nearest_worker(1)
endfunction
`))
	assert.Equal(t, 1.0, callNum(t, interp, "probe"), "adjacent colonist")
	lvl.PlaceEntity(8, 6, 1, w)
	assert.Equal(t, 0.0, callNum(t, interp, "probe"), "colonist stepped two tiles away")
}

// TestSporePodTouchRange runs the real spore_pod script: it bursts at a
// colonist on any of the eight tiles beside it, never at one two tiles off.
func TestSporePodTouchRange(t *testing.T) {
	script, err := os.ReadFile("../../data/scripts/ai/spore_pod.basic")
	require.NoError(t, err)
	for _, tc := range []struct {
		dx, dy int
		burst  bool
	}{
		{1, 0, true}, {-1, 0, true}, {0, 1, true}, {0, -1, true},
		{1, 1, true}, {-1, -1, true}, {2, 0, false}, {0, -2, false}, {2, 1, false},
	} {
		t.Run(fmt.Sprintf("%+d%+d", tc.dx, tc.dy), func(t *testing.T) {
			lvl := world.NewLevel(16, 16, 4)
			pod := mob(lvl, "monsters", 6, 6, 1)
			w := mob(lvl, "colony", 6+tc.dx, 6+tc.dy, 1)
			w.AddComponent(&components.WorkerComponent{})
			ai := &components.ScriptedAIComponent{Vars: map[string]any{}}
			interp := basic.NewMechanicalBasic()
			registerScriptedAIFuncs(interp, pod, lvl, ai)
			require.NoError(t, interp.Load(string(script)+`
function probe_cool()
    return get_var("cool") + 0
endfunction
`))
			_, err := interp.Call("on_turn")
			require.NoError(t, err)
			want := 0.0
			if tc.burst {
				want = 3.0
			}
			assert.Equal(t, want, callNum(t, interp, "probe_cool"), "cool after one turn")
		})
	}
}

// A wild hive creature never hunts the hive's own workers (a presence's
// drones): find_nearest_worker skips workers of the caller's faction and
// still finds a colonist behind them.
func TestFindNearestWorkerSkipsOwnFaction(t *testing.T) {
	lvl := world.NewLevel(16, 16, 4)
	self := mob(lvl, "hive", 5, 5, 1)
	drone := mob(lvl, "hive", 6, 5, 1)
	drone.AddComponent(&components.WorkerComponent{})
	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, self, lvl, &components.ScriptedAIComponent{Vars: map[string]any{}})
	require.NoError(t, interp.Load(`
function probe()
    return find_nearest_worker(4)
endfunction
function px()
    find_nearest_worker(4)
    return get_nearest_x()
endfunction
`))
	assert.Equal(t, 0.0, callNum(t, interp, "probe"), "the hive's own drone")
	w := mob(lvl, "colony", 8, 5, 1)
	w.AddComponent(&components.WorkerComponent{})
	assert.Equal(t, 0.0, callNum(t, interp, "probe"), "miss cooldown")
	for i := 0; i < 5; i++ {
		callNum(t, interp, "probe")
	}
	assert.Equal(t, 8.0, callNum(t, interp, "px"), "the colonist behind it")
}

// nearest_visible_enemy picks the same target every run when enemies are
// equally near: the Visible set is a map, so without a tie-break the pick
// followed its order, which changes from run to run. Four colonists one
// tile away on each side: the one with the lowest Y (north) wins, every
// time.
func TestNearestVisibleEnemyTieIsRepeatable(t *testing.T) {
	lvl := world.NewLevel(16, 16, 4)
	self := mob(lvl, "monsters", 5, 5, 1)
	vc := &components.VisionComponent{Range: 6, Visible: map[*ecs.Entity]components.VisibleEntry{}}
	self.AddComponent(vc)
	for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		w := mob(lvl, "colony", 5+d[0], 5+d[1], 1)
		w.AddComponent(&components.WorkerComponent{})
		vc.Visible[w] = components.VisibleEntry{Entity: w, X: 5 + d[0], Y: 5 + d[1], Z: 1}
	}
	interp := basic.NewMechanicalBasic()
	registerScriptedAIFuncs(interp, self, lvl, &components.ScriptedAIComponent{Vars: map[string]any{}})
	require.NoError(t, interp.Load(`
function sx()
    nearest_visible_enemy()
    return get_seen_x()
endfunction
function sy()
    nearest_visible_enemy()
    return get_seen_y()
endfunction
`))
	for i := 0; i < 50; i++ {
		require.Equal(t, 5.0, callNum(t, interp, "sx"), "call %d: x", i)
		require.Equal(t, 4.0, callNum(t, interp, "sy"), "call %d: y", i)
	}
}

// Two factions of one group never take each other for enemies; in
// different groups they do, whatever their doctrines (rival_selection_plan.md,
// "Groups"). The default (no relation set) is any two different factions.
func TestFindNearestEnemyRespectsGroups(t *testing.T) {
	t.Cleanup(func() { components.FactionsHostile = nil })
	probe := func() float64 {
		lvl := world.NewLevel(16, 16, 4)
		self := mob(lvl, "hive", 5, 5, 1)
		mob(lvl, "hive.2", 7, 5, 1)
		interp := basic.NewMechanicalBasic()
		registerScriptedAIFuncs(interp, self, lvl, &components.ScriptedAIComponent{Vars: map[string]any{}})
		require.NoError(t, interp.Load("function probe()\n    return find_nearest_enemy(6)\nendfunction\n"))
		return callNum(t, interp, "probe")
	}
	components.FactionsHostile = nil
	assert.Equal(t, 1.0, probe(), "default: two factions are enemies")
	components.FactionsHostile = func(a, b string) bool { return false }
	assert.Equal(t, 0.0, probe(), "one group: friends")
	components.FactionsHostile = func(a, b string) bool { return true }
	assert.Equal(t, 1.0, probe(), "two groups: enemies")
}
