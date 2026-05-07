package components

import "github.com/mechanical-lich/mlge/ecs"

type WorkbenchComponent struct {
	OwnedBy string
}

func (w *WorkbenchComponent) GetType() ecs.ComponentType {
	return Workbench
}
