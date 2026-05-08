package components

import "github.com/mechanical-lich/mlge/ecs"

type LaserBeamComponent struct {
	R      uint8
	G      uint8
	B      uint8
	Length int // visual beam length in tiles beyond the target (0 = exact attacker-to-target)
}

func (l *LaserBeamComponent) GetType() ecs.ComponentType { return LaserBeam }
