package components

import "github.com/mechanical-lich/mlge/ecs"

// PerceivedSmell is one entry in a SmellComponent's inbox: a tag that
// crossed the sensitivity threshold at the listener's sniff radius since
// its last turn.
type PerceivedSmell struct {
	Tag      string
	Strength float32
	X, Y, Z  int
}

// SmellComponent declares that an entity can perceive scents. Sensitivity
// is the minimum strength to register; SniffRadius is how far the entity
// looks for scent peaks (defaults to 3 if unset).
type SmellComponent struct {
	Sensitivity float32 `json:"sensitivity,omitempty"`
	SniffRadius int     `json:"sniffRadius,omitempty"`

	// Runtime — not persisted.
	// LastPerceived[tag] holds the strongest strength found within
	// SniffRadius last turn. Used to detect "newly perceived" tags (rose
	// above Sensitivity from below).
	LastPerceived map[string]float32 `json:"-"`
	NewSmells     []PerceivedSmell   `json:"-"`
	NewSmellsRead int                `json:"-"`
}

func (s *SmellComponent) GetType() ecs.ComponentType { return Smell }
