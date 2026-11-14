package task_requests

import (
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/task"
)

type BuildRequest struct {
	Type     string
	X        int
	Y        int
	Z        int
	Progress int
	Required int
	// Delivered is what has already been carried to the task site and consumed
	// from a worker's bag, blueprint -> quantity. The task's cost is satisfied
	// when Delivered covers it.
	Delivered map[string]int
	// Replace repairs: the Repairable entity at the tile gives way to the
	// built one, and sets its flag (a station's generator).
	Replace bool
	// Waited is the turns the done work has waited on someone else standing
	// in the footprint; past maxFootprintWait the builder lets the task go
	// back to the queue (materials kept) rather than wait forever on a
	// worker that never moves (a rival's sentry).
	Waited int `json:",omitempty"`
}

// ConvertRequest turns the tile at X, Y, Z into Tile (a floor tile: the
// conversion economy, wire_placer style) over Required ticks of work (phase
// C2, ConvertAction). The worker stands on or beside it.
type ConvertRequest struct {
	X        int
	Y        int
	Z        int
	Tile     string
	Progress int
	Required int
	// Rate multiplies the worker's work on it (0 or 1: none): the
	// settlement's doctrine's conversion rate from its known techs (the
	// hive's Resin Flow, the autonz' Lattice Weave), as the abstract step
	// reads it.
	Rate float64 `json:",omitempty"`
}

// DeployRequest replaces the worker with the As structure on X, Y, Z once
// it has stood there Required work steps, when the tile's floor is one of
// Floor (none: any). The autonz' relay drone deploying a relay pylon
// (autonz_plan.md).
type DeployRequest struct {
	X        int
	Y        int
	Z        int
	As       string
	Floor    []string
	Progress int
	Required int
}

type DigRequest struct {
	X        int
	Y        int
	Z        int
	Progress int
	Required int
}

type MineRequest struct {
	X        int
	Y        int
	Z        int
	Progress int
	Required int
	// Target is set when mining a Choppable entity (e.g. alien_crystal)
	// instead of a tile. nil means "mine the tile at (X,Y,Z)".
	Target *ecs.Entity
}

type ResearchRequest struct {
	TechKey  string
	Building string
	Progress int
	Required int
	// Delivered is what has already been carried to the task site and consumed
	// from a worker's bag, blueprint -> quantity. The task's cost is satisfied
	// when Delivered covers it.
	Delivered map[string]int
}

type CraftRequest struct {
	RecipeID   string
	WorkbenchX int
	WorkbenchY int
	WorkbenchZ int
	Progress   int
	Required   int
	// Delivered is what has already been carried to the task site and consumed
	// from a worker's bag, blueprint -> quantity. The task's cost is satisfied
	// when Delivered covers it.
	Delivered map[string]int
}

type EquipRequest struct {
	ItemBlueprint string // blueprint name of item to equip
	StorageX      int
	StorageY      int
	StorageZ      int
}

type UnequipRequest struct {
	Slot     string // e.g. "hand", "head", "torso"
	StorageX int
	StorageY int
	StorageZ int
}

type RetrieveRequest struct {
	Item *ecs.Entity // specific item entity to retrieve; nil check at pickup time
	// Dest, when set, forces the drop-off target after pickup instead of the
	// default "first accepting container" search. Used by the Store order so
	// players can dictate exactly which locker an item lands in.
	Dest *ecs.Entity
}

// RelocateRequest moves Qty units of Blueprint from Source to a destination —
// either a specific storage container (DestEntity) or a ground tile
// (DestEntity == nil, position in DestX/Y/Z). Worker phases: walk to source,
// withdraw into bag (Carried tracks the actually-withdrawn amount in case the
// source ran short), walk to destination, deposit.
type RelocateRequest struct {
	Source     *ecs.Entity
	Blueprint  string
	Qty        int
	DestEntity *ecs.Entity // nil → ground drop at DestX/Y/Z
	DestX      int
	DestY      int
	DestZ      int
	PickedUp   bool // true once material is in the worker's bag
	Carried    int  // actual quantity in bag (may be < Qty if source was short)
}

type SleepRequest struct {
	X        int
	Y        int
	Z        int
	Progress int // turns slept in current heal cycle
	Required int // turns per heal cycle (typically 10)
	// Mount state: PrevX/Y/Z is where the worker stood before climbing onto the
	// bed; used to dismount back to that tile when the task completes.
	OnBed bool
	PrevX int
	PrevY int
	PrevZ int
}

// PassoutRequest is used when a colonist collapses from exhaustion with no bed
// available. Recovery is 4× slower than a proper bed.
type PassoutRequest struct {
	Progress int // turns into current recovery cycle
	Required int // turns per recovery cycle (typically 40)
	// Bedroll is true when the rest is a proper sleep in a sleeping bag (in
	// place, bed-quality recovery) rather than an exhaustion collapse.
	Bedroll bool
}

// FilterableAction pairs a task action with the label shown in the colonist UI.
type FilterableAction struct {
	Action task.TaskAction
	Label  string
}

// FilterableActions is the ordered list of task types players can toggle per colonist.
var FilterableActions = []FilterableAction{
	{DigAction, "Dig"},
	{MineAction, "Mine"},
	{BuildAction, "Build"},
	{CraftAction, "Craft"},
	{ResearchAction, "Research"},
	{RetrieveAction, "Retrieve"},
	{AttackAction, "Combat"},
}

const (
	PickupAction   task.TaskAction = "pickup"
	BuildAction    task.TaskAction = "build"
	DigAction      task.TaskAction = "dig"
	MineAction     task.TaskAction = "mine"
	ForageAction   task.TaskAction = "forage"
	AttackAction   task.TaskAction = "attack"
	ResearchAction task.TaskAction = "research"
	CraftAction    task.TaskAction = "craft"
	EquipAction    task.TaskAction = "equip"
	UnequipAction  task.TaskAction = "unequip"
	RetrieveAction task.TaskAction = "retrieve"
	SleepAction    task.TaskAction = "sleep"
	PassoutAction  task.TaskAction = "passout"
	RelocateAction task.TaskAction = "relocate"
	ConvertAction  task.TaskAction = "convert"
	// DeployAction: a unit becomes a structure where it stands
	// (DeployRequest).
	DeployAction task.TaskAction = "deploy"
	// MoveAction walks a worker to the task's tile (the default handler):
	// a director's post, patrol waypoint or the walk to the hive's spire.
	MoveAction task.TaskAction = "move"
	// MoveHereAction is the player's ground click (no action: the worker
	// walks there and does what the tile offers). A worker with an explicit
	// task list (a type's, the player's filters) still takes it
	// (workerai's task pick).
	MoveHereAction task.TaskAction = ""
)

// HoldHere as a move task's Data: the worker stays on arrival until the task
// is completed by whoever gave it.
const HoldHere = "hold"

// IsInterruptibleAction reports whether the NeedsSystem may preempt a task
// with the given action when a colonist becomes too exhausted.
func IsInterruptibleAction(action task.TaskAction) bool {
	switch action {
	case AttackAction, SleepAction, PassoutAction, EquipAction, UnequipAction:
		return false
	}
	return true
}
