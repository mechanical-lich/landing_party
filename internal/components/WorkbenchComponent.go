package components

import "github.com/mechanical-lich/mlge/ecs"

type CraftingStationComponent struct {
	StationID string `json:"StationID"`
}

func (c *CraftingStationComponent) GetType() ecs.ComponentType {
	return CraftingStation
}
