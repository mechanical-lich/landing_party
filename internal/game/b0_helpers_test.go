package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

const (
	b0Width  = 16
	b0Height = 8
)

// b0FloorLevel is an open hull_floor level, walkable and matchable by spawn rules.
func b0FloorLevel() *world.Level {
	lvl := world.NewLevel(b0Width, b0Height, 1)
	lvl.AllocTerrain()
	for x := 0; x < b0Width; x++ {
		for y := 0; y < b0Height; y++ {
			lvl.SetFloor(x, y, 0, "hull_floor", 0)
		}
	}
	return lvl
}

// b0Planet runs a hand-built level under a real MainState with its own settlement.
func b0Planet(t *testing.T, name string) *MainState {
	t.Helper()
	s := &settlement.Settlement{Name: name, CenterX: 1, CenterY: 1}
	settlement.Settlements[name] = s
	ms, err := newMainStateFromLevel(b0FloorLevel(), SettlementConfig{Name: name, ColonyName: name, CampaignMode: true})
	if err != nil {
		t.Fatalf("main state %s: %v", name, err)
	}
	if ms.MainSettlement != s {
		t.Fatalf("main state %s did not adopt its settlement", name)
	}
	t.Cleanup(ms.teardown)
	return ms
}

// b0Campaign is a campaign on an isolated settlement map and temp save dir.
func b0Campaign(t *testing.T, locIDs ...string) *campaign.Campaign {
	t.Helper()
	t.Chdir(t.TempDir())
	prev := settlement.Settlements
	settlement.Settlements = make(map[string]*settlement.Settlement)
	t.Cleanup(func() { settlement.Settlements = prev })
	c := &campaign.Campaign{Name: "B0", CurrentLocationID: locIDs[0], Locations: map[string]*campaign.Location{}}
	for _, id := range locIDs {
		c.Locations[id] = &campaign.Location{ID: id, Name: id}
	}
	return c
}

// b0Worker adds a real colonist tagged for the level's settlement.
func b0Worker(t *testing.T, ms *MainState, x, y int) *ecs.Entity {
	t.Helper()
	e, err := factory.Create("colonist", x, y, 0)
	if err != nil {
		t.Fatalf("create colonist: %v", err)
	}
	tagColonistSettlement(e, ms.MainSettlement.Name)
	ms.level.AddEntity(e)
	return e
}

func b0CurrentTask(e *ecs.Entity) *task.Task {
	return e.GetComponent(components.Worker).(*components.WorkerComponent).CurrentTask
}

// b0Stock places a locker of metal_ore owned by the level's settlement, so build
// tasks are affordable there.
func b0Stock(t *testing.T, ms *MainState, x, y int) {
	t.Helper()
	locker, err := factory.Create("storage_locker", x, y, 0)
	if err != nil {
		t.Fatalf("create locker: %v", err)
	}
	sc := locker.GetComponent(components.Storage).(*components.StorageComponent)
	sc.OwnedBy = ms.MainSettlement.Name
	ore, err := factory.Create("metal_ore", 0, 0, 0)
	if err != nil {
		t.Fatalf("create ore: %v", err)
	}
	ore.GetComponent(components.Material).(*components.MaterialComponent).Quantity = 50
	if !sc.AddItem(ore) {
		t.Fatal("locker refused ore")
	}
	ms.level.AddEntity(locker)
}
