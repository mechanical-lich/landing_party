package components

import "github.com/mechanical-lich/mlge/ecs"

type AppearanceComponent struct {
	R          uint8
	G          uint8
	B          uint8
	Bounce     bool
	Bounces    bool
	Resource   string
	SpriteX    int
	SpriteY    int
	BounceAxis string // "x" (default) or "y"
}

func (a *AppearanceComponent) GetType() ecs.ComponentType { return Appearance }
