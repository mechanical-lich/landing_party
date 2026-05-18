package campaign

import (
	"math"

	"github.com/mechanical-lich/landing_party/internal/objective"
)

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
	// Quests holds per-quest progress (status).
	Quests map[string]*QuestProgress `json:"quests,omitempty"`
	// QuestDefs are the campaign's generated quest definitions, persisted here
	// (the procedural overworld is not re-derivable from a static file).
	// Bound at session start via BindQuests.
	QuestDefs []Quest `json:"quest_defs,omitempty"`
	// GenSeq counts dynamic-generation events; combined with Seed it makes
	// every expansion deterministic and reproducible across save/load.
	GenSeq int `json:"gen_seq,omitempty"`
	// TravelSeq counts jumps; it advances the per-travel quest roll every
	// jump (even on a miss) so the 1d6 chance actually varies, while staying
	// deterministic and persisted.
	TravelSeq int `json:"travel_seq,omitempty"`
	// Won/Lost are terminal campaign states (reached Home / total wipe).
	Won  bool `json:"won,omitempty"`
	Lost bool `json:"lost,omitempty"`
	// KilledTargets is the set of quest IDs whose tagged target entity has
	// died (named bosses / bounty creatures). Persisted so completion
	// survives freeze/thaw and save/load.
	KilledTargets map[string]bool `json:"killed_targets,omitempty"`

	questByID map[string]*Quest               `json:"-"`
	questEval map[string]*objective.Evaluator `json:"-"`
}

// HomeKind marks the single "Home" destination; travelling there wins the run.
const HomeKind = "home"

// HomeLocation returns the Home destination descriptor, or nil.
func (c *Campaign) HomeLocation() *Location {
	for _, loc := range c.Locations {
		if loc.Kind == HomeKind {
			return loc
		}
	}
	return nil
}

// MarkTargetKilled records that the tagged target for questID has died.
func (c *Campaign) MarkTargetKilled(questID string) {
	if questID == "" {
		return
	}
	if c.KilledTargets == nil {
		c.KilledTargets = map[string]bool{}
	}
	c.KilledTargets[questID] = true
}

// StoredColonists totals colonists not on the live level: the ship roster plus
// the recorded count left on every (frozen) location. The caller adds live
// colonists on the active level to decide a total wipe.
func (c *Campaign) StoredColonists() int {
	n := 0
	if c.Ship != nil {
		n += len(c.Ship.Roster)
	}
	for _, loc := range c.Locations {
		n += loc.Colonists
	}
	return n
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
// to toID: zero if it is the current location, otherwise the rounded star-map
// distance between them.
func (c *Campaign) FuelCost(toID string) int {
	to := c.Locations[toID]
	if to == nil || toID == c.CurrentLocationID {
		return 0
	}
	from := c.Locations[c.CurrentLocationID]
	if from == nil {
		return 0
	}
	return int(math.Round(math.Hypot(to.X-from.X, to.Y-from.Y)))
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
