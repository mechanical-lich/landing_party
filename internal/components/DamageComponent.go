package components

import "github.com/mechanical-lich/mlge/ecs"

type DamageComponent struct {
	Amount int
}

func (d *DamageComponent) GetType() ecs.ComponentType { return Damage }
