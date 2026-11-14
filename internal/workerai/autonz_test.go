package workerai

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/crafting"
	"github.com/mechanical-lich/landing_party/internal/factory"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deployOn runs a relay drone's deploy at (4, 1) of a wired corridor (the
// floor at the site set to floor) and reports the task.
func deployOn(t *testing.T, floor string) (*world.Level, *ecs.Entity, *task.Task) {
	t.Helper()
	colony(t)
	lvl := physicalLevel(t, 8, 3)
	lvl.SetFloor(4, 1, 0, floor, 0)
	e, wc, _ := physicalWorker(lvl, 1, 1)
	e.AddComponent(&components.KinComponent{Type: "relay_drone", Presence: "fp1"})
	tk := &task.Task{Action: task_requests.DeployAction, X: 4, Y: 1, InProgress: true,
		Data: task_requests.DeployRequest{X: 4, Y: 1, As: "relay_pylon", Floor: []string{"blue_wires", "red_wires"}, Required: 3}}
	wc.CurrentTask = tk
	t.Cleanup(fspath.ResetFrameCounter)
	for i := 0; i < 30 && !tk.Completed; i++ {
		fspath.ResetFrameCounter()
		HandleTaskState(lvl, e)
	}
	return lvl, e, tk
}

// A deploy walks the drone onto the wired tile, works there, and replaces
// it with the pylon on that tile, which carries its presence and settlement.
func TestDeploy_ReplacesUnit(t *testing.T) {
	lvl, drone, tk := deployOn(t, "blue_wires")
	require.True(t, tk.Completed)
	assert.False(t, onLevel(lvl, drone), "the drone is gone")
	var buf []*ecs.Entity
	lvl.GetEntitiesAt(4, 1, 0, &buf)
	require.Len(t, buf, 1)
	assert.Equal(t, "relay_pylon", buf[0].Blueprint)
	k := components.KinOf(buf[0])
	require.NotNil(t, k)
	assert.Equal(t, "fp1", k.Presence)
	assert.Equal(t, testColony, k.Settlement)
}

// Off wire the deploy is refused: the drone stays a drone.
func TestDeploy_FloorRequired(t *testing.T) {
	lvl, drone, tk := deployOn(t, "hull_floor")
	require.True(t, tk.Completed)
	assert.True(t, onLevel(lvl, drone), "the drone stays")
	var buf []*ecs.Entity
	lvl.GetEntitiesAt(4, 1, 0, &buf)
	for _, e := range buf {
		assert.NotEqual(t, "relay_pylon", e.Blueprint)
	}
}

// A placer within five tiles of a converter node needs half the work; one
// farther needs all of it.
func TestConvertWork_Boost(t *testing.T) {
	lvl := physicalLevel(t, 20, 3)
	assert.Equal(t, 8, ConvertWork(lvl, 10, 1, 0, 8), "no node")
	node, err := factory.Create("converter_node", 2, 1, 0)
	require.NoError(t, err)
	lvl.AddEntity(node)
	assert.Equal(t, 4, ConvertWork(lvl, 7, 1, 0, 8), "five tiles off: halved")
	assert.Equal(t, 8, ConvertWork(lvl, 8, 1, 0, 8), "six tiles off: whole")
}

// A typed print joins the crafter's settlement as a worker of its type with
// the crafter's presence (the autonz' fabricator).
func TestCraft_TypedPrintDeploys(t *testing.T) {
	prev := getRecipe
	getRecipe = func(string) (crafting.Recipe, bool) {
		return crafting.Recipe{Name: "Print Seeker", Output: "seeker_crew", KinType: "seeker", Station: "autonz_fabricator", DeployOnCraft: true}, true
	}
	t.Cleanup(func() { getRecipe = prev })
	colony(t)
	lvl := physicalLevel(t, 6, 3)
	e, wc, _ := physicalWorker(lvl, 2, 1)
	e.AddComponent(&components.KinComponent{Type: "wire_placer", Presence: "fp1"})
	tk := craftTask(3, 1, 1)
	wc.CurrentTask = tk
	for i := 0; i < 5 && !tk.Completed; i++ {
		HandleTaskState(lvl, e)
	}
	require.True(t, tk.Completed)
	var out *ecs.Entity
	for _, c := range lvl.Entities {
		if c.Blueprint == "seeker_crew" {
			out = c
		}
	}
	require.NotNil(t, out, "the seeker stands on the level")
	assert.True(t, components.IsColonyWorker(out, testColony))
	k := components.KinOf(out)
	require.NotNil(t, k)
	assert.Equal(t, "seeker", k.Type)
	assert.Equal(t, "fp1", k.Presence)
}

// A typed print takes its crafter's faction, not its blueprint's: the
// player's autonz print fights for the player (faction "player"), never for
// the rival autonz its blueprint names.
func TestCraft_TypedPrintTakesTheCraftersFaction(t *testing.T) {
	prev := getRecipe
	getRecipe = func(string) (crafting.Recipe, bool) {
		return crafting.Recipe{Name: "Print Seeker", Output: "seeker_crew", KinType: "seeker", Station: "autonz_fabricator", DeployOnCraft: true}, true
	}
	t.Cleanup(func() { getRecipe = prev })
	colony(t)
	lvl := physicalLevel(t, 6, 3)
	e, wc, _ := physicalWorker(lvl, 2, 1)
	e.AddComponent(&rlcomponents.DescriptionComponent{Name: "placer", Faction: "player"})
	e.AddComponent(&components.KinComponent{Type: "wire_placer", Presence: "fp1"})
	tk := craftTask(3, 1, 1)
	wc.CurrentTask = tk
	for i := 0; i < 5 && !tk.Completed; i++ {
		HandleTaskState(lvl, e)
	}
	require.True(t, tk.Completed)
	for _, c := range lvl.Entities {
		if c.Blueprint == "seeker_crew" {
			assert.Equal(t, "player", c.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction)
			return
		}
	}
	t.Fatal("no seeker printed")
}
