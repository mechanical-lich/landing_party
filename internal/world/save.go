package world

import (
	"encoding/json"
	"log"
	"os"

	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rllayered"
	"github.com/mechanical-lich/mlge/ecs"
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
	FloorRuns      []TileRun `json:"FloorRuns,omitempty"`
	MiddleRuns     []TileRun `json:"MiddleRuns,omitempty"`
	CeilingRuns    []TileRun `json:"CeilingRuns,omitempty"`
	Entities       []*SaveEntity
	StaticEntities []*SaveEntity
	Settlements    map[string]*settlement.Settlement
	MapSizeW       int
	MapSizeH       int
	MapSizeZ       int
}

// encodeSlotRuns RLE-compresses one slot across a tile array.
func encodeSlotRuns(tiles []Tile, pick func(t *Tile) (typ, variant int)) []TileRun {
	if len(tiles) == 0 {
		return nil
	}
	runs := make([]TileRun, 0, len(tiles)/4)
	t0, v0 := pick(&tiles[0])
	cur := TileRun{T: t0, V: v0, C: 1}
	for i := 1; i < len(tiles); i++ {
		ti, vi := pick(&tiles[i])
		if ti == cur.T && vi == cur.V {
			cur.C++
		} else {
			runs = append(runs, cur)
			cur = TileRun{T: ti, V: vi, C: 1}
		}
	}
	runs = append(runs, cur)
	return runs
}

func encodeFloorRuns(tiles []Tile) []TileRun {
	return encodeSlotRuns(tiles, func(t *Tile) (int, int) { return t.Floor.Type, t.Floor.Variant })
}
func encodeMiddleRuns(tiles []Tile) []TileRun {
	return encodeSlotRuns(tiles, func(t *Tile) (int, int) { return t.Middle.Type, t.Middle.Variant })
}
func encodeCeilingRuns(tiles []Tile) []TileRun {
	return encodeSlotRuns(tiles, func(t *Tile) (int, int) { return t.Ceiling.Type, t.Ceiling.Variant })
}

// decodeLayeredTiles rebuilds a tile array from three parallel RLE streams.
// Any stream may be empty (POC-era saves where ceilings were never painted).
func decodeLayeredTiles(floor, middle, ceiling []TileRun) []Tile {
	total := 0
	for _, r := range middle {
		total += r.C
	}
	if total == 0 {
		for _, r := range floor {
			total += r.C
		}
	}
	tiles := make([]Tile, total)
	apply := func(runs []TileRun, set func(t *Tile, typ, variant int)) {
		i := 0
		for _, r := range runs {
			for j := 0; j < r.C && i < total; j++ {
				set(&tiles[i], r.T, r.V)
				i++
			}
		}
	}
	apply(floor, func(t *Tile, typ, variant int) { t.Floor = rllayeredSlot(typ, variant) })
	apply(middle, func(t *Tile, typ, variant int) { t.Middle = rllayeredSlot(typ, variant) })
	apply(ceiling, func(t *Tile, typ, variant int) { t.Ceiling = rllayeredSlot(typ, variant) })
	return tiles
}

func rllayeredSlot(typ, variant int) rllayeredSlotT {
	return rllayeredSlotT{Type: typ, Variant: variant}
}

// rllayeredSlotT is a type alias to avoid pulling rllayered into every save
// declaration site.
type rllayeredSlotT = rllayered.Slot

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

// EntityToSaveEntity converts a live entity into its serializable form,
// applying the same component-specific handling SaveLevel uses (Inventory is
// flattened, HostileAI paths are dropped). Safe for entities not on any level
// (ship hold/roster, beaming).
func EntityToSaveEntity(entity *ecs.Entity) *SaveEntity {
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
	return se
}

func SaveLevel(level *Level) SaveData {
	saveEntities := make([]*SaveEntity, 0, len(level.Entities))
	for _, entity := range level.Entities {
		saveEntities = append(saveEntities, EntityToSaveEntity(entity))
	}

	staticSaveEntities := make([]*SaveEntity, 0, len(level.StaticEntities))
	for _, entity := range level.StaticEntities {
		staticSaveEntities = append(staticSaveEntities, EntityToSaveEntity(entity))
	}

	return SaveData{
		FloorRuns:      encodeFloorRuns(level.Data),
		MiddleRuns:     encodeMiddleRuns(level.Data),
		CeilingRuns:    encodeCeilingRuns(level.Data),
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

	tiles := decodeLayeredTiles(data.FloorRuns, data.MiddleRuns, data.CeilingRuns)
	for i, tile := range tiles {
		if i >= len(level.Data) {
			break
		}
		level.Data[i].Floor = tile.Floor
		level.Data[i].Middle = tile.Middle
		level.Data[i].Ceiling = tile.Ceiling
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

// RebuildLiveEntity reconstructs an entity from a SaveEntity that may still
// hold live component *structs* (as produced by EntityToSaveEntity in memory)
// rather than the map[string]any form RebuildEntity expects. It JSON
// round-trips the SaveEntity so component values become generic maps, then
// rebuilds. Use this for ship hold / roster / beaming.
func RebuildLiveEntity(entity *SaveEntity) *ecs.Entity {
	if entity == nil {
		return nil
	}
	raw, err := json.Marshal(entity)
	if err != nil {
		return RebuildEntity(entity)
	}
	var norm SaveEntity
	if err := json.Unmarshal(raw, &norm); err != nil {
		return RebuildEntity(entity)
	}
	return RebuildEntity(&norm)
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
