package gui

import (
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
)

const (
	BuildOptionChangedEventType event.EventType = "build_option_changed"
	CursorModeChangedEventType  event.EventType = "cursor_mode_changed"
	EntitySelectedEventType     event.EventType = "entity_selected"
	MainMenuEventType           event.EventType = "main_menu_event"
)

type BuildOptionChangedEvent struct{ Option string }

func (e BuildOptionChangedEvent) GetType() event.EventType { return BuildOptionChangedEventType }

type CursorModeChangedEvent struct{ Mode CursorModeType }

func (e CursorModeChangedEvent) GetType() event.EventType { return CursorModeChangedEventType }

type EntitySelectedEvent struct{ Entity *ecs.Entity }

func (e EntitySelectedEvent) GetType() event.EventType { return EntitySelectedEventType }

type MainMenuEvent struct{ Action string }

func (e MainMenuEvent) GetType() event.EventType { return MainMenuEventType }

const SaveGameEventType event.EventType = "save_game_event"

type SaveGameEvent struct{ Name string }

func (e SaveGameEvent) GetType() event.EventType { return SaveGameEventType }

const LoadGameEventType event.EventType = "load_game_event"

type LoadGameEvent struct{ Name string }

func (e LoadGameEvent) GetType() event.EventType { return LoadGameEventType }

const CraftRequestedEventType event.EventType = "craft_requested"

type CraftRequestedEvent struct {
	RecipeID string
	Station  *ecs.Entity
}

func (e CraftRequestedEvent) GetType() event.EventType { return CraftRequestedEventType }

const StationClickedEventType event.EventType = "station_clicked"

type StationClickedEvent struct{ Station *ecs.Entity }

func (e StationClickedEvent) GetType() event.EventType { return StationClickedEventType }

const ResearchStationClickedEventType event.EventType = "research_station_clicked"

type ResearchStationClickedEvent struct{ Station *ecs.Entity }

func (e ResearchStationClickedEvent) GetType() event.EventType {
	return ResearchStationClickedEventType
}

const ResearchRequestedEventType event.EventType = "research_requested"

type ResearchRequestedEvent struct {
	TechKey string
	Station *ecs.Entity
}

func (e ResearchRequestedEvent) GetType() event.EventType { return ResearchRequestedEventType }

const ColonistSelectedEventType event.EventType = "colonist_selected"

type ColonistSelectedEvent struct{ Entity *ecs.Entity }

func (e ColonistSelectedEvent) GetType() event.EventType { return ColonistSelectedEventType }

const EquipItemRequestedEventType event.EventType = "equip_item_requested"

type EquipItemRequestedEvent struct {
	ColonistEntity *ecs.Entity
	ItemBlueprint  string
}

func (e EquipItemRequestedEvent) GetType() event.EventType { return EquipItemRequestedEventType }

const UnequipItemRequestedEventType event.EventType = "unequip_item_requested"

type UnequipItemRequestedEvent struct {
	ColonistEntity *ecs.Entity
	Slot           string
}

func (e UnequipItemRequestedEvent) GetType() event.EventType { return UnequipItemRequestedEventType }

const SetTaskFilterEventType event.EventType = "set_task_filter"

// SetTaskFilterEvent is fired when the player toggles a task type on/off for a colonist.
// Action is the string form of task.TaskAction; Enabled=true means allow, false means block.
type SetTaskFilterEvent struct {
	Colonist *ecs.Entity
	Action   string
	Enabled  bool
}

func (e SetTaskFilterEvent) GetType() event.EventType { return SetTaskFilterEventType }

const DropOffRequestedEventType event.EventType = "drop_off_requested"

type DropOffRequestedEvent struct {
	Colonist *ecs.Entity
	Item     *ecs.Entity
}

func (e DropOffRequestedEvent) GetType() event.EventType { return DropOffRequestedEventType }

const SetSelfDefendEventType event.EventType = "set_self_defend"

// SetSelfDefendEvent is fired when the player toggles self-defense on/off for a colonist.
type SetSelfDefendEvent struct {
	Colonist *ecs.Entity
	Enabled  bool
}

func (e SetSelfDefendEvent) GetType() event.EventType { return SetSelfDefendEventType }

const EnterRogueModeEventType event.EventType = "enter_rogue_mode"

// EnterRogueModeEvent is fired when the player chooses to directly control a colonist.
type EnterRogueModeEvent struct{ Entity *ecs.Entity }

func (e EnterRogueModeEvent) GetType() event.EventType { return EnterRogueModeEventType }

const ExitRogueModeEventType event.EventType = "exit_rogue_mode"

// ExitRogueModeEvent is fired when the player exits Rogue mode via the on-screen button.
type ExitRogueModeEvent struct{}

func (e ExitRogueModeEvent) GetType() event.EventType { return ExitRogueModeEventType }
