package eventsystem

import "github.com/mechanical-lich/mlge/event"

const (
	EntityDied     event.EventType = "EntityDied"
	EntityKilled   event.EventType = "EntityKilled"
	TaskCompleted  event.EventType = "TaskCompleted"
	StructureBuilt event.EventType = "StructureBuilt"
	ItemStored     event.EventType = "ItemStored"
	ResearchDone   event.EventType = "ResearchDone"
)

type EntityDiedEvent struct {
	EntityName string
	Faction    string
	Blueprint  string
}

func (e EntityDiedEvent) GetType() event.EventType { return EntityDied }

type EntityKilledEvent struct {
	KillerName string
	TargetName string
	Blueprint  string
	X, Y, Z    int
}

func (e EntityKilledEvent) GetType() event.EventType { return EntityKilled }

type TaskCompletedEvent struct {
	WorkerName string
	Action     string
	Message    string
}

func (e TaskCompletedEvent) GetType() event.EventType { return TaskCompleted }

type StructureBuiltEvent struct {
	Type       string
	Settlement string
	X, Y, Z    int
}

func (e StructureBuiltEvent) GetType() event.EventType { return StructureBuilt }

type ItemStoredEvent struct {
	ItemName   string
	Blueprint  string
	Settlement string
}

func (e ItemStoredEvent) GetType() event.EventType { return ItemStored }

type ResearchDoneEvent struct {
	TechKey    string
	Settlement string
}

func (e ResearchDoneEvent) GetType() event.EventType { return ResearchDone }
