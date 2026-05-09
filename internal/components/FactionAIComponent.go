package components

import "github.com/mechanical-lich/mlge/ecs"

// FactionAIComponent replaces per-race AI components with a data-driven behavior key.
type FactionAIComponent struct {
	BehaviorKey string // references behavior profile in data
	Faction     string
	// TargetX/Y/Z record the position of the most recently engaged hostile
	// target. Used for render-time animations (lean toward prey while
	// adjacent). HasTarget is false when no target is in sight.
	TargetX   int  `json:",omitempty"`
	TargetY   int  `json:",omitempty"`
	TargetZ   int  `json:",omitempty"`
	HasTarget bool `json:",omitempty"`
}

func (f *FactionAIComponent) GetType() ecs.ComponentType { return FactionAI }
