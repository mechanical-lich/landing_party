package components

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

const (
	Appearance            ecs.ComponentType = "Appearance"
	EquipmentAppearance   ecs.ComponentType = "EquipmentAppearance"
	Choppable        ecs.ComponentType = "Choppable"
	Damage           ecs.ComponentType = "Damage"
	Drops            ecs.ComponentType = "Drops"
	FX               ecs.ComponentType = "FX"
	Selected         ecs.ComponentType = "Selected"
	Settlement       ecs.ComponentType = "Settlement"
	Storage          ecs.ComponentType = "Storage"
	Worker           ecs.ComponentType = "Worker"
	ResourceItem     ecs.ComponentType = "ResourceItem"
	ResearchBuilding ecs.ComponentType = "ResearchBuilding"
	FactionAI        ecs.ComponentType = "FactionAI"
	Hunger           ecs.ComponentType = "Hunger"
	Light            ecs.ComponentType = "Light"
	CraftingStation  ecs.ComponentType = "CraftingStation"
	Skills           ecs.ComponentType = "Skills"
	Script           ecs.ComponentType = "Script"
	ScriptedAI       ecs.ComponentType = "ScriptedAI"
	LaserBeam        ecs.ComponentType = "LaserBeam"
	QuestTarget      ecs.ComponentType = "QuestTarget"
	Datapad          ecs.ComponentType = "Datapad"
	Bed              ecs.ComponentType = "Bed"
)


// Shared component struct aliases from ml-rogue-lib
type (
	FoodComponent  = rlcomponents.FoodComponent
	LightComponent = rlcomponents.LightComponent

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
