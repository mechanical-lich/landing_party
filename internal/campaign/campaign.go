package campaign

import (
	"math"
	"strings"

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
	// ScanSeq counts ship-scanner sweeps; combined with Seed it makes each
	// scan's roll/placement deterministic and reproducible across save/load.
	ScanSeq int `json:"scan_seq,omitempty"`
	// Won/Lost are terminal campaign states (reached Home / total wipe).
	Won  bool `json:"won,omitempty"`
	Lost bool `json:"lost,omitempty"`
	// KilledTargets is the set of quest IDs whose tagged target entity has
	// died (named bosses / bounty creatures). Persisted so completion
	// survives freeze/thaw and save/load.
	KilledTargets map[string]bool `json:"killed_targets,omitempty"`
	// KnownTechs is the set of tech keys the colony has researched. Stored
	// here (not on Settlement) so it persists across planets and save/load.
	KnownTechs []string `json:"known_techs,omitempty"`

	// KnownEntities is the set of blueprint IDs the player has hovered at
	// least once. Populated regardless of whether the Encyclopedia is
	// unlocked, so the bestiary is already populated the moment the research
	// finishes. Persisted across save/load.
	KnownEntities []string `json:"known_entities,omitempty"`

	// TravelEdges counts how many times the ship has jumped directly between
	// each pair of locations, keyed by a canonical unordered pair. The Star Map
	// draws these as trade-route lines that darken with use.
	TravelEdges map[string]int `json:"travel_edges,omitempty"`

	questByID map[string]*Quest               `json:"-"`
	questEval map[string]*objective.Evaluator `json:"-"`
}

// travelEdgeKey builds the canonical (order-independent) key for a location pair.
func travelEdgeKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "\x00" + b
}

// RecordTravel increments the traversal count for the fromID↔toID route.
func (c *Campaign) RecordTravel(fromID, toID string) {
	if fromID == "" || toID == "" || fromID == toID {
		return
	}
	if c.TravelEdges == nil {
		c.TravelEdges = map[string]int{}
	}
	c.TravelEdges[travelEdgeKey(fromID, toID)]++
}

// TravelRoute is one traversed route between two locations with its use count.
type TravelRoute struct {
	A, B  string
	Count int
}

// TravelRoutes returns every recorded route, for rendering on the Star Map.
func (c *Campaign) TravelRoutes() []TravelRoute {
	out := make([]TravelRoute, 0, len(c.TravelEdges))
	for k, n := range c.TravelEdges {
		if i := strings.IndexByte(k, 0); i >= 0 {
			out = append(out, TravelRoute{A: k[:i], B: k[i+1:], Count: n})
		}
	}
	return out
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

// HasTech reports whether the campaign has researched the given tech key.
func (c *Campaign) HasTech(key string) bool {
	for _, k := range c.KnownTechs {
		if k == key {
			return true
		}
	}
	return false
}

// UnlockTech adds key to KnownTechs if not already present.
func (c *Campaign) UnlockTech(key string) {
	if !c.HasTech(key) {
		c.KnownTechs = append(c.KnownTechs, key)
	}
}

// Ship-scanner research chains. A level is the count of consecutive techs
// researched from the front of each chain.
var (
	scannerPowerTechs   = []string{"ship_scanner_1", "ship_scanner_2", "ship_scanner_3"}
	scanEfficiencyTechs = []string{"scan_efficiency_1", "scan_efficiency_2"}
)

func (c *Campaign) techChainLevel(chain []string) int {
	n := 0
	for _, k := range chain {
		if !c.HasTech(k) {
			break
		}
		n++
	}
	return n
}

// ScannerLevel is the ship scanner's power level (0 = no scanner researched).
func (c *Campaign) ScannerLevel() int { return c.techChainLevel(scannerPowerTechs) }

// ScanFuelCost is the fuel one scan burns at the current efficiency research.
func (c *Campaign) ScanFuelCost() int {
	costs := genConfig().ScanFuelCosts
	if len(costs) == 0 {
		return 0
	}
	lvl := c.techChainLevel(scanEfficiencyTechs)
	if lvl >= len(costs) {
		lvl = len(costs) - 1
	}
	return costs[lvl]
}

// KnownTechSet returns KnownTechs as a set for O(1) lookup.
func (c *Campaign) KnownTechSet() map[string]bool {
	set := make(map[string]bool, len(c.KnownTechs))
	for _, k := range c.KnownTechs {
		set[k] = true
	}
	return set
}

// IsKnownEntity reports whether blueprint has been registered in the
// Encyclopedia (i.e. the player has hovered an entity of this type at least
// once).
func (c *Campaign) IsKnownEntity(blueprint string) bool {
	for _, b := range c.KnownEntities {
		if b == blueprint {
			return true
		}
	}
	return false
}

// AddKnownEntity registers blueprint in the Encyclopedia. Returns true if the
// blueprint was newly added; false if it was already known (or empty). The
// boolean lets the hover hot path emit a one-shot "newly catalogued" message
// without keeping a separate dedup set.
func (c *Campaign) AddKnownEntity(blueprint string) bool {
	if blueprint == "" || c.IsKnownEntity(blueprint) {
		return false
	}
	c.KnownEntities = append(c.KnownEntities, blueprint)
	return true
}

// KnownEntitySet returns KnownEntities as a set for O(1) lookup.
func (c *Campaign) KnownEntitySet() map[string]bool {
	set := make(map[string]bool, len(c.KnownEntities))
	for _, b := range c.KnownEntities {
		set[b] = true
	}
	return set
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

// StoredColonists totals colonists recorded on frozen locations. The ship's
// crew lives on the ship level (counted separately by the wipe check) and is no
// longer part of this tally.
func (c *Campaign) StoredColonists() int {
	n := 0
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
