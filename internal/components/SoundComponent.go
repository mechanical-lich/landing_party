package components

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// Sound event names. An entity's SoundComponent maps these to audio clip keys;
// equipped gear can override them (see ResolveSound).
const (
	SoundHit     = "hit"     // reacting to being struck (flesh/armor)
	SoundHitting = "hitting" // landing a melee blow (fist/weapon)
	SoundShoot   = "shoot"   // firing a ranged weapon
	SoundDeath   = "death"   // dying
	SoundAlert   = "alert"   // noticing an enemy
)

// SoundComponent maps an entity's (or equippable item's) sound events to audio
// clip keys — the keys the audio system resolves against data/audio.json. On an
// actor these are its default noises; on an item they override the wearer's for
// the events the item cares about (armor → "hit", a weapon → "hitting", a ranged
// weapon → "shoot").
type SoundComponent struct {
	Sounds map[string]string `json:"Sounds"` // event name → clip key

	// Material is this entity's surface for impact sounds when it's struck
	// (e.g. "metal", "wood", "soft"). Gear can override it (metal armor makes a
	// colonist ring). Empty falls back to a generic impact.
	Material string `json:"Material,omitempty"`
	// Weight is how hard this entity/weapon hits ("heavy", "medium", "light"),
	// selecting the impact's force. On a weapon it overrides the wielder's.
	// Empty falls back to "medium".
	Weight string `json:"Weight,omitempty"`
}

func (s *SoundComponent) GetType() ecs.ComponentType { return Sound }

// Clip returns the clip key this component defines for an event, or "".
func (s *SoundComponent) Clip(event string) string {
	if s == nil || s.Sounds == nil {
		return ""
	}
	return s.Sounds[event]
}

// SoundClipOf returns the clip an entity's own SoundComponent defines for an
// event (no gear lookup), or "".
func SoundClipOf(e *ecs.Entity, event string) string {
	if e == nil || !e.HasComponent(Sound) {
		return ""
	}
	return e.GetComponent(Sound).(*SoundComponent).Clip(event)
}

// ResolveSound returns the clip key for an entity's sound event, letting
// equipped gear override the entity's own default: armor can change "hit", a
// held weapon "hitting", a ranged weapon "shoot". Equipment is checked first
// (gear wins); the entity's own SoundComponent is the fallback. Returns "" if
// nothing defines the event — callers then fall back to a generic tag sound.
func ResolveSound(entity *ecs.Entity, event string) string {
	return resolveSoundAttr(entity, func(sc *SoundComponent) string { return sc.Clip(event) })
}

// ResolveMaterial returns the impact surface an entity presents when struck,
// with gear overriding its innate material (metal armor → metallic hit). "" if
// undeclared.
func ResolveMaterial(entity *ecs.Entity) string {
	return resolveSoundAttr(entity, func(sc *SoundComponent) string { return sc.Material })
}

// ResolveWeight returns how hard an entity strikes, with an equipped weapon
// overriding the wielder's innate weight. "" if undeclared.
func ResolveWeight(entity *ecs.Entity) string {
	return resolveSoundAttr(entity, func(sc *SoundComponent) string { return sc.Weight })
}

// resolveSoundAttr reads a SoundComponent field, checking equipped gear first
// (gear wins) then the entity's own component, and returns the first non-empty.
func resolveSoundAttr(entity *ecs.Entity, get func(*SoundComponent) string) string {
	if entity == nil {
		return ""
	}
	if entity.HasComponent(rlcomponents.Inventory) {
		inv := entity.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
		for _, item := range []*ecs.Entity{inv.RightHand, inv.LeftHand, inv.Head, inv.Torso, inv.Legs, inv.Feet} {
			if item == nil || !item.HasComponent(Sound) {
				continue
			}
			if v := get(item.GetComponent(Sound).(*SoundComponent)); v != "" {
				return v
			}
		}
	}
	if entity.HasComponent(Sound) {
		if v := get(entity.GetComponent(Sound).(*SoundComponent)); v != "" {
			return v
		}
	}
	return ""
}

// ImpactClipKey composes the impact sound-set key for a strike:
// impact_{surface}_{weight}. The target's material picks the surface (default
// "generic"); the attacker's weight picks the force (default "medium"). The
// audio layer falls back to the generic impact clip if the set isn't loaded.
func ImpactClipKey(attacker, target *ecs.Entity) string {
	surface := ResolveMaterial(target)
	if surface == "" {
		surface = "generic"
	}
	weight := ResolveWeight(attacker)
	if weight == "" {
		weight = "medium"
	}
	return "impact_" + surface + "_" + weight
}
