package factory

import (
	"fmt"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/lore"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
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
			// A blueprint name wrapped like "<mutant>" or "<colonist>"
			// is a request to roll a random name of that type.
			dc.Name = lore.ResolveName(dc.Name)
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

// GetDescription returns the blueprint's raw DescriptionComponent without
// running the per-instance Create callback that resolves "<type>" name
// placeholders into random rolls. Use this when you want the blueprint's
// archetype name and lore (e.g. the Encyclopedia) rather than a fresh
// random instance.
func GetDescription(name string) *rlcomponents.DescriptionComponent {
	entity, err := jsonFactory.Create(name)
	if err != nil {
		return nil
	}
	if !entity.HasComponent(rlcomponents.Description) {
		return nil
	}
	return entity.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent)
}

// GetSize returns the SizeComponent for a blueprint without placing it.
// Returns nil if the blueprint doesn't exist or has no Size.
func GetSize(name string) *rlcomponents.SizeComponent {
	entity, err := jsonFactory.Create(name)
	if err != nil {
		return nil
	}
	if !entity.HasComponent(rlcomponents.Size) {
		return nil
	}
	return entity.GetComponent(rlcomponents.Size).(*rlcomponents.SizeComponent)
}

// materialTagCache memoizes per-blueprint material tags so per-frame UI
// callers (relocate/store hover, inspector lookups) don't allocate a fresh
// entity every tick just to read a tag slice.
var materialTagCache = map[string][]string{}

// GetMaterialTags returns the Material.Tags for a blueprint, or nil if the
// blueprint has no Material component. The result is cached and shared — do
// not mutate the returned slice.
func GetMaterialTags(name string) []string {
	if tags, ok := materialTagCache[name]; ok {
		return tags
	}
	entity, err := jsonFactory.Create(name)
	if err != nil || !entity.HasComponent(components.Material) {
		materialTagCache[name] = nil
		return nil
	}
	tags := entity.GetComponent(components.Material).(*components.MaterialComponent).Tags
	materialTagCache[name] = tags
	return tags
}

func CreateComponent(name string, data map[string]interface{}) (ecs.Component, error) {
	return jsonFactory.CreateComponent(name, data)
}
