package components

import "github.com/mechanical-lich/mlge/ecs"

// PoweredComponent ties an entity to a station's power grid (station_plan.md,
// section 10): while the level flag "power:<Grid>" is unset its light is held
// dark and, with Unlock, its door held locked; once the flag is set
// StationPowerSystem restores the light to Level/Range, unlocks the door and
// removes this component.
type PoweredComponent struct {
	Grid   string
	Level  int
	Range  int
	Unlock bool
}

func (c *PoweredComponent) GetType() ecs.ComponentType { return Powered }

// RepairableComponent marks a broken machine colonists can repair: the
// player orders a build of Into at its tile (the build's cost and time are
// Into's hidden buildable), which replaces it and sets the level flag
// SetsFlag (a generator's "power:main").
type RepairableComponent struct {
	Into     string
	SetsFlag string
}

func (c *RepairableComponent) GetType() ecs.ComponentType { return Repairable }
