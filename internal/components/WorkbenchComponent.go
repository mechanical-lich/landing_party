package components

import "github.com/mechanical-lich/mlge/ecs"

type CraftingStationComponent struct {
	StationID    string `json:"StationID"`
	RequiresTech string `json:"RequiresTech,omitempty"`
}

func (c *CraftingStationComponent) GetType() ecs.ComponentType {
	return CraftingStation
}
