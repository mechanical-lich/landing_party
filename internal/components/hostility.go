package components

import (
	"sync"

	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// Faction stances toward the player (phase C2, the plan's finding 3;
// docs/developer/factions.md, "Hostility"). A faction settlement's workers
// are colonists of another settlement: they carry no creature AI, so
// nothing treats them as enemies by default. The game marks the factions
// whose stance toward the player is hostile (the campaign's diplomacy), and
// those factions' workers become attack targets for the player's colonists
// (orders, bump attacks, a colonist meeting one on a tile). A neutral
// faction's workers are left alone. Creatures keep their own rules.

var (
	hostileMu       sync.RWMutex
	hostileFactions = map[string]bool{}
)

// SetFactionHostile records whether faction is hostile to the player.
func SetFactionHostile(faction string, hostile bool) {
	hostileMu.Lock()
	defer hostileMu.Unlock()
	if hostile {
		hostileFactions[faction] = true
	} else {
		delete(hostileFactions, faction)
	}
}

// ResetFactionHostility forgets every faction's stance (a new campaign).
func ResetFactionHostility() {
	hostileMu.Lock()
	defer hostileMu.Unlock()
	hostileFactions = map[string]bool{}
}

// IsFactionHostile reports whether faction is marked hostile to the player.
func IsFactionHostile(faction string) bool {
	hostileMu.RLock()
	defer hostileMu.RUnlock()
	return hostileFactions[faction]
}

// IsHostileWorker reports a living worker of a faction hostile to the
// player.
func IsHostileWorker(e *ecs.Entity) bool {
	if e == nil || !e.HasComponents(Worker, rlcomponents.Health, rlcomponents.Description) || e.HasComponent(rlcomponents.Dead) {
		return false
	}
	return IsFactionHostile(e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction)
}

// FactionsHostile reports whether two factions' bodies fight where they
// meet (rival_selection_plan.md, "Groups"): set by the game from the
// campaign's faction groups; nil is "any two different factions".
var FactionsHostile func(a, b string) bool

// HostileFactions is FactionsHostile with its default: two different
// factions are hostile, one faction never is.
func HostileFactions(a, b string) bool {
	if a == b {
		return false
	}
	if FactionsHostile == nil {
		return true
	}
	return FactionsHostile(a, b)
}
