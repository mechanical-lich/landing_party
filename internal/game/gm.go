package game

import (
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/config"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/scenario"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/utility"
)

func giveStartingEquipment(entity *ecs.Entity, equipment map[string]float64) {
	if len(equipment) == 0 || !entity.HasComponent(rlcomponents.Inventory) {
		return
	}
	inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for bp, chance := range equipment {
		if rand.Float64() < chance {
			item, err := factory.Create(bp, 0, 0, 0)
			if err != nil {
				continue
			}
			inv.AddItem(item)
		}
	}
	inv.EquipAllBest()
}

const maxLocationAttempts = 100

// GameMaster manages the living world: hostile spawns, ambient creatures, etc.
type GameMaster struct {
	level *world.Level
}

func (gm *GameMaster) Init(level *world.Level) {
	gm.level = level
}

func (gm *GameMaster) Update() {
	all := scenario.AllEnabled()
	if len(all) == 0 {
		return
	}
	sc := scenario.Active()

	hostileCount := 0
	for _, e := range gm.level.Entities {
		if e.HasComponent(components.FactionAI) && !e.HasComponent(rlcomponents.Dead) {
			hostileCount++
		}
	}

	hostileMax := sc.HostileMax
	if hostileMax <= 0 {
		return
	}
	if hostileCount >= hostileMax || len(sc.SpawnRules) == 0 {
		return
	}

	for i := 0; i < hostileMax-hostileCount; i++ {
		x, y, z := gm.pickSpawnLocation()
		if x == -1 {
			continue
		}
		ti := gm.level.GetTileAt(x, y, z)
		if ti == nil {
			continue
		}
		tile := ti.(*world.Tile)
		// Spawn rules match on surface tile name. Prefer Floor (the ground
		// surface entities walk on), fall back to Middle.
		nameSlot := tile.Floor
		if nameSlot.IsEmpty() {
			nameSlot = tile.Middle
		}
		if nameSlot.IsEmpty() {
			continue
		}
		bp := scenario.PickRandom(sc.SpawnRules, world.TileIndexToName[nameSlot.Type], tile.LightLevel, z)
		if bp == "" {
			continue
		}
		entity, err := factory.Create(bp, x, y, z)
		if err == nil {
			giveStartingEquipment(entity, sc.SpawnRules[bp].StartingEquipment)
			gm.level.AddEntity(entity)
		}
	}
}

func (gm *GameMaster) pickSpawnLocation() (int, int, int) {
	z := config.Global().StartingZ
	if utility.GetRandom(0, 100) < 30 {
		z = rand.Intn(config.Global().WorldGenSizeZ)
	}
	x, y := gm.getFreeSpaceAtZ(z)
	if x == -1 {
		return -1, -1, -1
	}
	return x, y, z
}

func (gm *GameMaster) getFreeSpaceAtZ(z int) (int, int) {
	for i := 0; i < maxLocationAttempts; i++ {
		x := rand.Intn(config.Global().WorldGenSizeW)
		y := rand.Intn(config.Global().WorldGenSizeH)
		tile := gm.level.GetTileAt(x, y, z)
		if tile != nil && !tile.IsSolid() && !tile.IsWater() && gm.level.GetEntityAt(x, y, z) == nil {
			return x, y
		}
	}
	return -1, -1
}

// GetFreeSpaceAtZ is exported for use during world setup.
func (gm *GameMaster) GetFreeSpaceAtZ(z int) (int, int) {
	return gm.getFreeSpaceAtZ(z)
}
