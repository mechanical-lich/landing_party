package systems

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// VisionSystem builds each listener's set of currently visible entities and
// the diff (newly entered / newly left FOV) since their last turn. Uses the
// same Bresenham losCheck the colony FOV uses, but per-listener rather than
// per-tile.
//
// Order requirement: must run before AI systems (FactionAI, ScriptedAI) so
// the visible/diff sets are fresh when scripts query them.
type VisionSystem struct{}

var visionSystemRequires = []ecs.ComponentType{
	rlcomponents.Position,
	components.Vision,
	rlcomponents.MyTurn,
}

func (s *VisionSystem) Requires() []ecs.ComponentType       { return visionSystemRequires }
func (s *VisionSystem) UpdateSystem(data interface{}) error { return nil }

func (s *VisionSystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	if entity.HasComponent(rlcomponents.Dead) {
		return nil
	}
	level := levelInterface.(*world.Level)
	vc := entity.GetComponent(components.Vision).(*components.VisionComponent)
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	sx, sy, sz := pc.GetX(), pc.GetY(), pc.GetZ()

	sightRange := vc.Range
	if sightRange <= 0 {
		sightRange = DefaultSightRadius
	}
	if vc.NightPenalty > 0 && level.IsNight() {
		sightRange = int(float32(sightRange) * vc.NightPenalty)
		if sightRange < 1 {
			sightRange = 1
		}
	}
	r2 := sightRange * sightRange

	if vc.Visible == nil {
		vc.Visible = make(map[*ecs.Entity]components.VisibleEntry)
	}

	// Build current visible set. Use a fresh map so the diff is a clean
	// set-difference; could ping-pong two maps later if this shows up in a
	// profile.
	current := make(map[*ecs.Entity]components.VisibleEntry, len(vc.Visible))
	for _, e := range level.Entities {
		// TODO - Make this more efficient
		if e == nil || e == entity {
			continue
		}
		if e.HasComponent(rlcomponents.Dead) {
			continue
		}
		if !e.HasComponent(rlcomponents.Position) {
			continue
		}
		ep := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
		ex, ey, ez := ep.GetX(), ep.GetY(), ep.GetZ()
		// Same-z only for v1. Vertical sight comes with floor/ceiling rules
		// the colony FOV already encodes; revisit when needed.
		if ez != sz {
			continue
		}
		dx := ex - sx
		dy := ey - sy
		if dx*dx+dy*dy > r2 {
			continue
		}
		if !losCheck(level, sx, sy, ex, ey, sz) {
			continue
		}
		current[e] = components.VisibleEntry{Entity: e, X: ex, Y: ey, Z: ez}
	}

	// Diff: NewlyVisible = current \ prev; NewlyHidden = prev \ current.
	vc.NewlyVisible = vc.NewlyVisible[:0]
	vc.NewlyVisibleRead = 0
	for e, entry := range current {
		if _, was := vc.Visible[e]; !was {
			vc.NewlyVisible = append(vc.NewlyVisible, entry)
		}
	}
	vc.NewlyHidden = vc.NewlyHidden[:0]
	vc.NewlyHiddenRead = 0
	for e, oldEntry := range vc.Visible {
		if _, still := current[e]; !still {
			// Preserve the position from when we last saw them.
			vc.NewlyHidden = append(vc.NewlyHidden, oldEntry)
		}
	}

	vc.Visible = current
	return nil
}
