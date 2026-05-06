package components

import "github.com/mechanical-lich/mlge/ecs"

type DropsComponent struct {
	Items []string
}

func (d *DropsComponent) GetType() ecs.ComponentType { return Drops }
