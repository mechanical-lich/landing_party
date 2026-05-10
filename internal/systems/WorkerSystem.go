package systems

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/scifi_settlements/internal/ai"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
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
					wc.CurrentTask.Stop()
					wc.CurrentTask = nil
				}
			}
			aiMemory.TargetX = -1
			aiMemory.TargetY = -1
			aiMemory.State = "findfood"
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
		ai.HandleGatherMaterialsCraftState(level, entity)
	case "findfood":
		ai.HandleFindFood(level, entity)
	case "haul":
		ai.HandleHaulState(level, entity)
	}

	return nil
}
