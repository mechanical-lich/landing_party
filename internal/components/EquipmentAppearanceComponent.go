package components

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

type ArmorRowConfig struct {
	Row  int
	Tags []string
}

type EquipmentAppearanceComponent struct {
	Resource            string
	SpriteSize          int
	BlockOriginX        int
	BlockOriginY        int
	AnimationFrames     int
	DefaultWeaponColumn int
	DefaultArmorRow     int
	WeaponColumns       map[string]int
	ArmorRows           []ArmorRowConfig
}

func (e *EquipmentAppearanceComponent) GetType() ecs.ComponentType { return EquipmentAppearance }

// ResolveSprite returns the sprite sheet coordinates for the entity's current equipment.
func (e *EquipmentAppearanceComponent) ResolveSprite(entity *ecs.Entity, bounce bool) (spriteX, spriteY int) {
	col := e.DefaultWeaponColumn
	row := e.DefaultArmorRow

	if entity.HasComponent(rlcomponents.Inventory) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)

		// Weapon: pick display key from first equipped hand, fall back to blueprint name.
		for _, hand := range []*ecs.Entity{inv.RightHand, inv.LeftHand} {
			if hand == nil || !hand.HasComponent(rlcomponents.Weapon) {
				continue
			}
			wc := hand.GetComponent(rlcomponents.Weapon).(*rlcomponents.WeaponComponent)
			key := wc.Display
			if key == "" {
				key = hand.Blueprint
			}
			if c, ok := e.WeaponColumns[key]; ok {
				col = c
			} else if c, ok := e.WeaponColumns["*"]; ok {
				col = c
			}
			break
		}

		// Armor: collect all tags from equipped armor slots.
		var equippedTags []string
		for _, slot := range []*ecs.Entity{inv.Head, inv.Torso, inv.Legs, inv.Feet} {
			if slot == nil || !slot.HasComponent(rlcomponents.Armor) {
				continue
			}
			ac := slot.GetComponent(rlcomponents.Armor).(*rlcomponents.ArmorComponent)
			equippedTags = append(equippedTags, ac.Tags...)
		}

		// Best-fit: pick the row whose tags overlap most with equipped tags.
		bestScore := -1
		for _, armorRow := range e.ArmorRows {
			score := 0
			for _, rowTag := range armorRow.Tags {
				for _, et := range equippedTags {
					if et == rowTag {
						score++
					}
				}
			}
			if score > bestScore {
				bestScore = score
				row = armorRow.Row
			}
		}
	}

	frames := e.AnimationFrames
	if frames < 1 {
		frames = 1
	}
	size := e.SpriteSize

	spriteX = e.BlockOriginX + col*frames*size
	if bounce && frames > 1 {
		spriteX += size
	}
	spriteY = e.BlockOriginY + row*size
	return
}
