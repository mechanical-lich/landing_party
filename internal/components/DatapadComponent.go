package components

import "github.com/mechanical-lich/mlge/ecs"

// DatapadComponent marks an item as a datapad carrying a clue: when a colonist
// recovers it the bound (hidden) quest is unlocked into the quest log, and the
// history event it records (EventID) becomes known and is read out.
type DatapadComponent struct {
	QuestID string
	EventID string `json:",omitempty"`
	// Text, when set, is what the datapad says (a clue a structure left);
	// otherwise it reads as its event's chronicle line.
	Text string `json:",omitempty"`
}

func (c *DatapadComponent) GetType() ecs.ComponentType { return Datapad }
