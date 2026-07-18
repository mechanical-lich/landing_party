package combat

import (
	"github.com/mechanical-lich/landing_party/internal/audio"
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/effect"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcombat"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// notifyCombatMusic switches to combat music when a colonist is on either side
// of a strike (attacking or attacked).
func notifyCombatMusic(a, b *ecs.Entity) {
	if (a != nil && a.HasComponent(components.Worker)) ||
		(b != nil && b.HasComponent(components.Worker)) {
		audio.NotifyCombat()
	}
}

// MeleeAttack performs a melee attack from attacker toward a tile position.
// Returns true if a target was hit.
func MeleeAttack(level *world.Level, attacker *ecs.Entity, targetX, targetY, targetZ int) bool {
	if attacker == nil || !attacker.HasComponent(rlcomponents.Position) {
		return false
	}
	target := level.GetEntityAt(targetX, targetY, targetZ)
	if target != nil && target.HasComponent(rlcomponents.Health) {
		rlcombat.Hit(level, attacker, target, false)
		notifyCombatMusic(attacker, target)
		// One impact at the point of contact, composed from the striker's weight
		// and the target's surface (weapon/armor can override either). Falls back
		// to the generic impact clip when the set isn't loaded.
		impactKey := components.ImpactClipKey(attacker, target)
		level.EmitSoundClip(targetX, targetY, targetZ, 6, world.SoundTagImpact, impactKey, attacker)
		level.EmitScent(targetX, targetY, targetZ, world.SmellTagBlood, 3.0)
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

	// The ranged weapon's "shoot" sound if it defines one, else the attacker's,
	// else the generic gunshot clip.
	shoot := components.SoundClipOf(weaponEntity, components.SoundShoot)
	if shoot == "" {
		shoot = components.ResolveSound(attacker, components.SoundShoot)
	}
	level.EmitSoundClip(pc.GetX(), pc.GetY(), pc.GetZ(), 20, world.SoundTagGunshot, shoot, attacker)

	target := level.GetEntityAt(targetX, targetY, targetZ)
	if target != nil && target.HasComponent(rlcomponents.Health) {
		rlcombat.Hit(level, attacker, target, false)
		notifyCombatMusic(attacker, target)
		// Projectile impact at the target — distinct from the shot at the shooter.
		impactKey := components.ImpactClipKey(attacker, target)
		level.EmitSoundClip(targetX, targetY, targetZ, 6, world.SoundTagImpact, impactKey, attacker)
		level.EmitScent(targetX, targetY, targetZ, world.SmellTagBlood, 3.0)
	}
	return true
}
