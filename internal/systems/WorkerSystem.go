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

	switch aiMemory.State {
	case "idle":
		ai.HandleWorkerIdleState(level, entity)
	case "task":
		ai.HandleTaskState(level, entity)
	case "dropoff":
		ai.HandleDropOffState(level, entity)
	case "gather_materials":
		ai.HandleGatherMaterialsState(level, entity)
	}

	return nil
}
