package game

import (
	"math/rand"

	"github.com/mechanical-lich/mlge/utility"
	"github.com/mechanical-lich/scifi_settlements/internal/config"
	"github.com/mechanical-lich/scifi_settlements/internal/factory"
	"github.com/mechanical-lich/scifi_settlements/internal/scenario"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

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
		_ = e // count hostiles once we have faction AI wired up
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
		bp := scenario.PickRandom(sc.SpawnRules, world.TileIndexToName[tile.Type], tile.LightLevel, z)
		if bp == "" {
			continue
		}
		entity, err := factory.Create(bp, x, y, z)
		if err == nil {
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
