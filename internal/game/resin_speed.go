package game

import (
	"math"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlsystems"
	"github.com/mechanical-lich/mlge/ecs"
)

// Tile speed (docs/brainstorm/hive_plan.md, "Resin"): a floor tile may carry
// a speed map, multiplying the initiative refill of a walking entity standing
// on it by its faction ("*" for every other). Hive resin quickens the hive's
// walkers and slows everyone else's; fliers ignore it. The rogue library's
// InitiativeSystem is unchanged: tileInitiative wraps it and rescales the
// counter just after it refills.

// tileInitiative is the initiative system with the tile speed applied.
type tileInitiative struct{ *rlsystems.InitiativeSystem }

func (t tileInitiative) UpdateEntity(levelInterface interface{}, entity *ecs.Entity) error {
	ic := entity.GetComponent(rlcomponents.Initiative).(*rlcomponents.InitiativeComponent)
	refills := ic.Ticks-t.Speed <= 0
	if err := t.InitiativeSystem.UpdateEntity(levelInterface, entity); err != nil {
		return err
	}
	if refills {
		if lvl, ok := levelInterface.(*world.Level); ok {
			if f := tileSpeed(lvl, entity); f != 1 {
				ic.Ticks = max(1, int(math.Round(float64(ic.Ticks)*f)))
			}
		}
	}
	return nil
}

// tileSpeed is the refill multiplier for entity on the floor under it: 1
// off any speed tile, for fliers, eggs and anything that does not walk.
func tileSpeed(lvl *world.Level, e *ecs.Entity) float64 {
	if e.HasComponent(rlcomponents.Inanimate) || !e.HasComponent(rlcomponents.Position) {
		return 1
	}
	if strings.HasSuffix(e.Blueprint, "_egg") || e.Blueprint == "hive_larva" {
		return 1
	}
	if k := components.KinOf(e); k != nil && k.Morph == structureMorph {
		return 1
	}
	if e.HasComponent(components.Skills) && e.GetComponent(components.Skills).(*components.SkillsComponent).Has("flying") {
		return 1
	}
	pc := e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent)
	t := lvl.GetTilePtr(pc.GetX(), pc.GetY(), pc.GetZ())
	if t == nil || t.Floor.IsEmpty() {
		return 1
	}
	sp := world.TileDefinitions[t.Floor.Type].Speed
	if len(sp) == 0 {
		return 1
	}
	if key := bodyDoctrine(e); key != "" {
		if f, ok := sp[key]; ok {
			return f
		}
	}
	if f, ok := sp["*"]; ok {
		return f
	}
	return 1
}

// bodyDoctrine is the doctrine e's people run, the key a tile's speed table
// uses: its settlement's (a player hive's drones run the hive doctrine under
// the player's faction key), else its faction's (a wild xeno's "hive").
func bodyDoctrine(e *ecs.Entity) string {
	if e.HasComponent(components.Settlement) {
		if key := doctrineOfSettlement(e.GetComponent(components.Settlement).(*components.SettlementComponent).Name); key != "" {
			return key
		}
	}
	if e.HasComponent(rlcomponents.Description) {
		if f := e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction; f != "" {
			return doctrineOfFaction(f)
		}
	}
	return ""
}
