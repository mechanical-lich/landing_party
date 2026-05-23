package components

import "github.com/mechanical-lich/mlge/ecs"

type AppearanceComponent struct {
	R           uint8
	G           uint8
	B           uint8
	Bounce      bool
	Bounces     bool
	Resource    string
	SpriteX     int
	SpriteY     int
	BounceAxis  string // "x" (default) or "y"
	SpriteSize  int    // 0 = use global sprite size; otherwise e.g. 16 for item sheets
	SpriteWidth  int   // explicit width override (use with SpriteHeight for non-square sprites)
	SpriteHeight int   // explicit height override
}

func (a *AppearanceComponent) GetType() ecs.ComponentType { return Appearance }
