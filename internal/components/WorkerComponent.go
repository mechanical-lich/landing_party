package components

import (
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

type WorkerComponent struct {
	Role        string
	CurrentTask *task.Task
}

func (w *WorkerComponent) GetType() ecs.ComponentType { return Worker }
