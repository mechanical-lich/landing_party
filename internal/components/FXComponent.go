package components

import "github.com/mechanical-lich/mlge/ecs"

type FXComponent struct {
	SpriteX  int
	SpriteY  int
	Resource string
	TTL      int
}

func (f *FXComponent) GetType() ecs.ComponentType { return FX }
