package task_requests

import "github.com/mechanical-lich/mlge/task"

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
)
