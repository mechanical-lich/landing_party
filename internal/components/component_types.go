package components

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

const (
	Appearance       ecs.ComponentType = "Appearance"
	Choppable        ecs.ComponentType = "Choppable"
	Drops            ecs.ComponentType = "Drops"
	FX               ecs.ComponentType = "FX"
	Selected         ecs.ComponentType = "Selected"
	Settlement       ecs.ComponentType = "Settlement"
	Storage          ecs.ComponentType = "Storage"
	Worker           ecs.ComponentType = "Worker"
	ResourceItem     ecs.ComponentType = "ResourceItem"
	ResearchBuilding ecs.ComponentType = "ResearchBuilding"
	FactionAI        ecs.ComponentType = "FactionAI"
)

// Shared component struct aliases from ml-rogue-lib
type (
	AIMemoryComponent       = rlcomponents.AIMemoryComponent
	ArmorComponent          = rlcomponents.ArmorComponent
	DeadComponent           = rlcomponents.DeadComponent
	DefensiveAIComponent    = rlcomponents.DefensiveAIComponent
	DescriptionComponent    = rlcomponents.DescriptionComponent
	DirectionComponent      = rlcomponents.DirectionComponent
	DoorComponent           = rlcomponents.DoorComponent
	HealthComponent         = rlcomponents.HealthComponent
	HostileAIComponent      = rlcomponents.HostileAIComponent
	InanimateComponent      = rlcomponents.InanimateComponent
	InitiativeComponent     = rlcomponents.InitiativeComponent
	InventoryComponent      = rlcomponents.InventoryComponent
	ItemComponent           = rlcomponents.ItemComponent
	MyTurnComponent         = rlcomponents.MyTurnComponent
	NeverSleepComponent     = rlcomponents.NeverSleepComponent
	PositionComponent       = rlcomponents.PositionComponent
	SolidComponent          = rlcomponents.SolidComponent
	StatsComponent          = rlcomponents.StatsComponent
	WanderAIComponent       = rlcomponents.WanderAIComponent
	WeaponComponent         = rlcomponents.WeaponComponent
)

func init() {
	ecs.InanimateComponentType = rlcomponents.Inanimate
}
