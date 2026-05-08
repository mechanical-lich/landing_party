package combat

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcombat"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/scifi_settlements/internal/components"
	"github.com/mechanical-lich/scifi_settlements/internal/effect"
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

// Shoot fires a ranged attack from attacker using the given weapon entity.
// Uses a laser beam effect if the weapon has a LaserBeamComponent, otherwise a sprite projectile.
// Returns true if in range and an attack was made.
func Shoot(level *world.Level, attacker *ecs.Entity, targetX, targetY, targetZ int, weaponEntity *ecs.Entity) bool {
	if attacker == nil || !attacker.HasComponent(rlcomponents.Position) {
		return false
	}
	if !weaponEntity.HasComponent(rlcomponents.Weapon) {
		return false
	}
	weapon := weaponEntity.GetComponent(rlcomponents.Weapon).(*rlcomponents.WeaponComponent)
	pc := attacker.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	if !rlcombat.IsInArrowPath(pc.GetX(), pc.GetY(), targetX, targetY, weapon.Range) {
		return false
	}

	if weaponEntity.HasComponent(components.LaserBeam) {
		lb := weaponEntity.GetComponent(components.LaserBeam).(*components.LaserBeamComponent)
		effect.GetEffectManager().AddEffect(effect.NewLaserEffect(
			pc.GetX(), pc.GetY(), targetX, targetY, lb.Length, lb.R, lb.G, lb.B,
		))
	} else {
		effect.GetEffectManager().AddEffect(effect.NewArrowEffect(
			pc.GetX(), pc.GetY(), pc.GetZ(),
			targetX, targetY, targetZ,
			weapon.ProjectileResource, weapon.ProjectileX, weapon.ProjectileY,
		))
	}

	target := level.GetEntityAt(targetX, targetY, targetZ)
	if target != nil && target.HasComponent(rlcomponents.Health) {
		rlcombat.Hit(level, attacker, target, false)
	}
	return true
}
