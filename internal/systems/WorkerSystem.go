package systems

import (
	"github.com/mechanical-lich/landing_party/internal/ai"
	"github.com/mechanical-lich/landing_party/internal/combat"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

type WorkerSystem struct{}

var workerSystemRequires = []ecs.ComponentType{rlcomponents.Position, components.Worker, rlcomponents.MyTurn}

func (s *WorkerSystem) Requires() []ecs.ComponentType { return workerSystemRequires }

func (s *WorkerSystem) UpdateSystem(data interface{}) error { return nil }

func (s *WorkerSystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	level := levelInterface.(*world.Level)

	if entity.HasComponent(rlcomponents.Dead) {
		return nil
	}

	raw := entity.GetComponent(rlcomponents.AIMemory)
	if raw == nil {
		return nil
	}
	aiMemory := raw.(*rlcomponents.AIMemoryComponent)
	if aiMemory.State == "" {
		aiMemory.State = "idle"
		return nil
	}

	// Player-controlled colonists run no autonomous AI; they act only on
	// direct player input handled in the game state.
	if wc := entity.GetComponent(components.Worker); wc != nil {
		if wc.(*components.WorkerComponent).RogueControlled {
			return nil
		}
	}

	if wc := entity.GetComponent(components.Worker); wc != nil {
		workerC := wc.(*components.WorkerComponent)
		if workerC.SwapCooldown > 0 {
			workerC.SwapCooldown--
		}
	}

	// Hunger: drain energy, seek food when hungry, take damage when starving
	if entity.HasComponent(components.Hunger) {
		hg := entity.GetComponent(components.Hunger).(*components.HungerComponent)
		if hg.Tick() {
			if hg.Energy > 0 {
				hg.Energy--
			}
		}
		if hg.IsStarving() && entity.HasComponent(rlcomponents.Health) {
			hc := entity.GetComponent(rlcomponents.Health).(*rlcomponents.HealthComponent)
			hc.Health--
		}
		if hg.IsHungry() && aiMemory.State != "findfood" {
			if entity.HasComponent(components.Worker) {
				wc := entity.GetComponent(components.Worker).(*components.WorkerComponent)
				if wc.CurrentTask != nil {
					wc.CurrentTask.ReQueue()
					wc.CurrentTask = nil
				}
			}
			aiMemory.TargetX = -1
			aiMemory.TargetY = -1
			aiMemory.State = "findfood"
		}
	}

	// Self-defense: counter-attack whoever just hit this worker, then return.
	if wc := entity.GetComponent(components.Worker); wc != nil {
		workerC := wc.(*components.WorkerComponent)
		if workerC.SelfDefend && aiMemory.Attacked {
			selfPC := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
			attacker := level.GetSolidEntityAt(aiMemory.AttackerX, aiMemory.AttackerY, selfPC.GetZ())
			aiMemory.Attacked = false
			if attacker != nil && attacker.HasComponent(rlcomponents.Health) {
				combat.MeleeAttack(level, entity, aiMemory.AttackerX, aiMemory.AttackerY, selfPC.GetZ())
				return nil
			}
		}
	}

	switch aiMemory.State {
	case "idle":
		ai.HandleWorkerIdleState(level, entity)
	case "task":
		ai.HandleTaskState(level, entity)
	case "dropoff":
		ai.HandleDropOffState(level, entity)
	case "gather_materials":
		ai.HandleGatherMaterialsState(level, entity)
	case "gather_materials_craft":
		// No longer used; redirect to task state
		aiMemory.State = "task"
	case "findfood":
		ai.HandleFindFood(level, entity)
	case "haul":
		ai.HandleHaulState(level, entity)
	}

	return nil
}
