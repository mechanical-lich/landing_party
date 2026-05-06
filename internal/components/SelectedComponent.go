package components

import "github.com/mechanical-lich/mlge/ecs"

type SelectedComponent struct{}

func (s *SelectedComponent) GetType() ecs.ComponentType { return Selected }
