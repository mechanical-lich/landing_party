package components

import (
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

type WorkerComponent struct {
	Role        string
	CurrentTask *task.Task
	// InteractTicks holds the worker at an interaction (pickup/dropoff) for a
	// few ticks so its work animation has time to play. The handlers increment
	// it while at the target and reset it on completion.
	InteractTicks  int
	LastTaskAction task.TaskAction
	// AllowedTasks lists the task types this colonist will accept.
	// nil or empty means no filter — all task types are allowed.
	AllowedTasks []task.TaskAction
	// SwapCooldown prevents a displaced colonist from being swapped again
	// immediately, avoiding oscillation in tight corridors.
	SwapCooldown int
}

func (w *WorkerComponent) GetType() ecs.ComponentType { return Worker }
