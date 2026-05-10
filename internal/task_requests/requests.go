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
)
