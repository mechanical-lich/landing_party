package components

import "github.com/mechanical-lich/mlge/ecs"

// MaterialComponent marks an entity as a material — a stackable substance
// that workers can carry, craft with, and store. Items without this component
// occupy one slot each and cannot stack.
//
// Tags drive both crafting (recipes can match by tag instead of blueprint ID)
// and storage filtering (containers accept items whose tags overlap
// AllowedTags / FilterTags).
type MaterialComponent struct {
	Tags     []string `json:"Tags,omitempty"`
	Quantity int      `json:"Quantity"`
	MaxStack int      `json:"MaxStack,omitempty"` // 0 or absent → non-stackable (1 per slot)
}

func (m *MaterialComponent) GetType() ecs.ComponentType { return Material }

// IsStackable reports whether this material supports multiple units per slot.
func (m *MaterialComponent) IsStackable() bool { return m.MaxStack >= 2 }

// HasTag reports whether this material carries the given tag.
func (m *MaterialComponent) HasTag(tag string) bool {
	for _, t := range m.Tags {
		if t == tag {
			return true
		}
	}
	return false
}
