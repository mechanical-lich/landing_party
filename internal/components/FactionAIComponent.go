package components

import "github.com/mechanical-lich/mlge/ecs"

// FactionAIComponent replaces per-race AI components with a data-driven behavior key.
type FactionAIComponent struct {
	BehaviorKey string // references behavior profile in data
	Faction     string
}

func (f *FactionAIComponent) GetType() ecs.ComponentType { return FactionAI }
