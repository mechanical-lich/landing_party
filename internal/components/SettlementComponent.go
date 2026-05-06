package components

import "github.com/mechanical-lich/mlge/ecs"

type SettlementComponent struct {
	Name string
}

func (s *SettlementComponent) GetType() ecs.ComponentType { return Settlement }
