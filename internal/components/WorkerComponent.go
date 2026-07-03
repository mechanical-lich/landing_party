package components

import (
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

type WorkerComponent struct {
	Role        string
	CurrentTask *task.Task
	// InteractTicks holds the worker at an interaction (pickup/dropoff) for a
	// few ticks so its work animation has time to play. The handlers increment
	// it while at the target and reset it on completion.
	InteractTicks int
	// ApproachFailTicks counts consecutive ticks a worker has failed to advance
	// toward an equip/unequip task's storage container. When it crosses the
	// cancel threshold the task is abandoned (with a message) so a worker whose
	// storage is walled off doesn't stay pinned to it forever. Reset on progress.
	ApproachFailTicks int
	LastTaskAction    task.TaskAction
	// AvailableTasks is the blueprint-level cap on what this worker is ever
	// capable of doing — the universe of toggles shown in the colonist
	// modal, AND the upper bound enforced at task-pick time. Empty = no
	// cap (all FilterableActions are available). Robots set this to scope
	// their chassis (e.g. excavator -> dig, mine); colonists leave it empty.
	AvailableTasks []task.TaskAction
	// AllowedTasks is the player's current per-worker filter — a subset of
	// AvailableTasks (or of all FilterableActions when AvailableTasks is
	// empty). nil means "no per-worker filter set"; for capped workers the
	// effective accept list defaults to AvailableTasks in that case.
	AllowedTasks []task.TaskAction
	// SwapCooldown prevents a displaced colonist from being swapped again
	// immediately, avoiding oscillation in tight corridors.
	SwapCooldown int
	// WorkCarry accumulates the sub-unit remainder of stat-scaled task progress
	// (see workerai.workStep). A worker advancing a task at e.g. 1.3 steps/tick
	// banks the 0.3 here so fractional speed bonuses aren't lost to truncation.
	WorkCarry float64
	// DropOffItem is the specific inventory item to deposit when entering the
	// "dropoff" state. Cleared after deposit.
	DropOffItem *ecs.Entity
	// DropOffDestEntity, when non-nil, is the specific storage container the
	// dropoff state must deposit into. When nil the destination is a ground
	// tile at (DropOffX, DropOffY, DropOffZ); the item is placed on the tile
	// (and Worker-bearing entities are deployed into the settlement).
	DropOffDestEntity *ecs.Entity
	DropOffX          int
	DropOffY          int
	DropOffZ          int
	// SelfDefend causes the worker to counter-attack when struck, if they have
	// not already used their turn this tick. Defaults to true.
	SelfDefend bool
	// RogueControlled is true while the player is directly puppeting this
	// colonist in Rogue mode. The worker AI is skipped entirely; the colonist
	// only acts in response to player input.
	RogueControlled bool
	// RogueExtract holds transient dig/mine progress while the player bumps a
	// tile in Rogue mode. Reset when the target tile changes or is cleared.
	RogueExtract *RogueExtractProgress
}

// RogueExtractProgress tracks how far a player-controlled dig or mine has
// progressed against a specific tile.
type RogueExtractProgress struct {
	X, Y, Z  int
	Progress int
	Required int
	Mining   bool // true = mine (ore), false = dig
}

func (w *WorkerComponent) GetType() ecs.ComponentType { return Worker }

// IsTaskAvailable reports whether action is in the worker's chassis cap.
// Empty AvailableTasks means "no cap" — every action is available.
func (w *WorkerComponent) IsTaskAvailable(action task.TaskAction) bool {
	if len(w.AvailableTasks) == 0 {
		return true
	}
	for _, a := range w.AvailableTasks {
		if a == action {
			return true
		}
	}
	return false
}
