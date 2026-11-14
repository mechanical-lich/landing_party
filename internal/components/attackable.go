package components

import (
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// IsAttackTarget reports whether an entity is a creature a colonist can be
// ordered (or bump) to attack: it has Health and is driven by a creature AI —
// faction (raiders), scripted (mutants, zombies, wildlife), or the rogue-lib
// hostile AI. Colonists, buildings, and items lack those AI components, so they
// are excluded. This is the single definition of "attackable enemy" shared by
// the click-to-attack order and the rogue bump-attack; keeping them in sync
// stopped ScriptedAI mobs (e.g. mutants) from being un-orderable. A worker of
// a faction hostile to the player is one too (hostility.go).
func IsAttackTarget(e *ecs.Entity) bool {
	return e != nil && e.HasComponent(rlcomponents.Health) &&
		(e.HasComponent(FactionAI) || e.HasComponent(ScriptedAI) || e.HasComponent(rlcomponents.HostileAI)) ||
		IsHostileWorker(e)
}

// FriendlyToPlayer reports a faction of the player's own group
// (rival_selection_plan.md, "Groups"): set by the game from the campaign;
// nil is none.
var FriendlyToPlayer func(faction string) bool

// IsFriendlyToPlayer reports an entity of a faction in the player's group.
func IsFriendlyToPlayer(e *ecs.Entity) bool {
	if e == nil || FriendlyToPlayer == nil || !e.HasComponent(rlcomponents.Description) {
		return false
	}
	f := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction
	return f != "" && FriendlyToPlayer(f)
}

// IsDefaultAttackTarget is what a plain click, a hover hint, a rogue bump
// and the map's red dot treat as an enemy: an attack target that is not of
// the player's group. The explicit Attack order may still target a friend.
func IsDefaultAttackTarget(e *ecs.Entity) bool {
	return IsAttackTarget(e) && !IsFriendlyToPlayer(e)
}
