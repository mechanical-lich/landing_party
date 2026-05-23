package components

import (
	"github.com/mechanical-lich/mechanical-basic/pkg/basic"
	"github.com/mechanical-lich/mlge/ecs"
)

// ScriptedAIComponent drives NPC behavior via a per-turn mechanical-basic script.
// Vars is persisted across turns so scripts can maintain state (lifecycle stage,
// turn counters, bond targets, etc.) without relying on global level flags.
type ScriptedAIComponent struct {
	Script string         `json:"script"`
	Vars   map[string]any `json:"vars,omitempty"`

	// Runtime caches — not persisted.
	Interp          *basic.MechBasic `json:"-"`
	InterpScript    string           `json:"-"`
	InterpHasOnTurn bool             `json:"-"`
	PathCache       []int            `json:"-"`
	PathTargetX     int              `json:"-"`
	PathTargetY     int              `json:"-"`
}

func (s *ScriptedAIComponent) GetType() ecs.ComponentType { return ScriptedAI }
