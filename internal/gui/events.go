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
