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

// eggRecipe is a hive egg recipe: laid on a grey egg, in its place.
var eggRecipe = crafting.Recipe{Name: "Warrior Egg", Output: "xeno_egg", KinType: "warrior", Station: "hive_larva",
	ConsumeStation: true, DeployOnCraft: true, RequiresTech: "hive_warrior_brood"}

// layEgg runs a free, one-step egg craft at a larva beside a drone of
// presence fp1, its settlement knowing techs. It returns the level, the larva
// and whether the task completed.
func layEgg(t *testing.T, larva bool, techs ...string) (*world.Level, *ecs.Entity) {
	t.Helper()
	prev := getRecipe
	getRecipe = func(string) (crafting.Recipe, bool) { return eggRecipe, true }
	t.Cleanup(func() { getRecipe = prev })
	st := colony(t)
	for _, k := range techs {
		st.UnlockTech(k)
	}
	lvl := physicalLevel(t, 6, 3)
	var lv *ecs.Entity
	if larva {
		var err error
		lv, err = factory.Create("hive_larva", 3, 1, 0)
		require.NoError(t, err)
		lvl.AddEntity(lv)
	}
	e, wc, _ := physicalWorker(lvl, 2, 1)
	e.AddComponent(&components.KinComponent{Morph: "crawler", Type: "drone", Presence: "fp1"})
	tk := craftTask(3, 1, 1)
	wc.CurrentTask = tk
	for i := 0; i < 5 && !tk.Completed; i++ {
		HandleTaskState(lvl, e)
	}
	require.True(t, tk.Completed)
	return lvl, lv
}

// biomassStack is a biomass stack of qty with its material tags.
func biomassStack(t *testing.T, qty int) *ecs.Entity {
	t.Helper()
	e, err := factory.Create("biomass", 0, 0, 0)
	require.NoError(t, err)
	e.GetComponent(components.Material).(*components.MaterialComponent).Quantity = qty
	return e
}

func eggsOn(lvl *world.Level) []*ecs.Entity {
	var out []*ecs.Entity
	for _, list := range [][]*ecs.Entity{lvl.Entities, lvl.StaticEntities} {
		for _, e := range list {
			if e.Blueprint == "xeno_egg" {
				out = append(out, e)
			}
		}
	}
	return out
}

// An egg recipe uses its station up: the grey egg goes and the coloured egg
// stands on its tile, of the crafter's presence and settlement, typed.
func TestCraft_ConsumesStation(t *testing.T) {
	lvl, larva := layEgg(t, true, "hive_warrior_brood")
	assert.True(t, larva.HasComponent(rlcomponents.Dead) || !onLevel(lvl, larva), "the grey egg is used up")
	eggs := eggsOn(lvl)
	require.Len(t, eggs, 1)
	k := components.KinOf(eggs[0])
	assert.Equal(t, "warrior", k.Type)
	assert.Equal(t, "fp1", k.Presence)
	assert.Equal(t, testColony, k.Settlement)
	pc := eggs[0].GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	assert.Equal(t, [2]int{3, 1}, [2]int{pc.GetX(), pc.GetY()}, "on the larva's tile")
	assert.False(t, eggs[0].HasComponent(components.Worker), "an egg is not a worker")

	lvl, _ = layEgg(t, false, "hive_warrior_brood")
	assert.Empty(t, eggsOn(lvl), "no grey egg, no egg: nothing is made from nothing")
}

// A recipe's requires_tech is refused until the crafter's settlement knows
// it: the task ends with the larva untouched.
func TestCraft_RequiredTech(t *testing.T) {
	lvl, larva := layEgg(t, true)
	assert.Empty(t, eggsOn(lvl))
	assert.True(t, onLevel(lvl, larva) && !larva.HasComponent(rlcomponents.Dead), "the grey egg stays")
}

// A settlement's diet: its workers eat biomass a unit of a stack at a time,
// from the bag first, else from storage; a settlement without one does not.
func TestEat_Diet(t *testing.T) {
	prev := SettlementDiet
	SettlementDiet = func(name string) []string {
		if name == testColony {
			return []string{"biomass"}
		}
		return nil
	}
	t.Cleanup(func() { SettlementDiet = prev })
	lvl := physicalLevel(t, 8, 3)
	e, _, mem := physicalWorker(lvl, 2, 1, biomassStack(t, 3))
	nc := &components.NeedsComponent{Hunger: 80, MaxHunger: 100, HungerThreshold: 60}
	e.AddComponent(nc)
	assert.True(t, FoodAvailable(lvl, e))
	mem.State, mem.TargetX, mem.TargetY = "findfood", -1, -1
	HandleFindFood(lvl, e)
	assert.Equal(t, 50, nc.Hunger, "one unit feeds DietNutrition")
	assert.Equal(t, 2, bagCount(e, "biomass"), "a unit of the stack eaten")

	// From storage: the pool keeps the rest of its stack.
	e2, _, mem2 := physicalWorker(lvl, 5, 1)
	nc2 := &components.NeedsComponent{Hunger: 80, MaxHunger: 100, HungerThreshold: 60}
	e2.AddComponent(nc2)
	e2.AddComponent(&rlcomponents.HealthComponent{MaxHealth: 10, Health: 10})
	chest := physicalChest(lvl, 6, 1, biomassStack(t, 2))
	assert.True(t, FoodAvailable(lvl, e2))
	mem2.State, mem2.TargetX, mem2.TargetY = "findfood", -1, -1
	for i := 0; i < 10 && nc2.Hunger == 80; i++ {
		HandleFindFood(lvl, e2)
		if mem2.State != "findfood" {
			mem2.State = "findfood"
		}
	}
	assert.Equal(t, 50, nc2.Hunger)
	assert.Equal(t, 1, chest.CountResource("biomass"))

	SettlementDiet = nil
	e3, _, _ := physicalWorker(lvl, 1, 1, biomassStack(t, 3))
	assert.False(t, FoodAvailable(lvl, e3), "biomass is not food without the diet")
}

// A director's move (a guard's post) walks there and ends: it never picks up
// the worker standing on the tile, as the player's Move Here does a robot; a
// hold stays.
func TestMoveTaskIsAWalk(t *testing.T) {
	lvl := physicalLevel(t, 8, 3)
	e, wc, mem := physicalWorker(lvl, 1, 1)
	other, _, _ := physicalWorker(lvl, 4, 1)
	wc.CurrentTask = &task.Task{Action: task_requests.MoveAction, X: 4, Y: 1, InProgress: true}
	t.Cleanup(fspath.ResetFrameCounter) // the path budget is per frame
	for i := 0; i < 20 && wc.CurrentTask != nil; i++ {
		fspath.ResetFrameCounter()
		HandleTaskState(lvl, e)
	}
	assert.Nil(t, wc.CurrentTask, "the walk ended")
	assert.True(t, onLevel(lvl, other), "the worker on the tile was not picked up")
	assert.Equal(t, "idle", mem.State)
	hold := &task.Task{Action: task_requests.MoveAction, X: 3, Y: 1, InProgress: true, Data: task_requests.HoldHere}
	wc.CurrentTask, mem.State = hold, "task"
	for i := 0; i < 20; i++ {
		fspath.ResetFrameCounter()
		HandleTaskState(lvl, e)
	}
	assert.Equal(t, hold, wc.CurrentTask, "a hold stays")
	assert.False(t, hold.Completed)
}
