package components

import "github.com/mechanical-lich/mlge/ecs"

type ResearchBuildingComponent struct {
	Type string
}

func (r *ResearchBuildingComponent) GetType() ecs.ComponentType { return ResearchBuilding }
