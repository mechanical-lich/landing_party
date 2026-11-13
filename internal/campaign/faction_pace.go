package campaign

import (
	"fmt"
	"sort"
	"strings"
)

// Ship pace (greyish_plan.md, step 0): how a fleet faction's ships stand on
// a day, for history-dump's detail lines and the greyish pace golden test.
// Read-only: nothing here changes a campaign.

// Ship states, as ShipState reports them.
const (
	ShipLanded     = "landed"
	ShipFlying     = "flying"
	ShipReturning  = "returning"
	ShipDocked     = "docked" // waiting in orbit of its mothership: nowhere free to go
	ShipMothership = "mothership"
	ShipBoarded    = "boarded"
)

// ShipState is a ship's state: landed, flying, returning (bound for its
// mothership with its hold), docked (waiting at the mothership with nowhere
// to go), a mothership, or held by a boarding. "" for a fleet that is no ship.
func (c *Campaign) ShipState(f *Fleet) string {
	switch {
	case f.Kind != FleetShip:
		return ""
	case f.Mothership:
		return ShipMothership
	case f.BoardedAt != "":
		return ShipBoarded
	case f.Landed():
		return ShipLanded
	case f.Returning:
		if ms := c.Mothership(f.Presence.Faction); ms != nil && f.FromID == ms.ToID && f.ToID == ms.ToID {
			return ShipDocked
		}
		return ShipReturning
	}
	return ShipFlying
}

// SpineTechs is how many of the doctrine's own research walk (its
// `research` list: the spine) techs knows. Techs from a second walk are not
// counted, so a pace table built on it does not move when a second walk is
// added (greyish_plan.md, "Tier 1: two walks").
func (d *Doctrine) SpineTechs(techs []string) int {
	n := 0
	for _, t := range d.Research {
		if hasStr(techs, t.ID) {
			n++
		}
	}
	return n
}

// ShipLine is one ship of a fleet faction for a pace table: tier, state,
// crew, hold units and spine techs ("freighter landed crew 5 hold 12
// techs 2"); a mothership shows its hoard ("mothership crew 10 hoard
// crystal 10, metal_ore 105 techs 3").
func (c *Campaign) ShipLine(d *Doctrine, f *Fleet) string {
	p := &f.Presence
	if f.Mothership {
		return fmt.Sprintf("mothership crew %d hoard %s techs %d", p.Population, stockLine(p.Stockpile), d.SpineTechs(p.KnownTechs))
	}
	tier := d.TierName(f.ShipTier)
	if tier == "" {
		tier = "ship"
	}
	return fmt.Sprintf("%s %s crew %d hold %d techs %d", tier, c.ShipState(f), p.Population, holdUnits(p.Stockpile), d.SpineTechs(p.KnownTechs))
}

// ShipFleets are faction's ships (its mothership first), by fleet ID
// number.
func (c *Campaign) ShipFleets(faction string) []*Fleet {
	var out []*Fleet
	for i := range c.Fleets {
		if f := &c.Fleets[i]; f.Kind == FleetShip && f.Presence.Faction == faction {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Mothership != out[b].Mothership {
			return out[a].Mothership
		}
		return fleetNum(out[a]) < fleetNum(out[b])
	})
	return out
}

// stockLine is a stockpile as "k v, k v" in key order ("-" for none).
func stockLine(stock map[string]int) string {
	keys := make([]string, 0, len(stock))
	for k, v := range stock {
		if v != 0 {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return "-"
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s %d", k, stock[k])
	}
	return strings.Join(parts, ", ")
}
