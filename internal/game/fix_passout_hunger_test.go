package game

import (
	"testing"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	fspath "github.com/mechanical-lich/landing_party/internal/path"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// passedOutWorker returns a colonist at (1,1) with maxed exhaustion, the given
// hunger and an empty bag (colonists start with a ration), so its first idle turn
// sends it to rest. A colonist gets a turn roughly every ten ticks.
func passedOutWorker(t *testing.T, ms *MainState, hunger int) (*ecs.Entity, *components.NeedsComponent) {
	t.Helper()
	w := b0Worker(t, ms, 1, 1)
	nc := w.GetComponent(components.Needs).(*components.NeedsComponent)
	nc.Exhaustion = nc.MaxExhaustion
	nc.Hunger = hunger
	w.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent).Bag = nil
	return w, nc
}

// rationLocker places a settlement locker at (x,y) holding n ration packs.
func rationLocker(t *testing.T, ms *MainState, x, y, n int) *components.StorageComponent {
	t.Helper()
	locker, err := factory.Create("storage_locker", x, y, 0)
	if err != nil {
		t.Fatalf("create locker: %v", err)
	}
	sc := locker.GetComponent(components.Storage).(*components.StorageComponent)
	sc.OwnedBy = ms.MainSettlement.Name
	for i := 0; i < n; i++ {
		ration, err := factory.Create("ration_pack", 0, 0, 0)
		if err != nil {
			t.Fatalf("create ration: %v", err)
		}
		if !sc.AddItem(ration) {
			t.Fatal("locker refused ration")
		}
	}
	ms.level.AddEntity(locker)
	return sc
}

// stepUntil steps the world until done reports true, and returns whether it did.
func stepUntil(ms *MainState, ticks int, done func() bool) bool {
	for i := 0; i < ticks; i++ {
		if done() {
			return true
		}
		fspath.ResetFrameCounter()
		ms.stepWorld()
	}
	return done()
}

func isPassedOut(w *ecs.Entity) bool {
	task := b0CurrentTask(w)
	return task != nil && task.Action == task_requests.PassoutAction
}

func healthOf(w *ecs.Entity) int {
	return w.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent).Health
}

// A worker who collapses before getting hungry wakes for food a few tiles away,
// eats and keeps their exhaustion, so maxing out again sends them back down.
func TestPassedOutWorkerWakesToEat(t *testing.T) {
	ms := b0Planet(t, "Hungry Sleeper")
	rationLocker(t, ms, 6, 1, 3)
	w, nc := passedOutWorker(t, ms, 60)
	startHealth := healthOf(w)

	if !stepUntil(ms, 200, func() bool { return isPassedOut(w) }) {
		t.Fatal("worker never passed out")
	}
	if !stepUntil(ms, 6000, func() bool { return nc.Hunger < nc.HungerThreshold-20 }) {
		t.Fatalf("never ate: hunger=%d health=%d", nc.Hunger, healthOf(w))
	}
	if healthOf(w) != startHealth {
		t.Errorf("health %d, want untouched %d", healthOf(w), startHealth)
	}
	if nc.Exhaustion < nc.MaxExhaustion/2 {
		t.Errorf("eating cleared exhaustion: %d", nc.Exhaustion)
	}
	nc.Exhaustion = nc.MaxExhaustion
	if !stepUntil(ms, 200, func() bool { return isPassedOut(w) }) {
		t.Error("exhaustion did not send the fed worker back to rest")
	}
}

// The reported case: already starving in the passout task with rations nearby.
func TestStarvingPassedOutWorkerEatsBeforeDying(t *testing.T) {
	ms := b0Planet(t, "Starving Sleeper")
	w, nc := passedOutWorker(t, ms, 50)
	if !stepUntil(ms, 200, func() bool { return isPassedOut(w) }) {
		t.Fatal("worker never passed out")
	}
	nc.Hunger = nc.StarveThreshold + 2
	rationLocker(t, ms, 6, 1, 3)

	if !stepUntil(ms, 1000, func() bool { return !nc.IsStarving() || healthOf(w) <= 0 }) {
		t.Fatalf("still starving: hunger=%d health=%d", nc.Hunger, healthOf(w))
	}
	if healthOf(w) <= 0 {
		t.Fatal("starved to death with rations five tiles away")
	}
}

// With nothing to eat the worker stays down, and hunger low enough not to matter
// lets them recover fully and get up as before.
func TestPassedOutWorkerWithoutFoodStillRecovers(t *testing.T) {
	ms := b0Planet(t, "Foodless Sleeper")
	w, nc := passedOutWorker(t, ms, 0)

	if !stepUntil(ms, 200, func() bool { return isPassedOut(w) }) {
		t.Fatal("worker never passed out")
	}
	if !stepUntil(ms, 20000, func() bool { return !isPassedOut(w) }) {
		t.Fatalf("never recovered: exhaustion=%d", nc.Exhaustion)
	}
	if nc.Exhaustion != 0 {
		t.Errorf("got up with exhaustion %d, want 0", nc.Exhaustion)
	}
}

// Starving with no food anywhere is not a reason to wake.
func TestStarvingWorkerWithoutFoodStaysPassedOut(t *testing.T) {
	ms := b0Planet(t, "Foodless Starver")
	w, nc := passedOutWorker(t, ms, 50)
	if !stepUntil(ms, 200, func() bool { return isPassedOut(w) }) {
		t.Fatal("worker never passed out")
	}
	nc.Hunger = nc.StarveThreshold + 2
	start := healthOf(w)
	stepUntil(ms, 400, func() bool { return !isPassedOut(w) || healthOf(w) < start-3 })
	if !isPassedOut(w) {
		t.Fatalf("worker woke with no food: hunger=%d", nc.Hunger)
	}
	if healthOf(w) >= start {
		t.Fatal("hunger never bit, so this proved nothing")
	}
}

// A spent, hungry worker with no food rests instead of standing idle for ever,
// which is also what a woken worker falls back to if the food has gone.
func TestSpentHungryWorkerWithoutFoodStillPassesOut(t *testing.T) {
	ms := b0Planet(t, "Spent and Hungry")
	w, nc := passedOutWorker(t, ms, 0)
	nc.Hunger = nc.HungerThreshold + 5
	if !stepUntil(ms, 200, func() bool { return isPassedOut(w) }) {
		t.Fatal("hungry worker with maxed exhaustion and no food never passed out")
	}
}

// Waking from a bed climbs off it rather than leaving the worker standing on it.
func TestWakingFromBedDismounts(t *testing.T) {
	ms := b0Planet(t, "Bed Sleeper")
	bed, err := factory.Create("bed", 3, 1, 0)
	if err != nil {
		t.Skipf("no bed blueprint: %v", err)
	}
	ms.level.AddEntity(bed)
	w, nc := passedOutWorker(t, ms, 0)
	onBed := func() bool {
		task := b0CurrentTask(w)
		if task == nil {
			return false
		}
		sr, ok := task.Data.(*task_requests.SleepRequest)
		return ok && sr.OnBed
	}
	if !stepUntil(ms, 300, onBed) {
		t.Fatal("worker never climbed into bed")
	}
	rationLocker(t, ms, 6, 1, 1)
	nc.Hunger = nc.HungerThreshold + 1
	if !stepUntil(ms, 200, func() bool { return !onBed() }) {
		t.Fatal("hungry worker stayed in bed")
	}
	pc := w.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if pc.GetX() == 3 && pc.GetY() == 1 {
		t.Error("worker still standing on the bed after waking")
	}
}
