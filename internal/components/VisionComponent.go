package components

import "github.com/mechanical-lich/mlge/ecs"

// VisibleEntry is one entry in a VisionComponent's visible set. Position is
// captured at the moment the entity was last in FOV, so entries in
// NewlyHidden carry the last-known location (the listener can't see the
// entity's *current* position anymore by definition).
type VisibleEntry struct {
	Entity  *ecs.Entity
	X, Y, Z int
}

// VisionComponent declares that an entity can perceive other entities via
// line of sight and stores its per-turn visible set + diff buffers.
//
// Range is the sight radius in tiles. If 0, the VisionSystem falls back to
// DefaultSightRadius.
//
// NightPenalty is a multiplier applied to Range when the level is dark.
// 1.0 = no penalty (default). 0.5 = halved range at night. 0 is treated as
// "unspecified" → no penalty, so a creature that's truly blind at night
// should use a small positive value (e.g. 0.01).
type VisionComponent struct {
	Range        int     `json:"range,omitempty"`
	NightPenalty float32 `json:"nightPenalty,omitempty"`

	// Runtime — not persisted.
	Visible      map[*ecs.Entity]VisibleEntry `json:"-"`
	NewlyVisible []VisibleEntry               `json:"-"`
	NewlyHidden  []VisibleEntry               `json:"-"`
	// Read pointers so scripts can drain the inboxes once per turn without
	// re-consuming on subsequent calls.
	NewlyVisibleRead int `json:"-"`
	NewlyHiddenRead  int `json:"-"`
}

func (v *VisionComponent) GetType() ecs.ComponentType { return Vision }
