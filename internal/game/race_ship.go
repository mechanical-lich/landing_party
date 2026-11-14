package game

import (
	"log"
	"sort"

	"github.com/mechanical-lich/landing_party/internal/campaign"
	"github.com/mechanical-lich/landing_party/internal/factory"
	"github.com/mechanical-lich/landing_party/internal/ship"
	"github.com/mechanical-lich/landing_party/internal/world"
	"github.com/mechanical-lich/ml-rogue-lib/pkg/rlcomponents"
	"github.com/mechanical-lich/mlge/ecs"
)

// A ship race (phase F step 6; playable_races_plan.md, "Step 6"): the greys
// fly a ship as the colony does, with a crew of their own types under the
// player's faction key, and the doctrine's modules aboard from the start.

// seedRaceCrew spawns a ship race's start crew aboard: race.start.units by
// type (in type order), each a worker of the ship's settlement under the
// player's faction with its type applied and its race.start.gear equipped;
// then the race.start.aboard structures by the hull centre.
func (wm *WorldManager) seedRaceCrew(lvl *world.Level, st *campaign.RaceStart, shipName string, cx, cy, cz int) {
	c := wm.Campaign
	d := campaign.PlayerDoctrine(c)
	faction := playerFactionKey(c)
	notePresenceTechs(c.Player.ID, c.KnownTechs())
	types := make([]string, 0, len(st.Units))
	for typ := range st.Units {
		types = append(types, typ)
	}
	sort.Strings(types)
	for _, typ := range types {
		dt := d.Types[typ]
		if dt == nil || dt.Blueprint == "" {
			log.Printf("[race] %s start: no type %q", d.Name, typ)
			continue
		}
		for i := 0; i < st.Units[typ]; i++ {
			e, err := factory.Create(dt.Blueprint, 0, 0, 0)
			if err != nil {
				continue
			}
			x, y, z := freeLandingTile(lvl, cx, cy, cz)
			if e.HasComponent(rlcomponents.Position) {
				e.GetComponent(rlcomponents.Position).(*rlcomponents.PositionComponent).SetPosition(x, y, z)
			}
			if e.HasComponent(rlcomponents.Description) {
				e.GetComponent(rlcomponents.Description).(*rlcomponents.DescriptionComponent).Faction = faction
			}
			tagColonistSettlement(e, shipName)
			factory.ApplyType(e, typ, c.Player.ID)
			equipStart(e, st.Gear[typ])
			lvl.AddEntity(e)
		}
	}
	for i, key := range st.Aboard {
		placeHiveThing(lvl, key, c.Player.ID, shipName, cx-3+3*i, cy+3, cz)
	}
}

// equipStart gives e the items and equips the best of them.
func equipStart(e *ecs.Entity, items []string) {
	if len(items) == 0 || !e.HasComponent(rlcomponents.Inventory) {
		return
	}
	inv := e.GetComponent(rlcomponents.Inventory).(*rlcomponents.InventoryComponent)
	for _, bp := range items {
		if it, err := factory.Create(bp, 0, 0, 0); err == nil {
			inv.AddItem(it)
		}
	}
	inv.EquipAllBest()
}

// buildShipLevelFor is a fresh ship level in the player doctrine's layout:
// the colony's grid of rooms, or a saucer (the greys, phase F step 6c).
func buildShipLevelFor(c *campaign.Campaign, owner string) *world.Level {
	if sp := playerSettlementOf(c).SaucerShip(); sp != nil {
		return ship.BuildSaucerLevel(owner, ship.Saucer{Radius: sp.Start.Radius, Decks: sp.Start.Decks, Floor: sp.Floor, Wall: sp.Wall})
	}
	return ship.BuildShipLevel(owner)
}
