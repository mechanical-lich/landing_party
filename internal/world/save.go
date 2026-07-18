package world

import (
	"encoding/json"
	"log"
	"os"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/settlement"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rllayered"
	"github.com/mechanical-lich/mlge/ecs"
)

// saveStorageData is the JSON-serialisable form of a StorageComponent,
// with items stored as SaveEntity records so each item's components are
// properly round-tripped through the factory on load.
type saveStorageData struct {
	Capacity    int           `json:"Capacity,omitempty"`
	OwnedBy     string        `json:"OwnedBy"`
	AllowedTags []string      `json:"AllowedTags,omitempty"`
	FilterTags  []string      `json:"FilterTags,omitempty"`
	Items       []*SaveEntity `json:"Items,omitempty"`
}

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

// SaveVersion is the current save-schema version, written into every SaveData.
// Bump it when a change isn't purely additive, and branch on data.Version in
// LoadSaveData to migrate. Legacy saves have Version 0.
const SaveVersion = 1

type TileRun struct {
	T int `json:"t"`
	V int `json:"v"`
	C int `json:"c"`
}

// IntRun / StrRun are run-length entries for the terrain skeleton (mostly long
// uniform runs, so RLE keeps saves compact). V/S is the value, C the count.
type IntRun struct {
	V int `json:"v"`
	C int `json:"c"`
}

type StrRun struct {
	S string `json:"s"`
	C int    `json:"c"`
}

func encodeTerrainRuns(t []TerrainKind) []IntRun {
	var runs []IntRun
	for i := 0; i < len(t); {
		v := t[i]
		j := i + 1
		for j < len(t) && t[j] == v {
			j++
		}
		runs = append(runs, IntRun{V: int(v), C: j - i})
		i = j
	}
	return runs
}

func decodeTerrainRuns(runs []IntRun, n int) []TerrainKind {
	out := make([]TerrainKind, n)
	i := 0
	for _, r := range runs {
		for c := 0; c < r.C && i < n; c++ {
			out[i] = TerrainKind(r.V)
			i++
		}
	}
	return out
}

func encodeInt16Runs(s []int16) []IntRun {
	var runs []IntRun
	for i := 0; i < len(s); {
		v := s[i]
		j := i + 1
		for j < len(s) && s[j] == v {
			j++
		}
		runs = append(runs, IntRun{V: int(v), C: j - i})
		i = j
	}
	return runs
}

func decodeInt16Runs(runs []IntRun, n int) []int16 {
	out := make([]int16, n)
	i := 0
	for _, r := range runs {
		for c := 0; c < r.C && i < n; c++ {
			out[i] = int16(r.V)
			i++
		}
	}
	return out
}

func encodeStrRuns(b []string) []StrRun {
	var runs []StrRun
	for i := 0; i < len(b); {
		v := b[i]
		j := i + 1
		for j < len(b) && b[j] == v {
			j++
		}
		runs = append(runs, StrRun{S: v, C: j - i})
		i = j
	}
	return runs
}

func decodeStrRuns(runs []StrRun, n int) []string {
	out := make([]string, n)
	i := 0
	for _, r := range runs {
		for c := 0; c < r.C && i < n; c++ {
			out[i] = r.S
			i++
		}
	}
	return out
}

type SaveData struct {
	FloorRuns  []TileRun `json:"FloorRuns,omitempty"`
	MiddleRuns []TileRun `json:"MiddleRuns,omitempty"`
	// CeilingRuns was a third slot in older saves; the ceiling is now the Floor
	// of the cell above (see rllayered.Tile). Old saves' CeilingRuns key is
	// ignored on load — it was always empty, so nothing is lost.
	// TileCatalog is a snapshot of TileIndexToName at save time. On load we
	// rebuild an old-index -> current-index remap by name so re-ordering or
	// splitting tile_definitions/*.json (which shifts indices) doesn't
	// silently rewrite the world. Absent on legacy saves -> indices are used
	// raw (graceful degradation, matches old behavior).
	TileCatalog    []string `json:"TileCatalog,omitempty"`
	Entities       []*SaveEntity
	StaticEntities []*SaveEntity
	Settlements    map[string]*settlement.Settlement
	MapSizeW       int
	MapSizeH       int
	MapSizeZ       int
	// ResourceAmount is the remaining yield of each mineable deposit tile, keyed
	// by packed coordinate. Persisted so partial mining survives save/load and
	// campaign freeze (and can't be reset by re-issuing a mine order).
	ResourceAmount map[TileCoord]int `json:"ResourceAmount,omitempty"`

	// Version is the save-schema version (see SaveVersion). Absent (0) on legacy
	// saves, which predate the terrain-skeleton/Flags persistence below.
	Version int `json:"Version,omitempty"`

	// Terrain skeleton — not derivable from the tile slots, and read at runtime
	// (AI burrowing via GetTerrainKind, worm surfacing via GetSurfaceZ, biome
	// queries, region anchors). Without these a loaded level returns void terrain.
	TerrainRuns []IntRun            `json:"TerrainRuns,omitempty"`
	SurfaceRuns []IntRun            `json:"SurfaceRuns,omitempty"`
	BiomeRuns   []StrRun            `json:"BiomeRuns,omitempty"`
	Regions     map[string][][3]int `json:"Regions,omitempty"`

	// Flags holds script/quest state (start coords, settlement name, colonist
	// faction, game flags read by the objective evaluator). Values are
	// float64/string, so JSON round-trips cleanly.
	Flags map[string]any `json:"Flags,omitempty"`
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

// buildTileRemap returns a slice where remap[savedIdx] gives the current tile
// index for the tile that lived at savedIdx in the saved catalog. Names absent
// from the current catalog map to 0 (the empty sentinel) and are logged so a
// removed-tile save degrades to empty space instead of a wrong tile. Returns
// nil if the saved catalog is empty — callers should then use indices raw.
func buildTileRemap(savedCatalog []string) []int {
	if len(savedCatalog) == 0 {
		return nil
	}
	remap := make([]int, len(savedCatalog))
	for i, name := range savedCatalog {
		if idx, ok := TileNameToIndex[name]; ok {
			remap[i] = idx
			continue
		}
		log.Printf("save: tile %q (saved index %d) not in current catalog, falling back to empty", name, i)
		remap[i] = 0
	}
	return remap
}

// decodeLayeredTiles rebuilds a tile array from three parallel RLE streams.
// Any stream may be empty (POC-era saves where ceilings were never painted).
// remap, when non-nil, translates saved tile indices to current indices.
func decodeLayeredTiles(floor, middle []TileRun, remap []int) []Tile {
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
	translate := func(t int) int {
		if remap == nil || t < 0 || t >= len(remap) {
			return t
		}
		return remap[t]
	}
	apply := func(runs []TileRun, set func(t *Tile, typ, variant int)) {
		i := 0
		for _, r := range runs {
			typ := translate(r.T)
			for j := 0; j < r.C && i < total; j++ {
				set(&tiles[i], typ, r.V)
				i++
			}
		}
	}
	apply(floor, func(t *Tile, typ, variant int) { t.Floor = rllayeredSlot(typ, variant) })
	apply(middle, func(t *Tile, typ, variant int) { t.Middle = rllayeredSlot(typ, variant) })
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
		case components.Storage:
			sc := comp.(*components.StorageComponent)
			saved := &saveStorageData{
				Capacity:    sc.Capacity,
				OwnedBy:     sc.OwnedBy,
				AllowedTags: sc.AllowedTags,
				FilterTags:  sc.FilterTags,
			}
			for _, item := range sc.Items {
				if item != nil {
					saved.Items = append(saved.Items, EntityToSaveEntity(item))
				}
			}
			se.Components[compType] = saved
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

	// Snapshot the catalog so a future load can remap by name if the order
	// changes (file split, rename, additions). Copy so later loads can't
	// alias and mutate this slice.
	catalog := make([]string, len(TileIndexToName))
	copy(catalog, TileIndexToName)

	var resourceAmount map[TileCoord]int
	if len(level.ResourceAmount) > 0 {
		resourceAmount = make(map[TileCoord]int, len(level.ResourceAmount))
		for k, v := range level.ResourceAmount {
			resourceAmount[k] = v
		}
	}

	return SaveData{
		Version:        SaveVersion,
		FloorRuns:      encodeFloorRuns(level.Data),
		MiddleRuns:     encodeMiddleRuns(level.Data),
		TileCatalog:    catalog,
		Entities:       saveEntities,
		StaticEntities: staticSaveEntities,
		Settlements:    settlement.Settlements,
		MapSizeW:       level.GetWidth(),
		MapSizeH:       level.GetHeight(),
		MapSizeZ:       level.GetDepth(),
		ResourceAmount: resourceAmount,
		TerrainRuns:    encodeTerrainRuns(level.Terrain),
		SurfaceRuns:    encodeInt16Runs(level.SurfaceMap),
		BiomeRuns:      encodeStrRuns(level.BiomeMap),
		Regions:        level.Regions,
		Flags:          level.Flags,
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
	// MapSize is written by SaveLevel and must always be present in a valid
	// save. A zero or negative dimension means the save predates the
	// per-location size persistence or is corrupted — fail loudly rather
	// than silently picking arbitrary defaults.
	if data.MapSizeW <= 0 || data.MapSizeH <= 0 || data.MapSizeZ <= 0 {
		log.Printf("save: invalid map size in save data (W=%d H=%d Z=%d); cannot load",
			data.MapSizeW, data.MapSizeH, data.MapSizeZ)
		return nil
	}
	level := NewLevel(data.MapSizeW, data.MapSizeH, data.MapSizeZ)

	remap := buildTileRemap(data.TileCatalog)
	tiles := decodeLayeredTiles(data.FloorRuns, data.MiddleRuns, remap)
	for i, tile := range tiles {
		if i >= len(level.Data) {
			break
		}
		level.Data[i].Floor = tile.Floor
		level.Data[i].Middle = tile.Middle
	}

	// Terrain skeleton: allocate defaults, then overlay whatever the save carried.
	// Legacy saves (no runs) at least get non-nil maps instead of void terrain.
	level.AllocTerrain()
	w, h, d := level.GetWidth(), level.GetHeight(), level.GetDepth()
	if len(data.TerrainRuns) > 0 {
		level.Terrain = decodeTerrainRuns(data.TerrainRuns, w*h*d)
	}
	if len(data.SurfaceRuns) > 0 {
		level.SurfaceMap = decodeInt16Runs(data.SurfaceRuns, w*h)
	}
	if len(data.BiomeRuns) > 0 {
		level.BiomeMap = decodeStrRuns(data.BiomeRuns, w*h)
	}
	if data.Regions != nil {
		level.Regions = data.Regions
	}
	if data.Flags != nil {
		level.Flags = data.Flags
	}

	for _, saveEntity := range data.Entities {
		level.AddEntity(RebuildEntity(saveEntity))
	}
	// Static (Inanimate) entities were saved but previously never rebuilt.
	for _, saveEntity := range data.StaticEntities {
		level.AddEntity(RebuildEntity(saveEntity))
	}

	for k, v := range data.ResourceAmount {
		if v > 0 {
			level.ResourceAmount[k] = v
		}
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
		if compType == components.Storage {
			compMap, ok := compInterface.(map[string]any)
			if !ok {
				continue
			}
			raw, err := json.Marshal(compMap)
			if err != nil {
				continue
			}
			var saved saveStorageData
			if err := json.Unmarshal(raw, &saved); err != nil {
				continue
			}
			sc := &components.StorageComponent{
				Capacity:    saved.Capacity,
				OwnedBy:     saved.OwnedBy,
				AllowedTags: saved.AllowedTags,
				FilterTags:  saved.FilterTags,
			}
			for _, savedItem := range saved.Items {
				if savedItem != nil {
					sc.Items = append(sc.Items, RebuildEntity(savedItem))
				}
			}
			newEntity.AddComponent(sc)
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
