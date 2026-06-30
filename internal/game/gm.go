package game

import (
	"math/rand"

	"github.com/mechanical-lich/landing_party/internal/components"
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
	// suppressSpawns disables hostile spawning entirely. Set for safe levels
	// like The Ship, which reads the global (orbited location's) scenario but
	// must never spawn that scenario's hostiles aboard. A future scenario
	// ship_effects block will be the opt-in path for deliberate boardings.
	suppressSpawns bool
}

func (gm *GameMaster) Init(level *world.Level) {
	gm.level = level
}

func (gm *GameMaster) Update() {
	// No spawning on safe levels, and never before a scenario is selected
	// (Active() would panic — e.g. visiting The Ship at campaign start).
	if gm.suppressSpawns || !scenario.HasActive() {
		return
	}
	all := scenario.AllEnabled()
	if len(all) == 0 {
		return
	}
	sc := scenario.Active()

	hostileCount := 0
	for _, e := range gm.level.Entities {
		if !e.HasComponent(rlcomponents.Dead) &&
			(e.HasComponent(components.FactionAI) || e.HasComponent(components.ScriptedAI)) {
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
		bp := scenario.PickRandom(sc.SpawnRules, world.TileIndexToName[nameSlot.Type], tile.LightLevel, z, gm.level.SurfaceZ)
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
	z := gm.level.SurfaceZ
	if utility.GetRandom(0, 100) < 30 {
		z = rand.Intn(gm.level.GetDepth())
	}
	x, y := gm.getFreeSpaceAtZ(z)
	if x == -1 {
		return -1, -1, -1
	}
	return x, y, z
}

func (gm *GameMaster) getFreeSpaceAtZ(z int) (int, int) {
	for i := 0; i < maxLocationAttempts; i++ {
		x := rand.Intn(gm.level.GetWidth())
		y := rand.Intn(gm.level.GetHeight())
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
