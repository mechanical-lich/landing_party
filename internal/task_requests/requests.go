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
}

type CraftRequest struct {
	RecipeID   string
	WorkbenchX int
	WorkbenchY int
	WorkbenchZ int
	Progress   int
	Required   int
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
)

// IsInterruptibleAction reports whether the NeedsSystem may preempt a task
// with the given action when a colonist becomes too exhausted.
func IsInterruptibleAction(action task.TaskAction) bool {
	switch action {
	case AttackAction, SleepAction, PassoutAction, EquipAction, UnequipAction:
		return false
	}
	return true
}
