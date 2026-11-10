package components

import "github.com/mechanical-lich/mlge/ecs"

// ConvertBoost is the component type of ConvertBoostComponent.
const ConvertBoost ecs.ComponentType = "ConvertBoost"

// ConvertBoostComponent speeds ConvertActions near a structure (the autonz'
// converter node; autonz_plan.md): a convert within Radius tiles needs
// Factor of its work.
type ConvertBoostComponent struct {
	Radius int     `json:"radius"`
	Factor float64 `json:"factor"`
}

func (c *ConvertBoostComponent) GetType() ecs.ComponentType { return ConvertBoost }
