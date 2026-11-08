package game

import (
	"encoding/json"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/objective"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// A station's dead generator is repaired as a haul-and-build: the Repair
// order (a click on it) queues a build of station_generator at its tile, the
// colonist carries machine parts and fuel there from the colony's locker,
// and on completion the generator runs, power:main is set, the powered light
// comes on and the powered door unlocks, and a station_power quest's
// objective holds.
func TestStationGeneratorRepair(t *testing.T) {
	useRealCosts(t)
	c := newE2EColony(t)
	c.addChest(t, 3, 1, map[string]int{"machine_parts": 6, "fuel": 8})
	gen, err := factory.Create("station_generator_broken", 20, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	c.ms.level.AddEntity(gen)
	light, _ := factory.Create("station_light", 10, 0, 0)
	c.ms.level.AddEntity(light)
	door, _ := factory.Create("airlock", 25, 2, 0)
	door.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent).Locked = true
	door.AddComponent(&components.PoweredComponent{Grid: "main", Unlock: true})
	c.ms.level.AddEntity(door)

	c.ms.addRepairTask(gen)
	c.ms.addRepairTask(gen) // a second click queues nothing more
	n := 0
	for _, tk := range c.ms.MainSettlement.Tasks.GetTasks() {
		if br, ok := tk.Data.(task_requests.BuildRequest); ok && br.Replace && br.Type == "station_generator" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d repair tasks queued, want 1", n)
	}
	c.tickUntil(t, "generator repaired", func() bool { return c.ms.level.Flags["power:main"] == 1.0 })
	c.tickUntil(t, "grid up", func() bool { return !light.HasComponent(components.Powered) })
	var running, broken int
	for _, list := range [][]*ecs.Entity{c.ms.level.Entities, c.ms.level.StaticEntities} {
		for _, e := range list {
			switch e.Blueprint {
			case "station_generator":
				running++
			case "station_generator_broken":
				broken++
			}
		}
	}
	if running != 1 || broken != 0 {
		t.Errorf("after the repair: %d running, %d broken generators", running, broken)
	}
	if lc := light.GetComponent(rlcomponents.Light).(*rlcomponents.LightComponent); lc.Level == 0 {
		t.Error("the station light is still dark")
	}
	if door.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent).Locked {
		t.Error("the powered door is still locked")
	}
	ctx := c.ms.buildEvalContext()
	if ctx.StructuresBuilt["station_generator"] != 1 {
		t.Errorf("structure_built sees %d station_generator", ctx.StructuresBuilt["station_generator"])
	}
	rules := objective.RuleSet{Rules: []objective.Rule{{ID: "station_power", Trigger: objective.TriggerFlagSet, Flag: "power:main"}}}
	if _, done := objective.New(rules).Evaluate(ctx); !done {
		t.Error("the station_power objective does not hold with the generator running")
	}
}

// A sealed door's lock and a powered door's hold survive a save and load,
// and so does the grid's flag.
func TestStationLocksSurviveSave(t *testing.T) {
	loreLoadData(t)
	lvl := world.NewLevel(10, 10, 1)
	sealed, _ := factory.Create("blast_door", 2, 2, 0)
	sd := sealed.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent)
	sd.Locked, sd.KeyId = true, "station_keycard"
	lvl.AddEntity(sealed)
	light, _ := factory.Create("station_light", 4, 4, 0)
	lvl.AddEntity(light)
	lvl.Modules = []world.Module{{ID: "archive", Kind: "archive", X: 1, Y: 1, W: 5, H: 5, Sealed: "keycard"}}
	lvl.Flags["power:main"] = 1.0
	b, err := json.Marshal(world.SaveLevel(lvl))
	if err != nil {
		t.Fatal(err)
	}
	var data world.SaveData
	if err := json.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	back := world.LoadSaveData(data)
	var door, lamp *ecs.Entity
	for _, list := range [][]*ecs.Entity{back.Entities, back.StaticEntities} {
		for _, e := range list {
			switch e.Blueprint {
			case "blast_door":
				door = e
			case "station_light":
				lamp = e
			}
		}
	}
	if door == nil || lamp == nil {
		t.Fatal("entities lost in the save")
	}
	if d := door.GetComponent(rlcomponents.Door).(*rlcomponents.DoorComponent); !d.Locked || d.KeyId != "station_keycard" {
		t.Errorf("sealed door after load: %+v", d)
	}
	if p, ok := lamp.GetComponent(components.Powered).(*components.PoweredComponent); !ok || p.Grid != "main" || p.Level != 110 {
		t.Errorf("station light's Powered after load: %+v", p)
	}
	if back.Flags["power:main"] != 1.0 || len(back.Modules) != 1 || back.Modules[0].Sealed != "keycard" {
		t.Errorf("after load: flag %v modules %+v", back.Flags["power:main"], back.Modules)
	}
}
