package game

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func placed(t *testing.T, ms *MainState, blueprint string, x, y int) *ecs.Entity {
	t.Helper()
	e, err := factory.Create(blueprint, x, y, 0)
	require.NoError(t, err)
	ms.level.AddEntity(e)
	return e
}

func taskOf(t *testing.T, ms *MainState, action task.TaskAction) *task.Task {
	t.Helper()
	for _, tk := range ms.MainSettlement.Tasks.GetTasks() {
		if tk.Action == action {
			return tk
		}
	}
	return nil
}

// assertRebuilt checks got is the rebuilt copy of want: a different pointer that
// lives on the thawed level and carries the same ID.
func assertRebuilt(t *testing.T, ms *MainState, want string, got *ecs.Entity) {
	t.Helper()
	require.NotNil(t, got)
	assert.Equal(t, want, components.EnsureStableID(got))
	assert.True(t, slices.Contains(ms.level.Entities, got) || slices.Contains(ms.level.StaticEntities, got),
		"target isn't an entity on the thawed level")
}

func TestEntityBoundOrdersSurviveFreezeAndThaw(t *testing.T) {
	c := b0Campaign(t, "a")
	wm := NewWorldManager(c)
	ms := b0Planet(t, locationColonyName(c, c.Locations["a"]))

	enemy := placed(t, ms, "colonist", 5, 5)
	src := placed(t, ms, "storage_locker", 2, 2)
	dest := placed(t, ms, "storage_locker", 3, 3)
	ore := placed(t, ms, "metal_ore", 4, 4)
	gem := placed(t, ms, "metal_ore", 6, 6)
	enemyID, srcID, destID := components.EnsureStableID(enemy), components.EnsureStableID(src), components.EnsureStableID(dest)
	oreID, gemID := components.EnsureStableID(ore), components.EnsureStableID(gem)

	q := &ms.MainSettlement.Tasks
	q.AddTask(&task.Task{Action: task_requests.AttackAction, X: 5, Y: 5, Data: enemy})
	q.AddTask(&task.Task{Action: task_requests.PickupAction, X: 6, Y: 6, Data: gem})
	q.AddTask(&task.Task{Action: task_requests.RetrieveAction, X: 4, Y: 4, Data: task_requests.RetrieveRequest{Item: ore, Dest: dest}})
	q.AddTask(&task.Task{Action: task_requests.MineAction, X: 5, Y: 5, Data: task_requests.MineRequest{X: 5, Y: 5, Required: 7, Target: gem}})
	q.AddTask(&task.Task{Action: task_requests.RelocateAction, X: 2, Y: 2, Data: task_requests.RelocateRequest{
		Source: src, Blueprint: "metal_ore", Qty: 9, DestEntity: dest,
	}})

	require.NoError(t, wm.Freeze(c.Locations["a"], ms))
	thawed, err := wm.buildParked("a")
	require.NoError(t, err)
	require.Len(t, thawed.MainSettlement.Tasks.GetTasks(), 5)

	assertRebuilt(t, thawed, enemyID, taskOf(t, thawed, task_requests.AttackAction).Data.(*ecs.Entity))
	assertRebuilt(t, thawed, gemID, taskOf(t, thawed, task_requests.PickupAction).Data.(*ecs.Entity))

	ret := taskOf(t, thawed, task_requests.RetrieveAction).Data.(task_requests.RetrieveRequest)
	assertRebuilt(t, thawed, oreID, ret.Item)
	assertRebuilt(t, thawed, destID, ret.Dest)

	mine := taskOf(t, thawed, task_requests.MineAction).Data.(task_requests.MineRequest)
	assertRebuilt(t, thawed, gemID, mine.Target)
	assert.Equal(t, 7, mine.Required)

	rel := taskOf(t, thawed, task_requests.RelocateAction).Data.(task_requests.RelocateRequest)
	assertRebuilt(t, thawed, srcID, rel.Source)
	assertRebuilt(t, thawed, destID, rel.DestEntity)
	assert.Equal(t, "metal_ore", rel.Blueprint)
	assert.Equal(t, 9, rel.Qty)
}

func TestOrderWhoseTargetIsGoneIsDropped(t *testing.T) {
	c := b0Campaign(t, "a")
	wm := NewWorldManager(c)
	ms := b0Planet(t, locationColonyName(c, c.Locations["a"]))

	gone := placed(t, ms, "colonist", 5, 5)
	src := placed(t, ms, "storage_locker", 2, 2)
	kept := placed(t, ms, "colonist", 6, 6)
	keptID := components.EnsureStableID(kept)

	q := &ms.MainSettlement.Tasks
	q.AddTask(&task.Task{Action: task_requests.AttackAction, Data: gone})
	q.AddTask(&task.Task{Action: task_requests.RelocateAction, Data: task_requests.RelocateRequest{Source: src, DestEntity: gone}})
	q.AddTask(&task.Task{Action: task_requests.AttackAction, Data: kept})
	ms.level.RemoveEntity(gone)

	require.NoError(t, wm.Freeze(c.Locations["a"], ms))
	thawed, err := wm.buildParked("a")
	require.NoError(t, err)

	got := thawed.MainSettlement.Tasks.GetTasks()
	require.Len(t, got, 1)
	assertRebuilt(t, thawed, keptID, got[0].Data.(*ecs.Entity))
}

// A worker mid-relocate keeps what they withdrew in their bag, so the reloaded
// order only covers the rest.
func TestRelocateResumesWithWhatIsLeft(t *testing.T) {
	c := b0Campaign(t, "a")
	wm := NewWorldManager(c)
	ms := b0Planet(t, locationColonyName(c, c.Locations["a"]))
	src := placed(t, ms, "storage_locker", 2, 2)
	ms.MainSettlement.Tasks.AddTask(&task.Task{Action: task_requests.RelocateAction, Data: task_requests.RelocateRequest{
		Source: src, Blueprint: "metal_ore", Qty: 9, PickedUp: true, Carried: 4, DestX: 7,
	}})
	require.NoError(t, wm.Freeze(c.Locations["a"], ms))
	thawed, err := wm.buildParked("a")
	require.NoError(t, err)

	rel := taskOf(t, thawed, task_requests.RelocateAction).Data.(task_requests.RelocateRequest)
	assert.Equal(t, 5, rel.Qty)
	assert.False(t, rel.PickedUp)
	assert.Zero(t, rel.Carried)
	assert.Equal(t, 7, rel.DestX)
}

// A save from before stable IDs and persisted queues: no StableID components and
// "Tasks":{}. It loads, and every entity, including container contents, gets an ID.
func TestOldFormatSaveLoadsAndAssignsIDs(t *testing.T) {
	ms := b0Planet(t, "Old Save")
	locker := placed(t, ms, "storage_locker", 2, 2)
	ration, err := factory.Create("ration_pack", 0, 0, 0)
	require.NoError(t, err)
	require.True(t, locker.GetComponent(components.Storage).(*components.StorageComponent).AddItem(ration))

	raw, err := json.Marshal(world.SaveLevel(ms.level, ms.MainSettlement.Name))
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	stripIDs(doc)
	for _, s := range doc["Settlements"].(map[string]any) {
		s.(map[string]any)["Tasks"] = map[string]any{}
	}
	raw, err = json.Marshal(doc)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "StableID")

	var data world.SaveData
	require.NoError(t, json.Unmarshal(raw, &data))
	level := world.LoadSaveData(data)
	require.NotNil(t, level)

	seen := map[string]bool{}
	for _, ents := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range ents {
			require.True(t, e.HasComponent(components.StableID), e.Blueprint)
			id := components.EnsureStableID(e)
			assert.False(t, seen[id], "duplicate ID")
			seen[id] = true
			if e.HasComponent(components.Storage) {
				for _, item := range e.GetComponent(components.Storage).(*components.StorageComponent).Items {
					assert.True(t, item.HasComponent(components.StableID), item.Blueprint)
				}
			}
		}
	}
	assert.NotEmpty(t, seen)
}

func stripIDs(v any) {
	switch d := v.(type) {
	case map[string]any:
		delete(d, "StableID")
		for _, child := range d {
			stripIDs(child)
		}
	case []any:
		for _, child := range d {
			stripIDs(child)
		}
	}
}
