package components

import "github.com/mechanical-lich/mlge/ecs"

// HungerComponent tracks an entity's food energy. Energy drains each turn;
// below HungerThreshold the entity seeks food, below StarveThreshold it takes damage.
type HungerComponent struct {
	Energy          int
	MaxEnergy       int
	HungerThreshold int // seek food below this
	StarveThreshold int // take damage below this
	DrainRate       int // ticks between each -1 energy drain (default 10)
	drainTick       int // internal counter
}

func (h *HungerComponent) GetType() ecs.ComponentType { return Hunger }

// Tick advances the drain counter. Returns true when energy should decrease.
func (h *HungerComponent) Tick() bool {
	rate := h.DrainRate
	if rate <= 0 {
		rate = 10
	}
	h.drainTick++
	if h.drainTick >= rate {
		h.drainTick = 0
		return true
	}
	return false
}

func (h *HungerComponent) IsHungry() bool   { return h.Energy < h.HungerThreshold }
func (h *HungerComponent) IsStarving() bool { return h.Energy < h.StarveThreshold }

func (h *HungerComponent) Eat(amount int) {
	h.Energy += amount
	if h.Energy > h.MaxEnergy {
		h.Energy = h.MaxEnergy
	}
}
