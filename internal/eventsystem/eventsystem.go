package eventsystem

import (
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/mlge/ecs"
	"github.com/mechanical-lich/mlge/event"
)

const (
	EntityDied     event.EventType = "EntityDied"
	EntityKilled   event.EventType = "EntityKilled"
	StructureBuilt event.EventType = "StructureBuilt"
	ItemStored     event.EventType = "ItemStored"
	ResearchDone   event.EventType = "ResearchDone"
	EntityDamaged  event.EventType = "EntityDamaged"
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

// EntityDamagedEvent fires on the level's own bus when a combat hit takes health
// off an entity. Amount is the health actually lost.
type EntityDamagedEvent struct {
	Victim   *ecs.Entity
	Attacker *ecs.Entity
	Amount   int
	Level    *world.Level
}

func (e EntityDamagedEvent) GetType() event.EventType { return EntityDamaged }
