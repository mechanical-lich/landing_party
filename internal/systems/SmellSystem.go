package systems

import (
	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// SmellSystem builds each listener's inbox of "newly perceived" scent tags
// since its last turn. Pulls don't go through this system — scripts query
// level.SmellAt / level.StrongestSmellWithin directly via script funcs.
//
// Order: runs after ScentSystem (so emissions and diffusion are applied
// first) and before AI systems.
type SmellSystem struct{}

var smellSystemRequires = []ecs.ComponentType{
	rlcomponents.Position,
	components.Smell,
	rlcomponents.MyTurn,
}

func (s *SmellSystem) Requires() []ecs.ComponentType       { return smellSystemRequires }
func (s *SmellSystem) UpdateSystem(data interface{}) error { return nil }

const defaultSniffRadius = 3

func (s *SmellSystem) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	if entity.HasComponent(rlcomponents.Dead) {
		return nil
	}
	level := levelInterface.(*world.Level)
	sc := entity.GetComponent(components.Smell).(*components.SmellComponent)
	pc := entity.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	sx, sy, sz := pc.GetX(), pc.GetY(), pc.GetZ()

	radius := sc.SniffRadius
	if radius <= 0 {
		radius = defaultSniffRadius
	}
	r2 := radius * radius

	if sc.LastPerceived == nil {
		sc.LastPerceived = make(map[string]float32)
	}
	sc.NewSmells = sc.NewSmells[:0]
	sc.NewSmellsRead = 0

	// For each tag, find the strongest tile within sniff radius. If it's
	// above sensitivity and was below last turn, it's a fresh perception.
	for tag, tagMap := range level.SmellMap {
		var bestStrength float32
		var bx, by, bz int
		for coord, strength := range tagMap {
			cx, cy, cz := coord.Unpack()
			if cz != sz {
				continue
			}
			dx := cx - sx
			dy := cy - sy
			if dx*dx+dy*dy > r2 {
				continue
			}
			if strength > bestStrength {
				bestStrength = strength
				bx, by, bz = cx, cy, cz
			}
		}
		tagStr := string(tag)
		if bestStrength < sc.Sensitivity {
			// Below threshold this turn — clear the "last perceived" so a
			// future appearance counts as new.
			if _, had := sc.LastPerceived[tagStr]; had {
				delete(sc.LastPerceived, tagStr)
			}
			continue
		}
		last := sc.LastPerceived[tagStr]
		if last < sc.Sensitivity {
			sc.NewSmells = append(sc.NewSmells, components.PerceivedSmell{
				Tag:      tagStr,
				Strength: bestStrength,
				X:        bx,
				Y:        by,
				Z:        bz,
			})
		}
		sc.LastPerceived[tagStr] = bestStrength
	}
	return nil
}
