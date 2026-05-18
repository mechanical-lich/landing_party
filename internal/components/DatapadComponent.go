package components

import "github.com/mechanical-lich/mlge/ecs"

// DatapadComponent marks an item as a datapad carrying a clue: when a colonist
// recovers it the bound (hidden) quest is unlocked into the quest log.
type DatapadComponent struct {
	QuestID string
}

func (c *DatapadComponent) GetType() ecs.ComponentType { return Datapad }
