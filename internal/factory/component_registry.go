package factory

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
)

func registerComponents() {
	jsonFactory.RegisterComponent("Description", func() ecs.Component { return &rlcomponents.DescriptionComponent{} })
	jsonFactory.RegisterComponent("Health", func() ecs.Component { return &rlcomponents.HealthComponent{} })
	jsonFactory.RegisterComponent("Appearance", func() ecs.Component { return &components.AppearanceComponent{} })
	jsonFactory.RegisterComponent("EquipmentAppearance", func() ecs.Component { return &components.EquipmentAppearanceComponent{} })
	jsonFactory.RegisterComponent("Solid", func() ecs.Component { return &rlcomponents.SolidComponent{} })
	jsonFactory.RegisterComponent("Initiative", func() ecs.Component { return &rlcomponents.InitiativeComponent{} })
	jsonFactory.RegisterComponent("Inventory", func() ecs.Component { return &rlcomponents.InventoryComponent{} })
	jsonFactory.RegisterComponent("Stats", func() ecs.Component { return &rlcomponents.StatsComponent{} })
	jsonFactory.RegisterComponent("Inanimate", func() ecs.Component { return &rlcomponents.InanimateComponent{} })
	jsonFactory.RegisterComponent("Position", func() ecs.Component { return &rlcomponents.PositionComponent{} })
	jsonFactory.RegisterComponent("Direction", func() ecs.Component { return &rlcomponents.DirectionComponent{} })
	jsonFactory.RegisterComponent("WanderAI", func() ecs.Component { return &rlcomponents.WanderAIComponent{} })
	jsonFactory.RegisterComponent("HostileAI", func() ecs.Component { return &rlcomponents.HostileAIComponent{} })
	jsonFactory.RegisterComponent("DefensiveAI", func() ecs.Component { return &rlcomponents.DefensiveAIComponent{} })
	jsonFactory.RegisterComponent("NeverSleep", func() ecs.Component { return &rlcomponents.NeverSleepComponent{} })
	jsonFactory.RegisterComponent("Item", func() ecs.Component { return &rlcomponents.ItemComponent{} })
	jsonFactory.RegisterComponent("Armor", func() ecs.Component { return &rlcomponents.ArmorComponent{} })
	jsonFactory.RegisterComponent("Weapon", func() ecs.Component { return &rlcomponents.WeaponComponent{} })
	jsonFactory.RegisterComponent("MyTurn", func() ecs.Component { return &rlcomponents.MyTurnComponent{} })
	jsonFactory.RegisterComponent("Dead", func() ecs.Component { return &rlcomponents.DeadComponent{} })
	jsonFactory.RegisterComponent("Door", func() ecs.Component { return &rlcomponents.DoorComponent{} })
	jsonFactory.RegisterComponent("AIMemory", func() ecs.Component { return &rlcomponents.AIMemoryComponent{} })
	jsonFactory.RegisterComponent("FX", func() ecs.Component { return &components.FXComponent{} })
	jsonFactory.RegisterComponent("Selected", func() ecs.Component { return &components.SelectedComponent{} })
	jsonFactory.RegisterComponent("Settlement", func() ecs.Component { return &components.SettlementComponent{} })
	jsonFactory.RegisterComponent("Storage", func() ecs.Component { return &components.StorageComponent{} })
	jsonFactory.RegisterComponent("Worker", func() ecs.Component { return &components.WorkerComponent{} })
	jsonFactory.RegisterComponent("Choppable", func() ecs.Component { return &components.ChoppableComponent{} })
	jsonFactory.RegisterComponent("Drops", func() ecs.Component { return &components.DropsComponent{} })
	jsonFactory.RegisterComponent("ResourceItem", func() ecs.Component { return &components.ResourceItemComponent{} })
	jsonFactory.RegisterComponent("ResearchBuilding", func() ecs.Component { return &components.ResearchBuildingComponent{} })
	jsonFactory.RegisterComponent("FactionAI", func() ecs.Component { return &components.FactionAIComponent{} })
	jsonFactory.RegisterComponent("Hunger", func() ecs.Component { return &components.HungerComponent{} })
	jsonFactory.RegisterComponent("Food", func() ecs.Component { return &rlcomponents.FoodComponent{} })
	jsonFactory.RegisterComponent("Light", func() ecs.Component { return &rlcomponents.LightComponent{} })
	jsonFactory.RegisterComponent("Workbench", func() ecs.Component { return &components.WorkbenchComponent{} })
	jsonFactory.RegisterComponent("Script", func() ecs.Component { return &components.ScriptComponent{} })
	jsonFactory.RegisterComponent("LaserBeam", func() ecs.Component { return &components.LaserBeamComponent{} })
}
