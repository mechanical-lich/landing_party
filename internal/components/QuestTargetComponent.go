package components

import "github.com/mechanical-lich/mlge/ecs"

// QuestTargetComponent marks an entity as the specific target of a quest
// (a named boss, a bounty creature, ...). When such an entity dies, the
// campaign records QuestID as a killed target so the bound quest can complete
// — independent of how many other entities of the same blueprint exist.
type QuestTargetComponent struct {
	QuestID string
}

func (c *QuestTargetComponent) GetType() ecs.ComponentType { return QuestTarget }
