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
	InteractTicks  int
	LastTaskAction task.TaskAction
	// AllowedTasks lists the task types this colonist will accept.
	// nil or empty means no filter — all task types are allowed.
	AllowedTasks []task.TaskAction
	// SwapCooldown prevents a displaced colonist from being swapped again
	// immediately, avoiding oscillation in tight corridors.
	SwapCooldown int
	// DropOffItem is the specific inventory item to deposit when entering the
	// "dropoff" state. Cleared after deposit.
	DropOffItem *ecs.Entity
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
