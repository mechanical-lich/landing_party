package systems

import (
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/emotes"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/task"
)

// Exhaustion constants — all tunable.
const (
	sleepRollMinPct = 60 // exhaustion must be at least this % of max before idle sleep roll fires

	// Note: exhaustion recovery per sleep cycle lives in workerai
	// (sleepRecoveryPerCycle), where the sleep task applies it.
	sleepCycleLength   = 10 // turns per sleep cycle (proper bed)
	passoutCycleLength = 40 // turns per recovery cycle (no bed, 4× slower)
)

// exhaustionPerAction maps task actions to the exhaustion added each colonist
// turn spent on that task. Actions absent from the map add no exhaustion.
var exhaustionPerAction = map[task.TaskAction]int{
	task_requests.DigAction:      1,
	task_requests.MineAction:     1,
	task_requests.BuildAction:    1,
	task_requests.PickupAction:   1,
	task_requests.RetrieveAction: 1,
	task_requests.ResearchAction: 1,
	task_requests.CraftAction:    1,
	task_requests.ForageAction:   1,
}

type NeedsSystem struct{}

var needsSystemRequires = []ecs.ComponentType{
	rlcomponents.Position,
	components.Worker,
	components.Needs,
	rlcomponents.MyTurn,
}

func (s *NeedsSystem) Requires() []ecs.ComponentType       { return needsSystemRequires }
func (s *NeedsSystem) UpdateSystem(data interface{}) error { return nil }

func (s *NeedsSystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	if entity.HasComponent(rlcomponents.Dead) {
		return nil
	}

	level := levelInterface.(*world.Level)
	nc := entity.GetComponent(components.Needs).(*components.NeedsComponent)
	wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)
	aiMemory := entity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent)

	// Don't run needs for player-controlled colonists.
	if wc.RogueControlled {
		return nil
	}

	name := entityDisplayName(entity)

	// --- Hunger accumulation ---
	if nc.TickHunger() && nc.Hunger < nc.MaxHunger {
		nc.Hunger++
	}
	if nc.IsStarving() && entity.HasComponent(rlcomponents.Health) {
		hc := entity.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
		hc.Health--
	}

	// --- Hunger task interrupt ---
	if nc.IsHungry() && wc.CurrentTask != nil && !wc.CurrentTask.Completed && wc.CurrentTask.Interruptible {
		message.PostMessage(name, "Too hungry to continue, needs food.")
		wc.CurrentTask.ReQueue()
		wc.CurrentTask = nil
		aiMemory.State = "idle"
		emotes.Set(entity, emotes.Swirl, 10, 1)
	}

	// --- Idle food-seeking ---
	if aiMemory.State == "idle" && nc.IsHungry() {
		aiMemory.TargetX = -1
		aiMemory.TargetY = -1
		aiMemory.State = "findfood"
		return nil
	}

	// --- Exhaustion accumulation ---
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		if gain, ok := exhaustionPerAction[wc.CurrentTask.Action]; ok {
			nc.Exhaustion += gain
			if nc.Exhaustion > nc.MaxExhaustion {
				nc.Exhaustion = nc.MaxExhaustion
			}
		}

		// Interrupt the task if exhaustion is maxed and the task allows it.
		if nc.Exhaustion >= nc.MaxExhaustion && wc.CurrentTask.Interruptible {
			message.PostMessage(name, "Too exhausted to continue, needs rest.")
			wc.CurrentTask.ReQueue()
			wc.CurrentTask = nil
			aiMemory.State = "idle"
			emotes.Set(entity, emotes.Sleep, 5, 1)
		}
	}

	// --- Idle sleep roll ---
	// Only fires once exhaustion is meaningfully high (sleepRollMinPct of max).
	// Probability then scales linearly across the remaining range up to 100%.
	if aiMemory.State != "idle" {
		return nil
	}
	minExhaustion := nc.MaxExhaustion * sleepRollMinPct / 100
	if nc.Exhaustion < minExhaustion {
		return nil
	}
	// Map [minExhaustion, MaxExhaustion] → [0, 100] for the roll.
	pct := (nc.Exhaustion - minExhaustion) * 100 / (nc.MaxExhaustion - minExhaustion)
	if rand.Intn(100) >= pct {
		return nil
	}

	assignRestTask(level, entity, wc, nc, aiMemory)
	return nil
}

// assignRestTask finds the best available rest option (known bed → visible bed
// → passout) and assigns it directly to the worker, bypassing the colony queue.
func assignRestTask(level *world.Level, entity *ecs.Entity, wc *components.WorkerComponent, nc *components.NeedsComponent, aiMemory *rlcomponents.AIMemoryComponent) {
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)

	bedX, bedY, bedZ, found := findBed(level, pc, nc)
	if found {
		t := &task.Task{
			Action: task_requests.SleepAction,
			X:      bedX, Y: bedY, Z: bedZ,
			Data: &task_requests.SleepRequest{
				X: bedX, Y: bedY, Z: bedZ,
				Required: sleepCycleLength,
			},
		}
		t.Start()
		wc.CurrentTask = t
		aiMemory.State = "task"
		emotes.Set(entity, emotes.Sleep, 5, 2)
		return
	}

	// No bed available. A sleeping bag lets the colonist bed down in place with
	// full (bed-quality) recovery instead of collapsing — used at the same point
	// a bed would be, not only when maxed out.
	if hasSleepingBag(entity) {
		t := &task.Task{
			Action: task_requests.PassoutAction,
			X:      pc.GetX(), Y: pc.GetY(), Z: pc.GetZ(),
			Data: &task_requests.PassoutRequest{Required: sleepCycleLength, Bedroll: true},
		}
		t.Start()
		wc.CurrentTask = t
		aiMemory.State = "task"
		emotes.Set(entity, emotes.Sleep, 5, 2)
		return
	}

	// No bag in hand — fetch one from settlement storage if available, then rest
	// with it next cycle (mirrors the find_food fetch).
	if entity.HasComponent(components.Settlement) {
		scName := entity.GetComponent(components.Settlement).(*components.SettlementComponent).Name
		if sleepingBagInStorage(level, scName) {
			// Clear any stale target left by a prior task so HandleFindBedroll
			// runs its storage search (mirrors the find_food transition above).
			aiMemory.TargetX = -1
			aiMemory.TargetY = -1
			aiMemory.State = "findbedroll"
			return
		}
	}

	// No bed and no sleeping bag anywhere: only pass out when fully maxed —
	// otherwise the colonist pushes through until a bed appears or exhaustion peaks.
	if nc.Exhaustion < nc.MaxExhaustion {
		return
	}
	t := &task.Task{
		Action: task_requests.PassoutAction,
		X:      pc.GetX(), Y: pc.GetY(), Z: pc.GetZ(),
		Data: &task_requests.PassoutRequest{Required: passoutCycleLength},
	}
	t.Start()
	wc.CurrentTask = t
	aiMemory.State = "task"
	message.PostMessage(entityDisplayName(entity), "Passed out from exhaustion.")
}

// sleepingBagInStorage reports whether any settlement-owned container holds a
// sleeping bag the colonist could go fetch.
func sleepingBagInStorage(level *world.Level, settlementName string) bool {
	for _, e := range level.Entities {
		if e == nil || !e.HasComponent(components.Storage) {
			continue
		}
		sc := e.GetComponent(components.Storage).(*components.StorageComponent)
		if sc.OwnedBy == settlementName && sc.HasItem("sleeping_bag") {
			return true
		}
	}
	return false
}

// hasSleepingBag reports whether the colonist is carrying a sleeping bag.
func hasSleepingBag(entity *ecs.Entity) bool {
	if !entity.HasComponent(rlcomponents.Inventory) {
		return false
	}
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for _, item := range inv.Bag {
		if item != nil && item.Blueprint == "sleeping_bag" {
			return true
		}
	}
	return false
}

// findBed returns the position of the best bed for this colonist: their known
// bed if it still exists, otherwise the nearest visible bed. Updates nc.LastBed
// when a new bed is discovered.
func findBed(level *world.Level, pc *rlcomponents.PositionComponent, nc *components.NeedsComponent) (x, y, z int, found bool) {
	cx, cy, cz := pc.GetX(), pc.GetY(), pc.GetZ()

	// Try the bed the colonist has used before.
	if nc.HasKnownBed {
		if bedExistsAt(level, nc.LastBedX, nc.LastBedY, nc.LastBedZ) {
			return nc.LastBedX, nc.LastBedY, nc.LastBedZ, true
		}
		nc.HasKnownBed = false
	}

	// Find the nearest currently-visible bed (check both entity lists).
	bestDist := -1
	bx, by, bz := 0, 0, 0
	for _, ents := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range ents {
			if !e.HasComponent(components.Bed) || !e.HasComponent(rlcomponents.Position) {
				continue
			}
			epc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			ex, ey, ez := epc.GetX(), epc.GetY(), epc.GetZ()
			if ez != cz || !level.GetVisible(ex, ey, ez) {
				continue
			}
			dx, dy := ex-cx, ey-cy
			dist := dx*dx + dy*dy
			if bestDist < 0 || dist < bestDist {
				bestDist = dist
				bx, by, bz = ex, ey, ez
			}
		}
	}

	if bestDist >= 0 {
		nc.LastBedX, nc.LastBedY, nc.LastBedZ = bx, by, bz
		nc.HasKnownBed = true
		return bx, by, bz, true
	}

	return 0, 0, 0, false
}

// bedExistsAt reports whether a bed entity is still present at (x, y, z).
func bedExistsAt(level *world.Level, x, y, z int) bool {
	for _, ents := range [][]*ecs.Entity{level.Entities, level.StaticEntities} {
		for _, e := range ents {
			if !e.HasComponent(components.Bed) || !e.HasComponent(rlcomponents.Position) {
				continue
			}
			epc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			if epc.GetX() == x && epc.GetY() == y && epc.GetZ() == z {
				return true
			}
		}
	}
	return false
}

// entityDisplayName returns the colonist's individual name or "Colonist".
func entityDisplayName(entity *ecs.Entity) string {
	if entity.HasComponent(rlcomponents.Description) {
		dc := entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
		if dc.Name != "" {
			return dc.Name
		}
	}
	return "Colonist"
}
