package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mechanical-lich/landing_party/internal/ai"
	"github.com/mechanical-lich/landing_party/internal/combat"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/effect"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/gui"
	"github.com/mechanical-lich/landing_party/internal/task_requests"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlentity"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/message"
	"github.com/mechanical-lich/mlge/task"
)

// stepWorld advances the simulation by exactly one round. It is the body that
// used to live inline in Update()'s real-time block, factored out so Rogue
// mode can drive it manually.
func (s *MainState) stepWorld() {
	s.tick++
	s.gm.Update()
	s.systemManager.UpdateSystems(s.level)
	for _, entity := range s.level.Entities {
		if entity == nil {
			continue
		}
		if entity.HasComponent(rlcomponents.Inanimate) {
			continue
		}
		s.systemManager.UpdateSystemsForEntity(s.level, entity)
	}
	s.cleanUpSystem.Update(s.level)
	s.collectDatapads()
	effect.GetEffectManager().Update()
	// A quest target may have just died during cleanup — resolve immediately
	// rather than waiting for the periodic tick (which barely advances in
	// Rogue mode), so the quest completes the moment the kill lands.
	if s.forceQuestEval {
		s.forceQuestEval = false
		s.evaluateQuests()
	}
}

// advancePlayerTurn is called after the player commits an action in Rogue
// mode. It advances the world by the controlled colonist's full recharge
// period so every other entity recharges by exactly that amount and faster
// entities get proportionally more turns.
func (s *MainState) advancePlayerTurn() {
	rounds := 1
	if s.rogueEntity != nil && s.rogueEntity.HasComponent(rlcomponents.Initiative) {
		ic := s.rogueEntity.GetComponent(rlcomponents.Initiative).(*rlcomponents.InitiativeComponent)
		period := ic.DefaultValue
		if ic.OverrideValue > 0 {
			period = ic.OverrideValue
		}
		speed := s.initiativeSystem.Speed
		if speed < 1 {
			speed = 1
		}
		rounds = period / speed
		if rounds < 1 {
			rounds = 1
		}
	}
	for i := 0; i < rounds; i++ {
		s.stepWorld()
	}
}

// enterRogueMode hands direct control of the given colonist to the player.
func (s *MainState) enterRogueMode(ent *ecs.Entity) {
	if ent == nil || !ent.HasComponent(components.Worker) || !ent.HasComponent(rlcomponents.Position) {
		return
	}
	wc := ent.GetComponent(components.Worker).(*components.WorkerComponent)
	if wc.CurrentTask != nil && !wc.CurrentTask.Completed {
		wc.CurrentTask.ReQueue()
	}
	wc.CurrentTask = nil
	wc.RogueControlled = true
	wc.RogueExtract = nil
	if ent.HasComponent(rlcomponents.AIMemory) {
		ent.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent).State = "idle"
	}

	s.rogueEntity = ent
	s.followEntity = ent
	s.CursorMode = gui.CursorModeRogue
	s.guiManager.CloseModal("colonistModal")
	s.guiManager.SetSidebarVisible(false)
	s.guiManager.ShowRogueExit()
	s.guiManager.SetSelectionLabel("Rogue Mode — "+rlentity.GetName(ent),
		"WASD move/attack/dig.  Click to fire.  X to exit.")
	message.AddMessage("Now controlling " + rlentity.GetName(ent) + ".")
}

// exitRogueMode returns the controlled colonist to autonomous work.
func (s *MainState) exitRogueMode() {
	if s.rogueEntity != nil && s.rogueEntity.HasComponent(components.Worker) {
		wc := s.rogueEntity.GetComponent(components.Worker).(*components.WorkerComponent)
		wc.RogueControlled = false
		wc.RogueExtract = nil
		if s.rogueEntity.HasComponent(rlcomponents.AIMemory) {
			s.rogueEntity.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent).State = "idle"
		}
		message.AddMessage(rlentity.GetName(s.rogueEntity) + " resumed normal duties.")
	}
	s.rogueEntity = nil
	s.followEntity = nil
	s.CursorMode = gui.CursorModeDefault
	s.guiManager.SetSidebarVisible(true)
	s.guiManager.HideRogueExit()
	s.guiManager.SetSelectionLabel("", "")
}

// handleRogueKeys processes player input while a colonist is controlled.
func (s *MainState) handleRogueKeys() {
	if s.rogueEntity == nil {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyX) {
		s.exitRogueMode()
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		if s.roguePickup() {
			s.advancePlayerTurn()
		}
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		if s.rogueStair(1) {
			s.advancePlayerTurn()
		}
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		if s.rogueStair(-1) {
			s.advancePlayerTurn()
		}
		return
	}
	dx, dy := 0, 0
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyW):
		dy = -1
	case inpututil.IsKeyJustPressed(ebiten.KeyS):
		dy = 1
	case inpututil.IsKeyJustPressed(ebiten.KeyA):
		dx = -1
	case inpututil.IsKeyJustPressed(ebiten.KeyD):
		dx = 1
	}
	if dx == 0 && dy == 0 {
		return
	}
	if s.rogueAct(dx, dy) {
		s.advancePlayerTurn()
	}
}

// rogueAct resolves a directional command into move / bump-attack / dig / mine
// against the destination tile. Returns true if a turn was consumed.
func (s *MainState) rogueAct(dx, dy int) bool {
	ent := s.rogueEntity
	pc := ent.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	tx, ty, tz := pc.GetX()+dx, pc.GetY()+dy, pc.GetZ()

	// Bump to attack: hostile entity with health on the destination tile.
	if target := s.level.GetEntityAt(tx, ty, tz); target != nil &&
		target.HasComponent(rlcomponents.Health) &&
		(target.HasComponent(components.FactionAI) || target.HasComponent(rlcomponents.HostileAI)) {
		rlentity.Face(ent, dx, dy)
		combat.MeleeAttack(s.level, ent, tx, ty, tz)
		return true
	}

	// Bump to dig / mine: solid Middle slot on the destination tile.
	tileI := s.level.GetTileAt(tx, ty, tz)
	if tileI != nil {
		tile := tileI.(*world.Tile)
		if !tile.Middle.IsEmpty() {
			def := world.TileDefinitions[tile.Middle.Type]
			mining := def.Name == "ore_deposit" || def.Name == "crystal_vein" || def.Name == "radioactive_ore"
			if mining || def.Solid {
				rlentity.Face(ent, dx, dy)
				s.rogueExtract(tx, ty, tz, tile, mining)
				return true
			}
		}
	}

	// Otherwise move — swapping with a friendly colonist if one is in the way.
	ent.GetComponent(components.Worker).(*components.WorkerComponent).RogueExtract = nil
	if s.rogueSwap(tx, ty, tz) {
		rlentity.Face(ent, dx, dy)
		return true
	}
	rlentity.HandleMovement(s.level, ent, dx, dy, 0)
	return true
}

// rogueSwap exchanges positions with a friendly same-settlement colonist
// occupying the destination tile, mirroring the autonomous colonist swap.
// Returns true if a swap happened.
func (s *MainState) rogueSwap(tx, ty, tz int) bool {
	ent := s.rogueEntity
	blocker := s.level.GetSolidEntityAt(tx, ty, tz)
	if blocker == nil || blocker == ent {
		return false
	}
	if !blocker.HasComponent(components.Worker) || !blocker.HasComponent(components.Settlement) ||
		!ent.HasComponent(components.Settlement) {
		return false
	}
	if ent.GetComponent(components.Settlement).(*components.SettlementComponent).Name !=
		blocker.GetComponent(components.Settlement).(*components.SettlementComponent).Name {
		return false
	}
	pc := ent.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	myX, myY, myZ := pc.GetX(), pc.GetY(), pc.GetZ()
	s.level.PlaceEntity(tx, ty, tz, ent)
	s.level.PlaceEntity(myX, myY, myZ, blocker)
	if blocker.HasComponent(rlcomponents.AIMemory) {
		blocker.GetComponent(rlcomponents.AIMemory).(*rlcomponents.AIMemoryComponent).CurrentSteps = nil
	}
	return true
}

// roguePickup picks up an item lying on the controlled colonist's tile.
// Returns true if an item was taken (a turn was consumed).
func (s *MainState) roguePickup() bool {
	ent := s.rogueEntity
	if !ent.HasComponent(rlcomponents.Inventory) {
		return false
	}
	pc := ent.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	item := s.level.GetEntityAt(pc.GetX(), pc.GetY(), pc.GetZ())
	if item == nil || !item.HasComponent(rlcomponents.Item) {
		return false
	}
	ai.PickupItemFromTile(s.level, ent, pc.GetX(), pc.GetY(), pc.GetZ())
	return true
}

// rogueStair moves the controlled colonist up (dz>0) or down (dz<0) a level,
// but only while standing on a matching stair tile. Returns true if a turn
// was consumed.
func (s *MainState) rogueStair(dz int) bool {
	ent := s.rogueEntity
	pc := ent.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	tileI := s.level.GetTileAt(pc.GetX(), pc.GetY(), pc.GetZ())
	if tileI == nil {
		return false
	}
	tile := tileI.(*world.Tile)
	if tile.Middle.IsEmpty() {
		return false
	}
	def := world.TileDefinitions[tile.Middle.Type]
	if dz > 0 && !def.StairsUp {
		return false
	}
	if dz < 0 && !def.StairsDown {
		return false
	}
	rlentity.HandleMovement(s.level, ent, 0, 0, dz)
	return true
}

// rogueExtract accumulates dig/mine progress against a tile, clearing it (and
// dropping ore when mining) once enough bumps have landed.
func (s *MainState) rogueExtract(tx, ty, tz int, tile *world.Tile, mining bool) {
	wc := s.rogueEntity.GetComponent(components.Worker).(*components.WorkerComponent)
	pc := s.rogueEntity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)

	rp := wc.RogueExtract
	if rp == nil || rp.X != tx || rp.Y != ty || rp.Z != tz || rp.Mining != mining {
		required := 5
		if mining {
			required = 50
		}
		rp = &components.RogueExtractProgress{X: tx, Y: ty, Z: tz, Required: required, Mining: mining}
		wc.RogueExtract = rp
	}
	rp.Progress++

	// Mining yields ore every 10 bumps so partial work isn't wasted.
	if mining && rp.Progress%10 == 0 {
		var blueprint string
		switch world.TileDefinitions[tile.Middle.Type].Name {
		case "ore_deposit":
			blueprint = "metal_ore"
		case "crystal_vein":
			blueprint = "crystal"
		case "radioactive_ore":
			blueprint = "radioactive_material"
		}
		if blueprint != "" {
			if ore, err := factory.Create(blueprint, pc.GetX(), pc.GetY(), pc.GetZ()); err == nil {
				s.level.AddEntity(ore)
				if s.MainSettlement != nil {
					s.MainSettlement.Tasks.AddTask(&task.Task{
						Action: task_requests.RetrieveAction,
						Data:   task_requests.RetrieveRequest{Item: ore},
						X:      pc.GetX(), Y: pc.GetY(), Z: pc.GetZ(),
					})
				}
			}
		}
	}

	if rp.Progress >= rp.Required {
		tile.Radiation = 0
		s.level.ClearMiddle(tx, ty, tz)
		s.level.InvalidateSunColumn(tx, ty)
		wc.RogueExtract = nil
		if mining {
			message.AddMessage("Mined out deposit.")
		} else {
			message.AddMessage("Dug out tile.")
		}
	}
}

// rogueFire fires the controlled colonist's equipped ranged weapon at a tile.
func (s *MainState) rogueFire(tx, ty int) {
	ent := s.rogueEntity
	if !ent.HasComponent(rlcomponents.Inventory) {
		return
	}
	pc := ent.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	inv := ent.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	var weapon *ecs.Entity
	for _, item := range []*ecs.Entity{inv.LeftHand, inv.RightHand} {
		if item != nil && item.HasComponent(rlcomponents.Weapon) {
			if item.GetComponent(rlcomponents.Weapon).(*rlcomponents.WeaponComponent).Ranged {
				weapon = item
				break
			}
		}
	}
	if weapon == nil {
		return
	}
	rlentity.Face(ent, tx-pc.GetX(), ty-pc.GetY())
	if combat.Shoot(s.level, ent, tx, ty, pc.GetZ(), weapon) {
		s.advancePlayerTurn()
	}
}
