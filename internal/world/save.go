package world

import (
	"encoding/json"
	"log"
	"os"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/scifi_settlements/internal/config"
	"github.com/mechanical-lich/scifi_settlements/internal/factory"
	"github.com/mechanical-lich/scifi_settlements/internal/settlement"
)

type SaveEntity struct {
	Blueprint  string
	Components map[ecs.ComponentType]any
}

type saveInventory struct {
	LeftHand          *SaveEntity   `json:"LeftHand,omitempty"`
	RightHand         *SaveEntity   `json:"RightHand,omitempty"`
	Head              *SaveEntity   `json:"Head,omitempty"`
	Torso             *SaveEntity   `json:"Torso,omitempty"`
	Legs              *SaveEntity   `json:"Legs,omitempty"`
	Feet              *SaveEntity   `json:"Feet,omitempty"`
	Bag               []*SaveEntity `json:"Bag,omitempty"`
	StartingInventory []string      `json:"StartingInventory,omitempty"`
}

type TileRun struct {
	T int `json:"t"`
	V int `json:"v"`
	C int `json:"c"`
}

type SaveData struct {
	TileRuns       []TileRun
	Entities       []*SaveEntity
	StaticEntities []*SaveEntity
	Settlements    map[string]*settlement.Settlement
	MapSizeW       int
	MapSizeH       int
	MapSizeZ       int
}

func encodeTileRuns(tiles []Tile) []TileRun {
	if len(tiles) == 0 {
		return nil
	}
	runs := make([]TileRun, 0, len(tiles)/4)
	cur := TileRun{T: tiles[0].Type, V: tiles[0].Variant, C: 1}
	for _, tile := range tiles[1:] {
		if tile.Type == cur.T && tile.Variant == cur.V {
			cur.C++
		} else {
			runs = append(runs, cur)
			cur = TileRun{T: tile.Type, V: tile.Variant, C: 1}
		}
	}
	runs = append(runs, cur)
	return runs
}

func decodeTileRuns(runs []TileRun) []Tile {
	total := 0
	for _, r := range runs {
		total += r.C
	}
	tiles := make([]Tile, total)
	i := 0
	for _, r := range runs {
		for j := 0; j < r.C; j++ {
			tiles[i] = Tile{Type: r.T, Variant: r.V}
			i++
		}
	}
	return tiles
}

func entityToSaveEntity(e *ecs.Entity) *SaveEntity {
	if e == nil {
		return nil
	}
	se := &SaveEntity{Blueprint: e.Blueprint, Components: make(map[ecs.ComponentType]any)}
	for compType, comp := range e.Components {
		se.Components[compType] = comp
	}
	return se
}

func inventoryToSave(inv *rlcomponents.InventoryComponent) saveInventory {
	si := saveInventory{StartingInventory: inv.StartingInventory}
	si.LeftHand = entityToSaveEntity(inv.LeftHand)
	si.RightHand = entityToSaveEntity(inv.RightHand)
	si.Head = entityToSaveEntity(inv.Head)
	si.Torso = entityToSaveEntity(inv.Torso)
	si.Legs = entityToSaveEntity(inv.Legs)
	si.Feet = entityToSaveEntity(inv.Feet)
	for _, item := range inv.Bag {
		si.Bag = append(si.Bag, entityToSaveEntity(item))
	}
	return si
}

func saveInventoryToComponent(si map[string]any) *rlcomponents.InventoryComponent {
	inv := &rlcomponents.InventoryComponent{}

	rebuildSlot := func(key string) *ecs.Entity {
		raw, ok := si[key]
		if !ok || raw == nil {
			return nil
		}
		m, ok := raw.(map[string]any)
		if !ok {
			return nil
		}
		return rebuildSaveEntityFromMap(m)
	}

	inv.LeftHand = rebuildSlot("LeftHand")
	inv.RightHand = rebuildSlot("RightHand")
	inv.Head = rebuildSlot("Head")
	inv.Torso = rebuildSlot("Torso")
	inv.Legs = rebuildSlot("Legs")
	inv.Feet = rebuildSlot("Feet")

	if bagRaw, ok := si["Bag"]; ok && bagRaw != nil {
		if bagSlice, ok := bagRaw.([]any); ok {
			for _, itemRaw := range bagSlice {
				if m, ok := itemRaw.(map[string]any); ok {
					inv.Bag = append(inv.Bag, rebuildSaveEntityFromMap(m))
				}
			}
		}
	}

	if siRaw, ok := si["StartingInventory"]; ok && siRaw != nil {
		if slice, ok := siRaw.([]any); ok {
			for _, v := range slice {
				if s, ok := v.(string); ok {
					inv.StartingInventory = append(inv.StartingInventory, s)
				}
			}
		}
	}

	return inv
}

func rebuildSaveEntityFromMap(m map[string]any) *ecs.Entity {
	se := &SaveEntity{Components: make(map[ecs.ComponentType]any)}
	if bp, ok := m["Blueprint"].(string); ok {
		se.Blueprint = bp
	}
	if compsRaw, ok := m["Components"]; ok && compsRaw != nil {
		if compsMap, ok := compsRaw.(map[string]any); ok {
			for k, v := range compsMap {
				se.Components[ecs.ComponentType(k)] = v
			}
		}
	}
	return RebuildEntity(se)
}

func SaveLevel(level *Level) SaveData {
	saveEntities := make([]*SaveEntity, 0, len(level.Entities))
	for _, entity := range level.Entities {
		se := &SaveEntity{Blueprint: entity.Blueprint, Components: make(map[ecs.ComponentType]any)}
		for compType, comp := range entity.Components {
			switch compType {
			case rlcomponents.HostileAI:
				comp.(*rlcomponents.HostileAIComponent).Path = nil
				se.Components[compType] = comp
			case rlcomponents.Inventory:
				se.Components[compType] = inventoryToSave(comp.(*rlcomponents.InventoryComponent))
			default:
				se.Components[compType] = comp
			}
		}
		saveEntities = append(saveEntities, se)
	}

	staticSaveEntities := make([]*SaveEntity, 0, len(level.StaticEntities))
	for _, entity := range level.StaticEntities {
		se := &SaveEntity{Blueprint: entity.Blueprint, Components: make(map[ecs.ComponentType]any)}
		for compType, comp := range entity.Components {
			if compType == rlcomponents.Inventory {
				se.Components[compType] = inventoryToSave(comp.(*rlcomponents.InventoryComponent))
			} else {
				se.Components[compType] = comp
			}
		}
		staticSaveEntities = append(staticSaveEntities, se)
	}

	return SaveData{
		TileRuns:       encodeTileRuns(level.Data),
		Entities:       saveEntities,
		StaticEntities: staticSaveEntities,
		Settlements:    settlement.Settlements,
		MapSizeW:       level.GetWidth(),
		MapSizeH:       level.GetHeight(),
		MapSizeZ:       level.GetDepth(),
	}
}

func SaveLevelToFile(level *Level, filename string) error {
	data := SaveLevel(level)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, jsonData, 0644)
}

func LoadSaveData(data SaveData) *Level {
	w, h, d := data.MapSizeW, data.MapSizeH, data.MapSizeZ
	if w <= 0 {
		w = config.Global().WorldGenSizeW
	}
	if h <= 0 {
		h = config.Global().WorldGenSizeH
	}
	if d <= 0 {
		d = config.Global().WorldGenSizeZ
	}
	level := NewLevel(w, h, d)

	tiles := decodeTileRuns(data.TileRuns)
	for i, tile := range tiles {
		if i >= len(level.Data) {
			break
		}
		level.Data[i].Type = tile.Type
		level.Data[i].Variant = tile.Variant
	}

	for _, saveEntity := range data.Entities {
		entity := RebuildEntity(saveEntity)
		level.AddEntity(entity)
	}

	settlement.Settlements = data.Settlements
	return level
}

func LoadLevelFromFile(filename string) (*Level, error) {
	raw, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var saveData SaveData
	if err := json.Unmarshal(raw, &saveData); err != nil {
		return nil, err
	}
	return LoadSaveData(saveData), nil
}

func RebuildEntity(entity *SaveEntity) *ecs.Entity {
	newEntity := &ecs.Entity{Blueprint: entity.Blueprint}
	for compType, compInterface := range entity.Components {
		if compType == rlcomponents.Inventory {
			compMap, ok := compInterface.(map[string]any)
			if !ok {
				continue
			}
			newEntity.AddComponent(saveInventoryToComponent(compMap))
			continue
		}
		compMap, ok := compInterface.(map[string]any)
		if !ok {
			continue
		}
		comp, err := factory.CreateComponent(string(compType), compMap)
		if err != nil {
			log.Println("Error creating component:", err)
			continue
		}
		newEntity.AddComponent(comp)
	}
	return newEntity
}
