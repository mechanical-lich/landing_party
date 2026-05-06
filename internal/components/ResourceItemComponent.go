package components

import "github.com/mechanical-lich/mlge/ecs"

type ResourceItemComponent struct {
	Type string
}

func (r *ResourceItemComponent) GetType() ecs.ComponentType { return ResourceItem }
