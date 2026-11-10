package campaign

import "fmt"

// Observer runs (docs/developer/factions.md, "Observer mode"): a New
// Expedition started with observer mode on has no player ship. The crew,
// hold and fuel the player would have flown are a fleet-carried presence of
// the player's doctrine (the colony), the expedition, starting adrift at the
// Wreck Site; it flies by the doctrine's expedition table and its colony is
// run by the settlement director whenever its site is loaded, exactly as a
// rival mirror's. Its techs are its own presence's (KnownTechs), as a
// rival's are; Campaign.Player is left as it is and holds nothing in an
// observer run.

// ExpeditionFaction is the expedition presence's faction key (its doctrine
// is the player's, "colony").
const ExpeditionFaction = "expedition"

// IsExpedition reports whether f carries an observer run's expedition.
func (f *Fleet) IsExpedition() bool { return f != nil && f.Presence.Faction == ExpeditionFaction }

// Expedition is the observer run's expedition fleet, or nil.
func (c *Campaign) Expedition() *Fleet {
	for i := range c.Fleets {
		if c.Fleets[i].IsExpedition() {
			return &c.Fleets[i]
		}
	}
	return nil
}

// AddExpeditionFleet puts the expedition on the Star Map: a ship of the
// player's doctrine at the current location (the Wreck Site), crew strong,
// the hold as its stockpile and fuel in its tanks, seen from the start.
// The table moves it on its first day (a site-less place is left at once).
func (c *Campaign) AddExpeditionFleet(crew int, hold map[string]int, fuel int) (*Fleet, error) {
	d := PlayerDoctrine(c)
	loc := c.CurrentLocation()
	if d == nil || loc == nil {
		return nil, fmt.Errorf("expedition: no doctrine or no start")
	}
	c.FactionSeq++
	c.FleetSeq++
	stock := map[string]int{}
	for k, v := range hold {
		if v > 0 && k != "fuel" {
			stock[k] = v
		}
	}
	key := PlayerDoctrineKey(c)
	c.Fleets = append(c.Fleets, Fleet{
		ID: fmt.Sprintf("fl%d", c.FleetSeq), Kind: FleetShip,
		FromID: loc.ID, ToID: loc.ID, FromX: loc.X, FromY: loc.Y, ToX: loc.X, ToY: loc.Y,
		LaunchDay: c.FactionDay, ArriveDay: c.FactionDay,
		LandedID: loc.ID, LandedDay: c.FactionDay,
		Known: true,
		Presence: FactionPresence{
			ID: fmt.Sprintf("fp%d", c.FactionSeq), Faction: ExpeditionFaction, Doctrine: key,
			Population: crew, Stockpile: stock, Fuel: fuel,
			ArrivedDay: c.FactionDay, LastStepDay: c.FactionDay, LastShotDay: NeverFired,
			Seed: c.Seed*7919 + 17, Known: true,
		},
	})
	// There is no player for it to be hostile to: neutral (the Star Map
	// tag's stance, and the hostility marks).
	if c.Diplomacy == nil {
		c.Diplomacy = map[string]string{}
	}
	c.Diplomacy[ExpeditionFaction] = StanceNeutral
	return &c.Fleets[len(c.Fleets)-1], nil
}
