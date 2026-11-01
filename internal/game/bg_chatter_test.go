package game

import (
	"strings"
	"testing"

	"github.com/mechanical-lich/landing_party/internal/chatter"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/task"
)

// makeHungryMidTask puts a worker over the hunger threshold while holding an
// interruptible task, which makes the needs system post "Too hungry to continue".
func makeHungryMidTask(e *ecs.Entity) {
	nc := e.GetComponent(components.Needs).(*components.NeedsComponent)
	nc.Hunger = nc.HungerThreshold + 5
	wc := e.GetComponent(components.Worker).(*components.WorkerComponent)
	wc.CurrentTask = &task.Task{X: 2, Y: 2, Action: task_requests.BuildAction, Interruptible: true}
}

func logHas(text string) bool {
	for _, line := range message.MessageLog {
		if strings.Contains(line, text) {
			return true
		}
	}
	return false
}

// Worker chatter from a background step must not reach the live message log,
// while the same chatter on the live step still does.
func TestBackgroundStepKeepsWorkerChatterOutOfTheLog(t *testing.T) {
	ms := b0Planet(t, "Chatter Colony")
	w := b0Worker(t, ms, 1, 1)

	// The needs system only acts on the worker's turn, so step enough ticks for
	// several turns, re-arming the hunger interrupt each tick.
	const ticks = 60

	message.MessageLog = nil
	for i := 0; i < ticks; i++ {
		makeHungryMidTask(w)
		ms.stepBackground()
	}
	event.GetQueuedInstance().HandleQueue()
	if chatter.Background() {
		t.Fatal("background flag left set after stepBackground")
	}
	if logHas("Too hungry") {
		t.Fatal("background worker chatter reached the live log")
	}

	message.MessageLog = nil
	for i := 0; i < ticks && !logHas("Too hungry"); i++ {
		makeHungryMidTask(w)
		ms.stepWorld()
		event.GetQueuedInstance().HandleQueue()
	}
	if !logHas("Too hungry") {
		t.Fatal("live worker chatter should reach the log")
	}
}
