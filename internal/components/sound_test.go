package components

import (
	"testing"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

func entWithSound(m map[string]string) *ecs.Entity {
	e := &ecs.Entity{}
	e.AddComponent(&SoundComponent{Sounds: m})
	return e
}

func TestResolveSoundEntityDefault(t *testing.T) {
	e := entWithSound(map[string]string{"hit": "base_hit", SoundDeath: "base_death"})

	if got := ResolveSound(e, SoundHit); got != "base_hit" {
		t.Fatalf("hit = %q, want base_hit", got)
	}
	if got := ResolveSound(e, SoundDeath); got != "base_death" {
		t.Fatalf("death = %q, want base_death", got)
	}
	if got := ResolveSound(e, "missing"); got != "" {
		t.Fatalf("undefined event = %q, want empty", got)
	}
}

func TestResolveSoundGearOverrides(t *testing.T) {
	armor := entWithSound(map[string]string{SoundHit: "armor_hit"})
	e := &ecs.Entity{}
	e.AddComponent(&SoundComponent{Sounds: map[string]string{SoundHit: "base_hit", SoundDeath: "base_death"}})
	e.AddComponent(&rlcomponents.InventoryComponent{Torso: armor})

	// Equipped armor overrides the wearer's "hit".
	if got := ResolveSound(e, SoundHit); got != "armor_hit" {
		t.Fatalf("hit with armor = %q, want armor_hit (gear override)", got)
	}
	// Events the gear doesn't define fall back to the entity's own default.
	if got := ResolveSound(e, SoundDeath); got != "base_death" {
		t.Fatalf("death = %q, want base_death (gear doesn't define it)", got)
	}
}

func TestResolveSoundNilAndBare(t *testing.T) {
	if got := ResolveSound(nil, SoundHit); got != "" {
		t.Fatalf("nil entity = %q, want empty", got)
	}
	if got := ResolveSound(&ecs.Entity{}, SoundHit); got != "" {
		t.Fatalf("entity without SoundComponent = %q, want empty", got)
	}
}

func TestResolveMaterialAndWeightGearOverride(t *testing.T) {
	// A soft (fleshy) target wearing metal armor rings like metal.
	armor := &ecs.Entity{}
	armor.AddComponent(&SoundComponent{Material: "metal"})
	target := &ecs.Entity{}
	target.AddComponent(&SoundComponent{Material: "soft"})
	target.AddComponent(&rlcomponents.InventoryComponent{Torso: armor})
	if got := ResolveMaterial(target); got != "metal" {
		t.Fatalf("material = %q, want metal (armor overrides flesh)", got)
	}

	// A light attacker swinging a heavy weapon hits heavy.
	weapon := &ecs.Entity{}
	weapon.AddComponent(&SoundComponent{Weight: "heavy"})
	attacker := &ecs.Entity{}
	attacker.AddComponent(&SoundComponent{Weight: "light"})
	attacker.AddComponent(&rlcomponents.InventoryComponent{RightHand: weapon})
	if got := ResolveWeight(attacker); got != "heavy" {
		t.Fatalf("weight = %q, want heavy (weapon overrides wielder)", got)
	}
}

func TestMiningClipKey(t *testing.T) {
	if got := MiningClipKey("metal"); got != "impact_metal_medium" {
		t.Fatalf("metal → %q, want impact_metal_medium", got)
	}
	if got := MiningClipKey("glass"); got != "impact_glass_medium" {
		t.Fatalf("glass → %q, want impact_glass_medium", got)
	}
	// Materials without a medium impact set (or unset) fall back to the generic
	// rock-mining set.
	for _, m := range []string{"", "stone", "rock", "bell", "generic"} {
		if got := MiningClipKey(m); got != "mine" {
			t.Fatalf("MiningClipKey(%q) = %q, want mine", m, got)
		}
	}
}

func TestFootstepClipKey(t *testing.T) {
	if got := FootstepClipKey("grass"); got != "footstep_grass" {
		t.Fatalf("grass → %q, want footstep_grass", got)
	}
	if got := FootstepClipKey("snow"); got != "footstep_snow" {
		t.Fatalf("snow → %q, want footstep_snow", got)
	}
	// Unset/unknown → default wood step.
	for _, m := range []string{"", "metal", "lava"} {
		if got := FootstepClipKey(m); got != "footstep_wood" {
			t.Fatalf("FootstepClipKey(%q) = %q, want footstep_wood", m, got)
		}
	}
}

func TestImpactClipKeyComposition(t *testing.T) {
	attacker := &ecs.Entity{}
	attacker.AddComponent(&SoundComponent{Weight: "heavy"})
	target := &ecs.Entity{}
	target.AddComponent(&SoundComponent{Material: "metal"})
	if got := ImpactClipKey(attacker, target); got != "impact_metal_heavy" {
		t.Fatalf("key = %q, want impact_metal_heavy", got)
	}

	// Undeclared → generic surface + medium weight.
	if got := ImpactClipKey(&ecs.Entity{}, &ecs.Entity{}); got != "impact_generic_medium" {
		t.Fatalf("default key = %q, want impact_generic_medium", got)
	}
}

func TestSoundClipOfSingleEntity(t *testing.T) {
	weapon := entWithSound(map[string]string{SoundShoot: "laser_zap"})
	if got := SoundClipOf(weapon, SoundShoot); got != "laser_zap" {
		t.Fatalf("SoundClipOf shoot = %q, want laser_zap", got)
	}
	if got := SoundClipOf(weapon, SoundHit); got != "" {
		t.Fatalf("SoundClipOf undefined = %q, want empty", got)
	}
	if got := SoundClipOf(nil, SoundShoot); got != "" {
		t.Fatalf("SoundClipOf(nil) = %q, want empty", got)
	}
}
