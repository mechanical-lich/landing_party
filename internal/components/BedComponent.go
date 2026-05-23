package components

import "github.com/mechanical-lich/mlge/ecs"

// BedComponent marks an entity as a bed that workers can use to rest and heal.
type BedComponent struct{}

func (b *BedComponent) GetType() ecs.ComponentType { return Bed }
