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
// stopped ScriptedAI mobs (e.g. mutants) from being un-orderable.
func IsAttackTarget(e *ecs.Entity) bool {
	return e != nil && e.HasComponent(rlcomponents.Health) &&
		(e.HasComponent(FactionAI) || e.HasComponent(ScriptedAI) || e.HasComponent(rlcomponents.HostileAI))
}
