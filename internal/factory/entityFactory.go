package factory

import (
	"fmt"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/lore"
)

var jsonFactory = ecs.NewJSONFactory()

func init() {
	registerComponents()
}

func FactoryLoad(filename string) error {
	return jsonFactory.LoadBlueprintsFromFile(filename)
}

func FactoryLoadDir(dir string) error {
	return jsonFactory.LoadBlueprintsFromDir(dir)
}

func Create(name string, x, y, z int) (*ecs.Entity, error) {
	if !jsonFactory.BlueprintExists(name) {
		return nil, fmt.Errorf("no blueprint found: %s", name)
	}

	entity, err := jsonFactory.CreateWithCallback(name, func(comp ecs.Component) error {
		if hc, ok := comp.(*rlcomponents.HealthComponent); ok {
			if hc.Health == 0 {
				hc.Health = hc.MaxHealth
			}
		}
		if dc, ok := comp.(*rlcomponents.DescriptionComponent); ok {
			if dc.Name == "<ColonistName>" {
				dc.Name = lore.RandomColonistName()
			}
		}
		if ic, ok := comp.(*rlcomponents.InventoryComponent); ok {
			if ic.StartingInventory != nil {
				for _, item := range ic.StartingInventory {
					itemEntity, err := Create(item, 0, 0, 0)
					if err != nil {
						return err
					}
					ic.AddItem(itemEntity)
				}
				ic.EquipAllBest()
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	pc := &rlcomponents.PositionComponent{}
	pc.SetPosition(x, y, z)
	entity.AddComponent(pc)
	entity.AddComponent(&rlcomponents.DirectionComponent{Direction: 0})

	return entity, nil
}

func BlueprintExists(name string) bool {
	return jsonFactory.BlueprintExists(name)
}

// GetAppearance returns the AppearanceComponent for a blueprint without placing it.
// Returns nil if the blueprint doesn't exist or has no Appearance.
func GetAppearance(name string) *components.AppearanceComponent {
	entity, err := jsonFactory.Create(name)
	if err != nil {
		return nil
	}
	if !entity.HasComponent(components.Appearance) {
		return nil
	}
	return entity.GetComponent(components.Appearance).(*components.AppearanceComponent)
}

func CreateComponent(name string, data map[string]interface{}) (ecs.Component, error) {
	return jsonFactory.CreateComponent(name, data)
}
