package components

import "github.com/mechanical-lich/mlge/ecs"

type ChoppableComponent struct {
	Health int
}

func (c *ChoppableComponent) GetType() ecs.ComponentType { return Choppable }
