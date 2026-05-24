package components

import "github.com/mechanical-lich/mlge/ecs"

type NeedsComponent struct {
	// Exhaustion
	Exhaustion    int
	MaxExhaustion int
	LastBedX      int
	LastBedY      int
	LastBedZ      int
	HasKnownBed   bool

	// Hunger (0 = sated, MaxHunger = starving)
	Hunger          int
	MaxHunger       int
	HungerThreshold int // seek food above this
	StarveThreshold int // take damage above this
	HungerRate      int // turns between each +1 hunger tick
	hungerTick      int // unexported — not persisted
}

func (n *NeedsComponent) GetType() ecs.ComponentType { return Needs }

func (n *NeedsComponent) IsHungry() bool   { return n.MaxHunger > 0 && n.Hunger > n.HungerThreshold }
func (n *NeedsComponent) IsStarving() bool { return n.MaxHunger > 0 && n.Hunger > n.StarveThreshold }

func (n *NeedsComponent) Eat(amount int) {
	n.Hunger -= amount
	if n.Hunger < 0 {
		n.Hunger = 0
	}
}

func (n *NeedsComponent) TickHunger() bool {
	if n.MaxHunger <= 0 {
		return false
	}
	rate := n.HungerRate
	if rate <= 0 {
		rate = 10
	}
	n.hungerTick++
	if n.hungerTick >= rate {
		n.hungerTick = 0
		return true
	}
	return false
}
