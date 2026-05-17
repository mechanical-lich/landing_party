package campaign

import "math"

// Campaign is the persistent root of a landing_party game: the ship hub, the
// space overworld (Locations), and campaign-wide progress. Only one location's
// level is live at a time; every other location is frozen on disk.
type Campaign struct {
	Name              string               `json:"name"`
	Seed              int64                `json:"seed"`
	CurrentLocationID string               `json:"current_location_id"`
	Locations         map[string]*Location `json:"locations"`
	Ship              *ShipState           `json:"ship"`
	Day               int                  `json:"day"`
}

// NewCampaign builds a fresh campaign from the data-driven overworld
// definition. The first discovered location is the starting location.
func NewCampaign(name string, seed int64, defs []Location, rosterCap int) *Campaign {
	c := &Campaign{
		Name:      name,
		Seed:      seed,
		Locations: make(map[string]*Location, len(defs)),
		Ship:      NewShipState(rosterCap),
	}
	for i := range defs {
		loc := defs[i]
		if loc.Seed == 0 {
			loc.Seed = seed + int64(i+1)
		}
		c.Locations[loc.ID] = &loc
		if c.CurrentLocationID == "" && loc.Discovered {
			c.CurrentLocationID = loc.ID
		}
	}
	return c
}

// CurrentLocation returns the descriptor for the location the landing party is
// currently on, or nil if none is set.
func (c *Campaign) CurrentLocation() *Location {
	if c == nil {
		return nil
	}
	return c.Locations[c.CurrentLocationID]
}

// FuelCost returns the fuel needed to travel from the ship's current location
// to toID: zero if it is the current location, otherwise the star-map distance
// (rounded). Falls back to the static FuelCost when coordinates are absent.
func (c *Campaign) FuelCost(toID string) int {
	to := c.Locations[toID]
	if to == nil {
		return 0
	}
	if toID == c.CurrentLocationID {
		return 0
	}
	from := c.Locations[c.CurrentLocationID]
	if from == nil || (from.X == 0 && from.Y == 0 && to.X == 0 && to.Y == 0) {
		return to.FuelCost
	}
	d := math.Hypot(to.X-from.X, to.Y-from.Y)
	return int(math.Round(d))
}

// DiscoveredLocations returns all locations the player can currently see on the
// star map.
func (c *Campaign) DiscoveredLocations() []*Location {
	var out []*Location
	for _, loc := range c.Locations {
		if loc.Discovered {
			out = append(out, loc)
		}
	}
	return out
}
