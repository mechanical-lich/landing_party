package components

import "github.com/mechanical-lich/mlge/ecs"

// ScriptComponent attaches a per-turn .basic script to an entity.
// OnTurn is the path to the .basic file to run each game tick.
type ScriptComponent struct {
	OnTurn string `json:"on_turn"`
}

func (s *ScriptComponent) GetType() ecs.ComponentType { return Script }
