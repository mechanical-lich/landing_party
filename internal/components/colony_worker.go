package components

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// IsColonyWorker reports whether e is a living member of the player's workforce on
// the level whose settlement is named colony (a location's colony, or the ship's
// settlement). Worker alone doesn't say that: colonists and robots qualify, but
// hostiles can carry Worker to use the task system, under their own settlement.
func IsColonyWorker(e *ecs.Entity, colony string) bool {
	if e == nil || !e.HasComponents(Worker, Settlement) || e.HasComponent(rlcomponents.Dead) {
		return false
	}
	return e.GetComponent(Settlement).(*SettlementComponent).Name == colony
}
