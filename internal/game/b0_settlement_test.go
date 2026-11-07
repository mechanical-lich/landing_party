package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/mapdef"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

func b0QueueBuild(ms *MainState, x, y int) *task.Task {
	tk := &task.Task{
		X: x, Y: y, Action: task_requests.BuildAction,
		Data: task_requests.BuildRequest{Type: "hull_wall", X: x, Y: y, Required: 5},
	}
	ms.MainSettlement.Tasks.AddTask(tk)
	return tk
}

func b0StepBoth(a, b *MainState) {
	fspath.ResetFrameCounter()
	a.stepWorld()
	b.stepBackground()
}

func b0TakeUntil(t *testing.T, a, b *MainState, worker *ecs.Entity, tk *task.Task) {
	t.Helper()
	for i := 0; i < 500; i++ {
		if b0CurrentTask(worker) == tk {
			return
		}
		b0StepBoth(a, b)
	}
	t.Fatalf("worker never picked up its own task; holds %+v", b0CurrentTask(worker))
}

// Workers on two loaded planets each work only their own planet's queue.
func TestWorkersOnlyTakeTheirOwnPlanetsTasks(t *testing.T) {
	useRealCosts(t)
	c := b0Campaign(t, "a", "b")
	installCampaignStorageHook(c)
	a := b0Planet(t, locationColonyName(c, c.Locations["a"]))
	b := b0Planet(t, locationColonyName(c, c.Locations["b"]))
	if a.MainSettlement == b.MainSettlement {
		t.Fatal("planets share a settlement")
	}
	workerA, workerB := b0Worker(t, a, 1, 1), b0Worker(t, b, 1, 1)
	b0Stock(t, a, 2, 2)
	b0Stock(t, b, 2, 2)

	taskA := b0QueueBuild(a, 8, 4)
	b0TakeUntil(t, a, b, workerA, taskA)
	if got := b0CurrentTask(workerB); got == taskA {
		t.Error("worker B took planet A's task")
	}

	taskB := b0QueueBuild(b, 8, 4)
	b0TakeUntil(t, a, b, workerB, taskB)
	if got := b0CurrentTask(workerA); got == taskB {
		t.Error("worker A took planet B's task")
	}
}

// Beaming down tags for the destination planet, and beaming up for the ship.
func TestBeamTagsColonistForDestinationPlanet(t *testing.T) {
	c := b0Campaign(t, "a", "b")
	wm := NewWorldManager(c)
	wm.SeedShipCrew(1)
	colonist := wm.ShipColonists()[0]
	settlementOf := func() string {
		return colonist.GetComponent(components.Settlement).(*components.SettlementComponent).Name
	}

	for _, id := range []string{"a", "b"} {
		c.CurrentLocationID = id
		wm.setCurrent(id, b0Planet(t, locationColonyName(c, c.Locations[id])))
		wm.landSet = false
		if err := wm.BeamDown(colonist); err != nil {
			t.Fatalf("beam down to %s: %v", id, err)
		}
		if got, want := settlementOf(), c.Locations[id].SettlementName; got != want {
			t.Errorf("on %s tagged %q, want %q", id, got, want)
		}
		if err := wm.BeamUp(colonist); err != nil {
			t.Fatalf("beam up from %s: %v", id, err)
		}
		if got, want := settlementOf(), wm.shipSettlementName(); got != want {
			t.Errorf("back aboard tagged %q, want %q", got, want)
		}
	}
}

// A location's level built from scratch records its settlement on the location
// and adopts it, and two locations in one campaign get different names.
func TestGeneratedLocationRecordsItsSettlement(t *testing.T) {
	if err := mapdef.Load("data/maps"); err != nil {
		t.Fatalf("load maps: %v", err)
	}
	if err := scenario.Load("data/scenarios"); err != nil {
		t.Fatalf("load scenarios: %v", err)
	}
	empty := t.TempDir()
	t.Cleanup(func() {
		_ = mapdef.Load(empty)
		_ = scenario.Load(empty)
	})
	c := b0Campaign(t, "a", "b")
	for id, loc := range c.Locations {
		loc.MapID, loc.ScenarioID, loc.Seed = "moon", "", 7
		loc.Name = "Loc " + id
	}
	wm := NewWorldManager(c)
	for _, id := range []string{"a", "b"} {
		ms, err := wm.buildParked(id)
		if err != nil {
			t.Fatalf("build %s: %v", id, err)
		}
		loc := c.Locations[id]
		if !strings.HasSuffix(loc.SettlementName, " @ Loc "+id) {
			t.Errorf("location %s settlement name = %q", id, loc.SettlementName)
		}
		if ms.MainSettlement == nil || ms.MainSettlement != settlement.Settlements[loc.SettlementName] {
			t.Errorf("location %s did not adopt its settlement", id)
		}
	}
	if c.Locations["a"].SettlementName == c.Locations["b"].SettlementName {
		t.Error("two locations got the same settlement name")
	}
}

// A campaign saved before per-location settlements is refused, not half-loaded.
func TestLoadRefusesCampaignWithoutLocationSettlements(t *testing.T) {
	b0Campaign(t, "a")
	root := campaignRoot("Old")
	writeOldCampaign(t, root, &campaign.Campaign{
		Version:   campaign.CampaignVersion,
		Name:      "Old",
		Locations: map[string]*campaign.Location{"a": {ID: "a", SaveFile: "loc_a.json.gz"}},
	})
	if _, err := LoadCampaign("Old"); err == nil {
		t.Error("old campaign loaded without error")
	}
	// A location never visited has no save to be out of date.
	writeOldCampaign(t, root, &campaign.Campaign{
		Version:   campaign.CampaignVersion,
		Name:      "Old",
		Locations: map[string]*campaign.Location{"a": {ID: "a"}},
	})
	if _, err := LoadCampaign("Old"); err != nil {
		t.Errorf("campaign with no frozen locations refused: %v", err)
	}
}

func writeOldCampaign(t *testing.T, root string, c *campaign.Campaign) {
	t.Helper()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeGz(filepath.Join(root, "campaign.json.gz"), c); err != nil {
		t.Fatal(err)
	}
}
