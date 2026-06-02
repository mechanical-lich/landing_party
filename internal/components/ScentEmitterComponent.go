package components

import "github.com/mechanical-lich/mlge/ecs"

// ScentEmission is one passive scent the entity drops each turn at its
// current tile.
type ScentEmission struct {
	Tag      string  `json:"tag"`
	Strength float32 `json:"strength"`
}

// ScentEmitterComponent declares that an entity passively deposits one or
// more scents at its tile every turn. Event-based emissions (blood on hit,
// etc.) go through level.EmitScent directly and don't need this component.
type ScentEmitterComponent struct {
	Emissions []ScentEmission `json:"emissions"`
}

func (s *ScentEmitterComponent) GetType() ecs.ComponentType { return ScentEmitter }
