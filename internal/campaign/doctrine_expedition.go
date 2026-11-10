package campaign

import (
	"fmt"
	"hash/fnv"
	"math"
)

// DoctrineExpedition is the expedition director's table (observer mode v2,
// docs/developer/factions.md, "Observer mode"): what the ship does on the
// Star Map when the game plays itself. internal/director's Expedition reads
// it once every EveryDays star-date days and acts through the player's own
// Star Map functions (travel paying fuel, beam crew and cargo, answer
// incidents and interceptions). The colony doctrine carries it.
type DoctrineExpedition struct {
	// EveryDays is how often it plans (1: once a star-date day).
	EveryDays int `json:"every_days"`
	// FuelReserve is the fuel a jump must leave in the hold.
	FuelReserve int `json:"fuel_reserve"`
	// FoundCrew colonists are beamed down to found a colony on a site
	// scoring at least FoundScore, with Kit (as much as the hold has).
	FoundCrew  int            `json:"found_crew"`
	FoundScore float64        `json:"found_score"`
	Kit        map[string]int `json:"kit,omitempty"`
	// A colony with fewer than ReinforceBelow alive gets crew from the ship
	// up to ReinforceTo; one down to PackUpAt or fewer, with no crew aboard
	// to send, is packed up (its survivors beamed up; 0: never).
	ReinforceBelow int `json:"reinforce_below"`
	ReinforceTo    int `json:"reinforce_to"`
	PackUpAt       int `json:"pack_up_at"`
	// StayDays is how long the ship stays in orbit before it moves on.
	StayDays int `json:"stay_days"`
	// Score rates a site, as a destination and for founding.
	Score ExpeditionScore `json:"score"`
	// IncidentFuel: an incident on a colony is answered by travelling there
	// (crew aboard to reinforce it, the jump at most this fuel and within
	// the reserve); otherwise Ignore.
	IncidentFuel int `json:"incident_fuel"`
	// Encounter is the interception answers to try in order ("parley",
	// "evade", "board"): the first that is possible is given.
	Encounter []string `json:"encounter"`

	// A rival ship that flies by the table (the mirror's): FleetFuel is its
	// fuel at launch and the most it holds, FleetRefuel the fuel its camp
	// refines a landed day (on top of any fuel in its stockpile), and it
	// leaves a site early only for one scoring FleetMoveMargin better.
	FleetFuel       int     `json:"fleet_fuel,omitempty"`
	FleetRefuel     int     `json:"fleet_refuel_per_day,omitempty"`
	FleetMoveMargin float64 `json:"fleet_move_margin,omitempty"`
}

// ExpeditionScore weights a site: its tier, its tags, never visited, each
// known faction presence, a colony of ours already there (when the ship
// carries a founding crew), one that needs crew (when it carries any), and
// each unit of fuel the jump costs.
type ExpeditionScore struct {
	Tiers     map[string]float64 `json:"tiers,omitempty"`
	Tags      map[string]float64 `json:"tags,omitempty"`
	Unvisited float64            `json:"unvisited"`
	Presence  float64            `json:"presence"`
	Colonized float64            `json:"colonized"`
	Reinforce float64            `json:"reinforce"`
	PerFuel   float64            `json:"per_fuel"`
}

// ExpeditionSite is one charted site as the Star Map shows it.
type ExpeditionSite struct {
	ID   string
	Tier string
	Tags []string
	// Visited: the ship has been in orbit (or the site was settled).
	Visited bool
	// Presences is how many faction presences the player knows there.
	Presences int
	// Colonists is our colony's living crew there (0: none).
	Colonists int
	// Cost is the fuel the jump there costs from where the ship is.
	Cost int
}

// ExpeditionView is what the director reads: the star date, the site in
// orbit ("" in transit or adrift), the ship's fuel and crew, the hold, and
// every charted site with a level (sorted by ID).
type ExpeditionView struct {
	Day   int
	Here  string
	Fuel  int
	Crew  int
	Hold  map[string]int
	Sites []ExpeditionSite
}

// Site is the view's site id, or nil.
func (v *ExpeditionView) Site(id string) *ExpeditionSite {
	for i := range v.Sites {
		if v.Sites[i].ID == id {
			return &v.Sites[i]
		}
	}
	return nil
}

// Affordable reports whether the jump to s leaves the fuel reserve.
func (t *DoctrineExpedition) Affordable(v ExpeditionView, s *ExpeditionSite) bool {
	return s.Cost > 0 && s.Cost <= v.Fuel-max(t.FuelReserve, 0)
}

// Destination is the best site, by score less the jump's fuel, among those
// worth the jump that the fuel reaches without touching the reserve, or
// nil. A site is worth it when the ship could found a colony there (no
// colony of ours, a founding crew aboard, the site scoring at least
// FoundScore) or when a colony of ours there needs crew and the ship has
// some. Nothing worth it: the ship stays where it is. seed breaks ties.
func (t *DoctrineExpedition) Destination(v ExpeditionView, seed int64) *ExpeditionSite {
	var best *ExpeditionSite
	bestScore := math.Inf(-1)
	for i := range v.Sites {
		s := &v.Sites[i]
		if s.ID == v.Here || !t.Affordable(v, s) || !t.Worth(v, s, seed) {
			continue
		}
		if sc := t.SiteScore(v, s, true, seed); sc > bestScore {
			best, bestScore = s, sc
		}
	}
	return best
}

// Worth reports whether s is worth a jump for a ship carrying v.Crew.
func (t *DoctrineExpedition) Worth(v ExpeditionView, s *ExpeditionSite, seed int64) bool {
	if s.Colonists > 0 {
		return s.Colonists < t.ReinforceBelow && v.Crew > 0
	}
	return v.Crew >= max(t.FoundCrew, 1) && t.SiteScore(v, s, false, seed) >= t.FoundScore
}

// SiteScore rates s for a ship carrying v.Crew; travel adds the jump's
// fuel cost. A small jitter from seed breaks ties the same way every run.
func (t *DoctrineExpedition) SiteScore(v ExpeditionView, s *ExpeditionSite, travel bool, seed int64) float64 {
	w := t.Score
	sc := w.Tiers[s.Tier]
	for _, tag := range s.Tags {
		sc += w.Tags[tag]
	}
	if !s.Visited {
		sc += w.Unvisited
	}
	sc += w.Presence * float64(s.Presences)
	if s.Colonists > 0 {
		if v.Crew >= max(t.FoundCrew, 1) {
			sc += w.Colonized
		}
		if s.Colonists < t.ReinforceBelow && v.Crew > 0 {
			sc += w.Reinforce
		}
	}
	if travel {
		sc += w.PerFuel * float64(s.Cost)
	}
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%d:%s", seed, s.ID)
	return sc + float64(h.Sum64()%1000)/1e5
}

// EncounterAnswer is the first of the table's answers that is possible: a
// parley when they will talk, an evade when the fuel covers it without
// touching the reserve, a boarding always. A nil table tries them in that
// order.
func (t *DoctrineExpedition) EncounterAnswer(canParley bool, evadeFuel, fuel int) string {
	order := []string{AnswerParley, AnswerEvade, AnswerBoard}
	if t != nil && len(t.Encounter) > 0 {
		order = t.Encounter
	}
	if t != nil {
		fuel -= max(t.FuelReserve, 0)
	}
	for _, a := range order {
		switch {
		case a == AnswerParley && canParley, a == AnswerEvade && evadeFuel <= fuel, a == AnswerBoard:
			return a
		}
	}
	return AnswerEvade
}
