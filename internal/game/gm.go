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
	// like The Ship, which has no scenario of its own. A future ship_effects
	// block, read from the orbited scenario (scenario.Active), will be the opt-in
	// path for deliberate boardings.
	suppressSpawns bool
	// player names the player's settlement on this level (MainState.colonyName);
	// the player's own bodies never count as hostiles. nil: none.
	player func() string
}

func (gm *GameMaster) Init(level *world.Level) {
	gm.level = level
}

// Update spawns hostiles for sc, the scenario of the level this GameMaster runs.
// A nil sc spawns nothing.
func (gm *GameMaster) Update(sc *scenario.Scenario) {
	if gm.suppressSpawns || sc == nil {
		return
	}

	hostileCount := gm.hostiles()

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

// hostiles counts the level's hostile bodies toward hostile_max: every
// scripted or faction creature and every settled hive body, wild or settled
// (hive_plan.md), never the player's own (phase F: a player hive's drones
// would otherwise throttle the scenario's waves).
func (gm *GameMaster) hostiles() int {
	n := 0
	for _, e := range gm.level.Entities {
		if !e.HasComponent(rlcomponents.Dead) &&
			(e.HasComponent(components.FactionAI) || e.HasComponent(components.ScriptedAI) || hiveWorker(e)) &&
			!gm.players(e) {
			n++
		}
	}
	return n
}

// players reports whether e is one of the player's own bodies.
func (gm *GameMaster) players(e *ecs.Entity) bool {
	return gm.player != nil && ownedBySettlement(e, gm.player())
}

// ownedBySettlement reports whether e belongs to settlement name: a worker of
// it, or a kin body (egg, juvenile, larva) it laid. "" owns nothing.
func ownedBySettlement(e *ecs.Entity, name string) bool {
	if e == nil || name == "" {
		return false
	}
	if e.HasComponent(components.Settlement) && e.GetComponent(components.Settlement).(*components.SettlementComponent).Name == name {
		return true
	}
	k := components.KinOf(e)
	return k != nil && k.Settlement == name
}
