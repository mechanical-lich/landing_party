package campaign

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

	"github.com/mechanical-lich/landing_party/internal/history"
	"github.com/mechanical-lich/landing_party/internal/research"
)

// Interception (phase D; the plan's Tier 3, "Interception";
// docs/developer/factions.md, "Interception"). A jump whose line passes
// within range of a faction ship in flight, or that arrives where a
// mothership (or a ship docked at it) is in orbit, meets that ship. The
// player answers: evade (fuel, and it may fail), board (a fight on the
// ship's own deck: an encounter location built from crashed-ship sections),
// or parley (a neutral faction only). Every outcome is drawn from the
// fleet's seed and the day, so the same campaign answers the same way.

// EncounterKind is an encounter location's Kind: a boarded ship.
const EncounterKind = "encounter"

// Encounter answers.
const (
	AnswerEvade  = "evade"
	AnswerBoard  = "board"
	AnswerParley = "parley"
)

// InterceptRules is factions.json's intercept block.
type InterceptRules struct {
	Range         float64   `json:"range"`
	SamplesPerDay int       `json:"samples_per_day"`
	EvadeFuel     []int     `json:"evade_fuel"`
	CatchChance   []float64 `json:"catch_chance"`
}

// Encounter is a faction ship met in space.
type Encounter struct {
	FleetID  string `json:"fleet_id"`
	Faction  string `json:"faction"`
	Doctrine string `json:"doctrine"`
	// At is "crossing" (met in flight) or "orbit" (waiting at the
	// destination); LocationID the jump's destination; X, Y where it was.
	At         string           `json:"at"`
	LocationID string           `json:"location_id"`
	X          float64          `json:"x"`
	Y          float64          `json:"y"`
	Day        history.StarDate `json:"day"`
	// Tier is the ship's tier, -1 a mothership's (the rules' last entry).
	Tier       int  `json:"tier"`
	Mothership bool `json:"mothership,omitempty"`
	// Answer and Outcome once answered: Outcome is "evaded", "caught" (an
	// evasion that failed: boarded), "boarded" or "parleyed".
	Answer  string `json:"answer,omitempty"`
	Outcome string `json:"outcome,omitempty"`
	// FuelCost is what the answer cost (an evasion's fuel).
	FuelCost int `json:"fuel_cost,omitempty"`
	// BoardingID is the encounter location the boarding party lands on.
	BoardingID string `json:"boarding_id,omitempty"`
}

// Pending reports an encounter awaiting an answer.
func (e *Encounter) Pending() bool { return e != nil && e.Answer == "" }

// ruleIndex is the encounter's index into the rules' per-tier lists.
func (e *Encounter) ruleIndex(n int) int {
	if n == 0 {
		return 0
	}
	if e.Mothership {
		return n - 1
	}
	return min(max(e.Tier, 0), n-1)
}

// EvadeFuel is the fuel an evasion costs.
func (e *Encounter) EvadeFuel() int {
	r := LoadFactionData().Intercept
	if len(r.EvadeFuel) == 0 {
		return 0
	}
	return r.EvadeFuel[e.ruleIndex(len(r.EvadeFuel))]
}

// CanParley reports whether parley is offered: the faction is neutral.
func (c *Campaign) CanParley(e *Encounter) bool {
	return e != nil && c.Stance(e.Faction) == StanceNeutral
}

// DetectInterception is the ship a jump from fromID to toID starting on
// day and lasting days meets, nil for none: a ship already waiting in orbit
// of the destination (a mothership, or a ship docked at it) first, else the
// first ship in fleet order whose path comes within range of the jump's
// line at any sample (samples_per_day a day, positions by star time). Eggs
// are never met; landed ships are on the ground.
func (c *Campaign) DetectInterception(fromID, toID string, day history.StarDate, days int) *Encounter {
	from, to := c.Locations[fromID], c.Locations[toID]
	if from == nil || to == nil || fromID == toID {
		return nil
	}
	r := LoadFactionData().Intercept
	if r.Range <= 0 {
		return nil
	}
	for i := range c.Fleets {
		f := &c.Fleets[i]
		if f.Kind == FleetShip && !f.Landed() && f.ToID == toID && (f.Mothership || f.FromID == toID) {
			return c.encounterWith(f, "orbit", toID, to.X, to.Y, day)
		}
	}
	samples := max(days, 1) * max(r.SamplesPerDay, 1)
	for i := range c.Fleets {
		f := &c.Fleets[i]
		if f.Kind != FleetShip || f.Landed() || f.Mothership || f.FromID == f.ToID {
			continue
		}
		for k := 0; k <= samples; k++ {
			share := float64(k) / float64(samples)
			t := float64(day) + share*float64(max(days, 1))
			px, py := from.X+(to.X-from.X)*share, from.Y+(to.Y-from.Y)*share
			fx, fy := f.Pos(t)
			if math.Hypot(fx-px, fy-py) <= max(r.Range+interceptRange(f.Presence.KnownTechs), 0) {
				return c.encounterWith(f, "crossing", toID, fx, fy, day)
			}
		}
	}
	return nil
}

func (c *Campaign) encounterWith(f *Fleet, at, toID string, x, y float64, day history.StarDate) *Encounter {
	f.Known = true // met face to face
	return &Encounter{
		FleetID: f.ID, Faction: f.Presence.Faction, Doctrine: f.Presence.Doctrine,
		At: at, LocationID: toID, X: x, Y: y, Day: day,
		Tier: f.ShipTier, Mothership: f.Mothership,
	}
}

// encounterRNG is the draw for an encounter: the fleet's seed and the day.
func (c *Campaign) encounterRNG(e *Encounter) *rand.Rand {
	seed := c.Seed*7919 + int64(e.Day)*104729
	if f := c.fleetByID(e.FleetID); f != nil {
		seed += f.Presence.Seed * 1000003
	}
	return rand.New(rand.NewSource(seed))
}

func (c *Campaign) fleetByID(id string) *Fleet {
	for i := range c.Fleets {
		if c.Fleets[i].ID == id {
			return &c.Fleets[i]
		}
	}
	return nil
}

// AnswerEncounter resolves the pending encounter with answer: an evasion
// costs EvadeFuel and is caught with the tier's catch_chance (then the ship
// is boarded anyway); a boarding builds the encounter location (the caller
// travels there); a parley (neutral only) lets both pass. It records a
// Legends line and returns the encounter. Deterministic for a campaign,
// fleet and day.
func (c *Campaign) AnswerEncounter(answer string) (*Encounter, error) {
	e := c.Encounter
	if !e.Pending() {
		return nil, fmt.Errorf("no encounter awaits an answer")
	}
	f := c.fleetByID(e.FleetID)
	d := LoadFactionData().Doctrine(e.Doctrine)
	if f == nil || d == nil {
		c.Encounter = nil
		return nil, fmt.Errorf("the ship is gone")
	}
	loc := c.Locations[e.LocationID]
	rng := c.encounterRNG(e)
	r := LoadFactionData().Intercept
	switch answer {
	case AnswerEvade:
		e.Answer, e.FuelCost = answer, e.EvadeFuel()
		chance := 0.0
		if len(r.CatchChance) > 0 {
			chance = r.CatchChance[e.ruleIndex(len(r.CatchChance))]
		}
		chance += interceptCatch(f.Presence.KnownTechs) // the greys' tractor lock
		if rng.Float64() >= chance {
			e.Outcome = "evaded"
			c.recordFaction("faction_evaded", loc, nil, d, nil)
			return e, nil
		}
		e.Outcome = "caught"
	case AnswerBoard:
		e.Answer, e.Outcome = answer, "boarded"
	case AnswerParley:
		if !c.CanParley(e) {
			return nil, fmt.Errorf("the %s will not talk", d.MorphAt(nil).Name)
		}
		e.Answer, e.Outcome = answer, "parleyed"
		c.recordFaction("faction_parley", loc, nil, d, nil)
		return e, nil
	default:
		return nil, fmt.Errorf("unknown answer %q", answer)
	}
	// Boarded (or caught): the ship's deck becomes a location.
	b := c.newBoarding(e, f, d)
	e.BoardingID = b.ID
	c.recordFaction("faction_boarded", loc, nil, d, map[string]string{"ship": b.Name})
	return e, nil
}

// Boarding is what an encounter location holds: the boarded fleet.
type Boarding struct {
	FleetID  string `json:"fleet_id"`
	Faction  string `json:"faction"`
	Doctrine string `json:"doctrine"`
	Class    string `json:"class"` // crashed_ship's: scout, freighter, warship, mothership
	Crew     int    `json:"crew"`
	Built    bool   `json:"built,omitempty"`
	Taken    bool   `json:"taken,omitempty"` // every crew member dead
}

// newBoarding adds the boarded ship's deck as a location beside where it
// was met: Kind encounter, map and scenario "boarding", its fleet held
// (Fleet.BoardedAt) while the player is aboard.
func (c *Campaign) newBoarding(e *Encounter, f *Fleet, d *Doctrine) *Location {
	c.EncounterSeq++
	class := "scout"
	if f.Mothership {
		class = "mothership"
	} else if n := strings.ToLower(d.TierName(f.ShipTier)); n != "" {
		class = n
	}
	name := fmt.Sprintf("%s %s", d.MorphAt(nil).Singular, class)
	name = strings.ToUpper(name[:1]) + name[1:]
	id := fmt.Sprintf("enc%d", c.EncounterSeq)
	loc := &Location{
		ID: id, Name: name, Kind: EncounterKind, MapID: "boarding", ScenarioID: "boarding",
		Seed: f.Presence.Seed*31 + int64(c.EncounterSeq), X: e.X + 2, Y: e.Y + 2,
		Identified: true, Visited: true,
		Boarding: &Boarding{FleetID: f.ID, Faction: f.Presence.Faction, Doctrine: f.Presence.Doctrine,
			Class: class, Crew: max(f.Presence.Population, 1)},
	}
	c.Locations[id] = loc
	f.BoardedAt = id
	return loc
}

// ReconcileBoarding counts a boarded ship's crew back (alive on its deck):
// the fleet's crew is what is left, and none left is the ship taken: the
// fleet is destroyed (a mothership by DestroyMothership's rule) and its
// hold stays aboard as loot. It reports whether the ship was taken now.
func (c *Campaign) ReconcileBoarding(loc *Location, alive int) bool {
	b := loc.Boarding
	if b == nil || b.Taken || !b.Built {
		return false
	}
	f := c.fleetByID(b.FleetID)
	if alive > 0 {
		if f != nil {
			f.Presence.Population = alive
		}
		return false
	}
	b.Taken = true
	d := LoadFactionData().Doctrine(b.Doctrine)
	if f != nil && f.Mothership {
		c.DestroyMothership(b.Faction, loc)
	} else if f != nil {
		c.RemoveFleet(f.ID)
		if d != nil {
			c.recordFaction("faction_ship_taken", loc, nil, d, nil)
		}
	}
	return true
}

// ReleaseBoarding lets a boarded ship go on (with whoever is left) when
// the player leaves its deck (the level freezes) without taking it. It
// reports whether it did: the deck then left with the ship.
func (c *Campaign) ReleaseBoarding(loc *Location) bool {
	if b := loc.Boarding; b != nil && !b.Taken {
		if f := c.fleetByID(b.FleetID); f != nil && f.BoardedAt == loc.ID {
			f.BoardedAt = ""
		}
		return true
	}
	return false
}

// RemoveFleet drops fleet id.
func (c *Campaign) RemoveFleet(id string) {
	kept := c.Fleets[:0]
	for _, f := range c.Fleets {
		if f.ID != id {
			kept = append(kept, f)
		}
	}
	c.Fleets = kept
	if len(c.Fleets) == 0 {
		c.Fleets = nil
	}
}

// interceptCatch is what a ship's known develop entries add to the player's
// chance of being caught evading it (the greys' tractor lock; Tier 3 only).
func interceptCatch(techs []string) float64 {
	v := 0.0
	for _, k := range techs {
		if t, ok := research.GetTech(k); ok && t.Develop != nil && t.Develop.Intercept != nil {
			v += t.Develop.Intercept.CatchChance
		}
	}
	return v
}

// interceptRange is what a ship's known develop entries add to the range a
// jump meets it in flight at (the greys' cloak: negative; Tier 3 only).
func interceptRange(techs []string) float64 {
	v := 0.0
	for _, k := range techs {
		if t, ok := research.GetTech(k); ok && t.Develop != nil && t.Develop.Intercept != nil {
			v += t.Develop.Intercept.Range
		}
	}
	return v
}
