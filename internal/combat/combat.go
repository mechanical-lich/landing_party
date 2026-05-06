package combat

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcombat"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/scifi_settlements/internal/world"
)

// MeleeAttack performs a melee attack from attacker toward a tile position.
// Returns true if a target was hit.
func MeleeAttack(level *world.Level, attacker *ecs.Entity, targetX, targetY, targetZ int) bool {
	if attacker == nil || !attacker.HasComponent(rlcomponents.Position) {
		return false
	}
	target := level.GetEntityAt(targetX, targetY, targetZ)
	if target != nil && target.HasComponent(rlcomponents.Health) {
		rlcombat.Hit(level, attacker, target, false)
		return true
	}
	return false
}
